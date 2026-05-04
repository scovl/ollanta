package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCustomRuleAIModelsIncludesMockWhenEnabled(t *testing.T) {
	t.Setenv("OLLANTA_AI_ENABLE_MOCK", "1")
	t.Setenv("OPENAI_API_KEY", "")
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer ollama.Close()
	t.Setenv("OLLANTA_AI_OLLAMA_BASE_URL", ollama.URL)

	rec := httptest.NewRecorder()
	NewCustomRuleAIHandler().Models(rec, httptest.NewRequest(http.MethodGet, "/api/v1/custom-rules/ai/models", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"id":"mock"`) {
		t.Fatalf("body = %s, want mock provider", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"configured":true`) {
		t.Fatalf("body = %s, want configured provider", rec.Body.String())
	}
}

func TestCustomRuleAIModelsShowsOpenAISetupRequiredWithoutSecret(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer ollama.Close()
	t.Setenv("OLLANTA_AI_OLLAMA_BASE_URL", ollama.URL)

	rec := httptest.NewRecorder()
	NewCustomRuleAIHandler().Models(rec, httptest.NewRequest(http.MethodGet, "/api/v1/custom-rules/ai/models", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var response struct {
		Providers []customRuleAIProviderOption `json:"providers"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	provider := findAIProvider(response.Providers, customRuleAIProviderOpenAI)
	if provider == nil {
		t.Fatalf("providers = %+v, want openai", response.Providers)
	}
	if provider.Status != customRuleAIProviderStatusSetup || !provider.SetupRequired || provider.Configured {
		t.Fatalf("provider = %+v, want setup-required openai", *provider)
	}
	if strings.Contains(rec.Body.String(), "OPENAI_API_KEY") {
		t.Fatalf("body exposed env secret name: %s", rec.Body.String())
	}
}

func TestCustomRuleAIModelsUsesCurrentOpenAIDefaults(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer ollama.Close()
	t.Setenv("OLLANTA_AI_OLLAMA_BASE_URL", ollama.URL)

	rec := httptest.NewRecorder()
	NewCustomRuleAIHandler().Models(rec, httptest.NewRequest(http.MethodGet, "/api/v1/custom-rules/ai/models", nil))

	var response struct {
		Providers []customRuleAIProviderOption `json:"providers"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	provider := findAIProvider(response.Providers, customRuleAIProviderOpenAI)
	if provider == nil {
		t.Fatalf("providers = %+v, want openai", response.Providers)
	}
	if provider.DefaultModel != "gpt-5.5" {
		t.Fatalf("default model = %q, want gpt-5.5", provider.DefaultModel)
	}
	if !containsString(provider.Models, "gpt-5.4-mini") || containsString(provider.Models, "gpt-4o") {
		t.Fatalf("models = %+v, want current gpt-5 family defaults only", provider.Models)
	}
}

func TestCustomRuleAIModelsIncludesAnthropicClaudeDefaults(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer ollama.Close()
	t.Setenv("OLLANTA_AI_OLLAMA_BASE_URL", ollama.URL)

	rec := httptest.NewRecorder()
	NewCustomRuleAIHandler().Models(rec, httptest.NewRequest(http.MethodGet, "/api/v1/custom-rules/ai/models", nil))

	var response struct {
		Providers []customRuleAIProviderOption `json:"providers"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	provider := findAIProvider(response.Providers, customRuleAIProviderAnthropic)
	if provider == nil {
		t.Fatalf("providers = %+v, want anthropic", response.Providers)
	}
	if provider.DefaultModel != "claude-sonnet-4-6" {
		t.Fatalf("default model = %q, want claude-sonnet-4-6", provider.DefaultModel)
	}
	if !containsString(provider.Models, "claude-opus-4-7") || !containsString(provider.Models, "claude-haiku-4-5") {
		t.Fatalf("models = %+v, want current Claude defaults", provider.Models)
	}
	if provider.Status != customRuleAIProviderStatusSetup || !provider.SetupRequired || provider.Configured {
		t.Fatalf("provider = %+v, want setup-required anthropic", *provider)
	}
	if strings.Contains(rec.Body.String(), "ANTHROPIC_API_KEY") {
		t.Fatalf("body exposed env secret name: %s", rec.Body.String())
	}
}

func TestCustomRuleAIModelsIncludesKimiAndQwenDefaults(t *testing.T) {
	t.Setenv("MOONSHOT_API_KEY", "")
	t.Setenv("KIMI_API_KEY", "")
	t.Setenv("DASHSCOPE_API_KEY", "")
	t.Setenv("QWEN_API_KEY", "")
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer ollama.Close()
	t.Setenv("OLLANTA_AI_OLLAMA_BASE_URL", ollama.URL)

	rec := httptest.NewRecorder()
	NewCustomRuleAIHandler().Models(rec, httptest.NewRequest(http.MethodGet, "/api/v1/custom-rules/ai/models", nil))

	var response struct {
		Providers []customRuleAIProviderOption `json:"providers"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	kimi := findAIProvider(response.Providers, customRuleAIProviderKimi)
	if kimi == nil {
		t.Fatalf("providers = %+v, want kimi", response.Providers)
	}
	if kimi.DefaultModel != "kimi-k2.6" || !containsString(kimi.Models, "kimi-k2-thinking") {
		t.Fatalf("kimi provider = %+v, want Kimi defaults", *kimi)
	}
	if kimi.Status != customRuleAIProviderStatusSetup || !kimi.SetupRequired || kimi.Configured {
		t.Fatalf("kimi provider = %+v, want setup-required", *kimi)
	}
	qwen := findAIProvider(response.Providers, customRuleAIProviderQwen)
	if qwen == nil {
		t.Fatalf("providers = %+v, want qwen", response.Providers)
	}
	if qwen.DefaultModel != "qwen3.6-max-preview" || !containsString(qwen.Models, "qwen3.6-plus") {
		t.Fatalf("qwen provider = %+v, want Qwen defaults", *qwen)
	}
	if qwen.Status != customRuleAIProviderStatusSetup || !qwen.SetupRequired || qwen.Configured {
		t.Fatalf("qwen provider = %+v, want setup-required", *qwen)
	}
	for _, secretName := range []string{"MOONSHOT_API_KEY", "KIMI_API_KEY", "DASHSCOPE_API_KEY", "QWEN_API_KEY"} {
		if strings.Contains(rec.Body.String(), secretName) {
			t.Fatalf("body exposed env secret name %s: %s", secretName, rec.Body.String())
		}
	}
}

func TestCustomRuleAIModelsDiscoversOllamaModels(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("path = %s, want /api/tags", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"models":[{"name":"deepseek-coder:latest"},{"name":"qwen2.5-coder:latest"}]}`))
	}))
	defer ollama.Close()
	t.Setenv("OLLANTA_AI_OLLAMA_BASE_URL", ollama.URL)

	rec := httptest.NewRecorder()
	NewCustomRuleAIHandler().Models(rec, httptest.NewRequest(http.MethodGet, "/api/v1/custom-rules/ai/models", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var response struct {
		Providers []customRuleAIProviderOption `json:"providers"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	provider := findAIProvider(response.Providers, customRuleAIProviderOllama)
	if provider == nil {
		t.Fatalf("providers = %+v, want ollama", response.Providers)
	}
	if provider.Status != customRuleAIProviderStatusReady || !provider.Local || !provider.Configured {
		t.Fatalf("provider = %+v, want connected local ollama", *provider)
	}
	if !containsString(provider.Models, "deepseek-coder:latest") || len(provider.ModelOptions) != 2 {
		t.Fatalf("provider models = %+v options=%+v", provider.Models, provider.ModelOptions)
	}
}

func TestCustomRuleAISuggestMockReturnsExamples(t *testing.T) {
	t.Setenv("OLLANTA_AI_ENABLE_MOCK", "1")

	body := `{
  "provider": "mock",
  "model": "deterministic",
  "intent": "Flag console logging in production code",
  "current": {
    "namespace": "custom",
    "language": "javascript",
    "engine": "text",
    "type": "code_smell",
    "severity": "major"
  }
}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/custom-rules/ai/suggest", strings.NewReader(body))
	NewCustomRuleAIHandler().Suggest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Suggestion customRuleAISuggestion `json:"suggestion"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Suggestion.TextPattern == "" {
		t.Fatalf("text pattern is empty")
	}
	if response.Suggestion.NoncompliantExample == "" || response.Suggestion.CompliantExample == "" {
		t.Fatalf("examples were not generated: %+v", response.Suggestion)
	}
}

func TestCustomRuleAISuggestOllamaReturnsDraft(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("path = %s, want /api/chat", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"message":{"content":"{\"rule_id\":\"no-console\",\"name\":\"No console logging\",\"language\":\"javascript\",\"type\":\"code_smell\",\"severity\":\"major\",\"engine\":\"text\",\"text_pattern\":\"console\\\\.log\",\"noncompliant_example\":\"console.log('debug')\",\"compliant_example\":\"return true\",\"message\":\"Remove console logging\"}"}}`))
	}))
	defer ollama.Close()
	t.Setenv("OLLANTA_AI_OLLAMA_BASE_URL", ollama.URL)

	body := `{
  "provider": "ollama",
  "model": "deepseek-coder:latest",
  "intent": "Flag console logging in production code",
  "current": {
    "namespace": "custom",
    "language": "javascript",
    "engine": "text",
    "type": "code_smell",
    "severity": "major"
  }
}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/custom-rules/ai/suggest", strings.NewReader(body))
	NewCustomRuleAIHandler().Suggest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Suggestion customRuleAISuggestion `json:"suggestion"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Suggestion.RuleID != "no-console" || response.Suggestion.CompliantExample == "" || response.Suggestion.NoncompliantExample == "" {
		t.Fatalf("suggestion = %+v, want ollama draft with examples", response.Suggestion)
	}
}

func TestCustomRuleAISuggestOpenAIUsesResponsesAPI(t *testing.T) {
	var requestedPath string
	var requestedModel string
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		var payload struct {
			Model string `json:"model"`
			Input []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requestedModel = payload.Model
		if len(payload.Input) != 2 {
			t.Fatalf("input = %+v, want system and user messages", payload.Input)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"output_text": `{"rule_id":"no-console","name":"No console logging","language":"javascript","type":"code_smell","severity":"major","engine":"text","text_pattern":"console\\.log","noncompliant_example":"console.log('debug')","compliant_example":"return true","message":"Remove console logging"}`,
		})
	}))
	defer ai.Close()
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OLLANTA_AI_OPENAI_BASE_URL", ai.URL)
	t.Setenv("OLLANTA_AI_OPENAI_API", "responses")

	body := `{
  "provider": "openai",
  "model": "gpt-5.5",
  "intent": "Flag console logging in production code",
  "current": {
    "namespace": "custom",
    "language": "javascript",
    "engine": "text",
    "type": "code_smell",
    "severity": "major"
  }
}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/custom-rules/ai/suggest", strings.NewReader(body))
	NewCustomRuleAIHandler().Suggest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if requestedPath != "/responses" {
		t.Fatalf("path = %s, want /responses", requestedPath)
	}
	if requestedModel != "gpt-5.5" {
		t.Fatalf("model = %s, want gpt-5.5", requestedModel)
	}
	var response struct {
		Suggestion customRuleAISuggestion `json:"suggestion"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Suggestion.RuleID != "no-console" {
		t.Fatalf("suggestion = %+v, want response draft", response.Suggestion)
	}
}

func TestCustomRuleAISuggestAnthropicUsesMessagesAPI(t *testing.T) {
	var requestedPath string
	var requestedModel string
	var requestedVersion string
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedVersion = r.Header.Get(customRuleAIHeaderAnthropicVersion)
		if r.Header.Get(customRuleAIHeaderAnthropicKey) != "test-key" {
			t.Fatalf("missing Anthropic API key header")
		}
		var payload struct {
			Model    string `json:"model"`
			System   string `json:"system"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			MaxTokens int `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requestedModel = payload.Model
		if payload.System == "" || len(payload.Messages) != 1 || payload.Messages[0].Role != "user" || payload.MaxTokens == 0 {
			t.Fatalf("payload = %+v, want system prompt and one user message", payload)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]string{{
				"type": "text",
				"text": `{"rule_id":"no-console","name":"No console logging","language":"javascript","type":"code_smell","severity":"major","engine":"text","text_pattern":"console\\.log","noncompliant_example":"console.log('debug')","compliant_example":"return true","message":"Remove console logging"}`,
			}},
		})
	}))
	defer ai.Close()
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OLLANTA_AI_ANTHROPIC_BASE_URL", ai.URL)

	body := `{
  "provider": "anthropic",
  "model": "claude-sonnet-4-6",
  "intent": "Flag console logging in production code",
  "current": {
    "namespace": "custom",
    "language": "javascript",
    "engine": "text",
    "type": "code_smell",
    "severity": "major"
  }
}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/custom-rules/ai/suggest", strings.NewReader(body))
	NewCustomRuleAIHandler().Suggest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if requestedPath != "/messages" {
		t.Fatalf("path = %s, want /messages", requestedPath)
	}
	if requestedModel != "claude-sonnet-4-6" {
		t.Fatalf("model = %s, want claude-sonnet-4-6", requestedModel)
	}
	if requestedVersion != customRuleAIDefaultAnthropicVersion {
		t.Fatalf("anthropic-version = %s, want %s", requestedVersion, customRuleAIDefaultAnthropicVersion)
	}
	var response struct {
		Suggestion customRuleAISuggestion `json:"suggestion"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Suggestion.RuleID != "no-console" {
		t.Fatalf("suggestion = %+v, want anthropic draft", response.Suggestion)
	}
}

func TestCustomRuleAISuggestKimiUsesChatCompletions(t *testing.T) {
	assertCustomRuleAISuggestOpenAICompatibleProvider(t, customRuleAIProviderKimi, "kimi-k2.6", "MOONSHOT_API_KEY", "OLLANTA_AI_KIMI_BASE_URL", "test-kimi-key")
}

func TestCustomRuleAISuggestQwenUsesChatCompletions(t *testing.T) {
	assertCustomRuleAISuggestOpenAICompatibleProvider(t, customRuleAIProviderQwen, "qwen3.6-max-preview", "DASHSCOPE_API_KEY", "OLLANTA_AI_QWEN_BASE_URL", "test-qwen-key")
}

func assertCustomRuleAISuggestOpenAICompatibleProvider(t *testing.T, provider, model, keyEnv, baseURLEnv, apiKey string) {
	t.Helper()
	var requestedPath string
	var requestedModel string
	var requestedAuth string
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedAuth = r.Header.Get("Authorization")
		var payload struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requestedModel = payload.Model
		if len(payload.Messages) != 2 || payload.Messages[0].Role != "system" || payload.Messages[1].Role != "user" {
			t.Fatalf("messages = %+v, want system and user messages", payload.Messages)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]string{
					"content": `{"rule_id":"no-console","name":"No console logging","language":"javascript","type":"code_smell","severity":"major","engine":"text","text_pattern":"console\\.log","noncompliant_example":"console.log('debug')","compliant_example":"return true","message":"Remove console logging"}`,
				},
			}},
		})
	}))
	defer ai.Close()
	t.Setenv(keyEnv, apiKey)
	t.Setenv(baseURLEnv, ai.URL)

	body := fmt.Sprintf(`{
  "provider": %q,
  "model": %q,
  "intent": "Flag console logging in production code",
  "current": {
    "namespace": "custom",
    "language": "javascript",
    "engine": "text",
    "type": "code_smell",
    "severity": "major"
  }
}`, provider, model)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/custom-rules/ai/suggest", strings.NewReader(body))
	NewCustomRuleAIHandler().Suggest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if requestedPath != "/chat/completions" {
		t.Fatalf("path = %s, want /chat/completions", requestedPath)
	}
	if requestedModel != model {
		t.Fatalf("model = %s, want %s", requestedModel, model)
	}
	if requestedAuth != "Bearer "+apiKey {
		t.Fatalf("authorization = %s, want bearer key", requestedAuth)
	}
	var response struct {
		Suggestion customRuleAISuggestion `json:"suggestion"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Suggestion.RuleID != "no-console" {
		t.Fatalf("suggestion = %+v, want compatible provider draft", response.Suggestion)
	}
}

func findAIProvider(providers []customRuleAIProviderOption, id string) *customRuleAIProviderOption {
	for index := range providers {
		if providers[index].ID == id {
			return &providers[index]
		}
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
