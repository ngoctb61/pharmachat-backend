# SPEC-07 - Admin Dashboard

Status: DRAFT
Traces to: FR-07, UC-G01..G05

---

## 1. Business Context
Admin can platform de quan ly: (1) Knowledge Base, (2) Meta integration, (3) Users, (4) Self-learning toggle. Khong can feature phuc tap (content, docs, ads, reports). Chi can nhung gi can thiet de he thong hoat dong.

## 2. User Stories
- AS A Admin I WANT quan ly KB modules (them/sua/xoa/deploy/upload file) SO THAT AI co du thong tin tu van.
- AS A Admin I WANT ket noi/quin ly Meta App SO THAT Messenger/Instagram hoat dong.
- AS A Admin I WANT quan ly users (them/sua/xoa, phan quyen) SO THAT sale co the su dung he thong.
- AS A Admin I WANT bat/tat self-learning mode SO THAT AI hoc tu chat logs.

- AS A Admin I WANT cau hinh LLM va Embedding tu UI SO THAT AI su dung dung model va API key.

## 3. Acceptance Criteria (EARS)
- THE SYSTEM SHALL provide KB management UI: list modules, create/edit versions, upload .txt/.md, deploy, rollback.
- THE SYSTEM SHALL provide Meta integration UI: connect, view status, sync pages, disconnect.
- THE SYSTEM SHALL provide AI config UI: cau hinh LLM (provider, model, API key, temperature) va Embedding (provider, model, API key, dimension).
- THE SYSTEM SHALL encrypt API key voi AES-256 truoc khi luu database.
- THE SYSTEM SHALL validate API key by calling test endpoint khi admin click "Kiem tra ket noi".
- WHERE user role is not admin THE SYSTEM SHALL hide all admin pages.
- WHEN AI config is saved THE SYSTEM SHALL immediately use new config for next chat request.

## 4. API Contracts / Data Models
- GET /api/admin/dashboard - basic metrics
- All KB endpoints from SPEC-02
- All Meta endpoints from SPEC-04
- User CRUD endpoints from SPEC-01
- POST /api/admin/self-learning/toggle - { mode: auto | manual }
- Frontend pages: /admin/kb, /admin/meta, /admin/users, /admin/self-learning

## 5. Technical Constraints
- React 19 + Vite + Tailwind for admin UI.
- All admin endpoints require admin:system permission.
- Dashboard metrics aggregated from existing tables (no new analytics tables).

## 6. Out of Scope
- Advanced analytics/reporting (Phase 2)
- Content management (Phase 2)
- Document generation (Phase 2)
- Ads management (Phase 2)
- System logs viewer (Phase 2)

## 7. NFR
- NFR-15: Dashboard load p95 < 2 seconds.
- NFR-09: KB deploy completes within 30 seconds.
- NFR-16: Admin UI responsive on mobile.

## 8. Error Handling Matrix
| Error | Detection | User-visible | Recovery |
|---|---|---|---|
| KB deploy fail | API error | "Deploy that bai" + error details | Retry |
| Meta connect fail | OAuth error | "Ket noi that bai" + instructions | Retry OAuth |
| User create fail | Validation error | Form validation message | Fix input |
| Dashboard load slow | Timeout | Loading spinner + timeout message | Refresh |

## 9. Open Questions
| Item | Owner | Due | Status |
|---|---|---|---|
| Need role customization beyond admin/sale? | Admin | T3 | open |

