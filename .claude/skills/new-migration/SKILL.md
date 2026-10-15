---
name: new-migration
description: Thêm một migration Postgres mới cho PharmaChat (bảng, cột, index, quyền) đúng quy ước goose + RLS + sqlc, rồi sinh lại code và chạy test. Dùng khi cần thay đổi schema hoặc thêm truy vấn sqlc.
---

# Thêm migration

1. **Đặt tên file**: lấy số lớn nhất trong `migrations/`, cộng 1, đệm 5 chữ số: `migrations/00002_<mo_ta_ngan>.sql`. Không sửa file migration đã có trên `master`.

2. **Khung file**:

   ```sql
   -- +goose Up
   CREATE TABLE ... (
       id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
       tenant_id  uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
       ...
       created_at timestamptz NOT NULL DEFAULT now()
   );

   ALTER TABLE <bang> ENABLE ROW LEVEL SECURITY;
   CREATE POLICY tenant_isolation ON <bang>
       USING (tenant_id = app_current_tenant())
       WITH CHECK (tenant_id = app_current_tenant());
   GRANT SELECT, INSERT, UPDATE, DELETE ON <bang> TO pharmachat_app;

   -- +goose Down
   DROP TABLE IF EXISTS <bang>;
   ```

   - Bảng append-only: chỉ `GRANT SELECT, INSERT`.
   - Có `updated_at`: thêm trigger `CREATE TRIGGER trg_<bang>_updated_at BEFORE UPDATE ON <bang> FOR EACH ROW EXECUTE FUNCTION set_updated_at();`.
   - Khối `DO $$ ... $$` hoặc function phải nằm giữa `-- +goose StatementBegin` và `-- +goose StatementEnd`.
   - Cột vector: `halfvec(3072)` + `USING hnsw (embedding halfvec_cosine_ops)`.

3. **Truy vấn**: thêm hoặc sửa file trong `internal/store/queries/` (dùng tham số có tên `@ten`), rồi chạy `make gen`. Không sửa tay `internal/store/*.go`.

4. **Test**: viết test tích hợp cho truy vấn mới (dùng `dbtest`), gồm một ca chứng minh tenant khác không đọc được dữ liệu. Chạy `make test`. `TestUpDownUp` sẽ kiểm tra migration chạy được cả lên lẫn xuống.

5. **Kiểm tra cuối**: `make gen-check` và `make lint` sạch.
