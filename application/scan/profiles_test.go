package scan

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
)

type testAnalyzer struct {
	key      string
	language string
	seen     map[string]string
}

func (a *testAnalyzer) Key() string                       { return a.key }
func (a *testAnalyzer) Name() string                      { return a.key }
func (a *testAnalyzer) Description() string               { return "" }
func (a *testAnalyzer) Language() string                  { return a.language }
func (a *testAnalyzer) Type() model.IssueType             { return model.TypeCodeSmell }
func (a *testAnalyzer) DefaultSeverity() model.Severity   { return model.SeverityMajor }
func (a *testAnalyzer) Tags() []string                    { return nil }
func (a *testAnalyzer) Params() map[string]model.ParamDef { return nil }
func (a *testAnalyzer) Check(ctx context.Context, ac port.AnalysisContext, issues *[]*model.Issue) error {
	a.seen = ac.Params
	*issues = append(*issues, &model.Issue{RuleKey: a.key, ComponentPath: ac.Path, Line: 1, Severity: model.SeverityMajor, Type: model.TypeCodeSmell})
	return ctx.Err()
}

func TestExecutorAppliesProfileFilterParamsAndSeverity(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	activeAnalyzer := &testAnalyzer{key: "go:todo-comment", language: model.LangGo}
	disabledAnalyzer := &testAnalyzer{key: "go:naming-conventions", language: model.LangGo}
	executor := NewExecutor(nil, []port.IAnalyzer{activeAnalyzer, disabledAnalyzer})
	executor.workers = 1
	policy := NewProfilePolicy([]*model.EffectiveQualityProfile{{
		Language: model.LangGo,
		Source:   model.ProfileSourceLocal,
		Rules: []*model.EffectiveRule{{
			RuleKey:  "go:todo-comment",
			Severity: string(model.SeverityCritical),
		}},
	}}, nil)

	issues, err := executor.Run(context.Background(), []DiscoveredFile{{Path: filePath, Language: model.LangGo}}, policy)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertAppliedProfile(t, issues, activeAnalyzer, disabledAnalyzer)
}

func assertAppliedProfile(t *testing.T, issues []*model.Issue, activeAnalyzer, disabledAnalyzer *testAnalyzer) {
	t.Helper()
	if len(issues) != 1 {
		t.Fatalf("issue count = %d, want 1", len(issues))
	}
	if issues[0].RuleKey != "go:todo-comment" {
		t.Fatalf("issue rule = %q, want go:todo-comment", issues[0].RuleKey)
	}
	if issues[0].Severity != model.SeverityCritical {
		t.Fatalf("issue severity = %q, want critical", issues[0].Severity)
	}
	if disabledAnalyzer.seen != nil {
		t.Fatal("disabled analyzer was executed")
	}
}

func TestResolveProfilePolicyLocalJSON(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "profiles.json")
	data := []byte(`{"version":1,"profiles":[{"language":"go","name":"Strict Go","rules":[{"key":"go:naming-conventions","severity":"critical"},{"key":"go:todo-comment","severity":"off"}]}]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	policy, err := ResolveProfilePolicy(context.Background(), &ScanOptions{Profiles: ProfileOptions{Source: ProfileSourceLocal, FilePath: path}}, []string{model.LangGo})
	if err != nil {
		t.Fatalf("ResolveProfilePolicy() error = %v", err)
	}
	rule, ok := policy.Rule(model.LangGo, "go:naming-conventions")
	if !ok {
		t.Fatal("expected go:naming-conventions to be active")
	}
	if rule.Severity != string(model.SeverityCritical) {
		t.Fatalf("rule = %+v, want critical", rule)
	}
	if _, ok := policy.Rule(model.LangGo, "go:todo-comment"); ok {
		t.Fatal("go:todo-comment should be disabled")
	}
	if snapshots := policy.Snapshots(); len(snapshots) != 1 || snapshots[0].ActiveRuleCount != 1 {
		t.Fatalf("snapshots = %+v, want one active rule", snapshots)
	}
}

func TestResolveProfilePolicyLocalYAML(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "profiles.yaml")
	data := []byte(`version: 1
profiles:
  - language: go
    name: YAML Go
    rules:
      - key: go:naming-conventions
        severity: critical
      - key: go:todo-comment
        active: false
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	policy, err := ResolveProfilePolicy(context.Background(), &ScanOptions{Profiles: ProfileOptions{Source: ProfileSourceLocal, FilePath: path}}, []string{model.LangGo})
	if err != nil {
		t.Fatalf("ResolveProfilePolicy() error = %v", err)
	}
	rule, ok := policy.Rule(model.LangGo, "go:naming-conventions")
	if !ok || rule.Severity != string(model.SeverityCritical) {
		t.Fatalf("rule=%+v ok=%v, want active YAML rule", rule, ok)
	}
	if _, ok := policy.Rule(model.LangGo, "go:todo-comment"); ok {
		t.Fatal("go:todo-comment should be disabled by YAML active=false")
	}
}

func TestResolveProfilePolicyRejectsUnsupportedVersionInStrictMode(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "profiles.json")
	if err := os.WriteFile(path, []byte(`{"version":99,"profiles":[{"language":"go","rules":[{"key":"go:todo-comment"}]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveProfilePolicy(context.Background(), &ScanOptions{Profiles: ProfileOptions{Source: ProfileSourceLocal, FilePath: path, Strict: true}}, []string{model.LangGo})
	if err == nil || !strings.Contains(err.Error(), "unsupported profile schema version") {
		t.Fatalf("error = %v, want unsupported version", err)
	}
}

func TestResolveProfilePolicyServerFallback(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	policy, err := ResolveProfilePolicy(context.Background(), &ScanOptions{
		ProjectKey: "demo",
		Server:     server.URL,
		Profiles:   ProfileOptions{Source: ProfileSourceServer, FetchTimeout: time.Second},
	}, []string{model.LangGo})
	if err != nil {
		t.Fatalf("ResolveProfilePolicy() error = %v", err)
	}
	if _, ok := policy.Rule(model.LangGo, "go:todo-comment"); !ok {
		t.Fatal("expected builtin fallback rule to be active")
	}
	if len(policy.Diagnostics()) == 0 {
		t.Fatal("expected server fallback diagnostic")
	}
}

func TestFetchServerEffectiveProfiles(t *testing.T) {
	t.Parallel()
	wantPath := "/api/v1/projects/demo/profiles/effective"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath {
			t.Fatalf("path = %q, want %q", r.URL.Path, wantPath)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("authorization = %q, want bearer token", got)
		}
		profiles := []*model.EffectiveQualityProfile{{Language: model.LangGo, ProfileName: "Remote", Source: model.ProfileSourceRemote, Rules: []*model.EffectiveRule{{RuleKey: "go:todo-comment", Severity: string(model.SeverityInfo)}}}}
		_ = json.NewEncoder(w).Encode(profiles)
	}))
	defer server.Close()

	profiles, err := fetchServerEffectiveProfiles(context.Background(), server.URL, "token", "demo", time.Second)
	if err != nil {
		t.Fatalf("fetchServerEffectiveProfiles() error = %v", err)
	}
	if len(profiles) != 1 || profiles[0].Rules[0].RuleKey != "go:todo-comment" {
		t.Fatalf("profiles = %+v", profiles)
	}
}
