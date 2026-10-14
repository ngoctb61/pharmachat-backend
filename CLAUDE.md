# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Go backend for PharmaChat, an AI sales and support chatbot for a Vietnamese pharma company (Huyen Song Khoe). Customers reach it through Facebook Messenger and Instagram DM. The frontend lives in a separate repo. It replaces an earlier .NET implementation; `docs/MIGRATION_GO_BACKEND.md` is the implementation plan (tasks GO-00..GO-47) and records the business decisions B-1..B-6. Specs are in `.sdd/` (Vietnamese without diacritics); task status is in `.sdd/TASKS.md`.

## Commands

```bash
make build                 # go build ./...
make test-unit             # go test -race -short ./...  (no Docker)
make test                  # includes integration tests; testcontainers starts pgvector/pgvector:pg17
make lint                  # golangci-lint v2.14.0 (local binary or Docker image)
make gen                   # sqlc generate (sqlc is a go.mod tool: `go tool sqlc`)
make gen-check             # sqlc diff, as CI runs it
go test -run TestRLS_ -v ./internal/platform/db/   # one test or group
make up / make down        # docker compose -f deploy/docker-compose.yml (needs deploy/.env)
make seed; make create-admin EMAIL=... NAME=...
```

CI (`.github/workflows/ci.yml`) runs lint, `sqlc diff`, `go mod tidy -diff`, the full test suite and Docker builds for all three binaries.

## Architecture

- `cmd/api`: public HTTP (chi). `cmd/agent`: gRPC server for the AI pipeline, plus HTTP `/health`. `cmd/pcctl`: operator CLI (migrate, seed, create-admin). Each service binary also answers `<binary> healthcheck`, which the distroless Docker healthchecks use.
- `internal/platform/*` is shared infrastructure (config from env only, slog with PII redaction, the HTTP error envelope, the DB pool and tenant scoping, AES-GCM, migrations). Business packages go in `internal/<domain>` and must not import each other.
- `migrations/` (goose, embedded in binaries) is the schema source of truth. `internal/store` is sqlc output generated from `migrations/` + `internal/store/queries/*.sql`; never edit it by hand.
- `pcctl migrate up` applies goose migrations, then River's job tables, then grants River tables to `pharmachat_app`.

### Tenant isolation (the part that spans several files)

Two Postgres roles matter. The **schema owner** (`DATABASE_ADMIN_URL`, used only by pcctl and tests) bypasses RLS. Services log in as `pharmachat_api`, a member of **`pharmachat_app`**, which is subject to RLS. Every table with a `tenant_id` column has a `tenant_isolation` policy on `app_current_tenant()`, which reads the transaction-local setting `app.tenant_id`.

- Tenant-scoped queries must run inside `db.WithTenant(ctx, pool, tenantID, fn)` (`internal/platform/db`). Outside it the app role sees zero rows, by design.
- A new table with `tenant_id` must be added to the RLS loop in its migration. `TestRLS_EveryTenantTableHasRowLevelSecurity` fails otherwise.
- `messages`, `agent_traces` and `compliance_logs` are append-only for the app role (no UPDATE/DELETE; `messages` allows UPDATE only on `external_message_id`, `metadata_json`). `permissions` is read-only.
- The login role is created by `deploy/postgres/10-app-role.sql`, which runs only on first volume init. Tests create it in `internal/platform/dbtest`.

### Conventions specific to this repo

- Client errors use `httpx.Error` and are written with `httpx.WriteError` as `{errorCode, message, requestId}`. User-facing messages are Vietnamese without diacritics. Any other error becomes a 500 `INTERNAL`, with the cause logged server-side only.
- Never put phone numbers, emails, message text or secrets in log attributes. `logx` redacts known keys and scrubs emails and phone numbers, but don't rely on it.
- Embeddings are `halfvec(3072)`, because pgvector cannot HNSW-index `vector` above 2000 dims. `embedding_configs.dimension` is pinned to 3072 by a CHECK constraint.
- LLM model and prices come from `llm_configs` (seeded as `claude-sonnet-4-6`, $3/$15 per MTok). Don't hardcode them.
- Integration tests use `dbtest.Run` in `TestMain` and call `dbtest.Require(t, env)` so that `-short` skips them.
- Default branch is `master`. Commits use conventional format and reference a SPEC or GO-xx task.
