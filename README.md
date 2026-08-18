# Uptime-Monitor

[![CI](https://github.com/YOUR_GITHUB_USER/uptime-monitor/actions/workflows/ci.yml/badge.svg)](https://github.com/YOUR_GITHUB_USER/uptime-monitor/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A lightweight **Oracle database health-check monitor** written in Go. It periodically connects to an Oracle database, executes a health-check SQL (`SELECT 1 FROM DUAL` by default), and reports `up` / `down` status to [Uptime Kuma](https://github.com/louislam/uptime-kuma) through its Push API — no agent or web UI required on the monitored host.

> Uptime Kuma does not natively support Oracle (TCP port 1521) health checks. This tool fills that gap.

## Features

- Periodically checks Oracle availability with a configurable interval and custom SQL
- Reports results to Uptime Kuma Push monitors (`status=up|down`, latency in `ping`)
- Two database drivers:
  - `sqlplus` — shells out to the Oracle `sqlplus` client; supports **fully static (CGO-free) builds**, simplest to deploy
  - `godror` — native Oracle driver via `database/sql`; requires CGO + Oracle Instant Client
- Connection / query timeouts and built-in retry to tolerate transient failures
- YAML configuration with environment-variable overrides (no secrets hard-coded in files)
- Structured file logging with size-based rotation
- Graceful shutdown on SIGINT / SIGTERM
- Cross-compiles to Linux `amd64` / `arm64` / `arm` as a single static binary
- Ships with a ready-made systemd unit template

## How it works

```
+----------------+   check every N sec   +----------------------+   GET report   +-------------------+
| Oracle DB      | --------------------> |   Uptime-Monitor     | -------------> |   Uptime Kuma     |
| (SQL*Net 1521) | <-------------------- | (single Go binary)   |  up / down     |  (Push monitor)   |
+----------------+    query result       +----------------------+    / ping      +-------------------+
                                                                                    |
                                           no heartbeat within the time window -> mark DOWN
```

## Quick start

### 1. Download

Grab the latest release asset from the [Releases page](https://github.com/YOUR_GITHUB_USER/uptime-monitor/releases) (e.g. `uptime-monitor-1.0.0-linux-amd64.tar.gz`), or build from source (see below).

### 2. Configure

```bash
tar -xzf uptime-monitor-1.0.0-linux-amd64.tar.gz -C /opt/
cd /opt/uptime-monitor-1.0.0-linux-amd64
cp conf/config.yaml.example conf/config.yaml
vi conf/config.yaml
```

```yaml
monitor:
  driver: "sqlplus"        # sqlplus | godror
  interval_seconds: 60
  query: "SELECT 1 FROM DUAL"
db:
  host: "127.0.0.1"
  port: 1521
  user: "system"
  password: ""             # prefer UM_DB_PASSWORD env var
  service: "orclpdb1"
  sqlplus_path: "sqlplus"
  oracle_home: "/opt/oracle/instantclient_21_12"   # REQUIRED for sqlplus (see note)
push:
  url: "http://127.0.0.1:3001/api/push/YOUR_PUSH_TOKEN"
log:
  file: "/var/log/uptime-monitor/uptime-monitor.log"
  level: "info"
  max_size_mb: 10
```

> **Note on `oracle_home`**: with the `sqlplus` driver, if `ORACLE_HOME` is not set you will get `SP2-0667: Message file sp1<lang>.msb not found`. Point `db.oracle_home` to your Instant Client directory; the program then injects `ORACLE_HOME`, `TNS_ADMIN`, `NLS_LANG` and adds `bin` / `lib` to `PATH` / `LD_LIBRARY_PATH` for the child process.

### 3. Create the Uptime Kuma Push monitor

1. Login to Uptime Kuma → **Add New Monitor**
2. Monitor Type: **Push**
3. Set heartbeat interval (e.g. 60 s) and status retention (e.g. 10–20 min)
4. Copy the generated Push URL (with token) into `push.url`

### 4. Run

```bash
# secrets via environment to keep config clean
export UM_DB_PASSWORD='your-password'
./bin/uptime-monitor -c conf/config.yaml
```

Or install as a systemd service (unit template included in the archive):

```bash
sudo cp uptime-monitor.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now uptime-monitor
```

See [docs/deploy.md](docs/deploy.md) (Chinese) for the full deployment guide.

## Configuration reference

All options live in YAML and can be overridden by `UM_*` environment variables (empty values are ignored).

| YAML path | Env var | Default | Description |
|-----------|---------|---------|-------------|
| `monitor.interval_seconds` | `UM_MONITOR_INTERVAL_SECONDS` | `60` | Check interval in seconds |
| `monitor.query` | `UM_MONITOR_QUERY` | `SELECT 1 FROM DUAL` | Health-check SQL |
| `monitor.query_timeout_sec` | `UM_MONITOR_QUERY_TIMEOUT_SEC` | `15` | SQL timeout in seconds |
| `monitor.driver` | `UM_MONITOR_DRIVER` | `sqlplus` | `sqlplus` or `godror` |
| `monitor.retry_times` | `UM_MONITOR_RETRY_TIMES` | `3` | Retry count per check |
| `monitor.retry_interval_ms` | `UM_MONITOR_RETRY_INTERVAL_MS` | `2000` | Retry interval in ms |
| `db.host` | `UM_DB_HOST` | — | Oracle host |
| `db.port` | `UM_DB_PORT` | `1521` | Oracle port |
| `db.user` | `UM_DB_USER` | — | Database user (read-only recommended) |
| `db.password` | `UM_DB_PASSWORD` | — | Database password |
| `db.service` | `UM_DB_SERVICE` | — | Oracle service name (SERVICE_NAME) |
| `db.conn_timeout_sec` | `UM_DB_CONN_TIMEOUT_SEC` | `10` | Connect timeout in seconds |
| `db.sqlplus_path` | `UM_DB_SQLPLUS_PATH` | `sqlplus` | Path to `sqlplus` binary |
| `db.oracle_home` | `UM_DB_ORACLE_HOME` | — | Oracle Instant Client home (sqlplus) |
| `db.instant_client_lib` | `UM_DB_INSTANT_CLIENT_LIB` | — | Instant Client lib dir (godror) |
| `push.url` | `UM_PUSH_URL` | — | Uptime Kuma Push monitor URL |
| `log.file` | `UM_LOG_FILE` | empty (stdout) | Log file path |
| `log.level` | `UM_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `log.max_size_mb` | `UM_LOG_MAX_SIZE_MB` | `10` | Rotation size in MB |

## Build from source

Requirements: Go >= 1.21.

```bash
# default: static linux/amd64 binary with sqlplus driver
bash build/build.sh

# other architectures (multi-arch in one go)
bash build/build-all.sh --arch=amd64,arm64,arm

# godror native driver (needs CGO + Oracle Instant Client SDK)
DRIVER=godror bash build/build.sh

# check version at runtime
./bin/uptime-monitor -v
```

Artifacts are written to `dist/` as `uptime-monitor-<version>-<os>-<arch>.tar.gz`.

## Drivers

| Driver | Pros | Cons | Best for |
|--------|------|------|----------|
| `sqlplus` | Static binary, no CGO, trivial packaging | Needs `sqlplus` on the target host | Quick, wide deployment |
| `godror` | In-process connection, no `sqlplus` | Requires CGO build + Instant Client shared libs | Native integration |

## Documentation

- [Deployment guide (中文)](docs/deploy.md)
- [Requirements / design (中文)](docs/requirement.md)

## Security

Report vulnerabilities via [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
