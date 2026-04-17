package rules_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/scovl/ollanta/ollantacore/constants"
	ollantarules "github.com/scovl/ollanta/ollantarules"
	"github.com/scovl/ollanta/ollantarules/languages/golang/rules"
)

// moduleRoot resolves the ollantarules module root relative to this test file.
func fixtureDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile: .../ollantarules/languages/golang/rules/rules_test.go
	// module root: go up 4 dirs
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	return filepath.Join(root, "testdata", "golang")
}

func fixture(name string) string {
	return filepath.Join(fixtureDir(), name)
}

func parseGoSource(t *testing.T, src string) *ollantarules.AnalysisContext {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return &ollantarules.AnalysisContext{
		Path:     "test.go",
		Source:   []byte(src),
		Language: constants.LangGo,
		Params:   map[string]string{},
		AST:      f,
		FileSet:  fset,
	}
}

func parseGoFile(t *testing.T, path string) *ollantarules.AnalysisContext {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return &ollantarules.AnalysisContext{
		Path:     path,
		Source:   src,
		Language: constants.LangGo,
		Params:   map[string]string{},
		AST:      f,
		FileSet:  fset,
	}
}

// ── NoLargeFunctions ────────────────────────────────────────────────────────

func TestNoLargeFunctions_Detects(t *testing.T) {
	ctx := parseGoFile(t, fixture("large_function.go"))
	r := &rules.NoLargeFunctions{}
	issues := r.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected at least one issue for large function")
	}
}

func TestNoLargeFunctions_Clean(t *testing.T) {
	ctx := parseGoFile(t, fixture("clean.go"))
	r := &rules.NoLargeFunctions{}
	issues := r.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues in clean file, got %d", len(issues))
	}
}

func TestNoLargeFunctions_CustomMaxLines(t *testing.T) {
	src := `package p
func Small() {
	x := 1
	_ = x
}`
	ctx := parseGoSource(t, src)
	ctx.Params["max_lines"] = "2" // very low threshold
	r := &rules.NoLargeFunctions{}
	issues := r.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue with max_lines=2")
	}
}

func TestNoLargeFunctions_ReportsLineNumber(t *testing.T) {
	ctx := parseGoFile(t, fixture("large_function.go"))
	r := &rules.NoLargeFunctions{}
	issues := r.Check(ctx)
	if len(issues) == 0 {
		t.Fatal("expected issues")
	}
	if issues[0].Line <= 0 {
		t.Error("issue should have positive line number")
	}
	if issues[0].Message == "" {
		t.Error("issue should have a message")
	}
}

func TestNoLargeFunctions_Method(t *testing.T) {
	src := `package p
type S struct{}
func (s S) Big() {
` + strings.Repeat("\t_ = 1\n", 41) + `}`
	ctx := parseGoSource(t, src)
	r := &rules.NoLargeFunctions{}
	issues := r.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for large method")
	}
}

// ── NamingConventions ───────────────────────────────────────────────────────

func TestNamingConventions_Underscore(t *testing.T) {
	ctx := parseGoFile(t, fixture("bad_names.go"))
	r := &rules.NamingConventions{}
	issues := r.Check(ctx)
	// Expect at least Get_Value (underscore) and MAXSIZE (ALL_CAPS)
	if len(issues) < 2 {
		t.Errorf("expected ≥2 naming issues, got %d", len(issues))
	}
}

func TestNamingConventions_Clean(t *testing.T) {
	ctx := parseGoFile(t, fixture("clean.go"))
	r := &rules.NamingConventions{}
	issues := r.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no naming issues in clean file, got %v", issues)
	}
}

func TestNamingConventions_ValidMixedCaps(t *testing.T) {
	src := `package p
func GetValue() int { return 0 }
func HTTPClient() {}
func ParseURL() string { return "" }
`
	ctx := parseGoSource(t, src)
	r := &rules.NamingConventions{}
	issues := r.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues for valid MixedCaps names, got %d", len(issues))
	}
}

func TestNamingConventions_UnderscoreReportsName(t *testing.T) {
	src := `package p
func Get_Value() int { return 0 }
`
	ctx := parseGoSource(t, src)
	r := &rules.NamingConventions{}
	issues := r.Check(ctx)
	if len(issues) == 0 {
		t.Fatal("expected issue")
	}
	if issues[0].Message == "" {
		t.Error("issue message should not be empty")
	}
}

// ── NoNakedReturns ──────────────────────────────────────────────────────────

func TestNoNakedReturns_Detects(t *testing.T) {
	ctx := parseGoFile(t, fixture("naked_returns.go"))
	r := &rules.NoNakedReturns{}
	issues := r.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected naked return issue in naked_returns.go")
	}
}

func TestNoNakedReturns_ShortFunctionNotFlagged(t *testing.T) {
	src := `package p
func short() (result int) {
	result = 1
	return
}
`
	ctx := parseGoSource(t, src)
	r := &rules.NoNakedReturns{}
	issues := r.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("short function naked return should not be flagged, got %d issues", len(issues))
	}
}

func TestNoNakedReturns_ExplicitReturn(t *testing.T) {
	src := `package p
func long() (result int, err error) {
	result = 42
	// padding
	// padding
	// padding
	// padding
	// padding
	// padding
	return result, nil
}
`
	ctx := parseGoSource(t, src)
	r := &rules.NoNakedReturns{}
	issues := r.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("explicit return should not be flagged, got %d issues", len(issues))
	}
}

func TestNoNakedReturns_NoNamedReturns(t *testing.T) {
	src := `package p
func noNamed() int {
	// padding
	// padding
	// padding
	// padding
	// padding
	// padding
	return 0
}
`
	ctx := parseGoSource(t, src)
	r := &rules.NoNakedReturns{}
	issues := r.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("no named returns — should not flag, got %d issues", len(issues))
	}
}
