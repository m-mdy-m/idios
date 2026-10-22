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
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		switch ext {
		case ".js", ".mjs":
			contentType = "text/javascript; charset=utf-8"
		case ".css":
			contentType = "text/css; charset=utf-8"
		case ".html":
			contentType = "text/html; charset=utf-8"
		case ".svg":
			contentType = "image/svg+xml"
		case ".json":
			contentType = "application/json; charset=utf-8"
		case ".woff2":
			contentType = "font/woff2"
		case ".woff":
			contentType = "font/woff"
		case ".ttf":
			contentType = "font/ttf"
		case ".wasm":
			contentType = "application/wasm"
		default:
			contentType = "application/octet-stream"
		}
	}
	w.Header().Set("Content-Type", contentType)
	if name == "index.html" {
		w.Header().Set("Cache-Control", "no-cache")
	} else if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
