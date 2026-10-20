# PharmaChat backend

Backend Go cho PharmaChat: chatbot AI tư vấn bán hàng 24/7 cho công ty dược phẩm qua Facebook Messenger và Instagram DM. Frontend nằm ở repo riêng.

- Nghiệp vụ, SPEC, quy tắc dự án và kế hoạch triển khai: Confluence space `SD`, trang [PharmaChat — Tài liệu dự án](https://pharmachat.atlassian.net/wiki/spaces/SD/pages/131076)
- Task và trạng thái: Jira project [PHA](https://pharmachat.atlassian.net/jira/software/projects/PHA/boards), mỗi giai đoạn là một epic

## Thành phần

| Binary | Vai trò |
|---|---|
| `cmd/api` | HTTP API, WebSocket, webhook Meta (cổng 8080) |
| `cmd/agent` | Pipeline AI qua gRPC (cổng 9090), `/health` ở cổng 8081 |
| `cmd/pcctl` | CLI vận hành: migration, seed, tạo admin |

Hạ tầng: PostgreSQL 17 + pgvector (dữ liệu, vector, hàng đợi job River) và Caddy (TLS 1.3).

## Chạy local

Cần Go (phiên bản trong `go.mod`) và Docker.

```bash
cp deploy/.env.example deploy/.env    # điền mật khẩu, ENCRYPTION_KEY và JWT_SIGNING_KEY theo hướng dẫn trong file
make up                               # build và chạy toàn bộ; migration tự chạy trước api/agent
make seed                             # tạo tenant Huyền Sống Khỏe, role Admin/Sale, cấu hình AI
make create-admin EMAIL=admin@example.com NAME="Quan tri"
curl -k https://localhost/health
```

## Phát triển

```bash
make test-unit   # unit test, không cần Docker
make test        # cả test tích hợp (testcontainers khởi động Postgres thật)
make lint        # golangci-lint (dùng Docker image nếu máy chưa cài đúng phiên bản)
make gen         # sinh lại code sqlc sau khi sửa migrations/ hoặc internal/store/queries/
```

## Bảo mật dữ liệu theo tenant

Mọi bảng có `tenant_id` đều bật Row-Level Security. Service kết nối bằng role `pharmachat_api` (thuộc `pharmachat_app`), nên chỉ thấy dữ liệu của tenant đã đặt qua `db.WithTenant`. Khi không đặt tenant, truy vấn trả về rỗng. `pcctl` kết nối bằng role chủ schema và bỏ qua RLS, chỉ dùng cho vận hành.
