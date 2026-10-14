# AGENTS.md — Context for AI Coding Agents

> PharmaChat backend: AI Agent Chat cho cong ty duoc pham
> Cap nhat: 2026-10-15 (chuyen sang Go, xem docs/MIGRATION_GO_BACKEND.md)
> Path: .sdd/AGENTS.md

---

## Project Overview

PharmaChat la he thong AI Agent Chat su dung RAG de tu van ban hang 24/7 cho cong ty duoc pham (Huyen Song Khoe). AI tu dong tra loi; sale theo doi va co the tiep quan hoi thoai khi can (quyet dinh B-1). Tich hop Meta Business Suite (Facebook Messenger, Instagram DM) chi cho chat. Su dung Claude Sonnet 4.6 + text-embedding-3-large + pgvector.

## Tech Stack

- Backend: Go, `cmd/api` (HTTP + WebSocket + webhook) va `cmd/agent` (gRPC)
- Database: PostgreSQL 17 + pgvector, migration goose, truy van sqlc
- Job queue: River tren Postgres
- AI: Claude Sonnet 4.6 (LLM) + text-embedding-3-large (embedding, 3072 dim, luu halfvec)
- Frontend: repo rieng (React 19 + Vite + TypeScript + Tailwind)
- Auth: JWT + refresh token, mat khau argon2id
- Real-time: WebSocket + Postgres LISTEN/NOTIFY
- Deploy: Docker Compose + Caddy (1 VPS 8GB RAM)

## Key Files

- .sdd/CONTEXT.md — Business context, cac quyet dinh brainstorming
- .sdd/constitution.md — Tech stack, coding standards, security policies
- .sdd/specs/ — 7 SPEC files (EARS notation)
- .sdd/TASKS.md — Trang thai task GO-xx
- docs/MIGRATION_GO_BACKEND.md — Ke hoach trien khai Go, cac quyet dinh B-1..B-6
- docs/DESIGN_DOCUMENT.md — Thiet ke goc (phan kien truc .NET da bi thay the)
- migrations/ — Database DDL (source of truth)

## Coding Rules

1. Package nghiep vu khong import nhau; dung chung qua internal/platform
2. snake_case trong DB, MixedCaps trong Go
3. Soft delete via deleted_at, truy van mac dinh loc deleted_at IS NULL
4. Multi-tenant: tenant_id tren MOI bang tenant + Row-Level Security; truy van chay trong db.WithTenant
5. JWT: 15 min lifetime, refresh via httpOnly cookie
6. Webhook: HMAC-SHA256 verification bang hmac.Equal tren body goc
7. PII: Never log raw phone/email/message content (logx co loc, nhung khong duoc dua vao log)
8. SQL: Parameterized only (sqlc / pgx)
9. LLM: Never hardcode model name or price - resolve from llm_configs table
10. Compliance: Always append medical disclaimer for product mentions
11. AI Persona: "Chuyen gia Duoc si", nu 30-35, than thien + chuyen nghiep (quyet dinh B-2)
12. AI: KHONG bao loi "minh khong chac chan" — luon tra loi confident, nhung KHONG bia thong tin ngoai KB
13. RAG: text-embedding-3-large (3072 dim) + pgvector halfvec cosine search
14. Memory: Contact memory + 50 tin gan nhat + tom tat cuon chieu (quyet dinh B-6) + vector search
15. Self-learning: FAQ + Pattern detection + Admin toggle

## Task Workflow

1. Pick a task from .sdd/TASKS.md (check dependencies first)
2. Read the relevant SPEC from .sdd/specs/ va muc tuong ung trong docs/MIGRATION_GO_BACKEND.md
3. Write test first (TDD)
4. Implement
5. Run `make lint test`
6. Commit with SPEC / GO-xx reference

## Branch Naming

- Nhanh mac dinh: master
- feat/<scope> — new features
- fix/<scope> — bug fixes
- chore/<scope> — maintenance

## PR Requirements

- CI green (lint + test + sqlc diff + docker build)
- One human reviewer
- Linked SPEC ID hoac GO-xx in PR description
- Updated docs / api/openapi.yaml if public contract changed
