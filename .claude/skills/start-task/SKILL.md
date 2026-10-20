---
name: start-task
description: Bắt đầu hoặc tiếp tục một task theo issue Jira (ví dụ "/start-task PHA-21" hoặc mã cũ "/start-task GO-07"). Tạo worktree .claude/worktrees/PHA-n, để aarch lên plan trong ticketed/PHA-n/, chờ người dùng gõ "approve", rồi nhập vai aeng để implement và tóm tắt. Dùng khi người dùng muốn làm một task hoặc hỏi "làm task tiếp theo".
---

# Bắt đầu task

Quy trình: **aarch lên plan → người dùng gõ `approve` → aeng implement → `/review-task` → `/fix-review`.**

Dữ liệu nghiệp vụ nằm trên Atlassian (site `pharmachat.atlassian.net`), đọc và ghi qua MCP `atlassian`:
- Task: Jira project `PHA`. Mỗi giai đoạn là một epic; issue con có tiêu đề `GO-xx · ...`. Phụ thuộc giữa các giai đoạn là link "blocks".
- SPEC, constitution, kế hoạch: Confluence space `SD`, dưới trang "PharmaChat — Tài liệu dự án".

Cấu trúc của một task:

```
.claude/worktrees/PHA-n/            git worktree, nhánh feat/PHA-n-<scope> (không commit, đã gitignore)
└── ticketed/PHA-n/                 commit cùng nhánh, đi kèm PR
    ├── plan.md                     aarch viết, người dùng duyệt
    ├── .approved                   hook plan-approve.sh ghi khi người dùng gõ "approve"
    ├── decision-NN-<slug>.md       quyết định thiết kế, chỉ khi task phức tạp
    ├── summary.md                  aeng viết sau khi implement và sau mỗi vòng sửa
    └── review-<YYYYMMDD-HHMM>.md   /review-task viết
```

## 0. Xác định issue

Đầu vào: key issue (`PHA-21`) hoặc mã cũ (`GO-07`, tìm bằng JQL `project = PHA AND summary ~ "GO-07"`). Nếu không có, tìm issue đầu tiên chưa xong mà epic cha không còn bị chặn: `project = PHA AND issuetype = Task AND statusCategory != Done ORDER BY key ASC`, rồi xác nhận với người dùng.

Nếu `.claude/worktrees/PHA-n` đã tồn tại thì đây là **tiếp tục task**: vào worktree (bước 1, chỉ phần `EnterWorktree`) rồi nhảy tới bước tương ứng với trạng thái:
- chưa có `ticketed/PHA-n/plan.md` → bước 2;
- có plan nhưng chưa duyệt (hook `plan-gate.sh` sẽ chặn sửa code) → trình bày lại plan, chờ `approve`;
- đã duyệt → bước 4 (tiếp tục implement hoặc báo đã xong và trỏ sang review).

## 1. Đọc yêu cầu và tạo worktree

1. Đọc mô tả issue, điều kiện xong của epic cha và các link chặn (`getJiraIssue` với `view: "evidence"` cho cả issue lẫn epic cha). Chỉ xét link chiều vào "is blocked by": trong `issuelinks` đó là mục có `inwardIssue`. Mục có `outwardIssue` ("blocks") là issue này chặn issue khác, không làm nó bị chặn. Nếu một `inwardIssue` của epic cha hoặc của issue chưa Done, **dừng lại và báo**.
2. Tạo worktree từ `master` mới nhất (ở thư mục repo chính):
   ```bash
   git fetch origin
   git worktree add --no-track .claude/worktrees/PHA-n -b feat/PHA-n-<scope> origin/master
   ```
   Dùng `fix/` thay `feat/` nếu là sửa lỗi. Phải có `--no-track`: thiếu nó thì nhánh mới lấy `origin/master` làm upstream, nên `git status`/`git pull` so với master. Upstream đúng được đặt lúc push lần đầu (`git push -u origin feat/PHA-n-<scope>`).
3. Nếu repo chính có `deploy/.env` (kiểm tra bằng `test -e`, **không đọc**), tạo symlink để `make up` chạy được trong worktree: `ln -s ../../../../deploy/.env .claude/worktrees/PHA-n/deploy/.env`.
4. Chuyển session vào worktree bằng tool `EnterWorktree` với `path` là `.claude/worktrees/PHA-n`. Từ đây mọi lệnh và mọi file đều trong worktree.
5. Chuyển issue sang **In Progress**.

## 2. aarch lên plan

1. Gọi subagent `aarch` (Agent tool, `subagent_type: "aarch"`), đưa key issue, epic cha, SPEC liên quan (giai đoạn 1 → SPEC-01, 2 → SPEC-02, 3 → SPEC-03, 4 → SPEC-05, 5 → SPEC-06, 6 → SPEC-04, 7 → SPEC-07) và yêu cầu trả về `plan.md` cùng các `decision-NN-*.md` nếu task phức tạp. Task chạm auth, RLS, mã hóa, webhook hoặc dữ liệu cá nhân thì gọi thêm `asec` để góp ý rủi ro bảo mật trước khi chốt plan.
2. Ghi nguyên văn kết quả vào `ticketed/PHA-n/`.
3. Trình bày cho người dùng: tóm tắt cách làm, bảng thay đổi, test sẽ viết, câu hỏi mở. Kết thúc bằng: *"Gõ `approve` để duyệt plan, hoặc góp ý để aarch sửa."* rồi **dừng lượt**.

Người dùng góp ý thì sửa plan (gọi lại `aarch` nếu thay đổi lớn) và trình bày lại. Không tự viết `.approved`; hook `plan-gate.sh` chặn việc đó.

## 3. Duyệt

Khi người dùng gõ `approve`, hook `plan-approve.sh` ghi `ticketed/PHA-n/.approved` (SHA-256 của plan và thời điểm duyệt) và báo vào context. Không thấy thông báo đó thì plan **chưa** được duyệt. Mọi sửa đổi `plan.md` sau đó đều làm mất duyệt.

Commit `ticketed/PHA-n/` (gồm `.approved`): `docs(ticketed): add approved plan (PHA-n)`.

## 4. aeng implement

Nhập vai **aeng**: đọc `.claude/agents/aeng.md` và làm theo phần "Khi implement".

1. Viết các test trong mục "Test viết trước", chạy để thấy fail đúng lý do, rồi implement. Có migration thì theo skill `new-migration`.
2. Plan sai hoặc thiếu: dừng và hỏi người dùng. Đổi plan thì phải duyệt lại.
3. `make lint` và `make test` phải sạch. Task có thay đổi hạ tầng hoặc endpoint thì chạy thêm skill `verify-stack`.
4. Commit theo conventional commits, có key `PHA-n` trong tiêu đề.

## 5. Tóm tắt và chuyển sang review

1. Viết `ticketed/PHA-n/summary.md`:
   ```markdown
   # Tóm tắt PHA-n · vòng implement
   ## Đã làm (đối chiếu plan)
   | Mục trong plan | Trạng thái | Ghi chú |
   ## Lệch so với plan
   ## Bằng chứng
   <lệnh đã chạy và kết quả: make lint, make test, test nào chứng minh điều kiện xong>
   ## Ngoài phạm vi phát hiện được
   ```
2. Commit `summary.md`.
3. Báo người dùng bằng tóm tắt ngắn và bước tiếp theo:
   > Chạy `/review-task` để review. Chạy ngay ở session này được; muốn review không bị ảnh hưởng bởi context lúc implement thì mở session mới: `cd .claude/worktrees/PHA-n && claude`, rồi chạy `/review-task`.

## 6. Sau review

- Người dùng đánh dấu quyết định trong file `review-*.md` rồi chạy `/fix-review` ở session này.
- Khi không còn finding **Blocker** hay **Major** chưa xử lý: push nhánh (không push `master`) và soạn mô tả PR theo `.github/pull_request_template.md`, kèm link tới `ticketed/PHA-n/`. **Chỉ tạo PR khi người dùng đồng ý.**
- Khi đã tạo PR: chuyển issue sang **In Review** và thêm comment tiếng Việt ghi bằng chứng ngắn kèm link PR. Chỉ chuyển sang **Done** khi PR đã merge.
- Sau khi merge: rời worktree (`ExitWorktree` với `keep`) rồi `git worktree remove .claude/worktrees/PHA-n` ở repo chính.
