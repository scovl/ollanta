export interface Report {
  metadata: Metadata;
  measures: Measures;
  issues: Issue[];
}

export interface Metadata {
  project_key: string;
  analysis_date: string;
  version: string;
  elapsed_ms: number;
}

export interface Measures {
  files: number;
  lines: number;
  ncloc: number;
  comments: number;
  bugs: number;
  code_smells: number;
  vulnerabilities: number;
  by_language: Record<string, number>;
}

export interface Issue {
  rule_key: string;
  component_path: string;
  line: number;
  column: number;
  end_line: number;
  end_column: number;
  message: string;
  type: IssueType;
  severity: Severity;
  status: string;
  engine_id: string;
  line_hash: string;
  tags: string[];
  secondary_locations: SecondaryLocation[];
}

export interface SecondaryLocation {
  file_path: string;
  message: string;
  start_line: number;
  start_column: number;
  end_line: number;
  end_column: number;
}

export type Severity = "blocker" | "critical" | "major" | "minor" | "info";
export type IssueType = "bug" | "code_smell" | "vulnerability" | "security_hotspot";

/** Computed quality gate result */
export interface GateResult {
  status: "passed" | "failed";
  conditions: GateCondition[];
}

export interface GateCondition {
  metric: string;
  operator: string;
  threshold: number;
  value: number;
  passed: boolean;
}

/** Issues grouped by file */
export interface FileGroup {
  path: string;
  shortPath: string;
  issues: Issue[];
  expanded: boolean;
}

export interface AIAgent {
  id: string;
  label: string;
  provider: string;
  model: string;
}

export interface AIAgentListResponse {
  agents: AIAgent[];
}

export interface AIProviderOption {
  id: string;
  label: string;
  models: string[];
  default_model: string;
  configured: boolean;
  requires_api_key: boolean;
}

export interface AIProviderListResponse {
  providers: AIProviderOption[];
}

export interface AIFixPreview {
  preview_id: string;
  agent: AIAgent;
  status: string;
  summary: string;
  explanation: string;
  diff: string;
  file_path: string;
  start_line: number;
  end_line: number;
  original_snippet: string;
  replacement: string;
}

export interface AIFixApplyResponse {
  preview_id: string;
  status: string;
  file_path: string;
  message: string;
}
