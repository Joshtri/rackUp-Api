// Package migrations embeds the SQL files in this folder into the binary.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
