package configfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReturnsNotFoundWhenDefaultConfigDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	var cfg struct{}
	path, found, err := Load("", &cfg)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if found {
		t.Fatal("Load() found = true, want false")
	}
	if path != "" {
		t.Fatalf("Load() path = %q, want empty", path)
	}
}

func TestLoadDecodesDefaultConfigFromWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	if err := os.WriteFile(DefaultName, []byte("name = \"demo\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var cfg struct {
		Name string `toml:"name"`
	}
	path, found, err := Load("", &cfg)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !found {
		t.Fatal("Load() found = false, want true")
	}
	if cfg.Name != "demo" {
		t.Fatalf("cfg.Name = %q, want demo", cfg.Name)
	}
	if want := filepath.Join(dir, DefaultName); path != want {
		t.Fatalf("Load() path = %q, want %q", path, want)
	}
}

func TestLoadReturnsErrorWhenExplicitConfigDoesNotExist(t *testing.T) {
	t.Parallel()

	var cfg struct{}
	_, found, err := Load(filepath.Join(t.TempDir(), "missing.toml"), &cfg)
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if found {
		t.Fatal("Load() found = true, want false")
	}
}
