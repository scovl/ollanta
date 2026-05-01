package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

const (
	defaultOpenAIBaseURL = "https://api.openai.com/v1"
	defaultOpenAIModel   = "gpt-4.1-mini"
)

var defaultOpenAIModels = []string{"gpt-4.1-mini", "gpt-4.1", "gpt-4o-mini", "gpt-4o", "o4-mini"}

const methodNotAllowedMessage = "method not allowed"

type aiAgentConfig struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	BaseURL   string `json:"base_url,omitempty"`
	APIKeyEnv string `json:"api_key_env,omitempty"`
	APIKey    string `json:"-"`
}

type aiAgentView struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type aiProviderOption struct {
	ID             string   `json:"id"`
	Label          string   `json:"label"`
	Models         []string `json:"models"`
	DefaultModel   string   `json:"default_model"`
	Configured     bool     `json:"configured"`
	RequiresAPIKey bool     `json:"requires_api_key"`
}

type aiFixRequest struct {
	AgentID  string       `json:"agent_id,omitempty"`
	Provider string       `json:"provider,omitempty"`
	Model    string       `json:"model,omitempty"`
	APIKey   string       `json:"api_key,omitempty"`
	Issue    domain.Issue `json:"issue"`
}

type aiFixApplyRequest struct {
	PreviewID string `json:"preview_id"`
}

type aiFixPreviewResponse struct {
	PreviewID       string      `json:"preview_id"`
	Agent           aiAgentView `json:"agent"`
	Status          string      `json:"status"`
	Summary         string      `json:"summary"`
	Explanation     string      `json:"explanation"`
	Diff            string      `json:"diff"`
	FilePath        string      `json:"file_path"`
	StartLine       int         `json:"start_line"`
	EndLine         int         `json:"end_line"`
	OriginalSnippet string      `json:"original_snippet"`
	Replacement     string      `json:"replacement"`
}

type aiFixApplyResponse struct {
	PreviewID string `json:"preview_id"`
	Status    string `json:"status"`
	FilePath  string `json:"file_path"`
	Message   string `json:"message"`
}

type aiProviderRequest struct {
	Issue       domain.Issue
	Rule        *ollantarules.RuleMeta
	FilePath    string
	Snippet     string
	Context     string
	StartLine   int
	EndLine     int
	ProjectRoot string
}

type aiProviderResponse struct {
	Summary     string
	Explanation string
	Replacement string
}

type aiProvider interface {
	GenerateFix(ctx context.Context, agent aiAgentConfig, request aiProviderRequest) (*aiProviderResponse, error)
}

type storedPreview struct {
	response        *aiFixPreviewResponse
	filePath        string
	originalSnippet string
	originalFileHash string
	originalHash    string
	replacement     string
	startLine       int
	endLine         int
}

type aiFixService struct {
	projectRoot string
	rules       map[string]*ollantarules.RuleMeta
	agents      map[string]aiAgentConfig
	providerOptions []aiProviderOption
	providers   map[string]aiProvider
	previews    map[string]storedPreview
	mu          sync.Mutex
	logger      *slog.Logger
}

func newAIFixService(projectRoot string, rules map[string]*ollantarules.RuleMeta, logger *slog.Logger) *aiFixService {
	agents, err := loadAIAgentConfigsFromEnv()
	if err != nil {
		logger.Error("invalid AI agent configuration", "error", err)
		agents = nil
	}

	agentMap := make(map[string]aiAgentConfig, len(agents))
	for _, agent := range agents {
		agentMap[agent.ID] = agent
	}
	providerOptions := loadAIProviderOptionsFromEnv(agents)

	return &aiFixService{
		projectRoot: projectRoot,
		rules:       rules,
		agents:      agentMap,
		providerOptions: providerOptions,
		providers: map[string]aiProvider{
			"mock":   mockAIProvider{},
			"openai": openAIProvider{client: &http.Client{Timeout: 60 * time.Second}},
		},
		previews: make(map[string]storedPreview),
		logger:   logger,
	}
}

func loadAIAgentConfigsFromEnv() ([]aiAgentConfig, error) {
	if raw := strings.TrimSpace(os.Getenv("OLLANTA_AI_AGENTS")); raw != "" {
		var agents []aiAgentConfig
		if err := json.Unmarshal([]byte(raw), &agents); err != nil {
			return nil, fmt.Errorf("parse OLLANTA_AI_AGENTS: %w", err)
		}
		for i := range agents {
			normalizeAgentConfig(&agents[i])
		}
		if err := validateAgentConfigs(agents); err != nil {
			return nil, err
		}
		return agents, nil
	}

	agents := make([]aiAgentConfig, 0, 2)
	if key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY")); key != "" {
		_ = key
		agent := aiAgentConfig{
			ID:        "openai-default",
			Label:     envOrDefault("OLLANTA_AI_OPENAI_LABEL", "OpenAI"),
			Provider:  "openai",
			Model:     envOrDefault("OLLANTA_AI_OPENAI_MODEL", defaultOpenAIModel),
			BaseURL:   envOrDefault("OLLANTA_AI_OPENAI_BASE_URL", defaultOpenAIBaseURL),
			APIKeyEnv: "OPENAI_API_KEY",
		}
		agents = append(agents, agent)
	}
	if os.Getenv("OLLANTA_AI_ENABLE_MOCK") == "1" {
		agents = append(agents, aiAgentConfig{
			ID:       "mock-agent",
			Label:    "Mock AI",
			Provider: "mock",
			Model:    "deterministic",
		})
	}
	return agents, validateAgentConfigs(agents)
}

func loadAIProviderOptionsFromEnv(agents []aiAgentConfig) []aiProviderOption {
	openAIModels := parseAIModelListEnv("OLLANTA_AI_OPENAI_MODELS", defaultOpenAIModels)
	defaultModel := envOrDefault("OLLANTA_AI_OPENAI_MODEL", defaultOpenAIModel)
	if !containsString(openAIModels, defaultModel) {
		openAIModels = append([]string{defaultModel}, openAIModels...)
	}

	options := []aiProviderOption{{
		ID:             "openai",
		Label:          envOrDefault("OLLANTA_AI_OPENAI_LABEL", "OpenAI"),
		Models:         openAIModels,
		DefaultModel:   defaultModel,
		Configured:     strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != "",
		RequiresAPIKey: true,
	}}

	if os.Getenv("OLLANTA_AI_ENABLE_MOCK") == "1" {
		options = append(options, aiProviderOption{
			ID:             "mock",
			Label:          "Mock AI",
			Models:         []string{"deterministic"},
			DefaultModel:   "deterministic",
			Configured:     true,
			RequiresAPIKey: false,
		})
	}

	for _, agent := range agents {
		mergeProviderOption(&options, agent)
	}

	return options
}

func parseAIModelListEnv(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return append([]string(nil), fallback...)
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\r':
			return true
		default:
			return false
		}
	})
	models := make([]string, 0, len(parts))
	for _, part := range parts {
		model := strings.TrimSpace(part)
		if model == "" || containsString(models, model) {
			continue
		}
		models = append(models, model)
	}
	if len(models) == 0 {
		return append([]string(nil), fallback...)
	}
	return models
}

func mergeProviderOption(options *[]aiProviderOption, agent aiAgentConfig) {
	for i := range *options {
		if (*options)[i].ID != agent.Provider {
			continue
		}
		if agent.Label != "" && (*options)[i].Label == "" {
			(*options)[i].Label = agent.Label
		}
		if agent.Model != "" && !containsString((*options)[i].Models, agent.Model) {
			(*options)[i].Models = append((*options)[i].Models, agent.Model)
		}
		if (*options)[i].DefaultModel == "" {
			(*options)[i].DefaultModel = agent.Model
		}
		if agent.APIKeyEnv != "" && strings.TrimSpace(os.Getenv(agent.APIKeyEnv)) != "" {
			(*options)[i].Configured = true
		}
		return
	}

	configured := true
	if agent.APIKeyEnv != "" {
		configured = strings.TrimSpace(os.Getenv(agent.APIKeyEnv)) != ""
	}
	*options = append(*options, aiProviderOption{
		ID:             agent.Provider,
		Label:          agent.Label,
		Models:         []string{agent.Model},
		DefaultModel:   agent.Model,
		Configured:     configured,
		RequiresAPIKey: agent.APIKeyEnv != "",
	})
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validateAgentConfigs(agents []aiAgentConfig) error {
	seen := make(map[string]struct{}, len(agents))
	for _, agent := range agents {
		if agent.ID == "" {
			return errors.New("AI agent id is required")
		}
		if agent.Label == "" {
			return fmt.Errorf("AI agent %q is missing label", agent.ID)
		}
		if agent.Provider == "" {
			return fmt.Errorf("AI agent %q is missing provider", agent.ID)
		}
		if agent.Model == "" {
			return fmt.Errorf("AI agent %q is missing model", agent.ID)
		}
		switch agent.Provider {
		case "mock", "openai":
		default:
			return fmt.Errorf("AI agent %q uses unsupported provider %q", agent.ID, agent.Provider)
		}
		if _, ok := seen[agent.ID]; ok {
			return fmt.Errorf("duplicate AI agent id %q", agent.ID)
		}
		seen[agent.ID] = struct{}{}
	}
	return nil
}

func normalizeAgentConfig(agent *aiAgentConfig) {
	agent.ID = strings.TrimSpace(agent.ID)
	agent.Label = strings.TrimSpace(agent.Label)
	agent.Provider = strings.ToLower(strings.TrimSpace(agent.Provider))
	agent.Model = strings.TrimSpace(agent.Model)
	agent.BaseURL = strings.TrimSpace(agent.BaseURL)
	agent.APIKeyEnv = strings.TrimSpace(agent.APIKeyEnv)
	agent.APIKey = strings.TrimSpace(agent.APIKey)
	if agent.Provider == "openai" && agent.BaseURL == "" {
		agent.BaseURL = defaultOpenAIBaseURL
	}
	if agent.Provider == "openai" && agent.APIKeyEnv == "" {
		agent.APIKeyEnv = "OPENAI_API_KEY"
	}
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func (s *aiFixService) handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, methodNotAllowedMessage)
		return
	}

	agents := make([]aiAgentView, 0, len(s.agents))
	for _, agent := range s.agents {
		agents = append(agents, aiAgentView{ID: agent.ID, Label: agent.Label, Provider: agent.Provider, Model: agent.Model})
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": agents})
}

func (s *aiFixService) handleProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, methodNotAllowedMessage)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"providers": s.providerOptions})
}

func (s *aiFixService) handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, methodNotAllowedMessage)
		return
	}

	var request aiFixRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	response, err := s.generatePreview(r.Context(), request)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeJSONError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *aiFixService) handleApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, methodNotAllowedMessage)
		return
	}

	var request aiFixApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	response, err := s.applyPreview(r.Context(), request.PreviewID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeJSONError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *aiFixService) generatePreview(ctx context.Context, request aiFixRequest) (*aiFixPreviewResponse, error) {
	agent, err := s.resolveRequestedAgent(request)
	if err != nil {
		return nil, err
	}

	provider, ok := s.providers[agent.Provider]
	if !ok {
		return nil, fmt.Errorf("unsupported AI provider %q", agent.Provider)
	}

	filePath, err := s.resolveIssuePath(request.Issue.ComponentPath)
	if err != nil {
		return nil, err
	}

	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	content := string(contentBytes)

	startLine, endLine, snippet, contextSnippet, err := extractIssueSnippet(content, request.Issue)
	if err != nil {
		return nil, err
	}

	providerRequest := aiProviderRequest{
		Issue:       request.Issue,
		Rule:        s.rules[request.Issue.RuleKey],
		FilePath:    filePath,
		Snippet:     snippet,
		Context:     contextSnippet,
		StartLine:   startLine,
		EndLine:     endLine,
		ProjectRoot: s.projectRoot,
	}

	providerResponse, err := provider.GenerateFix(ctx, agent, providerRequest)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(providerResponse.Replacement) == "" {
		return nil, errors.New("AI agent returned an empty replacement")
	}

	previewID, err := randomID()
	if err != nil {
		return nil, fmt.Errorf("generate preview id: %w", err)
	}

	response := &aiFixPreviewResponse{
		PreviewID:       previewID,
		Agent:           aiAgentView{ID: agent.ID, Label: agent.Label, Provider: agent.Provider, Model: agent.Model},
		Status:          "ready",
		Summary:         strings.TrimSpace(providerResponse.Summary),
		Explanation:     strings.TrimSpace(providerResponse.Explanation),
		Diff:            buildUnifiedSnippetDiff(snippet, providerResponse.Replacement, startLine),
		FilePath:        filePath,
		StartLine:       startLine,
		EndLine:         endLine,
		OriginalSnippet: snippet,
		Replacement:     providerResponse.Replacement,
	}

	s.mu.Lock()
	s.previews[previewID] = storedPreview{
		response:        response,
		filePath:        filePath,
		originalSnippet: snippet,
		originalFileHash: hashText(content),
		originalHash:    hashText(snippet),
		replacement:     providerResponse.Replacement,
		startLine:       startLine,
		endLine:         endLine,
	}
	s.mu.Unlock()

	return response, nil
}

func (s *aiFixService) resolveRequestedAgent(request aiFixRequest) (aiAgentConfig, error) {
	if request.AgentID != "" {
		agent, ok := s.agents[request.AgentID]
		if !ok {
			return aiAgentConfig{}, fmt.Errorf("unknown AI agent %q", request.AgentID)
		}
		return agent, nil
	}

	providerID := strings.ToLower(strings.TrimSpace(request.Provider))
	if providerID == "" {
		return aiAgentConfig{}, errors.New("AI provider is required")
	}
	model := strings.TrimSpace(request.Model)
	if model == "" {
		return aiAgentConfig{}, errors.New("AI model is required")
	}

	option, ok := s.findProviderOption(providerID)
	if !ok {
		return aiAgentConfig{}, fmt.Errorf("unsupported AI provider %q", providerID)
	}

	agent := aiAgentConfig{
		ID:       providerID + ":" + model,
		Label:    option.Label,
		Provider: providerID,
		Model:    model,
		APIKey:   strings.TrimSpace(request.APIKey),
	}
	if providerID == "openai" {
		agent.BaseURL = envOrDefault("OLLANTA_AI_OPENAI_BASE_URL", defaultOpenAIBaseURL)
		agent.APIKeyEnv = "OPENAI_API_KEY"
	}
	normalizeAgentConfig(&agent)
	return agent, nil
}

func (s *aiFixService) findProviderOption(providerID string) (aiProviderOption, bool) {
	for _, option := range s.providerOptions {
		if option.ID == providerID {
			return option, true
		}
	}
	return aiProviderOption{}, false
}

func (s *aiFixService) applyPreview(ctx context.Context, previewID string) (*aiFixApplyResponse, error) {
	_ = ctx
	s.mu.Lock()
	preview, ok := s.previews[previewID]
	s.mu.Unlock()
	if !ok {
		return nil, os.ErrNotExist
	}

	contentBytes, err := os.ReadFile(preview.filePath)
	if err != nil {
		return nil, err
	}
	content := string(contentBytes)
	if hashText(content) != preview.originalFileHash {
		return nil, errors.New("target file changed after preview generation; regenerate the AI fix")
	}

	currentSnippet, err := extractLineRange(content, preview.startLine, preview.endLine)
	if err != nil {
		return nil, err
	}
	if hashText(currentSnippet) != preview.originalHash {
		return nil, errors.New("target file changed after preview generation; regenerate the AI fix")
	}

	updatedContent, err := replaceLineRange(content, preview.startLine, preview.endLine, preview.replacement)
	if err != nil {
		return nil, err
	}

	fileInfo, err := os.Stat(preview.filePath)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(preview.filePath, []byte(updatedContent), fileInfo.Mode()); err != nil {
		return nil, fmt.Errorf("write updated file: %w", err)
	}

	s.logger.Info("applied AI fix preview", "preview_id", previewID, "file", preview.filePath)

	return &aiFixApplyResponse{
		PreviewID: previewID,
		Status:    "applied",
		FilePath:  preview.filePath,
		Message:   "AI fix applied to local file",
	}, nil
}

func (s *aiFixService) resolveIssuePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("issue is missing component_path")
	}

	resolved := path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(s.projectRoot, path)
	}
	resolved = filepath.Clean(resolved)

	rel, err := filepath.Rel(s.projectRoot, resolved)
	if err != nil {
		return "", fmt.Errorf("resolve issue path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("issue file is outside the scanned project")
	}
	return resolved, nil
}

func extractIssueSnippet(content string, issue domain.Issue) (int, int, string, string, error) {
	if issue.Line <= 0 {
		return 0, 0, "", "", errors.New("issue line is invalid")
	}
	endLine := issue.EndLine
	if endLine <= 0 || endLine < issue.Line {
		endLine = issue.Line
	}

	snippet, err := extractLineRange(content, issue.Line, endLine)
	if err != nil {
		return 0, 0, "", "", err
	}
	contextStart := issue.Line - 2
	if contextStart < 1 {
		contextStart = 1
	}
	contextEnd := endLine + 2
	contextSnippet, err := extractLineRange(content, contextStart, contextEnd)
	if err != nil {
		return 0, 0, "", "", err
	}
	return issue.Line, endLine, snippet, contextSnippet, nil
}

func extractLineRange(content string, startLine, endLine int) (string, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return "", errors.New("file is empty")
	}
	if startLine <= 0 || startLine > len(lines) {
		return "", errors.New("issue line is outside the file")
	}
	if endLine < startLine {
		return "", errors.New("invalid issue line range")
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	return strings.Join(lines[startLine-1:endLine], "\n"), nil
}

func replaceLineRange(content string, startLine, endLine int, replacement string) (string, error) {
	lines := strings.Split(content, "\n")
	if startLine <= 0 || startLine > len(lines) {
		return "", errors.New("issue line is outside the file")
	}
	if endLine < startLine {
		return "", errors.New("invalid issue line range")
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}

	replacementLines := strings.Split(replacement, "\n")
	updated := append([]string{}, lines[:startLine-1]...)
	updated = append(updated, replacementLines...)
	updated = append(updated, lines[endLine:]...)
	return strings.Join(updated, "\n"), nil
}

func buildUnifiedSnippetDiff(original, replacement string, startLine int) string {
	originalLines := strings.Split(original, "\n")
	replacementLines := strings.Split(replacement, "\n")
	var builder strings.Builder
	_, _ = fmt.Fprintf(&builder, "@@ lines %d-%d @@\n", startLine, startLine+len(originalLines)-1)
	for _, line := range originalLines {
		builder.WriteString("- ")
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	for _, line := range replacementLines {
		builder.WriteString("+ ")
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	return strings.TrimRight(builder.String(), "\n")
}

func hashText(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func randomID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("encode JSON response", "error", err)
	}
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

type mockAIProvider struct{}

func (mockAIProvider) GenerateFix(_ context.Context, agent aiAgentConfig, request aiProviderRequest) (*aiProviderResponse, error) {
	replacement := strings.ReplaceAll(request.Snippet, "== nil", "is nil")
	if replacement == request.Snippet {
		replacement = request.Snippet + "\n// Fixed with " + agent.Label
	}
	return &aiProviderResponse{
		Summary:     "Mock AI fix generated",
		Explanation: "Mock provider generated a deterministic replacement for local validation.",
		Replacement: replacement,
	}, nil
}

type openAIProvider struct {
	client *http.Client
}

func (p openAIProvider) GenerateFix(ctx context.Context, agent aiAgentConfig, request aiProviderRequest) (*aiProviderResponse, error) {
	apiKey := strings.TrimSpace(agent.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv(agent.APIKeyEnv))
	}
	if apiKey == "" {
		return nil, fmt.Errorf("AI agent %q is missing API key in %s", agent.ID, agent.APIKeyEnv)
	}

	prompt := buildAIFixPrompt(request)
	body := map[string]any{
		"model": agent.Model,
		"messages": []map[string]string{
			{
				"role": "system",
				"content": "You fix one static-analysis issue in user code. Return strict JSON with keys summary, explanation, replacement. replacement must contain only the code snippet that replaces the original snippet, with no markdown fences.",
			},
			{
				"role": "user",
				"content": prompt,
			},
		},
		"temperature": 0.1,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal AI request: %w", err)
	}

	endpoint := strings.TrimRight(agent.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build AI request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call AI provider: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read AI response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("AI provider returned %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(bodyBytes, &completion); err != nil {
		return nil, fmt.Errorf("decode AI response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return nil, errors.New("AI provider returned no choices")
	}

	content := completion.Choices[0].Message.Content
	jsonPayload, err := extractJSONObject(content)
	if err != nil {
		return nil, err
	}

	var result aiProviderResponse
	if err := json.Unmarshal([]byte(jsonPayload), &result); err != nil {
		return nil, fmt.Errorf("decode AI payload: %w", err)
	}
	return &result, nil
}

func buildAIFixPrompt(request aiProviderRequest) string {
	var builder strings.Builder
	builder.WriteString("Fix the following static-analysis issue in the provided code snippet.\n")
	builder.WriteString("Return JSON with keys summary, explanation, replacement.\n\n")
	builder.WriteString("Issue\n")
	_, _ = fmt.Fprintf(&builder, "- rule_key: %s\n", request.Issue.RuleKey)
	_, _ = fmt.Fprintf(&builder, "- severity: %s\n", request.Issue.Severity)
	_, _ = fmt.Fprintf(&builder, "- message: %s\n", request.Issue.Message)
	_, _ = fmt.Fprintf(&builder, "- file_path: %s\n", request.FilePath)
	_, _ = fmt.Fprintf(&builder, "- line_range: %d-%d\n", request.StartLine, request.EndLine)
	if request.Rule != nil {
		if request.Rule.Description != "" {
			_, _ = fmt.Fprintf(&builder, "- rule_description: %s\n", request.Rule.Description)
		}
		if request.Rule.Rationale != "" {
			_, _ = fmt.Fprintf(&builder, "- rule_rationale: %s\n", request.Rule.Rationale)
		}
	}
	builder.WriteString("\nContext\n")
	builder.WriteString(request.Context)
	builder.WriteString("\n\nOriginal snippet\n")
	builder.WriteString(request.Snippet)
	return builder.String()
}

func extractJSONObject(content string) (string, error) {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end < start {
		return "", errors.New("AI provider did not return a JSON object")
	}
	return trimmed[start : end+1], nil
}