# Quy trình làm việc

- **Mỗi thay đổi gắn với một issue Jira** `PHA-n` trong project PHA (https://pharmachat.atlassian.net). Mỗi giai đoạn là một epic; issue con có tiền tố `GO-xx · ` trong tiêu đề, nên tra theo mã cũ bằng JQL `project = PHA AND summary ~ "GO-07"`. Đọc mô tả issue, điều kiện xong trong epic cha và SPEC liên quan trên Confluence (space SD, trang "PharmaChat — Tài liệu dự án") trước khi code. Dùng MCP `atlassian` để đọc và chuyển trạng thái issue.
- **Phạm vi**: chỉ làm đúng task. Lỗi phát hiện ngoài phạm vi thì ghi lại và báo, không tự sửa lẫn vào.
- **Nhánh**: tách từ `master`, đặt tên `feat/PHA-n-<scope>`, `fix/PHA-n-<scope>`, `chore/<scope>`; có key `PHA-n` để Jira tự liên kết nhánh. Không push thẳng lên `master`, không force-push. Repo đang ở GitHub Free nên GitHub chưa chặn được; hook `.claude/hooks/guard-push.sh` chặn thay cho các phiên Claude.
- **Commit** theo conventional commits `<type>(<scope>): <tóm tắt> (PHA-n)`, phần thân giải thích tại sao. Ghi key `PHA-n` và SPEC trong PR.
- **Quyết định nghiệp vụ** B-1..B-6 ở trang Confluence "Kế hoạch migrate backend PharmaChat sang Go" mục 1 đã chốt; không mở lại. Khi SPEC và code mâu thuẫn, hỏi lại thay vì tự chọn.
- **Bí mật**: không đọc, in ra hay commit `deploy/.env`. Giá trị mẫu chỉ nằm trong `deploy/.env.example`.
