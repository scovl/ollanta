import { describe, expect, it } from "vitest";

import { renderAIFixContent, renderDetailTabs, type AIFixViewState } from "./detailView";
import type { AIFixPreview, AIProviderOption, Issue } from "./types";

function buildIssue(): Issue {
  return {
    rule_key: "go:nil-check",
    component_path: "/tmp/main.go",
    line: 12,
    column: 1,
    end_line: 12,
    end_column: 18,
    message: "Use the AI fix flow",
    type: "code_smell",
    severity: "major",
    status: "open",
    engine_id: "ollanta",
    line_hash: "hash",
    tags: [],
    secondary_locations: [],
  };
}

function buildState(preview: AIFixPreview | null = null): AIFixViewState {
  return {
    loadingOptions: false,
    loadingPreview: false,
    applying: false,
    selectedProviderId: "openai",
    selectedModel: "gpt-5.5",
    apiKey: "",
    statusMessage: "",
    errorMessage: "",
    preview,
  };
}

describe("detailView", () => {
  it("renders tabs with Fix with AI visible", () => {
    const html = renderDetailTabs("ai-fix");

    expect(html).toContain("Fix with AI");
    expect(html).toContain('data-detail-tab="ai-fix"');
    expect(html).toContain("detail-tab active");
  });

  it("renders empty-state when no providers are available", () => {
    const html = renderAIFixContent(buildIssue(), buildState(), []);

    expect(html).toContain("No AI provider is available for the local scanner.");
    expect(html).toContain("Generate a fix preview");
  });

  it("renders provider/model controls, preview and apply action", () => {
    const preview: AIFixPreview = {
      preview_id: "preview-1",
      agent: { id: "openai:gpt-5.5", label: "OpenAI", provider: "openai", model: "gpt-5.5" },
      status: "ready",
      summary: "Generated fix preview",
      explanation: "Preview explanation",
      diff: "@@ lines 12-12 @@\n- old\n+ new",
      file_path: "/tmp/main.go",
      start_line: 12,
      end_line: 12,
      original_snippet: "old",
      replacement: "new",
    };
    const providers: AIProviderOption[] = [{
      id: "openai",
      label: "OpenAI",
      models: ["gpt-5.5", "gpt-5.4-mini"],
      default_model: "gpt-5.5",
      configured: false,
      requires_api_key: true,
    }];

    const html = renderAIFixContent(buildIssue(), buildState(preview), providers);

    expect(html).toContain('id="ai-provider-select"');
    expect(html).toContain('id="ai-model-input"');
    expect(html).toContain('id="ai-api-key-input"');
    expect(html).toContain("Generate fix");
    expect(html).toContain("Preview explanation");
    expect(html).toContain("Apply to file");
    expect(html).toContain("gpt-5.5");
    expect(html).toContain("@@ lines 12-12 @@");
  });
});
