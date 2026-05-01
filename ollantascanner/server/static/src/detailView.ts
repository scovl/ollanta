import { esc } from "./html";
import type { AIProviderOption, AIFixPreview, Issue } from "./types";

export type DetailTabKey = "details" | "rule" | "ai-fix";

export interface AIFixViewState {
  loadingOptions: boolean;
  loadingPreview: boolean;
  applying: boolean;
  selectedProviderId: string;
  selectedModel: string;
  apiKey: string;
  statusMessage: string;
  errorMessage: string;
  preview: AIFixPreview | null;
}

export function renderDetailTabs(activeTab: DetailTabKey): string {
  const tabs = [
    { key: "details", label: "Details" },
    { key: "rule", label: "Rule" },
    { key: "ai-fix", label: "Fix with AI" },
  ];

  return tabs
    .map(tab => `<button class="detail-tab${activeTab === tab.key ? " active" : ""}" data-detail-tab="${tab.key}">${tab.label}</button>`)
    .join("");
}

export function renderAIFixContent(issue: Issue, state: AIFixViewState, providers: AIProviderOption[]): string {
  const locationSuffix = issue.end_line && issue.end_line !== issue.line ? `-${issue.end_line}` : "";
  const modelSection = renderAIFixModelSection(state, providers);
  const previewSection = renderAIFixPreviewSection(state);

  return `
    <div class="detail-section">
      <div class="detail-section-title">Fix with AI</div>
      <div class="detail-msg ai-fix-callout">Ollanta prepares the issue context, sends only the relevant snippet to the selected agent, and shows a preview before writing any changes to your code.</div>
    </div>

    <div class="detail-section">
      <div class="detail-field detail-field-stack">
        <span class="detail-field-label">Target</span>
        <span class="detail-field-value detail-mono-block">${esc(issue.component_path)}:${issue.line}${locationSuffix}</span>
      </div>
      <div class="detail-field detail-field-stack">
        <span class="detail-field-label">Issue</span>
        <span class="detail-field-value">${esc(issue.message)}</span>
      </div>
    </div>

    <div class="detail-section">
      <div class="detail-section-title">Model</div>
      ${modelSection}
      ${state.statusMessage ? `<div class="ai-fix-status ai-fix-status-ok">${esc(state.statusMessage)}</div>` : ""}
      ${state.errorMessage ? `<div class="ai-fix-status ai-fix-status-error">${esc(state.errorMessage)}</div>` : ""}
    </div>

    <div class="detail-section">
      <div class="detail-section-title">Preview</div>
      ${previewSection}
    </div>
  `;
}

function renderAIFixModelSection(state: AIFixViewState, providers: AIProviderOption[]): string {
  if (state.loadingOptions) {
    return `<div class="detail-loading">Loading AI models…</div>`;
  }
  if (providers.length === 0) {
    return `<div class="detail-empty">No AI provider is available for the local scanner.</div>`;
  }

  const selectedProvider = providers.find(provider => provider.id === state.selectedProviderId) ?? providers[0];
  const providerOptions = providers
    .map(provider => `<option value="${esc(provider.id)}"${state.selectedProviderId === provider.id ? " selected" : ""}>${esc(provider.label)}</option>`)
    .join("");
  const suggestedModels = selectedProvider?.models ?? [];
  const modelOptions = suggestedModels
    .map(model => `<option value="${esc(model)}"></option>`)
    .join("");
  let apiKeyHint = `<div class="ai-fix-helper">This provider can generate local previews without an API key.</div>`;
  let apiKeyPlaceholder = "Required for this provider";
  if (selectedProvider?.requires_api_key) {
    if (selectedProvider.configured) {
      apiKeyHint = `<div class="ai-fix-helper">Using the scanner's configured API key. Paste another key below to override it for this session.</div>`;
      apiKeyPlaceholder = "Optional override";
    } else {
      apiKeyHint = `<div class="ai-fix-helper">Paste an API key for the selected provider to generate the fix.</div>`;
    }
  }
  const apiKeyInput = selectedProvider?.requires_api_key
    ? `<div class="ai-fix-control-group">
          <label class="ai-fix-control-label" for="ai-api-key-input">API key</label>
          <input id="ai-api-key-input" class="ai-fix-select ai-fix-input" type="password" value="${esc(state.apiKey)}" placeholder="${apiKeyPlaceholder}" autocomplete="off">
        </div>`
    : "";
  const generateLabel = state.loadingPreview ? "Generating…" : "Generate fix";
  const generateDisabled = state.loadingPreview ? " disabled" : "";

  return `<div class="ai-fix-controls">
      <div class="ai-fix-control-group">
        <label class="ai-fix-control-label" for="ai-provider-select">Provider</label>
        <select id="ai-provider-select" class="ai-fix-select">${providerOptions}</select>
      </div>
      <div class="ai-fix-control-group">
        <label class="ai-fix-control-label" for="ai-model-input">Model</label>
        <input id="ai-model-input" class="ai-fix-select ai-fix-input" list="ai-model-options" value="${esc(state.selectedModel)}" placeholder="${esc(selectedProvider?.default_model || "gpt-4.1-mini")}" autocomplete="off">
        <datalist id="ai-model-options">${modelOptions}</datalist>
      </div>
      ${apiKeyInput}
      ${apiKeyHint}
      <button id="ai-generate-fix" class="ai-fix-button"${generateDisabled}>${generateLabel}</button>
    </div>`;
}

function renderAIFixPreviewSection(state: AIFixViewState): string {
  if (!state.preview) {
    return `<div class="detail-empty">Generate a fix preview to inspect the patch before Ollanta edits your local file.</div>`;
  }

  const previewSummary = state.preview.summary || "Generated fix preview";
  const explanation = state.preview.explanation
    ? `<div class="rule-rationale">${esc(state.preview.explanation)}</div>`
    : "";
  const applyLabel = state.applying ? "Applying…" : "Apply to file";
  const applyDisabled = state.applying ? " disabled" : "";

  return `
    <div class="ai-fix-preview-meta">
      <div><strong>Provider:</strong> ${esc(state.preview.agent.label)}</div>
      <div><strong>Model:</strong> ${esc(state.preview.agent.model)}</div>
      <div><strong>Summary:</strong> ${esc(previewSummary)}</div>
    </div>
    ${explanation}
    <pre class="rule-code ai-fix-diff"><code>${esc(state.preview.diff)}</code></pre>
    <div class="ai-fix-actions">
      <button id="ai-apply-fix" class="ai-fix-button ai-fix-button-primary"${applyDisabled}>${applyLabel}</button>
    </div>
  `;
}

