import { apiFetch } from '../core/api.js';
import { buildScopedPath } from '../core/scope.js';
import { state } from '../core/state.js';
import { escHtml, fmtNum } from '../core/utils.js';

let renderView = () => {};

export function configureActivityFeature(options) {
  renderView = options.render;
}

export function renderActivityTab() {
  const data = state.activityData;
  if (data === null) return `<div class="loading-state"><div class="spinner"></div></div>`;
  if (!data || !data.length) return `<div class="empty-state" style="padding:40px 0"><p>No scan activity yet.</p></div>`;

  const entries = data.map((entry, idx) => {
    const isLast = idx === data.length - 1;
    const dotCls = entry.gate_status === 'OK' ? 'dot-ok' : entry.gate_status === 'ERROR' ? 'dot-error' : entry.gate_status === 'WARN' ? 'dot-warn' : '';

    const eventsHtml = (entry.events || []).map(event => {
      let cls = 'ev-version';
      if (event.category === 'QUALITY_GATE') cls = event.value === 'OK' ? 'ev-gate' : 'ev-gate-fail';
      if (event.category === 'ISSUE_SPIKE') cls = 'ev-spike';
      if (event.category === 'FIRST_ANALYSIS') cls = 'ev-first';
      return `<span class="activity-event ${cls}">${escHtml(event.name)}</span>`;
    }).join('');

    return `<div class="activity-entry">
      <div class="activity-dot-col">
        <div class="activity-dot ${dotCls}"></div>
        ${!isLast ? '<div class="activity-line"></div>' : ''}
      </div>
      <div class="activity-body">
        <div class="activity-date">${entry.analysis_date ? new Date(entry.analysis_date).toLocaleString() : '\u2014'}</div>
        <div class="activity-main">
          <span class="activity-version">${escHtml(entry.version || 'No version')}</span>
          ${entry.branch ? `<span class="activity-branch">${escHtml(entry.branch)}</span>` : ''}
          ${entry.gate_status ? `<span class="badge ${entry.gate_status === 'OK' ? 'badge-ok' : entry.gate_status === 'WARN' ? 'badge-warn' : 'badge-error'}">${escHtml(entry.gate_status)}</span>` : ''}
        </div>
        <div class="activity-metrics">
          <span><span class="am-val">${fmtNum(entry.total_issues)}</span> issues</span>
          <span><span class="am-val" style="color:${entry.new_issues > 0 ? 'var(--warning)' : 'var(--text)'}">${fmtNum(entry.new_issues)}</span> new</span>
          <span><span class="am-val" style="color:var(--success)">${fmtNum(entry.closed_issues)}</span> closed</span>
        </div>
        ${eventsHtml ? `<div class="activity-events">${eventsHtml}</div>` : ''}
      </div>
    </div>`;
  }).join('');

  return `<div class="activity-timeline">${entries}</div>`;
}

export async function loadActivityData() {
  const project = state.currentProject;
  if (!project) return;
  try {
    const data = await apiFetch(buildScopedPath('/projects/' + encodeURIComponent(project.key) + '/activity?limit=30', state.scope));
    state.activityData = data.items || [];
  } catch {
    state.activityData = [];
  }
  renderView();
}