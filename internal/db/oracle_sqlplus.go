package db

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// sqlplusDriver 通过调用外部 sqlplus 客户端执行健康检查 SQL。
// 优点: 纯 Go 静态编译, 无需 CGO, 打包/部署最简单。
// 依赖: 目标机器需安装 Oracle Instant Client + sqlplus。
type sqlplusDriver struct {
	cfg DriverConfig
}

func newSqlplusDriver(cfg DriverConfig) *sqlplusDriver {
	if cfg.SqlplusPath == "" {
		cfg.SqlplusPath = "sqlplus"
	}
	return &sqlplusDriver{cfg: cfg}
}

// Check 组装 SQL 并调用 sqlplus 执行。
func (d *sqlplusDriver) Check(ctx context.Context) Result {
	start := time.Now()
	sql := "SET PAGESIZE 0\nSET FEEDBACK OFF\nSET HEADING OFF\nSET TIMING OFF\n" +
		"WHENEVER SQLERROR EXIT SQL.SQLCODE\n" +
		d.cfg.Query + "\nEXIT;\n"

	dsn := d.connString()

	cmdCtx, cancel := context.WithTimeout(ctx, d.cfg.QueryTimeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, d.cfg.SqlplusPath, "-S", dsn)
	cmd.Stdin = strings.NewReader(sql)
	cmd.Env = d.env()
	out, err := cmd.CombinedOutput()
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return Result{OK: false, Detail: extractErr(out, err), LatencyMs: latency}
	}

	first := firstNonEmptyField(string(out))
	if first == "1" {
		return Result{OK: true, Detail: "查询正常: SELECT 1 FROM DUAL 返回 1", LatencyMs: latency}
	}
	return Result{OK: false, Detail: "查询异常, 输出: " + extractErr(out, nil), LatencyMs: latency}
}

func (d *sqlplusDriver) connString() string {
	// user/pass@//host:port/service
	return d.cfg.User + "/" + d.cfg.Password +
		"@//" + d.cfg.Host + ":" + itoa(d.cfg.Port) + "/" + d.cfg.Service
}

// env 组装子进程环境变量。
// 若配置了 oracle_home, 注入 ORACLE_HOME/TNS_ADMIN/NLS_LANG, 并把 bin/lib
// 追加到 PATH/LD_LIBRARY_PATH, 避免 sqlplus 报 SP2-0667 消息文件缺失。
func (d *sqlplusDriver) env() []string {
	env := os.Environ()
	if d.cfg.OracleHome == "" {
		return env
	}
	home := d.cfg.OracleHome
	sep := string(os.PathListSeparator)
	env = append(env,
		"ORACLE_HOME="+home,
		"TNS_ADMIN="+home+"/network/admin",
		"NLS_LANG=AMERICAN_AMERICA.AL32UTF8",
		"PATH="+home+"/bin"+sep+os.Getenv("PATH"),
		"LD_LIBRARY_PATH="+home+"/lib"+sep+os.Getenv("LD_LIBRARY_PATH"),
	)
	return env
}

// 简单 int->string, 避免引入 strconv 处重复书写。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
