---
paths:
  - "migrations/**"
  - "internal/store/queries/**"
  - "sqlc.yaml"
---

# Quy tắc khi sửa schema và truy vấn

- **Không sửa migration đã merge** vào `master`. Mọi thay đổi schema là một file goose mới `migrations/NNNNN_<mô tả>.sql` có cả `-- +goose Up` và `-- +goose Down`. Khối `DO $$ ... $$` và function phải bọc trong `-- +goose StatementBegin` / `StatementEnd`.
- **Bảng mới có `tenant_id`** phải bật RLS và tạo policy `tenant_isolation` giống `00001_init.sql`. Nếu quên, test `TestRLS_EveryTenantTableHasRowLevelSecurity` sẽ fail.
- **Quyền cho `pharmachat_app`**: bảng mới tạo sau migration đầu không tự có quyền. Phải `GRANT` rõ ràng. Bảng append-only (nhật ký, trace) chỉ cấp `SELECT, INSERT`.
- **Enum** lưu dạng `text` + `CHECK (... IN (...))`. Thời gian dùng `timestamptz`, khóa chính `uuid DEFAULT gen_random_uuid()`.
- **Vector** dùng `halfvec(3072)` + index HNSW `halfvec_cosine_ops`. Kiểu `vector` không đánh index được trên 2000 chiều.
- **Sau khi sửa** migration hoặc file trong `internal/store/queries/`: chạy `make gen`, rồi `make test` (test migration chạy up → down → up trên Postgres thật).
- Truy vấn sqlc cho bảng tenant phải chạy trong `db.WithTenant`. Chỉ truy vấn vận hành của `pcctl` (file `bootstrap.sql`) mới chạy bằng role chủ schema.
