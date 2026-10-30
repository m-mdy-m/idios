package server

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"idios/clients/web"
)

func (s *Server) serveUI(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "API route not found"})
		return
	}
	assets, err := web.Filesystem()
	if err != nil {
		http.Error(w, "UI assets are unavailable", http.StatusInternalServerError)
		return
	}
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "" || name == "." {
		name = "index.html"
	}
	data, readErr := fs.ReadFile(assets, name)
	if readErr != nil {
		if filepath.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		name = "index.html"
		data, err = fs.ReadFile(assets, name)
		if err != nil {
			http.Error(w, "UI entrypoint is missing", http.StatusInternalServerError)
			return
		}
	}
	ext := strings.ToLower(filepath.Ext(name))
	contentType := staticContentType(ext)
	w.Header().Set("Content-Type", contentType)
	if name == "index.html" {
		w.Header().Set("Cache-Control", "no-cache")
	} else if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// staticContentType prefers a fixed table over mime.TypeByExtension. On Windows the standard
// library reads the registry, where .js/.mjs are sometimes registered as text/plain; browsers
// refuse to run module scripts and the PDF.js worker with that type, so the Reader would fail.
func staticContentType(ext string) string {
	switch ext {
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".html":
		return "text/html; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".json":
		return "application/json; charset=utf-8"
	case ".woff2":
		return "font/woff2"
	case ".woff":
		return "font/woff"
	case ".ttf":
		return "font/ttf"
	case ".wasm":
		return "application/wasm"
	}
	if contentType := mime.TypeByExtension(ext); contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}
