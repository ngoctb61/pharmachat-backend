---
name: start-task
description: Bắt đầu một task của kế hoạch PharmaChat theo issue Jira (ví dụ "/start-task PHA-21" hoặc mã cũ "/start-task GO-07"). Đọc issue và SPEC trên Atlassian, tạo nhánh đúng quy ước, viết test trước, rồi chuyển trạng thái issue trên Jira. Dùng khi người dùng muốn làm một task hoặc hỏi "làm task tiếp theo".
---

# Bắt đầu task

Dữ liệu nằm trên Atlassian (site `pharmachat.atlassian.net`), đọc và ghi qua MCP `atlassian`:
- Task: Jira project `PHA`. Mỗi giai đoạn là một epic; issue con có tiêu đề `GO-xx · ...`. Phụ thuộc giữa các giai đoạn là link "blocks".
- SPEC, constitution, kế hoạch: Confluence space `SD`, dưới trang "PharmaChat — Tài liệu dự án".

Đầu vào: key issue (`PHA-21`) hoặc mã cũ (`GO-07`, tìm bằng JQL `project = PHA AND summary ~ "GO-07"`). Nếu không có, tìm issue đầu tiên chưa xong mà epic cha không còn bị chặn: `project = PHA AND issuetype = Task AND statusCategory != Done ORDER BY key ASC`. Xác nhận với người dùng trước khi làm.

1. **Đọc yêu cầu**
   - Mô tả issue, cùng điều kiện xong và các link "is blocked by" của epic cha (`getJiraIssue` với `view: "evidence"`).
   - SPEC liên quan, mở qua web link của epic (giai đoạn 1 → SPEC-01, 2 → SPEC-02, 3 → SPEC-03, 4 → SPEC-05, 5 → SPEC-06, 6 → SPEC-04, 7 → SPEC-07).
   - Các quyết định B-1..B-6 ở mục 1 trang "Kế hoạch migrate backend PharmaChat sang Go" nếu task chạm tới chúng.
   - Nếu epic chặn epic cha của task chưa Done, hoặc issue có link "is blocked by" chưa xong, dừng lại và báo.

2. **Tạo nhánh** từ `master` mới nhất: `git switch master && git pull --ff-only && git switch -c feat/PHA-n-<scope>` (dùng `fix/` nếu phù hợp). Chuyển issue sang In Progress.

3. **Nêu kế hoạch ngắn**: file sẽ thêm hoặc sửa, migration (nếu có), test sẽ viết. Nếu SPEC và issue mâu thuẫn, hỏi người dùng trước khi code.

4. **Viết test trước**, chạy để thấy test fail đúng lý do, rồi mới implement. Có migration thì làm theo skill `new-migration`.

5. **Kiểm tra**: `make lint` và `make test` phải sạch. Task có thay đổi hạ tầng hoặc endpoint thì chạy thêm skill `verify-stack`.

6. **Hoàn tất**:
   - Commit theo conventional commits, có key `PHA-n` trong tiêu đề.
   - Push nhánh (không push `master`) và soạn mô tả PR theo `.github/pull_request_template.md`: điền bằng chứng kiểm chứng thật, xóa các mục không áp dụng. Chỉ tạo PR khi người dùng đồng ý.
   - Khi đã tạo PR, chuyển issue sang In Review và thêm comment tiếng Việt ghi bằng chứng ngắn (test nào chứng minh điều kiện xong) kèm link PR. Chỉ chuyển sang Done khi PR đã merge.
