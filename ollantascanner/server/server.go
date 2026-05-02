// Package server provides a lightweight local HTTP server that serves the
// Ollanta report as an interactive web UI. Static assets (HTML/CSS/JS) are
// embedded at compile time via go:embed so the binary has zero runtime
// dependencies.
package server

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	telemetry "github.com/scovl/ollanta/adapter/secondary/telemetry"
	"github.com/scovl/ollanta/ollantacore/branding"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

//go:embed static/dist
var staticFiles embed.FS

const (
	contentTypeHeader  = "Content-Type"
	cacheControlHeader = "Cache-Control"
	jsonContentType    = "application/json"
)

// Serve starts a local HTTP server that exposes:
//   - GET /           → index.html
//   - GET /style.css  → style.css
//   - GET /app.js     → compiled TypeScript bundle
//   - GET /report.json → the report file from reportPath
//
// It listens on bind:port (e.g. "127.0.0.1" for local-only, "0.0.0.0" for
// container/remote access) and logs the URL to stdout.
// The function blocks until the server is stopped.
func Serve(reportPath, bind string, port int) error {
	logger := slog.Default().With("component", "ollantascanner.server")
	projectRoot := filepath.Dir(filepath.Dir(reportPath))
	metricsReg := telemetry.NewRegistry()
	appMetrics := telemetry.NewMetrics(metricsReg)

	distFS, err := fs.Sub(staticFiles, "static/dist")
	if err != nil {
		return fmt.Errorf("server: embed sub: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/metrics", metricsReg.Handler())

	// Build rules map from the global registry (populated by init() in language packages).
	allMeta := ollantarules.Global().AllMeta()
	rules := make(map[string]*ollantarules.RuleMeta, len(allMeta))
	for _, m := range allMeta {
		rules[m.Key] = m
	}
	aiFixService := newAIFixService(projectRoot, rules, logger)

	mux.HandleFunc("/rules/", func(w http.ResponseWriter, r *http.Request) {
		key, _ := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/rules/"))
		if key == "" {
			w.Header().Set(contentTypeHeader, jsonContentType)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, `{"error":"missing rule key"}`)
			return
		}
		rule, ok := rules[key]
		if !ok {
			w.Header().Set(contentTypeHeader, jsonContentType)
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"error":"rule not found"}`)
			return
		}
		w.Header().Set(contentTypeHeader, jsonContentType)
		_ = json.NewEncoder(w).Encode(rule)
	})
	mux.HandleFunc("/api/ai/agents", aiFixService.handleAgents)
	mux.HandleFunc("/api/ai/providers", aiFixService.handleProviders)
	mux.HandleFunc("/api/ai/fixes/preview", aiFixService.handlePreview)
	mux.HandleFunc("/api/ai/fixes/apply", aiFixService.handleApply)
	mux.HandleFunc("/branding/ollanta-mark.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set(cacheControlHeader, "public, max-age=86400")
		if _, err := w.Write(branding.MarkPNG()); err != nil {
			slog.Error("write brand mark response", "error", err)
		}
	})

	// Serve the generated report JSON
	mux.HandleFunc("/report.json", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(reportPath)
		if err != nil {
			http.Error(w, "report not found", http.StatusNotFound)
			return
		}
		w.Header().Set(contentTypeHeader, jsonContentType)
		w.Header().Set(cacheControlHeader, "no-cache")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("/api/files/source", handleSourceFile(projectRoot))

	// Serve static assets from the embedded dist/ directory
	staticHandler := http.FileServer(http.FS(distFS))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		staticHandler.ServeHTTP(w, r)
	}))

	ln, err := listenWithLocalFallback(bind, port)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s", ln.Addr())
	logger.Info("Ollanta UI ready", "url", url)
	fmt.Printf("\nOpening report at %s\n(press Ctrl+C to stop)\n\n", url)

	srv := &http.Server{
		Handler:      telemetry.WrapHTTPHandler("ollantascanner", telemetry.TraceIDMiddleware(withObservability(mux, appMetrics))),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Try to open the browser (best-effort, no error on failure)
	openBrowser(url)

	return srv.Serve(ln)
}

func listenWithLocalFallback(bind string, port int) (net.Listener, error) {
	addr := fmt.Sprintf("%s:%d", bind, port)
	ln, err := net.Listen("tcp", addr)
	if err == nil {
		return ln, nil
	}
	if !isLocalBind(bind) || !isAddrInUse(err) {
		return nil, fmt.Errorf("server: listen %s: %w", addr, err)
	}

	for candidate := port + 1; candidate <= port+20; candidate++ {
		candidateAddr := fmt.Sprintf("%s:%d", bind, candidate)
		ln, listenErr := net.Listen("tcp", candidateAddr)
		if listenErr == nil {
			fmt.Fprintf(os.Stderr, "warning: %s is already in use; using %s instead\n", addr, candidateAddr)
			return ln, nil
		}
		if !isAddrInUse(listenErr) {
			return nil, fmt.Errorf("server: listen %s: %w", candidateAddr, listenErr)
		}
	}

	return nil, fmt.Errorf("server: listen %s: address is in use and no free fallback port was found", addr)
}

func isLocalBind(bind string) bool {
	switch strings.TrimSpace(strings.ToLower(bind)) {
	case "", "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func isAddrInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE) || strings.Contains(strings.ToLower(err.Error()), "address already in use") || strings.Contains(strings.ToLower(err.Error()), "only one usage of each socket address")
}

// ReportPath returns the canonical path for report.json inside projectDir.
func ReportPath(projectDir string) string {
	return filepath.Join(projectDir, ".ollanta", "report.json")
}

func handleSourceFile(projectRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		cleanPath, ok := cleanSourcePath(r.URL.Query().Get("path"))
		if !ok {
			writeJSONError(w, http.StatusBadRequest, "path must stay inside the project")
			return
		}
		writeSourceFile(w, projectRoot, cleanPath)
	}
}

func cleanSourcePath(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}
	cleanPath := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(cleanPath) || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", false
	}
	return cleanPath, true
}

func writeSourceFile(w http.ResponseWriter, projectRoot, cleanPath string) {
	fullPath := filepath.Join(projectRoot, cleanPath)
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		writeJSONError(w, http.StatusNotFound, "file not found")
		return
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "read file failed")
		return
	}
	content := string(data)
	writeJSON(w, http.StatusOK, map[string]any{
		"file": map[string]any{
			"path":       filepath.ToSlash(cleanPath),
			"language":   languageFromPath(cleanPath),
			"content":    content,
			"size_bytes": info.Size(),
			"line_count": countSourceLines(content),
		},
	})
}

func languageFromPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".json":
		return "json"
	case ".md":
		return "markdown"
	default:
		return "plain text"
	}
}

func countSourceLines(content string) int {
	if content == "" {
		return 0
	}
	lines := strings.Count(content, "\n")
	if !strings.HasSuffix(content, "\n") {
		lines++
	}
	return lines
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func withObservability(next http.Handler, metrics *telemetry.Metrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r)

		duration := time.Since(start)
		metrics.ObserveHTTPRequest(duration)
		attrs := telemetry.WithTraceAttrs(
			r.Context(),
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", duration.Milliseconds(),
		)
		if rec.status >= http.StatusInternalServerError || duration >= 5*time.Second {
			slog.WarnContext(r.Context(), "request completed", attrs...)
			return
		}
		slog.InfoContext(r.Context(), "request completed", attrs...)
	})
}
