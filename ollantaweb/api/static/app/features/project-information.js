import { apiFetch } from '../core/api.js';
import { buildScopedPath, normalizeScope } from '../core/scope.js';
import { state } from '../core/state.js';
import { escHtml, fmtDate, fmtK, fmtNum } from '../core/utils.js';

let renderView = () => {};

export function configureProjectInformationFeature(options) {
  renderView = options.render;
}

export async function loadProjectInfoData() {
  const key = state.currentProject?.key;
  if (!key) return;
  try {
    state.projectInfoData = await apiFetch(buildScopedPath('/projects/' + encodeURIComponent(key) + '/information', state.scope));
  } catch {
    state.projectInfoData = null;
  }
  renderView();
}

export function renderProjectInformationTab() {
  const data = state.projectInfoData;
  if (data === null) {
    return `<div class="loading-state"><div class="spinner"></div></div>`;
  }

  const project = data.project || state.currentProject || {};
  const scope = normalizeScope(data.scope || state.scope);
  const snapshot = data.code_snapshot || null;
  const measures = data.measures || {};

  return `<div class="info-grid">
    <section class="info-card">
      <h3>Project</h3>
      <dl class="info-list">
        <div><dt>Name</dt><dd>${escHtml(project.name || project.key || '\u2014')}</dd></div>
        <div><dt>Key</dt><dd class="mono">${escHtml(project.key || '\u2014')}</dd></div>
        <div><dt>Main branch</dt><dd>${escHtml(project.main_branch || scope.defaultBranch || '\u2014')}</dd></div>
        <div><dt>Visibility</dt><dd>${escHtml(project.visibility || '\u2014')}</dd></div>
        <div><dt>Version</dt><dd>${escHtml(state.currentScan?.version || '\u2014')}</dd></div>
        <div><dt>Updated</dt><dd>${project.updated_at ? fmtDate(project.updated_at) : '\u2014'}</dd></div>
      </dl>
    </section>
    <section class="info-card">
      <h3>Current scope</h3>
      <dl class="info-list">
        <div><dt>Type</dt><dd>${escHtml(scope.type || '\u2014')}</dd></div>
        <div><dt>Branch</dt><dd>${escHtml(scope.branch || scope.defaultBranch || '\u2014')}</dd></div>
        <div><dt>Pull request</dt><dd>${escHtml(scope.pullRequestKey || '\u2014')}</dd></div>
        <div><dt>Base branch</dt><dd>${escHtml(scope.pullRequestBase || '\u2014')}</dd></div>
        <div><dt>Commit</dt><dd class="mono">${escHtml(state.currentScan?.commit_sha || '\u2014')}</dd></div>
        <div><dt>Last analysis</dt><dd>${state.currentScan?.analysis_date ? fmtDate(state.currentScan.analysis_date) : '\u2014'}</dd></div>
      </dl>
    </section>
    <section class="info-card">
      <h3>Measures</h3>
      <dl class="info-list">
        <div><dt>Files</dt><dd>${fmtNum(measures.files || 0)}</dd></div>
        <div><dt>Lines</dt><dd>${fmtNum(measures.lines || 0)}</dd></div>
        <div><dt>NCLOC</dt><dd>${fmtNum(measures.ncloc || 0)}</dd></div>
        <div><dt>Issues</dt><dd>${fmtNum(measures.issues || 0)}</dd></div>
      </dl>
    </section>
    <section class="info-card">
      <h3>Code snapshot</h3>
      <dl class="info-list">
        <div><dt>Stored files</dt><dd>${fmtNum(snapshot?.stored_files || 0)} / ${fmtNum(snapshot?.total_files || 0)}</dd></div>
        <div><dt>Truncated files</dt><dd>${fmtNum(snapshot?.truncated_files || 0)}</dd></div>
        <div><dt>Omitted files</dt><dd>${fmtNum(snapshot?.omitted_files || 0)}</dd></div>
        <div><dt>Stored bytes</dt><dd>${fmtK(snapshot?.stored_bytes || 0)}</dd></div>
        <div><dt>Max file bytes</dt><dd>${fmtK(snapshot?.max_file_bytes || 0)}</dd></div>
        <div><dt>Updated</dt><dd>${snapshot?.updated_at ? fmtDate(snapshot.updated_at) : '\u2014'}</dd></div>
      </dl>
    </section>
  </div>`;
}