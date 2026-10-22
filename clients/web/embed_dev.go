//go:build !embed_web

package web

import (
	"embed"
	"io/fs"
)

//go:embed fallback
var files embed.FS

// Filesystem exposes a tiny development fallback unless the frontend bundle is embedded.
func Filesystem() (fs.FS, error) { return fs.Sub(files, "fallback") }
