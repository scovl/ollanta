package ollantascanner_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scovl/ollanta/ollantascanner/scan"
)

// writeFile is a helper that creates a file with the given content in dir.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestIntegration_GoProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "clean.go", "package p\nfunc Small() {}\n")
	writeFile(t, dir, "big.go", "package p\nfunc Big() {\n"+strings.Repeat("\t_ = 1\n", 42)+"}\n")

	opts := &scan.ScanOptions{
		ProjectDir: dir,
		Sources:    []string{"."},
		ProjectKey: "test",
		Format:     "summary",
	}
	r, err := scan.Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if r.Measures.Files < 2 {
		t.Errorf("expected ≥2 files, got %d", r.Measures.Files)
	}
	if len(r.Issues) == 0 {
		t.Error("expected issues for large function")
	}
}

func TestIntegration_MultiLanguage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "big.go", "package p\nfunc Big() {\n"+strings.Repeat("\t_ = 1\n", 42)+"}\n")
	writeFile(t, dir, "big.js", "function big() {\n"+strings.Repeat("  const x = 1;\n", 42)+"}\n")

	opts := &scan.ScanOptions{
		ProjectDir: dir,
		Sources:    []string{"."},
		ProjectKey: "multi",
		Format:     "summary",
	}
	r, err := scan.Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if r.Measures.ByLang["go"] == 0 {
		t.Error("expected Go files in report")
	}
	if r.Measures.ByLang["javascript"] == 0 {
		t.Error("expected JS files in report")
	}
	if len(r.Issues) < 2 {
		t.Errorf("expected ≥2 issues (one per language), got %d", len(r.Issues))
	}
}

func TestIntegration_ReportFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "big.go", "package p\nfunc Big() {\n"+strings.Repeat("\t_ = 1\n", 42)+"}\n")

	opts := &scan.ScanOptions{
		ProjectDir: dir,
		Sources:    []string{"."},
		ProjectKey: "rep",
		Format:     "all",
	}
	r, err := scan.Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	jsonPath, err := r.SaveJSON(dir)
	if err != nil {
		t.Fatalf("SaveJSON: %v", err)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Error("report.json not created")
	}

	sarifPath, err := r.SaveSARIF(dir)
	if err != nil {
		t.Fatalf("SaveSARIF: %v", err)
	}
	if _, err := os.Stat(sarifPath); err != nil {
		t.Error("report.sarif not created")
	}
}

func TestIntegration_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	opts := &scan.ScanOptions{
		ProjectDir: dir,
		Sources:    []string{"."},
		ProjectKey: "empty",
		Format:     "summary",
	}
	r, err := scan.Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if r.Measures.Files != 0 {
		t.Errorf("expected 0 files, got %d", r.Measures.Files)
	}
	if len(r.Issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(r.Issues))
	}
}

func TestIntegration_ExclusionFilter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package p\nfunc Small() {}\n")
	writeFile(t, dir, "big.go", "package p\nfunc Big() {\n"+strings.Repeat("\t_ = 1\n", 42)+"}\n")

	opts := &scan.ScanOptions{
		ProjectDir: dir,
		Sources:    []string{"."},
		Exclusions: []string{"big.go"},
		ProjectKey: "excl",
		Format:     "summary",
	}
	r, err := scan.Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if r.Measures.Files != 1 {
		t.Errorf("expected 1 file after exclusion, got %d", r.Measures.Files)
	}
	for _, iss := range r.Issues {
		if iss.ComponentPath == filepath.Join(dir, "big.go") {
			t.Error("excluded file should not produce issues")
		}
	}
}
