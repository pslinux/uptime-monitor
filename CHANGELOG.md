# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-08-18

### Added
- Initial open-source release.
- Periodic Oracle health checks with configurable interval and custom SQL (`SELECT 1 FROM DUAL` by default).
- Report `up` / `down` + latency to Uptime Kuma Push monitors.
- Two database drivers:
  - `sqlplus`: shell-out driver, supports fully static (CGO-free) cross-compiled binaries.
  - `godror`: native Oracle driver via `database/sql` (CGO + Oracle Instant Client required).
- Connection / query timeouts and retry logic for transient failures.
- YAML configuration with `UM_*` environment-variable overrides; secrets not hard-coded.
- File logging with size-based rotation; graceful shutdown on SIGINT/SIGTERM.
- Build scripts for single-arch and multi-arch (`amd64`, `arm64`, `arm`) packaging.
- systemd unit template and Chinese deployment documentation.
- Unit tests for core parsing and reporting modules.
