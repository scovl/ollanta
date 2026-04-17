package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var staticFiles embed.FS

// staticHandler returns an http.Handler that serves embedded static files.
// Any path that doesn't resolve to a file falls back to index.html (SPA support).
func staticHandler() http.Handler {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("api: static embed sub: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to open the requested file; fall back to index.html for the SPA.
		f, openErr := sub.Open(r.URL.Path[1:]) // strip leading /
		if openErr == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// Serve the SPA shell for unknown paths so the JS router takes over.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}
