package treesitter_test

import (
	"strings"
	"testing"

	parlanguages "github.com/scovl/ollanta/ollantaparser/languages"
	"github.com/scovl/ollanta/ollantarules/defaults"
	"github.com/scovl/ollanta/ollantarules/languages/treesitter"
)

func defaultSensor() *treesitter.TreeSitterSensor {
	return treesitter.NewTreeSitterSensor(defaults.NewRegistry(), parlanguages.DefaultRegistry())
}

func TestTreeSitterSensor_JS_LargeFunction(t *testing.T) {
	src := []byte("function bigFunc() {\n" + strings.Repeat("  const x = 1;\n", 42) + "}\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) == 0 {
		t.Error("expected issue for large JS function")
	}
}

func TestTreeSitterSensor_JS_SmallFunction(t *testing.T) {
	src := []byte("function small() { return 1; }\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "js:no-large-functions" {
			t.Error("small function should not be flagged")
		}
	}
}

func TestTreeSitterSensor_Python_LargeFunction(t *testing.T) {
	src := []byte("def big_func():\n" + strings.Repeat("    x = 1\n", 42) + "\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) == 0 {
		t.Error("expected issue for large Python function")
	}
}

func TestTreeSitterSensor_UnknownLanguage(t *testing.T) {
	s := defaultSensor()
	_, err := s.Analyze("test.rb", []byte("puts 'hello'"), "ruby", nil)
	if err == nil {
		t.Error("expected error for unsupported language")
	}
}

func TestTreeSitterSensor_IssueHasPositions(t *testing.T) {
	src := []byte("function bigFunc() {\n" + strings.Repeat("  const x = 1;\n", 42) + "}\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil || len(issues) == 0 {
		t.Fatalf("setup failed: err=%v issues=%d", err, len(issues))
	}
	if issues[0].Line <= 0 {
		t.Error("issue should have positive start line")
	}
	if issues[0].EndLine <= 0 {
		t.Error("issue should have positive end line")
	}
}

func TestTreeSitterSensor_CustomMaxLines(t *testing.T) {
	src := []byte("function medium() {\n" + strings.Repeat("  const x = 1;\n", 10) + "}\n")
	s := defaultSensor()
	// Filter to only js rule; cannot easily pass params via sensor directly,
	// but can verify the sensor honors active rules
	activeRules := map[string]bool{"js:no-large-functions": true}
	// 10 lines < default 40 — no issue
	issues, err := s.Analyze("test.js", src, "javascript", activeRules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "js:no-large-functions" {
			t.Errorf("10-line function should not be flagged with default threshold: %v", iss.Message)
		}
	}
}

// ── Wave 1 rule tests — Python ──────────────────────────────────────────────

func TestTreeSitterSensor_PY_UselessEqEq(t *testing.T) {
	src := []byte("if x == x:\n    pass\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "py:useless-eqeq" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected py:useless-eqeq issue")
	}
}

func TestTreeSitterSensor_PY_DictModifyIterating(t *testing.T) {
	src := []byte("for k in d.keys():\n    del d[k]\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "py:dict-modify-iterating" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected py:dict-modify-iterating issue")
	}
}

func TestTreeSitterSensor_PY_ReturnInInit(t *testing.T) {
	src := []byte("class User:\n    def __init__(self):\n        return\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "py:return-in-init" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected py:return-in-init issue")
	}
}

// ── Wave 1 rule tests — JavaScript ──────────────────────────────────────────

func TestTreeSitterSensor_JS_UselessEqEq(t *testing.T) {
	src := []byte("if (a == a) { return true; }\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "js:useless-eqeq" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected js:useless-eqeq issue")
	}
}

func TestTreeSitterSensor_JS_DetectEval(t *testing.T) {
	src := []byte("const r = eval(input);\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "js:detect-eval" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected js:detect-eval issue")
	}
}

func TestTreeSitterSensor_JS_DetectEval_NoFalsePositive(t *testing.T) {
	src := []byte("configureProjectFlowFeature({ render });\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "js:detect-eval" {
			t.Errorf("false positive: js:detect-eval flagged non-eval call %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_JS_DetectEval_NoFalsePositiveOtherCalls(t *testing.T) {
	src := []byte("bootBrowserApp();\napiFetch('/foo');\nrenderView();\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "js:detect-eval" {
			t.Errorf("false positive: js:detect-eval flagged non-eval call %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_JS_LeftoverDebugging(t *testing.T) {
	src := []byte("function f() { debugger; }\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "js:leftover-debugging" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected js:leftover-debugging issue")
	}
}

// ── Wave 2 rule tests — Python ──────────────────────────────────────────────

func TestTreeSitterSensor_PY_InsecureHash(t *testing.T) {
	src := []byte("import hashlib\nh = hashlib.md5(b'data').hexdigest()\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "py:insecure-hash" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected py:insecure-hash issue")
	}
}

func TestTreeSitterSensor_PY_DangerousSubprocess(t *testing.T) {
	src := []byte("import subprocess\nsubprocess.run('ls', shell=True)\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "py:dangerous-subprocess" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected py:dangerous-subprocess issue")
	}
}

func TestTreeSitterSensor_PY_DangerousSubprocess_NoFalsePositive(t *testing.T) {
	src := []byte("import mypkg\nmypkg.run('ls')\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "py:dangerous-subprocess" {
			t.Errorf("false positive: py:dangerous-subprocess flagged non-subprocess call %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_PY_DangerousOsExec(t *testing.T) {
	src := []byte("import os\nos.system('rm -rf /')\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "py:dangerous-os-exec" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected py:dangerous-os-exec issue")
	}
}

func TestTreeSitterSensor_PY_SyncSleepInAsync(t *testing.T) {
	src := []byte("import time\nasync def fetch():\n    time.sleep(1)\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "py:sync-sleep-in-async" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected py:sync-sleep-in-async issue")
	}
}

func TestTreeSitterSensor_PY_OpenNeverClosed(t *testing.T) {
	src := []byte("open('data.txt')\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "py:open-never-closed" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected py:open-never-closed issue")
	}
}

func TestTreeSitterSensor_PY_MissingHashWithEq(t *testing.T) {
	src := []byte("class Point:\n    def __eq__(self, other):\n        return self.x == other.x\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "py:missing-hash-with-eq" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected py:missing-hash-with-eq issue")
	}
}

func TestTreeSitterSensor_PY_UncheckedReturns(t *testing.T) {
	src := []byte("import os\nos.remove('file.txt')\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "py:unchecked-returns" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected py:unchecked-returns issue")
	}
}

func TestTreeSitterSensor_PY_UseDefusedXml(t *testing.T) {
	src := []byte("import xml.etree.ElementTree as ET\nET.parse('data.xml')\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "py:use-defused-xml" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected py:use-defused-xml issue")
	}
}

// ── Wave 2 rule tests — JavaScript / TypeScript ─────────────────────────────

func TestTreeSitterSensor_JS_DetectChildProcess(t *testing.T) {
	src := []byte("const cp = require('child_process');\ncp.exec('ls');\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "js:detect-child-process" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected js:detect-child-process issue")
	}
}

func TestTreeSitterSensor_JS_DetectInsecureWebsocket(t *testing.T) {
	src := []byte("const ws = new WebSocket('ws://example.com/socket');\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "js:detect-insecure-websocket" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected js:detect-insecure-websocket issue")
	}
}

func TestTreeSitterSensor_JS_DetectPseudoRandomBytes(t *testing.T) {
	src := []byte("const buf = crypto.pseudoRandomBytes(16);\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "js:detect-pseudoRandomBytes" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected js:detect-pseudoRandomBytes issue")
	}
}

func TestTreeSitterSensor_TS_UselessTernary(t *testing.T) {
	src := []byte("const ok = result ? true : false;\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.ts", src, "typescript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "ts:useless-ternary" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ts:useless-ternary issue")
	}
}

// ── Wave 3 rule tests — Python ──────────────────────────────────────────────

func TestTreeSitterSensor_PY_AvoidPyyamlLoad(t *testing.T) {
	src := []byte("import yaml\ndata = yaml.load(stream)\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "py:avoid-pyyaml-load" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected py:avoid-pyyaml-load issue")
	}
}

func TestTreeSitterSensor_PY_Pickle(t *testing.T) {
	src := []byte("import pickle\ndata = pickle.load(file)\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "py:pickle" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected py:pickle issue")
	}
}

func TestTreeSitterSensor_PY_Marshal(t *testing.T) {
	src := []byte("import marshal\ndata = marshal.load(file)\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "py:marshal" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected py:marshal issue")
	}
}

func TestTreeSitterSensor_PY_UnverifiedSSLContext(t *testing.T) {
	src := []byte("import ssl\nctx = ssl._create_unverified_context()\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "py:unverified-ssl-context" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected py:unverified-ssl-context issue")
	}
}

func TestTreeSitterSensor_PY_RegexDos(t *testing.T) {
	src := []byte("import re\nre.match(r'(a+)+', user_input)\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "py:regex-dos" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected py:regex-dos issue")
	}
}

// ── Wave 3 rule tests — JavaScript ──────────────────────────────────────────

func TestTreeSitterSensor_JS_DetectRedos(t *testing.T) {
	src := []byte("const re = /(a+)+/;\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "js:detect-redos" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected js:detect-redos issue")
	}
}

func TestTreeSitterSensor_JS_PathJoinResolveTraversal(t *testing.T) {
	src := []byte("const p = path.join(baseDir, req.query.file);\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "js:path-join-resolve-traversal" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected js:path-join-resolve-traversal issue")
	}
}

func TestTreeSitterSensor_JS_SpawnGitClone(t *testing.T) {
	src := []byte("spawn('git', ['clone', userUrl]);\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "js:spawn-git-clone" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected js:spawn-git-clone issue")
	}
}

func TestTreeSitterSensor_JS_IncompleteSanitization(t *testing.T) {
	src := []byte("const clean = input.replace(/</g, '');\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.RuleKey == "js:incomplete-sanitization" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected js:incomplete-sanitization issue")
	}
}

// TestTreeSitterSensor_JS_IncompleteSanitization_escHtml demonstrates the
// rule produces a false positive on escHtml's comprehensive HTML escaping
// chain. Each individual .replace() is part of a complete OWASP escape
// (covering & < > " '), but the rule flags each single-character pattern
// because it cannot see the chain context.
func TestTreeSitterSensor_JS_IncompleteSanitization_escHtml(t *testing.T) {
	src := []byte("function escHtml(s) {\n  return String(s)\n    .replace(/&/g, '&amp;')\n    .replace(/</g, '&lt;')\n    .replace(/>/g, '&gt;')\n    .replace(/\"/g, '&quot;')\n    .replace(/'/g, '&#39;');\n}\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fpCount := 0
	for _, iss := range issues {
		if iss.RuleKey == "js:incomplete-sanitization" {
			fpCount++
		}
	}
	// The rule flags each replace individually (5 false positives).
	// This test documents the known limitation — the rule does not
	// understand that single-character patterns are part of a chain.
	if fpCount == 0 {
		t.Error("expected false positives: escHtml chain should trigger the heuristic")
	}
}

// ─── Predicate negative tests (guardrail #13) ─────────────────────────────
// Each test provides input where the structural pattern matches but the
// predicate (#eq?, #match?) should reject it. These catch FilterPredicates
// regressions and prove rules don't fire where they shouldn't.

func TestTreeSitterSensor_JS_AssignedUndefined_NoFalsePositive(t *testing.T) {
	src := []byte("const x = initializeValue('x');\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "js:assigned-undefined" {
			t.Errorf("false positive: %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_JS_DetectInsecureWebsocket_NoFalsePositive(t *testing.T) {
	src := []byte("const ws = new MySocket('ws://example.com/socket');\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "js:detect-insecure-websocket" {
			t.Errorf("false positive: %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_JS_DetectRedos_NoFalsePositive(t *testing.T) {
	src := []byte("const re = /^[a-z]+$/;\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "js:detect-redos" {
			t.Errorf("false positive: %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_JS_LeftoverDebugging_NoFalsePositive(t *testing.T) {
	src := []byte("showAlert('hello');\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "js:leftover-debugging" {
			t.Errorf("false positive: %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_PY_AvoidPyyamlLoad_NoFalsePositive(t *testing.T) {
	src := []byte("import yamllib\ndata = yamllib.load(stream)\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "py:avoid-pyyaml-load" {
			t.Errorf("false positive: %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_PY_DangerousOsExec_NoFalsePositive(t *testing.T) {
	src := []byte("import myos\nmyos.system('echo hello')\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "py:dangerous-os-exec" {
			t.Errorf("false positive: %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_PY_InsecureHash_NoFalsePositive(t *testing.T) {
	src := []byte("import hashlib\nh = hashlib.sha256(b'data').hexdigest()\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "py:insecure-hash" {
			t.Errorf("false positive: %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_PY_Marshal_NoFalsePositive(t *testing.T) {
	src := []byte("import marshmallow\ndata = marshmallow.load(file)\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "py:marshal" {
			t.Errorf("false positive: %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_PY_Pickle_NoFalsePositive(t *testing.T) {
	src := []byte("import pickles\njar = pickles.load(file)\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "py:pickle" {
			t.Errorf("false positive: %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_PY_RegexDos_NoFalsePositive(t *testing.T) {
	src := []byte("import re\nre.compile(r'^[a-z]+$')\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "py:regex-dos" {
			t.Errorf("false positive: %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_PY_ReturnInInit_NoFalsePositive(t *testing.T) {
	src := []byte("class User:\n    def helper(self):\n        return 42\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "py:return-in-init" {
			t.Errorf("false positive: %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_PY_UncheckedReturns_NoFalsePositive(t *testing.T) {
	src := []byte("import myos\nmyos.remove('file.txt')\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "py:unchecked-returns" {
			t.Errorf("false positive: %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_PY_UseDefusedXml_NoFalsePositive(t *testing.T) {
	src := []byte("import mylxml as ET\nET.parse('data.xml')\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "py:use-defused-xml" {
			t.Errorf("false positive: %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_PY_UnverifiedSSLContext_NoFalsePositive(t *testing.T) {
	src := []byte("import ssl\nctx = ssl.SSLContext()\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "py:unverified-ssl-context" {
			t.Errorf("false positive: %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_TS_MomentDeprecated_NoFalsePositive(t *testing.T) {
	src := []byte("import dayjs from 'time';\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.ts", src, "typescript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "ts:moment-deprecated" {
			t.Errorf("false positive: %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_PY_OpenNeverClosed_NoFalsePositive(t *testing.T) {
	src := []byte("from mylib import open as copen\nf = copen('data.txt')\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "py:open-never-closed" {
			t.Errorf("false positive: %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_PY_ListModifyIterating_NoFalsePositive(t *testing.T) {
	src := []byte("items = [1,2,3]\nfor item in items:\n    cleaned.append(item)\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "py:list-modify-iterating" {
			t.Errorf("false positive: %q at line %d", iss.Message, iss.Line)
		}
	}
}

func TestTreeSitterSensor_PY_UnspecifiedOpenEncoding_NoFalsePositive(t *testing.T) {
	src := []byte("f = custom_open('data.txt', encoding='utf-8')\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "py:unspecified-open-encoding" {
			t.Errorf("false positive: %q at line %d", iss.Message, iss.Line)
		}
	}
}
