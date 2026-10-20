# Tóm tắt PHA-21 · vòng implement

Plan duyệt ở `2479618`. Hai câu hỏi mở không được trả lời nên giữ mặc định của plan: khóa 15 phút (`LockoutDuration`), claim `roles` là mảng.

## Đã làm (đối chiếu plan)
| Mục trong plan | Trạng thái | Ghi chú |
|---|---|---|
| `migrations/00002_auth_login.sql` | Xong | Index `ux_users_email_active`, function `auth_login_candidate` (SECURITY DEFINER, `search_path = public, pg_temp`), REVOKE PUBLIC / GRANT app. Down xóa cả hai. |
| `internal/store/queries/auth.sql` + `make gen` | Xong | `LoginCandidate`, `GetUserForLogin`, `ListUserRoleNames`, `RecordLoginFailure`, `RecordLoginSuccess`. Cả hai câu ghi đều kiểm lại khóa trong `WHERE`. Khoảng thời gian khóa dùng `make_interval(secs => ...)`, vì sqlc sinh sai tham số với `@x::int * interval '1 second'`. |
| `internal/auth/token.go` | Xong | HS256; `Parse` dùng `WithValidMethods(HS256)`, `WithIssuer`, `WithExpirationRequired` |
| `internal/auth/login.go` | Xong | Đọc (tx 1) → argon2id ngoài transaction, dưới semaphore 4 slot, chờ tối đa 2s → ghi (tx 2). Email không tồn tại thì verify hash giả. |
| `internal/auth/http.go` | Xong | Body giới hạn 4 KiB, `VALIDATION_ERROR` khi JSON hỏng hoặc thiếu trường |
| `config.API.JWTSigningKey` (`notEmpty`) | Xong | |
| `config_test.go`, `migrate_test.go` | Xong | `TestUpDownUp` giờ kiểm `00002` xuống rồi lên lại, còn `kb_chunks` vẫn còn sau Down |
| `seed.ErrUserExists` | Xong | Thông báo bỏ "in the tenant" |
| `cmd/api/main.go` | Xong | `routerDeps`; `TokenIssuer` được tạo trước khi mở DB, để lỗi khóa xuất hiện ngay lúc khởi động |
| `cmd/api/openapi_test.go` | Xong | Dùng `chi.Walk` so route `/api/*` với `paths` trong spec |
| `api/openapi.yaml` | Xong | OpenAPI 3.1, gồm `/api/auth/login`, envelope lỗi, `bearerAuth` |
| `deploy/.env.example`, `docker-compose.yml` | Xong | `JWT_SIGNING_KEY` chỉ cấp cho service `api`, kèm `:?` |
| CI job `openapi` + `make lint-api` | Xong | `redocly/cli:2.57.0` (phát hành 2026-09-30) |
| `go.mod` | Xong | Thêm `golang-jwt/jwt/v5 v5.3.1`; `go.yaml.in/yaml/v3` thành phụ thuộc trực tiếp |

## Lệch so với plan
- **Sửa `.claude/skills/verify-stack/SKILL.md` và `README.md`.** Plan không liệt kê hai file này. Vì compose bắt buộc phải có `JWT_SIGNING_KEY` (`:?`), file `.env` tạm mà `verify-stack` tạo ra sẽ làm stack không dựng được. Đã thêm dòng `JWT_SIGNING_KEY=$(openssl rand -base64 48)`, và README nhắc điền biến này.
- **Spec đặt `security: [bearerAuth]` ở mức gốc**, login ghi đè bằng `security: []`. Không có dòng này thì Redocly cảnh báo `no-unused-components`. Nó cũng nói đúng ý định: các endpoint sau này mặc định cần token.
- **`Parse` từ chối token có `user_id` hoặc `tenant_id` rỗng.** Plan không ghi điều này; thêm vì PHA-23 dùng hai id này để đặt tenant scope.
- **Có thêm `TestLogin_UserWithoutRoles`**: user không có role thì `roles` là `[]`, không phải `null` (decision-04).

## Bằng chứng
- `make lint`: `0 issues.`
- `make gen-check` và `go mod tidy -diff`: sạch.
- `make lint-api`: `Woohoo! Your API description is valid.`, không có warning.
- `go test -race -count=1 ./...`: mọi package `ok`. Chạy `-v` để xác nhận các test DB thật sự chạy, không bị skip:
  - `internal/auth`: 14 test login/handler, 3 test token, 3 test password cũ, tất cả `PASS`.
  - `TestUpDownUp` và `TestOpenAPI_RoutesMatchSpec`: `PASS`.
- **Test xử lý race thật sự bắt được lỗi:** tạm bỏ điều kiện khóa trong `RecordLoginSuccess` rồi chạy lại `make gen`. Khi đó `TestLogin_ConcurrentCannotBypassLock` FAIL: `Expected error with "ACCOUNT_LOCKED..." in chain but got nil`, tức request đang chạy lấy được token. Khôi phục câu SQL thì test pass trở lại.
- **`/verify-stack`** với `.env` tạm (đã xóa sau khi xong):
  - Container: postgres, api, agent healthy; pcctl `Exited (0)`.
  - `/health` trả `{"status":"ok","checks":{"postgres":"ok"}}` và có `x-request-id`.
  - `/api/khong-ton-tai` trả envelope `NOT_FOUND`.
  - TLS 1.2 bị từ chối với `alert internal error`, TLS 1.3 kết nối được.
  - `pcctl seed` chạy 2 lần, cả hai lần cùng `tenant_id`.
  - `create-admin` với email `" Admin@Example.com "`, rồi login bằng `admin@example.com` trả 200: `roles:["Admin"]`, `tenantId` đúng tenant đã seed. Payload JWT có `user_id`, `tenant_id`, `roles`, `iss:"pharmachat-api"`, `exp`, `iat`.
  - Tài khoản thứ hai sai mật khẩu 5 lần: lần 1-4 trả `INVALID_CREDENTIALS`, lần 5 trả `ACCOUNT_LOCKED`. Sau đó nhập mật khẩu đúng vẫn trả `ACCOUNT_LOCKED`.
  - Email không tồn tại trả `INVALID_CREDENTIALS`; JSON hỏng trả `VALIDATION_ERROR`; `GET` trả `METHOD_NOT_ALLOWED`.
  - Log của api không chứa email nào (`grep -c example.com` ra 0 trên 19 dòng log của route login).
- Binary api không khởi động khi khóa ký sai:
  - Thiếu `JWT_SIGNING_KEY`: `api: load config: env: environment variable "JWT_SIGNING_KEY" should not be empty`, exit 1.
  - Khóa 5 byte: `api: JWT_SIGNING_KEY must be at least 32 bytes, got 5`, exit 1.
- Không có endpoint đăng ký: grep `register|signup` trong `cmd internal api` chỉ ra `RegisterHealthServer` (gRPC) và `jwt.RegisteredClaims`.

## Ngoài phạm vi phát hiện được
- Sau khi PR này merge, cần báo repo frontend kéo `api/openapi.yaml` về. Plan đã ghi điều này trong mục Rủi ro.
