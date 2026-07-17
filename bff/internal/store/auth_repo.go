package store

import (
	"database/sql"
	"errors"
	"time"
)

// 认证相关错误。注意与 user_job_stat 用的 UserRepo 区分（后者用于采样统计）。
var (
	ErrUserExists   = errors.New("user already exists")
	ErrUserNotFound = errors.New("user not found")
)

// AuthUser 是 users 表的行映射。
type AuthUser struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    int64
}

// AuthRepo 提供 users 表的读写（注册/登录）。
type AuthRepo struct {
	db *sql.DB
}

func NewAuthRepo(db *sql.DB) *AuthRepo { return &AuthRepo{db: db} }

// Create 插入新用户。用户名重复时返回 ErrUserExists。
func (r *AuthRepo) Create(username, passwordHash string) (*AuthUser, error) {
	now := time.Now().Unix()
	res, err := r.db.Exec(
		"INSERT INTO users(username, password_hash, created_at) VALUES(?,?,?)",
		username, passwordHash, now,
	)
	if err != nil {
		// username 上有 UNIQUE 约束，重复插入会失败；统一视为已存在。
		return nil, ErrUserExists
	}
	id, _ := res.LastInsertId()
	return &AuthUser{ID: id, Username: username, PasswordHash: passwordHash, CreatedAt: now}, nil
}

// FindByUsername 按用户名查询；不存在返回 ErrUserNotFound。
func (r *AuthRepo) FindByUsername(username string) (*AuthUser, error) {
	row := r.db.QueryRow(
		"SELECT id, username, password_hash, created_at FROM users WHERE username=?",
		username,
	)
	var u AuthUser
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}
