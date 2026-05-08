import {
  SEV_COLOR,
  SEV_LABEL,
  SEV_ORDER,
  TYPE_COLOR,
  TYPE_LABEL,
  state,
} from '../core/state.js';
import { escAttr, escHtml, fmtK, fmtNum, fmtPct } from '../core/utils.js';
import { renderProjectInformationPanel } from './project-information.js';

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

function metricSignal(label, value, colorCls, statusCls, typeFilter, action) {
  const interactive = typeFilter || action ? ' clickable' : '';
  const dataAttr = typeFilter ? ` data-mc-type="${typeFilter}"` : '';
  const actionAttr = action ? ` data-overview-action="${escAttr(action)}"` : '';
  return `<button class="metric-signal ${statusCls}${interactive}"${dataAttr}${actionAttr}>
    <span class="metric-signal-label">${label}</span>
    <span class="metric-signal-value ${colorCls || ''}">${fmtNum(value)}</span>
  </button>`;
}

function metricSignalPct(label, value, statusCls, action, emptyHint) {
  const isEmpty = value == null;
  const displayValue = isEmpty ? '\u2014' : fmtPct(value);
  const hint = isEmpty && emptyHint ? `<span class="metric-signal-hint">${escHtml(emptyHint)}</span>` : '';
  if (!action) {
    return `<div class="metric-signal ${statusCls}">
      <span class="metric-signal-label">${label}</span>
      <span class="metric-signal-value">${displayValue}</span>
      ${hint}
    </div>`;
  }
  return `<button class="metric-signal ${statusCls} clickable" data-overview-action="${escAttr(action)}">
    <span class="metric-signal-label">${label}</span>
    <span class="metric-signal-value">${displayValue}</span>
    ${hint}
  </button>`;
}

function metricSignalK(label, value, statusCls) {
  return `<div class="metric-signal ${statusCls}">
    <span class="metric-signal-label">${label}</span>
    <span class="metric-signal-value">${fmtK(value)}</span>
  </div>`;
}

function hasMutationMetrics(metrics = {}) {
  return metrics.mutation_score != null || metrics.changed_mutation_score != null || metrics.mutants_total > 0 || metrics.changed_mutants_total > 0;
}

function mutationCardClass(score) {
  if (score == null) return 'card-neutral';
  if (score >= 80) return 'card-green';
  if (score >= 60) return 'card-yellow';
  return 'card-red';
}

function renderMutationSignals(metrics = {}) {
  if (!hasMutationMetrics(metrics)) {
    return `<section class="overview-panel summary-section">
      <p class="section-title">Mutation Testing</p>
      <div class="empty-state compact">No mutation report collected for this scan.</div>
    </section>`;
  }
  const score = metrics.changed_mutation_score ?? metrics.mutation_score;
  const total = metrics.changed_mutants_total ?? metrics.mutants_total ?? 0;
  const killed = metrics.changed_mutants_killed ?? metrics.mutants_killed ?? 0;
  const survived = metrics.changed_mutants_survived ?? metrics.mutants_survived ?? 0;
  const scopeLabel = metrics.changed_mutation_score != null || metrics.changed_mutants_total > 0 ? 'Changed Code Mutation' : 'Mutation Testing';

  return `<section class="overview-panel summary-section">
    <p class="section-title">${scopeLabel}</p>
    <div class="metric-signals compact">
      ${metricSignalPct('Mutation Score', score, mutationCardClass(score))}
      ${metricSignal('Mutants', total, 'muted', 'card-neutral')}
      ${metricSignal('Killed', killed, 'success', killed > 0 ? 'card-green' : 'card-neutral')}
      ${metricSignal('Survived', survived, survived > 0 ? 'warning' : 'success', survived > 0 ? 'card-yellow' : 'card-green')}
      ${metricSignal('Skipped', metrics.mutants_skipped || 0, 'muted', (metrics.mutants_skipped || 0) > 0 ? 'card-yellow' : 'card-neutral')}
      ${metricSignal('Errors', metrics.mutants_error || 0, (metrics.mutants_error || 0) > 0 ? 'danger' : 'success', (metrics.mutants_error || 0) > 0 ? 'card-red' : 'card-green')}
    </div>
  </section>`;
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

  return `<div class="metric-signals">
    ${metricSignal('Bugs', bugs, bugColorClass, bugCardClass, 'bug')}
    ${metricSignal('Vulnerabilities', vulns, vulnColorClass, vulnCardClass, 'vulnerability')}
    ${metricSignal('Code Smells', smells, 'muted', smellCardClass, 'code_smell')}
    ${metricSignalPct('Coverage', coverage, coverageCardClass(coverage), 'coverage')}
    ${metricSignalPct('Duplication', dupDensity, duplicationCardClass(dupDensity))}
    ${metricSignalK('Lines of Code', ncloc, 'card-neutral')}
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
          const short = filePath.replaceAll('\\', '/').split('/').slice(-3).join('/');
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
  const status = scan.status || '\u2014';
  const statusCls = status === 'completed' ? 'success' : status === 'failed' ? 'danger' : 'muted';

  return `
    <p class="section-title">Latest Scan</p>
    <div class="latest-scan">
      <div class="latest-scan-row">
        <span class="latest-scan-status ${statusCls}">\u25CF ${escHtml(status)}</span>
        <span class="latest-scan-version">${escHtml(scan.version || '\u2014')}</span>
      </div>
      <div class="latest-scan-row">
        <span class="latest-scan-label">Branch</span>
        <span>${escHtml(scan.branch || '\u2014')}</span>
      </div>
      <div class="latest-scan-row">
        <span class="latest-scan-label">Commit</span>
        <span class="mono" title="${escAttr(scan.commit_sha || '')}">${commit}</span>
      </div>
      <div class="latest-scan-row">
        <span class="latest-scan-label">Analyzed</span>
        <span>${escHtml(analysisDate)}</span>
      </div>
      <div class="latest-scan-row">
        <span class="latest-scan-label">Elapsed</span>
        <span>${escHtml(elapsed)}</span>
      </div>
    </div>`;
}

function gatePresentation(status) {
  if (status === 'OK') {
    return { cls: 'gate-passed', icon: '\u2713', text: 'Passed' };
  }
  if (status === 'WARN') {
    return { cls: 'gate-warn', icon: '!', text: 'Warning' };
  }
  return { cls: 'gate-failed', icon: '\u2717', text: 'Failed' };
}

function plural(value, singular, pluralValue) {
  return value === 1 ? singular : pluralValue;
}

function violatesGateCondition(condition, value) {
  if (condition.operator === 'GT') return value > condition.threshold;
  if (condition.operator === 'LT') return value < condition.threshold;
  if (condition.operator === 'GTE') return value >= condition.threshold;
  if (condition.operator === 'LTE') return value <= condition.threshold;
  if (condition.operator === 'EQ') return value === condition.threshold;
  return value !== condition.threshold;
}

function gateReasonMessage(metric, value) {
  const metricMessages = {
    bugs: count => `${count} ${plural(count, 'bug', 'bugs')} found`,
    vulnerabilities: count => `${count} ${plural(count, 'vulnerability', 'vulnerabilities')} found`,
    new_bugs: count => `${count} new ${plural(count, 'bug', 'bugs')} introduced`,
    new_vulnerabilities: count => `${count} new ${plural(count, 'vulnerability', 'vulnerabilities')} introduced`,
    code_smells: count => `${count} code ${plural(count, 'smell', 'smells')} detected`,
    coverage: count => `Code coverage is ${count}%`,
    duplicated_lines_density: count => `Duplication at ${count}%`,
  };
  return metricMessages[metric]?.(value) || null;
}

function renderGateHero(gate, measures) {
  const gateMeasures = measures || {};
  if (!gate?.status || gate.status === 'NONE') {
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
  const { cls, icon, text } = gatePresentation(status);

  let reasonsHtml = '';
  if (status !== 'OK') {
    const reasons = (gate.conditions || []).map(condition => {
      const value = gateMeasures[condition.metric];
      const violated = value !== undefined && violatesGateCondition(condition, value);
      if (!violated) return null;
      const metricLabel = overviewMetricLabel(condition.metric);
      const opLabel = overviewOperatorLabel(condition.operator);
      return `${metricLabel} ${opLabel} ${condition.threshold} (actual: ${value})`;
    }).filter(Boolean);
    if (reasons.length) {
      const reasonItems = reasons.map(reason => `<li>${reason}</li>`).join('');
      reasonsHtml = `<ul class="gate-reasons">${reasonItems}</ul>`;
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

function overviewMetricLabel(metric) {
  const labels = { bugs: 'Bugs', vulnerabilities: 'Vulnerabilities', code_smells: 'Code Smells', coverage: 'Coverage', new_bugs: 'New Bugs', new_vulnerabilities: 'New Vulnerabilities', new_code_smells: 'New Code Smells', new_coverage: 'Coverage on New Code', duplicated_lines_density: 'Duplicated Lines', new_duplicated_lines_density: 'Duplicated Lines on New Code' };
  return labels[metric] || metric;
}

function overviewOperatorLabel(op) {
  const labels = { GT: 'is greater than', LT: 'is less than', GTE: 'is greater than or equal', LTE: 'is less than or equal', EQ: 'equals', NE: 'is not equal' };
  return labels[op] || op;
}

function summaryHeroClass(status) {
  if (status === 'ready') return 'gate-passed';
  if (status === 'attention') return 'gate-warn';
  if (status === 'needs_setup' || status === 'empty') return 'gate-loading';
  return 'gate-failed';
}

function summaryStatusText(status) {
  if (status === 'ready') return 'Ready';
  if (status === 'attention') return 'Attention';
  if (status === 'needs_setup') return 'Needs setup';
  if (status === 'empty') return 'Awaiting scans';
  return 'Blocked';
}

function comparisonSymbol(operator) {
  if (operator === 'GT') return '>';
  if (operator === 'GTE') return '>=';
  if (operator === 'LT') return '<';
  if (operator === 'LTE') return '<=';
  if (operator === 'EQ') return '=';
  return '!=';
}

function formatSummaryMetricValue(metric, value) {
  if (value == null) return '\u2014';
  if (metric === 'coverage' || metric === 'new_coverage' || metric === 'duplicated_lines_density' || metric === 'new_duplications') {
    return fmtPct(value);
  }
  return fmtNum(value);
}

function renderSummaryEmptyState(emptyState) {
  return `<div class="empty-state">
      <div class="empty-icon">\uD83D\uDD2C</div>
      <p>${escHtml(emptyState?.headline || 'No completed scans for this scope yet.')}</p>
      ${emptyState?.recommended_action ? `<p class="summary-empty-note">${escHtml(emptyState.recommended_action)}</p>` : ''}
    </div>`;
}

function summaryStatusIcon(status) {
  if (status === 'ready') return '\u2713';
  if (status === 'attention') return '!';
  if (status === 'needs_setup' || status === 'empty') return '\u2014';
  return '\u2717';
}

function renderSummaryReason(reason) {
  const label = reason.label || reason.metric || 'Condition';
  const actual = formatSummaryMetricValue(reason.metric, reason.actual);
  const operator = comparisonSymbol(reason.operator || 'NE');
  const threshold = formatSummaryMetricValue(reason.metric, reason.threshold);
  return `<li><span class="gate-reason-label">${escHtml(label)}</span><span class="gate-reason-actual">${escHtml(actual)}</span><span class="gate-reason-op">${escHtml(operator)}</span><span class="gate-reason-threshold">${escHtml(threshold)}</span></li>`;
}

function renderSummaryHero(review) {
  if (!review) {
    return '';
  }

  const cls = summaryHeroClass(review.status);
  const icon = summaryStatusIcon(review.status);
  const reasonItems = (review.reasons || []).map(renderSummaryReason).join('');
  const reasonsHtml = reasonItems ? `<ul class="gate-reasons">${reasonItems}</ul>` : '';

  return `<div class="gate-hero ${cls}">
    <div class="gate-badge">
      <span class="gate-icon">${icon}</span>
      <div class="gate-text">
        <span class="gate-label">Review Summary</span>
        <span class="gate-status-text">${summaryStatusText(review.status)}</span>
      </div>
    </div>
    <div class="summary-hero-copy">
      <div class="summary-headline">${escHtml(review.headline || 'Review Summary')}</div>
      ${reasonsHtml}
    </div>
  </div>`;
}

function renderSummaryNewCode(newCode) {
  const metrics = newCode?.metrics || {};
  const hasMetrics = Object.keys(metrics).length > 0;
  if (!hasMetrics && !newCode?.baseline) {
    return '';
  }

  return `<section class="overview-panel summary-section">
    <p class="section-title">New Code Focus</p>
    ${newCode?.baseline?.label ? `<div class="summary-baseline">${escHtml(newCode.baseline.label)}</div>` : ''}
    <div class="metric-signals compact">
      ${metricSignal('New Issues', metrics.new_issues || 0, (metrics.new_issues || 0) > 0 ? 'warning' : 'success', (metrics.new_issues || 0) > 0 ? 'card-yellow' : 'card-green', null, 'new-issues')}
      ${metricSignal('New Bugs', metrics.new_bugs || 0, (metrics.new_bugs || 0) > 0 ? 'danger' : 'success', (metrics.new_bugs || 0) > 0 ? 'card-red' : 'card-green', 'bug')}
      ${metricSignal('New Vulnerabilities', metrics.new_vulnerabilities || 0, (metrics.new_vulnerabilities || 0) > 0 ? 'warning' : 'success', (metrics.new_vulnerabilities || 0) > 0 ? 'card-yellow' : 'card-green', 'vulnerability')}
      ${metricSignal('New Code Smells', metrics.new_code_smells || 0, 'muted', (metrics.new_code_smells || 0) > 0 ? 'card-yellow' : 'card-green', 'code_smell')}
      ${metricSignalPct('New Coverage', metrics.new_coverage, coverageCardClass(metrics.new_coverage), 'coverage', 'No new lines')}
      ${metricSignalPct('Changed Mutation', metrics.changed_mutation_score, mutationCardClass(metrics.changed_mutation_score), null, 'No mutation report')}
      ${metricSignal('Survived Mutants', metrics.changed_mutants_survived || 0, (metrics.changed_mutants_survived || 0) > 0 ? 'warning' : 'success', (metrics.changed_mutants_survived || 0) > 0 ? 'card-yellow' : 'card-green', null, 'survived-mutants')}
      ${metricSignalPct('New Duplication', metrics.new_duplications, duplicationCardClass(metrics.new_duplications), null, 'No new lines')}
      ${metricSignal('Closed Issues', metrics.closed_issues || 0, 'success', (metrics.closed_issues || 0) > 0 ? 'card-green' : 'card-neutral', null, 'closed-issues')}
    </div>
  </section>`;
}

function severityBadge(severity = 'info') {
  const value = severity;
  return `<span class="sev-badge" style="background:var(--sev-${escAttr(value)}, var(--accent));">${escHtml(value)}</span>`;
}

function renderSummaryMustFixNow(items) {
  if (!items?.length) {
    return '';
  }

  return `<section class="overview-panel summary-section hotspot-section">
    <p class="section-title">Must Fix Now</p>
    <div class="hotspot-list">
      ${items.map((item, index) => {
        const short = (item.component_path || '').replaceAll('\\', '/').split('/').slice(-2).join('/');
        const fullPath = item.component_path || '';
        const location = item.line ? `:${fmtNum(item.line)}` : '';
        const message = item.message || item.rule_key || '';
        const why = item.why_selected ? `<span class="summary-row-why">${escHtml(item.why_selected)}</span>` : '';
        return `<button class="hotspot-row mustfix-row" data-overview-issue-index="${index}" title="${escAttr(fullPath + location)}">
          <div class="mustfix-head">
            ${severityBadge(item.severity)}
            <span class="mustfix-path mono">${escHtml(short)}<span class="mustfix-line">${escHtml(location)}</span></span>
          </div>
          <div class="mustfix-body">
            <span class="mustfix-message">${escHtml(message)}</span>
            ${why}
          </div>
        </button>`;
      }).join('')}
    </div>
  </section>`;
}

function renderSummaryImpactedFiles(files) {
  if (!files?.length) {
    return '';
  }

  return `<section class="overview-panel summary-section hotspot-section">
    <p class="section-title">Impacted Files</p>
    <div class="hotspot-list">
      ${files.map(item => {
        const short = (item.component_path || '').replaceAll('\\', '/').split('/').slice(-3).join('/');
        const meta = [
          `${fmtNum(item.issue_count || 0)} issue${(item.issue_count || 0) === 1 ? '' : 's'}`,
          item.coverage == null ? '' : `${fmtPct(item.coverage)} coverage`,
          item.duplicated_lines_density == null ? '' : `${fmtPct(item.duplicated_lines_density)} duplication`,
        ].filter(Boolean);
        return `<div class="hotspot-row" data-file="${escAttr(item.component_path || '')}">
          <div class="summary-row-main">
            <div class="summary-row-title">${escHtml(short)}</div>
            <div class="summary-row-subtitle">${escHtml(item.component_path || '')}</div>
          </div>
          <div class="summary-row-meta">
            ${meta.map(entry => `<span class="summary-row-note">${escHtml(entry)}</span>`).join('')}
          </div>
        </div>`;
      }).join('')}
    </div>
  </section>`;
}

function renderSummaryOverall(overall) {
  const metrics = overall?.metrics || {};
  if (!Object.keys(metrics).length) {
    return '';
  }

  return `<section class="overview-panel summary-section">
    <p class="section-title">Overall Snapshot</p>
    <div class="metric-signals rail-stack">
      ${renderOverviewMetricsInner(metrics)}
    </div>
  </section>`;
}

function renderOverviewMetricsInner(measures) {
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

  return [
    metricSignal('Bugs', bugs, bugColorClass, bugCardClass, 'bug'),
    metricSignal('Vulnerabilities', vulns, vulnColorClass, vulnCardClass, 'vulnerability'),
    metricSignal('Code Smells', smells, 'muted', smellCardClass, 'code_smell'),
    metricSignalPct('Coverage', coverage, coverageCardClass(coverage), 'coverage'),
    metricSignalPct('Duplication', dupDensity, duplicationCardClass(dupDensity)),
    metricSignalK('Lines of Code', ncloc, 'card-neutral'),
  ].join('');
}

function renderReviewSummaryTab(overview) {
  const summary = overview.summary || {};
  if (summary.empty_state) {
    return renderSummaryEmptyState(summary.empty_state);
  }

  return `<div class="overview-shell">
    <div class="overview-primary">
      ${renderSummaryHero(summary.review)}
      ${renderSummaryNewCode(summary.new_code)}
      ${hasMutationMetrics(summary.overall_code?.metrics || {}) ? renderMutationSignals(summary.overall_code?.metrics || {}) : ''}
      <div class="overview-workbench">
        ${renderSummaryMustFixNow(summary.must_fix_now || [])}
        ${renderSummaryImpactedFiles(summary.impacted_files || [])}
      </div>
      ${renderProjectInformationPanel()}
    </div>
    <aside class="overview-rail">
      ${renderSummaryOverall(summary.overall_code)}
      <section class="overview-panel">${renderOverviewScanInfo(overview.last_scan)}</section>
    </aside>
  </div>`;
}

export function renderOverviewTab() {
  const overview = state.overviewData;
  if (!overview) {
    return renderOverviewEmptyState();
  }

  if (overview.summary) {
    return renderReviewSummaryTab(overview);
  }

  const measures = overview.measures || {};
  const facets = overview.facets || {};
  const severityDist = facets.by_severity || {};
  const typeDist = facets.by_type || {};

  return `<div class="overview-shell">
    <div class="overview-primary">
      ${renderGateHero(overview.quality_gate || {}, measures)}
      ${renderOverviewNewCode(overview.new_code || {})}
      <section class="overview-panel">${renderOverviewMetrics(measures)}</section>
      ${renderMutationSignals(measures)}
      <div class="overview-workbench">
        <section class="overview-panel">${renderDistribution('Severity', SEV_ORDER, SEV_LABEL, SEV_COLOR, severityDist)}</section>
        <section class="overview-panel">${renderDistribution('Type', ['bug', 'code_smell', 'vulnerability'], TYPE_LABEL, TYPE_COLOR, typeDist)}</section>
      </div>
      ${renderProjectInformationPanel()}
    </div>
    <aside class="overview-rail">
      <section class="overview-panel">${renderOverviewHotspots(facets.by_file || {})}</section>
      <section class="overview-panel">${renderOverviewScanInfo(overview.last_scan)}</section>
    </aside>
  </div>`;
}
