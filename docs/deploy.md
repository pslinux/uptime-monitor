# Uptime-Monitor 部署文档

> 版本：v1.0.0 ｜ 适用平台：Linux（amd64/arm64/arm）｜ 更新日期：2026-08-18
>
> 配套：README.md ｜ 需求文档 requirement.md ｜ 变更记录 CHANGELOG.md

---

## 1. 概述

Uptime-Monitor 是使用 Go 编写的 Oracle 数据库健康监控程序。它周期连接 Oracle 执行 `SELECT 1 FROM DUAL`，将结果上报到 Uptime Kuma 的 Push 接口（`up` / `down` + 延迟），由 Uptime Kuma 统一展示与告警。

本文档说明从构建、配置到部署、验证、告警的完整流程。

---

## 2. 部署架构

```
┌────────────┐   每60秒检查   ┌────────────────────┐   GET上报   ┌────────────────┐
│ 监控服务器  │ ────────────▶ │  Uptime-Monitor    │ ──────────▶ │  Uptime Kuma    │
│  (Oracle)  │ ◀──────────── │  (Go 二进制)        │  up/down    │  (Docker :7001) │
└────────────┘   返回结果      └────────────────────┘   │push     └────────┬───────┘
                                                       │                  │ 告警触发
                                        超过"状态有效期"未上报自动判宕机     ▼
                                                                 ┌────────────────┐
                                                                 │ SMTP 邮箱 /      │
                                                                 │ 企业微信机器人   │
                                                                 └────────────────┘
```

监控范围：

- **数据库**：Oracle 实例可用性（通过探针 + Push 类型监控）
- **应用**：Web URL、业务端口、主机存活（Uptime Kuma 直接 HTTP / TCP / Ping 探测）

---

## 3. 环境要求

| 组件 | 要求 |
|------|------|
| 操作系统 | Linux（openEuler/CentOS/Ubuntu 均可） |
| 运行环境 | 无（Go 静态二进制，不依赖解释器） |
| sqlplus 驱动 | 需安装 Oracle Instant Client + sqlplus |
| godror 驱动 | 需 CGO 编译 + 运行机 Oracle Instant Client 动态库 |
| Uptime Kuma | 已部署（见第 6 章），且已创建 Push 监控类型 |

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
bash build/build-all.sh                 # amd64 + arm64 + arm
bash build/build-all.sh --arch=amd64    # 仅 amd64
```

### 4.5 查看版本

```bash
./bin/uptime-monitor -v     # 打印构建时注入的版本号
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
├── docs/                       # 部署/需求文档
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

## 6. Uptime Kuma 部署

### 6.1 Docker 一键部署（推荐）

```bash
# 端口映射 7001(宿主机):3001(容器), 数据持久化到宿主机 /opt/uptime-kuma/data
docker run -d --name uptime-kuma --restart=always \
  -p 7001:3001 \
  -v /opt/uptime-kuma/data:/app/data \
  louislam/uptime-kuma:1

# 确认运行状态
docker ps | grep uptime-kuma
```

### 6.2 首次初始化

1. 浏览器访问 `http://127.0.0.1:7001/`
2. 创建管理员账号（第一个账号即管理员，负责配置监控与通知）
3. 数据自动保存于 SQLite：`/opt/uptime-kuma/data/kuma.db`

### 6.3 常用运维命令

| 操作 | 命令 |
| ---- | ---- |
| 查看日志 | `docker logs -f uptime-kuma` |
| 重启 | `docker restart uptime-kuma` |
| 停止 | `docker stop uptime-kuma` |
| 升级 | `docker pull louislam/uptime-kuma:1` 后按 6.4 重建 |
| 备份 | 直接拷贝 `/opt/uptime-kuma/data/` 目录（含 kuma.db） |
| 卸载 | `docker rm -f uptime-kuma` |

### 6.4 平滑升级（保留数据）

```bash
docker pull louislam/uptime-kuma:1
docker stop uptime-kuma
docker rm uptime-kuma
docker run -d --name uptime-kuma --restart=always \
  -p 7001:3001 \
  -v /opt/uptime-kuma/data:/app/data \
  louislam/uptime-kuma:1
```

---

## 7. Uptime Kuma 侧配置

### 7.1 数据库监控（Push 类型）

1. 登录 Uptime Kuma → 添加新监控
2. 监控类型选择「Push / 推送」
3. 设置心跳间隔 60 秒、状态有效期 10-20 分钟
4. 保存后复制生成的 Push 地址（含 token），填入 `config.yaml` 的 `push.url`
5. 探针按间隔自动上报；Kuma 超过"状态有效期"未收到上报即判定宕机

> ⚠️ Push URL 只保留 `.../api/push/<token>`，不要拼接 `?status=up&msg=...` 查询串，否则上报可能 404。

### 7.2 应用监控（HTTP / TCP / Ping）

| 监控类型 | 适用场景 |
| -------- | -------- |
| HTTP(s) | Web 应用、URL 可达性、状态码 |
| TCP 端口 | 应用服务端口连通性 |
| Ping | 主机存活 |

直接添加监控并填入目标地址即可，无需额外探针。

### 7.3 告警通知

> 通知配置仅 **管理员账号** 可见；企业微信 Webhook 需管理员权限，无权限时使用 SMTP 邮箱方式。

#### 7.3.1 邮件通知（SMTP，企业微信邮箱）

1. 企业邮箱网页端 → 设置 → 收发信设置 → 开启 **SMTP 服务**
2. 设置 → 安全设置 → 生成 **客户端专用密码**（授权码，仅显示一次）
3. Uptime Kuma → 设置 → 通知 → 添加通知 → **Email (SMTP)**

| 字段 | 值 |
| ---- | ---- |
| Hostname | `smtp.exmail.qq.com` |
| Port | `465` |
| Security | `SSL/TLS` |
| Username | 企业邮箱完整地址 |
| Password | 客户端专用密码（非登录密码） |
| From Email | 发信邮箱地址 |
| To Email | 收件人邮箱（可多个，逗号分隔） |

> 若 465 被网络拦截，可改用 `587` + `STARTTLS`；填写后先点 **Test** 验证发信。

#### 7.3.2 企业微信 Webhook（备选）

```bash
# Webhook 地址格式
https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY
```

- Uptime Kuma 新版通知选 **企业微信 / WeCom**，填入 URL 即可
- 消息支持占位符：`{{name}}` `{{status}}` `{{msg}}` `{{time}}`
- 群机器人限 20 条/分钟

#### 7.3.3 绑定监控

在监控编辑页勾选已创建的通知；异常（down）与恢复（up）时自动推送。

---

## 8. 验证

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 手动运行程序 | 日志显示数据库正常、上报成功 |
| 2 | 查看 Uptime Kuma | Push 监控为「正常/绿色」 |
| 3 | 临时停止 Oracle 服务 | 下次检查上报 down，监控变「宕机/红色」并触发通知 |
| 4 | 恢复 Oracle 服务 | 下次检查恢复 up，监控回「正常」 |

---

## 9. 回滚与卸载

```bash
sudo systemctl stop uptime-monitor
sudo systemctl disable uptime-monitor
sudo rm -f /etc/systemd/system/uptime-monitor.service
sudo systemctl daemon-reload
rm -rf /opt/uptime-monitor-1.0.0-linux-amd64
```

---

## 10. 常见问题

| 问题 | 排查方法 |
|------|----------|
| 启动报"配置加载失败" | 检查 `conf/config.yaml` 字段是否完整合法 |
| 报 SP2-0667 Message file not found | `db.oracle_home` 未配置或路径错误，指向 Instant Client 实际安装目录 |
| sqlplus 驱动找不到 sqlplus | 安装 Instant Client，或配置 `db.sqlplus_path` 为绝对路径 |
| 报 ORA-01017 用户名/密码错误 | 核对 `conf/config.yaml` 与 systemd `UM_DB_PASSWORD` 两处是否一致 |
| godror 驱动连接失败 | 确认 `LD_LIBRARY_PATH` 指向 Instant Client 库 |
| 一直上报 down | 检查网络连通、防火墙放行 1521：`firewall-cmd --add-port=1521/tcp` |
| 上报返回 HTTP 404 | Push URL 带查询串或 token 不对：只保留 `.../api/push/<token>`；如已删除监控请重新创建 |
| 页面显示 No heartbeat in the time window | 心跳间隔与状态有效期不匹配：间隔设为探针上报周期，有效期大于间隔 |
| 未到周期不检查 | 确认 `interval_seconds` 与 Uptime Kuma 心跳一致 |
| 收不到告警邮件 | 检查 SMTP 授权码、端口 465/587 放行、监控是否勾选了通知 |

---

## 11. 发布包清单

`uptime-monitor-1.0.0-linux-amd64.tar.gz` 包含：

```
├── bin/uptime-monitor
├── conf/config.yaml.example
├── docs/部署文档.md
├── docs/需求文档.md
└── uptime-monitor.service
```

发布渠道：GitHub Releases（`v*` tag 自动构建 amd64/arm64/arm 三平台包）。
