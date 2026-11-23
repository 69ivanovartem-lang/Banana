package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func webDir() string {
	if d := os.Getenv("BANANA_WEB_DIR"); d != "" {
		return d
	}
	return "./web"
}

// handleStatic раздаёт собранный фронт (Vite dist).
// Нет такого файла — отдаём index.html (SPA).
func handleStatic(w http.ResponseWriter, r *http.Request) {
	dir := webDir()
	name := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")

	if name != "" && name != "." && !strings.Contains(name, "..") {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			http.ServeFile(w, r, p)
			return
		}
	}

	idx := filepath.Join(dir, "index.html")
	if _, err := os.Stat(idx); err != nil {
		http.Error(w, "frontend not built: "+dir, http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, idx)
}