---
name: review-task
description: Review thay đổi của một task PHA-n bằng ba persona aarch, aeng, asec chạy song song, rồi ghi ticketed/PHA-n/review-<YYYYMMDD-HHMM>.md với các finding chia theo mức Blocker, Major, Minor, Nit, Question. Chạy trong một session mới mở ở worktree .claude/worktrees/PHA-n, không phải session đã implement. Dùng khi người dùng gõ "/review-task" hoặc muốn review task đang làm.
---

# Review task

Session này chỉ **đọc và ghi file review**: không sửa code, không commit, không push, không chuyển trạng thái Jira. Việc sửa thuộc về session implement qua `/fix-review`.

Review dựa trên **diff, plan và issue/SPEC**, không dựa trên lịch sử chat lúc implement. Vì vậy nên chạy trong session mới. Nếu session này chính là session đã implement task, báo người dùng và đề nghị mở session mới trước khi tiếp tục.

## 1. Xác định task và thu thập đầu vào

1. Key `PHA-n`: lấy từ tham số, hoặc từ tên nhánh hiện tại (`feat/PHA-n-...`, `fix/PHA-n-...`). Thư mục làm việc phải là worktree `.claude/worktrees/PHA-n`; nếu không, báo người dùng cách mở (`cd .claude/worktrees/PHA-n && claude`) và dừng.
2. `git fetch origin`, rồi lấy:
   - commit đang review: `git rev-parse --short HEAD`;
   - danh sách file và diff: `git diff --stat origin/master...HEAD`, `git diff origin/master...HEAD`;
   - nếu có thay đổi chưa commit (`git status --porcelain`), báo người dùng rằng review chỉ tính các commit, và ghi chú điều đó trong file review.
3. Đọc `ticketed/PHA-n/plan.md`, các `decision-*.md`, `summary.md` và các `review-*.md` trước đó (để không nêu lại finding đã có quyết định "hoãn" hoặc "bỏ qua").
4. Đọc issue PHA-n và điều kiện xong của epic cha trên Jira.

## 2. Ba persona review song song

Gọi **cùng lúc** ba subagent `aarch`, `aeng`, `asec` (ba lệnh Agent trong một lượt). Prompt cho mỗi persona gồm: key, commit, đường dẫn worktree, lệnh để xem diff, đường dẫn plan/decision/summary/review cũ, và yêu cầu sau:

> Review thay đổi `origin/master...HEAD` theo lăng kính của bạn. Chỉ báo vấn đề nằm trong diff hoặc do diff gây ra. Lỗi có sẵn từ trước thì để riêng trong danh sách "Ngoài phạm vi". Mỗi finding phải có: mức (Blocker/Major/Minor/Nit/Question theo định nghĩa bên dưới), `file:line`, tiêu đề ngắn, vấn đề, tình huống cụ thể gây lỗi và hậu quả cụ thể, đề xuất sửa. Không chứng minh được bằng code thì không báo. Kiểm tra thêm: code có làm đúng plan đã duyệt và đạt điều kiện xong không. Trả về danh sách finding, không viết lời mở đầu.

Định nghĩa mức (đưa nguyên văn vào prompt):

| Mức | Khi nào |
|---|---|
| **Blocker** | Không được merge: kết quả sai trên đường chạy thật, lỗ hổng bảo mật, mất hoặc lộ dữ liệu, phá cô lập tenant, chưa đạt điều kiện xong, CI sẽ đỏ. |
| **Major** | Nên sửa trong PR này: lỗi ở nhánh ít gặp nhưng có thật, thiếu test cho hành vi bị đổi, lệch plan hoặc SPEC không có giải thích, lỗi xảy ra mà log không chẩn đoán được. |
| **Minor** | Sửa nếu rẻ: độ bền, rõ ràng, trùng lặp nhỏ. |
| **Nit** | Phong cách, đặt tên, câu chữ. Tùy ý. |
| **Question** | Reviewer cần người dùng trả lời trước khi đánh giá được. |

## 3. Kiểm chứng và gộp

Với từng finding nhận về:
1. Mở đúng `file:line` và kiểm tra tình huống nêu ra có xảy ra thật không. Không tái hiện được thì **bỏ**, chỉ đếm số lượng bị bỏ.
2. Tự đánh giá lại mức theo bảng trên; không lấy nguyên mức của persona.
3. Gộp các finding trùng nhau, ghi đủ tên các persona đã nêu.
4. Finding về lỗi có sẵn từ trước chuyển sang mục "Ngoài phạm vi".

## 4. Ghi file review

Ghi `ticketed/PHA-n/review-<YYYYMMDD-HHMM>.md` (giờ địa phương, `date +%Y%m%d-%H%M`). Không commit; session implement sẽ commit cùng các bản sửa.

```markdown
# Review PHA-n · <YYYY-MM-DD HH:MM>

- Commit: <sha> trên nhánh <nhánh>
- Persona: aarch, aeng, asec
- Đã bỏ <k> finding không tái hiện được

| Mức | Số lượng |
|---|---|
| 🔴 Blocker | 0 |
| 🟠 Major | 0 |
| 🟡 Minor | 0 |
| ⚪ Nit | 0 |
| ❓ Question | 0 |

## 🔴 Blocker

### BLK-1 · <tiêu đề> · `path/file.go:88` · asec
**Vấn đề:** ...
**Tái hiện:** <tình huống cụ thể> → <hậu quả cụ thể>
**Đề xuất sửa:** ...
**Quyết định:** [ ] sửa  [ ] hoãn  [ ] bỏ qua
**Ghi chú:**

## 🟠 Major
(MAJ-1, MAJ-2, ... cùng định dạng)

## 🟡 Minor
(MIN-1, ...)

## ⚪ Nit
(NIT-1, ...)

## ❓ Question
(Q-1, ...: thay "Đề xuất sửa" bằng "Câu hỏi", thay "Quyết định" bằng "Trả lời:")

## Ngoài phạm vi
| Vấn đề | `file:line` | Vì sao ngoài phạm vi |
|---|---|---|
```

Bỏ các mục không có finding, nhưng giữ bảng số lượng.

## 5. Báo người dùng

Trả lời ngắn: bảng số lượng theo mức, đường dẫn file review, và bước tiếp theo:
> Mở file review, đánh dấu **Quyết định** cho từng finding (trả lời các Question), rồi quay lại session implement và chạy `/fix-review`.
