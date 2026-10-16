# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Go backend for PharmaChat, an AI sales and support chatbot for a Vietnamese pharma company (Huyen Song Khoe). Customers reach it through Facebook Messenger and Instagram DM. The frontend lives in a separate repo. It replaces an earlier .NET implementation.

Business docs and task tracking live on Atlassian (`pharmachat.atlassian.net`), not in this repo; read them through the `atlassian` MCP server:
- Confluence space `SD`, under the page "PharmaChat — Tài liệu dự án": CONTEXT, the constitution, SPEC-01..07, the Go migration plan (which records business decisions B-1..B-6) and the old .NET design document.
- Jira project `PHA`: one epic per phase (PHA-4..PHA-13) and one issue per original task, titled `GO-xx · ...`. Branches, commits and PRs reference the `PHA-n` key. Jira tickets are written in Vietnamese.

`.sdd/AGENTS.md` stays in the repo as a short context file for AI agents.

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

## Shared Claude Code setup (`.claude/`)

- `rules/`: `workflow.md` always applies; `go-code.md` loads for `*.go`; `migrations.md` loads for `migrations/` and `internal/store/queries/`. Coding conventions live there, not here.
- `skills/`: `/start-task PHA-n` (or a legacy `GO-xx`), `/new-migration`, `/verify-stack`.
- `settings.json`: allowlists routine make/go/git/gh commands. It denies reading `deploy/.env` and editing generated `internal/store/*.go`. Hooks run `gofmt` after Go edits and block force-pushes and pushes to `master` (`hooks/guard-push.sh`), standing in for branch protection, which GitHub Free lacks on private repos.
- Personal overrides go in `.claude/settings.local.json` (gitignored).
- `.mcp.json` registers the Atlassian Rovo MCP server (`https://mcp.atlassian.com/v2/mcp`) for Jira and Confluence, and `settings.json` pre-approves it. Each person signs in with their own Atlassian account by running `/mcp` once.
