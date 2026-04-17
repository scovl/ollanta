package ollantaparser_test

import (
	"testing"

	"github.com/scovl/ollanta/ollantaparser"
	"github.com/scovl/ollanta/ollantaparser/languages"
)

func TestParse_JavaScript(t *testing.T) {
	src := []byte(`function greet(name) { return name; }`)
	pf, err := ollantaparser.Parse("sample.js", src, languages.JavaScript)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer pf.Close()

	if pf.Language != "javascript" {
		t.Errorf("Language: got %q", pf.Language)
	}
	if pf.RootNode() == nil {
		t.Error("RootNode should not be nil")
	}
	if pf.RootNode().HasError() {
		t.Error("syntax tree has errors")
	}
}

func TestParse_Python(t *testing.T) {
	src := []byte("def greet(name):\n    return name\n")
	pf, err := ollantaparser.Parse("sample.py", src, languages.Python)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer pf.Close()

	if pf.Language != "python" {
		t.Errorf("Language: got %q", pf.Language)
	}
	if pf.RootNode().HasError() {
		t.Error("syntax tree has errors")
	}
}

func TestParse_TypeScript(t *testing.T) {
	src := []byte(`function greet(name: string): void { console.log(name); }`)
	pf, err := ollantaparser.Parse("sample.ts", src, languages.TypeScript)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer pf.Close()

	if pf.Language != "typescript" {
		t.Errorf("Language: got %q", pf.Language)
	}
}

func TestParse_Rust(t *testing.T) {
	src := []byte(`fn greet(name: &str) { println!("{}", name); }`)
	pf, err := ollantaparser.Parse("sample.rs", src, languages.Rust)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer pf.Close()

	if pf.Language != "rust" {
		t.Errorf("Language: got %q", pf.Language)
	}
}

func TestParseFile_JavaScript(t *testing.T) {
	pf, err := ollantaparser.ParseFile("testdata/sample.js", languages.JavaScript)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	defer pf.Close()

	if pf.Path != "testdata/sample.js" {
		t.Errorf("Path: got %q", pf.Path)
	}
}

func TestParseFile_NotFound(t *testing.T) {
	_, err := ollantaparser.ParseFile("nonexistent.js", languages.JavaScript)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
