import { esc } from "./html";
import type { Report, Issue, Severity, GateResult, GateCondition, FileGroup, AIProviderListResponse, AIProviderOption, AIFixPreview, AIFixApplyResponse } from "./types";
import { renderAIFixContent, renderDetailTabs } from "./detailView";

// ══════════════════════════════════════════════════════════════════════════
// State
// ══════════════════════════════════════════════════════════════════════════

let report: Report;
let allIssues: Issue[] = [];
let filteredIssues: Issue[] = [];
let fileGroups: FileGroup[] = [];
let selectedIssue: Issue | null = null;
let selectedIndex = -1;
let activeTab = "overview";
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

let aiFixState: AIFixPanelState = createEmptyAIFixState();

let filterSeverity = "all";
let filterType = "all";
let filterRule = "all";
let searchText = "";

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
    renderHotspotFiles();
    renderLanguages();
    populateFilters();
    applyFilters();
    buildFileGroups();
    renderFileTree();
    setupTabs();
    setupKeyboard();

    el("tab-issue-count").textContent = String(allIssues.length);
    el("tab-file-count").textContent = String(new Set(allIssues.map(i => i.component_path)).size);
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
  const conditions: GateCondition[] = [
    { metric: "Bugs", operator: "=", threshold: 0, value: m.bugs, passed: m.bugs === 0 },
    { metric: "Vulnerabilities", operator: "=", threshold: 0, value: m.vulnerabilities, passed: m.vulnerabilities === 0 },
  ];
  const status = conditions.every(c => c.passed) ? "passed" : "failed";
  return { status, conditions };
}

function renderGate(): void {
  const gate = computeGate();
  const hero = el("gate-hero");
  hero.classList.remove("gate-loading");
  hero.classList.add(gate.status === "passed" ? "gate-passed" : "gate-failed");

  el("gate-icon").textContent = gate.status === "passed" ? "✓" : "✗";
  el("gate-status").textContent = gate.status === "passed" ? "Passed" : "Failed";

  const condHtml = gate.conditions.map(c => {
    const cls = c.passed ? "cond-pass" : "cond-fail";
    const icon = c.passed ? "✓" : "✗";
    return `<div class="gate-cond ${cls}">
      <span class="gate-cond-icon">${icon}</span>
      <span class="gate-cond-metric">${esc(c.metric)}</span>
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
  setMetric("m-bugs", m.bugs);
  setMetric("m-vulns", m.vulnerabilities);
  setMetric("m-smells", m.code_smells);
  setMetric("m-ncloc", m.ncloc);
  setMetric("m-files", m.files);
  setMetric("m-comments", m.comments);

  // Color-code the cards
  colorCard("card-bugs", m.bugs, [0, 1, 5]);
  colorCard("card-vulns", m.vulnerabilities, [0, 1, 3]);
  colorCard("card-smells", m.code_smells, [0, 10, 50]);
  addClass("card-ncloc", "card-neutral");
  addClass("card-files", "card-neutral");
  addClass("card-comments", "card-neutral");
}

function setMetric(id: string, val: number): void {
  el(id).textContent = val.toLocaleString();
}

function colorCard(id: string, val: number, thresholds: [number, number, number]): void {
  if (val <= thresholds[0]) addClass(id, "card-green");
  else if (val <= thresholds[1]) addClass(id, "card-yellow");
  else addClass(id, "card-red");
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

  // Click to jump to files tab
  el("hotspot-files").querySelectorAll(".hotspot-row").forEach(row => {
    row.addEventListener("click", () => {
      const path = (row as HTMLElement).dataset["path"]!;
      switchTab("files");
      expandFileGroup(path);
    });
  });
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
}

function switchTab(tab: string): void {
  activeTab = tab;
  document.querySelectorAll(".tab").forEach(t => t.classList.remove("active"));
  document.querySelector(`.tab[data-tab="${tab}"]`)?.classList.add("active");
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
    return `<div class="sev-chip${active}" data-sev="${sev}"
      style="--chip-color:${color};--chip-bg:${color}15">
      <span class="chip-dot" style="background:${color}"></span>
      ${sev}
      <span class="chip-count">${count}</span>
    </div>`;
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
}

function renderIssueList(): void {
  const container = el("issue-list");
  const issueLabel = filteredIssues.length === 1 ? "issue" : "issues";
  el("issue-count").textContent = `${filteredIssues.length} ${issueLabel}`;

  if (!filteredIssues.length) {
    container.innerHTML = `<div class="empty-state">No issues match the current filters.</div>`;
    return;
  }

  container.innerHTML = filteredIssues.map((issue, idx) => {
    const color = SEV_COLORS[issue.severity] ?? "#64748b";
    const file = shortenPath(issue.component_path);
    const loc = issue.end_line && issue.end_line !== issue.line
      ? `L${issue.line}–${issue.end_line}` : `L${issue.line}`;
    const typeLabel = TYPE_LABELS[issue.type] ?? issue.type;
    return `<div class="issue-row" data-idx="${idx}">
      <span class="issue-sev">
        <span class="issue-sev-dot" style="background:${color}"></span>
        ${esc(issue.severity)}
      </span>
      <span class="issue-type">${esc(typeLabel)}</span>
      <div class="issue-main">
        <span class="issue-msg">${esc(issue.message)}</span>
        <span class="issue-file" title="${esc(issue.component_path)}">${esc(file)}:${loc}</span>
      </div>
      <span class="issue-rule">${esc(issue.rule_key)}</span>
    </div>`;
  }).join("");

  // Click handlers
  container.querySelectorAll(".issue-row").forEach(row => {
    row.addEventListener("click", () => {
      const idx = Number.parseInt((row as HTMLElement).dataset["idx"]!, 10);
      selectIssue(idx);
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

function renderFileTree(): void {
  const container = el("file-tree");
  if (!fileGroups.length) {
    container.innerHTML = `<div class="empty-state">No issues found</div>`;
    return;
  }

  container.innerHTML = fileGroups.map((fg, gi) =>
    `<div class="file-group${fg.expanded ? " expanded" : ""}" data-gi="${gi}">
      <div class="file-group-header">
        <span class="file-group-chevron">▶</span>
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
  renderFileTree();
  const group = document.querySelector(`.file-group[data-gi="${gi}"]`);
  group?.scrollIntoView({ behavior: "smooth", block: "start" });
}

// ══════════════════════════════════════════════════════════════════════════
// Issue Detail Panel (slide-out — SonarQube's issue detail view)
// ══════════════════════════════════════════════════════════════════════════

function selectIssue(idx: number): void {
  selectedIndex = idx;
  selectedIssue = filteredIssues[idx] ?? null;
  // Highlight selected row
  document.querySelectorAll(".issue-row").forEach(r => r.classList.remove("selected"));
  document.querySelector(`.issue-row[data-idx="${idx}"]`)?.classList.add("selected");
  if (selectedIssue) openDetail(selectedIssue);
}

function openDetail(issue: Issue): void {
  selectedIssue = issue;
  detailTab = "details";
  detailRuleHtml = "";
  detailRuleLoading = true;
  aiFixState = createEmptyAIFixState();
  el("detail-title").textContent = issue.rule_key;
  renderDetailBody(issue);
  el("detail-panel").classList.add("open");
  el("detail-overlay").classList.add("open");

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
        <div class="detail-section-title">✘ Noncompliant Code</div>
        <pre class="rule-code noncompliant"><code>${esc(rule.noncompliant_code)}</code></pre>
      </div>`;
    }
    if (rule.compliant_code) {
      rhtml += `<div class="detail-section">
        <div class="detail-section-title">✔ Compliant Code</div>
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

function closeDetail(): void {
  el("detail-panel").classList.remove("open");
  el("detail-overlay").classList.remove("open");
  selectedIssue = null;
  detailRuleHtml = "";
  detailRuleLoading = false;
  aiFixState = createEmptyAIFixState();
  document.querySelectorAll(".issue-row").forEach(r => r.classList.remove("selected"));
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
      ${detailRuleLoading ? `<div class="detail-loading">Loading rule details…</div>` : detailRuleHtml}
    </div>
    <div class="detail-tab-panel${detailTab === "ai-fix" ? "" : " hidden"}" data-detail-panel="ai-fix">
      ${renderAIFixContent(issue, aiFixState, cachedAIProviders ?? [])}
    </div>
  `;

  el("detail-body").innerHTML = html;
  bindDetailPanelEvents(issue);
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
      if (selectedIndex < filteredIssues.length - 1) selectIssue(selectedIndex + 1);
      scrollToSelected();
    } else if (e.key === "k" || e.key === "ArrowUp") {
      e.preventDefault();
      if (selectedIndex > 0) selectIssue(selectedIndex - 1);
      scrollToSelected();
    } else if (e.key === "Enter") {
      if (selectedIssue) openDetail(selectedIssue);
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

