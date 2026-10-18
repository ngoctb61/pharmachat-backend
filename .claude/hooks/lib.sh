#!/bin/sh
# Shared helpers for hook scripts. Reads one JSON field from the hook payload on
# stdin, using jq when installed and a sed fallback otherwise (enough for the
# simple string fields these hooks need).

# json_field <jq-path> <fallback-key>  (payload must be in $PAYLOAD)
json_field() {
	if command -v jq >/dev/null 2>&1; then
		printf '%s' "$PAYLOAD" | jq -r "$1 // empty"
	else
		printf '%s' "$PAYLOAD" | sed -n "s/.*\"$2\"[[:space:]]*:[[:space:]]*\"\\([^\"]*\\)\".*/\\1/p" | head -n 1
	fi
}

# sha256_of <file>: prints the file's SHA-256 digest.
sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

# plan_approved <plan.md>: succeeds when the .approved file beside the plan records
# the plan's current hash, so editing an approved plan needs a new approval.
plan_approved() {
	marker="$(dirname "$1")/.approved"
	[ -f "$marker" ] && [ "$(head -n 1 "$marker" | cut -d' ' -f1)" = "$(sha256_of "$1")" ]
}
