// Package config 负责加载与校验 YAML 配置文件。
//
// 1.0 阶段只支持 YAML + 环境变量覆盖（不引入运行时热更新）。
// 详细设计见 ../../../docs/backend/02-provider.md。
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 顶层配置结构
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Storage   StorageConfig   `yaml:"storage"`
	Security  SecurityConfig  `yaml:"security"`
	Workers   []WorkerConfig  `yaml:"workers"`
	Routing   RoutingConfig   `yaml:"routing"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Providers map[string]ProviderConfig `yaml:"providers"`
}

// ServerConfig HTTP server 配置
type ServerConfig struct {
	Listen string `yaml:"listen"`
	TLS    TLSConfig `yaml:"tls"`
}

// TLSConfig TLS 配置（1.0 简化版，2.0 上 mTLS）
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	SQLitePath string `yaml:"sqlite_path"`
}

// SecurityConfig 安全相关配置
type SecurityConfig struct {
	MasterKeyEnv string `yaml:"master_key_env"`
	JWTSecretEnv string `yaml:"jwt_secret_env"`
}

// WorkerConfig 注册到 Master 的 Worker 信息
type WorkerConfig struct {
	ID        string   `yaml:"id"`
	Region    string   `yaml:"region"`
	Endpoint  string   `yaml:"endpoint"`
	Providers []string `yaml:"providers"`
}

// RoutingConfig 路由策略
type RoutingConfig struct {
	Strategy             string        `yaml:"strategy"`
	HealthCheckInterval  time.Duration `yaml:"health_check_interval"`
	SingleFallback       bool          `yaml:"single_fallback"`
}

// RateLimitConfig 限流配置（review 2.3）
type RateLimitConfig struct {
	User    UserRateLimit    `yaml:"user"`
	Provider map[string]int  `yaml:"provider"`
	Global  GlobalRateLimit  `yaml:"global"`
}

// UserRateLimit 用户级限流
type UserRateLimit struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
	Burst            int `yaml:"burst"`
}

// GlobalRateLimit 全局并发限流
type GlobalRateLimit struct {
	ConcurrentStreams int `yaml:"concurrent_streams"`
}

// ProviderConfig Provider 上游配置（仅 Worker 用）
type ProviderConfig struct {
	BaseURL         string        `yaml:"base_url"`
	UpstreamTimeout time.Duration `yaml:"upstream_timeout"`
}

// Load 从 path 加载配置。环境变量可覆盖 secret 类字段。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// 默认值
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = ":8080"
	}
	if cfg.Routing.HealthCheckInterval == 0 {
		cfg.Routing.HealthCheckInterval = 10 * time.Second
	}
	if cfg.RateLimit.User.RequestsPerMinute == 0 {
		cfg.RateLimit.User.RequestsPerMinute = 100
	}
	if cfg.RateLimit.User.Burst == 0 {
		cfg.RateLimit.User.Burst = 20
	}
	if cfg.RateLimit.Global.ConcurrentStreams == 0 {
		cfg.RateLimit.Global.ConcurrentStreams = 500
	}

	// 校验
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	// 1.0 阶段：master_key_env 未配置不报错，MasterKey() 调用时才报错
	// 2.0 阶段：要求显式配置 + env 已设
	if c.Security.JWTSecretEnv != "" {
		if os.Getenv(c.Security.JWTSecretEnv) == "" {
			return fmt.Errorf("security.jwt_secret_env=%s but env not set", c.Security.JWTSecretEnv)
		}
	}
	return nil
}

// MasterKey 从环境变量读取 master key（用于 AES-GCM 加密）
func (c *Config) MasterKey() ([]byte, error) {
	if c.Security.MasterKeyEnv == "" {
		return nil, fmt.Errorf("security.master_key_env not configured")
	}
	v := os.Getenv(c.Security.MasterKeyEnv)
	if v == "" {
		return nil, fmt.Errorf("env %s is empty", c.Security.MasterKeyEnv)
	}
	return []byte(v), nil
}
