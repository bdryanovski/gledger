// Package web serves the embedded React frontend.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed static/dist
var staticFiles embed.FS

// Handler returns an http.Handler that serves the embedded React SPA.
// Any path that does not resolve to a real file is served as index.html
// so that React Router's client-side routing works correctly.
func Handler() http.Handler {
	dist, err := fs.Sub(staticFiles, "static/dist")
	if err != nil {
		panic("web: could not sub embedded static files: " + err.Error())
	}

	fsHandler := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip leading slash and try to open the file.
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		_, err := dist.Open(path)
		if err != nil {
			// File not found → serve index.html for SPA client-side routing.
			r.URL.Path = "/"
		}
		fsHandler.ServeHTTP(w, r)
	})
}
