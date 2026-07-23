package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// 上游作业监控服务地址解析所需的环境变量。
// 生产环境通过这些环境变量注入服务地址，而非在配置文件中写死 IP。
const (
	envRestfulIP   = "RESTFULIP" // 作业监控服务 IP（优先）
	envJSFServer   = "JSF_SERVER" // 作业监控服务 IP（RESTFULIP 为空时的回退）
	envRestfulPort = "RESTFULPORT" // 作业监控服务端口
	defaultUpstreamPort = 18080    // 端口环境变量为空时的默认值
	upstreamPath        = "/job/call" // 上游统一入口路径
)

type Config struct {
	Server   ServerCfg   `yaml:"server"`
	Upstream UpstreamCfg `yaml:"upstream"`
	Sampler  SamplerCfg  `yaml:"sampler"`
	Log      LogCfg      `yaml:"log"`
	AI       AICfg       `yaml:"ai"`
	Auth     AuthCfg     `yaml:"auth"`
	Storage  StorageCfg  `yaml:"storage"`
}

type ServerCfg struct {
	Port int `yaml:"port"`
}

type UpstreamCfg struct {
	// JobServiceURL 仅作为开发/测试环境的回退地址。
	// 生产环境应通过环境变量 RESTFULIP/JSF_SERVER + RESTFULPORT 注入，
	// 见 ResolveURL。
	JobServiceURL string `yaml:"jobServiceURL"`
	TimeoutSec    int    `yaml:"timeoutSec"`
}

// ResolveURL 解析上游作业监控服务的完整 URL。
//
// 解析优先级：
//  1. 环境变量 RESTFULIP（IP） + RESTFULPORT（端口，缺省 18080）
//  2. 环境变量 JSF_SERVER（IP，RESTFULIP 为空时回退） + RESTFULPORT
//  3. 配置文件 upstream.jobServiceURL（开发/测试回退）
//
// 生产环境通过环境变量注入地址，避免在配置文件中写死 IP；
// 当环境变量均未设置时，回退到配置文件中的 jobServiceURL，保证开发流程不变。
func (u *UpstreamCfg) ResolveURL() string {
	ip := strings.TrimSpace(os.Getenv(envRestfulIP))
	if ip == "" {
		ip = strings.TrimSpace(os.Getenv(envJSFServer))
	}
	if ip != "" {
		port := resolveUpstreamPort()
		url := fmt.Sprintf("http://%s:%d%s", ip, port, upstreamPath)
		slog.Info("upstream url resolved from env",
			"envIP", envRestfulIP, "envPort", envRestfulPort,
			"ip", ip, "port", port, "url", url)
		return url
	}
	// 环境变量未设置：回退配置文件（开发/测试）
	slog.Info("upstream url from config (env vars not set)",
		"jobServiceURL", u.JobServiceURL)
	return u.JobServiceURL
}

// resolveUpstreamPort 从 RESTFULPORT 读取端口，非法或为空时返回默认值 18080。
func resolveUpstreamPort() int {
	raw := strings.TrimSpace(os.Getenv(envRestfulPort))
	if raw == "" {
		return defaultUpstreamPort
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port <= 0 || port > 65535 {
		slog.Warn("invalid RESTFULPORT, using default",
			"raw", raw, "default", defaultUpstreamPort)
		return defaultUpstreamPort
	}
	return port
}

type SamplerCfg struct {
	Enabled       bool `yaml:"enabled"`
	JsfIntervalSec int  `yaml:"jsfIntervalSec"`
	JobIntervalSec int  `yaml:"jobIntervalSec"`
	JobPageSize   int  `yaml:"jobPageSize"`
	JobPageSleepMs int  `yaml:"jobPageSleepMs"`
	RetainDays    int  `yaml:"retainDays"`
}

type LogCfg struct {
	NgpEnv           string `yaml:"ngpEnv"`
	ProjectsConfRel  string `yaml:"projectsConfRel"`
	MaxLogLines      int    `yaml:"maxLogLines"`
	// File 指定日志文件路径；为空则只输出到 stdout。
	File string `yaml:"file"`
	// Level 日志级别：debug/info/warn/error，默认 info。
	Level string `yaml:"level"`
	// MaxSizeMB 单个日志文件最大 MB，超过则滚动切分，默认 100。
	MaxSizeMB int `yaml:"maxSizeMB"`
	// MaxBackups 保留的旧日志文件数量，默认 7。
	MaxBackups int `yaml:"maxBackups"`
	// MaxAgeDays 旧日志文件最大保留天数，默认 30。
	MaxAgeDays int `yaml:"maxAgeDays"`
}

type AICfg struct {
	Enabled  bool   `yaml:"enabled"`
	Provider string `yaml:"provider"`
	Endpoint string `yaml:"endpoint"`
	APIKey   string `yaml:"apiKey"`
	Model    string `yaml:"model"`
}

type StorageCfg struct {
	SqlitePath string `yaml:"sqlitePath"`
}

// AuthCfg 控制登录鉴权。JWTSecret 为空时使用内置兜底值（仅适合测试）。
type AuthCfg struct {
	JWTSecret    string `yaml:"jwtSecret"`
	TokenTTLHour int    `yaml:"tokenTTLHour"`
}

var (
	once sync.Once
	cfg  *Config
)

// Load reads config from the given path. If path is empty, default locations
// are tried: env BFF_CONF, then ./configs/config.yaml.
func Load(path string) (*Config, error) {
	if path == "" {
		path = os.Getenv("BFF_CONF")
	}
	if path == "" {
		path = "configs/config.yaml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	c.applyDefaults()
	once.Do(func() { cfg = &c })
	cfg = &c
	return &c, nil
}

func (c *Config) applyDefaults() {
	if c.Server.Port == 0 {
		c.Server.Port = 8088
	}
	if c.Upstream.TimeoutSec == 0 {
		c.Upstream.TimeoutSec = 10
	}
	if c.Sampler.JsfIntervalSec == 0 {
		c.Sampler.JsfIntervalSec = 60
	}
	if c.Sampler.JobIntervalSec == 0 {
		c.Sampler.JobIntervalSec = 60
	}
	if c.Sampler.JobPageSize == 0 {
		c.Sampler.JobPageSize = 500
	}
	if c.Sampler.RetainDays == 0 {
		c.Sampler.RetainDays = 30
	}
	if c.Log.NgpEnv == "" {
		c.Log.NgpEnv = "NGP"
	}
	if c.Log.ProjectsConfRel == "" {
		c.Log.ProjectsConfRel = "configs/ndp/projects.conf"
	}
	if c.Log.MaxLogLines == 0 {
		c.Log.MaxLogLines = 5000
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Log.MaxSizeMB == 0 {
		c.Log.MaxSizeMB = 100
	}
	if c.Log.MaxBackups == 0 {
		c.Log.MaxBackups = 7
	}
	if c.Log.MaxAgeDays == 0 {
		c.Log.MaxAgeDays = 30
	}
	if c.Storage.SqlitePath == "" {
		c.Storage.SqlitePath = "./data/dashboard.db"
	}
	if c.Auth.TokenTTLHour == 0 {
		c.Auth.TokenTTLHour = 72
	}
}
