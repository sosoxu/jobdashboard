package config

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
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
	JobServiceURL string `yaml:"jobServiceURL"`
	TimeoutSec    int    `yaml:"timeoutSec"`
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
