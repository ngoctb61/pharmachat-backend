# Plan PHA-21 · GO-07 · API đăng nhập: argon2id, khóa sau 5 lần sai, JWT 15 phút

## Căn cứ
- Issue: PHA-21 (GO-07). Chặn PHA-75 (FE-05: trang login, access token trong bộ nhớ, refresh, logout).
- Epic: PHA-5 "Giai đoạn 1 — Auth và multi-tenant (SPEC-01)". Điều kiện xong của epic: login, refresh, logout chạy được; test chéo tenant pass; không có endpoint đăng ký công khai. Các issue khác trong epic: PHA-22 (GO-08: refresh, logout, cookie httpOnly), PHA-23 (GO-09: middleware tenant và middleware quyền), PHA-24 (GO-10: test cô lập tenant, truy cập chéo tenant trả 403).
- SPEC-01, các mục:
  - §3: login thành công trả JWT chứa `tenant_id`, `user_id` và claim role.
  - §4: `POST /api/auth/login` nhận `{ email, password }`.
  - §5: khóa ký JWT dài ít nhất 32 ký tự, lấy từ biến môi trường; argon2id dạng PHC; refresh token nằm trong cookie httpOnly.
  - §7: NFR-01 login p95 dưới 500ms; NFR-02 JWT sống 15 phút.
  - §8: ma trận lỗi gồm Invalid credentials, Account locked, Tenant inactive.
- Kế hoạch migrate sang Go:
  - Mục 2: B1 (băm SHA-256), B3 (login không chạy được vì cần tenant), B6 (không có đăng ký công khai).
  - Mục 5.3: truy vấn login chạy trước khi biết tenant, phải đi qua một đường hẹp.
  - Mục 8: hợp đồng API.
- Kế hoạch tách repo frontend:
  - Mục 4, F-3: tạo `api/openapi.yaml` từ GO-07, có lint spec và test đối chiếu route.
  - Luồng token: access token 15 phút nằm trong body.
- Không có quyết định B-1..B-6 nào liên quan trực tiếp.

## Cách làm
- **Hai bước tra user, giữ RLS cho mọi thao tác ghi** (decision-02).
  - Bước 1: một function `SECURITY DEFINER` hẹp, `auth_login_candidate(email)`, chỉ trả `(user_id, tenant_id)` từ email đã chuẩn hóa.
  - Bước 2: mọi việc còn lại chạy trong `db.WithTenant(tenant_id)` bằng role `pharmachat_app`: đọc hash và cờ khóa, đọc trạng thái tenant, ghi bộ đếm sai và `last_login_at`.
  - Không thêm role DB hay biến `DATABASE_URL` thứ hai.
- **Email là duy nhất toàn hệ thống** cho user chưa xóa, nhờ unique index trên `lower(email)` (decision-02). Hợp đồng `{email, password}` không có ô chọn tenant, nên email phải xác định được đúng một user.
- **Phạm vi token** (decision-01):
  - PHA-21 chỉ trả access token trong body `{ token, user }`.
  - Refresh token, cookie và `Set-Cookie` trên response login thuộc PHA-22. Khi đó PHA-22 sửa thêm `/api/auth/login` trong `api/openapi.yaml`.
- **Khóa tài khoản** dùng sẵn hai cột `users.access_failed_count` và `users.lockout_end` (decision-03).
  - Lần sai thứ 5 đặt `lockout_end = now + thời lượng khóa` và đưa bộ đếm về 0.
  - Trong lúc bị khóa, request bị từ chối mà không kiểm mật khẩu và không tăng bộ đếm.
  - **Ba bước, argon2id nằm ngoài transaction** (góp ý asec):
    1. Transaction 1, ngắn, trong `WithTenant`: đọc hash, trạng thái khóa, `is_active` của user và tenant.
    2. Verify argon2id ngoài transaction, dưới semaphore. Không giữ kết nối DB trong lúc băm.
    3. Transaction 2, ngắn, trong `WithTenant`: ghi kết quả bằng câu có điều kiện.
  - Mục đích: pool mặc định của pgxpool chỉ có max(4, NumCPU) kết nối, và `db.Open` (`internal/platform/db/db.go:20-24`) giữ nguyên mặc định đó. Nếu băm trong transaction, một loạt request login (không cần đăng nhập) có thể chiếm hết pool, làm `/health` và mọi endpoint khác phải chờ.
  - **Quyết định khóa ngay lúc ghi** (góp ý asec), để các request chạy song song không vượt được khóa:
    - `RecordLoginSuccess`: `UPDATE ... SET access_failed_count = 0, lockout_end = NULL, last_login_at = now() WHERE id = $1 AND (lockout_end IS NULL OR lockout_end <= now()) RETURNING id`. Ra 0 dòng thì trả `ACCOUNT_LOCKED`, không phát token.
    - `RecordLoginFailure`: một câu `UPDATE ... SET access_failed_count = access_failed_count + 1 ... RETURNING`, cùng điều kiện `WHERE`, nguyên tử, để các request song song không đếm thiếu.
  - Kết quả "sai mật khẩu" phải được **commit**: transaction 2 trả về giá trị kết quả, không trả error, vì trả error thì transaction bị rollback.
- **JWT** (decision-04):
  - HS256, thư viện `golang-jwt/jwt/v5` (theo tech stack mục 4), khóa lấy từ `JWT_SIGNING_KEY` và phải dài ít nhất 32 byte.
  - Claim: `user_id`, `tenant_id`, `roles` (mảng tên role), `iat`, `exp = iat + 15 phút`, `iss`.
  - Package `auth` có thêm hàm `Parse`, dùng `jwt.NewParser(jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("pharmachat-api"), jwt.WithExpirationRequired())`. Mặc định golang-jwt/v5 nhận token không có `exp` và không kiểm `iss` (góp ý asec). Test dùng `Parse` để chứng minh token hợp lệ, và PHA-23 dùng lại.
- **Chống dò tài khoản và chống làm nghẽn api.**
  - Email không tồn tại vẫn chạy một lần verify argon2id với một hash giả tạo sẵn lúc khởi động, rồi trả cùng mã lỗi `INVALID_CREDENTIALS` như khi sai mật khẩu.
  - Số lượt verify argon2id chạy đồng thời bị giới hạn bằng semaphore (4). Mỗi lượt cần 19 MiB, trong khi container `api` có `mem_limit: 256m`. Endpoint này không cần đăng nhập, nên nếu không giới hạn thì một loạt request song song có thể làm container bị OOM-kill. Giới hạn tốc độ (rate limit) vẫn thuộc GO-43 (PHA-59).
  - Chờ semaphore có hạn (góp ý asec): `select` trên `ctx.Done()` và một timer khoảng 2 giây. Hết hạn thì trả 503 `SERVICE_BUSY`, để hàng đợi không phình mãi và người đăng nhập thật không phải chờ vô hạn.
- **Hợp đồng `api/openapi.yaml` (OpenAPI 3.1)** gồm `/api/auth/login`, envelope lỗi `{errorCode, message, requestId}` và `securitySchemes.bearerAuth` để dùng về sau.
  - F-3 mục 4 đã chốt hai cơ chế giữ spec khớp với code, và chưa issue nào khác nhận làm:
    - một test Go so route `/api/*` của chi với `paths` trong spec;
    - một job CI lint spec bằng Redocly CLI, ghim version.
- Code nằm trong `internal/auth` (đã có `password.go`), chỉ import `internal/platform/*` và `internal/store`. `cmd/api` khởi tạo và gắn route vào router.

## Thay đổi dự kiến
| File / package | Thêm / sửa | Nội dung |
|---|---|---|
| `migrations/00002_auth_login.sql` | Thêm | Unique index `ux_users_email_active`; function `auth_login_candidate`; REVOKE/GRANT. Down xóa cả hai. |
| `internal/store/queries/auth.sql` | Thêm | `LoginCandidate`, `GetUserForLogin` (join `tenants` để lấy `is_active`), `ListUserRoleNames`, `RecordLoginFailure`, `RecordLoginSuccess` |
| `internal/store/*.go` | Sinh lại | `make gen`, không sửa tay |
| `internal/auth/token.go` | Thêm | `TokenIssuer` (`NewTokenIssuer(key, now)`, `Issue`, `Parse`), `Claims`, hằng `AccessTokenTTL = 15 * time.Minute` |
| `internal/auth/login.go` | Thêm | `Service.Login(ctx, email, password) (LoginResult, error)`: chuẩn hóa email, tra user, kiểm tra khóa, verify, ghi kết quả, phát token. Có các lỗi định sẵn kiểu `*httpx.Error`, hash giả và semaphore. |
| `internal/auth/http.go` | Thêm | `Handler.Login`: giới hạn body 4 KiB, decode JSON, kiểm tra trường bắt buộc, gọi `Service.Login`, trả JSON hoặc `httpx.WriteError` |
| `internal/platform/config/config.go` | Sửa | `API.JWTSigningKey string env:"JWT_SIGNING_KEY,notEmpty"` |
| `internal/platform/config/config_test.go` | Sửa | `TestLoadRequiresDatabaseURL` phải đặt thêm `JWT_SIGNING_KEY`; thêm trường hợp thiếu biến này thì lỗi |
| `internal/platform/migrate/migrate_test.go` | Sửa | Dòng 37-41 giả định `Down` xóa `kb_chunks`. Khi có `00002`, `Down` chỉ lùi `00002`, nên đổi sang kiểm tra function `auth_login_candidate` và index mới biến mất rồi xuất hiện lại. |
| `internal/seed/seed.go:180-181` | Sửa | Thông báo của `ErrUserExists` hiện ghi "in the tenant". Với index toàn hệ thống, trùng email ở tenant khác cũng ra lỗi này, nên đổi thành "a user with this email already exists". |
| `cmd/api/main.go` | Sửa | `newRouter` nhận struct phụ thuộc (log, pool, auth handler); route `POST /api/auth/login`; tạo `TokenIssuer` từ config, lỗi khởi động nếu khóa ngắn hơn 32 byte |
| `cmd/api/openapi_test.go` | Thêm | Test so route với spec (gọi `chi.Walk`, bỏ qua `/health`) |
| `api/openapi.yaml` | Thêm | Spec OpenAPI 3.1 như mô tả ở trên |
| `deploy/.env.example` | Sửa | `JWT_SIGNING_KEY=` kèm hướng dẫn `openssl rand -base64 48` |
| `deploy/docker-compose.yml` | Sửa | Chỉ service `api` nhận `JWT_SIGNING_KEY: ${JWT_SIGNING_KEY:?...}`, không đặt vào anchor `service-env` vì `agent` không cần |
| `.github/workflows/ci.yml`, `Makefile` | Sửa | Job `openapi` chạy Redocly CLI ghim version qua Docker image; target `make lint-api` |
| `go.mod` / `go.sum` | Sửa | Thêm `github.com/golang-jwt/jwt/v5`; chuyển `go.yaml.in/yaml/v3` thành phụ thuộc trực tiếp vì test đọc spec |

## Migration
`migrations/00002_auth_login.sql` không tạo bảng mới, nên không có RLS hay GRANT bảng mới.
- Up:
  - `CREATE UNIQUE INDEX ux_users_email_active ON users (lower(email)) WHERE deleted_at IS NULL;`
  - `CREATE FUNCTION auth_login_candidate(p_email text) RETURNS TABLE (user_id uuid, tenant_id uuid) LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public, pg_temp`. Thân function lấy `u.id, u.tenant_id` từ `users u JOIN tenants t`, điều kiện `lower(u.email) = lower(btrim(p_email))`, `u.deleted_at IS NULL` và `t.deleted_at IS NULL`. Bọc trong `StatementBegin`/`StatementEnd`.
  - `REVOKE ALL ON FUNCTION auth_login_candidate(text) FROM PUBLIC; GRANT EXECUTE ON FUNCTION auth_login_candidate(text) TO pharmachat_app;`
- Down: `DROP FUNCTION IF EXISTS auth_login_candidate(text); DROP INDEX IF EXISTS ux_users_email_active;`
- Function thuộc schema owner (role chạy `pcctl migrate`), nên bỏ qua được RLS. Đây là đường duy nhất để role app đọc chéo tenant, và nó chỉ trả về hai id.
- D5: chưa có production nên không lo dữ liệu trùng email làm index tạo thất bại.

## Test viết trước
| Test | Chứng minh điều gì |
|---|---|
| `auth.TestTokenIssuer_IssueAndParse` | Token có `user_id`, `tenant_id`, `roles`, `iss`; `exp - iat` đúng 15 phút |
| `auth.TestNewTokenIssuer_RejectsShortKey` | Khóa ngắn hơn 32 byte thì báo lỗi (SPEC-01 §5) |
| `auth.TestTokenIssuer_ParseRejects` (dạng bảng) | Từ chối token hết hạn, token ký bằng khóa khác, `alg: none`, HS512, token thiếu `exp`, token có `iss` khác |
| `config.TestLoadRequiresJWTSigningKey` | Thiếu hoặc để rỗng `JWT_SIGNING_KEY` thì `Load[API]` báo lỗi |
| `auth.TestLogin_Success` (DB thật, pool `env.App`) | Chạy được dưới RLS bằng role app. Trả token và user kèm roles; `last_login_at` được đặt; bộ đếm về 0 |
| `auth.TestLogin_EmailNormalized` | `" Admin@X.com "` đăng nhập được user lưu `admin@x.com` |
| `auth.TestLogin_UnknownEmailAndWrongPasswordLookAlike` | Cả hai trả `INVALID_CREDENTIALS` 401, cùng `Message` |
| `auth.TestLogin_WrongPasswordIsPersisted` | Sau một lần sai, `access_failed_count = 1` đọc bằng owner, tức là kết quả sai đã được commit, không bị rollback |
| `auth.TestLogin_LocksAfterFiveFailures` | Lần sai thứ 1-4 trả 401; lần thứ 5 trả `ACCOUNT_LOCKED` 403 và `lockout_end` được đặt. Mật khẩu đúng trong lúc bị khóa vẫn trả `ACCOUNT_LOCKED`, bộ đếm không đổi |
| `auth.TestLogin_ConcurrentCannotBypassLock` | Tài khoản bị khóa trong lúc các request khác vẫn đang verify: những request đó, kể cả có mật khẩu đúng, không nhận token và không xóa `lockout_end`. Dùng một hook trong `Service` chặn giữa bước verify và bước ghi để tạo thứ tự xen kẽ một cách ổn định |
| `auth.TestLogin_BusyReturns503` | Giữ hết slot semaphore, request tiếp theo nhận 503 `SERVICE_BUSY` sau khi hết hạn chờ |
| `auth.TestLogin_LockExpires` | Đặt `lockout_end` về quá khứ bằng owner thì mật khẩu đúng đăng nhập được, khóa được xóa |
| `auth.TestLogin_SuccessResetsCounter` | Sai 3 lần rồi đúng thì bộ đếm về 0 |
| `auth.TestLogin_InactiveStates` (dạng bảng) | User `is_active=false` + mật khẩu đúng trả `ACCOUNT_DISABLED` 403; tenant `is_active=false` trả `TENANT_INACTIVE` 403; mật khẩu sai trong cả hai trường hợp vẫn trả `INVALID_CREDENTIALS`; user đã xóa mềm trả `INVALID_CREDENTIALS` |
| `auth.TestLogin_EmailUniqueAcrossTenants` | Owner chèn cùng email (khác hoa thường) vào tenant thứ hai thì bị lỗi 23505 |
| `auth.TestLoginCandidate_AppRoleSeesOnlyIDs` | Role app gọi được function và nhận đúng `(user_id, tenant_id)`; ngoài `WithTenant` vẫn đếm `SELECT count(*) FROM users` ra 0 |
| `auth.TestLoginHandler` (dạng bảng, `httptest`) | 200 đúng hình dạng JSON. Các trường hợp 400 `VALIDATION_ERROR`: JSON hỏng, thiếu trường, body quá 4 KiB. Lỗi 401 và 403 dùng envelope có `requestId` |
| `main.TestOpenAPI_RoutesMatchSpec` | Mỗi route `/api/*` của chi có trong `paths` của spec với đúng method, và ngược lại |
| `migrate.TestUpDownUp` (sửa) | `00002` lên, xuống, lên lại được |

## Điều kiện xong của task
- `make test-unit` và `make test` pass, gồm mọi test ở bảng trên và `TestRLS_*` hiện có.
- `make lint`, `make gen-check` sạch; `go mod tidy -diff` không còn khác biệt.
- `make lint-api` (Redocly) sạch với `api/openapi.yaml`.
- Kiểm chứng trên stack thật bằng `/verify-stack`:
  1. Chạy `make up && make seed && make create-admin EMAIL=... NAME=...`.
  2. `curl -sk -X POST https://localhost/api/auth/login -d '{"email":...,"password":...}'` trả 200 với `token` và `user.roles = ["Admin"]`.
  3. Gửi sai mật khẩu 5 lần: lần thứ 5 trả 403 `ACCOUNT_LOCKED`.
  4. `api` khởi động thất bại với thông báo rõ ràng khi thiếu `JWT_SIGNING_KEY`.
- Không có endpoint đăng ký nào: `grep -rn "register" cmd internal api` không ra route nào.

## Rủi ro
- **sqlc và function `RETURNS TABLE`.** Nếu `sqlc generate` không suy ra được kiểu cột của `SELECT * FROM auth_login_candidate(@email)`, dùng `SELECT c.user_id::uuid, c.tenant_id::uuid FROM auth_login_candidate(@email) c` để ép kiểu. Không viết pgx tay ngoài sqlc.
- **Lộ trạng thái khóa.** `ACCOUNT_LOCKED` cho người ngoài biết email đó tồn tại. SPEC-01 §8 yêu cầu hiện thông báo này, nên chấp nhận. Kẻ tấn công còn có thể cố ý khóa tài khoản người khác; rate limit theo IP ở GO-43 sẽ giảm rủi ro này.
- **Thời gian xử lý.** Hai lượt argon2id (đăng nhập thật, hoặc lượt verify hash giả) tốn khoảng 30-60ms. Nếu semaphore đầy, request phải chờ. Vẫn nằm trong NFR-01 (p95 dưới 500ms) với tải bình thường.
- **Frontend tưởng 401 là token hết hạn.** 401 từ `/api/auth/login` nghĩa là sai thông tin đăng nhập, không phải token hết hạn. Spec phải ghi rõ, để middleware FE (FE-03, FE-05) không gọi refresh với các route `/api/auth/*`.
- **Đổi hợp đồng public.** Đây là phiên bản đầu của `api/openapi.yaml`. Sau khi merge, báo repo frontend kéo spec về bằng `make sync-contract`.

## Ngoài phạm vi
- Refresh token, cookie, `/api/auth/refresh`, `/api/auth/logout`: PHA-22.
- Middleware xác thực JWT, middleware tenant, middleware quyền: PHA-23. Task này chỉ cung cấp `TokenIssuer.Parse`.
- Test cô lập tenant toàn hệ thống, truy cập chéo tenant trả 403: PHA-24.
- Rate limit theo IP: GO-43 (PHA-59).
- Mở khóa tài khoản và CRUD user: GO-36 (PHA-52).
- Rà spec cho đủ: GO-47 (PHA-63).

## Câu hỏi mở
- **Khóa tài khoản trong bao lâu?** SPEC-01 §8 ghi cách xử lý là "Contact admin", nhưng không nói khóa vĩnh viễn hay khóa có thời hạn. Việc admin mở khóa phải chờ GO-36 (giai đoạn 7). Vì vậy nếu khóa vĩnh viễn, đến lúc đó chỉ mở được bằng cách sửa DB tay.
  - Plan đang dùng mặc định **khóa 15 phút** (hằng `LockoutDuration`). asec khuyên giữ mức này: ai cũng khóa được tài khoản người khác, nên nếu khóa vĩnh viễn thì chỉ cần 5 request là khóa được admin duy nhất của một tenant, cho đến khi có người sửa DB tay.
  - Nếu chọn khóa đến khi admin mở, chỉ cần đổi `lockout_end` thành `'infinity'` và sửa test `TestLogin_LockExpires`.
