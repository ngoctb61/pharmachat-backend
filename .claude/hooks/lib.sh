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
