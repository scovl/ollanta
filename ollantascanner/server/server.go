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

// codeSnapshotEntry is a parsed file from report.json's code_snapshot.
type codeSnapshotEntry struct {
	Path      string `json:"path"`
	Language  string `json:"language"`
	Content   string `json:"content,omitempty"`
	SizeBytes int    `json:"size_bytes,omitempty"`
	LineCount int    `json:"line_count,omitempty"`
}

// loadCodeSnapshot reads report.json and returns a path→file lookup of its
// embedded code_snapshot. Returns an empty map when the report is absent or the
// snapshot is empty.
func loadCodeSnapshot(reportPath string) map[string]codeSnapshotEntry {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil
	}
	var payload struct {
		CodeSnapshot struct {
			Files []codeSnapshotEntry `json:"files"`
		} `json:"code_snapshot"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}
	files := payload.CodeSnapshot.Files
	if len(files) == 0 {
		return nil
	}
	m := make(map[string]codeSnapshotEntry, len(files))
	for _, f := range files {
		m[f.Path] = f
	}
	return m
}

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

	snapshotFiles := loadCodeSnapshot(reportPath)

	distFS, err := fs.Sub(staticFiles, "static/dist")
	if err != nil {
		return fmt.Errorf("server: embed sub: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			logger.Error("write healthz response", "error", err)
		}
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			logger.Error("write readyz response", "error", err)
		}
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
			if _, err := fmt.Fprint(w, `{"error":"missing rule key"}`); err != nil {
				logger.Error("write rules error response", "error", err)
			}
			return
		}
		rule, ok := rules[key]
		if !ok {
			w.Header().Set(contentTypeHeader, jsonContentType)
			w.WriteHeader(http.StatusNotFound)
			if _, err := fmt.Fprint(w, `{"error":"rule not found"}`); err != nil {
				logger.Error("write rules error response", "error", err)
			}
			return
		}
		w.Header().Set(contentTypeHeader, jsonContentType)
		if err := json.NewEncoder(w).Encode(rule); err != nil {
			logger.Error("encode rule response", "key", key, "error", err)
		}
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
		if _, err := w.Write(data); err != nil {
			logger.Error("write report.json response", "error", err)
		}
	})
	mux.HandleFunc("/api/files/source", handleSourceFile(projectRoot, snapshotFiles))

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
	logger.Info("Ollanta UI ready", "url", url, "hint", "press Ctrl+C to stop")

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
			slog.Warn("port already in use; using fallback", "requested", addr, "using", candidateAddr)
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

func handleSourceFile(projectRoot string, snapshotFiles map[string]codeSnapshotEntry) http.HandlerFunc {
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
		writeSourceFile(w, projectRoot, cleanPath, snapshotFiles)
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

func writeSourceFile(w http.ResponseWriter, projectRoot, cleanPath string, snapshotFiles map[string]codeSnapshotEntry) {
	slashed := filepath.ToSlash(cleanPath)
	if entry, ok := snapshotFiles[slashed]; ok && entry.Content != "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"file": map[string]any{
				"path":       slashed,
				"language":   entry.Language,
				"content":    entry.Content,
				"size_bytes": entry.SizeBytes,
				"line_count": entry.LineCount,
			},
		})
		return
	}

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
			"path":       slashed,
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
