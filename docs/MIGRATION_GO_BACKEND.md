# Kế hoạch migrate backend PharmaChat sang Go

> Ngày lập: 2026-10-15
> Phạm vi: backend, hạ tầng và xử lý nền. Kế hoạch tách repo frontend sẽ làm riêng sau.
> Nguồn yêu cầu: `.sdd/specs/01..07`, `CONTEXT.md`, `DESIGN_DOCUMENT.md`, `constitution.md`

---

## 1. Các quyết định đã chốt

| # | Quyết định | Lựa chọn |
|---|---|---|
| D1 | Database | **PostgreSQL + pgvector**: một DB lo cả dữ liệu quan hệ lẫn vector, bỏ Qdrant và SQL Server |
| D2 | Kiến trúc | **2 service**: `api` (HTTP, WebSocket, webhook) và `agent` (gRPC), giữ đúng thiết kế trong DESIGN_DOCUMENT |
| D3 | Phạm vi | **Toàn bộ MVP theo SPEC-01..07**. Code .NET chỉ dùng để tham khảo, không port y nguyên |
| D4 | Redis | **Bỏ.** Hệ thống chỉ chạy trên một VPS, nên phân phối sự kiện realtime dùng Postgres `LISTEN/NOTIFY` và hàng đợi job dùng bảng Postgres |
| D5 | Dữ liệu cũ | **Không cần migrate dữ liệu.** Repo không có cấu hình deploy hay docker-compose nào, tức là chưa có môi trường production |

### Quyết định business (chốt ngày 2026-10-15)

| # | Câu hỏi | Quyết định |
|---|---|---|
| B-1 | Người thật can thiệp | **Sale được tiếp quản.** Mỗi hội thoại có `handling_mode` là `ai` hoặc `human`. Ở chế độ `human`, AI không trả lời. Sale tiếp quản bằng tay và trả lại cho AI bằng tay. Hội thoại tự chuyển sang `human` khi có triệu chứng nặng (`SEVERITY_ESCALATED`) hoặc khi khách gửi ảnh. Điều này thay thế câu "không có human fallback" trong CONTEXT.md. |
| B-2 | Persona | **Giữ như CONTEXT.md:** không tự nhận là AI, luôn trả lời tự tin, không nói "mình không chắc chắn". Phía business chấp nhận rủi ro về chính sách Meta và quy định quảng cáo dược. |
| B-3 | Model mặc định | **Claude Sonnet 4.6** (`claude-sonnet-4-6`, $3/$15 cho mỗi 1M token vào/ra), seed vào `llm_configs` |
| B-4 | Hạn mức chi phí | **$200/tháng/tenant**, lưu trong cấu hình của tenant, admin chỉnh được |
| B-5 | Khách gửi ảnh | **Chuyển cho sale.** AI không đọc ảnh. Gửi câu trả lời tạm "Mình đã nhận được ảnh, nhân viên sẽ phản hồi bạn ngay", chuyển hội thoại sang `human` và báo sale. |
| B-6 | Hội thoại dài | **Tóm tắt cuốn chiếu.** Luôn gửi 50 tin gần nhất. Mỗi khi phần cũ dư thêm 50 tin, một job chạy nền gộp phần đó vào bản tóm tắt. Cần tin cũ cụ thể thì tìm bằng vector. |

## 2. Không mang sang: lỗi trong code .NET hiện tại

Danh sách dưới đây có được từ việc đọc code. Máy hiện tại chưa cài `dotnet` nên chưa chạy thử được. Repo Go phải thiết kế để các lỗi này không tái diễn.

| # | Lỗi | Vị trí | Hậu quả | Cách làm đúng trong Go |
|---|---|---|---|---|
| B1 | Mật khẩu băm bằng SHA-256, không có salt | `AuthService.cs:91` | Lộ DB là dò ngược được mật khẩu bằng bảng tra sẵn. SPEC-01 yêu cầu PBKDF2. | argon2id (`golang.org/x/crypto/argon2`) |
| B2 | `TenantAccessor` đăng ký kiểu Singleton | `Program.cs:32` | Mọi request dùng chung một tenant ID. Khi có hai request đồng thời, request này có thể đọc dữ liệu của tenant kia. | Đưa tenant vào `context.Context` của từng request, kết hợp Row-Level Security (mục 5.3) |
| B3 | Login không chạy được | `AppDbContext.cs:22`, `ITenantAccessor.cs:17` | Constructor của DbContext đọc `TenantId`. Request login chưa có JWT nên getter ném exception, và `AuthService` không khởi tạo được. | Các truy vấn auth chạy ngoài phạm vi tenant, qua một role DB riêng |
| B4 | Policy `"admin"` chưa được đăng ký | `KbEndpoints.cs:17`, `Program.cs:66` | Mọi endpoint ghi KB bị lỗi 500 vì không tìm thấy policy | Middleware kiểm tra quyền (permission) rõ ràng, có test |
| B5 | `/api/chat/conversations/inbound` lấy `tenantId` từ body | `ChatEndpoints.cs:19` | Bất kỳ user đã đăng nhập nào cũng chèn được tin nhắn vào tenant khác | Tin nhắn vào chỉ đi qua webhook Meta đã xác thực HMAC. Tenant suy ra từ page ID. |
| B6 | `/api/auth/register` cho phép truy cập ẩn danh | `AuthEndpoints.cs:17` | Ai biết slug của tenant cũng tự đăng ký vào được | Không có đăng ký công khai. Admin tạo user, và có lệnh CLI để seed admin đầu tiên. |
| B7 | Hub cho phép join bất kỳ conversation nào | `ChatHub.cs:9` | Không kiểm tra tenant, nên nghe lén được hội thoại của tenant khác | Phòng WebSocket gắn với tenant, kiểm tra quyền sở hữu trước khi cho join |
| B8 | Deploy KB không tạo embedding | `KbService.cs:158-183` | Chỉ đổi trạng thái version, RAG không có dữ liệu để tìm | Deploy tạo một job: chia đoạn, tạo embedding, ghi `kb_chunks`, rồi mới kích hoạt version |
| B9 | Chi phí LLM chỉ lưu trong RAM, không bao giờ reset | `CostTracker.cs` | Restart là mất số liệu. Hạn mức tính trên toàn bộ thời gian chạy, không theo tháng. | Bảng `llm_usage` cộng dồn theo tenant và tháng |
| B10 | Model và giá hardcode, số token ước lượng bằng số ký tự chia 4 | `ClaudeChatClient.cs:42,46` | Sai yêu cầu "lấy model từ `llm_configs`", chi phí tính sai | Lấy model và giá từ `llm_configs`, số token lấy từ `Usage` trong response |
| B11 | Xóa vector chỉ xóa 100 đoạn đầu | `KbDeployService.cs:96` | Version có hơn 100 đoạn sẽ để lại vector rác | `DELETE FROM kb_chunks WHERE kb_version_id = $1` |
| B12 | Mật khẩu `sa` và JWT key nằm trong `appsettings.json` | `appsettings.json:10,14` | Bí mật bị commit vào git | Chỉ đọc từ biến môi trường. Có `.env.example`, không có giá trị thật. |
| B13 | Test dùng EF InMemory | `ChatServiceTests.cs:19` và các file khác | Sai yêu cầu "test với DB thật" trong constitution | testcontainers-go với ảnh `pgvector/pgvector` |

## 3. Kiến trúc đích

```
           Meta (Messenger/IG)                 Frontend (repo riêng)
                 │ webhook                          │ REST + WebSocket
                 ▼                                  ▼
        ┌───────────────────────── cmd/api ─────────────────────────┐
        │ HTTP router · xác thực JWT · tenant ctx · WS hub ·        │
        │ webhook Meta · Meta Send API · job worker (inbound/send)  │
        └──────────────┬──────────────────────────────┬─────────────┘
                       │ gRPC (ChatAgent.Reply)        │ SQL / LISTEN-NOTIFY
                       ▼                               ▼
        ┌──────── cmd/agent ────────┐        ┌──────── PostgreSQL ────────┐
        │ pipeline AI · compliance ·│◄──────►│ dữ liệu + pgvector +       │
        │ RAG · memory · KB embed · │  SQL   │ hàng đợi job (River)       │
        │ job tự học (periodic)     │        └────────────────────────────┘
        └──────────┬────────────────┘
                   ▼
        Anthropic API · OpenAI Embeddings
```

**Luồng một tin nhắn đến:**
1. `api` nhận `POST /api/webhooks/meta` và xác thực HMAC trên body gốc.
2. Ghi `messages` (direction=in), dùng `mid` của Meta làm khóa chống trùng.
3. Tạo job `ProcessInbound`, trả 200 ngay. Meta yêu cầu phản hồi trong 20 giây.
4. NOTIFY để WebSocket đẩy tin nhắn mới lên inbox của sale.
5. Worker trong `api` lấy job. Nếu hội thoại đang ở `human` thì chỉ báo sale, không gọi AI. Nếu tin có ảnh thì làm theo B-5. Còn lại thì gửi WS `TypingIndicator` rồi gọi `agent.Reply` qua gRPC.
6. Ghi câu trả lời của AI, gọi Meta Send API (thử lại 3 lần, có backoff), rồi NOTIFY.

**Phân chia trách nhiệm:**
- `api` sở hữu mọi thứ đối ngoại: HTTP, WebSocket, Meta.
- `agent` sở hữu mọi thứ liên quan tới AI: LLM, embedding, compliance, tự học. Chỉ `agent` giữ API key của Anthropic và OpenAI.
- Cả hai dùng chung DB nhưng qua package truy vấn riêng. `agent` dùng role DB có quyền hẹp hơn.

## 4. Tech stack

| Hạng mục | Chọn | Ghi chú |
|---|---|---|
| Ngôn ngữ | Go bản stable mới nhất | Ghi phiên bản trong `go.mod` và ảnh Docker |
| HTTP router | `go-chi/chi/v5` | Tương thích `net/http`, nhiều middleware có sẵn |
| DB driver | `jackc/pgx/v5` + `pgxpool` | |
| Truy vấn | `sqlc` | SQL viết tay, sinh code Go có kiểu. Thay cho EF Core. |
| Migration | `pressly/goose` | Đánh số `00001_init.sql`… và nhúng vào binary |
| Vector | `pgvector/pgvector-go` | Xem lưu ý về số chiều ở mục 5.2 |
| Hàng đợi job | `riverqueue/river` | Lưu trên Postgres, có periodic job, retry và chống trùng |
| gRPC | `google.golang.org/grpc` + `buf` | Proto đặt trong `proto/pharmachat/agent/v1` |
| WebSocket | `coder/websocket` | Thay SignalR |
| JWT | `golang-jwt/jwt/v5` | |
| Mật khẩu | `golang.org/x/crypto/argon2` | argon2id |
| Mã hóa bí mật | `crypto/aes` + `crypto/cipher` (GCM) | AES-256-GCM cho API key và token Meta. Khóa lấy từ biến môi trường. |
| LLM | `github.com/anthropics/anthropic-sdk-go` | Lấy số token từ `resp.Usage`. Cache system prompt bằng `CacheControl`. |
| Embedding | `github.com/openai/openai-go` | `text-embedding-3-large` |
| Log | `log/slog` (JSON) | Có bước lọc PII trước khi ghi log |
| Config | `caarlos0/env` | Chỉ đọc từ biến môi trường |
| Rate limit | `golang.org/x/time/rate` | Giới hạn theo tenant cho `/api/*`, theo IP cho `/api/webhooks/*` |
| Test | `testing` + `testify` + `testcontainers-go` | |
| Lint | `golangci-lint` | Chạy trong CI. Thay cho `TreatWarningsAsErrors`. |
| Reverse proxy | Caddy | TLS tự động, bật TLS 1.3 |

**Về model LLM:** dùng `claude-sonnet-4-6` theo B-3. Model và giá được đọc từ `llm_configs`, nên muốn đổi model chỉ cần sửa dòng cấu hình, không phải sửa code. Với Sonnet 4.6, dùng adaptive thinking (`thinking: {type: "adaptive"}`), không dùng `budget_tokens` vì tham số này đã deprecated. Riêng bước phân loại intent khi không chắc, nên tắt thinking để giữ độ trễ thấp.

## 5. Dữ liệu

### 5.1 Chuyển DDL từ SQL Server sang Postgres

Viết lại `0001_init.sql` thành `migrations/00001_init.sql`:

| SQL Server | Postgres |
|---|---|
| `UNIQUEIDENTIFIER DEFAULT NEWID()` | `uuid DEFAULT gen_random_uuid()` |
| `NVARCHAR(n)` / `NVARCHAR(MAX)` | `text`, giới hạn độ dài bằng `CHECK` khi cần |
| `DATETIMEOFFSET` / `SYSDATETIMEOFFSET()` | `timestamptz` / `now()` |
| `BIT` | `boolean` |
| `settings_json NVARCHAR(MAX)` | `jsonb` |
| Enum lưu dạng chuỗi | `text` + `CHECK (status IN (...))` |
| Partial index `WHERE ...` | Giữ nguyên, Postgres có hỗ trợ |

Bảng và cột **cần thêm** (SPEC có yêu cầu nhưng DDL hiện tại chưa có):

| Bảng / cột | Phục vụ |
|---|---|
| `kb_chunks (id, tenant_id, kb_version_id, module_code, chunk_index, content, embedding)` | RAG (SPEC-02/03), thay Qdrant |
| `contact_memories.embedding` | Tìm trí nhớ về khách bằng vector (SPEC-03) |
| `message_embeddings` hoặc cột `messages.embedding` | Tìm trong lịch sử chat (CONTEXT §10) |
| `conversations.compliance_level` | `normal`, `disclaimer_required` hoặc `escalated` (SPEC-05) |
| `conversations.handling_mode`, `assigned_to`, `handoff_reason`, `handed_off_at` | Sale tiếp quản (B-1). `handoff_reason` nhận `manual`, `severe_symptom` hoặc `image`. |
| `conversation_summaries (conversation_id, summary, covered_until_message_id)` | Tóm tắt cuốn chiếu (B-6) |
| `tenants.settings_json` → `monthly_llm_cap_usd` (mặc định 200) | Hạn mức chi phí (B-4) |
| `refresh_tokens (id, user_id, token_hash, expires_at, revoked_at)` | Refresh token, logout (SPEC-01) |
| `kb_suggestions` | Dữ liệu cho chế độ tự học (DESIGN §6) |
| `llm_usage (tenant_id, month, input_tokens, output_tokens, usd_cost)` | Hạn mức chi phí theo tháng (SPEC-03) |
| `tenants.settings_json` → `self_learning_mode` | Chế độ duyệt tự động hay admin duyệt (SPEC-07) |
| Bảng job của River | Sinh bằng migration của River |

### 5.2 Lưu ý về pgvector

`text-embedding-3-large` sinh vector **3072 chiều**. Kiểu `vector` của pgvector chỉ đánh index HNSW/IVFFlat được tới 2000 chiều, nên không đánh index trực tiếp được. Có hai cách:

- **Cách A (khuyến nghị):** cột `halfvec(3072)` và index HNSW `halfvec_cosine_ops`. `halfvec` index được tới 4000 chiều, cần pgvector 0.7 trở lên. Giữ nguyên chất lượng embedding, mỗi vector tốn một nửa dung lượng.
- **Cách B:** gọi OpenAI với `dimensions: 1536` rồi dùng `vector(1536)`. Nhẹ hơn nhưng độ chính xác giảm một chút.

Dữ liệu KB nhỏ (5 module), nên cách A đủ nhẹ cho VPS 8GB. `embedding_configs.dimension` phải khớp với kiểu cột. Khi đổi số chiều phải tạo lại embedding cho toàn bộ dữ liệu, nên cần có lệnh `reembed`.

### 5.3 Cô lập tenant

Có hai lớp, để lỗi kiểu B2 không thể lặp lại:

1. **Ở tầng ứng dụng:** middleware đưa `tenantID` vào `context.Context`. Mọi truy vấn sqlc trên bảng có tenant đều nhận `tenant_id` làm tham số.
2. **Ở tầng DB, bằng Row-Level Security:** mỗi request chạy trong một transaction có `SET LOCAL app.tenant_id = '...'`, và policy `USING (tenant_id = current_setting('app.tenant_id')::uuid)` áp cho mọi bảng có tenant. Role DB của ứng dụng không phải owner nên không bỏ qua được RLS. Các truy vấn login và webhook chạy trước khi biết tenant, nên đi qua một role riêng chỉ được đọc đúng các bảng cần thiết.

Có một test bắt buộc: tenant A không đọc được dữ liệu của tenant B ở mọi repository, kể cả khi quên truyền `tenant_id`.

Soft delete: các bảng aggregate có `deleted_at`. Các view hoặc truy vấn sqlc mặc định lọc `deleted_at IS NULL`.

## 6. Cấu trúc repo

```
pharmachat-backend/
├── cmd/
│   ├── api/            # main của service api
│   ├── agent/          # main của service agent
│   └── pcctl/          # CLI: migrate, seed, tạo admin, reembed
├── internal/
│   ├── platform/       # config, db (pgxpool, tx + RLS), log, crypto, httpx (lỗi chuẩn)
│   ├── auth/           # login, refresh, argon2, JWT, middleware quyền
│   ├── tenant/
│   ├── kb/             # module, version, upload, deploy (tạo job)
│   ├── inbox/          # conversation, message, contact, memory (đọc)
│   ├── realtime/       # WS hub, LISTEN/NOTIFY
│   ├── meta/           # oauth, webhook (HMAC), send api, đồng bộ page
│   ├── admin/          # dashboard, llm/embedding config, tự học
│   ├── agent/          # (chỉ cmd/agent dùng) pipeline, intent, rag, memory, llm
│   ├── compliance/     # quy tắc, phát hiện, log. Dùng trong agent.
│   ├── jobs/           # định nghĩa job River
│   └── store/          # code sqlc sinh ra + truy vấn .sql
├── proto/pharmachat/agent/v1/agent.proto
├── migrations/
├── api/openapi.yaml    # hợp đồng API cho repo frontend
├── deploy/             # docker-compose.yml, Caddyfile, .env.example
├── .sdd/               # SPEC, TASKS và constitution chuyển sang, cập nhật cho Go
├── Makefile
└── CLAUDE.md
```

Quy tắc: package nghiệp vụ không import package nghiệp vụ khác. Dùng chung chỉ qua `platform` hoặc qua interface. Constitution tương ứng cần sửa Article 1 (tech stack) và Article 2 (layout) cho Go.

## 7. Các giai đoạn triển khai

Mỗi giai đoạn phải đạt **điều kiện xong** thì mới sang giai đoạn sau. Số task đánh lại thành `GO-xx` và ghi nguồn SPEC.

### Giai đoạn 0: Nền móng

| Task | Nội dung |
|---|---|
| GO-00 | **Tạo repo GitHub mới, tách biệt với repo .NET hiện tại** (tạm đặt tên `pharmachat-backend`, chế độ private). Nhánh mặc định đặt tên là **`master`**. Bảo vệ `master`: bắt buộc qua PR, CI xanh và 1 người review, cấm force-push (constitution Art. 4 và 7). Khai báo GitHub Actions secrets cho CI. Chép thư mục `.sdd/` sang repo mới rồi cập nhật cho Go: constitution Art. 1–2, Art. 4 (đổi `main` thành `master`, kể cả trong AGENTS.md), CONTEXT.md và SPEC-03 theo B-1, đóng các câu hỏi mở theo B-5 và B-6. Kèm luôn file kế hoạch này. Repo .NET cũ chuyển sang archive (chỉ đọc), README trỏ sang repo mới. |
| GO-01 | Khởi tạo khung code trong repo mới: `go.mod`, cấu trúc thư mục theo mục 6, Makefile (`build`, `test`, `lint`, `gen`, `migrate`), golangci-lint |
| GO-02 | `platform`: config đọc từ env, slog JSON có lọc PII, lỗi chuẩn `{errorCode, message, requestId}`, middleware request ID, recover |
| GO-03 | Migration Postgres đầy đủ (mục 5.1 và 5.2), cấu hình sqlc, RLS policy |
| GO-04 | `deploy/docker-compose.yml`: `postgres` (ảnh pgvector), `api`, `agent`, `caddy`. Có healthcheck và volume. |
| GO-05 | CI (GitHub Actions): lint, test với testcontainers, build ảnh Docker. Chặn merge khi đỏ. |
| GO-06 | `pcctl`: `migrate`, `seed` (tenant Huyền Sống Khỏe, các role, admin, `llm_configs`), `create-admin` |

**Điều kiện xong:** repo mới đã có trên GitHub với nhánh `master` được bảo vệ. `make up` dựng được cả hệ thống, `/health` của hai service trả OK, CI xanh.

### Giai đoạn 1: Auth và multi-tenant (SPEC-01)

| Task | Nội dung |
|---|---|
| GO-07 | `POST /api/auth/login`: argon2id, khóa tài khoản sau 5 lần sai, JWT 15 phút chứa `user_id`, `tenant_id`, `role` |
| GO-08 | `POST /api/auth/refresh` và `POST /api/auth/logout`: refresh token xoay vòng, lưu dạng hash, cookie httpOnly Secure SameSite |
| GO-09 | Middleware tenant (context + `SET LOCAL`) và middleware quyền (`admin:system` và các quyền khác) |
| GO-10 | Test cô lập tenant (mục 5.3) và test truy cập chéo tenant phải trả 403 |

**Điều kiện xong:** login, refresh, logout chạy được. Test chéo tenant pass. Không có endpoint đăng ký công khai.

### Giai đoạn 2: Knowledge Base (SPEC-02)

| Task | Nội dung |
|---|---|
| GO-11 | CRUD module và version (Markdown), upload .txt/.md (tối đa 10MB, kiểm tra UTF-8) |
| GO-12 | Deploy: tạo job `EmbedKbVersion`. Agent chia đoạn (logic giống `ChunkContent` hiện tại: tách theo đoạn văn, khoảng 1000 ký tự, cắt ở khoảng trắng), tạo embedding theo lô có retry, ghi `kb_chunks`, rồi **trong một transaction** đánh dấu version là deployed và archive version cũ |
| GO-13 | Rollback: kích hoạt lại một version cũ, embedding của version đó vẫn còn trong `kb_chunks` |
| GO-14 | Theo dõi trạng thái deploy (`deploying`, `deployed`, `failed`) và đẩy qua WS cho admin |

**Điều kiện xong:** deploy một version 50KB xong trong dưới 30 giây (NFR-09). Truy vấn vector trả đúng đoạn văn của version đang active.

### Giai đoạn 3: Agent service (SPEC-03)

| Task | Nội dung |
|---|---|
| GO-15 | `agent.proto`: `Reply` (unary) và `StreamReply` (server stream, để dành). Request chứa `tenant_id`, `conversation_id`, `message_id`. Agent tự nạp ngữ cảnh từ DB, không nhận lịch sử qua gRPC. |
| GO-16 | Client LLM: đọc `llm_configs` đang active (giải mã API key), Anthropic SDK, timeout, thử lại 2 lần với backoff khi gặp 429/5xx, cache system prompt, ghi `Usage` vào `llm_usage` |
| GO-17 | Ngữ cảnh: 50 tin gần nhất cộng `conversation_summaries`, trí nhớ về khách (vector + loại), top-5 đoạn KB theo module phù hợp với intent. Job `SummarizeConversation` gộp thêm 50 tin cũ mỗi khi phần chưa tóm tắt vượt 50 tin (B-6). |
| GO-18 | Pipeline: che PII → phân loại intent (từ khóa trước, LLM khi không chắc) → kiểm tra compliance trước khi gọi LLM → RAG → gọi LLM → lọc compliance sau khi có câu trả lời → ghi `agent_sessions`/`agent_traces` (chỉ thêm, không sửa) |
| GO-19 | System prompt lấy từ `kb_versions` (Article 6 của constitution), có prompt mặc định khi chưa có version nào. Persona theo B-2. Vì AI không được nói "không chắc chắn", prompt phải có hướng xử lý khi KB không có thông tin: chuyển hướng tự nhiên (hỏi thêm nhu cầu, giới thiệu sản phẩm có trong KB), không tự bịa thành phần, liều dùng hay giá. Bộ test GO-23 phải có các câu hỏi nằm ngoài KB để đo việc này. |
| GO-20 | Hạn mức chi phí: đọc `monthly_llm_cap_usd` (mặc định $200, B-4), cảnh báo ở 80%, dừng ở 100% theo tháng. Lấy giá từ `llm_configs`. Khi AI dừng vì hạn mức, hội thoại mới chuyển sang `human` để sale xử lý. |
| GO-21 | Dự phòng theo bảng lỗi của SPEC-03: không kết nối được vector thì tìm bằng từ khóa (Postgres FTS), LLM lỗi thì trả câu "Vui lòng thử lại" |
| GO-22 | Trích và cập nhật trí nhớ về khách sau mỗi lượt trả lời (job chạy riêng, không làm chậm câu trả lời) |
| GO-23 | Bộ test đánh giá độ chính xác: chạy `kb_test_cases` để đo NFR-05 (≥ 85%) |

**Điều kiện xong:** gọi `Reply` qua `grpcurl` nhận được câu trả lời có căn cứ từ KB. p95 dưới 5 giây trên bộ test.

### Giai đoạn 4: Compliance (SPEC-05)

| Task | Nội dung |
|---|---|
| GO-24 | Package `compliance`: 4 quy tắc `DISCLAIMER_REQUIRED`, `RX_BLOCKED`, `SEVERITY_ESCALATED`, `CURE_CLAIM_BLOCKED`. Danh sách từ khóa nằm trong KB-04, không hardcode. |
| GO-25 | `CURE_CLAIM_BLOCKED`: phát hiện trong câu trả lời thì sinh lại (tối đa 1 lần), sau đó thay bằng câu an toàn |
| GO-26 | Ghi `compliance_logs` bất đồng bộ (nếu ghi lỗi thì vẫn gửi câu trả lời, thử lại sau) và cập nhật `conversations.compliance_level`. Gặp `SEVERITY_ESCALATED` thì gửi câu "Vui lòng liên hệ bác sĩ ngay", chuyển hội thoại sang `human` (`handoff_reason = severe_symptom`) và báo sale qua WS (B-1). |
| GO-27 | Bộ test bảng: mỗi quy tắc có các ca nên kích hoạt và không nên kích hoạt. Kiểm tra p95 dưới 500ms (NFR-08). |

**Điều kiện xong:** 100% câu trả lời nhắc tới sản phẩm có disclaimer (NFR-13), đo trên bộ test.

### Giai đoạn 5: API inbox và realtime (SPEC-06)

| Task | Nội dung |
|---|---|
| GO-28 | `GET /api/inbox/conversations` (lọc theo trạng thái và kênh, phân trang cursor, **một truy vấn duy nhất** thay cho N+1 như `ChatService.GetConversationsAsync` hiện tại), `GET /api/inbox/conversations/{id}`, `GET /api/inbox/conversations/{id}/memory` |
| GO-29 | WS `/ws/inbox`: xác thực bằng JWT khi kết nối, phòng theo tenant và theo conversation, sự kiện `ReceiveMessage`, `TypingIndicator`, `NewConversation` (giữ tên sự kiện như SPEC-04 §5.3) |
| GO-30 | Phân phối sự kiện qua Postgres `LISTEN/NOTIFY` để worker và api cùng đẩy được sự kiện, và để sau này chạy được nhiều instance |
| GO-30a | Tiếp quản (B-1): `POST /api/inbox/conversations/{id}/takeover`, `POST .../release` (trả lại cho AI), `POST .../messages` (sale gửi tin, chỉ được khi đang `human`, gửi ra qua Meta Send API). Sự kiện WS `HandoffChanged`. Bộ lọc "Tôi xử lý" lọc theo `handling_mode = human AND assigned_to = user hiện tại`. |

**Điều kiện xong:** tải danh sách p95 dưới 500ms với 10.000 hội thoại seed sẵn (NFR-03). Sự kiện tới client trong dưới 1 giây.

### Giai đoạn 6: Tích hợp Meta (SPEC-04)

| Task | Nội dung |
|---|---|
| GO-31 | `POST /api/admin/meta/config`, luồng OAuth (`connect`, `callback`, `meta_oauth_states` chống CSRF), lưu System User Token đã mã hóa AES-GCM |
| GO-32 | `GET /api/webhooks/meta` (xác thực verify token) và `POST /api/webhooks/meta`: đọc body gốc, so `X-Hub-Signature-256` bằng `hmac.Equal`, chống trùng theo `mid`, ánh xạ page ID sang tenant, tạo job |
| GO-32a | Tin có đính kèm ảnh (B-5): lưu metadata của ảnh (URL Meta, không tải về), gửi câu trả lời tạm, chuyển hội thoại sang `human` (`handoff_reason = image`), báo sale |
| GO-33 | Send API: worker gửi tin, thử lại 3 lần với backoff, chờ 30 giây khi gặp 429, ghi `message_id` của Meta. Dùng chung cho tin của AI và tin của sale. |
| GO-34 | Đồng bộ page (`meta_assets`), `validate`, `status`, ngắt kết nối. Job chạy mỗi 6 giờ kiểm tra token (NFR-12); token lỗi thì chuyển `reconnect_required` và báo admin qua WS. |
| GO-35 | Test bằng payload webhook mẫu của Meta, và kiểm thử end-to-end với một page test thật |

**Điều kiện xong:** nhắn tin trên page test, nhận được câu trả lời của AI trên Messenger trong 3–5 giây.

### Giai đoạn 7: API admin (SPEC-07)

| Task | Nội dung |
|---|---|
| GO-36 | CRUD user và phân role (thay cho đăng ký công khai) |
| GO-37 | API cấu hình LLM và embedding: lưu key dạng AES-256-GCM, chỉ trả về dạng che (`sk-...abcd`). `POST .../test` để thử kết nối. Đổi cấu hình là áp dụng ngay (agent đọc cấu hình mỗi request, có cache ngắn và bị xóa qua NOTIFY). |
| GO-38 | `GET /api/admin/dashboard`: số liệu tổng hợp từ các bảng có sẵn |
| GO-39 | `POST /api/admin/self-learning/toggle` |

### Giai đoạn 8: Tự học (DESIGN §6)

| Task | Nội dung |
|---|---|
| GO-40 | Job River chạy mỗi 24 giờ trong `agent`: gom câu hỏi 24 giờ qua theo intent và embedding, tìm câu hỏi lặp lại trên 3 lần mà độ tương đồng với KB thấp, sinh bản nháp FAQ, ghi `kb_suggestions` |
| GO-41 | API duyệt hoặc bỏ gợi ý. Ở chế độ tự duyệt thì gộp vào bản nháp mới của KB-02 và deploy. |

### Giai đoạn 9: Vận hành và go-live

| Task | Nội dung |
|---|---|
| GO-42 | Job lưu trữ: xóa nội dung tin nhắn gốc sau 30 ngày, chỉ giữ hash để audit (Article 3), và xóa `compliance_logs` sau 90 ngày (NFR-14) |
| GO-43 | Rate limit, HTTPS/TLS 1.3 qua Caddy, header bảo mật |
| GO-44 | Backup: `pg_dump` hằng ngày ra nơi lưu trữ ngoài VPS, có tài liệu khôi phục và đã khôi phục thử ít nhất một lần |
| GO-45 | Seed dữ liệu thật: sản phẩm Boganic Premium, FAQ, chính sách, quy tắc compliance |
| GO-46 | Test E2E: webhook giả lập → AI → Send API giả lập → sự kiện WS |
| GO-47 | `api/openapi.yaml` hoàn chỉnh và `CLAUDE.md` cho repo Go |

## 8. Hợp đồng API cho repo frontend

Vì frontend sẽ tách repo, backend phải coi **`api/openapi.yaml` là hợp đồng chính thức**:
- Viết dần trong các giai đoạn 1–7, có kiểm tra trong CI rằng route thực tế khớp với spec.
- Đường dẫn theo SPEC: `/api/auth/*`, `/api/kb/*`, `/api/inbox/*`, `/api/admin/*`. Code .NET hiện dùng `/api/chat/*`, khác với SPEC-06. Repo Go dùng `/api/inbox/*`.
- Danh mục sự kiện WebSocket có tài liệu riêng (`api/websocket.md`).
- Repo frontend sẽ sinh client từ file này. Phần đó thuộc kế hoạch frontend.

## 9. Chiến lược test

- **Unit:** compliance, chia đoạn, intent, che PII, mã hóa, HMAC. Dùng test dạng bảng, không gọi mạng.
- **Tích hợp:** mọi truy vấn sqlc, RLS và job chạy trên Postgres thật (testcontainers).
- **Hợp đồng:** gRPC kiểm tra bằng `buf breaking`, HTTP kiểm tra theo OpenAPI.
- **LLM:** dùng interface và client giả cho unit test. Bộ đánh giá ở GO-23 gọi API thật, chạy thủ công hoặc theo lịch vì tốn tiền.
- Mỗi bug fix kèm một test hồi quy (Article 5).

## 10. Rủi ro

Tất cả câu hỏi business đã được chốt ở mục 1 (B-1 tới B-6). Khi chuyển `.sdd/` sang repo Go, cần cập nhật CONTEXT.md §1, §5 và SPEC-03 cho khớp với B-1 (sale được tiếp quản), cùng các câu hỏi mở đã đóng trong SPEC-03 và SPEC-05.

| Rủi ro | Cách giảm |
|---|---|
| Persona B-2 bị Meta hoặc cơ quan quản lý xử lý | Phía business đã chấp nhận rủi ro này. Persona nằm trong system prompt lấy từ `kb_versions`, nên đổi được ngay mà không cần deploy lại code. |
|---|---|
| Meta App Review kéo dài, chậm giai đoạn 6 | Nộp App Review ngay khi bắt đầu giai đoạn 0. Phát triển bằng page test ở chế độ dev. |
| Hai service làm tăng độ phức tạp khi deploy | Một docker-compose, healthcheck, và `api` vẫn chạy khi `agent` sập (job tự retry) |
| RAM của VPS 8GB | Đã bỏ SQL Server, Qdrant và Redis. Postgres giới hạn khoảng 2GB, hai service Go dưới 200MB. |

## 11. Thứ tự và phụ thuộc

```
GĐ0 → GĐ1 → GĐ2 ─┬→ GĐ3 → GĐ4 ─┬→ GĐ6 → GĐ9
                  └→ GĐ5 ────────┘   GĐ7 → GĐ8 ↗
```

- GĐ5 (inbox) làm song song với GĐ3–4 được vì chỉ phụ thuộc dữ liệu.
- GĐ7 có thể bắt đầu ngay sau GĐ1, trừ phần cấu hình LLM phải chờ GĐ3.
- Repo .NET đóng băng (chỉ đọc) từ GO-00 và được archive trên GitHub. Không cần chạy song song hai hệ thống vì chưa có production.
