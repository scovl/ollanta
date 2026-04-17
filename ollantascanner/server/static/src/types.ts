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
  end_line: number;
  severity: Severity;
  type: IssueType;
  message: string;
  status: string;
  hash: string;
}

export type Severity = "blocker" | "critical" | "major" | "minor" | "info";
export type IssueType = "BUG" | "CODE_SMELL" | "VULNERABILITY";
