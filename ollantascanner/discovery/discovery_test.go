package discovery_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/scovl/ollanta/ollantascanner/discovery"
)

func TestDiscover_GoFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "main.go"), "package main\n")
	write(t, filepath.Join(dir, "util.go"), "package main\n")

	files, err := discovery.Discover(dir, []string{"."}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
	for _, f := range files {
		if f.Language != "go" {
			t.Errorf("expected language=go, got %q", f.Language)
		}
	}
}

func TestDiscover_MultiLanguage(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "main.go"), "package main\n")
	write(t, filepath.Join(dir, "app.js"), "console.log(1)\n")
	write(t, filepath.Join(dir, "script.py"), "print(1)\n")

	files, err := discovery.Discover(dir, []string{"."}, nil)
	if err != nil {
		t.Fatal(err)
	}
	langs := map[string]int{}
	for _, f := range files {
		langs[f.Language]++
	}
	if langs["go"] != 1 || langs["javascript"] != 1 || langs["python"] != 1 {
		t.Errorf("unexpected language counts: %v", langs)
	}
}

func TestDiscover_IgnoresVendorDir(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "main.go"), "package main\n")
	vendorDir := filepath.Join(dir, "vendor")
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(vendorDir, "dep.go"), "package dep\n")

	files, err := discovery.Discover(dir, []string{"."}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file (vendor skipped), got %d", len(files))
	}
}

func TestDiscover_IgnoresGitDir(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "main.go"), "package main\n")
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(gitDir, "hook.go"), "package git\n")

	files, err := discovery.Discover(dir, []string{"."}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file (.git skipped), got %d", len(files))
	}
}

func TestDiscover_ExclusionGlob(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "main.go"), "package main\n")
	write(t, filepath.Join(dir, "main_test.go"), "package main\n")

	files, err := discovery.Discover(dir, []string{"."}, []string{"*_test.go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file (*_test.go excluded), got %d", len(files))
	}
}

func TestDiscover_NonexistentDir(t *testing.T) {
	files, err := discovery.Discover("/nonexistent/path", []string{"."}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty result for nonexistent dir, got %d", len(files))
	}
}

func TestDiscover_UnknownExtension(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "main.go"), "package main\n")
	write(t, filepath.Join(dir, "readme.md"), "# readme\n")
	write(t, filepath.Join(dir, "data.csv"), "a,b,c\n")

	files, err := discovery.Discover(dir, []string{"."}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("expected only 1 file (unknown extensions ignored), got %d", len(files))
	}
}

func TestDiscover_MultipleSourceDirs(t *testing.T) {
	dir := t.TempDir()
	subA := filepath.Join(dir, "a")
	subB := filepath.Join(dir, "b")
	if err := os.MkdirAll(subA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(subB, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(subA, "a.go"), "package a\n")
	write(t, filepath.Join(subB, "b.go"), "package b\n")

	files, err := discovery.Discover(dir, []string{"a", "b"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files from 2 source dirs, got %d", len(files))
	}
}

func TestDiscover_NoDuplicates(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "main.go"), "package main\n")

	// Same dir listed twice
	files, err := discovery.Discover(dir, []string{".", "."}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file (deduplication), got %d", len(files))
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
