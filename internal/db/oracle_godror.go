//go:build cgo

package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/godror/godror"
)

// godrorDriver 使用原生 Oracle 驱动 (database/sql + godror) 执行健康检查。
// 优点: 无需外部 sqlplus, 程序内完成连接与查询。
// 代价: 需要 CGO 编译, 运行时依赖 Oracle Instant Client 动态库(通过 LD_LIBRARY_PATH 指定)。
type godrorDriver struct {
	cfg DriverConfig
}

func newGodrorDriver(cfg DriverConfig) (DBDriver, error) {
	if cfg.InstantClient == "" {
		return nil, fmt.Errorf("godror 驱动需要配置 instant_client_lib (Oracle Instant Client 路径)")
	}
	return &godrorDriver{cfg: cfg}, nil
}

// Check 通过 database/sql 执行查询并扫描结果。
func (d *godrorDriver) Check(ctx context.Context) Result {
	start := time.Now()

	dsn := fmt.Sprintf(`user="%s" password="%s" connectString="%s:%d/%s"`,
		d.cfg.User, d.cfg.Password, d.cfg.Host, d.cfg.Port, d.cfg.Service)

	connCtx, cancel := context.WithTimeout(ctx, d.cfg.ConnTimeout)
	defer cancel()

	conn, err := sql.Open("godror", dsn)
	if err != nil {
		return Result{OK: false, Detail: "打开连接失败: " + err.Error(), LatencyMs: time.Since(start).Milliseconds()}
	}
	defer conn.Close()

	if err := conn.PingContext(connCtx); err != nil {
		return Result{OK: false, Detail: "Ping 失败: " + err.Error(), LatencyMs: time.Since(start).Milliseconds()}
	}

	queryCtx, qCancel := context.WithTimeout(ctx, d.cfg.QueryTimeout)
	defer qCancel()

	rows, err := conn.QueryContext(queryCtx, d.cfg.Query)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return Result{OK: false, Detail: "查询失败: " + err.Error(), LatencyMs: latency}
	}
	defer rows.Close()

	// 读取第一行结果, 有返回即认为可用。
	if rows.Next() {
		var val interface{}
		if err := rows.Scan(&val); err != nil {
			return Result{OK: false, Detail: "扫描结果失败: " + err.Error(), LatencyMs: latency}
		}
		return Result{OK: true, Detail: fmt.Sprintf("查询正常: 返回 %v", val), LatencyMs: latency}
	}
	return Result{OK: false, Detail: "查询无返回结果", LatencyMs: latency}
}
