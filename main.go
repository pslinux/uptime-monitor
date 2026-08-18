// Uptime-Monitor: Oracle 数据库健康监控程序(Go 版)。
//
// 功能:
//   - 周期性连接 Oracle 执行健康检查 SQL(默认 SELECT 1 FROM DUAL)
//   - 将结果上报到 Uptime Kuma 的 Push 接口
//   - 支持 sqlplus 与 godror 两种数据库驱动
//   - 独立配置文件 + 环境变量覆盖
//   - 支持优雅停机(处理 SIGINT/SIGTERM)
//
// 用法:
//   uptime-monitor -c /etc/uptime-monitor/config.yaml
//   uptime-monitor -v
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"uptime-monitor/internal/config"
	"uptime-monitor/internal/db"
	"uptime-monitor/internal/logger"
	"uptime-monitor/internal/notifier"
)

// version 通过 -ldflags "-X main.version=x.y.z" 在构建时注入, 默认 dev。
var version = "dev"

func main() {
	configPath := flag.String("c", "config.yaml", "配置文件路径")
	showVersion := flag.Bool("v", false, "打印版本号并退出")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "配置加载失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	log, err := logger.New(cfg.Log.Level, cfg.Log.File, cfg.Log.MaxSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "日志初始化失败: %v\n", err)
		os.Exit(1)
	}
	defer log.Close()

	// 创建数据库驱动
	driver, err := db.NewDriver(cfg.Monitor.Driver, db.DriverConfig{
		Host:          cfg.DB.Host,
		Port:          cfg.DB.Port,
		User:          cfg.DB.User,
		Password:      cfg.DB.Password,
		Service:       cfg.DB.Service,
		Query:         cfg.Monitor.Query,
		ConnTimeout:   time.Duration(cfg.DB.ConnTimeoutSec) * time.Second,
		QueryTimeout:  time.Duration(cfg.Monitor.QueryTimeoutSec) * time.Second,
		SqlplusPath:   cfg.DB.SqlplusPath,
		OracleHome:    cfg.DB.OracleHome,
		InstantClient: cfg.DB.InstantClientLib,
	})
	if err != nil {
		log.Error("驱动初始化失败: %v", err)
		os.Exit(1)
	}

	// 创建上报客户端
	pusher, err := notifier.New(cfg.Push.URL, 10*time.Second)
	if err != nil {
		log.Error("上报客户端初始化失败: %v", err)
		os.Exit(1)
	}

	// 优雅停机信号
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("Uptime-Monitor 启动: driver=%s host=%s:%d service=%s interval=%ds",
		cfg.Monitor.Driver, cfg.DB.Host, cfg.DB.Port, cfg.DB.Service, cfg.Monitor.IntervalSeconds)

	var wg sync.WaitGroup
	wg.Add(1)
	go runLoop(ctx, &wg, cfg, log, driver, pusher)

	<-ctx.Done()
	log.Info("收到退出信号, 正在关闭...")
	stop()
	wg.Wait()
	log.Info("已退出")
}

// runLoop 周期执行健康检查并上报。
func runLoop(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config,
	log *logger.Logger, driver db.DBDriver, pusher *notifier.Push) {
	defer wg.Done()

	// 启动时立即执行一次, 便于快速验证。
	executeOnce(ctx, cfg, log, driver, pusher)

	ticker := time.NewTicker(cfg.Interval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			executeOnce(ctx, cfg, log, driver, pusher)
		}
	}
}

// executeOnce 执行单次检查(含重试)并上报结果。
func executeOnce(ctx context.Context, cfg *config.Config, log *logger.Logger,
	driver db.DBDriver, pusher *notifier.Push) {

	retry := cfg.Monitor.RetryTimes
	if retry <= 0 {
		retry = 1
	}
	var result db.Result
	for i := 0; i < retry; i++ {
		result = driver.Check(ctx)
		if result.OK {
			break
		}
		if i < retry-1 {
			log.Warn("检查失败(第 %d/%d 次): %s, 稍后重试", i+1, retry, result.Detail)
			time.Sleep(time.Duration(cfg.Monitor.RetryIntervalMs) * time.Millisecond)
		}
	}

	status := "down"
	if result.OK {
		status = "up"
	}

	msg := ""
	if !result.OK {
		msg = result.Detail
		log.Error("数据库异常: %s", result.Detail)
	} else {
		log.Info("数据库正常: %s (耗时 %dms)", result.Detail, result.LatencyMs)
	}

	if err := pusher.Report(status, msg, result.LatencyMs); err != nil {
		log.Error("上报失败: %v", err)
	}
}
