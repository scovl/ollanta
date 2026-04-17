import type { Report, Issue, Severity, IssueType } from "./types";

// ── State ───────────────────────────────────────────────────────────────────

let allIssues: Issue[] = [];
let filteredIssues: Issue[] = [];

let filterSeverity = "all";
let filterType = "all";
let filterRule = "all";
let searchText = "";
let sortKey: keyof Issue = "severity";
let sortAsc = true;

const SEV_ORDER: Record<Severity, number> = {
  blocker: 0, critical: 1, major: 2, minor: 3, info: 4,
};

// ── Bootstrap ────────────────────────────────────────────────────────────────

async function init(): Promise<void> {
  try {
    const res = await fetch("/report.json");
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const report: Report = await res.json();
    renderHeader(report);
    renderMeasures(report);
    allIssues = report.issues ?? [];
    populateFilters();
    applyFilters();
  } catch (e) {
    document.getElementById("app")!.innerHTML =
      `<div class="error">Failed to load report: ${String(e)}</div>`;
  }
}

document.addEventListener("DOMContentLoaded", init);

// ── Header ───────────────────────────────────────────────────────────────────

function renderHeader(r: Report): void {
  const date = new Date(r.metadata.analysis_date).toLocaleString();
  (document.getElementById("project-key")!).textContent = r.metadata.project_key;
  (document.getElementById("scan-date")!).textContent = date;
  (document.getElementById("scan-version")!).textContent = `v${r.metadata.version}`;
  (document.getElementById("elapsed")!).textContent = `${r.metadata.elapsed_ms}ms`;
}

// ── Measures ─────────────────────────────────────────────────────────────────

function renderMeasures(r: Report): void {
  const m = r.measures;
  const cards: Array<[string, string | number, string]> = [
    ["Files", m.files, "📁"],
    ["Lines", m.lines.toLocaleString(), "📄"],
    ["NCLOC", m.ncloc.toLocaleString(), "📝"],
    ["Bugs", m.bugs, "🐛"],
    ["Code Smells", m.code_smells, "🌿"],
    ["Vulnerabilities", m.vulnerabilities, "🔒"],
  ];

  const grid = document.getElementById("measures")!;
  grid.innerHTML = cards.map(([label, value, icon]) =>
    `<div class="card">
       <span class="card-icon">${icon}</span>
       <span class="card-value">${value}</span>
       <span class="card-label">${label}</span>
     </div>`
  ).join("");

  // Language breakdown
  const langs = Object.entries(m.by_language).sort((a, b) => b[1] - a[1]);
  if (langs.length) {
    document.getElementById("by-lang")!.innerHTML =
      langs.map(([l, n]) => `<span class="tag">${l}: ${n}</span>`).join(" ");
  }
}

// ── Filters ──────────────────────────────────────────────────────────────────

function populateFilters(): void {
  const rules = [...new Set(allIssues.map(i => i.rule_key))].sort();
  const sel = document.getElementById("filter-rule") as HTMLSelectElement;
  rules.forEach(r => {
    const opt = document.createElement("option");
    opt.value = r; opt.textContent = r;
    sel.appendChild(opt);
  });

  document.getElementById("filter-severity")!.addEventListener("change", e => {
    filterSeverity = (e.target as HTMLSelectElement).value;
    applyFilters();
  });
  document.getElementById("filter-type")!.addEventListener("change", e => {
    filterType = (e.target as HTMLSelectElement).value;
    applyFilters();
  });
  sel.addEventListener("change", e => {
    filterRule = (e.target as HTMLSelectElement).value;
    applyFilters();
  });
  document.getElementById("search")!.addEventListener("input", e => {
    searchText = (e.target as HTMLInputElement).value.toLowerCase();
    applyFilters();
  });
}

// ── Filter + Sort ─────────────────────────────────────────────────────────────

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
  sortIssues();
  renderTable();
}

function sortIssues(): void {
  filteredIssues.sort((a, b) => {
    let va: string | number = a[sortKey] as string | number;
    let vb: string | number = b[sortKey] as string | number;
    if (sortKey === "severity") {
      va = SEV_ORDER[a.severity as Severity] ?? 99;
      vb = SEV_ORDER[b.severity as Severity] ?? 99;
    }
    if (va < vb) return sortAsc ? -1 : 1;
    if (va > vb) return sortAsc ? 1 : -1;
    return 0;
  });
}

// ── Table ─────────────────────────────────────────────────────────────────────

const SEV_COLORS: Record<string, string> = {
  blocker: "#e53e3e", critical: "#dd6b20", major: "#d69e2e",
  minor: "#38a169", info: "#718096",
};

const TYPE_ICONS: Record<string, string> = {
  BUG: "🐛", CODE_SMELL: "🌿", VULNERABILITY: "🔒",
};

function renderTable(): void {
  const tbody = document.getElementById("issue-tbody")!;
  const count = document.getElementById("issue-count")!;
  count.textContent = `${filteredIssues.length} issue${filteredIssues.length !== 1 ? "s" : ""}`;

  if (!filteredIssues.length) {
    tbody.innerHTML = `<tr><td colspan="5" class="empty">No issues match the current filters.</td></tr>`;
    return;
  }

  tbody.innerHTML = filteredIssues.map(i => {
    const file = i.component_path.replace(/\\/g, "/").split("/").slice(-3).join("/");
    const loc = i.end_line && i.end_line !== i.line
      ? `${i.line}–${i.end_line}` : `${i.line}`;
    const color = SEV_COLORS[i.severity] ?? "#718096";
    const icon = TYPE_ICONS[i.type] ?? "❓";
    return `<tr>
      <td><span class="sev-badge" style="background:${color}">${i.severity}</span></td>
      <td>${icon} ${escHtml(i.type)}</td>
      <td class="mono">${escHtml(i.rule_key)}</td>
      <td class="file-cell" title="${escHtml(i.component_path)}">${escHtml(file)}<span class="loc">:${loc}</span></td>
      <td class="msg">${escHtml(i.message)}</td>
    </tr>`;
  }).join("");
}

// ── Sorting from column headers ───────────────────────────────────────────────

document.addEventListener("DOMContentLoaded", () => {
  document.querySelectorAll("th[data-sort]").forEach(th => {
    th.addEventListener("click", () => {
      const key = (th as HTMLElement).dataset["sort"] as keyof Issue;
      if (sortKey === key) { sortAsc = !sortAsc; } else { sortKey = key; sortAsc = true; }
      document.querySelectorAll("th[data-sort]").forEach(h =>
        (h as HTMLElement).classList.remove("sort-asc", "sort-desc"));
      (th as HTMLElement).classList.add(sortAsc ? "sort-asc" : "sort-desc");
      sortIssues();
      renderTable();
    });
  });
});

// ── Helpers ───────────────────────────────────────────────────────────────────

function escHtml(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

// suppress unused-type warnings — types are used via import
export type { IssueType };
