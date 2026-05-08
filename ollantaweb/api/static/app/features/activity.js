import { apiFetch } from '../core/api.js';
import { buildScopedPath } from '../core/scope.js';
import { state } from '../core/state.js';
import { escAttr, escHtml, fmtDate, fmtNum, fmtPct } from '../core/utils.js';

const ACTIVITY_PAGE_SIZE = 60;

const EVENT_CATEGORIES = [
  { id: 'QUALITY_GATE', label: 'Quality Gate' },
  { id: 'ISSUE_SPIKE', label: 'Issue Spike' },
  { id: 'VERSION', label: 'Version' },
  { id: 'FIRST_ANALYSIS', label: 'First Analysis' },
  { id: 'ANALYSIS', label: 'Analysis' },
];

const TIME_WINDOWS = [
  { id: '7d', label: '7d', days: 7 },
  { id: '30d', label: '30d', days: 30 },
  { id: '90d', label: '90d', days: 90 },
  { id: 'all', label: 'All', days: null },
];

let renderView = () => {};

export function configureActivityFeature(options) {
  renderView = options.render;
}

export function pickTestSignal(measures) {
  const m = measures || {};
  const failed = (Number(m.test_failures) || 0) + (Number(m.test_errors) || 0);
  if (failed > 0) {
    return { kind: 'failing', value: failed, label: 'failing tests', display: fmtNum(failed) };
  }
  const mutation = m.changed_mutation_score ?? m.mutation_score;
  if (mutation != null) {
    return { kind: 'mutation', value: mutation, label: 'mutation score', display: fmtPct(mutation) };
  }
  if (m.coverage != null) {
    return { kind: 'coverage', value: m.coverage, label: 'coverage', display: fmtPct(m.coverage) };
  }
  return { kind: 'none', value: null, label: 'No test signal recorded', display: 'No data' };
}

export function applyActivityFilters(items, filters) {
  if (!Array.isArray(items) || !items.length) return [];
  const cats = (filters?.categories || []).filter(Boolean);
  const win = filters?.window || 'all';
  const cutoff = computeWindowCutoff(win);
  return items.filter(entry => {
    if (cutoff !== null) {
      const ts = Date.parse(entry.analysis_date);
      if (Number.isFinite(ts) && ts < cutoff) return false;
    }
    if (cats.length === 0) return true;
    const events = entry.events || [];
    if (events.some(ev => cats.includes(ev.category))) return true;
    return cats.includes('ANALYSIS') && events.length === 0;
  });
}

function computeWindowCutoff(window) {
  const win = TIME_WINDOWS.find(w => w.id === window);
  if (!win || win.days == null) return null;
  return Date.now() - win.days * 24 * 60 * 60 * 1000;
}

export function renderActivityTab() {
  const payload = state.activityData;
  if (payload === null) return `<div class="loading-state"><div class="spinner"></div></div>`;

  const allEntries = normalizeActivityPayload(payload);
  if (!allEntries.length) return `<div class="empty-state" style="padding:40px 0"><p>No scan activity yet.</p></div>`;

  const filters = state.activityFilters || { categories: [], window: 'all' };
  const filtered = applyActivityFilters(allEntries, filters);
  const visible = filtered.length ? filtered : allEntries;

  return `<div class="activity-board">
    ${renderActivityHeader(allEntries)}
    ${renderActivityFilters(filters, filtered.length, allEntries.length)}
    ${renderActivityTrend(visible, filtered.length, allEntries.length)}
    <div class="activity-grid">
      ${renderActivityTimeline(visible)}
      ${renderActivityScanList(visible)}
    </div>
    ${renderActivityCompare(allEntries)}
    ${renderActivityLoadMore()}
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
  const cls = gateStatusClass(status);
  return `<span class="badge ${cls}">${escHtml(status)}</span>`;
}

function gateStatusClass(status) {
  if (status === 'OK') return 'badge-ok';
  if (status === 'WARN') return 'badge-warn';
  return 'badge-error';
}

function gateChangeSummary(count) {
  if (!count) return 'No gate changes in view';
  const suffix = count === 1 ? '' : 's';
  return `${fmtNum(count)} gate change${suffix} in view`;
}

function renderActivityHeader(entries) {
  const latest = entries[0];
  const previous = entries[1];
  const latestMeasures = latest.measures || {};
  const issueDelta = latest.delta?.issues ?? (previous ? latest.total_issues - previous.total_issues : 0);
  const gateChanges = entries.reduce((count, entry) => count + (entry.events || []).filter(event => event.category === 'QUALITY_GATE').length, 0);
  const signal = pickTestSignal(latestMeasures);

  return `<section class="activity-summary" aria-label="Activity summary">
    <div class="activity-summary-card gate-${String(latest.gate_status || 'unknown').toLowerCase()}">
      <span class="activity-card-label">Latest gate</span>
      <strong>${escHtml(latest.gate_status || 'UNKNOWN')}</strong>
      <span>${gateChangeSummary(gateChanges)}</span>
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
    <div class="activity-summary-card test-signal-${signal.kind}">
      <span class="activity-card-label">Test signal</span>
      <strong>${escHtml(signal.display)}</strong>
      <span>${escHtml(signal.label)}</span>
    </div>
  </section>`;
}

function renderActivityFilters(filters, filteredCount, totalCount) {
  const cats = filters.categories || [];
  const win = filters.window || 'all';
  const categoryButtons = EVENT_CATEGORIES.map(cat => {
    const active = cats.includes(cat.id);
    return `<button type="button" class="activity-filter-chip${active ? ' active' : ''}" data-activity-cat="${cat.id}" aria-pressed="${active ? 'true' : 'false'}">${escHtml(cat.label)}</button>`;
  }).join('');
  const windowButtons = TIME_WINDOWS.map(w => {
    const active = w.id === win;
    return `<button type="button" class="activity-window-btn${active ? ' active' : ''}" data-activity-window="${w.id}" aria-pressed="${active ? 'true' : 'false'}">${escHtml(w.label)}</button>`;
  }).join('');
  const summary = cats.length || win !== 'all'
    ? `${fmtNum(filteredCount)} of ${fmtNum(totalCount)} in view`
    : `${fmtNum(totalCount)} loaded`;
  const exportBtn = `<button type="button" class="btn-sm btn-outline" id="activityExportBtn">Export CSV</button>`;
  return `<section class="activity-filters" aria-label="Activity filters">
    <div class="activity-filter-group">
      <span class="activity-filter-label">Events</span>
      <div class="activity-filter-chips">${categoryButtons}</div>
      ${cats.length ? `<button type="button" class="btn-sm btn-ghost" id="activityClearCategoriesBtn">Clear</button>` : ''}
    </div>
    <div class="activity-filter-group">
      <span class="activity-filter-label">Window</span>
      <div class="activity-window-group">${windowButtons}</div>
    </div>
    <div class="activity-filter-group" style="margin-left:auto;gap:12px">
      ${exportBtn}
      <span class="activity-filter-summary">${summary}</span>
    </div>
  </section>`;
}

function computeNiceTicks(min, max, targetCount) {
  const span = max - min;
  if (span <= 0) return { min, max, step: 1, values: [min] };
  const rough = span / targetCount;
  const pow10 = Math.pow(10, Math.floor(Math.log10(rough)));
  const d = rough / pow10;
  let step;
  if (d <= 1) step = pow10;
  else if (d <= 2) step = 2 * pow10;
  else if (d <= 5) step = 5 * pow10;
  else step = 10 * pow10;
  const niceMin = Math.floor(min / step) * step;
  const niceMax = Math.ceil(max / step) * step;
  const values = [];
  for (let v = niceMin; v <= niceMax + step * 0.0001; v += step) {
    values.push(Math.round(v * 1000000) / 1000000);
  }
  return { min: niceMin, max: niceMax, step, values };
}

function buildXAxisLabels(firstTs, lastTs, timeRange, left, plotWidth, width, right, height) {
  const labelCount = 5;
  let labels = '';
  for (let i = 0; i < labelCount; i++) {
    const t = firstTs + (timeRange * i) / (labelCount - 1);
    const x = left + (plotWidth * i) / (labelCount - 1);
    const anchor = i === 0 ? 'start' : i === labelCount - 1 ? 'end' : 'middle';
    const date = new Date(t);
    const text = timeRange < 2 * 24 * 60 * 60 * 1000
      ? date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
      : timeRange < 90 * 24 * 60 * 60 * 1000
      ? date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
      : date.toLocaleDateString(undefined, { year: 'numeric', month: 'short' });
    labels += `<text x="${x.toFixed(1)}" y="${height - 10}" class="activity-chart-label" text-anchor="${anchor}">${escHtml(text)}</text>`;
  }
  return labels;
}

function buildChartTooltipText(scanId) {
  const items = state.activityData?.items || [];
  const entry = items.find(e => String(e.scan_id) === String(scanId));
  if (!entry) return '';
  const measures = entry.measures || {};
  const parts = [];
  parts.push(new Date(entry.analysis_date).toLocaleString());
  parts.push(`Issues: ${fmtNum(entry.total_issues)}`);
  if (entry.gate_status) parts.push(`Gate: ${entry.gate_status}`);
  if (entry.version) parts.push(`Version: ${entry.version}`);
  if (measures.coverage != null) parts.push(`Coverage: ${fmtPct(measures.coverage)}`);
  const mutation = measures.changed_mutation_score ?? measures.mutation_score;
  if (mutation != null) parts.push(`Mutation: ${fmtPct(mutation)}`);
  return parts.join(' \u00b7 ');
}

function renderActivityTrend(entries, filteredCount, totalCount) {
  const points = [...entries].reverse();
  if (!points.length) return '';

  const values = points.map(entry => Number(entry.total_issues || 0));
  const rawMax = Math.max(...values, 1);
  const rawMin = Math.min(...values, 0);
  const yTicks = computeNiceTicks(rawMin, rawMax, 5);
  const max = yTicks.max;
  const min = yTicks.min;
  const spread = Math.max(max - min, 1);

  const width = 760;
  const height = 280;
  const left = 54;
  const right = 18;
  const top = 26;
  const bottom = 42;
  const plotWidth = width - left - right;
  const plotHeight = height - top - bottom;

  const firstTs = Date.parse(points[0].analysis_date);
  const lastTs = Date.parse(points[points.length - 1].analysis_date);
  const timeRange = Math.max(lastTs - firstTs, 1);

  const xFor = (entry, _index) => {
    const ts = Date.parse(entry.analysis_date);
    if (!Number.isFinite(ts)) return left + plotWidth / 2;
    if (timeRange <= 1 || points.length === 1) return left + plotWidth / 2;
    return left + ((ts - firstTs) / timeRange) * plotWidth;
  };

  const yFor = value => top + plotHeight - ((Number(value || 0) - min) / spread) * plotHeight;

  const path = points.map((entry, index) => `${index === 0 ? 'M' : 'L'} ${xFor(entry, index).toFixed(1)} ${yFor(entry.total_issues).toFixed(1)}`).join(' ');
  const area = `${path} L ${xFor(points[points.length - 1], points.length - 1).toFixed(1)} ${top + plotHeight} L ${xFor(points[0], 0).toFixed(1)} ${top + plotHeight} Z`;

  const grid = yTicks.values.map(val => {
    const y = yFor(val);
    return `<g><line x1="${left}" y1="${y}" x2="${width - right}" y2="${y}" class="activity-chart-grid"/><text x="${left - 12}" y="${y + 4}" class="activity-chart-label" text-anchor="end">${fmtNum(val)}</text></g>`;
  }).join('');

  const markers = points.map((entry, index) => {
    const x = xFor(entry, index).toFixed(1);
    const y = yFor(entry.total_issues).toFixed(1);
    const gate = chartGateClass(entry.gate_status);
    const tooltip = buildChartTooltipText(entry.scan_id);
    return `<circle cx="${x}" cy="${y}" r="5" class="activity-chart-point ${gate}" data-activity-scan="${escAttr(String(entry.scan_id || ''))}" tabindex="0" role="button" aria-label="Open issues for analysis ${escAttr(new Date(entry.analysis_date).toLocaleString())}"><title>${escHtml(tooltip)}</title></circle>`;
  }).join('');

  const eventTicks = points.flatMap((entry, index) => (entry.events || []).map(event => ({ entry, index, event }))).map(item => {
    const x = xFor(item.entry, item.index).toFixed(1);
    return `<line x1="${x}" y1="${top + plotHeight + 12}" x2="${x}" y2="${top + plotHeight + 28}" class="activity-event-tick"><title>${escHtml(item.event.name)}</title></line>`;
  }).join('');

  const xLabels = buildXAxisLabels(firstTs, lastTs, timeRange, left, plotWidth, width, right, height);

  const first = points[0];
  const last = points[points.length - 1];
  const subtitle = filteredCount && filteredCount !== totalCount
    ? `${fmtNum(points.length)} of ${fmtNum(totalCount)} analyses (filtered) from ${escHtml(new Date(first.analysis_date).toLocaleDateString())} to ${escHtml(new Date(last.analysis_date).toLocaleDateString())}`
    : `${fmtNum(points.length)} analyses from ${escHtml(new Date(first.analysis_date).toLocaleDateString())} to ${escHtml(new Date(last.analysis_date).toLocaleDateString())}`;

  return `<section class="activity-trend-panel">
    <div class="activity-panel-head">
      <div>
        <p class="section-title">Issue Timeline</p>
        <p class="muted">${subtitle}</p>
      </div>
      <div class="activity-chart-legend">
        <span><i class="legend-line"></i>Total issues</span>
        <span><i class="activity-legend-dot ok"></i>Passed</span>
        <span><i class="activity-legend-dot error"></i>Failed</span>
      </div>
    </div>
    <svg class="activity-chart" viewBox="0 0 ${width} ${height}" role="img" aria-label="Project issue trend over time">
      ${grid}
      <path d="${area}" class="activity-chart-area"></path>
      <path d="${path}" class="activity-chart-line"></path>
      ${markers}
      ${eventTicks}
      ${xLabels}
    </svg>
  </section>`;
}

function chartGateClass(status) {
  if (status === 'OK') return 'ok';
  if (status === 'WARN') return 'warn';
  if (status === 'ERROR') return 'error';
  return 'unknown';
}

function renderActivityTimeline(entries) {
  const eventRows = entries.flatMap(entry => {
    const baseEvents = entry.events?.length ? entry.events : [{ category: 'ANALYSIS', name: 'Analysis completed' }];
    return baseEvents.map(event => ({ entry, event }));
  });

  const rows = eventRows.map(({ entry, event }, index) => {
    const isLast = index === eventRows.length - 1;
    const cls = eventClass(event);
    const hadIssues = (entry.total_issues || 0) > 0;
    const isClickable = entry.scan_id != null && hadIssues;
    const titleAttr = isClickable
      ? ` title="View issues from this analysis"`
      : hadIssues
        ? ''
        : ` title="No issues in this analysis"`;
    return `<div class="activity-entry${isClickable ? ' clickable' : ''}"${isClickable ? ` data-activity-scan="${escAttr(String(entry.scan_id))}" tabindex="0" role="button"${titleAttr}` : titleAttr}>
      <div class="activity-dot-col">
        <div class="activity-dot ${cls}"></div>
        ${isLast ? '' : '<div class="activity-line"></div>'}
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
          ${entry.commit_sha ? `<span class="mono" title="${escAttr(entry.commit_sha)}">${escHtml(compactSha(entry.commit_sha))}</span>` : ''}
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
  const selection = state.activityCompareSelection || [];
  const rows = entries.map(entry => {
    const measures = entry.measures || {};
    const coverage = measures.coverage == null ? '\u2014' : fmtPct(measures.coverage);
    const mutation = measures.changed_mutation_score ?? measures.mutation_score;
    const checked = selection.includes(entry.scan_id) ? ' checked' : '';
    const selectedClass = selection.includes(entry.scan_id) ? ' selected' : '';
    const branch = entry.branch || '';
    const hasIssues = (entry.total_issues || 0) > 0;
    const linkCls = hasIssues ? 'activity-scan-link clickable' : 'activity-scan-link';
    const linkTitle = hasIssues ? 'View issues from this analysis' : 'No issues in this analysis';
    return `<tr data-activity-scan="${escAttr(String(entry.scan_id || ''))}" data-activity-branch="${escAttr(branch)}" class="activity-scan-row${selectedClass}">
      <td class="activity-scan-pick"><input type="checkbox" class="activity-compare-pick" data-activity-scan="${escAttr(String(entry.scan_id || ''))}" data-activity-branch="${escAttr(branch)}"${checked} aria-label="Compare scan"></td>
      <td class="${linkCls}" title="${escAttr(linkTitle)}"><strong>${fmtDate(entry.analysis_date)}</strong><span>${escHtml(new Date(entry.analysis_date).toLocaleDateString())}</span></td>
      <td>${gateBadge(entry.gate_status)}</td>
      <td class="num">${fmtNum(entry.total_issues)}<span>${signedValue(entry.delta?.issues || 0)}</span></td>
      <td class="num dangerish">${fmtNum(entry.new_issues)}</td>
      <td class="num successish">${fmtNum(entry.closed_issues)}</td>
      <td class="num">${coverage}<span>${mutation == null ? 'mutation n/a' : `${fmtPct(mutation)} mutation`}</span></td>
    </tr>`;
  }).join('');

  return `<section class="activity-panel activity-scans-panel">
    <div class="activity-panel-head">
      <p class="section-title">Recent Analyses</p>
      <p class="muted" style="margin:0;font-size:12px">Tip: tick two rows to compare</p>
    </div>
    <div class="activity-scan-table-wrap">
      <table class="activity-scan-table">
        <thead><tr><th aria-label="Compare"></th><th>Analysis</th><th>Gate</th><th>Issues</th><th>New</th><th>Closed</th><th>Tests</th></tr></thead>
        <tbody>${rows}</tbody>
      </table>
    </div>
  </section>`;
}

function renderActivityCompare(allEntries) {
  const selection = state.activityCompareSelection || [];
  if (selection.length !== 2) return '';
  const a = allEntries.find(e => e.scan_id === selection[0]);
  const b = allEntries.find(e => e.scan_id === selection[1]);
  if (!a || !b) return '';
  const [left, right] = Date.parse(a.analysis_date) <= Date.parse(b.analysis_date) ? [a, b] : [b, a];
  const fields = [
    { key: 'total_issues', label: 'Issues' },
    { key: 'new_issues', label: 'New' },
    { key: 'closed_issues', label: 'Closed' },
    { key: 'total_bugs', label: 'Bugs' },
    { key: 'total_vulnerabilities', label: 'Vulnerabilities' },
    { key: 'total_code_smells', label: 'Code smells' },
  ];
  const rows = fields.map(field => {
    const lv = Number(left[field.key] || 0);
    const rv = Number(right[field.key] || 0);
    const delta = rv - lv;
    const cls = delta > 0 ? 'compare-up' : delta < 0 ? 'compare-down' : 'compare-flat';
    return `<tr><th scope="row">${escHtml(field.label)}</th><td class="num">${fmtNum(lv)}</td><td class="num">${fmtNum(rv)}</td><td class="num ${cls}">${signedValue(delta)}</td></tr>`;
  }).join('');
  const measureFields = [
    { key: 'coverage', label: 'Coverage' },
    { key: 'mutation_score', label: 'Mutation' },
  ];
  const measureRows = measureFields.map(field => {
    const lv = left.measures?.[field.key];
    const rv = right.measures?.[field.key];
    const lvDisp = lv == null ? '\u2014' : fmtPct(lv);
    const rvDisp = rv == null ? '\u2014' : fmtPct(rv);
    if (lv == null || rv == null) {
      return `<tr><th scope="row">${escHtml(field.label)}</th><td class="num">${lvDisp}</td><td class="num">${rvDisp}</td><td class="num">\u2014</td></tr>`;
    }
    const delta = rv - lv;
    const cls = delta > 0 ? 'compare-up' : delta < 0 ? 'compare-down' : 'compare-flat';
    return `<tr><th scope="row">${escHtml(field.label)}</th><td class="num">${lvDisp}</td><td class="num">${rvDisp}</td><td class="num ${cls}">${signedValue(Math.round(delta * 10) / 10)}</td></tr>`;
  }).join('');
  return `<section class="activity-compare-panel" aria-label="Scan comparison">
    <div class="activity-panel-head">
      <div>
        <p class="section-title">Compare analyses</p>
        <p class="muted">${escHtml(new Date(left.analysis_date).toLocaleString())} \u2192 ${escHtml(new Date(right.analysis_date).toLocaleString())}</p>
      </div>
      <button type="button" class="btn-sm btn-ghost" id="activityCompareCloseBtn">Close</button>
    </div>
    <table class="activity-compare-table">
      <thead><tr><th></th><th>Older</th><th>Newer</th><th>Delta</th></tr></thead>
      <tbody>
        <tr><th scope="row">Gate</th><td>${gateBadge(left.gate_status)}</td><td>${gateBadge(right.gate_status)}</td><td>${left.gate_status === right.gate_status ? 'no change' : `${escHtml(left.gate_status || 'UNKNOWN')} \u2192 ${escHtml(right.gate_status || 'UNKNOWN')}`}</td></tr>
        ${rows}
        ${measureRows}
      </tbody>
    </table>
  </section>`;
}

function renderActivityLoadMore() {
  const page = state.activityPage || { hasMore: false };
  if (!page.hasMore) return '';
  const label = state.loadingMoreActivity ? 'Loading…' : 'Load more analyses';
  return `<div class="activity-load-more"><button type="button" class="btn-sm btn-outline" id="activityLoadMoreBtn"${state.loadingMoreActivity ? ' disabled' : ''}>${label}</button></div>`;
}

function exportActivityCSV(entries) {
  if (!entries.length) return;
  const headers = [
    'analysis_date', 'gate_status', 'version', 'branch', 'commit_sha',
    'total_issues', 'new_issues', 'closed_issues',
    'total_bugs', 'total_vulnerabilities', 'total_code_smells',
    'total_files', 'total_ncloc', 'coverage', 'mutation_score',
  ];
  const rows = entries.map(entry => {
    const m = entry.measures || {};
    const cells = [
      entry.analysis_date,
      entry.gate_status,
      entry.version,
      entry.branch,
      entry.commit_sha,
      entry.total_issues,
      entry.new_issues,
      entry.closed_issues,
      entry.total_bugs,
      entry.total_vulnerabilities,
      entry.total_code_smells,
      entry.total_files,
      entry.total_ncloc,
      m.coverage != null ? fmtPct(m.coverage) : '',
      m.mutation_score != null ? fmtPct(m.mutation_score) : '',
    ];
    return cells.map(v => {
      const s = String(v ?? '');
      if (s.includes(',') || s.includes('"') || s.includes('\n')) {
        return '"' + s.replace(/"/g, '""') + '"';
      }
      return s;
    }).join(',');
  });
  const csv = [headers.join(','), ...rows].join('\n');
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `activity-${state.currentProject?.key || 'project'}.csv`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
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
  state.activityPage = { offset: 0, hasMore: false };
  state.activityCompareSelection = [];
  try {
    const data = await apiFetch(buildScopedPath('/projects/' + encodeURIComponent(project.key) + '/activity?limit=' + ACTIVITY_PAGE_SIZE, state.scope));
    const items = data?.items || [];
    state.activityData = { items };
    const total = Number(data?.total || items.length);
    state.activityPage = { offset: items.length, hasMore: items.length < total, total };
  } catch {
    state.activityData = { items: [] };
  }
  renderView();
}

async function loadMoreActivity() {
  const project = state.currentProject;
  if (!project) return;
  const page = state.activityPage || { offset: 0 };
  state.loadingMoreActivity = true;
  renderView();
  try {
    const url = buildScopedPath(
      '/projects/' + encodeURIComponent(project.key) +
      '/activity?limit=' + ACTIVITY_PAGE_SIZE + '&offset=' + page.offset,
      state.scope,
    );
    const data = await apiFetch(url);
    const newItems = data?.items || [];
    const merged = (state.activityData?.items || []).concat(newItems);
    state.activityData = { items: merged };
    const total = Number(data?.total || merged.length);
    state.activityPage = { offset: merged.length, hasMore: merged.length < total, total };
  } catch {
    state.activityPage = { ...page, hasMore: false };
  }
  state.loadingMoreActivity = false;
  renderView();
}

function openScanInIssues(scanId) {
  if (scanId == null || scanId === '') return;
  state.issueFilter = state.issueFilter || {};
  state.issueFilter.scanId = String(scanId);
  state.issues = [];
  state.issueOffset = 0;
  document.querySelector('.tab-btn[data-tab="issues"]')?.click();
}

function toggleCompareSelection(scanId, branch) {
  const id = Number(scanId);
  if (!Number.isFinite(id)) return false;
  const selection = state.activityCompareSelection || [];
  const items = state.activityData?.items || [];
  if (selection.includes(id)) {
    state.activityCompareSelection = selection.filter(s => s !== id);
    return true;
  }
  if (selection.length >= 1) {
    const first = items.find(item => item.scan_id === selection[0]);
    const firstBranch = first?.branch || '';
    if (firstBranch && branch && firstBranch !== branch) {
      return false;
    }
  }
  if (selection.length < 2) {
    state.activityCompareSelection = [...selection, id];
  } else {
    state.activityCompareSelection = [selection[1], id];
  }
  return true;
}

function bindActivityInteractions() {
  document.querySelectorAll('.activity-filter-chip').forEach(btn => {
    btn.addEventListener('click', () => {
      const cat = btn.dataset.activityCat;
      const filters = state.activityFilters || { categories: [], window: 'all' };
      const cats = new Set(filters.categories || []);
      if (cats.has(cat)) cats.delete(cat); else cats.add(cat);
      state.activityFilters = { ...filters, categories: [...cats] };
      renderView();
    });
  });

  document.getElementById('activityClearCategoriesBtn')?.addEventListener('click', () => {
    const filters = state.activityFilters || { categories: [], window: 'all' };
    state.activityFilters = { ...filters, categories: [] };
    renderView();
  });

  document.querySelectorAll('.activity-window-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const win = btn.dataset.activityWindow;
      const filters = state.activityFilters || { categories: [], window: 'all' };
      state.activityFilters = { ...filters, window: win };
      renderView();
    });
  });

  document.querySelectorAll('.activity-chart-point[data-activity-scan]').forEach(pt => {
    const handler = () => openScanInIssues(pt.dataset.activityScan);
    pt.addEventListener('click', handler);
    pt.addEventListener('keydown', event => {
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault();
        handler();
      }
    });
  });

  document.querySelectorAll('.activity-entry[data-activity-scan]').forEach(row => {
    const handler = () => openScanInIssues(row.dataset.activityScan);
    row.addEventListener('click', handler);
    row.addEventListener('keydown', event => {
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault();
        handler();
      }
    });
  });

  document.querySelectorAll('.activity-scan-row .activity-scan-link.clickable').forEach(cell => {
    cell.addEventListener('click', () => {
      const row = cell.closest('.activity-scan-row');
      openScanInIssues(row?.dataset.activityScan);
    });
  });

  document.querySelectorAll('.activity-compare-pick').forEach(box => {
    box.addEventListener('click', event => event.stopPropagation());
    box.addEventListener('change', () => {
      const ok = toggleCompareSelection(box.dataset.activityScan, box.dataset.activityBranch || '');
      if (!ok) {
        box.checked = false;
        box.title = 'Cannot compare scans across different branches';
        return;
      }
      renderView();
    });
  });

  document.getElementById('activityCompareCloseBtn')?.addEventListener('click', () => {
    state.activityCompareSelection = [];
    renderView();
  });

  document.getElementById('activityLoadMoreBtn')?.addEventListener('click', () => {
    loadMoreActivity();
  });

  document.getElementById('activityExportBtn')?.addEventListener('click', () => {
    const allEntries = normalizeActivityPayload(state.activityData);
    const filters = state.activityFilters || { categories: [], window: 'all' };
    const filtered = applyActivityFilters(allEntries, filters);
    exportActivityCSV(filtered.length ? filtered : allEntries);
  });
}

export function bindActivityTabContent() {
  bindActivityInteractions();
}
