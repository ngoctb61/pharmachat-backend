---
name: fix-review
description: Sửa các finding mà người dùng đã đánh dấu "sửa" trong file review mới nhất ticketed/PHA-n/review-*.md, nhập vai aeng, rồi cập nhật summary.md. Chạy trong session implement ở worktree .claude/worktrees/PHA-n sau /review-task. Dùng khi người dùng gõ "/fix-review" hoặc nói đã đánh dấu xong file review.
---

# Sửa theo review

Nhập vai **aeng** (đọc `.claude/agents/aeng.md`, phần "Khi implement"). Chỉ sửa đúng các finding được đánh dấu; không sửa thêm gì khác.

## 1. Đọc quyết định

1. Xác định key `PHA-n` từ nhánh hiện tại; thư mục làm việc phải là `.claude/worktrees/PHA-n`.
2. Mở file `ticketed/PHA-n/review-*.md` mới nhất (theo tên). Lập bảng các finding với quyết định: `[x] sửa`, `[x] hoãn`, `[x] bỏ qua`, hoặc chưa đánh dấu.
3. **Dừng và hỏi** nếu: có Blocker hoặc Major chưa đánh dấu, có Question chưa trả lời mà ảnh hưởng tới cách sửa, hoặc ghi chú của người dùng mâu thuẫn với đề xuất sửa. Không tự đoán quyết định.

## 2. Sửa

Với từng finding `sửa`, theo thứ tự Blocker → Major → Minor → Nit:
1. Đọc lại `file:line` và tình huống tái hiện.
2. Finding là lỗi hành vi thì viết test hồi quy trước, chạy để thấy fail, rồi sửa.
3. Cách sửa cần đổi plan (thêm file, đổi hướng giải, đổi schema hoặc hợp đồng) thì dừng và hỏi; đổi `plan.md` sẽ cần duyệt lại.

Sau khi sửa hết: `make lint` và `make test` phải sạch. Commit theo conventional commits có `(PHA-n)`, có thể gom theo nhóm, và ghi ID finding trong thân commit.

## 3. Ghi lại

1. Thêm vào `ticketed/PHA-n/summary.md` một mục:
   ```markdown
   ## Vòng sửa theo review-<YYYYMMDD-HHMM>
   | Finding | Quyết định | Đã làm | Commit |
   |---|---|---|---|
   ## Bằng chứng
   <make lint, make test, test hồi quy mới>
   ```
2. Không sửa nội dung file review, ngoài việc giữ nguyên các dấu người dùng đã đánh.
3. Commit file review và `summary.md`: `docs(ticketed): record review fixes (PHA-n)`.
4. Nếu nhánh đã có PR (`gh pr view --json url,state`), push để PR cập nhật (`git push`, không force).
5. Finding `hoãn` mà là lỗi đã tái hiện được: hỏi người dùng có tạo issue Jira (tiếng Việt) cho nó không. Không tự tạo.

## 4. Bước tiếp theo

Báo người dùng số finding đã sửa, hoãn, bỏ qua, rồi đề xuất một trong các bước:
- còn Blocker/Major vừa sửa đáng kể: review lại bằng `/review-task` (tạo file `review-*.md` mới);
- không còn Blocker/Major mở và chưa có PR: push nhánh và tạo PR theo bước 6 của `/start-task` (chỉ khi người dùng đồng ý);
- không còn Blocker/Major mở và đã có PR: cập nhật mô tả PR là đã review xong, báo PR sẵn sàng merge (bước 6 của `/start-task`).
