# PharmaChat — Project Constitution

> Trang thai: DRAFT (v0.2, ban Go) — ratify trong team review truoc khi merge master.
> 1 khi ratified, file nay **immutable** tru khi co RFC duoc approve.
> Path: .sdd/constitution.md
> v0.2 (2026-10-15): chuyen tu .NET sang Go theo docs/MIGRATION_GO_BACKEND.md.

---

## ARTICLE 1 — TECH STACK (immutable once ratified)

- **Backend**: Go (ban stable ghi trong `go.mod`). 2 service: `cmd/api` (HTTP, WebSocket, webhook) va `cmd/agent` (gRPC), cung CLI van hanh `cmd/pcctl`.
- **HTTP**: `net/http` + `go-chi/chi/v5`. **gRPC**: `google.golang.org/grpc`, proto trong `proto/`.
- **Database**: **PostgreSQL 17 + pgvector**. Migration goose trong `migrations/` la source of truth. Truy van viet tay, sinh code bang `sqlc` — khong dung ORM, khong tu sinh schema.
- **Vector store**: pgvector, cot `halfvec(3072)` + index HNSW `halfvec_cosine_ops`. Khong dung Qdrant.
- **Job queue**: River (`riverqueue/river`) tren Postgres. Khong dung Redis.
- **AI**: Anthropic Claude qua `anthropic-sdk-go`; embedding OpenAI `text-embedding-3-large` qua `openai-go`.
- **LLM Config**: Cau hinh qua bang `llm_configs` — **khong hardcode** model hay gia. Mac dinh: `claude-sonnet-4-6` (quyet dinh B-3).
- **Auth**: JWT (`golang-jwt/jwt/v5`) + refresh token xoay vong, mat khau argon2id.
- **Real-time**: WebSocket (`coder/websocket`) + Postgres `LISTEN/NOTIFY`.
- **Observability**: `log/slog` JSON, co buoc loc PII.
- **Deploy**: Docker Compose (1 VPS 8GB RAM), Caddy lam reverse proxy + TLS.

> Them dependency bat buoc can RFC truoc khi implement.

---

## ARTICLE 2 — CODING STANDARDS

- **Layout**: `cmd/<binary>` + `internal/<nghiep vu>` + `internal/platform` (ha tang dung chung). Package nghiep vu khong import package nghiep vu khac; dung chung qua `platform` hoac interface.
- **Naming**: theo Go (MixedCaps, package ngan, chu thuong); snake_case cho bang va cot SQL.
- **Soft delete**: Aggregate root co `deleted_at`. Truy van sqlc mac dinh loc `deleted_at IS NULL`.
- **Multi-tenant**: `tenant_id` tren MOI bang co du lieu tenant, bat buoc co Row-Level Security. Truy van tenant chay trong `db.WithTenant`.
- **Enums**: luu dang `text` + `CHECK (... IN (...))`. Khong magic numbers.
- **Errors**: tra `error`, boc bang `fmt.Errorf("...: %w", err)`; thong bao loi phai neu dung nguyen nhan.
- **File size**: 400 LOC typical, 800 max. Split when split is natural.
- **No magic numbers** ngoai hang so co ten hoac config.
- **Logging**: nhan `*slog.Logger` qua tham so/struct, khong dung logger global.
- **Lint**: `golangci-lint` (cau hinh `.golangci.yml`) phai sach.

---

## ARTICLE 3 — SECURITY POLICIES (non-negotiable)

- **No secrets in source.** Chi doc tu bien moi truong; `deploy/.env.example` chi chua ten bien, khong chua gia tri that.
- **PII protection**: `contacts.phone`, `contacts.email`, noi dung tin nhan KHONG duoc log. Chi log metadata da loc.
- **Conversation purge**: Noi dung goc xoa sau **30 ngay**. Giu ban ghi audit da hash.
- **SQL**: Parameterized only (sqlc / pgx). Khong noi chuoi SQL.
- **HTTPS / TLS 1.3** bat buoc tren moi endpoint public.
- **Webhook HMAC**: Meta webhook phai verify HMAC-SHA256 tren body goc bang `hmac.Equal` truoc khi parse. Fail → 401.
- **JWT**: Song ngan (15 phut), refresh qua cookie httpOnly. Khoa tai khoan sau 5 lan dang nhap sai.
- **Secrets at rest**: API key va token Meta ma hoa AES-256-GCM truoc khi luu DB.
- **Rate limiting**: Theo tenant tren `/api/*`, theo IP tren `/api/webhooks/*`.
- **Stack traces**: Khong tra ve client. Error response dung `{ errorCode, message, requestId }`.

---

## ARTICLE 4 — GIT WORKFLOW

- Nhanh mac dinh: **`master`**. Branch off `master`. Naming: `feat/<scope>`, `fix/<scope>`, `chore/<scope>`.
- Conventional commits: `<type>(<scope>): <summary>` — body giai thich *tai sao*, khong phai *cai gi*.
- Squash on merge. PR phai tham chieu SPEC ID (SPEC-01, UC-A07...) hoac task GO-xx.
- No force-push to `master`.
- Never `--no-verify`.

---

## ARTICLE 5 — TESTING REQUIREMENTS

- **TDD** cho logic nghiep vu — viet test truoc, roi implement.
- **Coverage floor**: 80% tren package nghiep vu trong `internal/`. Bao cao trong CI.
- **Test types**: `go test` (unit, dang bang), testcontainers-go (integration).
- Integration test chay tren Postgres that (`pgvector/pgvector:pg17`) — khong mock DB.
- `go test -short` chi chay unit test; integration test tu skip.
- Moi bug fix PHAI kem regression test.

---

## ARTICLE 6 — AI AGENT RULES

- Agent doc prompt tu bang `kb_versions` — **khong hardcode prompt trong code**.
- Agent trace ghi vao `agent_traces` (append-only; role dich vu khong co quyen UPDATE/DELETE).
- Han muc chi phi LLM: mac dinh $200/thang/tenant (quyet dinh B-4), canh bao o 80%, dung o 100%.
- Model dung cho moi request lay tu `llm_configs` — khong bao gio hardcode.
- Sale co the tiep quan hoi thoai (quyet dinh B-1). Hoi thoai tu chuyen sang sale khi co trieu chung nang, khi khach gui anh, hoac khi cham han muc chi phi.
- Compliance gate: bat buoc disclaimer, chan tu van thuoc ke don, chuyen sale khi trieu chung nang.

---

## ARTICLE 7 — REVIEW PROCESS

- All PRs require:
  1. CI green (lint, test, sqlc diff, go mod tidy, build Docker image).
  2. One human reviewer approval.
  3. Linked SPEC / UC ID hoac task GO-xx.
  4. Updated `docs/` hoac `api/openapi.yaml` neu public contract thay doi.
- Security-sensitive PRs require 2 human approvers.
- Thay doi migration phai review ca `migrations/` VA truy van sqlc lien quan, kem test RLS neu them bang co `tenant_id`.

---

Ratified by: ___________________ (date: ____)
