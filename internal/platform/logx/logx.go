// Package logx builds the structured JSON logger. Every record passes through a
// redaction step so phone numbers, emails, message content and secrets never reach
// the logs (constitution Art. 3).
package logx

import (
	"io"
	"log/slog"
	"regexp"
	"strings"
)

const redacted = "[REDACTED]"

// Attribute keys whose values are always dropped, matched case-insensitively.
var sensitiveKeys = map[string]bool{
	"phone": true, "phone_number": true, "email": true, "content": true, "message_text": true,
	"text": true, "body": true, "password": true, "token": true, "access_token": true,
	"refresh_token": true, "api_key": true, "secret": true, "authorization": true, "cookie": true,
}

var (
	emailRe = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	// Vietnamese mobile numbers: 0xxxxxxxxx or +84xxxxxxxxx, optional separators.
	phoneRe = regexp.MustCompile(`(?:\+84|\b0)(?:[\s.\-]?\d){9}\b`)
)

// New returns a JSON logger writing to w at the given level ("debug", "info", "warn", "error").
func New(w io.Writer, level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:       lvl,
		ReplaceAttr: redact,
	}))
}

func redact(_ []string, a slog.Attr) slog.Attr {
	if sensitiveKeys[strings.ToLower(a.Key)] {
		return slog.String(a.Key, redacted)
	}
	if a.Value.Kind() == slog.KindString {
		return slog.String(a.Key, Scrub(a.Value.String()))
	}
	return a
}

// Scrub masks emails and phone numbers embedded in free text, such as error messages.
func Scrub(s string) string {
	s = emailRe.ReplaceAllString(s, redacted)
	return phoneRe.ReplaceAllString(s, redacted)
}
