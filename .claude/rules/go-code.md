---
paths:
  - "**/*.go"
---

# Quy tắc khi viết code Go

- **Truy vấn có tenant** phải chạy trong `db.WithTenant(ctx, pool, tenantID, fn)`. Không bao giờ truy vấn bảng tenant trực tiếp trên pool của service: RLS sẽ trả về 0 dòng và lỗi rất khó nhận ra.
- **Lỗi trả cho client** dùng `*httpx.Error` (có `Status`, `Code`, `Message`) và ghi bằng `httpx.WriteError`. `Message` viết tiếng Việt không dấu, không chứa chi tiết nội bộ. Mọi lỗi khác tự thành 500 `INTERNAL`.
- **Bọc lỗi** bằng `fmt.Errorf("<đang làm gì>: %w", err)` để log nêu đúng nguyên nhân. Không nuốt lỗi.
- **Log**: nhận `*slog.Logger` qua tham số hoặc struct, không dùng logger global. Không đưa số điện thoại, email, nội dung tin nhắn, token hay API key vào attribute. `logx` có lọc nhưng chỉ là lớp dự phòng.
- **Config** chỉ đọc từ biến môi trường qua `internal/platform/config`. Biến bắt buộc dùng tag `notEmpty`, không dùng `required` (tag `required` chấp nhận chuỗi rỗng).
- **Bí mật lưu DB** (API key, token Meta) mã hóa bằng `crypto.Sealer` trước khi ghi.
- **Model và giá LLM** đọc từ `llm_configs`, không hardcode.
- **Package nghiệp vụ** trong `internal/<domain>` không import package nghiệp vụ khác. Dùng chung qua `internal/platform` hoặc interface.
- **Không sửa tay** `internal/store/*.go`: đó là code sqlc sinh ra. Sửa `internal/store/queries/*.sql` rồi chạy `make gen`.

## Test

- Viết test trước (TDD), ưu tiên test dạng bảng.
- Test cần DB thật: dùng `dbtest.Run(m, &env)` trong `TestMain` và gọi `dbtest.Require(t, env)` ở đầu mỗi test để `-short` bỏ qua được. Không mock DB.
- Mỗi bug fix kèm một test hồi quy.
- Trước khi báo xong: `make lint` và `make test` phải sạch.
