import { esc } from "./html";
import type { Report, Issue, Severity, GateResult, GateCondition, FileGroup, AIProviderListResponse, AIProviderOption, AIFixPreview, AIFixApplyResponse, TestFileCoverage, TestModuleSignal, TestMutationSummary, TestMutantSignal } from "./types";
import { renderAIFixContent, renderDetailTabs } from "./detailView";

// ══════════════════════════════════════════════════════════════════════════
// State
// ══════════════════════════════════════════════════════════════════════════

let report: Report;
let allIssues: Issue[] = [];
let filteredIssues: Issue[] = [];
let fileGroups: FileGroup[] = [];
let coverageFiles: CoverageFileView[] = [];
let selectedCoveragePath = "";
let coverageSourceLoading = false;
let coverageSourceError = "";
let coverageSourceCache = new Map<string, CoverageSourceFile>();
let selectedIssue: Issue | null = null;
let selectedIndex = -1;
let activeTab = "overview";
let lastFocusedElement: HTMLElement | null = null;
let detailTab: "details" | "rule" | "ai-fix" = "details";
let detailRuleHtml = "";
let detailRuleLoading = false;
let cachedAIProviders: AIProviderOption[] | null = null;

type AIFixPanelState = {
  loadingOptions: boolean;
  loadingPreview: boolean;
  applying: boolean;
  selectedProviderId: string;
  selectedModel: string;
  apiKey: string;
  statusMessage: string;
  errorMessage: string;
  preview: AIFixPreview | null;
};

type CoverageFileView = {
  moduleName: string;
  moduleRoot: string;
  path: string;
  linesToCover: number;
  coveredLines: number;
  coveredLineNumbers: number[];
  uncoveredLines: number[];
  coverage: number | null;
};

type CoverageSourceFile = {
  path: string;
  language: string;
  content: string;
  line_count: number;
  size_bytes: number;
};

let aiFixState: AIFixPanelState = createEmptyAIFixState();

let filterSeverity = "all";
let filterType = "all";
let filterRule = "all";
let filterQuality = "all";
let filterTag = "all";
let searchText = "";
let issuesViewMode: "flat" | "grouped" = "flat";

let mutantFilterModule = "all";
let mutantSortField: "file" | "line" | "module" = "file";
let mutantSortDir: "asc" | "desc" = "asc";

const SEV_ORDER: Record<Severity, number> = {
  blocker: 0, critical: 1, major: 2, minor: 3, info: 4,
};

const SEV_COLORS: Record<string, string> = {
  blocker: "#ef4444", critical: "#f97316", major: "#eab308",
  minor: "#22c55e", info: "#64748b",
};

const TYPE_LABELS: Record<string, string> = {
  bug: "Bug", code_smell: "Code Smell", vulnerability: "Vulnerability",
  security_hotspot: "Hotspot",
};

const QUALITY_LABELS: Record<string, string> = {
  security: "Security", reliability: "Reliability",
  maintainability: "Maintainability", testability: "Testability",
};

function iconHtml(name: string, label: string): string {
  return `<span class="icon-${esc(name)}" role="img" aria-label="${esc(label)}"></span>`;
}

// ══════════════════════════════════════════════════════════════════════════
// Bootstrap
// ══════════════════════════════════════════════════════════════════════════

async function init(): Promise<void> {
  try {
    const res = await fetch("/report.json");
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    report = await res.json();
    allIssues = report.issues ?? [];

    renderHeader();
    renderGate();
    renderMeasures();
    renderOverviewCharts();
    renderPriorityIssues();
    renderHotspotFiles();
    buildCoverageFiles();
    renderCoverageSummary();
    renderMutationSummary();
    renderTestSignalModules();
    renderCoverageDetails();
    renderMutantsTab();
    if (coverageFiles.length) {
      void selectCoverageFile(selectedCoveragePath || coverageFiles[0].path);
    }
    renderLanguages();
    populateFilters();
    applyFilters();
    buildFileGroups();
    renderGroupedIssues();
    setupTabs();
    setupKeyboard();
    setupMutantFilters();

    el("tab-issue-count").textContent = String(allIssues.length);
    el("tab-coverage-count").textContent = formatPercent(report.measures.coverage ?? report.test_signals?.summary?.coverage);
    el("tab-mutant-count").textContent = String(mutationOverview().survived);
  } catch (e) {
    el("app").innerHTML =
      `<div class="error">Failed to load report: ${String(e)}</div>`;
  }
}

document.addEventListener("DOMContentLoaded", init);

// ══════════════════════════════════════════════════════════════════════════
// Header
// ══════════════════════════════════════════════════════════════════════════

function renderHeader(): void {
  const m = report.metadata;
  const date = new Date(m.analysis_date).toLocaleString();
  el("project-key").textContent = m.project_key;
  el("scan-date").textContent = date;
  el("scan-version").textContent = `v${m.version}`;
  el("elapsed").textContent = `${m.elapsed_ms}ms`;
}

// ══════════════════════════════════════════════════════════════════════════
// Quality Gate (computed locally — SonarQube's #1 UX pattern)
// ══════════════════════════════════════════════════════════════════════════

function computeGate(): GateResult {
  const m = report.measures;
  const ts = report.test_signals?.summary;
  const conditions: GateCondition[] = [
    { metric: "Bugs", operator: "=", threshold: 0, value: m.bugs, passed: m.bugs === 0 },
    { metric: "Vulnerabilities", operator: "=", threshold: 0, value: m.vulnerabilities, passed: m.vulnerabilities === 0 },
    { metric: "Code Smells", operator: "\u2264", threshold: 10, value: m.code_smells, passed: m.code_smells <= 10, severity: m.code_smells <= 10 ? undefined : (m.code_smells <= 20 ? "warning" : undefined) },
  ];

  if (m.coverage != null) {
    conditions.push({ metric: "Coverage", operator: "\u2265", threshold: 70, value: m.coverage, passed: m.coverage >= 70, severity: m.coverage >= 70 ? undefined : (m.coverage >= 60 ? "warning" : undefined) });
  } else {
    conditions.push({ metric: "Coverage", operator: "\u2265", threshold: 70, value: 0, passed: false, severity: "missing" });
  }

  if (ts) {
    if (ts.tests != null) {
      conditions.push({ metric: "Test Failures", operator: "=", threshold: 0, value: ts.test_failures ?? 0, passed: (ts.test_failures ?? 0) === 0 });
    }
    if (ts.mutation_score != null) {
      conditions.push({ metric: "Mutation Score", operator: "\u2265", threshold: 60, value: ts.mutation_score, passed: ts.mutation_score >= 60, severity: ts.mutation_score >= 60 ? undefined : (ts.mutation_score >= 40 ? "warning" : undefined) });
    }
    if (ts.changed_mutation_score != null) {
      conditions.push({ metric: "Changed Mutation", operator: "\u2265", threshold: 60, value: ts.changed_mutation_score, passed: ts.changed_mutation_score >= 60, severity: ts.changed_mutation_score >= 60 ? undefined : (ts.changed_mutation_score >= 40 ? "warning" : undefined) });
    }
  }

  const failed = conditions.filter(c => !c.passed && c.severity !== "warning" && c.severity !== "missing");
  const status = failed.length === 0 ? "passed" : "failed";
  return { status, conditions };
}

function renderGate(): void {
  const gate = computeGate();
  const hero = el("gate-hero");
  hero.classList.remove("gate-loading");
  hero.classList.add(gate.status === "passed" ? "gate-passed" : "gate-failed");

  const gateIcon = el("gate-icon");
  gateIcon.className = `gate-icon icon-${gate.status === "passed" ? "pass" : "fail"}`;
  gateIcon.setAttribute("aria-label", gate.status === "passed" ? "Passed" : "Failed");
  el("gate-status").textContent = gate.status === "passed" ? "Passed" : "Failed";
  if (gate.status === "passed") {
    const failedConditions = gate.conditions.filter(c => !c.passed && c.severity !== "warning");
    const warnings = gate.conditions.filter(c => !c.passed && c.severity === "warning");
    if (warnings.length && !failedConditions.length) {
      el("gate-status").textContent = "Passed with warnings";
      hero.classList.add("gate-warn");
    }
  }

  const condHtml = gate.conditions.map(c => {
    const cls = c.passed ? "cond-pass" : (c.severity === "warning" ? "cond-warn" : "cond-fail");
    const icon = c.passed ? iconHtml("pass", "Passed") : iconHtml("fail", "Failed");
    return `<div class="gate-cond ${cls}">
      <span class="gate-cond-icon">${icon}</span>
      <span class="gate-cond-metric">${esc(c.metric)} ${esc(c.operator)} ${c.threshold}</span>
      <span class="gate-cond-value">${c.value}</span>
    </div>`;
  }).join("");
  el("gate-conditions").innerHTML = condHtml;
}

// ══════════════════════════════════════════════════════════════════════════
// Measures (color-coded cards — green/yellow/red based on count)
// ══════════════════════════════════════════════════════════════════════════

function renderMeasures(): void {
  const m = report.measures;
  const ts = report.test_signals?.summary;
  setMetric("m-bugs", m.bugs);
  setMetric("m-vulns", m.vulnerabilities);
  setMetric("m-smells", m.code_smells);
  setMetricText("m-coverage", formatPercent(m.coverage));
  setMetric("m-ncloc", m.ncloc);
  setMetric("m-files", m.files);
  setMetric("m-comments", m.comments);

  if (ts) {
    setMetric("m-tests", ts.tests ?? 0);
    setMetric("m-test-failures", ts.test_failures ?? 0);
    setMetric("m-test-skipped", ts.test_skipped ?? 0);
    setMetric("m-mutants-skipped", ts.mutants_skipped ?? m.mutants_skipped ?? 0);
    setMetric("m-mutants-error", ts.mutants_error ?? m.mutants_error ?? 0);
  } else {
    setMetric("m-tests", m.tests ?? 0);
    setMetric("m-test-failures", m.test_failures ?? 0);
    setMetric("m-test-skipped", m.test_skipped ?? 0);
    setMetric("m-mutants-skipped", m.mutants_skipped ?? 0);
    setMetric("m-mutants-error", m.mutants_error ?? 0);
  }

  // Color-code the cards
  colorCard("card-bugs", m.bugs, [0, 1, 5]);
  colorCard("card-vulns", m.vulnerabilities, [0, 1, 3]);
  colorCard("card-smells", m.code_smells, [0, 10, 50]);
  colorCoverageCard("card-coverage", m.coverage);
  colorInverseCard("card-tests", ts?.tests ?? m.tests, [50, 20, 0]);
  colorCard("card-test-failures", ts?.test_failures ?? m.test_failures ?? 0, [0, 1, 5]);
  addClass("card-ncloc", "card-neutral");
  addClass("card-files", "card-neutral");
  addClass("card-comments", "card-neutral");
  addClass("card-test-skipped", "card-neutral");
  addClass("card-mutants-skipped", "card-neutral");
  addClass("card-mutants-error", (ts?.mutants_error ?? m.mutants_error ?? 0) > 0 ? "card-red" : "card-neutral");

  const dup = m.duplicated_lines_density;
  setMetricText("m-duplication", formatPercent(dup));
  colorCard("card-duplication", dup ?? 0, [3, 10, 20]);
  addClass("card-duplication", dup == null ? "card-neutral" : "");

  const health = report.test_signals?.health;
  if (health) {
    setMetricText("m-test-health", `${health.status} · ${health.score}`);
    const healthCard = el("card-test-health");
    healthCard.classList.remove("card-neutral", "card-green", "card-yellow", "card-red");
    if (health.status === "healthy") healthCard.classList.add("card-green");
    else if (health.status === "at_risk") healthCard.classList.add("card-red");
    else if (health.status === "partial") healthCard.classList.add("card-yellow");
    else healthCard.classList.add("card-neutral");
  } else {
    setMetricText("m-test-health", "—");
    addClass("card-test-health", "card-neutral");
  }

  const coverageCard = el("card-coverage");
  coverageCard.classList.add("clickable");
  coverageCard.addEventListener("click", () => {
    switchTab("coverage");
  });

  makeMetricLink("card-bugs", () => navigateToIssues("bug"));
  makeMetricLink("card-vulns", () => navigateToIssues("vulnerability"));
  makeMetricLink("card-smells", () => navigateToIssues("code_smell"));
  makeMetricLink("card-test-failures", () => switchTab("mutants"));
  makeMetricLink("card-mutants-error", () => switchTab("mutants"));
}

function makeMetricLink(id: string, onClick: () => void): void {
  const card = el(id);
  card.classList.add("clickable");
  card.setAttribute("role", "button");
  card.setAttribute("tabindex", "0");
  card.setAttribute("aria-label", `View ${card.querySelector(".metric-label")?.textContent ?? ""} details`);
  card.addEventListener("click", () => {
    console.log("[Ollanta] Clicked metric card:", id);
    onClick();
  });
  card.addEventListener("keydown", (e: Event) => {
    const ke = e as KeyboardEvent;
    if (ke.key === "Enter" || ke.key === " ") {
      ke.preventDefault();
      onClick();
    }
  });
  // Add arrow indicator
  const arrow = document.createElement("span");
  arrow.className = "metric-arrow";
  arrow.setAttribute("aria-hidden", "true");
  arrow.textContent = "\u2192";
  card.appendChild(arrow);
}

function navigateToIssues(type: string): void {
  filterType = type;
  (el("filter-type") as HTMLSelectElement).value = type;
  applyFilters();
  switchTab("issues");
}

function setMetric(id: string, val: number): void {
  el(id).textContent = val.toLocaleString();
}

function setMetricText(id: string, val: string): void {
  el(id).textContent = val;
}

function formatPercent(val: number | undefined): string {
  return val == null ? "—" : `${val.toFixed(1)}%`;
}

function colorCard(id: string, val: number, thresholds: [number, number, number]): void {
  if (val <= thresholds[0]) addClass(id, "card-green");
  else if (val <= thresholds[1]) addClass(id, "card-yellow");
  else addClass(id, "card-red");
}

function colorCoverageCard(id: string, val: number | undefined): void {
  if (val == null) addClass(id, "card-neutral");
  else if (val >= 80) addClass(id, "card-green");
  else if (val >= 60) addClass(id, "card-yellow");
  else addClass(id, "card-red");
}

function colorInverseCard(id: string, val: number | undefined, thresholds: [number, number, number]): void {
  if (val == null) { addClass(id, "card-neutral"); return; }
  if (val >= thresholds[0]) addClass(id, "card-green");
  else if (val >= thresholds[1]) addClass(id, "card-yellow");
  else addClass(id, "card-red");
}

// ══════════════════════════════════════════════════════════════════════════
// Mutation testing summary
// ══════════════════════════════════════════════════════════════════════════

function renderMutationSummary(): void {
  const container = el("mutation-summary");
  const mutation = mutationOverview();
  if (!mutation.hasSignal) {
    container.innerHTML = `<div class="empty-state compact">No mutation report was collected for this scan. Add a supported report such as <span class="mono">ollanta-mutations.json</span>, Stryker JSON, or PIT XML to see mutation score and survived mutants.</div>`;
    return;
  }

  container.innerHTML = `<div class="mutation-kpis">
      <div><span class="mutation-kpi-value ${mutationScoreClass(mutation.score)}">${formatPercent(mutation.score)}</span><span class="mutation-kpi-label">mutation score</span></div>
      <div><span class="mutation-kpi-value">${mutation.killed.toLocaleString()}/${mutation.total.toLocaleString()}</span><span class="mutation-kpi-label">killed mutants</span></div>
      <div><span class="mutation-kpi-value ${mutation.survived > 0 ? 'mut-warning' : 'mut-success'}">${mutation.survived.toLocaleString()}</span><span class="mutation-kpi-label">survived mutants</span></div>
    </div>
    ${renderMutationModules(mutation.modules)}
    ${renderSurvivedMutants(mutation.survivedMutants)}`;
}

function mutationOverview(): { hasSignal: boolean; score?: number; total: number; killed: number; survived: number; modules: TestModuleSignal[]; survivedMutants: TestMutantSignal[] } {
  const summary = report.test_signals?.summary;
  const modules = (report.test_signals?.modules ?? []).filter(module => module.mutation);
  const total = summary?.changed_mutants_total || summary?.mutants_total || report.measures.changed_mutants_total || report.measures.mutants_total || 0;
  const killed = summary?.changed_mutants_killed || summary?.mutants_killed || report.measures.changed_mutants_killed || report.measures.mutants_killed || 0;
  const survived = summary?.changed_mutants_survived || summary?.mutants_survived || report.measures.changed_mutants_survived || report.measures.mutants_survived || 0;
  const score = summary?.changed_mutation_score ?? summary?.mutation_score ?? report.measures.changed_mutation_score ?? report.measures.mutation_score;
  const survivedMutants = modules.flatMap(module => module.mutation?.survived_mutants ?? []).slice(0, 8);
  return { hasSignal: modules.length > 0 || total > 0 || score != null, score, total, killed, survived, modules, survivedMutants };
}

function renderMutationModules(modules: TestModuleSignal[]): string {
  if (!modules.length) return "";
  return `<div class="mutation-module-list">
    ${modules.slice(0, 5).map(module => {
      const mutation = module.mutation as TestMutationSummary;
      const score = mutation.changed_code_score ?? mutation.score;
      const survived = mutation.changed_survived ?? mutation.survived ?? 0;
      const total = mutation.changed_total ?? mutation.total ?? 0;
      return `<div class="mutation-module-row">
        <span class="mutation-module-main"><span class="mutation-module-name">${esc(module.name || module.root)}</span><span class="mutation-module-meta">${esc(mutation.tool || 'mutation')} · ${total.toLocaleString()} mutants</span></span>
        <span class="mutation-pill ${mutationScoreClass(score)}">${formatPercent(score)}</span>
        <span class="mutation-survived ${survived > 0 ? 'mut-warning' : 'mut-success'}">${survived.toLocaleString()} survived</span>
      </div>`;
    }).join("")}
  </div>`;
}

function renderSurvivedMutants(mutants: TestMutantSignal[]): string {
  if (!mutants.length) return "";
  return `<div class="mutation-survivors">
    ${mutants.map(mutant => `<div class="mutation-survivor-row">
      <span class="mutation-survivor-file">${esc(shortenPath(mutant.file || ''))}${mutant.line ? `:L${mutant.line}` : ''}</span>
      <span class="mutation-survivor-meta">${esc(mutant.mutator || mutant.description || 'survived mutant')}</span>
    </div>`).join("")}
  </div>`;
}

function mutationScoreClass(score: number | undefined): string {
  if (score == null) return "card-neutral";
  if (score >= 80) return "card-green";
  if (score >= 60) return "card-yellow";
  return "card-red";
}

// ══════════════════════════════════════════════════════════════════════════
// Mutants Tab (full table with filters + sort)
// ══════════════════════════════════════════════════════════════════════════

function renderMutantsTab(): void {
  const container = el("mutants-page");
  const mutation = mutationOverview();
  if (!mutation.hasSignal) {
    container.innerHTML = `<div class="empty-state">No mutation data collected. Run with <span class="mono">-with-mutations</span> to see survived mutants.</div>`;
    return;
  }

  const allSurvived = mutation.modules.flatMap(module => (module.mutation?.survived_mutants ?? []).map(m => ({ ...m, moduleName: module.name || module.root })));
  const modules = [...new Set(allSurvived.map(m => m.moduleName))].sort();

  let display = allSurvived;
  if (mutantFilterModule !== "all") {
    display = display.filter(m => m.moduleName === mutantFilterModule);
  }
  display.sort((a, b) => {
    let cmp = 0;
    if (mutantSortField === "file") cmp = (a.file || "").localeCompare(b.file || "");
    else if (mutantSortField === "line") cmp = (a.line ?? 0) - (b.line ?? 0);
    else if (mutantSortField === "module") cmp = a.moduleName.localeCompare(b.moduleName);
    return mutantSortDir === "asc" ? cmp : -cmp;
  });

  const toolbarHtml = `
    <div class="mutants-toolbar">
      <div class="toolbar-left">
        <select id="mutant-filter-module">
          <option value="all">All modules</option>
          ${modules.map(m => `<option value="${esc(m)}"${m === mutantFilterModule ? " selected" : ""}>${esc(m)}</option>`).join("")}
        </select>
        <select id="mutant-sort-field">
          <option value="file"${mutantSortField === "file" ? " selected" : ""}>Sort by file</option>
          <option value="line"${mutantSortField === "line" ? " selected" : ""}>Sort by line</option>
          <option value="module"${mutantSortField === "module" ? " selected" : ""}>Sort by module</option>
        </select>
        <select id="mutant-sort-dir">
          <option value="asc"${mutantSortDir === "asc" ? " selected" : ""}>Ascending</option>
          <option value="desc"${mutantSortDir === "desc" ? " selected" : ""}>Descending</option>
        </select>
      </div>
      <div class="toolbar-right">
        <span class="result-count">${display.length.toLocaleString()} survived</span>
      </div>
    </div>
  `;

  const kpisHtml = `
    <div class="mutation-kpis">
      <div><span class="mutation-kpi-value ${mutationScoreClass(mutation.score)}">${formatPercent(mutation.score)}</span><span class="mutation-kpi-label">mutation score</span></div>
      <div><span class="mutation-kpi-value">${mutation.killed.toLocaleString()}/${mutation.total.toLocaleString()}</span><span class="mutation-kpi-label">killed mutants</span></div>
      <div><span class="mutation-kpi-value ${mutation.survived > 0 ? 'mut-warning' : 'mut-success'}">${mutation.survived.toLocaleString()}</span><span class="mutation-kpi-label">survived mutants</span></div>
    </div>
  `;

  let tableHtml = "";
  if (display.length) {
    tableHtml = `
      <table class="mutants-table">
        <thead>
          <tr>
            <th>File</th>
            <th>Line</th>
            <th>Module</th>
            <th>Mutator</th>
            <th>Description</th>
          </tr>
        </thead>
        <tbody>
          ${display.map(mutant => `
            <tr class="mutant-row">
              <td class="mutant-file">${esc(shortenPath(mutant.file || ''))}</td>
              <td class="mutant-line">${mutant.line ?? '—'}</td>
              <td class="mutant-module">${esc(mutant.moduleName)}</td>
              <td class="mutant-mutator">${esc(mutant.mutator || '—')}</td>
              <td class="mutant-desc">${esc(mutant.description || mutant.replacement || 'survived mutant')}</td>
            </tr>
          `).join("")}
        </tbody>
      </table>
    `;
  } else {
    tableHtml = `<div class="empty-state compact">No survived mutants match the current filter.</div>`;
  }

  container.innerHTML = toolbarHtml + kpisHtml + tableHtml;

  // Re-bind filter controls after re-render
  setupMutantFilters();
}

function setupMutantFilters(): void {
  const moduleSel = document.getElementById("mutant-filter-module") as HTMLSelectElement | null;
  const sortFieldSel = document.getElementById("mutant-sort-field") as HTMLSelectElement | null;
  const sortDirSel = document.getElementById("mutant-sort-dir") as HTMLSelectElement | null;

  moduleSel?.addEventListener("change", e => {
    mutantFilterModule = (e.target as HTMLSelectElement).value;
    renderMutantsTab();
  });
  sortFieldSel?.addEventListener("change", e => {
    mutantSortField = (e.target as HTMLSelectElement).value as "file" | "line" | "module";
    renderMutantsTab();
  });
  sortDirSel?.addEventListener("change", e => {
    mutantSortDir = (e.target as HTMLSelectElement).value as "asc" | "desc";
    renderMutantsTab();
  });
}

// ══════════════════════════════════════════════════════════════════════════
// Test Signal Modules
// ══════════════════════════════════════════════════════════════════════════

function renderTestSignalModules(): void {
  const modules = report.test_signals?.modules ?? [];
  if (!modules.length) return;

  const container = el("ts-modules");
  let html = "";

  for (const module of modules) {
    const health = module.health;
    const coverage = module.coverage;
    const mutation = module.mutation;
    const suites = module.suites ?? [];
    const failed = suites.reduce((sum, suite) => sum + (suite.failures ?? 0) + (suite.errors ?? 0), 0);
    const total = suites.reduce((sum, suite) => sum + (suite.tests ?? 0), 0);
    const role = module.architecture_role ?? "";
    const roleBadge = role ? `<span class="ts-role-badge">${esc(role)}</span>` : "";

    const healthCls = healthClass(health?.status);
    const covPct = formatPercent(coverage?.coverage);
    const covCls = coverageClass(coverage?.coverage ?? undefined);
    const mutPct = formatPercent(mutation?.changed_code_score ?? mutation?.score);
    const mutCls = mutationScoreClass(mutation?.changed_code_score ?? mutation?.score);

    html += `<div class="ts-module">
      <div class="ts-module-head">
        <span class="ts-module-name">${esc(module.name)}</span>
        ${roleBadge}
        <span class="ts-health-badge ${healthCls}">${health?.status ?? "no data"}</span>
      </div>
      <div class="ts-module-meta">
        ${coverage ? `<span class="ts-metric ${covCls}">Coverage ${covPct}</span>` : ""}
        ${mutation?.score != null || mutation?.changed_code_score != null ? `<span class="ts-metric ${mutCls}">Mutation ${mutPct}</span>` : ""}
        ${total > 0 ? `<span class="ts-metric">${total} test${total === 1 ? "" : "s"}</span>` : ""}
        ${failed > 0 ? `<span class="ts-metric ts-fail">${failed} failed</span>` : ""}
      </div>
      ${health?.recommendations?.length ? `<div class="ts-recommendations">${health.recommendations.map(r => `<div class="ts-rec">${esc(r)}</div>`).join("")}</div>` : ""}
    </div>`;
  }

  const health = report.test_signals?.health;
  const healthSummary = health
    ? `<span class="ts-health-badge ${healthClass(health.status)}">${health.status} · score ${health.score}</span>`
    : "";

  container.innerHTML = `<div class="ts-header">
      <h3>Test Signals</h3>
      ${healthSummary}
    </div>
    <div class="ts-module-list">${html || `<div class="empty-state compact">No module-level test data was collected.</div>`}</div>`;
}

function healthClass(status: string | undefined): string {
  if (status === "healthy") return "card-green";
  if (status === "at_risk") return "card-red";
  if (status === "partial") return "card-yellow";
  return "card-neutral";
}

// ══════════════════════════════════════════════════════════════════════════
// Overview Charts (severity + type distribution bars)
// ══════════════════════════════════════════════════════════════════════════

function renderOverviewCharts(): void {
  // Severity distribution
  const sevCounts = countBy(allIssues, i => i.severity);
  const sevMax = Math.max(1, ...Object.values(sevCounts));
  const sevOrder: Severity[] = ["blocker", "critical", "major", "minor", "info"];

  let sevHtml = "";
  let propHtml = "";
  const total = allIssues.length || 1;

  for (const sev of sevOrder) {
    const count = sevCounts[sev] ?? 0;
    const pct = (count / sevMax) * 100;
    const color = SEV_COLORS[sev] ?? "#64748b";
    sevHtml += `<div class="sev-bar-row">
      <span class="sev-bar-label">${sev}</span>
      <div class="sev-bar-track"><div class="sev-bar-fill" style="width:${pct}%;background:${color}"></div></div>
      <span class="sev-bar-count">${count}</span>
    </div>`;
    if (count > 0) {
      propHtml += `<div class="sev-segment" style="width:${(count/total)*100}%;background:${color}" title="${sev}: ${count}"></div>`;
    }
  }
  el("sev-bars").innerHTML = sevHtml;
  el("sev-proportional").innerHTML = propHtml;

  // Type distribution
  const typeCounts = countBy(allIssues, i => i.type);
  const typeMax = Math.max(1, ...Object.values(typeCounts));
  const typeColors: Record<string, string> = {
    bug: "#ef4444", vulnerability: "#f97316", code_smell: "#22c55e",
    security_hotspot: "#eab308",
  };

  let typeHtml = "";
  for (const [type, label] of Object.entries(TYPE_LABELS)) {
    const count = typeCounts[type] ?? 0;
    const pct = (count / typeMax) * 100;
    const color = typeColors[type] ?? "#64748b";
    typeHtml += `<div class="sev-bar-row">
      <span class="sev-bar-label">${label}</span>
      <div class="sev-bar-track"><div class="sev-bar-fill" style="width:${pct}%;background:${color}"></div></div>
      <span class="sev-bar-count">${count}</span>
    </div>`;
  }
  el("type-bars").innerHTML = typeHtml;
}

// ══════════════════════════════════════════════════════════════════════════
// Priority Queue (highest-signal issues for the overview)
// ══════════════════════════════════════════════════════════════════════════

function renderPriorityIssues(): void {
  const topIssues = [...allIssues]
    .sort((a, b) => {
      const severityDelta = (SEV_ORDER[a.severity] ?? 99) - (SEV_ORDER[b.severity] ?? 99);
      if (severityDelta !== 0) return severityDelta;
      return a.component_path.localeCompare(b.component_path) || a.line - b.line;
    })
    .slice(0, 6);

  if (!topIssues.length) {
    el("priority-issues").innerHTML = `<div class="empty-state compact">No issues found</div>`;
    return;
  }

  el("priority-issues").innerHTML = topIssues.map((issue, idx) => {
    const color = SEV_COLORS[issue.severity] ?? "#64748b";
    const file = shortenPath(issue.component_path);
    return `<button class="priority-row" data-idx="${idx}">
      <span class="issue-sev-dot" style="background:${color}"></span>
      <span class="priority-main">
        <span class="priority-title">${esc(issue.message)}</span>
        <span class="priority-meta" title="${esc(issue.component_path)}">${esc(file)}:L${issue.line} · ${esc(issue.rule_key)}</span>
      </span>
      <span class="priority-severity">${esc(issue.severity)}</span>
    </button>`;
  }).join("");

  el("priority-issues").querySelectorAll(".priority-row").forEach(row => {
    row.addEventListener("click", () => {
      const idx = Number.parseInt((row as HTMLElement).dataset["idx"]!, 10);
      openDetail(topIssues[idx]);
    });
  });
}

// ══════════════════════════════════════════════════════════════════════════
// Hotspot Files (top 10 files with most issues)
// ══════════════════════════════════════════════════════════════════════════

function renderHotspotFiles(): void {
  const fileCounts = countBy(allIssues, i => i.component_path);
  const sorted = Object.entries(fileCounts).sort((a, b) => b[1] - a[1]).slice(0, 10);

  if (!sorted.length) {
    el("hotspot-files").innerHTML = `<div class="empty-state">No issues found</div>`;
    return;
  }

  el("hotspot-files").innerHTML = sorted.map(([path, count]) => {
    const short = shortenPath(path);
    return `<div class="hotspot-row" data-path="${esc(path)}">
      <span class="hotspot-file" title="${esc(path)}">${esc(short)}</span>
      <span class="hotspot-count">${count}</span>
    </div>`;
  }).join("");

  // Click to jump to issues tab in grouped mode
  el("hotspot-files").querySelectorAll(".hotspot-row").forEach(row => {
    row.addEventListener("click", () => {
      const path = (row as HTMLElement).dataset["path"]!;
      issuesViewMode = "grouped";
      switchTab("issues");
      expandFileGroup(path);
    });
  });
}

// ══════════════════════════════════════════════════════════════════════════
// Coverage drill-down
// ══════════════════════════════════════════════════════════════════════════

function buildCoverageFiles(): void {
  const modules = report.test_signals?.modules ?? [];
  coverageFiles = modules.flatMap(module => (module.files ?? []).map(file => coverageFileView(module.name, module.root, file)))
    .filter(file => file.linesToCover > 0)
    .sort((left, right) => (left.coverage ?? 101) - (right.coverage ?? 101) || right.uncoveredLines.length - left.uncoveredLines.length || left.path.localeCompare(right.path));
  if (!selectedCoveragePath && coverageFiles.length) {
    selectedCoveragePath = coverageFiles[0].path;
  }
}

function coverageFileView(moduleName: string, moduleRoot: string, file: TestFileCoverage): CoverageFileView {
  const linesToCover = file.lines_to_cover ?? 0;
  const coveredLines = file.covered_lines ?? 0;
  const coverage = linesToCover > 0 ? coveredLines * 100 / linesToCover : null;
  return {
    moduleName,
    moduleRoot,
    path: file.path,
    linesToCover,
    coveredLines,
    coveredLineNumbers: file.covered_line_numbers ?? [],
    uncoveredLines: file.uncovered_lines ?? [],
    coverage,
  };
}

function renderCoverageSummary(): void {
  const container = el("coverage-summary");
  if (!coverageFiles.length) {
    container.innerHTML = `<div class="empty-state compact">Run with <span class="mono">-with-tests</span> and provide a coverage report to see file-level details.</div>`;
    return;
  }
  const summary = report.test_signals?.summary;
  const worstFiles = coverageFiles.slice(0, 5);
  container.innerHTML = `<div class="coverage-kpis">
      <div><span class="coverage-kpi-value">${formatPercent(summary?.coverage ?? report.measures.coverage)}</span><span class="coverage-kpi-label">overall</span></div>
      <div><span class="coverage-kpi-value">${(summary?.covered_lines ?? 0).toLocaleString()}/${(summary?.lines_to_cover ?? 0).toLocaleString()}</span><span class="coverage-kpi-label">covered lines</span></div>
      <div><span class="coverage-kpi-value">${(summary?.modules_with_coverage ?? 0).toLocaleString()}</span><span class="coverage-kpi-label">modules</span></div>
    </div>
    <div class="coverage-file-list">
      ${worstFiles.map(renderCoverageMiniRow).join("")}
    </div>`;

  container.querySelectorAll(".coverage-mini-row").forEach(row => {
    row.addEventListener("click", () => {
      const path = (row as HTMLElement).dataset["coveragePath"];
      if (path) {
        switchTab("coverage");
        void selectCoverageFile(path);
      }
    });
  });
}

function renderCoverageMiniRow(file: CoverageFileView): string {
  return `<button class="coverage-mini-row" data-coverage-path="${esc(file.path)}">
    <span class="coverage-mini-main">
      <span class="coverage-file-name" title="${esc(file.path)}">${esc(shortenPath(file.path))}</span>
      <span class="coverage-file-meta">${esc(file.moduleName)} · ${file.uncoveredLines.length.toLocaleString()} uncovered lines</span>
    </span>
    <span class="coverage-pill ${coverageClass(file.coverage)}">${formatPercent(file.coverage ?? undefined)}</span>
  </button>`;
}

function renderCoverageDetails(): void {
  const container = el("coverage-details");
  if (!coverageFiles.length) {
    container.innerHTML = `<div class="empty-state">No file-level coverage was collected for this scan.</div>`;
    return;
  }
  container.innerHTML = `<div class="coverage-toolbar">
      <div>
        <h3>Coverage Files</h3>
        <p>${coverageFiles.length.toLocaleString()} files with line-level detail. Overall coverage includes all measured files, not only those listed here.</p>
      </div>
      <span class="coverage-pill ${coverageClass(report.measures.coverage ?? null)}">${formatPercent(report.measures.coverage)}</span>
    </div>
    <div class="coverage-browser">
      <aside class="coverage-sidebar">
        <div class="coverage-file-list coverage-file-list-large">
          ${coverageFiles.map(renderCoverageDetailRow).join("")}
        </div>
      </aside>
      <section class="coverage-code-viewer">
        ${renderCoverageSourceViewer()}
      </section>
    </div>`;

  container.querySelectorAll(".coverage-row").forEach(row => {
    row.addEventListener("click", () => {
      const path = (row as HTMLElement).dataset["coveragePath"];
      if (path) {
        void selectCoverageFile(path);
      }
    });
  });
}

function renderCoverageDetailRow(file: CoverageFileView): string {
  const active = file.path === selectedCoveragePath ? " active" : "";
  return `<button class="coverage-row${active}" data-coverage-path="${esc(file.path)}">
    <div class="coverage-row-main">
      <div class="coverage-row-title" title="${esc(file.path)}">${esc(file.path)}</div>
      <div class="coverage-row-subtitle">${esc(file.moduleName)} · ${esc(file.moduleRoot)} · ${file.coveredLines.toLocaleString()}/${file.linesToCover.toLocaleString()} lines covered</div>
    </div>
    <div class="coverage-row-meter">
      <span class="coverage-pill ${coverageClass(file.coverage)}">${formatPercent(file.coverage ?? undefined)}</span>
      <div class="coverage-track"><div class="coverage-fill ${coverageClass(file.coverage)}" style="width:${file.coverage ?? 0}%"></div></div>
    </div>
  </button>`;
}

async function selectCoverageFile(path: string): Promise<void> {
  if (!coverageFiles.some(file => file.path === path)) {
    return;
  }
  selectedCoveragePath = path;
  coverageSourceError = "";
  if (coverageSourceCache.has(path)) {
    coverageSourceLoading = false;
    renderCoverageDetails();
    return;
  }
  coverageSourceLoading = true;
  renderCoverageDetails();
  try {
    const res = await fetch(`/api/files/source?path=${encodeURIComponent(path)}`);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const payload = await res.json() as { file: CoverageSourceFile };
    coverageSourceCache.set(path, payload.file);
  } catch (error) {
    coverageSourceError = `Could not load source for ${path}: ${String(error)}`;
  } finally {
    coverageSourceLoading = false;
    renderCoverageDetails();
  }
}

function renderCoverageSourceViewer(): string {
  const coverageFile = coverageFiles.find(file => file.path === selectedCoveragePath);
  if (!coverageFile) {
    return `<div class="code-empty"><p>Select a file to inspect coverage.</p></div>`;
  }
  if (coverageSourceLoading) {
    return `<div class="code-empty"><div class="spinner"></div></div>`;
  }
  if (coverageSourceError) {
    return `<div class="code-empty"><p>${esc(coverageSourceError)}</p></div>`;
  }
  const sourceFile = coverageSourceCache.get(coverageFile.path);
  if (!sourceFile) {
    return `<div class="code-empty"><p>Select a coverage file to inspect source lines.</p></div>`;
  }

  const covered = new Set(coverageFile.coveredLineNumbers);
  const uncovered = new Set(coverageFile.uncoveredLines);
  const lines = sourceFile.content.split("\n");
  const rows = lines.map((line, index) => {
    const lineNumber = index + 1;
    const isUncovered = uncovered.has(lineNumber);
    const isCovered = !isUncovered && covered.has(lineNumber);
    const presentation = coverageLinePresentation(isCovered, isUncovered);
    return `<div class="code-line${presentation.stateClass}">
      <span class="code-gutter">${lineNumber}</span>
      <code class="code-text">${line.length ? esc(line) : "&nbsp;"}</code>
      <span class="code-markers">${renderCoverageLineChip(presentation)}</span>
    </div>`;
  }).join("");
  const detailState = coverageFile.coveredLineNumbers.length ? "covered and uncovered lines" : "uncovered lines only";
  return `<div class="code-viewer-shell coverage-source-shell">
    <div class="code-viewer-head">
      <div>
        <div class="code-viewer-path mono">${esc(sourceFile.path)}</div>
        <div class="code-viewer-meta">${esc(sourceFile.language || "plain text")} · ${sourceFile.line_count.toLocaleString()} lines · ${detailState}</div>
      </div>
      <div class="code-viewer-stats"><span class="coverage-pill ${coverageClass(coverageFile.coverage)}">${formatPercent(coverageFile.coverage ?? undefined)}</span></div>
    </div>
    <div class="coverage-legend">
      <span><span class="legend-dot legend-covered"></span>Covered</span>
      <span><span class="legend-dot legend-uncovered"></span>Not covered</span>
    </div>
    <div class="code-surface">${rows}</div>
  </div>`;
}

function coverageLinePresentation(isCovered: boolean, isUncovered: boolean): { stateClass: string; marker: string; chipClass: string } {
  if (isUncovered) {
    return { stateClass: " is-uncovered", marker: "not covered", chipClass: " chip-uncovered" };
  }
  if (isCovered) {
    return { stateClass: " is-covered", marker: "covered", chipClass: " chip-covered" };
  }
  return { stateClass: "", marker: "", chipClass: "" };
}

function renderCoverageLineChip(presentation: { marker: string; chipClass: string }): string {
  if (!presentation.marker) {
    return "";
  }
  return `<span class="coverage-line-chip${presentation.chipClass}">${presentation.marker}</span>`;
}

function coverageClass(val: number | null | undefined): string {
  if (val == null) return "card-neutral";
  if (val >= 80) return "card-green";
  if (val >= 60) return "card-yellow";
  return "card-red";
}

// ══════════════════════════════════════════════════════════════════════════
// Languages
// ══════════════════════════════════════════════════════════════════════════

function renderLanguages(): void {
  const langs = Object.entries(report.measures.by_language).sort((a, b) => b[1] - a[1]);
  const max = Math.max(1, langs[0]?.[1] ?? 1);
  if (!langs.length) {
    el("by-lang").innerHTML = `<span class="empty-state">No language data</span>`;
    return;
  }
  el("by-lang").innerHTML = langs.map(([lang, count]) =>
    `<div class="lang-row">
      <span class="lang-name">${esc(lang)}</span>
      <div class="lang-bar-track"><div class="lang-bar-fill" style="width:${(count/max)*100}%"></div></div>
      <span class="lang-count">${count} files</span>
    </div>`
  ).join("");
}

// ══════════════════════════════════════════════════════════════════════════
// Tabs
// ══════════════════════════════════════════════════════════════════════════

function setupTabs(): void {
  document.querySelectorAll(".tab").forEach(btn => {
    btn.addEventListener("click", () => {
      const tab = (btn as HTMLElement).dataset["tab"]!;
      switchTab(tab);
    });
  });

  const viewToggle = document.getElementById("view-mode-toggle");
  if (viewToggle) {
    viewToggle.addEventListener("click", () => {
      issuesViewMode = issuesViewMode === "flat" ? "grouped" : "flat";
      viewToggle.textContent = issuesViewMode === "grouped" ? "List view" : "Group by file";
      viewToggle.setAttribute("aria-pressed", String(issuesViewMode === "grouped"));
      applyFilters();
    });
  }

  const tablist = document.querySelector(".tabs")!;
  tablist.addEventListener("keydown", (e: KeyboardEvent) => {
    const tabs = Array.from(document.querySelectorAll<HTMLElement>(".tab[role='tab']"));
    const currentIndex = tabs.findIndex(t => t.getAttribute("aria-selected") === "true");
    if (e.key === "ArrowRight") {
      e.preventDefault();
      const next = tabs[(currentIndex + 1) % tabs.length];
      next.focus();
      switchTab(next.dataset["tab"]!);
    } else if (e.key === "ArrowLeft") {
      e.preventDefault();
      const prev = tabs[(currentIndex - 1 + tabs.length) % tabs.length];
      prev.focus();
      switchTab(prev.dataset["tab"]!);
    }
  });
}

function switchTab(tab: string): void {
  activeTab = tab;
  document.querySelectorAll(".tab").forEach(t => {
    t.classList.remove("active");
    t.setAttribute("aria-selected", "false");
  });
  const activeBtn = document.querySelector(`.tab[data-tab="${tab}"]`) as HTMLElement | null;
  activeBtn?.classList.add("active");
  activeBtn?.setAttribute("aria-selected", "true");
  document.querySelectorAll(".panel").forEach(p => (p as HTMLElement).classList.add("hidden"));
  el(`panel-${tab}`).classList.remove("hidden");
}

// ══════════════════════════════════════════════════════════════════════════
// Filters + Severity Chips
// ══════════════════════════════════════════════════════════════════════════

function populateFilters(): void {
  const rules = [...new Set(allIssues.map(i => i.rule_key))].sort((left, right) => left.localeCompare(right));
  const sel = el("filter-rule") as HTMLSelectElement;
  rules.forEach(r => {
    const opt = document.createElement("option");
    opt.value = r; opt.textContent = r;
    sel.appendChild(opt);
  });

  const tagSet = new Set<string>();
  for (const issue of allIssues) {
    for (const tag of issue.tags ?? []) {
      tagSet.add(tag);
    }
  }
  const tagSel = el("filter-tag") as HTMLSelectElement;
  [...tagSet].sort().forEach(t => {
    const opt = document.createElement("option");
    opt.value = t; opt.textContent = t;
    tagSel.appendChild(opt);
  });

  el("filter-severity").addEventListener("change", e => {
    filterSeverity = (e.target as HTMLSelectElement).value;
    applyFilters();
  });
  el("filter-type").addEventListener("change", e => {
    filterType = (e.target as HTMLSelectElement).value;
    applyFilters();
  });
  sel.addEventListener("change", e => {
    filterRule = (e.target as HTMLSelectElement).value;
    applyFilters();
  });
  el("filter-quality").addEventListener("change", e => {
    filterQuality = (e.target as HTMLSelectElement).value;
    applyFilters();
  });
  tagSel.addEventListener("change", e => {
    filterTag = (e.target as HTMLSelectElement).value;
    applyFilters();
  });
  el("search").addEventListener("input", e => {
    searchText = (e.target as HTMLInputElement).value.toLowerCase();
    applyFilters();
  });

  renderSevChips();
}

function renderSevChips(): void {
  const sevCounts = countBy(allIssues, i => i.severity);
  const sevs: Severity[] = ["blocker", "critical", "major", "minor", "info"];
  el("sev-chips").innerHTML = sevs.map(sev => {
    const count = sevCounts[sev] ?? 0;
    const color = SEV_COLORS[sev] ?? "#64748b";
    const active = filterSeverity === sev ? " active" : "";
    return `<button class="sev-chip${active}" data-sev="${sev}"
      style="--chip-color:${color};--chip-bg:${color}15" aria-pressed="${active ? "true" : "false"}">
      <span class="chip-dot" style="background:${color}"></span>
      ${sev}
      <span class="chip-count">${count}</span>
    </button>`;
  }).join("");

  el("sev-chips").querySelectorAll(".sev-chip").forEach(chip => {
    chip.addEventListener("click", () => {
      const sev = (chip as HTMLElement).dataset["sev"]!;
      filterSeverity = filterSeverity === sev ? "all" : sev;
      (el("filter-severity") as HTMLSelectElement).value = filterSeverity;
      applyFilters();
      renderSevChips();
    });
  });
}

// ══════════════════════════════════════════════════════════════════════════
// Filter + Render Issues
// ══════════════════════════════════════════════════════════════════════════

function applyFilters(): void {
  filteredIssues = allIssues.filter(i => {
    if (filterSeverity !== "all" && i.severity !== filterSeverity) return false;
    if (filterType !== "all" && i.type !== filterType) return false;
    if (filterRule !== "all" && i.rule_key !== filterRule) return false;
    if (filterQuality !== "all" && i.quality !== filterQuality) return false;
    if (filterTag !== "all" && !(i.tags ?? []).includes(filterTag)) return false;
    if (searchText) {
      const hay = `${i.component_path} ${i.message} ${i.rule_key}`.toLowerCase();
      if (!hay.includes(searchText)) return false;
    }
    return true;
  });

  // Sort: blocker first, then critical, etc.
  filteredIssues.sort((a, b) => {
    const sa = SEV_ORDER[a.severity] ?? 99;
    const sb = SEV_ORDER[b.severity] ?? 99;
    return sa - sb;
  });

  selectedIndex = -1;
  renderIssueList();

  const count = filteredIssues.length;
  const announcer = document.getElementById("filter-announcer");
  if (announcer) {
    announcer.textContent = `${count} issue${count === 1 ? "" : "s"} match the current filters`;
  }
}

function renderIssueList(): void {
  const flatContainer = el("issue-list");
  const groupedContainer = el("issue-grouped");
  const issueLabel = filteredIssues.length === 1 ? "issue" : "issues";
  el("issue-count").textContent = `${filteredIssues.length} ${issueLabel}`;

  if (!filteredIssues.length) {
    flatContainer.innerHTML = `<div class="empty-state">No issues match the current filters.</div>`;
    groupedContainer.innerHTML = `<div class="empty-state">No issues match the current filters.</div>`;
    return;
  }

  if (issuesViewMode === "grouped") {
    flatContainer.classList.add("hidden");
    groupedContainer.classList.remove("hidden");
    renderGroupedIssues();
    return;
  }

  flatContainer.classList.remove("hidden");
  groupedContainer.classList.add("hidden");

  flatContainer.innerHTML = filteredIssues.map((issue, idx) => {
    const color = SEV_COLORS[issue.severity] ?? "#64748b";
    const file = shortenPath(issue.component_path);
    const loc = issue.end_line && issue.end_line !== issue.line
      ? `L${issue.line}–${issue.end_line}` : `L${issue.line}`;
    const typeLabel = TYPE_LABELS[issue.type] ?? issue.type;
    const qualityBadge = issue.quality ? `<span class="quality-badge quality-${esc(issue.quality)}">${esc(QUALITY_LABELS[issue.quality] ?? issue.quality)}</span>` : "";
    return `<div class="issue-row" role="button" tabindex="0" aria-label="${esc(issue.severity)} issue: ${esc(issue.message)}" data-idx="${idx}">
      <span class="issue-sev">
        <span class="issue-sev-dot" style="background:${color}"></span>
        ${esc(issue.severity)}
      </span>
      <span class="issue-type">${esc(typeLabel)}</span>
      <div class="issue-main">
        <span class="issue-msg">${esc(issue.message)}</span>
        <span class="issue-file" title="${esc(issue.component_path)}">${esc(file)}:${loc}</span>
      </div>
      ${qualityBadge}
      <span class="issue-rule">${esc(issue.rule_key)}</span>
    </div>`;
  }).join("");

  // Click / keyboard handlers
  flatContainer.querySelectorAll(".issue-row").forEach(row => {
    row.addEventListener("click", () => {
      const idx = Number.parseInt((row as HTMLElement).dataset["idx"]!, 10);
      selectIssue(idx);
    });
    row.addEventListener("keydown", (e: Event) => {
      const ke = e as KeyboardEvent;
      if (ke.key === "Enter" || ke.key === " ") {
        ke.preventDefault();
        const idx = Number.parseInt((row as HTMLElement).dataset["idx"]!, 10);
        selectIssue(idx);
      }
    });
  });
}

// ══════════════════════════════════════════════════════════════════════════
// File Tree (issues grouped by file — SonarQube's "File" view)
// ══════════════════════════════════════════════════════════════════════════

function buildFileGroups(): void {
  const groups = new Map<string, Issue[]>();
  for (const issue of allIssues) {
    const path = issue.component_path;
    if (!groups.has(path)) groups.set(path, []);
    groups.get(path)!.push(issue);
  }
  fileGroups = [...groups.entries()]
    .sort((a, b) => b[1].length - a[1].length)
    .map(([path, issues]) => ({
      path,
      shortPath: shortenPath(path),
      issues: [...issues].sort((a, b) => a.line - b.line),
      expanded: false,
    }));
}

function renderGroupedIssues(): void {
  const container = el("issue-grouped");
  if (!fileGroups.length) {
    container.innerHTML = `<div class="empty-state">No issues found</div>`;
    return;
  }

  container.innerHTML = fileGroups.map((fg, gi) =>
    `<div class="file-group${fg.expanded ? " expanded" : ""}" data-gi="${gi}">
      <div class="file-group-header">
        <span class="file-group-chevron icon-chevron" role="img" aria-label="Expand"></span>
        <span class="file-group-name" title="${esc(fg.path)}">${esc(fg.shortPath)}</span>
        <span class="file-group-count">${fg.issues.length}</span>
      </div>
      <div class="file-group-issues" style="${fg.expanded ? "" : "display:none"}">
        ${fg.issues.map((issue, ii) => {
          const color = SEV_COLORS[issue.severity] ?? "#64748b";
          return `<div class="file-issue" data-gi="${gi}" data-ii="${ii}">
            <span class="issue-sev">
              <span class="issue-sev-dot" style="background:${color}"></span>
              ${esc(issue.severity)}
            </span>
            <span class="issue-msg">${esc(issue.message)}</span>
            <span class="file-issue-line">L${issue.line}</span>
          </div>`;
        }).join("")}
      </div>
    </div>`
  ).join("");

  // Expand/collapse handlers
  container.querySelectorAll(".file-group-header").forEach(hdr => {
    hdr.addEventListener("click", () => {
      const group = hdr.closest(".file-group") as HTMLElement;
      const gi = Number.parseInt(group.dataset["gi"]!, 10);
      fileGroups[gi].expanded = !fileGroups[gi].expanded;
      group.classList.toggle("expanded");
      const issues = group.querySelector(".file-group-issues") as HTMLElement;
      issues.style.display = fileGroups[gi].expanded ? "" : "none";
    });
  });

  // Issue click handlers
  container.querySelectorAll(".file-issue").forEach(row => {
    row.addEventListener("click", (e) => {
      e.stopPropagation();
      const gi = Number.parseInt((row as HTMLElement).dataset["gi"]!, 10);
      const ii = Number.parseInt((row as HTMLElement).dataset["ii"]!, 10);
      const issue = fileGroups[gi].issues[ii];
      openDetail(issue);
    });
  });
}

function expandFileGroup(path: string): void {
  const gi = fileGroups.findIndex(fg => fg.path === path);
  if (gi < 0) return;
  fileGroups[gi].expanded = true;
  renderGroupedIssues();
  const group = document.querySelector(`.file-group[data-gi="${gi}"]`);
  group?.scrollIntoView({ behavior: "smooth", block: "start" });
}

// ══════════════════════════════════════════════════════════════════════════
// Issue Detail Panel (slide-out — SonarQube's issue detail view)
// ══════════════════════════════════════════════════════════════════════════

function selectIssue(idx: number, open = true): void {
  selectedIndex = idx;
  selectedIssue = filteredIssues[idx] ?? null;
  // Highlight selected row
  document.querySelectorAll(".issue-row").forEach(r => r.classList.remove("selected"));
  const row = document.querySelector(`.issue-row[data-idx="${idx}"]`) as HTMLElement | null;
  row?.classList.add("selected");
  row?.focus();
  if (open && selectedIssue) openDetail(selectedIssue);
}

function openDetail(issue: Issue): void {
  lastFocusedElement = document.activeElement as HTMLElement | null;
  selectedIssue = issue;
  detailTab = "details";
  detailRuleHtml = "";
  detailRuleLoading = true;
  aiFixState = createEmptyAIFixState();
  el("detail-title").textContent = issue.message || issue.rule_key;
  renderDetailBody(issue);
  el("detail-panel").classList.add("open");
  el("detail-overlay").classList.add("open");

  const firstFocusable = el("detail-panel").querySelector<HTMLElement>("button, [href], input, select, textarea, [tabindex]:not([tabindex='-1'])");
  firstFocusable?.focus();

  void fetchRuleDetail(issue.rule_key);
}

async function fetchRuleDetail(ruleKey: string): Promise<void> {
  try {
    const res = await fetch(`/rules/${encodeURIComponent(ruleKey)}`);
    if (!res.ok) throw new Error("not found");
    const rule = await res.json();

    let rhtml = "";
    if (rule.rationale) {
      rhtml += `<div class="detail-section">
        <div class="detail-section-title">Why is this a problem?</div>
        <div class="rule-rationale">${esc(rule.rationale)}</div>
      </div>`;
    }
    if (rule.description && rule.description !== rule.rationale) {
      rhtml += `<div class="detail-section">
        <div class="detail-section-title">Description</div>
        <div class="rule-rationale">${esc(rule.description)}</div>
      </div>`;
    }
    if (rule.noncompliant_code) {
      rhtml += `<div class="detail-section">
        <div class="detail-section-title">${iconHtml("cross", "Noncompliant")} Noncompliant Code</div>
        <pre class="rule-code noncompliant"><code>${esc(rule.noncompliant_code)}</code></pre>
      </div>`;
    }
    if (rule.compliant_code) {
      rhtml += `<div class="detail-section">
        <div class="detail-section-title">${iconHtml("check", "Compliant")} Compliant Code</div>
        <pre class="rule-code compliant"><code>${esc(rule.compliant_code)}</code></pre>
      </div>`;
    }
    detailRuleHtml = rhtml || `<div class="detail-empty">No additional rule details available.</div>`;
  } catch {
    detailRuleHtml = `<div class="detail-empty">Rule details are not available for this issue.</div>`;
  } finally {
    detailRuleLoading = false;
    if (selectedIssue?.rule_key === ruleKey) {
      renderDetailBody(selectedIssue);
    }
  }
}

function bindDetailCopyButton(issue: Issue): void {
  document.getElementById("detailCopy")?.addEventListener("click", () => {
    void copyIssueDetail(issue);
  });
}

async function copyIssueDetail(issue: Issue): Promise<void> {
  const lines: string[] = [];
  lines.push(`Issue: ${issue.message || ""}`);
  lines.push(`Severity: ${issue.severity}`);
  lines.push(`Type: ${TYPE_LABELS[issue.type] ?? issue.type}`);
  lines.push(`Rule: ${issue.rule_key}`);
  if (issue.engine_id) lines.push(`Engine: ${issue.engine_id}`);
  lines.push(`File: ${issue.component_path}`);
  const loc = issue.end_line && issue.end_line !== issue.line
    ? `lines ${issue.line}\u2013${issue.end_line}`
    : `line ${issue.line}`;
  lines.push(`Location: ${loc}${issue.column ? ", column " + issue.column : ""}`);
  lines.push(`Status: ${issue.status}`);
  if (issue.tags?.length) lines.push(`Tags: ${issue.tags.join(", ")}`);

  try {
    const res = await fetch(`/rules/${encodeURIComponent(issue.rule_key)}`);
    if (res.ok) {
      const rule = await res.json();
      if (rule.rationale) lines.push(`\nWhy is this a problem?\n${rule.rationale}`);
      if (rule.noncompliant_code) lines.push(`\nNoncompliant code:\n${rule.noncompliant_code}`);
      if (rule.compliant_code) lines.push(`\nCompliant code:\n${rule.compliant_code}`);
    }
  } catch { /* rule details unavailable */ }

  try {
    await navigator.clipboard.writeText(lines.join("\n"));
    const btn = document.getElementById("detailCopy");
    if (btn) {
      btn.innerHTML = `${iconHtml("check", "Copied")} Copied`;
      setTimeout(() => { btn.innerHTML = `${iconHtml("copy", "Copy")} Copy`; }, 2000);
    }
  } catch {
    const btn = document.getElementById("detailCopy");
    if (btn) {
      btn.innerHTML = `${iconHtml("warn", "Failed")} Failed`;
      setTimeout(() => { btn.innerHTML = `${iconHtml("copy", "Copy")} Copy`; }, 2000);
    }
  }
}

function closeDetail(): void {
  el("detail-panel").classList.remove("open");
  el("detail-overlay").classList.remove("open");
  selectedIssue = null;
  detailRuleHtml = "";
  detailRuleLoading = false;
  aiFixState = createEmptyAIFixState();
  document.querySelectorAll(".issue-row").forEach(r => r.classList.remove("selected"));
  if (lastFocusedElement) {
    lastFocusedElement.focus();
    lastFocusedElement = null;
  }
}

function renderDetailBody(issue: Issue): void {
  const html = `
    <div class="detail-tabs">
      ${renderDetailTabs(detailTab)}
    </div>
    <div class="detail-tab-panel${detailTab === "details" ? "" : " hidden"}" data-detail-panel="details">
      ${renderIssueDetailContent(issue)}
    </div>
    <div class="detail-tab-panel${detailTab === "rule" ? "" : " hidden"}" data-detail-panel="rule">
      ${detailRuleLoading ? `<div class="detail-loading">Loading rule details\u2026</div>` : detailRuleHtml}
    </div>
    <div class="detail-tab-panel${detailTab === "ai-fix" ? "" : " hidden"}" data-detail-panel="ai-fix">
      ${renderAIFixContent(issue, aiFixState, cachedAIProviders ?? [])}
    </div>
  `;

  el("detail-body").innerHTML = html;
  bindDetailPanelEvents(issue);
  bindDetailCopyButton(issue);
}

function renderIssueDetailContent(issue: Issue): string {
  const color = SEV_COLORS[issue.severity] ?? "#64748b";
  const typeLabel = TYPE_LABELS[issue.type] ?? issue.type;
  const loc = issue.end_line && issue.end_line !== issue.line
    ? `${issue.line}:${issue.column} – ${issue.end_line}:${issue.end_column}`
    : `${issue.line}:${issue.column}`;

  let html = `
    <div class="detail-section">
      <div class="detail-msg">${esc(issue.message)}</div>
    </div>
    <div class="detail-section">
      <div class="detail-section-title">Properties</div>
      <div class="detail-field">
        <span class="detail-field-label">Severity</span>
        <span class="detail-field-value"><span class="issue-sev-dot" style="background:${color};display:inline-block;width:8px;height:8px;border-radius:50%;margin-right:6px"></span>${esc(issue.severity)}</span>
      </div>
      <div class="detail-field">
        <span class="detail-field-label">Type</span>
        <span class="detail-field-value">${esc(typeLabel)}</span>
      </div>
      ${issue.quality ? `<div class="detail-field">
        <span class="detail-field-label">Quality</span>
        <span class="detail-field-value"><span class="quality-badge quality-${esc(issue.quality)}">${esc(QUALITY_LABELS[issue.quality] ?? issue.quality)}</span></span>
      </div>` : ""}
      <div class="detail-field">
        <span class="detail-field-label">Rule</span>
        <span class="detail-field-value" style="font-family:var(--font-mono);color:var(--accent)">${esc(issue.rule_key)}</span>
      </div>
      <div class="detail-field">
        <span class="detail-field-label">Status</span>
        <span class="detail-field-value">${esc(issue.status)}</span>
      </div>
      ${issue.engine_id ? `<div class="detail-field">
        <span class="detail-field-label">Engine</span>
        <span class="detail-field-value">${esc(issue.engine_id)}</span>
      </div>` : ""}
      ${issue.tags?.length ? `<div class="detail-field">
        <span class="detail-field-label">Tags</span>
        <span class="detail-field-value">${issue.tags.map(t => esc(t)).join(", ")}</span>
      </div>` : ""}
    </div>
    <div class="detail-section">
      <div class="detail-section-title">Location</div>
      <div class="detail-field">
        <span class="detail-field-label">File</span>
        <span class="detail-field-value" style="font-family:var(--font-mono);font-size:12px;word-break:break-all">${esc(issue.component_path)}</span>
      </div>
      <div class="detail-field">
        <span class="detail-field-label">Lines</span>
        <span class="detail-field-value" style="font-family:var(--font-mono)">${loc}</span>
      </div>
    </div>`;

  if (issue.secondary_locations?.length) {
    html += `<div class="detail-section">
      <div class="detail-section-title">Related Locations (${issue.secondary_locations.length})</div>
      <div class="detail-loc-list">
        ${issue.secondary_locations.map(sl => `
          <div class="detail-loc-item">
            <div class="detail-loc-file">${esc(sl.file_path || issue.component_path)}:${sl.start_line}</div>
            ${sl.message ? `<div class="detail-loc-msg">${esc(sl.message)}</div>` : ""}
          </div>
        `).join("")}
      </div>
    </div>`;
  }

  return html;
}

function bindDetailPanelEvents(issue: Issue): void {
  document.querySelectorAll(".detail-tab").forEach(button => {
    button.addEventListener("click", () => {
      detailTab = ((button as HTMLElement).dataset["detailTab"] as "details" | "rule" | "ai-fix") ?? "details";
      renderDetailBody(issue);
      if (detailTab === "ai-fix") {
        void ensureAIFixProviders();
      }
    });
  });

  const providerSelect = document.getElementById("ai-provider-select") as HTMLSelectElement | null;
  providerSelect?.addEventListener("change", () => {
    aiFixState.selectedProviderId = providerSelect.value;
    aiFixState.selectedModel = "";
    syncAIFixSelection();
    aiFixState.preview = null;
    aiFixState.statusMessage = "";
    aiFixState.errorMessage = "";
    renderSelectedIssueDetail();
  });

  const modelInput = document.getElementById("ai-model-input") as HTMLInputElement | null;
  modelInput?.addEventListener("input", () => {
    aiFixState.selectedModel = modelInput.value;
  });

  const apiKeyInput = document.getElementById("ai-api-key-input") as HTMLInputElement | null;
  apiKeyInput?.addEventListener("input", () => {
    aiFixState.apiKey = apiKeyInput.value;
  });

  document.getElementById("ai-generate-fix")?.addEventListener("click", () => {
    void requestAIFixPreview(issue);
  });
  document.getElementById("ai-apply-fix")?.addEventListener("click", () => {
    void applyAIFixPreview();
  });
}

function createEmptyAIFixState(): AIFixPanelState {
  return {
    loadingOptions: false,
    loadingPreview: false,
    applying: false,
    selectedProviderId: "",
    selectedModel: "",
    apiKey: "",
    statusMessage: "",
    errorMessage: "",
    preview: null,
  };
}

function getSelectedAIProvider(): AIProviderOption | null {
  if (!cachedAIProviders || cachedAIProviders.length === 0) {
    return null;
  }
  return cachedAIProviders.find(provider => provider.id === aiFixState.selectedProviderId) ?? cachedAIProviders[0];
}

function syncAIFixSelection(): void {
  if (!cachedAIProviders || cachedAIProviders.length === 0) {
    aiFixState.selectedProviderId = "";
    aiFixState.selectedModel = "";
    return;
  }

  if (!cachedAIProviders.some(provider => provider.id === aiFixState.selectedProviderId)) {
    aiFixState.selectedProviderId = cachedAIProviders[0].id;
  }

  const selectedProvider = getSelectedAIProvider();
  if (!selectedProvider) {
    aiFixState.selectedModel = "";
    return;
  }

  if (!aiFixState.selectedModel) {
    aiFixState.selectedModel = selectedProvider.default_model || selectedProvider.models[0] || "";
  }
}

async function ensureAIFixProviders(): Promise<void> {
  if (cachedAIProviders) {
    syncAIFixSelection();
    renderSelectedIssueDetail();
    return;
  }

  aiFixState.loadingOptions = true;
  aiFixState.errorMessage = "";
  renderSelectedIssueDetail();
  try {
    const res = await fetch("/api/ai/providers");
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}`);
    }
    const data = await res.json() as AIProviderListResponse;
    cachedAIProviders = data.providers ?? [];
    syncAIFixSelection();
  } catch (error) {
    aiFixState.errorMessage = `Failed to load AI models: ${String(error)}`;
    cachedAIProviders = [];
  } finally {
    aiFixState.loadingOptions = false;
    renderSelectedIssueDetail();
  }
}

async function requestAIFixPreview(issue: Issue): Promise<void> {
  const selectedProvider = getSelectedAIProvider();
  const selectedModel = aiFixState.selectedModel.trim();

  if (!selectedProvider || !aiFixState.selectedProviderId) {
    aiFixState.errorMessage = "Choose an AI provider before generating a fix.";
    renderSelectedIssueDetail();
    return;
  }
  if (!selectedModel) {
    aiFixState.errorMessage = "Choose a model before generating a fix.";
    renderSelectedIssueDetail();
    return;
  }
  if (selectedProvider.requires_api_key && !selectedProvider.configured && !aiFixState.apiKey.trim()) {
    aiFixState.errorMessage = "Provide an API key for the selected provider before generating a fix.";
    renderSelectedIssueDetail();
    return;
  }

  aiFixState.selectedModel = selectedModel;
  aiFixState.loadingPreview = true;
  aiFixState.statusMessage = "";
  aiFixState.errorMessage = "";
  renderSelectedIssueDetail();
  try {
    const body = {
      provider: aiFixState.selectedProviderId,
      model: selectedModel,
      api_key: aiFixState.apiKey.trim() || undefined,
      issue,
    };
    const res = await fetch("/api/ai/fixes/preview", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const data = await res.json() as AIFixPreview | { error: string };
    if (!res.ok || "error" in data) {
      throw new Error("error" in data ? data.error : `HTTP ${res.status}`);
    }
    aiFixState.preview = data;
    aiFixState.statusMessage = "Fix preview generated. Review the diff before applying it.";
  } catch (error) {
    aiFixState.errorMessage = `Failed to generate AI fix: ${String(error)}`;
    aiFixState.preview = null;
  } finally {
    aiFixState.loadingPreview = false;
    renderSelectedIssueDetail();
  }
}

async function applyAIFixPreview(): Promise<void> {
  if (!aiFixState.preview) {
    return;
  }

  aiFixState.applying = true;
  aiFixState.errorMessage = "";
  renderSelectedIssueDetail();
  try {
    const res = await fetch("/api/ai/fixes/apply", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ preview_id: aiFixState.preview.preview_id }),
    });
    const data = await res.json() as AIFixApplyResponse | { error: string };
    if (!res.ok || "error" in data) {
      throw new Error("error" in data ? data.error : `HTTP ${res.status}`);
    }
    aiFixState.statusMessage = data.message;
  } catch (error) {
    aiFixState.errorMessage = `Failed to apply AI fix: ${String(error)}`;
  } finally {
    aiFixState.applying = false;
    renderSelectedIssueDetail();
  }
}

function renderSelectedIssueDetail(): void {
  if (selectedIssue) {
    renderDetailBody(selectedIssue);
  }
}

// Close handlers
document.addEventListener("DOMContentLoaded", () => {
  el("detail-close").addEventListener("click", closeDetail);
  el("detail-overlay").addEventListener("click", closeDetail);
});

// ══════════════════════════════════════════════════════════════════════════
// Keyboard Navigation (j/k to move, Enter to open, Esc to close)
// ══════════════════════════════════════════════════════════════════════════

function setupKeyboard(): void {
  document.addEventListener("keydown", (e: KeyboardEvent) => {
    // Don't capture when typing in inputs
    const tag = (e.target as HTMLElement).tagName;
    if (tag === "INPUT" || tag === "SELECT" || tag === "TEXTAREA") return;

    if (e.key === "Escape") {
      closeDetail();
      return;
    }

    if (activeTab !== "issues") return;

    if (e.key === "j" || e.key === "ArrowDown") {
      e.preventDefault();
      if (selectedIndex < filteredIssues.length - 1) selectIssue(selectedIndex + 1, false);
      scrollToSelected();
    } else if (e.key === "k" || e.key === "ArrowUp") {
      e.preventDefault();
      if (selectedIndex > 0) selectIssue(selectedIndex - 1, false);
      scrollToSelected();
    }
  });
}

function scrollToSelected(): void {
  const row = document.querySelector(`.issue-row[data-idx="${selectedIndex}"]`);
  row?.scrollIntoView({ behavior: "smooth", block: "nearest" });
}

// ══════════════════════════════════════════════════════════════════════════
// Helpers
// ══════════════════════════════════════════════════════════════════════════

function el(id: string): HTMLElement {
  return document.getElementById(id)!;
}

function addClass(id: string, cls: string): void {
  el(id).classList.add(cls);
}

function countBy<T>(items: T[], keyFn: (item: T) => string): Record<string, number> {
  const counts: Record<string, number> = {};
  for (const item of items) {
    const key = keyFn(item);
    counts[key] = (counts[key] ?? 0) + 1;
  }
  return counts;
}

function shortenPath(path: string): string {
  const normalized = path.replaceAll("\\", "/");
  const segments = normalized.split("/").filter(Boolean);
  if (segments.length <= 2) {
    return normalized;
  }
  return `${segments.slice(-2).join("/")}`;
}

