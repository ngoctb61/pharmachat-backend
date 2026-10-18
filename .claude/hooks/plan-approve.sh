#!/bin/sh
# UserPromptSubmit: when the user types "approve" (or "approve PHA-n"), approve the
# pending plan .claude/worktrees/PHA-n/ticketed/PHA-n/plan.md by writing its SHA-256
# to .approved beside it; plan-gate.sh then lets code edits through. Only this hook
# writes .approved, so an approval always comes from the user's own message.
PAYLOAD=$(cat)
. "$(dirname "$0")/lib.sh"

prompt=$(json_field '.prompt' prompt | tr -d '\r' | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')
case "$prompt" in
	approve|Approve|APPROVE) want= ;;
	approve\ *|Approve\ *|APPROVE\ *)
		want=$(printf '%s' "${prompt#* }" | tr '[:lower:]' '[:upper:]')
		printf '%s' "$want" | grep -Eq '^PHA-[0-9]+$' || exit 0
		;;
	*) exit 0 ;;
esac

# Task worktrees live under the main checkout, whichever worktree this session is in.
cwd=$(json_field '.cwd' cwd)
common=$(git -C "${cwd:-.}" rev-parse --path-format=absolute --git-common-dir 2>/dev/null) || exit 0
root=$(dirname "$common")

pending=
for plan in "$root"/.claude/worktrees/PHA-*/ticketed/PHA-*/plan.md; do
	[ -f "$plan" ] || continue
	key=$(basename "$(dirname "$plan")")
	if [ -n "$want" ] && [ "$key" != "$want" ]; then
		continue
	fi
	plan_approved "$plan" && continue
	pending="$pending $plan"
done

# shellcheck disable=SC2086 # paths under .claude/worktrees/PHA-n contain no spaces
set -- $pending
if [ "$#" -eq 0 ]; then
	echo "plan-approve.sh: no plan is waiting for approval${want:+ for $want}; nothing was recorded."
	exit 0
fi
if [ "$#" -gt 1 ]; then
	keys=
	for plan in "$@"; do
		keys="$keys $(basename "$(dirname "$plan")")"
	done
	echo "Có nhiều plan đang chờ duyệt:$keys. Gõ \"approve PHA-n\" để chọn plan cần duyệt." >&2
	exit 2
fi

plan=$1
key=$(basename "$(dirname "$plan")")
hash=$(sha256_of "$plan")
printf '%s  %s\n' "$hash" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >"$(dirname "$plan")/.approved"
echo "plan-approve.sh: the user approved the plan for $key (plan.md sha256 $hash). Approval recorded in ticketed/$key/.approved; implementation may start."
exit 0
