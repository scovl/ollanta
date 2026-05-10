import { apiFetch } from '../core/api.js';
import { buildScopedPath } from '../core/scope.js';
import { state } from '../core/state.js';
import { escAttr, escHtml, fmtK, fmtNum, fmtPct } from '../core/utils.js';

let renderView = () => {};

export function configureCodeFeature(options) {
  renderView = options.render;
}

export async function loadCodeTreeData() {
  const key = state.currentProject?.key;
  if (!key) return;
  try {
    state.codeTreeData = await apiFetch(buildScopedPath('/projects/' + encodeURIComponent(key) + '/code/tree', state.scope));
    const items = state.codeTreeData?.items || [];
    const files = coverageFiles();
    if (!state.codeSelectedPath || !files.some(file => file.component_path === state.codeSelectedPath)) {
      state.codeSelectedPath = files[0]?.component_path || items[0]?.path || '';
    }
    if (state.codeSelectedPath) {
      await loadCodeFileData(state.codeSelectedPath);
      return;
    }
    state.codeFileData = null;
  } catch {
    state.codeTreeData = { items: [] };
    state.codeFileData = null;
  }
  renderView();
}

export async function loadCodeFileData(path) {
  const key = state.currentProject?.key;
  if (!key || !path) return;
  state.codeSelectedPath = path;
  try {
    state.codeFileData = await apiFetch(buildScopedPath('/projects/' + encodeURIComponent(key) + '/code/file?path=' + encodeURIComponent(path), state.scope));
  } catch {
    state.codeFileData = null;
  }
  renderView();
}

function renderCodeIssueChips(issues) {
  return issues.map(issue => {
    const sev = issue.severity || 'info';
    return `<span class="code-line-chip sev-${escAttr(sev)}">${escHtml(sev)}</span>`;
  }).join('');
}

function coverageClass(coverage) {
  if (coverage === null || coverage === undefined) return 'card-neutral';
  if (coverage >= 80) return 'card-green';
  if (coverage >= 60) return 'card-yellow';
  return 'card-red';
}

function coverageByPath() {
  const files = state.overviewData?.summary?.coverage?.files || [];
  return new Map(files.map(file => [file.component_path, file]));
}

function coverageFiles() {
  return state.overviewData?.summary?.coverage?.files || [];
}

function renderCoverageLineChip(isCovered, isUncovered) {
  if (isUncovered) return '<span class="coverage-line-chip chip-uncovered">not covered</span>';
  if (isCovered) return '<span class="coverage-line-chip chip-covered">covered</span>';
  return '';
}

function coverageLineClass(isCovered, isUncovered) {
  if (isUncovered) return ' is-uncovered';
  if (isCovered) return ' is-covered';
  return '';
}

function renderCodeViewer() {
  const payload = state.codeFileData;
  if (!payload?.file) {
    return `<div class="code-empty"><p>Select a coverage file to inspect source lines.</p></div>`;
  }

  const file = payload.file;
  const issues = payload.issues || [];
  const coverage = coverageByPath().get(file.path);
  const coveredLines = new Set(coverage?.covered_line_numbers || []);
  const uncoveredLines = new Set(coverage?.uncovered_line_numbers || []);
  const hasLineCoverage = coveredLines.size > 0 || uncoveredLines.size > 0;
  const lines = (file.content || '').split('\n');
  const issueMap = new Map();
  issues.forEach(issue => {
    const start = Number(issue.line || 1);
    const end = Number(issue.end_line || start);
    for (let line = start; line <= end; line += 1) {
      const items = issueMap.get(line) || [];
      items.push(issue);
      issueMap.set(line, items);
    }
  });

  const rows = lines.map((line, index) => {
    const lineNumber = index + 1;
    const lineIssues = issueMap.get(lineNumber) || [];
    const isUncovered = uncoveredLines.has(lineNumber);
    const isCovered = !isUncovered && coveredLines.has(lineNumber);
    const coverageChip = renderCoverageLineChip(isCovered, isUncovered);
    return `<div class="code-line${lineIssues.length ? ' has-issue' : ''}${coverageLineClass(isCovered, isUncovered)}">
      <span class="code-gutter">${lineNumber}</span>
      <code class="code-text">${line.length ? escHtml(line) : '&nbsp;'}</code>
      <span class="code-markers">${coverageChip}${renderCodeIssueChips(lineIssues)}</span>
    </div>`;
  }).join('');

  const notices = [];
  if (file.is_truncated) notices.push('This file was truncated in the snapshot payload.');
  if (file.is_omitted) notices.push(file.omitted_reason || 'This file content was omitted from the snapshot.');

  const detailState = hasLineCoverage ? 'covered and uncovered lines' : 'line totals only';
  const coverageStats = coverage ? `<span class="coverage-pill ${coverageClass(coverage.coverage)}">${fmtPct(coverage.coverage)}</span>` : '';
  const coverageLegend = hasLineCoverage
    ? `<div class="coverage-legend">
      <span><span class="legend-dot legend-covered"></span>Covered</span>
      <span><span class="legend-dot legend-uncovered"></span>Not covered</span>
    </div>`
    : '';

  return `<div class="code-viewer-shell">
    <div class="code-viewer-head">
      <div>
        <div class="code-viewer-path mono">${escHtml(file.path)}</div>
        <div class="code-viewer-meta">${escHtml(file.language || 'plain text')} · ${fmtNum(file.line_count || lines.length)} lines · ${fmtK(file.size_bytes || 0)} · ${escHtml(detailState)}</div>
      </div>
      <div class="code-viewer-stats">${coverageStats}${fmtNum(issues.length)} issue${issues.length === 1 ? '' : 's'}</div>
    </div>
    ${notices.map(message => `<div class="code-notice">${escHtml(message)}</div>`).join('')}
    ${coverageLegend}
    <div class="code-surface">${rows}</div>
    ${issues.length ? `<div class="code-issue-list">
      ${issues.map((issue, index) => `<button class="code-issue-item" data-code-issue="${index}">
        <span class="code-issue-title">${escHtml(issue.rule_key || 'issue')}</span>
        <span class="code-issue-meta">${escHtml(issue.severity || 'info')} · line ${fmtNum(issue.line || 1)}</span>
        <span class="code-issue-message">${escHtml(issue.message || '')}</span>
      </button>`).join('')}
    </div>` : ''}
  </div>`;
}

function renderCoverageDetailRow(file) {
  const active = file.component_path === state.codeSelectedPath ? ' active' : '';
  const lineMeta = file.lines_to_cover ? `${fmtNum(file.covered_lines || 0)}/${fmtNum(file.lines_to_cover)} lines covered` : 'coverage report lines';
  const lineDetails = file.covered_line_numbers?.length || file.uncovered_line_numbers?.length ? `${fmtNum(file.uncovered_line_numbers?.length || 0)} uncovered lines` : 'totals only';
  return `<button class="coverage-row${active}" data-coverage-source-path="${escAttr(file.component_path || '')}">
    <div class="coverage-row-main">
      <div class="coverage-row-title" title="${escAttr(file.component_path || '')}">${escHtml(file.component_path || '')}</div>
      <div class="coverage-row-subtitle">${escHtml(lineMeta)} · ${escHtml(lineDetails)}</div>
    </div>
    <div class="coverage-row-meter">
      <span class="coverage-pill ${coverageClass(file.coverage)}">${fmtPct(file.coverage)}</span>
      <div class="coverage-track"><div class="coverage-fill ${coverageClass(file.coverage)}" style="width:${Math.max(0, Math.min(100, file.coverage ?? 0))}%"></div></div>
    </div>
  </button>`;
}

export function renderCoverageTab() {
  const data = state.codeTreeData;
  const coverage = state.overviewData?.summary?.coverage;
  const files = coverageFiles();
  if (!coverage?.files?.length) {
    return `<div class="empty-state">
      <div class="empty-icon">%</div>
      <p>No file-level coverage was collected for this scope.</p>
      <p class="empty-detail">Run a new scan with coverage reports enabled to populate this view.</p>
    </div>`;
  }
  if (data === null) {
    return `<div class="loading-state"><div class="spinner"></div></div>`;
  }

  return `<div class="coverage-toolbar">
      <div>
        <h3>Coverage Files</h3>
        <p>${fmtNum(files.length)} files with executable lines from collected test reports</p>
      </div>
      <span class="coverage-pill ${coverageClass(coverage.coverage)}">${fmtPct(coverage.coverage)}</span>
    </div>
    <div class="coverage-browser">
      <aside class="coverage-sidebar">
        <div class="coverage-file-list coverage-file-list-large">
          ${files.map(renderCoverageDetailRow).join('')}
        </div>
      </aside>
      <section class="coverage-code-viewer">${renderCodeViewer()}</section>
    </div>`;
}
