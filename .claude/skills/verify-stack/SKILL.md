---
name: verify-stack
description: Dựng toàn bộ hệ thống PharmaChat bằng docker compose với file .env tạm, kiểm tra health, TLS, seed và tạo admin, rồi dọn sạch. Dùng để kiểm chứng thay đổi về hạ tầng, Dockerfile, compose, Caddy, migration hoặc endpoint trên stack thật.
---

# Kiểm chứng stack thật

Cần Docker và cổng 80, 443 còn trống. Nếu `deploy/.env` đã tồn tại thì **không ghi đè**: hỏi người dùng có dùng file đó không (và không đọc nội dung của nó).

1. **Tạo `.env` tạm** (chỉ khi chưa có `deploy/.env`):

   ```sh
   cat > deploy/.env <<EOF
   APP_ENV=development
   DOMAIN=localhost
   OWNER_DB_PASSWORD=$(openssl rand -hex 24)
   APP_DB_PASSWORD=$(openssl rand -hex 24)
   ENCRYPTION_KEY=$(openssl rand -base64 32)
   EOF
   ```

2. **Dựng**: `docker compose -f deploy/docker-compose.yml up -d --build`. Nếu service không healthy, xem `docker compose -f deploy/docker-compose.yml logs <service>` và báo nguyên nhân cụ thể.

3. **Kiểm tra** (đều phải đạt):
   - `docker compose -f deploy/docker-compose.yml ps`: postgres, api, agent healthy; pcctl `Exited (0)`.
   - `curl -sk https://localhost/health` trả `{"status":"ok",...}` kèm header `x-request-id`.
   - `curl -sk https://localhost/api/khong-ton-tai` trả envelope `{"errorCode":"NOT_FOUND",...}`.
   - `openssl s_client -connect localhost:443 -tls1_2` thất bại; `-tls1_3` thành công. (curl của macOS không hỗ trợ cờ `--tlsv1.3`.)
   - `docker compose -f deploy/docker-compose.yml run --rm -T pcctl seed` chạy được hai lần mà không tạo trùng.
   - Thêm kiểm tra riêng cho thay đổi đang làm (endpoint mới, migration mới...).

4. **Dọn**: `docker compose -f deploy/docker-compose.yml down -v`, rồi xóa `deploy/.env` **chỉ khi** file do bước 1 tạo ra.

5. **Báo cáo**: liệt kê từng kiểm tra kèm kết quả thật đã thấy, không tóm tắt chung chung.
