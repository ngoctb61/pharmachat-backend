#!/bin/sh
# PreToolUse (Write|Edit|NotebookEdit): inside a task worktree (.claude/worktrees/PHA-n),
# files outside ticketed/ may only change once the user approved ticketed/PHA-n/plan.md
# by typing "approve" (plan-approve.sh). Editing the plan afterwards needs a new
# approval. Edits made through Bash are not covered; this guards against starting
# early by mistake, it is not a sandbox.
PAYLOAD=$(cat)
. "$(dirname "$0")/lib.sh"

file=$(json_field '.tool_input.file_path' file_path)
[ -n "$file" ] || file=$(json_field '.tool_input.notebook_path' notebook_path)

block() {
	echo "Blocked by .claude/hooks/plan-gate.sh: $1" >&2
	exit 2
}

case "$file" in
	*/.approved) block "only the user approves a plan, by typing \"approve\" in the chat; plan-approve.sh then writes .approved." ;;
	*/.claude/worktrees/PHA-[0-9]*/*) ;;
	*) exit 0 ;;
esac

key=${file#*/.claude/worktrees/}
key=${key%%/*}
worktree=${file%%/.claude/worktrees/*}/.claude/worktrees/$key
case "${file#"$worktree"/}" in
	ticketed/*) exit 0 ;;
esac

plan=$worktree/ticketed/$key/plan.md
[ -f "$plan" ] || block "$key has no plan yet. Write ticketed/$key/plan.md with aarch, show it to the user and wait for them to type \"approve\"."
plan_approved "$plan" || block "the plan for $key is not approved yet, or changed after approval. Show ticketed/$key/plan.md to the user and wait for them to type \"approve\"."
exit 0
