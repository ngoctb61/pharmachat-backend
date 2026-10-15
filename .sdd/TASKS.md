# TASKS.md — PharmaChat Go backend

> Phien ban: 0.3 (2026-10-15, chuyen sang Go)
> Noi dung chi tiet tung task: docs/MIGRATION_GO_BACKEND.md muc 7.
> Danh sach TASK-01..24 cua ban .NET da bi thay the boi GO-00..47.

Trang thai: `todo` | `doing` | `done` | `blocked`

## Giai doan 0 — Nen mong

| Task | Noi dung | Trang thai | Ghi chu |
|---|---|---|---|
| GO-00 | Repo GitHub moi, bao ve `master`, chuyen `.sdd/` | blocked | Repo ngoctb61/pharmachat-backend (private) da tao. Bao ve nhanh can GitHub Pro hoac repo public. Archive repo .NET can quyen admin cua NguyenTung309 |
| GO-01 | Khung code, go.mod, Makefile, golangci-lint | done | |
| GO-02 | platform: config, slog loc PII, error envelope, request ID, recover | done | |
| GO-03 | Migration Postgres + pgvector, sqlc, RLS | done | Test RLS chay tren Postgres that |
| GO-04 | docker-compose: postgres, pcctl, api, agent, caddy | done | Da dung thu: health OK, chi chap nhan TLS 1.3 |
| GO-05 | CI GitHub Actions | done | Lan chay dau tien xanh ca 6 job |
| GO-06 | pcctl: migrate, seed, create-admin | done | |

## Giai doan 1..9

| Giai doan | Task | Trang thai |
|---|---|---|
| 1 — Auth va multi-tenant (SPEC-01) | GO-07..GO-10 | todo |
| 2 — Knowledge Base (SPEC-02) | GO-11..GO-14 | todo |
| 3 — Agent service (SPEC-03) | GO-15..GO-23 | todo |
| 4 — Compliance (SPEC-05) | GO-24..GO-27 | todo |
| 5 — Inbox va realtime (SPEC-06) | GO-28..GO-30a | todo |
| 6 — Meta (SPEC-04) | GO-31..GO-35 (gom GO-32a) | todo |
| 7 — Admin API (SPEC-07) | GO-36..GO-39 | todo |
| 8 — Tu hoc | GO-40..GO-41 | todo |
| 9 — Van hanh va go-live | GO-42..GO-47 | todo |
