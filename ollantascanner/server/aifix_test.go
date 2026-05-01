package server

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

type staticAIProvider struct {
	response *aiProviderResponse
	err      error
}

const testAgentID = "mock-agent"

func (p staticAIProvider) GenerateFix(_ context.Context, _ aiAgentConfig, _ aiProviderRequest) (*aiProviderResponse, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.response, nil
}

func TestAIFixServiceGeneratePreviewAndApply(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "main.go")
	content := "package main\n\nfunc demo(value any) {\n\tif value == nil {\n\t\tprintln(\"hi\")\n\t}\n}\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	service := newTestAIFixService(projectDir, staticAIProvider{response: &aiProviderResponse{
		Summary:     "Use a deterministic mock fix",
		Explanation: "Normalized provider response",
		Replacement: "\tif value is nil {",
	}})
	request := aiFixRequest{
		AgentID: testAgentID,
		Issue: domain.Issue{
			RuleKey:       "go:nil-check",
			ComponentPath: filePath,
			Line:          4,
			EndLine:       4,
			Message:       "Use the configured fix",
			Severity:      domain.SeverityMajor,
		},
	}

	preview, err := service.generatePreview(context.Background(), request)
	if err != nil {
		t.Fatalf("generatePreview: %v", err)
	}
	if preview.Agent.ID != testAgentID {
		t.Fatalf("expected preview agent mock-agent, got %s", preview.Agent.ID)
	}
	if !strings.Contains(preview.Diff, "+ \tif value is nil {") {
		t.Fatalf("expected diff to contain replacement, got %q", preview.Diff)
	}

	result, err := service.applyPreview(context.Background(), preview.PreviewID)
	if err != nil {
		t.Fatalf("applyPreview: %v", err)
	}
	if result.Status != "applied" {
		t.Fatalf("expected applied status, got %s", result.Status)
	}

	updated, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if !strings.Contains(string(updated), "if value is nil {") {
		t.Fatalf("expected updated file to contain AI fix, got %q", string(updated))
	}
}

func TestAIFixServiceGeneratePreviewFromSelectedModel(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "main.go")
	content := "package main\n\nfunc demo(value any) {\n\tif value == nil {\n\t\tprintln(\"hi\")\n\t}\n}\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	service := newTestAIFixService(projectDir, staticAIProvider{response: &aiProviderResponse{
		Summary:     "Use the selected model",
		Explanation: "Generated from a provider/model selection",
		Replacement: "\tif value is nil {",
	}})
	service.agents = map[string]aiAgentConfig{}

	preview, err := service.generatePreview(context.Background(), aiFixRequest{
		Provider: "mock",
		Model:    "deterministic",
		Issue: domain.Issue{
			RuleKey:       "go:nil-check",
			ComponentPath: filePath,
			Line:          4,
			EndLine:       4,
			Message:       "Use the selected model",
		},
	})
	if err != nil {
		t.Fatalf("generatePreview: %v", err)
	}
	if preview.Agent.Provider != "mock" {
		t.Fatalf("expected provider mock, got %s", preview.Agent.Provider)
	}
	if preview.Agent.Model != "deterministic" {
		t.Fatalf("expected model deterministic, got %s", preview.Agent.Model)
	}
}

func TestAIFixServiceRejectsStaleFile(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "main.go")
	content := "package main\n\nfunc demo(value any) {\n\tif value == nil {\n\t\tprintln(\"hi\")\n\t}\n}\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	service := newTestAIFixService(projectDir, staticAIProvider{response: &aiProviderResponse{
		Summary:     "Preview",
		Explanation: "Will fail on stale file",
		Replacement: "\tif value is nil {",
	}})

	preview, err := service.generatePreview(context.Background(), aiFixRequest{
		AgentID: testAgentID,
		Issue: domain.Issue{
			RuleKey:       "go:nil-check",
			ComponentPath: filePath,
			Line:          4,
			EndLine:       4,
			Message:       "Use the configured fix",
		},
	})
	if err != nil {
		t.Fatalf("generatePreview: %v", err)
	}

	if err := os.WriteFile(filePath, []byte(strings.Replace(content, "println", "fmt.Println", 1)), 0o644); err != nil {
		t.Fatalf("mutate file: %v", err)
	}

	_, err = service.applyPreview(context.Background(), preview.PreviewID)
	if err == nil {
		t.Fatal("expected stale file error")
	}
	if !strings.Contains(err.Error(), "changed after preview") {
		t.Fatalf("expected stale file message, got %v", err)
	}
}

func TestExtractIssueSnippetUsesIssueRange(t *testing.T) {
	content := strings.Join([]string{
		"package main",
		"",
		"func demo() {",
		"\tfirst()",
		"\tsecond()",
		"}",
	}, "\n")

	start, end, snippet, contextSnippet, err := extractIssueSnippet(content, domain.Issue{Line: 4, EndLine: 5})
	if err != nil {
		t.Fatalf("extractIssueSnippet: %v", err)
	}
	if start != 4 || end != 5 {
		t.Fatalf("expected lines 4-5, got %d-%d", start, end)
	}
	if snippet != "\tfirst()\n\tsecond()" {
		t.Fatalf("unexpected snippet %q", snippet)
	}
	if !strings.Contains(contextSnippet, "func demo() {") {
		t.Fatalf("expected surrounding context, got %q", contextSnippet)
	}
}

func TestLoadAIProviderOptionsIncludesOpenAIByDefault(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OLLANTA_AI_OPENAI_MODELS", "")
	t.Setenv("OLLANTA_AI_ENABLE_MOCK", "")

	options := loadAIProviderOptionsFromEnv(nil)
	if len(options) == 0 {
		t.Fatal("expected at least one AI provider option")
	}
	if options[0].ID != "openai" {
		t.Fatalf("expected first provider to be openai, got %s", options[0].ID)
	}
	if options[0].Configured {
		t.Fatal("expected openai option to be unconfigured without OPENAI_API_KEY")
	}
	if options[0].DefaultModel != defaultOpenAIModel {
		t.Fatalf("expected default model %s, got %s", defaultOpenAIModel, options[0].DefaultModel)
	}
	if !containsString(options[0].Models, defaultOpenAIModel) {
		t.Fatalf("expected models to contain %s, got %v", defaultOpenAIModel, options[0].Models)
	}
}

func newTestAIFixService(projectRoot string, provider aiProvider) *aiFixService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := newAIFixService(projectRoot, map[string]*ollantarules.RuleMeta{}, logger)
	service.agents = map[string]aiAgentConfig{
		testAgentID: {
			ID:       testAgentID,
			Label:    "Mock AI",
			Provider: "mock",
			Model:    "deterministic",
		},
	}
	service.providerOptions = []aiProviderOption{{
		ID:             "mock",
		Label:          "Mock AI",
		Models:         []string{"deterministic"},
		DefaultModel:   "deterministic",
		Configured:     true,
		RequiresAPIKey: false,
	}}
	service.providers = map[string]aiProvider{"mock": provider}
	return service
}