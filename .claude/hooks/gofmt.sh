#!/bin/sh
# PostToolUse (Write|Edit): gofmt the edited file when it is a Go source file.
# Generated sqlc code is formatted by sqlc itself, so it is skipped.
PAYLOAD=$(cat)
. "$(dirname "$0")/lib.sh"

file=$(json_field '.tool_input.file_path' file_path)
case "$file" in
	*/internal/store/*.go) exit 0 ;;
	*.go) ;;
	*) exit 0 ;;
esac
[ -f "$file" ] || exit 0
command -v gofmt >/dev/null 2>&1 || exit 0
gofmt -w "$file"
