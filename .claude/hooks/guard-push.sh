#!/bin/sh
# PreToolUse (Bash, git push only): stand-in for GitHub branch protection, which
# GitHub Free does not offer on private repos (constitution Art. 4, on Confluence).
# Blocks force pushes and any push that would update master; exit 2 tells Claude why.
PAYLOAD=$(cat)
. "$(dirname "$0")/lib.sh"

cmd=$(json_field '.tool_input.command' command)
case "$cmd" in
	*"git "*push*) ;;
	*) exit 0 ;;
esac

block() {
	echo "Blocked by .claude/hooks/guard-push.sh: $1 Push a feat/, fix/ or chore/ branch and open a PR instead (constitution Art. 4)." >&2
	exit 2
}

# Only text that is itself a `git push` invocation counts: a command segment
# (split on newlines, &&, ||, ; and |) that starts with `git [-C dir] push`.
# Matching the whole command blocked `gh pr create` and `git commit` whenever
# their message mentioned "git push" and "master". Prints one line per push:
# the -C dir (or "."), a tab, then the arguments after `push`.
pushes=$(printf '%s\n' "$cmd" | awk '
	{
		gsub(/&&|\|\||[;|]/, "\n")
		n = split($0, segs, "\n")
		for (i = 1; i <= n; i++) {
			s = segs[i]
			sub(/^[ \t]+/, "", s)
			if (s !~ /^git[ \t]/) continue
			sub(/^git[ \t]+/, "", s)
			dir = "."
			while (match(s, /^-C[ \t]+[^ \t]+[ \t]+/)) {
				d = substr(s, 1, RLENGTH)
				sub(/^-C[ \t]+/, "", d); sub(/[ \t]+$/, "", d)
				dir = d
				s = substr(s, RLENGTH + 1)
			}
			if (s !~ /^push([ \t]|$)/) continue
			sub(/^push[ \t]*/, "", s)
			print dir "\t" s
		}
	}')
[ -n "$pushes" ] || exit 0

cwd=$(json_field '.cwd' cwd)
tab=$(printf '\t')
printf '%s\n' "$pushes" | while IFS="$tab" read -r dir args; do
	positional=0
	set -f
	# shellcheck disable=SC2086 # word-splitting the arguments is intended
	set -- $args
	set +f
	for arg in "$@"; do
		case "$arg" in
			--force*|-f|-[!-]*f*|+*) block "force pushes are not allowed." ;;
			*master*) block "pushing to master is not allowed." ;;
			-*) ;;
			*) positional=$((positional + 1)) ;;
		esac
	done
	# Without an explicit refspec (`git push`, `git push origin`), git pushes the
	# current branch, so check which branch that is.
	if [ "$positional" -lt 2 ]; then
		case "$dir" in
			/*) where=$dir ;;
			*) where="${cwd:-.}/$dir" ;;
		esac
		branch=$(git -C "$where" rev-parse --abbrev-ref HEAD 2>/dev/null)
		if [ "$branch" = "master" ]; then
			block "the current branch is master."
		fi
	fi
done
