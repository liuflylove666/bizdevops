# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project overview

Enterprise DevOps platform: Go 1.25 backend (Gin + GORM + MySQL + Redis) + Vue 3 frontend (Ant Design Vue + Element Plus + Vite). Provides Kubernetes management, CI/CD pipelines (with ArgoCD GitOps), observability/alerting, AI copilot, RBAC, cost & security, and database change management.

Module path: `devops` (single Go module at repo root). Frontend lives in `web/` as a separate npm project. The two are deployed together via a single Docker image (Nginx + supervisord) — see `deploy/`.

## Common commands

### Build & run (preferred path: docker compose)

The user has no local Go toolchain. Do **not** run `go build` / `go run` locally — use docker compose for any build/start.

```bash
# From repo root — uses docker-compose.yml which `include`s deploy/docker-compose.yaml
docker compose up -d --build              # build image + start mysql + redis + devops
docker compose logs -f devops             # tail backend/frontend logs
docker compose restart devops             # restart app only
docker compose down                       # stop (keep volumes)
docker compose down -v                    # stop + drop data volumes (destructive)

# Wipe persisted data and re-init MySQL with init_tables.sql (0→1 reset)
rm -rf deploy/MySqlData deploy/redisData deploy/DevOpsData
mkdir -p deploy/MySqlData deploy/redisData deploy/DevOpsData
docker compose up -d --build
# Rebuild MySQL only:
sh deploy/reinit-mysql-data.sh
```

After startup: frontend at `http://localhost`, swagger at `http://localhost/swagger/index.html`, default login `admin` / `admin123`.

### Backend tests / lint

```bash
go test ./...                             # full test suite (run inside container or CI)
go test ./pkg/validator/...               # single package
go test -run TestXxx ./internal/modules/...   # single test
go vet ./...
```

Test files exist sparsely (mostly `pkg/` and a few handlers in `internal/modules/.../handler`). Most modules have no Go tests — use `go vet` and Swagger sanity-check after handler changes.

### Frontend

```bash
cd web
npm install
npm run dev                               # vite dev server on :3000, proxies /app/api → :8080
npm run build                             # production build (no type-check)
npm run build:check                       # vue-tsc + vite build (run before shipping FE)
```

Override backend during dev: `echo "VITE_DEV_PROXY_TARGET=http://localhost:8090" > web/.env`.

### Database

`migrations/init_tables.sql` is the **full** schema (113+ tables, idempotent on a fresh DB). For existing DBs, run `migrations/upgrades.sql` and any newer `patch_NNN_*.sql` in numeric order. See `migrations/README.md`. Backend also runs `database.AutoMigrate(db)` for a small allow-list of GORM-managed models on startup (see `cmd/server/main.go:89` and `internal/infrastructure/database/database.go`) — most tables are NOT in AutoMigrate; relying on raw SQL migrations is the norm.

## Architecture

### Backend wiring (the IOC + init() pattern is unusual and load-bearing)

`cmd/server/main.go` underscore-imports every `internal/modules/*/handler` package. Each handler package's `init()` calls `ioc.Api.RegisterContainer("XxxHandler", &XxxApiHandler{})`. After config + DB + Redis are ready, `main` calls `ioc.Api.Init()`, which iterates the registered objects and runs each one's `Init()` — and that's where the actual handler is constructed and routes are attached to `cfg.Application.GinRootRouter()`.

Implications when adding a new module:
1. Add `_ "devops/internal/modules/<name>/handler"` to `cmd/server/main.go` imports.
2. In the handler package, register an `Object` via `ioc.Api.RegisterContainer` in `init()`.
3. Build dependencies (repos, services) and register routes inside that object's `Init()`. Do NOT register routes at package init time — DB/config aren't ready yet.

Cross-module DB/Redis access goes through `internal/repository.GetDB(ctx)` / `GetRedis()` (set once from `main`). `cfg.GetDB()` / `cfg.GetRedis()` are kept for backward compat.

### Layered layout

- `cmd/server/` — entrypoint, swagger annotations on `main`.
- `internal/config/config.go` — single `Config` struct; loads `.env` by walking up from cwd; holds Gin app, DB, Redis handles. `cfg.Application.GinRootRouter()` is the `/app/api/v1` group every handler attaches under.
- `internal/modules/<name>/handler/` — HTTP handlers + per-module `*_ioc.go` registration files.
- `internal/modules/<name>/repository/` — module-local GORM repos (some modules have none and reuse shared repos).
- `internal/repository/` — shared repos + `base.go` (global DB/Redis setters), `scoped_db.go`, `compat.go`.
- `internal/service/<domain>/` — business logic, called from handlers. ~40 sub-packages (auth, deploy, release, kubernetes, pipeline, notification, database, jira, nacos, argocd, …).
- `internal/domain/` — newer DDD-style domain models (currently `database/`, `notification/`).
- `internal/models/` — GORM models split by domain (`application/`, `deploy/`, `infrastructure/`, `monitoring/`, `notification/`, `system/`, `pipeline/`, `biz/`, `artifact/`, `ai/`). `models.go` re-exports type aliases for backwards compatibility.
- `internal/infrastructure/` — `database/` (MySQL init, AutoMigrate), `cache/` (Redis init).
- `pkg/` — `ioc`, `middleware` (auth/audit/permission/recovery/tracing/page), `logger`, `response`, `errors`, `dto`, `validator`, `httpclient`, `excel`, `llm`, `utils`.

### Routes & auth

All API routes live under `/app/api/v1` (the BasePath in swagger). `pkg/middleware.InitAuth(jwtSecret)` is initialized in `main` before any handler `Init()`. Permission checks go through `pkg/middleware/permission.go` (RBAC roles: `super_admin`, `admin`, plus regular). There is **no refresh token** on the backend — frontend assumes single-token JWT.

### Frontend

- Vite alias `@` → `web/src`. Dev proxy `/app/api` → backend.
- Pinia stores in `web/src/stores/` (`user`, `permission`, `theme`, `favorite`, `ai`).
- **Dual UI stack on purpose**: Ant Design Vue is primary; Element Plus also imported. When editing a view, match the surrounding library — don't migrate.
- Single shared axios instance handles auth headers + 401 redirect; do not create per-feature instances.
- View directory mirrors backend modules: `views/{application,pipeline,k8s,deploy,release,approval,database,observability,...}`.

### Architecture decisions (`docs/adr/`)

Treat ADRs as load-bearing — several recent ones invalidate older code paths still visible in history:
- **ADR-0001** GitOps (ArgoCD) is the sole CD engine.
- **ADR-0002** `Release` is the aggregate root for the deploy domain (subsumed `DeployRecord` / `EnvPromotionRecord` / `NacosRelease` semantics).
- **ADR-0003** Argo Rollouts replaces self-built canary/blue-green. The self-built progressive delivery code is **removed**; only `Release.RolloutStrategy` enum remains.
- **ADR-0005** Notification channels unified (WeCom / generic webhook / Slack / Feishu / DingTalk **removed**; Telegram is the surviving channel). Don't reintroduce removed channels.
- **ADR-0006** Global Copilot lives in the chrome, not as a left-nav menu item.
- **ADR-0007** Jira is the authoritative planning source.

## Conventions and gotchas

- **Encryption**: for credential-style secrets, use the package-level AES-GCM key pattern from `credential_service.go`. Do not introduce or use a generic `EncryptionService` abstraction.
- **Pipeline notifications**: there is no `pipeline_notify_configs` table on the backend. Notification config is delivered via the unified 4-tab notification center, not a dedicated pipeline table.
- **Database module** (`internal/service/database/`, `views/database/`):
  - Phases 1–4 (ACL / execution records / tickets / rollback) shipped and running.
  - Phase 5 Yearning SQL engine — **code-ready but runtime-disabled**: `engine_rpc_auditor.go` adapter is wired against the Yearning `Engine.Check` RPC, but `instance_handler.go:62` only enables it when `YEARNING_ENGINE_RPC` env is set. The variable is unset and no Yearning service is in `docker-compose.yml`, so prod falls back to `BuiltinAuditor` (~12 hardcoded rules in `auditor.go`). To switch on: run a Yearning engine, set the env, restart `devops`.
  - Frontend has 8 pages under `web/src/views/database/` and 8 routes in `router/index.ts`, but **v2 menu (`config/menu.v2.ts:122`) only exposes `/database/instances`** — `console`, `tickets`, `rules`, `statements`, `logs` are routed but not surfaced in main nav. The old v1 menu in `MainLayout.vue` listed all 6 (kept as historical reference, not rendered).
  - AI SQL Phase still pending.
- When assessing a feature's coverage, **do not** judge by Go service-reference count alone. Open the actual Vue view and confirm the UI surface matches before reporting a gap.
- Charsets: keep all SQL files and source files UTF-8. MySQL is configured for `utf8mb4` / `utf8mb4_unicode_ci` — DSN builders rely on this (see `internal/config/config.go` `DSN()`).
- K8s API rewrite: `K8S_API_SERVER_HOST_REWRITE` rewrites `127.0.0.1` in kubeconfig to a host (defaults to `host.docker.internal` in compose). Do not set this when running the backend directly on the host.

## Reference directories worth knowing

- `migrations/README.md` — exact ordering of init vs upgrade vs patch scripts.
- `internal/modules/README.md` — older module overview (some counts are stale; trust the directory).
- `docs/adr/` and `docs/roadmap/` — design intent and v2.x progress notes.
- `deploy/` — Dockerfile (multi-stage), supervisord, nginx, kind setup scripts.
