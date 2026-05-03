export interface Report {
  metadata: Metadata;
  measures: Measures;
  issues: Issue[];
  test_signals?: TestSignalReport;
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
  coverage?: number;
  tests?: number;
  test_failures?: number;
  test_errors?: number;
  test_skipped?: number;
  test_duration_ms?: number;
  mutation_score?: number;
  mutants_total?: number;
  mutants_killed?: number;
  mutants_survived?: number;
  mutants_timeout?: number;
  mutants_skipped?: number;
  mutants_error?: number;
  changed_mutation_score?: number;
  changed_mutants_total?: number;
  changed_mutants_killed?: number;
  changed_mutants_survived?: number;
  by_language: Record<string, number>;
}

export interface TestSignalReport {
  summary: TestSignalSummary;
  modules: TestModuleSignal[];
}

export interface TestSignalSummary {
  modules?: number;
  modules_with_coverage?: number;
  lines_to_cover?: number;
  covered_lines?: number;
  coverage?: number;
  mutants_total?: number;
  mutants_killed?: number;
  mutants_survived?: number;
  mutants_timeout?: number;
  mutants_skipped?: number;
  mutants_error?: number;
  mutation_score?: number;
  changed_mutants_total?: number;
  changed_mutants_killed?: number;
  changed_mutants_survived?: number;
  changed_mutation_score?: number;
}

export interface TestModuleSignal {
  name: string;
  root: string;
  language?: string;
  architecture_role?: string;
  mutation?: TestMutationSummary;
  coverage?: TestCoverageSummary;
  files?: TestFileCoverage[];
}

export interface TestMutationSummary {
  tool?: string;
  status?: string;
  confidence?: string;
  score?: number;
  changed_code_score?: number;
  total?: number;
  killed?: number;
  survived?: number;
  timeout?: number;
  skipped?: number;
  errors?: number;
  changed_total?: number;
  changed_killed?: number;
  changed_survived?: number;
  partial?: boolean;
  stale?: boolean;
  survived_mutants?: TestMutantSignal[];
}

export interface TestMutantSignal {
  id?: string;
  status?: string;
  mutator?: string;
  file?: string;
  line?: number;
  replacement?: string;
  description?: string;
  changed_code?: boolean;
  confidence?: string;
}

export interface TestCoverageSummary {
  lines_to_cover?: number;
  covered_lines?: number;
  uncovered_lines?: number;
  coverage?: number;
}

export interface TestFileCoverage {
  path: string;
  lines_to_cover?: number;
  covered_lines?: number;
  covered_line_numbers?: number[];
  uncovered_lines?: number[];
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
