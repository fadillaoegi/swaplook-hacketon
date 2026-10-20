package httpapi

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD")
		s.json(w, 405, map[string]string{"error": "Method not allowed."})
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" || name == "merchant" || name == "credits" {
		name = "index.html"
	}
	if !fs.ValidPath(name) || !publicAsset(name) {
		s.json(w, 404, map[string]string{"error": "Page or asset not found."})
		return
	}
	info, err := fs.Stat(s.Config.Assets, name)
	if err != nil || !info.Mode().IsRegular() {
		if name == "index.html" {
			s.json(w, 503, map[string]string{"error": "Frontend build is unavailable."})
		} else {
			s.json(w, 404, map[string]string{"error": "Asset not found."})
		}
		return
	}
	content, err := fs.ReadFile(s.Config.Assets, name)
	if err != nil {
		s.json(w, 503, map[string]string{"error": "Asset unavailable."})
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(content))
}

func publicAsset(name string) bool {
	for _, segment := range strings.Split(name, "/") {
		if strings.HasPrefix(segment, ".") {
			return false
		}
	}
	if name == "index.html" || name == "favicon.ico" || name == "favicon.svg" || name == "apple-touch-icon.png" || name == "robots.txt" {
		return true
	}
	if strings.HasPrefix(name, "brand/") {
		return path.Ext(name) == ".svg" || path.Ext(name) == ".png"
	}
	if !strings.HasPrefix(name, "assets/") && !strings.HasPrefix(name, "catalog/") {
		return false
	}
	switch path.Ext(name) {
	case ".js", ".css", ".svg", ".png", ".jpg", ".jpeg", ".webp", ".ico", ".woff2":
		return true
	}
	return false
}
