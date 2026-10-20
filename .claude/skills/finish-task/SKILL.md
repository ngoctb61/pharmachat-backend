---
name: finish-task
description: Dọn một task sau khi PR đã merge - chuyển issue Jira PHA-n sang Done kèm comment, pull master ở repo chính, xóa worktree .claude/worktrees/PHA-n và nhánh local lẫn remote, nhắc báo repo frontend nếu api/openapi.yaml đổi. Dùng khi người dùng gõ "/finish-task", "/finish-task PHA-n" hoặc nói PR của task đã merge. Cũng dùng được cho nhánh chore/ (bỏ bước Jira).
---

# Kết thúc task

Chỉ chạy khi PR **đã merge**. Không sửa code. Chỉ xóa những gì đã chắc nằm trong PR đã merge.

## 1. Xác định task và PR

1. Nhánh: lấy từ tham số (`PHA-n`, số PR, hoặc tên nhánh `chore/...`), hoặc từ nhánh hiện tại nếu session đang trong worktree của task. Với `PHA-n`, nhánh là nhánh của worktree `.claude/worktrees/PHA-n` (`git worktree list`).
2. `gh pr view <nhánh hoặc số PR> --json number,url,state,mergedAt,mergeCommit,headRefName,headRefOid,files`.
   - `state` khác `MERGED`: báo trạng thái hiện tại và **dừng**.
3. Ghi lại: số PR, link, merge commit, `headRefOid`, và PR có đổi `api/openapi.yaml` không (trong `files`).

## 2. Jira (bỏ qua nếu nhánh không có key `PHA-n`)

1. Chuyển issue sang **Done**.
2. Thêm comment tiếng Việt: link PR, merge commit, một dòng tóm tắt đã giao gì, và các finding đã hoãn (nếu có, từ `ticketed/PHA-n/review-*.md` và `summary.md`) kèm issue Jira đã tạo cho chúng.

## 3. Dọn git (ở repo chính)

Session đang trong worktree thì `ExitWorktree` với `keep` trước; các lệnh dưới chạy ở repo chính.

1. `git status --porcelain --branch`: repo chính phải ở `master` và sạch. Nếu không, báo và dừng phần git (không tự stash hay reset).
2. `git fetch origin --prune`, rồi `git pull --ff-only`. Từ đây hook và skill ở repo chính là bản mới nhất.
3. Worktree của task (`.claude/worktrees/PHA-n` hoặc worktree của nhánh chore): `git worktree remove <path>`. Git từ chối nếu còn thay đổi chưa commit: **dừng và báo** danh sách file, không dùng `--force`.
   Nếu còn worktree review `.claude/worktrees/review-pr-<N>` của chính PR này thì xóa luôn bằng `git worktree remove --force` (chỉ chứa file review chưa commit).
4. Nhánh local: so `git rev-parse <nhánh>` với `headRefOid`.
   - Trùng: `git branch -D <nhánh>` (dùng `-D` vì PR merge kiểu squash thì `-d` không nhận ra là đã merge; việc so commit ở trên đã đảm bảo không mất gì).
   - Khác: nhánh local có commit không nằm trong PR. **Dừng và báo**, không xóa.
5. Nhánh remote: nếu còn (`git ls-remote --heads origin <nhánh>`), xóa bằng `git push origin --delete <nhánh>`.

## 4. Báo người dùng

Liệt kê từng việc đã làm và kết quả thật (trạng thái Jira, commit `master` sau khi pull, worktree và nhánh đã xóa). Nếu PR có đổi `api/openapi.yaml`: nhắc người dùng báo repo frontend kéo bản mới, kèm nháp một tin nhắn ngắn (link PR, endpoint thêm hoặc đổi). Không tự gửi.
