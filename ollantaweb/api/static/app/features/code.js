import { apiFetch } from '../core/api.js';
import { buildScopedPath } from '../core/scope.js';
import { state } from '../core/state.js';
import { escAttr, escHtml, fmtK, fmtNum } from '../core/utils.js';

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
    if ((!state.codeSelectedPath || !items.some(item => item.path === state.codeSelectedPath)) && items.length > 0) {
      state.codeSelectedPath = items[0].path;
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

function renderCodeViewer() {
  const payload = state.codeFileData;
  if (!payload?.file) {
    return `<div class="code-empty"><p>Select a file to inspect the latest code snapshot.</p></div>`;
  }

  const file = payload.file;
  const issues = payload.issues || [];
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
    return `<div class="code-line${lineIssues.length ? ' has-issue' : ''}">
      <span class="code-gutter">${lineNumber}</span>
      <code class="code-text">${line.length ? escHtml(line) : '&nbsp;'}</code>
      <span class="code-markers">${renderCodeIssueChips(lineIssues)}</span>
    </div>`;
  }).join('');

  const notices = [];
  if (file.is_truncated) notices.push('This file was truncated in the snapshot payload.');
  if (file.is_omitted) notices.push(file.omitted_reason || 'This file content was omitted from the snapshot.');

  return `<div class="code-viewer-shell">
    <div class="code-viewer-head">
      <div>
        <div class="code-viewer-path mono">${escHtml(file.path)}</div>
        <div class="code-viewer-meta">${escHtml(file.language || 'plain text')} · ${fmtNum(file.line_count || lines.length)} lines · ${fmtK(file.size_bytes || 0)}</div>
      </div>
      <div class="code-viewer-stats">${fmtNum(issues.length)} issue${issues.length === 1 ? '' : 's'}</div>
    </div>
    ${notices.map(message => `<div class="code-notice">${escHtml(message)}</div>`).join('')}
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

export function renderCodeTab() {
  const data = state.codeTreeData;
  if (data === null) {
    return `<div class="loading-state"><div class="spinner"></div></div>`;
  }
  const items = data.items || [];
  const snapshot = data.code_snapshot || null;
  if (items.length === 0) {
    return `<div class="empty-state">
      <div class="empty-icon">{ }</div>
      <p>No code snapshot available for the current scope.</p>
      <p class="empty-detail">Run a new analysis on this ${state.scope.type === 'pull_request' ? 'pull request' : 'branch'} to populate code browsing.</p>
    </div>`;
  }

  return `<div class="code-browser">
    <aside class="code-sidebar">
      <div class="code-sidebar-head">
        <h3>Files</h3>
        <p>${fmtNum(snapshot?.stored_files || items.length)} stored · ${fmtNum(snapshot?.total_files || items.length)} discovered</p>
      </div>
      <div class="code-tree">
        ${items.map(item => `<button class="code-tree-item${item.path === state.codeSelectedPath ? ' active' : ''}${item.is_omitted ? ' muted' : ''}" data-code-path="${escAttr(item.path)}">
          <span class="code-tree-path mono">${escHtml(item.path)}</span>
          <span class="code-tree-meta">${fmtNum(item.line_count || 0)} lines${item.is_truncated ? ' · truncated' : ''}${item.is_omitted ? ' · omitted' : ''}</span>
        </button>`).join('')}
      </div>
    </aside>
    <section class="code-viewer">${renderCodeViewer()}</section>
  </div>`;
}