# Quyết định 02 · Tra user khi login lúc chưa biết tenant, dưới RLS
- Bối cảnh:
  - `users` bật RLS theo `app.tenant_id`. Lúc login chưa biết tenant, nên role `pharmachat_app` thấy 0 dòng. Lỗi B3 của bản .NET chính là chuyện này.
  - Kế hoạch mục 5.3 ghi: truy vấn login đi qua một đường riêng, chỉ đọc đúng phần cần thiết.
  - Ràng buộc hiện có là `UNIQUE (tenant_id, email)`, nên cùng một email có thể nằm ở nhiều tenant. Trong khi đó body login chỉ có `{email, password}`.
- Các phương án:
  - A. Thêm role DB thứ hai có quyền đọc `users` và `BYPASSRLS`, kèm `AUTH_DATABASE_URL`. Ưu: đúng nguyên văn mục 5.3. Nhược:
    - phải đổi `deploy/postgres/10-app-role.sql`, mà file này chỉ chạy khi khởi tạo volume, nên volume đang có không nhận được role mới;
    - thêm một pool và một bí mật;
    - role đó đọc được mọi cột của mọi user.
  - B. Một function `SECURITY DEFINER auth_login_candidate(email)` chỉ trả `(user_id, tenant_id)`. Sau đó mọi việc chạy trong `WithTenant` bằng role app. Ưu:
    - phần vượt RLS nhỏ nhất (hai id);
    - hash mật khẩu, cờ khóa và mọi thao tác ghi vẫn nằm dưới RLS;
    - không đổi deploy.

    Nhược: thêm một function phải giữ `search_path` an toàn và `REVOKE ... FROM PUBLIC`.
  - C. Policy RLS phụ cho phép SELECT khi `app.login_email` khớp. Nhược: code nào cũng tự đặt được setting này, nên thực chất là một lối tắt vượt RLS.
- Còn chuyện email trùng ở nhiều tenant:
  - D1. Unique index toàn hệ thống `lower(email) WHERE deleted_at IS NULL`. Ưu: login luôn xác định được đúng một user, khớp hợp đồng SPEC (không có ô chọn tenant).
  - D2. Cho phép trùng, thử mật khẩu với từng user. Nhược: không rõ phải tăng bộ đếm sai của user nào; thời gian xử lý tăng theo số tenant.
- Chọn: B + D1. Đây là cách hẹp nhất đáp ứng tinh thần mục 5.3 (một đường hẹp, chỉ đọc đúng phần cần) mà không phải đổi hạ tầng deploy. D1 là hệ quả trực tiếp của hợp đồng `{email, password}`.
- Hệ quả:
  - Migration `00002_auth_login.sql` tạo index và function.
  - Sau này GO-36 tạo user ở tenant khác bằng một email đã dùng thì nhận lỗi 23505, phải báo "email đã được dùng".
  - `ErrUserExists` của `pcctl create-admin` đổi thông báo cho đúng nguyên nhân.
  - Nếu sau này cần một người thuộc nhiều tenant, phải thêm ô chọn tenant vào login. Đó là đổi hợp đồng.
