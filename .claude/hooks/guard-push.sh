#!/bin/sh
# PreToolUse (Bash, git push only): stand-in for GitHub branch protection, which
# GitHub Free does not offer on private repos (.sdd/constitution.md Art. 4).
# Blocks force pushes and any push that would update master; exit 2 tells Claude why.
PAYLOAD=$(cat)
. "$(dirname "$0")/lib.sh"

cmd=$(json_field '.tool_input.command' command)
case "$cmd" in
	*"git push"*) ;;
	*) exit 0 ;;
esac

block() {
	echo "Blocked by .claude/hooks/guard-push.sh: $1 Push a feat/, fix/ or chore/ branch and open a PR instead (constitution Art. 4)." >&2
	exit 2
}

case "$cmd" in
	*--force*|*" -f"*|*"+"*) block "force pushes are not allowed." ;;
	*master*) block "pushing to master is not allowed." ;;
esac

# Without an explicit refspec (`git push`, `git push origin`), git pushes the
# current branch, so check which branch that is.
positional=0
set -f
# shellcheck disable=SC2086 # word-splitting the command into arguments is intended
set -- ${cmd#*git push}
set +f
for arg in "$@"; do
	case "$arg" in
		"&&"|"||"|";"|"|") break ;;
		-*) ;;
		*) positional=$((positional + 1)) ;;
	esac
done
if [ "$positional" -lt 2 ]; then
	cwd=$(json_field '.cwd' cwd)
	branch=$(git -C "${cwd:-.}" rev-parse --abbrev-ref HEAD 2>/dev/null)
	if [ "$branch" = "master" ]; then
		block "the current branch is master."
	fi
fi
exit 0
