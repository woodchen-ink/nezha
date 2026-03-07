# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Nezha Monitoring — a self-hosted server and website monitoring tool. Go backend with embedded frontend static files, gRPC agent communication, SQLite storage, and VictoriaMetrics for time series data.

## Build & Development Commands

```bash
# Prerequisites: install swag, protoc, protoc-gen-go, protoc-gen-go-grpc, yq

# Generate swagger docs and protobuf code
swag init --pd -d . -g ./cmd/dashboard/main.go -o ./cmd/dashboard/docs --requiredByDefault
protoc --go-grpc_out="require_unimplemented_servers=false:." --go_out="." proto/*.proto

# Frontend placeholder files (needed for build if not fetching real frontends)
touch ./cmd/dashboard/user-dist/a
touch ./cmd/dashboard/admin-dist/a

# Download frontend distributions (requires yq)
script/fetch-frontends.sh

# Run tests
go test -v ./...

# Build
go build -v ./cmd/dashboard

# Cross-platform build (devcontainer)
goreleaser build --snapshot --clean
```

## Architecture

### Entry Point & Server Startup

`cmd/dashboard/main.go` — single binary serving both HTTP (Gin) and gRPC on the same port via protocol detection (`newHTTPandGRPCMux`). CLI flags: `-c` config path (default `data/config.yaml`), `-db` SQLite path (default `data/sqlite.db`), `-v` version.

Initialization order: frontend templates → config → timezone/cache → memory limit → SQLite (GORM auto-migrate) → TSDB → singleton services → cron jobs.

### Layer Structure

- **`cmd/dashboard/controller/`** — HTTP handlers (Gin). Uses generic handler wrappers: `commonHandler` (authenticated), `adminHandler` (admin-only), `listHandler` (with filtering/permission), `pCommonHandler` (paginated). All routes defined in `controller.go:routers()`.
- **`cmd/dashboard/rpc/`** — gRPC server for agent communication. Agents report system state/info via streaming RPCs. Also handles task dispatch and web terminal IO streaming.
- **`service/singleton/`** — Singleton services holding in-memory state. Global variables (`DB`, `Cache`, `ServerShared`, `CronShared`, etc.) initialized in `LoadSingleton()`. The generic `class[K,V]` type provides thread-safe map + sorted list with permission checking.
- **`model/`** — GORM models and API request/response DTOs. Models implement `CommonInterface` for permission filtering.
- **`pkg/`** — Utility packages: `ddns/` (DNS providers), `geoip/`, `grpcx/`, `i18n/`, `tsdb/` (VictoriaMetrics wrapper), `utils/`, `websocketx/`.
- **`proto/`** — Protobuf definitions for agent-dashboard gRPC communication.

### Key Services (singleton package)

- **AlertSentinel** (`alertsentinel.go`) — evaluates alert rules against server state, triggers notifications
- **ServiceSentinel** (`servicesentinel.go`) — HTTP/TCP/ICMP probe scheduling and result tracking
- **CronClass** (`crontask.go`) — distributed cron task scheduling and dispatch to agents
- **ServerClass** (`server.go`) — in-memory server registry with online/offline tracking
- **NATClass** (`nat.go`) — NAT/port forwarding with domain-based routing

### Frontend Integration

Frontends are separate repos, built as static files, and embedded via `//go:embed *-dist`. Template versions defined in `service/singleton/frontend-templates.yaml`. Admin panel at `/dashboard/*`, user panel at `/*`.

### Database

- **SQLite** via GORM with auto-migration. Tables: servers, users, server_groups, services, alert_rules, notifications, notification_groups, crons, transfers, nats, ddns_profiles, wafs, oauth2_binds.
- **VictoriaMetrics** (embedded) for time series metrics. Wrapper in `pkg/tsdb/`.

### Authentication

JWT (cookie-based, cookie name from config) with optional OAuth2 (GitHub, GitLab, Gitee, Gitea). WAF middleware in `controller/waf/`. IP validation on tokens.

### API

REST at `/api/v1/*`. Response format: `{ success: bool, data: T, error: string }`. Swagger docs auto-generated, available at `/swagger/` in debug mode.

## Key Patterns

- Handler functions return `(T, error)` — framework wraps into JSON response automatically
- `gormError` and `wsError` custom types trigger different response handling in the generic `handle()` function
- Permission filtering: models implement `HasPermission(*gin.Context) bool`, checked via `listHandler` and `class.CheckPermission`
- Config loaded via Koanf from YAML + environment variables
- i18n via gotext (`singleton.Localizer`)
- Comments and variable names are primarily in Chinese

## CI

GitHub Actions (`test.yml`): runs `go test ./...` and build on Ubuntu/Windows/macOS. Gosec security scanner on Linux (excludes: G104, G115, G117, G203, G402, G703, G704).
