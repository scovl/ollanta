package executor_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	parlanguages "github.com/scovl/ollanta/ollantaparser/languages"
	"github.com/scovl/ollanta/ollantarules/defaults"
	gosensor "github.com/scovl/ollanta/ollantarules/languages/golang"
	tssensor "github.com/scovl/ollanta/ollantarules/languages/treesitter"
	"github.com/scovl/ollanta/ollantascanner/discovery"
	"github.com/scovl/ollanta/ollantascanner/executor"
)

func newExecutor() *executor.Executor {
	reg := defaults.NewRegistry()
	parserReg := parlanguages.DefaultRegistry()
	return executor.New(
		gosensor.NewGoSensor(reg),
		tssensor.NewTreeSitterSensor(reg, parserReg),
	)
}

func TestExecutor_EmptyFiles(t *testing.T) {
	e := newExecutor()
	issues, err := e.Run(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues for empty file list, got %d", len(issues))
	}
}

func TestExecutor_RouteGoFile(t *testing.T) {
	dir := t.TempDir()
	src := "package p\nfunc Big() {\n" + strings.Repeat("\t_ = 1\n", 42) + "}\n"
	path := filepath.Join(dir, "big.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	e := newExecutor()
	files := []discovery.DiscoveredFile{{Path: path, Language: "go"}}
	issues, err := e.Run(context.Background(), files)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) == 0 {
		t.Error("expected issues for large Go function")
	}
}

func TestExecutor_RouteJSFile(t *testing.T) {
	dir := t.TempDir()
	src := "function big() {\n" + strings.Repeat("  const x = 1;\n", 42) + "}\n"
	path := filepath.Join(dir, "big.js")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	e := newExecutor()
	files := []discovery.DiscoveredFile{{Path: path, Language: "javascript"}}
	issues, err := e.Run(context.Background(), files)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) == 0 {
		t.Error("expected issues for large JS function")
	}
}

func TestExecutor_FaultIsolation(t *testing.T) {
	// Non-existent file — executor should not crash
	files := []discovery.DiscoveredFile{{Path: "/nonexistent/file.go", Language: "go"}}
	e := newExecutor()
	issues, err := e.Run(context.Background(), files)
	if err != nil {
		t.Fatal(err)
	}
	_ = issues // no crash is the test
}

func TestExecutor_MultiLanguage(t *testing.T) {
	dir := t.TempDir()

	goSrc := "package p\nfunc Big() {\n" + strings.Repeat("\t_ = 1\n", 42) + "}\n"
	goPath := filepath.Join(dir, "big.go")
	if err := os.WriteFile(goPath, []byte(goSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	jsSrc := "function big() {\n" + strings.Repeat("  const x = 1;\n", 42) + "}\n"
	jsPath := filepath.Join(dir, "big.js")
	if err := os.WriteFile(jsPath, []byte(jsSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	files := []discovery.DiscoveredFile{
		{Path: goPath, Language: "go"},
		{Path: jsPath, Language: "javascript"},
	}
	e := newExecutor()
	issues, err := e.Run(context.Background(), files)
	if err != nil {
		t.Fatal(err)
	}
	langs := map[string]bool{}
	for _, iss := range issues {
		if len(iss.RuleKey) > 2 {
			langs[iss.RuleKey[:2]] = true
		}
	}
	if !langs["go"] {
		t.Error("expected Go issues")
	}
	if !langs["js"] {
		t.Error("expected JS issues")
	}
}
