---
name: review-task
description: Review thay đổi của một task PHA-n bằng ba persona aarch, aeng, asec chạy song song, rồi ghi file review-<YYYYMMDD-HHMM>.md với các finding chia theo mức Blocker, Major, Minor, Nit, Question. Hai chế độ - task của mình (chạy trong worktree .claude/worktrees/PHA-n, review commit local) và PR của người khác ("/review-task PHA-x", số PR hoặc link PR - đọc ticket Jira và diff của PR). Dùng khi người dùng gõ "/review-task", muốn review task đang làm, hoặc được nhờ review PR/ticket của đồng nghiệp.
---

# Review task

Skill này chỉ **đọc và ghi file review**: không sửa code, không commit, không push, không chuyển trạng thái Jira. Với task của mình, việc sửa thuộc về `/fix-review`. Với PR của người khác, chỉ đăng nhận xét lên PR khi người dùng đồng ý (mục 5).

Review dựa trên **diff, plan (nếu có) và issue/SPEC**, không dựa trên lịch sử chat. Ba persona luôn chạy dạng subagent với context riêng, nhưng bước kiểm chứng và chấm mức (mục 3) do session hiện tại làm. Nếu chạy trong session đã implement, chỉ dựa vào code và tài liệu, không dùng những gì "đã biết" từ lúc implement để bỏ qua hay hạ mức một finding.

## 1. Xác định chế độ và thu thập đầu vào

Đầu vào: không có, `PHA-x`, số PR (`#12`, `12`) hoặc link PR.

- **Task của mình**: session đang ở worktree `.claude/worktrees/PHA-n` và đầu vào trống hoặc đúng `PHA-n`. Làm mục 1A.
- **PR của người khác** (hoặc PR không có worktree trên máy này): mọi trường hợp còn lại. Làm mục 1B.

### 1A. Task của mình

1. Key `PHA-n` lấy từ tham số hoặc từ tên nhánh (`feat/PHA-n-...`, `fix/PHA-n-...`).
2. `git fetch origin`, rồi lấy:
   - commit đang review: `git rev-parse --short HEAD`;
   - diff: `git diff --stat origin/master...HEAD`, `git diff origin/master...HEAD`;
   - nếu có thay đổi chưa commit (`git status --porcelain`), báo người dùng rằng review chỉ tính các commit, và ghi chú điều đó trong file review.
3. Đọc `ticketed/PHA-n/plan.md`, các `decision-*.md`, `summary.md` và các `review-*.md` trước đó (để không nêu lại finding đã có quyết định "hoãn" hoặc "bỏ qua").
4. Đọc issue PHA-n và điều kiện xong của epic cha trên Jira.
5. Nơi ghi file review: `ticketed/PHA-n/review-<YYYYMMDD-HHMM>.md` trong worktree.

### 1B. PR của người khác

1. **Tìm PR:**
   - Số hoặc link PR: `gh pr view <N> --json number,title,url,author,state,isDraft,baseRefName,headRefName,headRefOid,body`.
   - `PHA-x`: `gh pr list --state open --search "PHA-x" --json number,title,url,author,headRefName`, chỉ giữ PR có `PHA-x` trong tiêu đề hoặc tên nhánh. Không có PR nào thì báo và dừng. Có nhiều PR thì hỏi người dùng chọn PR nào.
   - Key `PHA-x` lấy từ tiêu đề hoặc tên nhánh của PR. Không có key thì review không có ticket, ghi rõ điều đó trong file review.
2. **Lấy code vào một worktree riêng để đọc**, không đụng nhánh của tác giả. Việc này phải làm ở repo chính: nếu session đang trong worktree khác thì `ExitWorktree` với `keep` trước (chạy từ trong worktree thì worktree mới bị tạo lồng bên trong nó).
   ```bash
   git fetch origin "pull/<N>/head" "<baseRefName>"
   git worktree add --detach .claude/worktrees/review-pr-<N> FETCH_HEAD
   ```
   Nếu `.claude/worktrees/review-pr-<N>` đã có (review lại sau khi tác giả sửa), không tạo mới: vào worktree đó rồi `git fetch origin "pull/<N>/head" && git checkout --detach FETCH_HEAD`.
   Rồi `EnterWorktree` với `path` là `.claude/worktrees/review-pr-<N>`.
3. Kiểm tra `git rev-parse HEAD` trùng `headRefOid` của PR. Lấy diff bằng `git diff --stat origin/<baseRefName>...HEAD` và `git diff origin/<baseRefName>...HEAD`.
4. Đọc mô tả PR, các nhận xét đã có (`gh pr view <N> --comments`) để không nêu lại điều người khác đã nêu, và `ticketed/PHA-x/` nếu nhánh có (plan, decision, summary, review cũ). Không có `ticketed/PHA-x/` thì dựa vào mô tả PR và Jira, và ghi trong file review là không có plan để đối chiếu.
5. Đọc issue PHA-x, điều kiện xong của epic cha và SPEC liên quan trên Jira và Confluence.
6. Nơi ghi file review: `ticketed/PHA-x/review-<YYYYMMDD-HHMM>.md` trong worktree `review-pr-<N>`. File này không commit, không push.

## 2. Ba persona review song song

Gọi **cùng lúc** ba subagent `aarch`, `aeng`, `asec` (ba lệnh Agent trong một lượt). Prompt cho mỗi persona gồm: key, commit, đường dẫn worktree, lệnh để xem diff (`origin/master...HEAD` hoặc `origin/<baseRefName>...HEAD`), đường dẫn plan/decision/summary/review cũ nếu có, với PR của người khác thì thêm mô tả PR, và yêu cầu sau:

> Review thay đổi `<base>...HEAD` theo lăng kính của bạn. Chỉ báo vấn đề nằm trong diff hoặc do diff gây ra. Lỗi có sẵn từ trước thì để riêng trong danh sách "Ngoài phạm vi". Mỗi finding phải có: mức (Blocker/Major/Minor/Nit/Question theo định nghĩa bên dưới), `file:line`, tiêu đề ngắn, vấn đề, tình huống cụ thể gây lỗi và hậu quả cụ thể, đề xuất sửa. Không chứng minh được bằng code thì không báo. Kiểm tra thêm: code có đạt điều kiện xong của issue không, và có làm đúng plan đã duyệt không (nếu có plan). Trả về danh sách finding, không viết lời mở đầu.

Định nghĩa mức (đưa nguyên văn vào prompt):

| Mức | Khi nào |
|---|---|
| **Blocker** | Không được merge: kết quả sai trên đường chạy thật, lỗ hổng bảo mật, mất hoặc lộ dữ liệu, phá cô lập tenant, chưa đạt điều kiện xong, CI sẽ đỏ. |
| **Major** | Nên sửa trong PR này: lỗi ở nhánh ít gặp nhưng có thật, thiếu test cho hành vi bị đổi, lệch plan hoặc SPEC không có giải thích, lỗi xảy ra mà log không chẩn đoán được. |
| **Minor** | Sửa nếu rẻ: độ bền, rõ ràng, trùng lặp nhỏ. |
| **Nit** | Phong cách, đặt tên, câu chữ. Tùy ý. |
| **Question** | Reviewer cần người dùng (hoặc tác giả PR) trả lời trước khi đánh giá được. |

## 3. Kiểm chứng và gộp

Với từng finding nhận về:
1. Mở đúng `file:line` và kiểm tra tình huống nêu ra có xảy ra thật không. Không tái hiện được thì **bỏ**, chỉ đếm số lượng bị bỏ.
2. Tự đánh giá lại mức theo bảng trên; không lấy nguyên mức của persona.
3. Gộp các finding trùng nhau, ghi đủ tên các persona đã nêu.
4. Finding về lỗi có sẵn từ trước chuyển sang mục "Ngoài phạm vi".

## 4. Ghi file review

Ghi vào nơi đã xác định ở mục 1 (giờ địa phương, `date +%Y%m%d-%H%M`). Với task của mình, không commit; `/fix-review` sẽ commit cùng các bản sửa.

```markdown
# Review PHA-n · <YYYY-MM-DD HH:MM>

- Commit: <sha> trên nhánh <nhánh>
- PR: <link, tác giả> (chỉ khi review PR của người khác)
- Persona: aarch, aeng, asec
- Đã bỏ <k> finding không tái hiện được
- Kết luận đề xuất: Approve | Request changes | Comment (chỉ khi review PR của người khác)

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

Bỏ các mục không có finding, nhưng giữ bảng số lượng. Với PR của người khác, bỏ hai dòng **Quyết định** và **Ghi chú** (quyết định thuộc về tác giả), và chọn kết luận đề xuất: còn Blocker hoặc Major thì `Request changes`; chỉ còn Question thì `Comment`; còn lại `Approve`.

## 5. Báo người dùng

Trả lời ngắn: bảng số lượng theo mức, đường dẫn đầy đủ của file review, và bước tiếp theo.

- **Task của mình:**
  > Mở file review, đánh dấu **Quyết định** cho từng finding (trả lời các Question), rồi chạy `/fix-review` trong session implement.
- **PR của người khác:** nêu kết luận đề xuất, rồi hỏi người dùng có muốn đăng lên PR không. Chỉ đăng khi người dùng đồng ý:
  - mặc định đăng dạng nhận xét: `gh pr review <N> --comment --body-file <file review>`;
  - chỉ dùng `--approve` hoặc `--request-changes` khi người dùng nói rõ;
  - người dùng có thể sửa file review trước khi đăng.

  Báo thêm: worktree `.claude/worktrees/review-pr-<N>` được giữ lại để review lại sau khi tác giả sửa (chạy lại `/review-task #<N>`). Xong việc thì `ExitWorktree` với `keep` rồi `git worktree remove --force .claude/worktrees/review-pr-<N>` ở repo chính (`--force` vì file review chưa commit nằm trong đó).
