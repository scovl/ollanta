package ollantaparser_test

import (
	"testing"

	"github.com/scovl/ollanta/ollantaparser"
	"github.com/scovl/ollanta/ollantaparser/languages"
)

func TestQueryRunner_Run_FunctionNames_JS(t *testing.T) {
	src := []byte(`
function foo() {}
function bar() {}
const baz = () => {};
`)
	pf, err := ollantaparser.Parse("test.js", src, languages.JavaScript)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer pf.Close()

	qr := ollantaparser.NewQueryRunner()
	matches, err := qr.Run(pf, `(function_declaration name: (identifier) @fn.name)`, languages.JavaScript)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(matches) != 2 {
		t.Errorf("expected 2 function declarations, got %d", len(matches))
	}

	names := make([]string, 0, len(matches))
	for _, m := range matches {
		if node, ok := m.Captures["fn.name"]; ok {
			names = append(names, qr.Text(pf, node))
		}
	}
	if len(names) != 2 {
		t.Errorf("expected 2 captured names, got %v", names)
	}
}

func TestQueryRunner_Run_Python_Functions(t *testing.T) {
	src := []byte("def foo():\n    pass\n\ndef bar():\n    pass\n")
	pf, err := ollantaparser.Parse("test.py", src, languages.Python)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer pf.Close()

	qr := ollantaparser.NewQueryRunner()
	matches, err := qr.Run(pf, `(function_definition name: (identifier) @fn.name)`, languages.Python)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(matches) != 2 {
		t.Errorf("expected 2 function definitions, got %d", len(matches))
	}
}

func TestQueryRunner_Run_InvalidQuery(t *testing.T) {
	src := []byte(`function foo() {}`)
	pf, err := ollantaparser.Parse("test.js", src, languages.JavaScript)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer pf.Close()

	qr := ollantaparser.NewQueryRunner()
	_, err = qr.Run(pf, `(this_node_does_not_exist @bad)`, languages.JavaScript)
	if err == nil {
		t.Error("expected error for invalid query")
	}
}

func TestQueryRunner_Text(t *testing.T) {
	src := []byte(`function myFunction() {}`)
	pf, err := ollantaparser.Parse("test.js", src, languages.JavaScript)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer pf.Close()

	qr := ollantaparser.NewQueryRunner()
	matches, err := qr.Run(pf, `(function_declaration name: (identifier) @name)`, languages.JavaScript)
	if err != nil || len(matches) == 0 {
		t.Fatalf("Run failed or no matches: %v", err)
	}

	node := matches[0].Captures["name"]
	text := qr.Text(pf, node)
	if text != "myFunction" {
		t.Errorf("Text: got %q, want %q", text, "myFunction")
	}
}

func TestQueryRunner_Position(t *testing.T) {
	src := []byte("function foo() {}")
	pf, err := ollantaparser.Parse("test.js", src, languages.JavaScript)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer pf.Close()

	qr := ollantaparser.NewQueryRunner()
	matches, err := qr.Run(pf, `(function_declaration name: (identifier) @name)`, languages.JavaScript)
	if err != nil || len(matches) == 0 {
		t.Fatalf("Run failed or no matches: %v", err)
	}

	node := matches[0].Captures["name"]
	startLine, _, _, _ := qr.Position(node)
	if startLine != 1 {
		t.Errorf("expected startLine 1, got %d", startLine)
	}
}

func TestQueryRunner_Text_NilNode(t *testing.T) {
	qr := ollantaparser.NewQueryRunner()
	src := []byte(`x`)
	pf, _ := ollantaparser.Parse("t.js", src, languages.JavaScript)
	defer pf.Close()
	if qr.Text(pf, nil) != "" {
		t.Error("Text(nil) should return empty string")
	}
}
