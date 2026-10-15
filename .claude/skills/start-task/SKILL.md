---
name: start-task
description: Bắt đầu một task GO-xx của kế hoạch PharmaChat (ví dụ "/start-task GO-07"). Đọc task trong docs/MIGRATION_GO_BACKEND.md và SPEC liên quan, tạo nhánh đúng quy ước, viết test trước, rồi cập nhật .sdd/TASKS.md khi xong. Dùng khi người dùng muốn làm một task GO-xx hoặc hỏi "làm task tiếp theo".
---

# Bắt đầu task GO-xx

Đầu vào: mã task (ví dụ `GO-07`). Nếu không có, đọc `.sdd/TASKS.md`, chọn task `todo` đầu tiên có các task phụ thuộc đã `done`, rồi xác nhận với người dùng trước khi làm.

1. **Đọc yêu cầu**
   - Dòng của task trong `docs/MIGRATION_GO_BACKEND.md` mục 7, kèm điều kiện xong của giai đoạn đó.
   - SPEC liên quan trong `.sdd/specs/` (giai đoạn 1 → SPEC-01, 2 → SPEC-02, 3 → SPEC-03, 4 → SPEC-05, 5 → SPEC-06, 6 → SPEC-04, 7 → SPEC-07).
   - Các quyết định B-1..B-6 ở mục 1 nếu task chạm tới chúng.
   - Kiểm tra task phụ thuộc trong `.sdd/TASKS.md`. Nếu chưa xong, dừng lại và báo.

2. **Tạo nhánh** từ `master` mới nhất: `git switch master && git pull --ff-only && git switch -c feat/<scope>` (dùng `fix/` hoặc `chore/` nếu phù hợp). Đổi task thành `doing` trong `.sdd/TASKS.md`.

3. **Nêu kế hoạch ngắn**: file sẽ thêm hoặc sửa, migration (nếu có), test sẽ viết. Nếu SPEC và kế hoạch mâu thuẫn, hỏi người dùng trước khi code.

4. **Viết test trước**, chạy để thấy test fail đúng lý do, rồi mới implement. Có migration thì làm theo skill `new-migration`.

5. **Kiểm tra**: `make lint` và `make test` phải sạch. Task có thay đổi hạ tầng hoặc endpoint thì chạy thêm skill `verify-stack`.

6. **Hoàn tất**:
   - Đổi task thành `done` trong `.sdd/TASKS.md`, ghi chú bằng chứng ngắn (test nào chứng minh điều kiện xong).
   - Commit theo conventional commits, có mã task trong tiêu đề hoặc thân.
   - Push nhánh (không push `master`) và soạn mô tả PR theo `.github/pull_request_template.md`: điền bằng chứng kiểm chứng thật, xóa các mục không áp dụng. Chỉ tạo PR khi người dùng đồng ý.
