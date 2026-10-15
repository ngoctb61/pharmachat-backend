# Quy trình làm việc

- **Mỗi thay đổi gắn với một task** `GO-xx` trong `.sdd/TASKS.md` (chi tiết ở `docs/MIGRATION_GO_BACKEND.md` mục 7) và SPEC liên quan trong `.sdd/specs/`. Đọc cả hai trước khi code. Khi xong task, cập nhật trạng thái trong `.sdd/TASKS.md`.
- **Phạm vi**: chỉ làm đúng task. Lỗi phát hiện ngoài phạm vi thì ghi lại và báo, không tự sửa lẫn vào.
- **Nhánh**: tách từ `master`, đặt tên `feat/<scope>`, `fix/<scope>`, `chore/<scope>`. Không push thẳng lên `master`, không force-push. Repo đang ở GitHub Free nên GitHub chưa chặn được; hook `.claude/hooks/guard-push.sh` chặn thay cho các phiên Claude.
- **Commit** theo conventional commits `<type>(<scope>): <tóm tắt>`, phần thân giải thích tại sao. Ghi mã task hoặc SPEC trong PR.
- **Quyết định nghiệp vụ** B-1..B-6 ở `docs/MIGRATION_GO_BACKEND.md` mục 1 đã chốt; không mở lại. Khi SPEC và code mâu thuẫn, hỏi lại thay vì tự chọn.
- **Bí mật**: không đọc, in ra hay commit `deploy/.env`. Giá trị mẫu chỉ nằm trong `deploy/.env.example`.
