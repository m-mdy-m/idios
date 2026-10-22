//go:build embed_web

package web

import (
	"embed"
	"io/fs"
)

// The release build runs `pnpm build` first, then embeds its static production output.
//
//go:embed all:dist
var files embed.FS

// Filesystem exposes the compiled frontend from the IDIOS executable.
func Filesystem() (fs.FS, error) { return fs.Sub(files, "dist") }
