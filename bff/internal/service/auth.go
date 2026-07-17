package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/dashboard/bff/internal/store"
)

// 认证业务错误。
var (
	ErrUserExists         = errors.New("用户名已存在")
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrInvalidToken       = errors.New("无效或已过期的登录凭证")
)

// AuthService 负责注册/登录与 token 签发/校验。
// token 采用 HMAC-SHA256 签名的自定义格式（payload.signature），
// 避免引入额外 JWT 依赖；payload 为 base64url 编码的 {u,e} JSON。
type AuthService struct {
	repo   *store.AuthRepo
	secret []byte
	ttl    time.Duration
}

func NewAuthService(repo *store.AuthRepo, secret string, ttlHours int) *AuthService {
	if secret == "" {
		secret = "job-dashboard-default-secret-change-me"
	}
	if ttlHours <= 0 {
		ttlHours = 72
	}
	return &AuthService{
		repo:   repo,
		secret: []byte(secret),
		ttl:    time.Duration(ttlHours) * time.Hour,
	}
}

// AuthResult 是登录/注册成功后返回给前端的结果。
type AuthResult struct {
	Token string   `json:"token"`
	User  AuthUser `json:"user"`
}

// AuthUser 是对外暴露的用户信息（不含密码哈希）。
type AuthUser struct {
	Username string `json:"username"`
}

// Register 创建新用户并直接签发 token（注册即登录）。
func (s *AuthService) Register(ctx context.Context, username, password string) (*AuthResult, error) {
	if username == "" || password == "" {
		return nil, errors.New("用户名和密码不能为空")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	if _, err := s.repo.Create(username, string(hash)); err != nil {
		if errors.Is(err, store.ErrUserExists) {
			return nil, ErrUserExists
		}
		return nil, err
	}
	token, err := s.issueToken(username)
	if err != nil {
		return nil, err
	}
	return &AuthResult{Token: token, User: AuthUser{Username: username}}, nil
}

// Login 校验密码后签发 token。
func (s *AuthService) Login(ctx context.Context, username, password string) (*AuthResult, error) {
	u, err := s.repo.FindByUsername(username)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	token, err := s.issueToken(username)
	if err != nil {
		return nil, err
	}
	return &AuthResult{Token: token, User: AuthUser{Username: username}}, nil
}

type tokenClaims struct {
	U string `json:"u"` // 用户名
	E int64  `json:"e"` // 过期时间（unix 秒）
}

func (s *AuthService) issueToken(username string) (string, error) {
	claims := tokenClaims{U: username, E: time.Now().Add(s.ttl).Unix()}
	body, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig, nil
}

// ParseToken 校验签名与有效期，返回用户名。
func (s *AuthService) ParseToken(token string) (string, error) {
	dot := -1
	for i, r := range token {
		if r == '.' {
			dot = i
			break
		}
	}
	if dot < 0 {
		return "", ErrInvalidToken
	}
	payload, sig := token[:dot], token[dot+1:]

	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", ErrInvalidToken
	}
	body, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", ErrInvalidToken
	}
	var c tokenClaims
	if err := json.Unmarshal(body, &c); err != nil {
		return "", ErrInvalidToken
	}
	if time.Now().Unix() >= c.E {
		return "", ErrInvalidToken
	}
	return c.U, nil
}
