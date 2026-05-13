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

func parseGoSourceWithComments(t *testing.T, src string) *ollantarules.AnalysisContext {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
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
	issues := rules.NoLargeFunctions.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected at least one issue for large function")
	}
}

func TestNoLargeFunctions_Clean(t *testing.T) {
	ctx := parseGoFile(t, fixture("clean.go"))
	issues := rules.NoLargeFunctions.Check(ctx)
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
	issues := rules.NoLargeFunctions.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue with max_lines=2")
	}
}

func TestNoLargeFunctions_ReportsLineNumber(t *testing.T) {
	ctx := parseGoFile(t, fixture("large_function.go"))
	issues := rules.NoLargeFunctions.Check(ctx)
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
	` + strings.Repeat("\t_ = 1\n", 61) + `}`
	ctx := parseGoSource(t, src)
	issues := rules.NoLargeFunctions.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for large method")
	}
}

// ── NamingConventions ───────────────────────────────────────────────────────

func TestNamingConventions_Underscore(t *testing.T) {
	ctx := parseGoFile(t, fixture("bad_names.go"))
	issues := rules.NamingConventions.Check(ctx)
	// Expect at least Get_Value (underscore) and MAXSIZE (ALL_CAPS)
	if len(issues) < 2 {
		t.Errorf("expected ≥2 naming issues, got %d", len(issues))
	}
}

func TestNamingConventions_Clean(t *testing.T) {
	ctx := parseGoFile(t, fixture("clean.go"))
	issues := rules.NamingConventions.Check(ctx)
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
	issues := rules.NamingConventions.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues for valid MixedCaps names, got %d", len(issues))
	}
}

func TestNamingConventions_UnderscoreReportsName(t *testing.T) {
	src := `package p
func Get_Value() int { return 0 }
`
	ctx := parseGoSource(t, src)
	issues := rules.NamingConventions.Check(ctx)
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
	issues := rules.NoNakedReturns.Check(ctx)
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
	issues := rules.NoNakedReturns.Check(ctx)
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
	issues := rules.NoNakedReturns.Check(ctx)
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
	issues := rules.NoNakedReturns.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("no named returns — should not flag, got %d issues", len(issues))
	}
}

// ── UselessEqEq ─────────────────────────────────────────────────────────────

func TestUselessEqEq_DetectsSelfComparison(t *testing.T) {
	src := `package p
func check(a, b int) bool {
	return a == a
}`
	ctx := parseGoSource(t, src)
	issues := rules.UselessEqEq.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for self-comparison")
	}
}

func TestUselessEqEq_NoIssueOnDifferentOperands(t *testing.T) {
	src := `package p
func check(a, b int) bool {
	return a == b
}`
	ctx := parseGoSource(t, src)
	issues := rules.UselessEqEq.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

// ── UselessIfElse ───────────────────────────────────────────────────────────

func TestUselessIfElse_DetectsConstantTrue(t *testing.T) {
	src := `package p
func f() int {
	if true {
		return 1
	}
	return 0
}`
	ctx := parseGoSource(t, src)
	issues := rules.UselessIfElse.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for constant true condition")
	}
}

func TestUselessIfElse_NoIssueOnVariable(t *testing.T) {
	src := `package p
func f(x bool) int {
	if x {
		return 1
	}
	return 0
}`
	ctx := parseGoSource(t, src)
	issues := rules.UselessIfElse.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

// ── UseFilepathJoin ─────────────────────────────────────────────────────────

func TestUseFilepathJoin_DetectsStringConcat(t *testing.T) {
	src := `package p
func build(dir, file string) string {
	return dir + "/" + file
}`
	ctx := parseGoSource(t, src)
	issues := rules.UseFilepathJoin.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for path string concatenation")
	}
}

func TestUseFilepathJoin_NoIssueOnNormalConcat(t *testing.T) {
	src := `package p
func greet(a, b string) string {
	return a + " " + b
}`
	ctx := parseGoSource(t, src)
	issues := rules.UseFilepathJoin.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

// ── BadTmp ──────────────────────────────────────────────────────────────────

func TestBadTmp_DetectsHardcodedTmp(t *testing.T) {
	src := `package p
import "os"
func f() {
	os.Create("/tmp/data.txt")
}`
	ctx := parseGoSource(t, src)
	issues := rules.BadTmp.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for hardcoded /tmp/ path")
	}
}

func TestBadTmp_NoIssueOnCreateTemp(t *testing.T) {
	src := `package p
import "os"
func f() {
	os.CreateTemp("", "data-*.txt")
}`
	ctx := parseGoSource(t, src)
	issues := rules.BadTmp.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

// ── MathRandom ──────────────────────────────────────────────────────────────

func TestMathRandom_DetectsMathRand(t *testing.T) {
	src := `package p
import "math/rand"
func token() int {
	return rand.Intn(1000)
}`
	ctx := parseGoSource(t, src)
	issues := rules.MathRandom.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for math/rand usage")
	}
}

func TestMathRandom_NoIssueWithoutImport(t *testing.T) {
	src := `package p
func token() int {
	return 42
}`
	ctx := parseGoSource(t, src)
	issues := rules.MathRandom.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

// ── MD5UsedAsPassword ───────────────────────────────────────────────────────

func TestMD5UsedAsPassword_DetectsMD5(t *testing.T) {
	src := `package p
import "crypto/md5"
func hash(pwd string) [16]byte {
	return md5.Sum([]byte(pwd))
}`
	ctx := parseGoSource(t, src)
	issues := rules.MD5UsedAsPassword.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for md5 usage")
	}
}

func TestMD5UsedAsPassword_NoIssueWithoutImport(t *testing.T) {
	src := `package p
func hash(pwd string) string {
	return pwd
}`
	ctx := parseGoSource(t, src)
	issues := rules.MD5UsedAsPassword.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

// ── BindAll ─────────────────────────────────────────────────────────────────

func TestBindAll_DetectsAllInterfaces(t *testing.T) {
	src := `package p
import "net"
func listen() {
	net.Listen("tcp", "0.0.0.0:8080")
}`
	ctx := parseGoSource(t, src)
	issues := rules.BindAll.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for 0.0.0.0")
	}
}

func TestBindAll_DetectsIPv6All(t *testing.T) {
	src := `package p
import "net"
func listen() {
	net.Listen("tcp", "[::]:8080")
}`
	ctx := parseGoSource(t, src)
	issues := rules.BindAll.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for [::]")
	}
}

func TestBindAll_NoIssueOnLocalhost(t *testing.T) {
	src := `package p
import "net"
func listen() {
	net.Listen("tcp", "127.0.0.1:8080")
}`
	ctx := parseGoSource(t, src)
	issues := rules.BindAll.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

// ── MissingSSLMinVersion ────────────────────────────────────────────────────

func TestMissingSSLMinVersion_DetectsMissing(t *testing.T) {
	src := `package p
import "crypto/tls"
func cfg() *tls.Config {
	return &tls.Config{}
}`
	ctx := parseGoSource(t, src)
	issues := rules.MissingSSLMinVersion.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for missing MinVersion")
	}
}

func TestMissingSSLMinVersion_NoIssueWhenPresent(t *testing.T) {
	src := `package p
import "crypto/tls"
func cfg() *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS12}
}`
	ctx := parseGoSource(t, src)
	issues := rules.MissingSSLMinVersion.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

// ── WeakCrypto ──────────────────────────────────────────────────────────────

func TestWeakCrypto_DetectsDESImport(t *testing.T) {
	src := `package p
import "crypto/des"
func f() {
	des.NewCipher(nil)
}`
	ctx := parseGoSource(t, src)
	issues := rules.WeakCrypto.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for weak crypto")
	}
}

func TestWeakCrypto_NoIssueOnAES(t *testing.T) {
	src := `package p
import "crypto/aes"
func f() {
	aes.NewCipher(nil)
}`
	ctx := parseGoSource(t, src)
	issues := rules.WeakCrypto.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

// ── DecompressionBomb ───────────────────────────────────────────────────────

func TestDecompressionBomb_DetectsGzipNewReader(t *testing.T) {
	src := `package p
import "compress/gzip"
func f(r io.Reader) {
	gzip.NewReader(r)
}`
	ctx := parseGoSource(t, src)
	issues := rules.DecompressionBomb.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for gzip.NewReader")
	}
}

func TestDecompressionBomb_NoIssueOnSafeCode(t *testing.T) {
	src := `package p
func f() int { return 1 }`
	ctx := parseGoSource(t, src)
	issues := rules.DecompressionBomb.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

// TestDecompressionBomb_StillFlagsWhenLimitReaderWraps documents a known
// rule limitation: the check does not analyze whether io.LimitReader follows
// the gzip.NewReader call. Properly wrapped code is still flagged because
// the rule only looks at the call expression node, not the surrounding
// statements for size-limit mitigation.
func TestDecompressionBomb_StillFlagsWhenLimitReaderWraps(t *testing.T) {
	src := `package p
import "compress/gzip"
func f(r io.Reader) {
	gr, _ := gzip.NewReader(r)
	defer gr.Close()
	_ = io.LimitReader(gr, 200<<20)
}`
	ctx := parseGoSource(t, src)
	issues := rules.DecompressionBomb.Check(ctx)
	if len(issues) == 0 {
		t.Error("known limitation: rule does not detect io.LimitReader mitigation")
	}
	t.Log("rule flags gzip.NewReader even when io.LimitReader wraps it — mitigation must be verified manually")
}

// ── FilepathCleanMisuse ─────────────────────────────────────────────────────

func TestFilepathCleanMisuse_DetectsCleanInOpen(t *testing.T) {
	src := `package p
import (
	"os"
	"path/filepath"
)
func f(p string) {
	os.Open(filepath.Clean(p))
}`
	ctx := parseGoSource(t, src)
	issues := rules.FilepathCleanMisuse.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for filepath.Clean in os.Open")
	}
}

func TestFilepathCleanMisuse_NoIssueWithoutClean(t *testing.T) {
	src := `package p
import "os"
func f(p string) {
	os.Open(p)
}`
	ctx := parseGoSource(t, src)
	issues := rules.FilepathCleanMisuse.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

// ── LoopPointer ─────────────────────────────────────────────────────────────

func TestLoopPointer_DetectsRangeCapture(t *testing.T) {
	src := `package p
func f(items []int) {
	for _, item := range items {
		go func() {
			_ = item
		}()
	}
}`
	ctx := parseGoSource(t, src)
	issues := rules.LoopPointer.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for captured range variable")
	}
}

func TestLoopPointer_NoIssueWhenPassedAsArg(t *testing.T) {
	src := `package p
func f(items []int) {
	for _, item := range items {
		go func(i int) {
			_ = i
		}(item)
	}
}`
	ctx := parseGoSource(t, src)
	issues := rules.LoopPointer.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues when variable is passed as argument, got %d", len(issues))
	}
}

// ── CookieMissingHttponly ───────────────────────────────────────────────────

func TestCookieMissingHttponly_DetectsMissing(t *testing.T) {
	src := `package p
import "net/http"
func cookie() *http.Cookie {
	return &http.Cookie{Name: "session", Value: "abc"}
}`
	ctx := parseGoSource(t, src)
	issues := rules.CookieMissingHttponly.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for missing HttpOnly")
	}
}

func TestCookieMissingHttponly_NoIssueWhenPresent(t *testing.T) {
	src := `package p
import "net/http"
func cookie() *http.Cookie {
	return &http.Cookie{Name: "session", Value: "abc", HttpOnly: true}
}`
	ctx := parseGoSource(t, src)
	issues := rules.CookieMissingHttponly.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

// ── CookieMissingSecure ─────────────────────────────────────────────────────

func TestCookieMissingSecure_DetectsMissing(t *testing.T) {
	src := `package p
import "net/http"
func cookie() *http.Cookie {
	return &http.Cookie{Name: "session", Value: "abc"}
}`
	ctx := parseGoSource(t, src)
	issues := rules.CookieMissingSecure.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for missing Secure")
	}
}

func TestCookieMissingSecure_NoIssueWhenPresent(t *testing.T) {
	src := `package p
import "net/http"
func cookie() *http.Cookie {
	return &http.Cookie{Name: "session", Value: "abc", Secure: true}
}`
	ctx := parseGoSource(t, src)
	issues := rules.CookieMissingSecure.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

// ── TemplateHTMLDoesNotEscape ───────────────────────────────────────────────

func TestTemplateHTMLDoesNotEscape_DetectsDynamicInput(t *testing.T) {
	src := `package p
import "html/template"
func render(user string) template.HTML {
	return template.HTML(user)
}`
	ctx := parseGoSource(t, src)
	issues := rules.TemplateHTMLDoesNotEscape.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for template.HTML with dynamic input")
	}
}

func TestTemplateHTMLDoesNotEscape_NoIssueOnLiteral(t *testing.T) {
	src := `package p
import "html/template"
func render() template.HTML {
	return template.HTML("<b>safe</b>")
}`
	ctx := parseGoSource(t, src)
	issues := rules.TemplateHTMLDoesNotEscape.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues for string literal, got %d", len(issues))
	}
}

// ── UnsafeUsage ─────────────────────────────────────────────────────────────

func TestUnsafeUsage_DetectsUnsafePointer(t *testing.T) {
	src := `package p
import "unsafe"
func f(x int) unsafe.Pointer {
	return unsafe.Pointer(&x)
}`
	ctx := parseGoSource(t, src)
	issues := rules.UnsafeUsage.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for unsafe.Pointer")
	}
}

func TestUnsafeUsage_NoIssueWithoutUnsafe(t *testing.T) {
	src := `package p
func f() int { return 1 }`
	ctx := parseGoSource(t, src)
	issues := rules.UnsafeUsage.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

// ── ZipTraversal ────────────────────────────────────────────────────────────

func TestZipTraversal_DetectsOpenReader(t *testing.T) {
	src := `package p
import "archive/zip"
func f() {
	zip.OpenReader("archive.zip")
}`
	ctx := parseGoSource(t, src)
	issues := rules.ZipTraversal.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for zip.OpenReader")
	}
}

func TestZipTraversal_NoIssueWithoutZip(t *testing.T) {
	src := `package p
func f() int { return 1 }`
	ctx := parseGoSource(t, src)
	issues := rules.ZipTraversal.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

// ── SwitchNoDefault ─────────────────────────────────────────────────────────

func TestSwitchNoDefault_DetectsMissingDefault(t *testing.T) {
	src := `package p
func f(x int) {
	switch x {
	case 1:
		println("one")
	case 2:
		println("two")
	}
}`
	ctx := parseGoSource(t, src)
	issues := rules.SwitchNoDefault.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected issue for switch without default")
	}
}

func TestSwitchNoDefault_NoIssueWhenDefaultPresent(t *testing.T) {
	src := `package p
func f(x int) {
	switch x {
	case 1:
		println("one")
	default:
		println("other")
	}
}`
	ctx := parseGoSource(t, src)
	issues := rules.SwitchNoDefault.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issue for switch with default, got %d", len(issues))
	}
}

func TestSwitchNoDefault_NoIssueOnEmptySwitch(t *testing.T) {
	src := `package p
func f(x int) {
	switch x {
	}
}`
	ctx := parseGoSource(t, src)
	issues := rules.SwitchNoDefault.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issue for empty switch, got %d", len(issues))
	}
}

// ── CognitiveComplexity ──────────────────────────────────────────────────────

func TestCognitiveComplexity_DetectsComplexFunction(t *testing.T) {
	src := `package p
func Complex(x int) int {
	if x > 0 {
		for i := 0; i < x; i++ {
			if i%2 == 0 {
				switch i {
				case 1:
					return 1
				case 2:
					if x > 10 && i < 5 {
						return 2
					}
				}
			}
		}
	}
	return 0
}`
	ctx := parseGoSource(t, src)
	issues := rules.CognitiveComplexity.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected at least one issue for high cognitive complexity")
	}
}

func TestCognitiveComplexity_Clean(t *testing.T) {
	src := `package p
func Simple(x int) int {
	return x + 1
}`
	ctx := parseGoSource(t, src)
	issues := rules.CognitiveComplexity.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues in simple function, got %d", len(issues))
	}
}

// ── FunctionNestingDepth ─────────────────────────────────────────────────────

func TestFunctionNestingDepth_DetectsDeepNesting(t *testing.T) {
	src := `package p
func Deep(x int) int {
	if x > 0 {
		if x > 1 {
			for i := 0; i < x; i++ {
				if i%2 == 0 {
					switch i {
					case 1:
						return 1
					}
				}
			}
		}
	}
	return 0
}`
	ctx := parseGoSource(t, src)
	issues := rules.FunctionNestingDepth.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected at least one issue for deep nesting")
	}
}

func TestFunctionNestingDepth_Clean(t *testing.T) {
	src := `package p
func Flat(x int) int {
	return x + 1
}`
	ctx := parseGoSource(t, src)
	issues := rules.FunctionNestingDepth.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues in flat function, got %d", len(issues))
	}
}

// ── MagicNumber ──────────────────────────────────────────────────────────────

func TestMagicNumber_DetectsMagicNumbers(t *testing.T) {
	src := `package p
func f() int {
	x := 42
	return x + 99
}`
	ctx := parseGoSource(t, src)
	issues := rules.MagicNumber.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected at least one issue for magic numbers")
	}
}

func TestMagicNumber_NoIssueOnCommonValues(t *testing.T) {
	src := `package p
func f() int {
	x := 0
	y := 1
	z := -1
	return x + y + z
}`
	ctx := parseGoSource(t, src)
	issues := rules.MagicNumber.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues for authorized values (0, 1, -1), got %d", len(issues))
	}
}

func TestMagicNumber_NoIssueOnConst(t *testing.T) {
	src := `package p
const answer = 42
func f() int {
	return answer
}`
	ctx := parseGoSource(t, src)
	issues := rules.MagicNumber.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues for const declaration, got %d", len(issues))
	}
}

// ── TooManyParameters ────────────────────────────────────────────────────────

func TestTooManyParameters_Detects(t *testing.T) {
	src := `package p
func Many(a, b, c int, d, e, f string) {}
func Few(x int) {}
`
	ctx := parseGoSource(t, src)
	issues := rules.TooManyParameters.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected at least one issue for too many parameters")
	}
}

func TestTooManyParameters_Clean(t *testing.T) {
	src := `package p
func Few(a, b int) {}
func AlsoOK(x, y, z string) {}
`
	ctx := parseGoSource(t, src)
	issues := rules.TooManyParameters.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues for functions with few params, got %d", len(issues))
	}
}

// ── TodoComment ──────────────────────────────────────────────────────────────

func TestTodoComment_DetectsTODO(t *testing.T) {
	src := `package p
// TODO: implement this
func f() {}
`
	ctx := parseGoSourceWithComments(t, src)
	issues := rules.TodoComment.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected at least one issue for TODO comment")
	}
}

func TestTodoComment_DetectsFIXME(t *testing.T) {
	src := `package p
// FIXME: this is broken
func f() {}
`
	ctx := parseGoSourceWithComments(t, src)
	issues := rules.TodoComment.Check(ctx)
	if len(issues) == 0 {
		t.Error("expected at least one issue for FIXME comment")
	}
}

func TestTodoComment_NoIssueOnCleanComment(t *testing.T) {
	src := `package p
// This is a normal comment describing the function.
func f() {
	// Another normal comment
	x := 1
	_ = x
}
`
	ctx := parseGoSourceWithComments(t, src)
	issues := rules.TodoComment.Check(ctx)
	if len(issues) != 0 {
		t.Errorf("expected no issues for clean comments, got %d", len(issues))
	}
}
