# Quyết định 03 · Khóa tài khoản và mã lỗi của login
- Bối cảnh:
  - Issue yêu cầu khóa sau 5 lần sai. Bảng `users` có sẵn `access_failed_count` và `lockout_end`.
  - SPEC-01 §8 có ba tình huống: Invalid credentials ("Email hoac mat khau khong dung"), Account locked ("Tai khoan da bi khoa"), Tenant inactive (403).
  - User `is_active=false` không có trong ma trận lỗi.
  - Frontend gọi refresh khi gặp 401.
- Các phương án:
  - Đếm số lần sai:
    - A. Đọc rồi ghi, khóa dòng bằng `SELECT ... FOR UPDATE`. Nhược: giữ khóa dòng trong suốt lượt argon2id.
    - B. Một câu `UPDATE ... SET access_failed_count = access_failed_count + 1 ... RETURNING`. Lần thứ 5 đặt `lockout_end` và đưa bộ đếm về 0 trong cùng câu. Ưu: nguyên tử, không giữ khóa lâu.
  - Thứ tự kiểm tra:
    - A. Báo khóa, vô hiệu hóa hay tenant ngừng hoạt động trước khi kiểm mật khẩu. Nhược: ai cũng dò được trạng thái tài khoản.
    - B. Báo khóa trước, không kiểm mật khẩu, để chặn dò tiếp. Trạng thái vô hiệu hóa và tenant ngừng hoạt động chỉ báo khi mật khẩu đúng. Mật khẩu sai thì luôn trả `INVALID_CREDENTIALS`.
- Chọn: B cho cả hai. Bổ sung theo góp ý asec:
  - Trạng thái khóa được kiểm lại ngay trong câu ghi: `WHERE lockout_end IS NULL OR lockout_end <= now()`, cho cả lần thành công lẫn lần thất bại. Nếu không, các request đã đọc "chưa khóa" trước khi lần sai thứ 5 khóa tài khoản vẫn verify xong; request trúng mật khẩu sẽ xóa khóa và nhận JWT.
  - argon2id chạy ngoài transaction. Đọc là transaction 1, ghi là transaction 2, để không giữ kết nối DB trong 30-60ms băm.
- Mã lỗi:

  | errorCode | HTTP | Khi nào |
  |---|---|---|
  | `VALIDATION_ERROR` | 400 | Body không phải JSON hợp lệ, thiếu `email` hoặc `password`, hoặc quá 4 KiB |
  | `INVALID_CREDENTIALS` | 401 | Không có email, mật khẩu sai, hoặc user đã xóa mềm |
  | `ACCOUNT_LOCKED` | 403 | `lockout_end > now`, hoặc lần sai thứ 5 vừa xảy ra |
  | `ACCOUNT_DISABLED` | 403 | Mật khẩu đúng nhưng `users.is_active = false` |
  | `TENANT_INACTIVE` | 403 | Mật khẩu đúng nhưng `tenants.is_active = false` (SPEC §8) |
  | `SERVICE_BUSY` | 503 | Chờ slot argon2id quá khoảng 2 giây |

  Lý do chọn 403 cho các trường hợp khóa: cùng nhóm "liên hệ admin" với `TENANT_INACTIVE` mà SPEC đã định 403. FE phân biệt các trường hợp bằng `errorCode`.
- Hệ quả:
  - Kết quả sai mật khẩu phải được commit: transaction ghi trả về giá trị kết quả, handler mới đổi thành `*httpx.Error`.
  - Thời lượng khóa là hằng `LockoutDuration`, tạm để 15 phút, chờ trả lời câu hỏi mở trong plan.
  - Spec ghi rõ 401 của `/api/auth/login` không có nghĩa là cần refresh.
  - `Message` viết tiếng Việt không dấu theo `go-code.md`.
