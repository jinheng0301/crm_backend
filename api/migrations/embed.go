package migrations

import "embed"

// Files are embedded so the service can apply migrations from any working directory.
//
//go:embed *.up.sql
var files embed.FS
