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
	ProjectsConfRel string `yaml:"projectsConfRel"`
	MaxLogLines      int    `yaml:"maxLogLines"`
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
	if c.Storage.SqlitePath == "" {
		c.Storage.SqlitePath = "./data/dashboard.db"
	}
	if c.Auth.TokenTTLHour == 0 {
		c.Auth.TokenTTLHour = 72
	}
}
