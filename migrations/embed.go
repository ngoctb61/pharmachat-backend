// Package migrations embeds the goose SQL migrations so binaries can apply them.
package migrations

import "embed"

// FS holds the goose migration files, applied by internal/platform/migrate.
//
//go:embed *.sql
var FS embed.FS
