// Package server provides a lightweight local HTTP server that serves the
// Ollanta report as an interactive web UI. Static assets (HTML/CSS/JS) are
// embedded at compile time via go:embed so the binary has zero runtime
// dependencies.
package server

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

//go:embed static/dist
var staticFiles embed.FS

// Serve starts a local HTTP server that exposes:
//   - GET /           → index.html
//   - GET /style.css  → style.css
//   - GET /app.js     → compiled TypeScript bundle
//   - GET /report.json → the report file from reportPath
//
// It listens on the given port and logs the URL to stdout.
// The function blocks until the server is stopped.
func Serve(reportPath string, port int) error {
	distFS, err := fs.Sub(staticFiles, "static/dist")
	if err != nil {
		return fmt.Errorf("server: embed sub: %w", err)
	}

	mux := http.NewServeMux()

	// Serve the generated report JSON
	mux.HandleFunc("/report.json", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(reportPath)
		if err != nil {
			http.Error(w, "report not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(data)
	})

	// Serve static assets from the embedded dist/ directory
	mux.Handle("/", http.FileServer(http.FS(distFS)))

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("server: listen %s: %w", addr, err)
	}

	url := fmt.Sprintf("http://%s", ln.Addr())
	log.Printf("Ollanta UI ready → %s\n", url)
	fmt.Printf("\nOpening report at %s\n(press Ctrl+C to stop)\n\n", url)

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Try to open the browser (best-effort, no error on failure)
	openBrowser(url)

	return srv.Serve(ln)
}

// ReportPath returns the canonical path for report.json inside projectDir.
func ReportPath(projectDir string) string {
	return filepath.Join(projectDir, ".ollanta", "report.json")
}
