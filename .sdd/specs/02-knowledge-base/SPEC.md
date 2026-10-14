# SPEC-02 - Knowledge Base

Status: DRAFT
Traces to: FR-02, UC-B01..B12

---

## 1. Business Context
KB la IP cua cong ty duoc pham. AI chi tu van chinh xac khi KB chuan. Muc tieu accuracy >= 85% tren 20-cau test set. Admin co the them san pham bat cu luc nao — he thong tu dong embed + phan loai. Support upload .txt/.md hoac viet Markdown truc tiep.

## 2. User Stories
- AS A Admin I WANT them/sua/xoa KB modules SO THAT AI co du thong tin tu van.
- AS A Admin I WANT deploy KB version moi khong downtime SO THAT AI gap ngay noi dung moi.
- AS A Admin I WANT upload file .txt/.md SO THAT khong can copy-paste Markdown.
- AS A Admin I WANT xem accuracy score moi version SO THAT toi co the rollback neu giam.

## 3. Acceptance Criteria (EARS)
- THE SYSTEM SHALL store KB modules in kb_modules keyed by (tenant_id, code).
- WHEN a new kb_versions.status transitions to deployed THE SYSTEM SHALL embed content vao pgvector trong 30 giay.
- THE SYSTEM SHALL chunk content thanh cac chunk ~1000 ky tu.
- THE SYSTEM SHALL embed each chunk voi text-embedding-3-large (3072 dim).
- THE SYSTEM SHALL store vector trong pgvector voi metadata: tenant_id, module_code, version_id, snippet.
- WHEN accuracy_score < 0.85 THE SYSTEM SHALL emit alert va prevent auto-deploy.
- WHILE a version is deployed THE SYSTEM SHALL prevent edits - new content goes to new version.
- THE SYSTEM SHALL accept upload .txt and .md files.
- THE SYSTEM SHALL validate file size max 10MB per upload.
- WHERE admin clicks Deploy THE SYSTEM SHALL embed + upsert pgvector atomically.

## 4. API Contracts / Data Models
- GET /api/kb/modules - list module + latest version
- POST /api/kb/modules - create new module
- POST /api/kb/modules/{id}/versions - create draft version
- PUT /api/kb/versions/{id} - edit draft content (Markdown)
- POST /api/kb/versions/{id}/upload - upload .txt/.md file, convert to Markdown
- POST /api/kb/versions/{id}/deploy - embed + upsert pgvector
- POST /api/kb/versions/{id}/rollback - flip to previous deployed
- Tables: kb_modules, kb_versions, kb_test_cases

## 5. Technical Constraints
- Embedding: text-embedding-3-large via OpenAI API.
- Vector Store: pgvector (Postgres).
- Chunk size: ~1000 ky tu/chunk.
- Retry: max 4 attempts voi exponential backoff (8s, 16s, 24s, 32s).
- File upload: chi .txt va .md, max 10MB.

## 6. Out of Scope
- Auto-ingest from external sources (manual upload only)
- PDF/DOCX upload (Phase 2)
- Multi-language KB (only Vietnamese in Phase 1)

## 7. NFR
- NFR-05: Accuracy >= 85% / alert on drop.
- NFR-02: Search p95 < 200ms.
- NFR-09: KB deploy completes within 30 seconds.

## 8. Error Handling Matrix
| Error | Detection | User-visible | Recovery |
|---|---|---|---|
| Embedding API timeout | SK exception | Retry toast | Retry x3 with backoff |
| pgvector upsert fail | Exception | "Deploy that bai" | Retry + manual |
| File too large | Validation | "File qua lon (max 10MB)" | Nhap lai |
| File format not supported | Validation | "Chi ho tro .txt va .md" | Chon file khac |
| Deploy fails mid-flight | Atomic txn | Rollback | None - txn rolls back |

## 9. Open Questions
| Item | Owner | Due | Status |
|---|---|---|---|
| Chunk size 1000 hay 500 ky tu tot hon? | Backend | T2 | open |
