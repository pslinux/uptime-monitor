// Package config 负责加载与校验监控程序配置。
// 配置采用独立 YAML 文件, 支持环境变量覆盖, 便于不同环境(测试/生产)复用同一二进制。
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 顶层配置结构。
type Config struct {
	Monitor Monitor `yaml:"monitor"`
	DB      DB      `yaml:"db"`
	Push    Push    `yaml:"push"`
	Log     Log     `yaml:"log"`
}

// Monitor 运行行为配置。
type Monitor struct {
	IntervalSeconds int    `yaml:"interval_seconds"` // 执行间隔(秒), 需与 Uptime Kuma 心跳一致
	Query           string `yaml:"query"`            // 健康检查 SQL
	QueryTimeoutSec int    `yaml:"query_timeout_sec"`
	Driver          string `yaml:"driver"`           // sqlplus | godror
	RetryTimes      int    `yaml:"retry_times"`      // V3: 单次检查失败重试次数
	RetryIntervalMs int    `yaml:"retry_interval_ms"`
}

// DB 数据库连接配置。
type DB struct {
	Host             string `yaml:"host"`
	Port             int    `yaml:"port"`
	User             string `yaml:"user"`
	Password         string `yaml:"password"`
	Service          string `yaml:"service"` // Oracle SERVICE_NAME
	ConnTimeoutSec   int    `yaml:"conn_timeout_sec"`
	SqlplusPath      string `yaml:"sqlplus_path"`       // sqlplus 驱动模式下可执行文件路径
	OracleHome       string `yaml:"oracle_home"`        // sqlplus 模式 ORACLE_HOME, 解决 SP2-0667 消息文件缺失
	InstantClientLib string `yaml:"instant_client_lib"` // godror 模式需要的 LD_LIBRARY_PATH
}

// Push Uptime Kuma Push 接口配置。
type Push struct {
	URL string `yaml:"url"` // 形如 http://IP:7001/api/push/<token>
}

// Log 日志配置。
type Log struct {
	File    string `yaml:"file"` // 日志文件路径, 空表示输出到 stdout
	Level   string `yaml:"level"` // debug | info | warn | error
	MaxSize int    `yaml:"max_size_mb"` // 单文件大小上限(MB), V3 简单轮转
}

// Load 从指定路径加载配置文件, 并用环境变量覆盖关键字段。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cfg.applyEnvOverrides()
	return cfg, nil
}

// applyDefaults 填充默认值。
func (c *Config) applyDefaults() {
	if c.Monitor.IntervalSeconds <= 0 {
		c.Monitor.IntervalSeconds = 60
	}
	if c.Monitor.Query == "" {
		c.Monitor.Query = "SELECT 1 FROM DUAL"
	}
	if c.Monitor.QueryTimeoutSec <= 0 {
		c.Monitor.QueryTimeoutSec = 15
	}
	if c.Monitor.Driver == "" {
		c.Monitor.Driver = "sqlplus"
	}
	if c.DB.Port <= 0 {
		c.DB.Port = 1521
	}
	if c.DB.ConnTimeoutSec <= 0 {
		c.DB.ConnTimeoutSec = 10
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Log.MaxSize <= 0 {
		c.Log.MaxSize = 10
	}
}

// Validate 校验必填字段。
func (c *Config) Validate() error {
	if c.DB.Host == "" {
		return fmt.Errorf("db.host 不能为空")
	}
	if c.DB.User == "" {
		return fmt.Errorf("db.user 不能为空")
	}
	if c.DB.Password == "" {
		return fmt.Errorf("db.password 不能为空(建议通过环境变量注入)")
	}
	if c.DB.Service == "" {
		return fmt.Errorf("db.service 不能为空")
	}
	if c.Push.URL == "" {
		return fmt.Errorf("push.url 不能为空")
	}
	if c.Monitor.Driver != "sqlplus" && c.Monitor.Driver != "godror" {
		return fmt.Errorf("monitor.driver 仅支持 sqlplus 或 godror, 当前: %s", c.Monitor.Driver)
	}
	return nil
}

// applyEnvOverrides 允许环境变量覆盖敏感/环境相关配置。
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("UM_DB_HOST"); v != "" {
		c.DB.Host = v
	}
	if v := os.Getenv("UM_DB_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &c.DB.Port)
	}
	if v := os.Getenv("UM_DB_USER"); v != "" {
		c.DB.User = v
	}
	if v := os.Getenv("UM_DB_PASSWORD"); v != "" {
		c.DB.Password = v
	}
	if v := os.Getenv("UM_DB_SERVICE"); v != "" {
		c.DB.Service = v
	}
	if v := os.Getenv("UM_DB_ORACLE_HOME"); v != "" {
		c.DB.OracleHome = v
	}
	if v := os.Getenv("UM_PUSH_URL"); v != "" {
		c.Push.URL = v
	}
	if v := os.Getenv("UM_INTERVAL"); v != "" {
		fmt.Sscanf(v, "%d", &c.Monitor.IntervalSeconds)
	}
}

// Interval 返回执行周期。
func (c *Config) Interval() time.Duration {
	return time.Duration(c.Monitor.IntervalSeconds) * time.Second
}
