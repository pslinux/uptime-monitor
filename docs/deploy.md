# Uptime-Monitor 部署文档

> 版本：v1.0.0 ｜ 适用平台：Linux（amd64/arm64）｜ 更新日期：2026-08-18

---

## 1. 概述

Uptime-Monitor 是使用 Go 编写的 Oracle 数据库健康监控程序。它周期连接 Oracle 执行 `SELECT 1 FROM DUAL`，并将结果上报到 Uptime Kuma 的 Push 接口。本文档说明从构建、配置到部署、验证的完整流程。

---

## 2. 部署架构

```
┌────────────┐   每60秒检查   ┌────────────────────┐   GET上报   ┌────────────────┐
│ 监控服务器  │ ────────────▶ │  Uptime-Monitor    │ ──────────▶ │  Uptime Kuma    │
│  (Oracle)  │ ◀──────────── │  (Go 二进制)        │  up/down    │  (uptime-kuma)   │
└────────────┘   返回结果      └────────────────────┘   │push     └────────────────┘
                                                       │
                                        超过"状态有效期"未上报自动判宕机
```

---

## 3. 环境要求

| 组件 | 要求 |
|------|------|
| 操作系统 | Linux（openEuler/CentOS/Ubuntu 均可） |
| 运行环境 | 无（Go 静态二进制，不依赖解释器） |
| sqlplus 驱动 | 需安装 Oracle Instant Client + sqlplus |
| godror 驱动 | 需 CGO 编译 + 运行机 Oracle Instant Client 动态库 |
| Uptime Kuma | 已部署，且已创建 Push 监控类型 |

---

## 4. 构建（可选，若直接使用发布包可跳过）

### 4.1 前提

- Go >= 1.21
- 代码目录：`uptime-monitor/`

### 4.2 构建 sqlplus 静态驱动（推荐）

```bash
cd uptime-monitor
chmod +x build/build.sh
bash build/build.sh
# 生成: dist/uptime-monitor-1.0.0-linux-amd64.tar.gz
```

### 4.3 构建 godror 原生驱动

需先安装 Oracle Instant Client 开发库并设置 `ORACLE_HOME`：

```bash
export ORACLE_HOME=/usr/lib/oracle/21/client64
export CGO_CFLAGS="-I$ORACLE_HOME/sdk/include"
export CGO_LDFLAGS="-L$ORACLE_HOME/lib"
cd uptime-monitor
DRIVER=godror bash build/build.sh
```

### 4.4 多架构一键构建

```bash
bash build/build-all.sh                 # amd64 + arm64
bash build/build-all.sh --arch=amd64    # 仅 amd64
```

---

## 5. 部署步骤

### 5.1 上传并解压

```bash
# 上传发布包到目标服务器后执行
tar -xzf uptime-monitor-1.0.0-linux-amd64.tar.gz -C /opt/
cd /opt/uptime-monitor-1.0.0-linux-amd64
```

目录结构：

```
uptime-monitor-1.0.0-linux-amd64/
├── bin/uptime-monitor          # 可执行文件
├── conf/config.yaml.example    # 配置示例
├── docs/                       # 文档
└── uptime-monitor.service      # systemd 单元模板
```

### 5.2 配置

```bash
cp conf/config.yaml.example conf/config.yaml
vi conf/config.yaml
```

关键配置项：

```yaml
monitor:
  interval_seconds: 60
  driver: "sqlplus"        # sqlplus 或 godror
db:
  host: "127.0.0.1"
  port: 1521
  user: "system"
  password: ""             # 建议用环境变量注入
  service: "orclpdb1"
  sqlplus_path: "sqlplus"
  oracle_home: "/opt/oracle/instantclient_21_12"   # 必配! 见下方说明
push:
  url: "http://127.0.0.1:3001/api/push/YOUR_PUSH_TOKEN"
```

> **`oracle_home` 必配**：sqlplus 驱动若未设置 ORACLE_HOME，会报 `SP2-0667: Message file sp1<lang>.msb not found`。填入 Instant Client 目录后，程序自动注入 `ORACLE_HOME`/`TNS_ADMIN`/`NLS_LANG` 并把 `bin`、`lib` 加入 PATH/LD_LIBRARY_PATH，不再依赖 shell 环境。
> 也可用环境变量覆盖：`export UM_DB_ORACLE_HOME=/opt/oracle/instantclient_21_12`
>
> 密码可通过环境变量注入，避免明文写入文件：
> `export UM_DB_PASSWORD='你的密码'`

### 5.3 启动前验证

```bash
# 首次手动运行, 观察日志确认能连库并上报
UM_DB_PASSWORD='密码' ./bin/uptime-monitor -c conf/config.yaml
```

看到如下日志即为正常：

```
[INFO] Uptime-Monitor 启动: driver=sqlplus host=127.0.0.1 ...
[INFO] 数据库正常: 查询正常... (耗时 xx ms)
[INFO] 上报成功: status=up
```

按 `Ctrl+C` 停止，确认优雅退出。

### 5.4 配置 systemd 服务（推荐）

```bash
# 编辑服务单元, 设置环境变量
sudo tee /etc/systemd/system/uptime-monitor.service > /dev/null <<EOF
[Unit]
Description=Uptime-Monitor (Oracle health check)
After=network.target

[Service]
Type=simple
ExecStart=/opt/uptime-monitor-1.0.0-linux-amd64/bin/uptime-monitor -c /opt/uptime-monitor-1.0.0-linux-amd64/conf/config.yaml
Restart=always
RestartSec=10
Environment=UM_DB_PASSWORD=            # 填入数据库密码, 或留空由 conf/config.yaml 提供
Environment=UM_PUSH_URL=               # 可选, 留空则不覆盖 config.yaml 中的 push.url

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable uptime-monitor
sudo systemctl start uptime-monitor
```

状态与日志：

```bash
systemctl status uptime-monitor
journalctl -u uptime-monitor -f
```

### 5.5 godror 驱动模式附加配置

若使用 godror 驱动，需指定 Oracle 动态库路径：

```bash
export LD_LIBRARY_PATH=/usr/lib/oracle/21/client64/lib:$LD_LIBRARY_PATH
```

并在 `config.yaml` 中：

```yaml
db:
  instant_client_lib: "/usr/lib/oracle/21/client64/lib"
monitor:
  driver: "godror"
```

---

## 6. Uptime Kuma 侧配置

1. 登录 Uptime Kuma → 新增监控
2. 监控类型选择「Push / 推送」
3. 设置心跳间隔 60 秒、状态有效期 10-20 分钟
4. 保存后复制生成的 Push 地址（含 token），填入 `config.yaml` 的 `push.url`
5. 配置通知方式（邮件/钉钉/企业微信）并绑定该监控

---

## 7. 验证

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 手动运行程序 | 日志显示数据库正常、上报成功 |
| 2 | 查看 Uptime Kuma | Push 监控为「正常/绿色」 |
| 3 | 临时停止 Oracle 服务 | 下次检查上报 down，监控变「宕机/红色」并触发通知 |
| 4 | 恢复 Oracle 服务 | 下次检查恢复 up，监控回「正常」 |

---

## 8. 回滚与卸载

```bash
sudo systemctl stop uptime-monitor
sudo systemctl disable uptime-monitor
sudo rm -f /etc/systemd/system/uptime-monitor.service
sudo systemctl daemon-reload
rm -rf /opt/uptime-monitor-1.0.0-linux-amd64
```

---

## 9. 常见问题

| 问题 | 排查方法 |
|------|----------|
| 启动报"配置加载失败" | 检查 `conf/config.yaml` 字段是否完整合法 |
| 报 SP2-0667 Message file not found | `db.oracle_home` 未配置或路径错误，指向 Instant Client 实际安装目录 |
| sqlplus 驱动找不到 sqlplus | 安装 Instant Client，或配置 `db.sqlplus_path` 为绝对路径 |
| godror 驱动连接失败 | 确认 `LD_LIBRARY_PATH` 指向 Instant Client 库 |
| 一直上报 down | 检查网络连通、防火墙放行 1521：`firewall-cmd --add-port=1521/tcp` |
| 上报返回 HTTP 404 | Push 监控已删除或 token 不对：登录 Uptime Kuma 重新创建「Push」监控，把新地址更新到 `push.url` |
| 未到周期不检查 | 确认 `interval_seconds` 与 Uptime Kuma 心跳一致 |

---

## 10. 发布包清单

`uptime-monitor-1.0.0-linux-amd64.tar.gz` 包含：

```
├── bin/uptime-monitor
├── conf/config.yaml.example
├── docs/部署文档.md
├── docs/需求文档.md
└── uptime-monitor.service
```
