import {
  SEV_COLOR,
  SEV_LABEL,
  SEV_ORDER,
  TYPE_COLOR,
  TYPE_LABEL,
  state,
} from '../core/state.js';
import { escAttr, escHtml, fmtK, fmtNum, fmtPct } from '../core/utils.js';

function renderOverviewEmptyState() {
  return `<div class="empty-state">
      <div class="empty-icon">\uD83D\uDD2C</div>
      <p>No scans yet for this project.<br>Run <code>ollanta</code> to submit a scan.</p>
    </div>`;
}

function coverageCardClass(coverage) {
  if (coverage == null) {
    return 'card-neutral';
  }
  if (coverage >= 80) {
    return 'card-green';
  }
  if (coverage >= 60) {
    return 'card-yellow';
  }
  return 'card-red';
}

function duplicationCardClass(dupDensity) {
  if (dupDensity == null) {
    return 'card-neutral';
  }
  if (dupDensity <= 3) {
    return 'card-green';
  }
  if (dupDensity <= 10) {
    return 'card-yellow';
  }
  return 'card-red';
}

function metricCard(label, value, colorCls, cardCls, typeFilter) {
  return `<div class="metric-card ${cardCls} clickable" data-mc-type="${typeFilter || ''}">
    <div class="metric-value ${colorCls}">${fmtNum(value)}</div>
    <div class="metric-label">${label}</div>
    <div class="metric-hint">View issues \u203A</div>
  </div>`;
}

function metricCardPct(label, value, cardCls) {
  return `<div class="metric-card ${cardCls}">
    <div class="metric-value">${value != null ? fmtPct(value) : '\u2014'}</div>
    <div class="metric-label">${label}</div>
  </div>`;
}

function metricCardK(label, value, cardCls) {
  return `<div class="metric-card ${cardCls}">
    <div class="metric-value">${fmtK(value)}</div>
    <div class="metric-label">${label}</div>
  </div>`;
}

function renderDistribution(title, order, labels, colors, data) {
  const total = order.reduce((sum, key) => sum + (data[key] || 0), 0);
  if (total === 0) return '';

  const rows = order.map(key => {
    const count = data[key] || 0;
    const pct = total > 0 ? (count / total * 100) : 0;
    return `<div class="dist-row">
      <span class="dist-label">${labels[key] || key}</span>
      <div class="dist-bar">
        <div class="dist-fill" style="width:${pct}%;background:${colors[key] || 'var(--accent)'}"></div>
      </div>
      <span class="dist-count">${fmtNum(count)}</span>
    </div>`;
  }).join('');

  return `<div class="dist-section">
    <p class="dist-title">${title} Distribution</p>
    <div class="dist-bar-wrap">${rows}</div>
  </div>`;
}

function renderOverviewMetrics(measures) {
  const bugs = measures.bugs || 0;
  const vulns = measures.vulnerabilities || 0;
  const smells = measures.code_smells || 0;
  const coverage = measures.coverage;
  const dupDensity = measures.duplicated_lines_density;
  const ncloc = measures.ncloc || 0;
  const bugCardClass = bugs > 0 ? 'card-red' : 'card-green';
  const bugColorClass = bugs > 0 ? 'danger' : 'success';
  const vulnCardClass = vulns > 0 ? 'card-yellow' : 'card-green';
  const vulnColorClass = vulns > 0 ? 'warning' : 'success';
  const smellCardClass = smells > 20 ? 'card-yellow' : 'card-green';

  return `<div class="measures-row">
    ${metricCard('Bugs', bugs, bugColorClass, bugCardClass, 'bug')}
    ${metricCard('Vulnerabilities', vulns, vulnColorClass, vulnCardClass, 'vulnerability')}
    ${metricCard('Code Smells', smells, 'muted', smellCardClass, 'code_smell')}
    ${metricCardPct('Coverage', coverage, coverageCardClass(coverage))}
    ${metricCardPct('Duplication', dupDensity, duplicationCardClass(dupDensity))}
    ${metricCardK('Lines of Code', ncloc, 'card-neutral')}
  </div>`;
}

function renderOverviewHotspots(fileDist) {
  const fileEntries = Object.entries(fileDist || {}).sort((a, b) => b[1] - a[1]).slice(0, 10);
  if (fileEntries.length === 0) {
    return '';
  }

  return `
    <div class="hotspot-section">
      <p class="section-title">Hotspot Files</p>
      <div class="hotspot-list">
        ${fileEntries.map(([filePath, count]) => {
          const short = filePath.replace(/\\/g, '/').split('/').slice(-3).join('/');
          return `<div class="hotspot-row" data-file="${escAttr(filePath)}">
            <span class="hotspot-file">${escHtml(short)}</span>
            <span class="hotspot-count">${count}</span>
          </div>`;
        }).join('')}
      </div>
    </div>`;
}

function renderOverviewNewCode(newCode) {
  if (newCode.new_issues == null && newCode.closed_issues == null) {
    return '';
  }

  const newIssueColor = newCode.new_issues > 0 ? 'var(--warning)' : 'var(--success)';
  return `
    <div class="new-code-section">
      <span class="new-code-badge">New Code</span>
      <div class="new-code-metrics">
        <span><span class="ncm-val" style="color:${newIssueColor}">${fmtNum(newCode.new_issues || 0)}</span> new issues</span>
        <span><span class="ncm-val" style="color:var(--success)">${fmtNum(newCode.closed_issues || 0)}</span> closed</span>
      </div>
    </div>`;
}

function renderOverviewScanInfo(scan) {
  if (!scan) {
    return '';
  }

  const commit = scan.commit_sha ? escHtml(scan.commit_sha.slice(0, 8)) : '\u2014';
  const analysisDate = scan.analysis_date ? new Date(scan.analysis_date).toLocaleString() : '\u2014';
  const elapsed = scan.elapsed_ms ? (scan.elapsed_ms / 1000).toFixed(1) + 's' : '\u2014';

  return `
    <p class="section-title">Latest Scan</p>
    <div class="scan-info">
      <div>
        <div class="info-label">Version</div>
        <div class="info-value">${escHtml(scan.version || '\u2014')}</div>
      </div>
      <div>
        <div class="info-label">Branch</div>
        <div class="info-value">${escHtml(scan.branch || '\u2014')}</div>
      </div>
      <div>
        <div class="info-label">Commit</div>
        <div class="info-value mono" style="font-size:12px">${commit}</div>
      </div>
      <div>
        <div class="info-label">Status</div>
        <div class="info-value">${escHtml(scan.status || '\u2014')}</div>
      </div>
      <div>
        <div class="info-label">Analysis date</div>
        <div class="info-value">${analysisDate}</div>
      </div>
      <div>
        <div class="info-label">Elapsed</div>
        <div class="info-value">${elapsed}</div>
      </div>
    </div>`;
}

function renderGateHero(gate, measures) {
  const gateMeasures = measures || {};
  if (!gate || !gate.status || gate.status === 'NONE') {
    return `<div class="gate-hero gate-loading">
      <div class="gate-badge">
        <span class="gate-icon">\u2014</span>
        <div class="gate-text">
          <span class="gate-label">Quality Gate</span>
          <span class="gate-status-text">Not configured</span>
        </div>
      </div>
    </div>`;
  }

  const status = gate.status;
  const cls = status === 'OK' ? 'gate-passed' : status === 'WARN' ? 'gate-warn' : 'gate-failed';
  const icon = status === 'OK' ? '\u2713' : status === 'WARN' ? '!' : '\u2717';
  const text = status === 'OK' ? 'Passed' : status === 'WARN' ? 'Warning' : 'Failed';

  let reasonsHtml = '';
  if (status !== 'OK') {
    const metricMessages = {
      bugs: value => `${value} bug${value !== 1 ? 's' : ''} found`,
      vulnerabilities: value => `${value} vulnerability${value !== 1 ? 'ies' : 'y'} found`,
      new_bugs: value => `${value} new bug${value !== 1 ? 's' : ''} introduced`,
      new_vulnerabilities: value => `${value} new vulnerability${value !== 1 ? 'ies' : 'y'} introduced`,
      code_smells: value => `${value} code smell${value !== 1 ? 's' : ''} detected`,
      coverage: value => `Code coverage is ${value}%`,
      duplicated_lines_density: value => `Duplication at ${value}%`,
    };
    const reasons = (gate.conditions || []).map(condition => {
      const value = gateMeasures[condition.metric];
      const violated = value !== undefined && (
        condition.operator === 'GT'  ? value > condition.threshold :
        condition.operator === 'LT'  ? value < condition.threshold :
        condition.operator === 'GTE' ? value >= condition.threshold :
        condition.operator === 'LTE' ? value <= condition.threshold :
        condition.operator === 'EQ'  ? value === condition.threshold :
        value !== condition.threshold
      );
      if (!violated) return null;
      const formatter = metricMessages[condition.metric];
      return formatter ? formatter(value) : null;
    }).filter(Boolean);
    if (reasons.length) {
      reasonsHtml = `<ul class="gate-reasons">${reasons.map(reason => `<li>${reason}</li>`).join('')}</ul>`;
    }
  }

  return `<div class="gate-hero ${cls}">
    <div class="gate-badge">
      <span class="gate-icon">${icon}</span>
      <div class="gate-text">
        <span class="gate-label">Quality Gate</span>
        <span class="gate-status-text">${text}</span>
      </div>
    </div>
    ${reasonsHtml}
  </div>`;
}

export function renderOverviewTab() {
  const overview = state.overviewData;
  if (!overview) {
    return renderOverviewEmptyState();
  }

  const measures = overview.measures || {};
  const facets = overview.facets || {};
  const severityDist = facets.by_severity || {};
  const typeDist = facets.by_type || {};

  return [
    renderGateHero(overview.quality_gate || {}, measures),
    renderOverviewNewCode(overview.new_code || {}),
    renderOverviewMetrics(measures),
    renderDistribution('Severity', SEV_ORDER, SEV_LABEL, SEV_COLOR, severityDist),
    renderDistribution('Type', ['bug', 'code_smell', 'vulnerability'], TYPE_LABEL, TYPE_COLOR, typeDist),
    renderOverviewHotspots(facets.by_file || {}),
    renderOverviewScanInfo(overview.last_scan),
  ].join('');
}