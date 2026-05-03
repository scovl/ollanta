import { apiFetch } from '../core/api.js';
import { buildScopedPath } from '../core/scope.js';
import { state } from '../core/state.js';
import { escHtml, fmtDate, fmtNum, fmtPct } from '../core/utils.js';

let renderView = () => {};

export function configureActivityFeature(options) {
  renderView = options.render;
}

export function renderActivityTab() {
  const payload = state.activityData;
  if (payload === null) return `<div class="loading-state"><div class="spinner"></div></div>`;

  const entries = normalizeActivityPayload(payload);
  if (!entries.length) return `<div class="empty-state" style="padding:40px 0"><p>No scan activity yet.</p></div>`;

  return `<div class="activity-board">
    ${renderActivityHeader(entries)}
    ${renderActivityTrend(entries)}
    <div class="activity-grid">
      ${renderActivityTimeline(entries)}
      ${renderActivityScanList(entries)}
    </div>
  </div>`;
}

function normalizeActivityPayload(payload) {
  if (Array.isArray(payload)) return payload;
  return payload?.items || [];
}

function trendWord(delta) {
  if (delta > 0) return 'up';
  if (delta < 0) return 'down';
  return 'flat';
}

function signedValue(value) {
  if (!value) return '0';
  return `${value > 0 ? '+' : ''}${fmtNum(value)}`;
}

function compactSha(value) {
  if (!value) return '';
  return String(value).slice(0, 8);
}

function gateBadge(status) {
  if (!status) return `<span class="badge">UNKNOWN</span>`;
  const cls = status === 'OK' ? 'badge-ok' : status === 'WARN' ? 'badge-warn' : 'badge-error';
  return `<span class="badge ${cls}">${escHtml(status)}</span>`;
}

function renderActivityHeader(entries) {
  const latest = entries[0];
  const previous = entries[1];
  const latestMeasures = latest.measures || {};
  const issueDelta = latest.delta?.issues ?? (previous ? latest.total_issues - previous.total_issues : 0);
  const gateChanges = entries.reduce((count, entry) => count + (entry.events || []).filter(event => event.category === 'QUALITY_GATE').length, 0);
  const coverage = latestMeasures.coverage;
  const mutation = latestMeasures.changed_mutation_score ?? latestMeasures.mutation_score;
  const failedTests = (latestMeasures.test_failures || 0) + (latestMeasures.test_errors || 0);

  return `<section class="activity-summary" aria-label="Activity summary">
    <div class="activity-summary-card gate-${String(latest.gate_status || 'unknown').toLowerCase()}">
      <span class="activity-card-label">Latest gate</span>
      <strong>${escHtml(latest.gate_status || 'UNKNOWN')}</strong>
      <span>${gateChanges ? `${fmtNum(gateChanges)} gate change${gateChanges === 1 ? '' : 's'} in view` : 'No gate changes in view'}</span>
    </div>
    <div class="activity-summary-card trend-${trendWord(issueDelta)}">
      <span class="activity-card-label">Issue trend</span>
      <strong>${fmtNum(latest.total_issues)}</strong>
      <span>${signedValue(issueDelta)} since previous scan</span>
    </div>
    <div class="activity-summary-card">
      <span class="activity-card-label">Security / reliability</span>
      <strong>${fmtNum(latest.total_vulnerabilities)} / ${fmtNum(latest.total_bugs)}</strong>
      <span>${fmtNum(latest.total_code_smells)} maintainability issues</span>
    </div>
    <div class="activity-summary-card">
      <span class="activity-card-label">Test signal</span>
      <strong>${coverage == null ? 'No coverage' : fmtPct(coverage)}</strong>
      <span>${mutation == null ? `${fmtNum(failedTests)} failing tests` : `${fmtPct(mutation)} mutation score`}</span>
    </div>
  </section>`;
}

function renderActivityTrend(entries) {
  const points = [...entries].reverse();
  const values = points.map(entry => Number(entry.total_issues || 0));
  const max = Math.max(...values, 1);
  const min = Math.min(...values, 0);
  const spread = Math.max(max - min, 1);
  const width = 760;
  const height = 280;
  const left = 54;
  const right = 18;
  const top = 26;
  const bottom = 42;
  const plotWidth = width - left - right;
  const plotHeight = height - top - bottom;
  const xFor = index => left + (points.length === 1 ? plotWidth / 2 : (plotWidth * index) / (points.length - 1));
  const yFor = value => top + plotHeight - ((Number(value || 0) - min) / spread) * plotHeight;
  const path = points.map((entry, index) => `${index === 0 ? 'M' : 'L'} ${xFor(index).toFixed(1)} ${yFor(entry.total_issues).toFixed(1)}`).join(' ');
  const area = `${path} L ${xFor(points.length - 1).toFixed(1)} ${top + plotHeight} L ${xFor(0).toFixed(1)} ${top + plotHeight} Z`;
  const grid = [0, 0.25, 0.5, 0.75, 1].map(step => {
    const y = top + plotHeight * step;
    const label = Math.round(max - spread * step);
    return `<g><line x1="${left}" y1="${y}" x2="${width - right}" y2="${y}" class="activity-chart-grid"/><text x="${left - 12}" y="${y + 4}" class="activity-chart-label" text-anchor="end">${fmtNum(label)}</text></g>`;
  }).join('');
  const markers = points.map((entry, index) => {
    const x = xFor(index).toFixed(1);
    const y = yFor(entry.total_issues).toFixed(1);
    const gate = entry.gate_status === 'OK' ? 'ok' : entry.gate_status === 'WARN' ? 'warn' : entry.gate_status === 'ERROR' ? 'error' : 'unknown';
    return `<circle cx="${x}" cy="${y}" r="4" class="activity-chart-point ${gate}"><title>${escHtml(new Date(entry.analysis_date).toLocaleString())}: ${fmtNum(entry.total_issues)} issues</title></circle>`;
  }).join('');
  const eventTicks = points.flatMap((entry, index) => (entry.events || []).map(event => ({ entry, index, event }))).map(item => {
    const x = xFor(item.index).toFixed(1);
    return `<line x1="${x}" y1="${top + plotHeight + 12}" x2="${x}" y2="${top + plotHeight + 28}" class="activity-event-tick"><title>${escHtml(item.event.name)}</title></line>`;
  }).join('');
  const first = points[0];
  const last = points[points.length - 1];

  return `<section class="activity-trend-panel">
    <div class="activity-panel-head">
      <div>
        <p class="section-title">Issue Timeline</p>
        <p class="muted">${fmtNum(points.length)} analyses from ${escHtml(new Date(first.analysis_date).toLocaleDateString())} to ${escHtml(new Date(last.analysis_date).toLocaleDateString())}</p>
      </div>
      <div class="activity-chart-legend">
        <span><i class="legend-line"></i>Total issues</span>
        <span><i class="legend-dot ok"></i>Passed</span>
        <span><i class="legend-dot error"></i>Failed</span>
      </div>
    </div>
    <svg class="activity-chart" viewBox="0 0 ${width} ${height}" role="img" aria-label="Project issue trend over time">
      ${grid}
      <path d="${area}" class="activity-chart-area"></path>
      <path d="${path}" class="activity-chart-line"></path>
      ${markers}
      ${eventTicks}
      <text x="${left}" y="${height - 10}" class="activity-chart-label">${escHtml(new Date(first.analysis_date).toLocaleDateString())}</text>
      <text x="${width - right}" y="${height - 10}" class="activity-chart-label" text-anchor="end">${escHtml(new Date(last.analysis_date).toLocaleDateString())}</text>
    </svg>
  </section>`;
}

function renderActivityTimeline(entries) {
  const eventRows = entries.flatMap(entry => {
    const baseEvents = entry.events?.length ? entry.events : [{ category: 'ANALYSIS', name: 'Analysis completed' }];
    return baseEvents.map(event => ({ entry, event }));
  }).slice(0, 18);

  const rows = eventRows.map(({ entry, event }, index) => {
    const isLast = index === eventRows.length - 1;
    const cls = eventClass(event);
    return `<div class="activity-entry">
      <div class="activity-dot-col">
        <div class="activity-dot ${cls}"></div>
        ${!isLast ? '<div class="activity-line"></div>' : ''}
      </div>
      <div class="activity-body">
        <div class="activity-date">${fmtDate(entry.analysis_date)} <span>${escHtml(new Date(entry.analysis_date).toLocaleString())}</span></div>
        <div class="activity-main">
          <span class="activity-version">${escHtml(event.name)}</span>
          ${gateBadge(entry.gate_status)}
        </div>
        <div class="activity-metrics">
          <span><span class="am-val">${fmtNum(entry.total_issues)}</span> issues</span>
          <span><span class="am-val dangerish">${fmtNum(entry.new_issues)}</span> new</span>
          <span><span class="am-val successish">${fmtNum(entry.closed_issues)}</span> closed</span>
          ${entry.commit_sha ? `<span class="mono">${escHtml(compactSha(entry.commit_sha))}</span>` : ''}
        </div>
      </div>
    </div>`;
  }).join('');

  return `<section class="activity-panel activity-events-panel">
    <div class="activity-panel-head"><p class="section-title">Notable Events</p></div>
    <div class="activity-timeline">${rows}</div>
  </section>`;
}

function renderActivityScanList(entries) {
  const rows = entries.slice(0, 12).map(entry => {
    const measures = entry.measures || {};
    const coverage = measures.coverage == null ? '\u2014' : fmtPct(measures.coverage);
    const mutation = measures.changed_mutation_score ?? measures.mutation_score;
    return `<tr>
      <td><strong>${fmtDate(entry.analysis_date)}</strong><span>${escHtml(new Date(entry.analysis_date).toLocaleDateString())}</span></td>
      <td>${gateBadge(entry.gate_status)}</td>
      <td class="num">${fmtNum(entry.total_issues)}<span>${signedValue(entry.delta?.issues || 0)}</span></td>
      <td class="num dangerish">${fmtNum(entry.new_issues)}</td>
      <td class="num successish">${fmtNum(entry.closed_issues)}</td>
      <td class="num">${coverage}<span>${mutation == null ? 'mutation n/a' : `${fmtPct(mutation)} mutation`}</span></td>
    </tr>`;
  }).join('');

  return `<section class="activity-panel activity-scans-panel">
    <div class="activity-panel-head"><p class="section-title">Recent Analyses</p></div>
    <div class="activity-scan-table-wrap">
      <table class="activity-scan-table">
        <thead><tr><th>Analysis</th><th>Gate</th><th>Issues</th><th>New</th><th>Closed</th><th>Tests</th></tr></thead>
        <tbody>${rows}</tbody>
      </table>
    </div>
  </section>`;
}

function eventClass(event) {
  if (event.category === 'QUALITY_GATE') return 'dot-gate';
  if (event.category === 'ISSUE_SPIKE') return 'dot-spike';
  if (event.category === 'FIRST_ANALYSIS') return 'dot-first';
  if (event.category === 'VERSION') return 'dot-version';
  return 'dot-analysis';
}

export async function loadActivityData() {
  const project = state.currentProject;
  if (!project) return;
  try {
    const data = await apiFetch(buildScopedPath('/projects/' + encodeURIComponent(project.key) + '/activity?limit=60', state.scope));
    state.activityData = data || { items: [] };
  } catch {
    state.activityData = { items: [] };
  }
  renderView();
}