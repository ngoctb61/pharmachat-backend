# SPEC-01 - Auth & Multi-tenant

Status: DRAFT
Traces to: FR-01, UC-A01..A05

---

## 1. Business Context
He thong can phan biet giua cac tenant (doanh nghiep) va nguoi dung (sale, admin). Moi tenant chi thay du lieu cua minh. Auth la nen tang cho toan bo he thong. Sale thay toan bo inbox cong ty (khong phan quyen theo sale).

## 2. User Stories
- AS A Admin I WANT dang nhap voi email/password SO THAT toi quan ly he thong.
- AS A Sale I WANT dang nhap va xem toan bo inbox SO THAT toi theo doi AI tu van.
- AS A Khach hang I WANT gui tin nhan qua Messenger SO THAT AI tu van cho toi (khong can dang nhap).

## 3. Acceptance Criteria (EARS)
- THE SYSTEM SHALL store tenants in tenants table keyed by slug (unique).
- THE SYSTEM SHALL store users in users table, each linked to exactly one tenant_id.
- WHEN a user logs in successfully THE SYSTEM SHALL return a JWT containing tenant_id, user_id, role claims.
- WHILE a request is in-flight THE SYSTEM SHALL resolve tenant_id from JWT and apply Postgres Row-Level Security (set_config app.tenant_id per transaction).
- IF a user attempts to access data from another tenant THEN THE SYSTEM SHALL return 403 Forbidden.
- WHERE a user has role sale THE SYSTEM SHALL show all conversations (khong phan quyen theo sale).

## 4. API Contracts / Data Models
- POST /api/auth/login - { email, password } -> { token, refreshToken, user }
- POST /api/auth/refresh - refresh JWT
- POST /api/auth/logout - invalidate refresh token
- Tables: tenants, users, roles, user_roles, permissions, role_permissions

## 5. Technical Constraints
- JWT signing key >= 32 chars random, stored in env var.
- Password hashed via argon2id (salted, PHC format).
- Refresh token stored in httpOnly secure cookie.

## 6. Out of Scope
- OAuth/social login (Phase 2)
- Multi-factor authentication (Phase 2)
- Per-sale conversation isolation (khong can — moi sale thay het)

## 7. NFR
- NFR-01: Login p95 < 500ms.
- NFR-02: JWT lifetime 15 min, refresh 7 days.

## 8. Error Handling Matrix
| Error | Detection | User-visible | Recovery |
|---|---|---|---|
| Invalid credentials | Identity check fail | Email hoac mat khau khong dung | Retry |
| Expired JWT | Token validation fail | 401 + redirect login | Refresh token |
| Account locked | Identity lockout | Tai khoan da bi khoa | Contact admin |
| Tenant inactive | Query filter empty | 403 Forbidden | Contact admin |

## 9. Open Questions
| Item | Owner | Due | Status |
|---|---|---|---|
| Can passwordless login (magic link)? | Admin | T2 | open |
