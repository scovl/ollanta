package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	customRuleAIProviderMock            = "mock"
	customRuleAIProviderOpenAI          = "openai"
	customRuleAIProviderAnthropic       = "anthropic"
	customRuleAIProviderKimi            = "kimi"
	customRuleAIProviderQwen            = "qwen"
	customRuleAIProviderOllama          = "ollama"
	customRuleAIProviderStatusReady     = "connected"
	customRuleAIProviderStatusSetup     = "setup_required"
	customRuleAIProviderStatusDown      = "unavailable"
	customRuleAIDefaultOpenAIBaseURL    = "https://api.openai.com/v1"
	customRuleAIDefaultOpenAIModel      = "gpt-5.5"
	customRuleAIDefaultAnthropicBaseURL = "https://api.anthropic.com/v1"
	customRuleAIDefaultAnthropicModel   = "claude-sonnet-4-6"
	customRuleAIDefaultAnthropicVersion = "2023-06-01"
	customRuleAIDefaultKimiBaseURL      = "https://api.moonshot.ai/v1"
	customRuleAIDefaultKimiModel        = "kimi-k2.6"
	customRuleAIDefaultQwenBaseURL      = "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"
	customRuleAIDefaultQwenModel        = "qwen3.6-max-preview"
	customRuleAIDefaultOllamaBaseURL    = "http://host.docker.internal:11434"
	customRuleAIDefaultOllamaModel      = "llama3.1:latest"
	customRuleAIOpenAIAPIChat           = "chat"
	customRuleAIOpenAIAPIResponses      = "responses"
	customRuleAIHeaderContentType       = "Content-Type"
	customRuleAIContentTypeJSON         = "application/json"
	customRuleAIHeaderAnthropicKey      = "x-api-key"
	customRuleAIHeaderAnthropicVersion  = "anthropic-version"
	customRuleAISetupURL                = "#custom-rule-ai-provider-setup"
	customRuleAIReadyMessage            = "Ready for Rule Studio draft generation."
	customRuleAIEngineText              = "text"
	customRuleAIEngineGoAST             = "go-ast"
	customRuleAIEngineTreeSitter        = "tree-sitter"
)

var customRuleAIDefaultOpenAIModels = []string{"gpt-5.5", "gpt-5.4", "gpt-5.4-mini", "gpt-5.4-nano"}
var customRuleAIDefaultAnthropicModels = []string{"claude-opus-4-7", "claude-sonnet-4-6", "claude-haiku-4-5"}
var customRuleAIDefaultKimiModels = []string{"kimi-k2.6", "kimi-k2.5", "kimi-k2-0905-preview", "kimi-k2-turbo-preview", "kimi-k2-thinking"}
var customRuleAIDefaultQwenModels = []string{"qwen3.6-max-preview", "qwen3.6-plus", "qwen3.6-flash", "qwen3-coder-plus", "qwen3.6-35b-a3b"}

type CustomRuleAIHandler struct {
	client *http.Client
}

type customRuleAIProviderOption struct {
	ID             string                    `json:"id"`
	Label          string                    `json:"label"`
	Kind           string                    `json:"kind"`
	Status         string                    `json:"status"`
	Models         []string                  `json:"models"`
	ModelOptions   []customRuleAIModelOption `json:"model_options"`
	DefaultModel   string                    `json:"default_model"`
	Configured     bool                      `json:"configured"`
	SetupRequired  bool                      `json:"setup_required"`
	RequiresAPIKey bool                      `json:"requires_api_key"`
	Local          bool                      `json:"local"`
	SetupURL       string                    `json:"setup_url,omitempty"`
	Message        string                    `json:"message,omitempty"`
	BaseURL        string                    `json:"base_url,omitempty"`
	Diagnostics    []string                  `json:"diagnostics,omitempty"`
}

type customRuleAIModelOption struct {
	ID            string `json:"id"`
	Label         string `json:"label"`
	Status        string `json:"status"`
	Local         bool   `json:"local"`
	SetupRequired bool   `json:"setup_required"`
	Message       string `json:"message,omitempty"`
}

type customRuleAICloudChatProviderConfig struct {
	ID             string
	LabelEnv       string
	LabelFallback  string
	ModelsEnv      string
	FallbackModels []string
	ModelEnv       string
	DefaultModel   string
	BaseURLEnv     string
	DefaultBaseURL string
	APIKeyEnvs     []string
	SetupMessage   string
}

type customRuleAIProviderProbeError struct {
	Message       string
	SetupRequired bool
}

func (e customRuleAIProviderProbeError) Error() string {
	return e.Message
}

type customRuleAISuggestRequest struct {
	Provider string                   `json:"provider"`
	Model    string                   `json:"model"`
	Intent   string                   `json:"intent"`
	Current  customRuleAICurrentDraft `json:"current"`
}

type customRuleAICurrentDraft struct {
	PackName            string `json:"pack_name"`
	Namespace           string `json:"namespace"`
	RuleID              string `json:"rule_id"`
	Name                string `json:"name"`
	Language            string `json:"language"`
	Type                string `json:"type"`
	Severity            string `json:"severity"`
	Engine              string `json:"engine"`
	TextPattern         string `json:"text_pattern"`
	GoASTPattern        string `json:"go_ast_pattern"`
	Target              string `json:"target"`
	TreeSitterQuery     string `json:"tree_sitter_query"`
	NoncompliantExample string `json:"noncompliant_example"`
	CompliantExample    string `json:"compliant_example"`
	Message             string `json:"message"`
}

type customRuleAISuggestion struct {
	PackName            string `json:"pack_name,omitempty"`
	RuleID              string `json:"rule_id"`
	Name                string `json:"name"`
	Language            string `json:"language"`
	Type                string `json:"type"`
	Severity            string `json:"severity"`
	Engine              string `json:"engine"`
	TextPattern         string `json:"text_pattern,omitempty"`
	GoASTPattern        string `json:"go_ast_pattern,omitempty"`
	Target              string `json:"target,omitempty"`
	TreeSitterQuery     string `json:"tree_sitter_query,omitempty"`
	NoncompliantExample string `json:"noncompliant_example"`
	CompliantExample    string `json:"compliant_example"`
	Message             string `json:"message"`
}

func NewCustomRuleAIHandler() *CustomRuleAIHandler {
	return &CustomRuleAIHandler{client: &http.Client{Timeout: 60 * time.Second}}
}

// Models handles GET /api/v1/custom-rules/ai/models.
// @Summary List AI models
// @Description Returns available AI providers and models for rule drafting
// @Tags custom-rules-ai
// @Produce json
// @Success 200 {object} customRuleAIProvidersResponse
// @Router /api/v1/custom-rules/ai/models [get]
func (h *CustomRuleAIHandler) Models(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, http.StatusOK, map[string]any{"providers": h.customRuleAIProviders(r.Context())})
}

// Suggest handles POST /api/v1/custom-rules/ai/suggest.
// @Summary Suggest rule draft
// @Description Ask an AI provider to suggest or improve a custom rule draft
// @Tags custom-rules-ai
// @Accept json
// @Produce json
// @Param body body customRuleAISuggestRequest true "Suggestion request"
// @Success 200 {object} customRuleAISuggestResponse
// @Router /api/v1/custom-rules/ai/suggest [post]
func (h *CustomRuleAIHandler) Suggest(w http.ResponseWriter, r *http.Request) {
	var req customRuleAISuggestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
	req.Model = strings.TrimSpace(req.Model)
	req.Intent = strings.TrimSpace(req.Intent)
	if req.Intent == "" {
		jsonError(w, http.StatusBadRequest, "intent is required")
		return
	}
	if req.Provider == "" || req.Model == "" {
		jsonError(w, http.StatusBadRequest, "AI provider and model are required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	var suggestion customRuleAISuggestion
	var err error
	switch req.Provider {
	case customRuleAIProviderMock:
		suggestion = customRuleAIMockSuggestion(req)
	case customRuleAIProviderOpenAI:
		suggestion, err = h.openAISuggestion(ctx, req)
	case customRuleAIProviderAnthropic:
		suggestion, err = h.anthropicSuggestion(ctx, req)
	case customRuleAIProviderKimi:
		suggestion, err = h.kimiSuggestion(ctx, req)
	case customRuleAIProviderQwen:
		suggestion, err = h.qwenSuggestion(ctx, req)
	case customRuleAIProviderOllama:
		suggestion, err = h.ollamaSuggestion(ctx, req)
	default:
		err = fmt.Errorf("unsupported AI provider %q", req.Provider)
	}
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonOK(w, http.StatusOK, map[string]any{"suggestion": normalizeCustomRuleAISuggestion(suggestion, req)})
}

func (h *CustomRuleAIHandler) customRuleAIProviders(ctx context.Context) []customRuleAIProviderOption {
	providers := []customRuleAIProviderOption{h.openAIProvider(ctx)}
	providers = append(providers, h.anthropicProvider(ctx))
	providers = append(providers, h.kimiProvider(ctx))
	providers = append(providers, h.qwenProvider(ctx))
	providers = append(providers, h.ollamaProvider(ctx))

	if os.Getenv("OLLANTA_AI_ENABLE_MOCK") == "1" {
		providers = append(providers, customRuleAIProviderOption{
			ID:             customRuleAIProviderMock,
			Label:          "Mock AI",
			Kind:           "development",
			Status:         customRuleAIProviderStatusReady,
			Models:         []string{"deterministic"},
			ModelOptions:   customRuleAIModelOptions([]string{"deterministic"}, customRuleAIProviderStatusReady, true, "Deterministic development-only model."),
			DefaultModel:   "deterministic",
			Configured:     true,
			SetupRequired:  false,
			RequiresAPIKey: false,
			Local:          true,
			Message:        "Deterministic development-only model.",
		})
	}
	return providers
}

func (h *CustomRuleAIHandler) openAIProvider(ctx context.Context) customRuleAIProviderOption {
	models := customRuleAIModelListEnv("OLLANTA_AI_OPENAI_MODELS", customRuleAIDefaultOpenAIModels)
	defaultModel := customRuleAIEnvOrDefault("OLLANTA_AI_OPENAI_MODEL", customRuleAIDefaultOpenAIModel)
	if !customRuleAIContains(models, defaultModel) {
		models = append([]string{defaultModel}, models...)
	}
	baseURL := customRuleAIEnvOrDefault("OLLANTA_AI_OPENAI_BASE_URL", customRuleAIDefaultOpenAIBaseURL)
	provider := customRuleAICloudProviderOption(customRuleAIProviderOpenAI, customRuleAIEnvOrDefault("OLLANTA_AI_OPENAI_LABEL", "OpenAI"), models, defaultModel, baseURL, "Connect an OpenAI-compatible provider before using these models.")
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	return h.withCloudProviderHealth(ctx, provider, apiKey, func(ctx context.Context) error {
		return h.probeOpenAICompatibleModels(ctx, baseURL, apiKey)
	})
}

func (h *CustomRuleAIHandler) anthropicProvider(ctx context.Context) customRuleAIProviderOption {
	models := customRuleAIModelListEnv("OLLANTA_AI_ANTHROPIC_MODELS", customRuleAIDefaultAnthropicModels)
	defaultModel := customRuleAIEnvOrDefault("OLLANTA_AI_ANTHROPIC_MODEL", customRuleAIDefaultAnthropicModel)
	if !customRuleAIContains(models, defaultModel) {
		models = append([]string{defaultModel}, models...)
	}
	baseURL := customRuleAIEnvOrDefault("OLLANTA_AI_ANTHROPIC_BASE_URL", customRuleAIDefaultAnthropicBaseURL)
	provider := customRuleAICloudProviderOption(customRuleAIProviderAnthropic, customRuleAIEnvOrDefault("OLLANTA_AI_ANTHROPIC_LABEL", "Anthropic Claude"), models, defaultModel, baseURL, "Connect Anthropic Claude before using these models.")
	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	return h.withCloudProviderHealth(ctx, provider, apiKey, func(ctx context.Context) error {
		return h.probeAnthropicModels(ctx, baseURL, apiKey)
	})
}

func (h *CustomRuleAIHandler) kimiProvider(ctx context.Context) customRuleAIProviderOption {
	return h.customRuleAICloudChatProvider(ctx, customRuleAICloudChatProviderConfig{
		ID:             customRuleAIProviderKimi,
		LabelEnv:       "OLLANTA_AI_KIMI_LABEL",
		LabelFallback:  "Kimi K2",
		ModelsEnv:      "OLLANTA_AI_KIMI_MODELS",
		FallbackModels: customRuleAIDefaultKimiModels,
		ModelEnv:       "OLLANTA_AI_KIMI_MODEL",
		DefaultModel:   customRuleAIDefaultKimiModel,
		BaseURLEnv:     "OLLANTA_AI_KIMI_BASE_URL",
		DefaultBaseURL: customRuleAIDefaultKimiBaseURL,
		APIKeyEnvs:     []string{"MOONSHOT_API_KEY", "KIMI_API_KEY"},
		SetupMessage:   "Connect Kimi before using these models.",
	})
}

func (h *CustomRuleAIHandler) qwenProvider(ctx context.Context) customRuleAIProviderOption {
	return h.customRuleAICloudChatProvider(ctx, customRuleAICloudChatProviderConfig{
		ID:             customRuleAIProviderQwen,
		LabelEnv:       "OLLANTA_AI_QWEN_LABEL",
		LabelFallback:  "Qwen",
		ModelsEnv:      "OLLANTA_AI_QWEN_MODELS",
		FallbackModels: customRuleAIDefaultQwenModels,
		ModelEnv:       "OLLANTA_AI_QWEN_MODEL",
		DefaultModel:   customRuleAIDefaultQwenModel,
		BaseURLEnv:     "OLLANTA_AI_QWEN_BASE_URL",
		DefaultBaseURL: customRuleAIDefaultQwenBaseURL,
		APIKeyEnvs:     []string{"DASHSCOPE_API_KEY", "QWEN_API_KEY"},
		SetupMessage:   "Connect Qwen or DashScope before using these models.",
	})
}

func (h *CustomRuleAIHandler) customRuleAICloudChatProvider(ctx context.Context, config customRuleAICloudChatProviderConfig) customRuleAIProviderOption {
	models := customRuleAIModelListEnv(config.ModelsEnv, config.FallbackModels)
	selectedModel := customRuleAIEnvOrDefault(config.ModelEnv, config.DefaultModel)
	if !customRuleAIContains(models, selectedModel) {
		models = append([]string{selectedModel}, models...)
	}
	baseURL := customRuleAIEnvOrDefault(config.BaseURLEnv, config.DefaultBaseURL)
	provider := customRuleAICloudProviderOption(config.ID, customRuleAIEnvOrDefault(config.LabelEnv, config.LabelFallback), models, selectedModel, baseURL, config.SetupMessage)
	apiKey := customRuleAIEnvFirst(config.APIKeyEnvs...)
	return h.withCloudProviderHealth(ctx, provider, apiKey, func(ctx context.Context) error {
		return h.probeOpenAICompatibleModels(ctx, baseURL, apiKey)
	})
}

func customRuleAICloudProviderOption(id, label string, models []string, defaultModel, baseURL, setupMessage string) customRuleAIProviderOption {
	return customRuleAIProviderOption{
		ID:             id,
		Label:          label,
		Kind:           "cloud",
		Status:         customRuleAIProviderStatusSetup,
		Models:         models,
		ModelOptions:   customRuleAIModelOptions(models, customRuleAIProviderStatusSetup, false, setupMessage),
		DefaultModel:   defaultModel,
		Configured:     false,
		SetupRequired:  true,
		RequiresAPIKey: true,
		Local:          false,
		SetupURL:       customRuleAISetupURL,
		Message:        setupMessage,
		BaseURL:        baseURL,
	}
}

func (h *CustomRuleAIHandler) withCloudProviderHealth(ctx context.Context, provider customRuleAIProviderOption, apiKey string, probe func(context.Context) error) customRuleAIProviderOption {
	if strings.TrimSpace(apiKey) == "" {
		return provider
	}
	if err := probe(ctx); err != nil {
		provider.Configured = false
		provider.SetupRequired = true
		provider.Status = customRuleAIProviderStatusDown
		provider.Message = "Configured provider is not reachable."
		var probeErr customRuleAIProviderProbeError
		if errors.As(err, &probeErr) && probeErr.SetupRequired {
			provider.Status = customRuleAIProviderStatusSetup
			provider.Message = "Provider rejected the configured credentials."
		}
		provider.Diagnostics = []string{err.Error()}
		provider.ModelOptions = customRuleAIModelOptions(provider.Models, provider.Status, provider.Local, provider.Message)
		return provider
	}
	provider.Status = customRuleAIProviderStatusReady
	provider.Configured = true
	provider.SetupRequired = false
	provider.Message = customRuleAIReadyMessage
	provider.ModelOptions = customRuleAIModelOptions(provider.Models, provider.Status, provider.Local, provider.Message)
	return provider
}

func (h *CustomRuleAIHandler) probeOpenAICompatibleModels(ctx context.Context, baseURL, apiKey string) error {
	return h.probeAIModelsEndpoint(ctx, strings.TrimRight(baseURL, "/")+"/models", map[string]string{"Authorization": "Bearer " + apiKey})
}

func (h *CustomRuleAIHandler) probeAnthropicModels(ctx context.Context, baseURL, apiKey string) error {
	return h.probeAIModelsEndpoint(ctx, strings.TrimRight(baseURL, "/")+"/models", map[string]string{
		customRuleAIHeaderAnthropicKey:     apiKey,
		customRuleAIHeaderAnthropicVersion: customRuleAIEnvOrDefault("OLLANTA_AI_ANTHROPIC_VERSION", customRuleAIDefaultAnthropicVersion),
	})
}

func (h *CustomRuleAIHandler) probeAIModelsEndpoint(ctx context.Context, endpoint string, headers map[string]string) error {
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return customRuleAIProviderProbeError{Message: "build provider model check: " + err.Error()}
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return customRuleAIProviderProbeError{Message: "provider model check failed: " + err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return customRuleAIProviderProbeError{Message: fmt.Sprintf("provider model check returned %d; verify credentials", resp.StatusCode), SetupRequired: true}
	}
	return customRuleAIProviderProbeError{Message: fmt.Sprintf("provider model check returned %d from %s", resp.StatusCode, endpoint)}
}

func (h *CustomRuleAIHandler) ollamaProvider(ctx context.Context) customRuleAIProviderOption {
	baseURL := strings.TrimRight(customRuleAIEnvOrDefault("OLLANTA_AI_OLLAMA_BASE_URL", customRuleAIDefaultOllamaBaseURL), "/")
	provider := customRuleAIProviderOption{
		ID:             customRuleAIProviderOllama,
		Label:          customRuleAIEnvOrDefault("OLLANTA_AI_OLLAMA_LABEL", "Ollama"),
		Kind:           "local",
		Status:         customRuleAIProviderStatusDown,
		Configured:     false,
		SetupRequired:  true,
		RequiresAPIKey: false,
		Local:          true,
		SetupURL:       customRuleAISetupURL,
		Message:        "Start Ollama and pull a model to use local rule drafting.",
		BaseURL:        baseURL,
	}
	models, err := h.ollamaModels(ctx, baseURL)
	if err != nil {
		provider.Diagnostics = []string{err.Error()}
		return provider
	}
	if len(models) == 0 {
		provider.Status = customRuleAIProviderStatusSetup
		provider.Message = "Ollama is reachable, but no local models are installed. Pull a model such as deepseek-coder, qwen2.5-coder, llama3.1, or a Kimi model available to your Ollama runtime."
		return provider
	}
	defaultModel := customRuleAIEnvOrDefault("OLLANTA_AI_OLLAMA_MODEL", models[0])
	if !customRuleAIContains(models, defaultModel) {
		defaultModel = models[0]
	}
	provider.Status = customRuleAIProviderStatusReady
	provider.Models = models
	provider.ModelOptions = customRuleAIModelOptions(models, customRuleAIProviderStatusReady, true, "Local Ollama model.")
	provider.DefaultModel = defaultModel
	provider.Configured = true
	provider.SetupRequired = false
	provider.Message = "Local Ollama models are available for Rule Studio drafts."
	return provider
}

func (h *CustomRuleAIHandler) ollamaModels(ctx context.Context, baseURL string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 700*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("build Ollama model request: %w", err)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Ollama unavailable at %s", baseURL)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Ollama returned %d from %s", resp.StatusCode, baseURL)
	}
	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode Ollama models: %w", err)
	}
	models := make([]string, 0, len(payload.Models))
	for _, item := range payload.Models {
		if name := strings.TrimSpace(item.Name); name != "" && !customRuleAIContains(models, name) {
			models = append(models, name)
		}
	}
	sort.Strings(models)
	return models, nil
}

func customRuleAIModelOptions(models []string, status string, local bool, message string) []customRuleAIModelOption {
	options := make([]customRuleAIModelOption, 0, len(models))
	for _, model := range models {
		options = append(options, customRuleAIModelOption{
			ID:            model,
			Label:         model,
			Status:        status,
			Local:         local,
			SetupRequired: status != customRuleAIProviderStatusReady,
			Message:       message,
		})
	}
	return options
}

func (h *CustomRuleAIHandler) openAISuggestion(ctx context.Context, req customRuleAISuggestRequest) (customRuleAISuggestion, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return customRuleAISuggestion{}, errors.New("OPENAI_API_KEY is required for OpenAI rule drafts")
	}
	baseURL := strings.TrimRight(customRuleAIEnvOrDefault("OLLANTA_AI_OPENAI_BASE_URL", customRuleAIDefaultOpenAIBaseURL), "/")
	if customRuleAIOpenAIAPIStyle(baseURL) == customRuleAIOpenAIAPIResponses {
		return h.openAIResponsesSuggestion(ctx, req, baseURL, apiKey)
	}
	return h.openAIChatSuggestion(ctx, req, baseURL, apiKey)
}

func (h *CustomRuleAIHandler) openAIChatSuggestion(ctx context.Context, req customRuleAISuggestRequest, baseURL, apiKey string) (customRuleAISuggestion, error) {
	prompt, err := customRuleAIPrompt(req)
	if err != nil {
		return customRuleAISuggestion{}, err
	}
	body := map[string]any{
		"model": req.Model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": customRuleAISystemPrompt(),
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.2,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("marshal AI request: %w", err)
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("build AI request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set(customRuleAIHeaderContentType, customRuleAIContentTypeJSON)

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("call AI provider: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("read AI response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return customRuleAISuggestion{}, fmt.Errorf("AI provider returned %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(bodyBytes, &completion); err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("decode AI response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return customRuleAISuggestion{}, errors.New("AI provider returned no choices")
	}

	jsonPayload, err := customRuleAIExtractJSONObject(completion.Choices[0].Message.Content)
	if err != nil {
		return customRuleAISuggestion{}, err
	}
	var suggestion customRuleAISuggestion
	if err := json.Unmarshal([]byte(jsonPayload), &suggestion); err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("decode AI payload: %w", err)
	}
	return suggestion, nil
}

func (h *CustomRuleAIHandler) openAIResponsesSuggestion(ctx context.Context, req customRuleAISuggestRequest, baseURL, apiKey string) (customRuleAISuggestion, error) {
	prompt, err := customRuleAIPrompt(req)
	if err != nil {
		return customRuleAISuggestion{}, err
	}
	body := map[string]any{
		"model": req.Model,
		"input": []map[string]string{
			{
				"role":    "system",
				"content": customRuleAISystemPrompt(),
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"text": map[string]any{"format": map[string]string{"type": "json_object"}},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("marshal AI request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/responses", bytes.NewReader(payload))
	if err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("build AI request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set(customRuleAIHeaderContentType, customRuleAIContentTypeJSON)

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("call AI provider: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("read AI response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return customRuleAISuggestion{}, fmt.Errorf("AI provider returned %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	content, err := customRuleAIResponsesOutputText(bodyBytes)
	if err != nil {
		return customRuleAISuggestion{}, err
	}
	jsonPayload, err := customRuleAIExtractJSONObject(content)
	if err != nil {
		return customRuleAISuggestion{}, err
	}
	var suggestion customRuleAISuggestion
	if err := json.Unmarshal([]byte(jsonPayload), &suggestion); err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("decode AI payload: %w", err)
	}
	return suggestion, nil
}

func (h *CustomRuleAIHandler) anthropicSuggestion(ctx context.Context, req customRuleAISuggestRequest) (customRuleAISuggestion, error) {
	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if apiKey == "" {
		return customRuleAISuggestion{}, errors.New("ANTHROPIC_API_KEY is required for Anthropic rule drafts")
	}
	prompt, err := customRuleAIPrompt(req)
	if err != nil {
		return customRuleAISuggestion{}, err
	}
	body := map[string]any{
		"model":       req.Model,
		"max_tokens":  4096,
		"system":      customRuleAISystemPrompt(),
		"temperature": 0.2,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("marshal Anthropic request: %w", err)
	}

	baseURL := strings.TrimRight(customRuleAIEnvOrDefault("OLLANTA_AI_ANTHROPIC_BASE_URL", customRuleAIDefaultAnthropicBaseURL), "/")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/messages", bytes.NewReader(payload))
	if err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("build Anthropic request: %w", err)
	}
	httpReq.Header.Set(customRuleAIHeaderAnthropicKey, apiKey)
	httpReq.Header.Set(customRuleAIHeaderAnthropicVersion, customRuleAIEnvOrDefault("OLLANTA_AI_ANTHROPIC_VERSION", customRuleAIDefaultAnthropicVersion))
	httpReq.Header.Set(customRuleAIHeaderContentType, customRuleAIContentTypeJSON)

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("call Anthropic provider: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("read Anthropic response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return customRuleAISuggestion{}, fmt.Errorf("Anthropic returned %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	content, err := customRuleAIAnthropicOutputText(bodyBytes)
	if err != nil {
		return customRuleAISuggestion{}, err
	}
	jsonPayload, err := customRuleAIExtractJSONObject(content)
	if err != nil {
		return customRuleAISuggestion{}, err
	}
	var suggestion customRuleAISuggestion
	if err := json.Unmarshal([]byte(jsonPayload), &suggestion); err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("decode Anthropic payload: %w", err)
	}
	return suggestion, nil
}

func (h *CustomRuleAIHandler) kimiSuggestion(ctx context.Context, req customRuleAISuggestRequest) (customRuleAISuggestion, error) {
	apiKey := customRuleAIEnvFirst("MOONSHOT_API_KEY", "KIMI_API_KEY")
	if apiKey == "" {
		return customRuleAISuggestion{}, errors.New("MOONSHOT_API_KEY or KIMI_API_KEY is required for Kimi rule drafts")
	}
	baseURL := strings.TrimRight(customRuleAIEnvOrDefault("OLLANTA_AI_KIMI_BASE_URL", customRuleAIDefaultKimiBaseURL), "/")
	return h.openAIChatSuggestion(ctx, req, baseURL, apiKey)
}

func (h *CustomRuleAIHandler) qwenSuggestion(ctx context.Context, req customRuleAISuggestRequest) (customRuleAISuggestion, error) {
	apiKey := customRuleAIEnvFirst("DASHSCOPE_API_KEY", "QWEN_API_KEY")
	if apiKey == "" {
		return customRuleAISuggestion{}, errors.New("DASHSCOPE_API_KEY or QWEN_API_KEY is required for Qwen rule drafts")
	}
	baseURL := strings.TrimRight(customRuleAIEnvOrDefault("OLLANTA_AI_QWEN_BASE_URL", customRuleAIDefaultQwenBaseURL), "/")
	return h.openAIChatSuggestion(ctx, req, baseURL, apiKey)
}

func (h *CustomRuleAIHandler) ollamaSuggestion(ctx context.Context, req customRuleAISuggestRequest) (customRuleAISuggestion, error) {
	prompt, err := customRuleAIPrompt(req)
	if err != nil {
		return customRuleAISuggestion{}, err
	}
	body := map[string]any{
		"model":  req.Model,
		"stream": false,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": customRuleAISystemPrompt(),
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"options": map[string]any{"temperature": 0.2},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("marshal Ollama request: %w", err)
	}

	baseURL := strings.TrimRight(customRuleAIEnvOrDefault("OLLANTA_AI_OLLAMA_BASE_URL", customRuleAIDefaultOllamaBaseURL), "/")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("build Ollama request: %w", err)
	}
	httpReq.Header.Set(customRuleAIHeaderContentType, customRuleAIContentTypeJSON)

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("call Ollama provider: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("read Ollama response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return customRuleAISuggestion{}, fmt.Errorf("Ollama returned %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var completion struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(bodyBytes, &completion); err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("decode Ollama response: %w", err)
	}
	jsonPayload, err := customRuleAIExtractJSONObject(completion.Message.Content)
	if err != nil {
		return customRuleAISuggestion{}, err
	}
	var suggestion customRuleAISuggestion
	if err := json.Unmarshal([]byte(jsonPayload), &suggestion); err != nil {
		return customRuleAISuggestion{}, fmt.Errorf("decode Ollama payload: %w", err)
	}
	return suggestion, nil
}

func customRuleAISystemPrompt() string {
	return "You draft custom static-analysis rules for Ollanta. Return strict JSON only, with keys rule_id, name, language, type, severity, engine, text_pattern, go_ast_pattern, target, tree_sitter_query, noncompliant_example, compliant_example, message. Do not include markdown fences."
}

func customRuleAIOpenAIAPIStyle(baseURL string) string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("OLLANTA_AI_OPENAI_API"))) {
	case "responses", "response":
		return customRuleAIOpenAIAPIResponses
	case "chat", "chat_completions", "chat-completions":
		return customRuleAIOpenAIAPIChat
	}
	if strings.TrimRight(baseURL, "/") == customRuleAIDefaultOpenAIBaseURL {
		return customRuleAIOpenAIAPIResponses
	}
	return customRuleAIOpenAIAPIChat
}

func customRuleAIResponsesOutputText(body []byte) (string, error) {
	var payload struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("decode AI response: %w", err)
	}
	if strings.TrimSpace(payload.OutputText) != "" {
		return payload.OutputText, nil
	}
	for _, output := range payload.Output {
		for _, content := range output.Content {
			if strings.TrimSpace(content.Text) != "" {
				return content.Text, nil
			}
		}
	}
	return "", errors.New("AI provider returned no output text")
}

func customRuleAIAnthropicOutputText(body []byte) (string, error) {
	var payload struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("decode Anthropic response: %w", err)
	}
	for _, content := range payload.Content {
		if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
			return content.Text, nil
		}
	}
	return "", errors.New("Anthropic returned no text content")
}

func customRuleAIPrompt(req customRuleAISuggestRequest) (string, error) {
	current, err := json.MarshalIndent(req.Current, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal current draft: %w", err)
	}
	return strings.Join([]string{
		"Create or improve an Ollanta custom rule draft from this user intent.",
		"Respect the selected engine when it is already present in current.engine.",
		"Allowed engines: text, go-ast, tree-sitter.",
		"For text, fill text_pattern as a regexp and leave tree_sitter_query empty.",
		"For go-ast, use language go, go_ast_pattern forbidden_call or forbidden_import, and fill target.",
		"For tree-sitter, fill tree_sitter_query and leave text_pattern empty.",
		"Always provide one noncompliant example that should match and one compliant example that should not match.",
		"Intent:",
		req.Intent,
		"Current draft JSON:",
		string(current),
	}, "\n"), nil
}

func customRuleAIMockSuggestion(req customRuleAISuggestRequest) customRuleAISuggestion {
	current := req.Current
	engine := customRuleAIFirst(current.Engine, customRuleAIEngineText)
	name := customRuleAIFirst(current.Name, customRuleAITitle(req.Intent))
	ruleID := customRuleAIFirst(current.RuleID, customRuleAISlug(name))
	language := customRuleAIFirst(current.Language, "go")
	textPattern := customRuleAIFirst(current.TextPattern, "panic|debugger|TODO")
	if strings.Contains(strings.ToLower(req.Intent), "console") {
		textPattern = customRuleAIFirst(current.TextPattern, "console.log")
		language = customRuleAIFirst(current.Language, "javascript")
	}
	suggestion := customRuleAISuggestion{
		PackName:            customRuleAIFirst(current.PackName, "Rule Studio"),
		RuleID:              ruleID,
		Name:                name,
		Language:            language,
		Type:                customRuleAIFirst(current.Type, "code_smell"),
		Severity:            customRuleAIFirst(current.Severity, "major"),
		Engine:              engine,
		TextPattern:         textPattern,
		GoASTPattern:        customRuleAIFirst(current.GoASTPattern, "forbidden_call"),
		Target:              customRuleAIFirst(current.Target, "fmt.Println"),
		TreeSitterQuery:     current.TreeSitterQuery,
		NoncompliantExample: customRuleAIFirst(current.NoncompliantExample, customRuleAIExample(language, engine, true, textPattern)),
		CompliantExample:    customRuleAIFirst(current.CompliantExample, customRuleAIExample(language, engine, false, textPattern)),
		Message:             customRuleAIFirst(current.Message, name),
	}
	return suggestion
}

func normalizeCustomRuleAISuggestion(s customRuleAISuggestion, req customRuleAISuggestRequest) customRuleAISuggestion {
	s.PackName = strings.TrimSpace(customRuleAIFirst(s.PackName, req.Current.PackName, "Rule Studio"))
	s.Name = strings.TrimSpace(customRuleAIFirst(s.Name, customRuleAITitle(req.Intent)))
	s.RuleID = strings.TrimSpace(customRuleAIFirst(s.RuleID, req.Current.RuleID, customRuleAISlug(s.Name)))
	s.Language = customRuleAIAllowed(customRuleAIFirst(s.Language, req.Current.Language, "go"), []string{"go", "javascript", "typescript", "python", "rust"}, "go")
	s.Type = customRuleAIAllowed(customRuleAIFirst(s.Type, req.Current.Type, "code_smell"), []string{"code_smell", "bug", "vulnerability", "security_hotspot"}, "code_smell")
	s.Severity = customRuleAIAllowed(customRuleAIFirst(s.Severity, req.Current.Severity, "major"), []string{"blocker", "critical", "major", "minor", "info"}, "major")
	s.Engine = customRuleAIAllowed(customRuleAIFirst(s.Engine, req.Current.Engine, customRuleAIEngineText), []string{customRuleAIEngineText, customRuleAIEngineGoAST, customRuleAIEngineTreeSitter}, customRuleAIEngineText)
	s.TextPattern = strings.TrimSpace(customRuleAIFirst(s.TextPattern, req.Current.TextPattern))
	s.GoASTPattern = customRuleAIAllowed(customRuleAIFirst(s.GoASTPattern, req.Current.GoASTPattern, "forbidden_call"), []string{"forbidden_call", "forbidden_import"}, "forbidden_call")
	s.Target = strings.TrimSpace(customRuleAIFirst(s.Target, req.Current.Target))
	s.TreeSitterQuery = strings.TrimSpace(customRuleAIFirst(s.TreeSitterQuery, req.Current.TreeSitterQuery))
	s.NoncompliantExample = strings.TrimSpace(customRuleAIFirst(s.NoncompliantExample, req.Current.NoncompliantExample))
	s.CompliantExample = strings.TrimSpace(customRuleAIFirst(s.CompliantExample, req.Current.CompliantExample))
	s.Message = strings.TrimSpace(customRuleAIFirst(s.Message, req.Current.Message, s.Name))

	if s.Engine == customRuleAIEngineGoAST {
		s.Language = "go"
		if s.Target == "" {
			s.Target = "fmt.Println"
		}
		s.TextPattern = ""
		s.TreeSitterQuery = ""
	}
	if s.Engine == customRuleAIEngineText {
		if s.TextPattern == "" {
			s.TextPattern = "panic|debugger|TODO"
		}
		s.TreeSitterQuery = ""
	}
	if s.Engine == customRuleAIEngineTreeSitter {
		if s.TreeSitterQuery == "" {
			s.TreeSitterQuery = "(identifier) @name"
		}
		s.TextPattern = ""
	}
	if s.NoncompliantExample == "" {
		s.NoncompliantExample = customRuleAIExample(s.Language, s.Engine, true, s.TextPattern)
	}
	if s.CompliantExample == "" {
		s.CompliantExample = customRuleAIExample(s.Language, s.Engine, false, s.TextPattern)
	}
	return s
}

func customRuleAIModelListEnv(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return append([]string(nil), fallback...)
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' || r == '\n' || r == '\r' })
	models := make([]string, 0, len(parts))
	for _, part := range parts {
		model := strings.TrimSpace(part)
		if model != "" && !customRuleAIContains(models, model) {
			models = append(models, model)
		}
	}
	if len(models) == 0 {
		return append([]string(nil), fallback...)
	}
	return models
}

func customRuleAIEnvOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func customRuleAIEnvFirst(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func customRuleAIContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func customRuleAIExtractJSONObject(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", errors.New("AI provider returned an empty response")
	}
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return "", errors.New("AI provider did not return a JSON object")
	}
	return content[start : end+1], nil
}

func customRuleAIFirst(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func customRuleAIAllowed(value string, allowed []string, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, item := range allowed {
		if value == item {
			return value
		}
	}
	return fallback
}

func customRuleAITitle(intent string) string {
	words := strings.Fields(strings.TrimSpace(intent))
	if len(words) > 6 {
		words = words[:6]
	}
	if len(words) == 0 {
		return "Custom rule"
	}
	for i, word := range words {
		words[i] = strings.Trim(word, " .,;:!?()[]{}\"'")
	}
	return "Detect " + strings.Join(words, " ")
}

func customRuleAISlug(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func customRuleAIExample(language, engine string, noncompliant bool, pattern string) string {
	if engine == customRuleAIEngineGoAST {
		if noncompliant {
			return "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"debug\")\n}"
		}
		return "package main\n\nfunc main() {\n\tprintln(\"ok\")\n}"
	}
	if language == "javascript" || language == "typescript" {
		if noncompliant {
			if strings.Contains(pattern, "console") {
				return "function save() {\n  console.log('debug');\n}"
			}
			return "function save() {\n  debugger;\n}"
		}
		return "function save() {\n  return true;\n}"
	}
	if noncompliant {
		return "package main\n\nfunc main() {\n\tpanic(\"debug\")\n}"
	}
	return "package main\n\nfunc main() {\n\tprintln(\"ok\")\n}"
}
