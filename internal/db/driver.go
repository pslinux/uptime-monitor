// Package db 抽象数据库健康检查能力。
// 提供 DBDriver 接口, 以便支持多种实现:
//   - sqlplus: 调用外部 sqlplus 客户端执行查询 (V1, 纯 Go 静态编译, 部署简单)
//   - godror:  使用原生 Oracle 驱动 database/sql (V2, 免 sqlplus, 需 CGO/Instant Client)
package db

import (
	"context"
	"fmt"
	"time"
)

// Result 单次健康检查结果。
type Result struct {
	OK        bool   // 数据库是否可用
	Detail    string // 详细说明/错误信息
	LatencyMs int64  // 检查耗时(毫秒)
}

// DBDriver 健康检查驱动器接口。
type DBDriver interface {
	// Check 执行健康检查 SQL 并返回结果。
	Check(ctx context.Context) Result
}

// NewDriver 根据配置的 driver 名称创建对应驱动。
func NewDriver(driverName string, cfg DriverConfig) (DBDriver, error) {
	switch driverName {
	case "sqlplus":
		return newSqlplusDriver(cfg), nil
	case "godror":
		// godror 驱动在未启用 CGO 编译时不可用。
		return newGodrorDriver(cfg)
	default:
		return nil, fmt.Errorf("不支持的驱动: %s", driverName)
	}
}

// DriverConfig 传给驱动的连接参数。
type DriverConfig struct {
	Host           string
	Port           int
	User           string
	Password       string
	Service        string
	Query          string
	ConnTimeout    time.Duration
	QueryTimeout   time.Duration
	SqlplusPath    string
	OracleHome     string // sqlplus 模式下 ORACLE_HOME, 解决 SP2-0667
	InstantClient  string // godror 模式下 LD_LIBRARY_PATH
}
