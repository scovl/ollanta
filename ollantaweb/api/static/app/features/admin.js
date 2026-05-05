import { apiFetch } from '../core/api.js';
import { state } from '../core/state.js';
import { escAttr, escHtml, fmtDate, fmtNum } from '../core/utils.js';

let renderView = () => {};
let showToastMessage = () => {};

export function configureAdminFeature(options) {
  renderView = options.render;
  showToastMessage = options.showToast;
}

export async function loadGateData() {
  try {
    const data = await apiFetch('/quality-gates');
    state.gateData = data.items || (Array.isArray(data) ? data : []);
  } catch {
    state.gateData = [];
  }
  renderView();
}

export function renderGateTab() {
  const gates = state.gateData;
  if (gates === null) return `<div class="loading-state"><div class="spinner"></div></div>`;

  const rows = (gates || []).map(gate => `
    <div class="gate-card">
      <div class="gate-header">
        <div>
          <span class="gate-name">${escHtml(gate.name)}</span>
          ${gate.is_default ? `<span class="badge badge-ok" style="font-size:11px;margin-left:8px">Default</span>` : ''}
          ${gate.is_builtin ? `<span class="badge badge-warn" style="font-size:11px;margin-left:8px">Built-in</span>` : ''}
        </div>
        <div class="gate-actions">
          <button class="btn-sm btn-outline assign-gate-btn" data-gate-id="${gate.id}" data-gate-name="${escAttr(gate.name)}">Assign to project</button>
          ${gate.is_builtin ? '' : `<button class="btn-sm btn-ghost set-default-gate-btn" data-gate-id="${gate.id}" data-gate-name="${escAttr(gate.name)}">Set default</button>`}
          <button class="btn-sm btn-ghost copy-gate-btn" data-gate-id="${gate.id}" data-gate-name="${escAttr(gate.name)}">Copy</button>
          ${gate.is_builtin ? '' : `<button class="btn-sm btn-danger del-gate-btn" data-gate-id="${gate.id}" data-gate-name="${escAttr(gate.name)}">Delete</button>`}
          <button class="btn-sm btn-ghost expand-gate-btn" data-gate-id="${gate.id}">Conditions \u25BE</button>
        </div>
      </div>
      <div class="gate-conditions hidden" id="gate-cond-${gate.id}">
        <div class="loading-inline">Loading\u2026</div>
      </div>
    </div>`).join('');

  const emptyMsg = (gates || []).length ? '' : `<div class="empty-state" style="padding:20px 0"><p>No quality gates configured.</p></div>`;

  return `<div class="tab-section">
    <p class="section-title" style="margin-top:24px">Quality Gates</p>
    <p style="color:var(--text-muted);font-size:13px;margin-bottom:16px">Conditions that must pass for a project analysis to be considered successful.</p>
    <div class="create-form" style="margin-bottom:20px">
      <div class="form-row">
        <input id="newGateName" class="filter-input" placeholder="Gate name" style="width:250px">
        <button class="btn btn-primary" id="createGateBtn" style="width:auto;padding:6px 18px;margin-top:0">Create gate</button>
      </div>
    </div>
    <div class="gate-list">${rows || ''}</div>
    ${emptyMsg}
  </div>`;
}

export async function loadWebhooksData() {
  const project = state.currentProject;
  if (!project) return;
  try {
    const data = await apiFetch('/webhooks?project_key=' + encodeURIComponent(project.key));
    state.webhooksData = data.items || (Array.isArray(data) ? data : []);
  } catch {
    state.webhooksData = [];
  }
  try {
    state.newCodePeriod = await apiFetch('/projects/' + encodeURIComponent(project.key) + '/new-code-period');
  } catch {
    state.newCodePeriod = null;
  }
  renderView();
}

export function renderWebhooksTab() {
  const webhooks = state.webhooksData;
  if (webhooks === null) return `<div class="loading-state"><div class="spinner"></div></div>`;

  const ncp = state.newCodePeriod;
  const ncpStr = formatNewCodePeriod(ncp);

  const webhookRows = webhooks.length === 0
    ? `<div class="empty-state" style="padding:20px 0"><p>No webhooks configured.</p></div>`
    : webhooks.map(webhook => `
      <div class="webhook-row">
        <div class="webhook-info">
          <span class="webhook-name">${escHtml(webhook.name)}</span>
          <span class="webhook-url" title="${escAttr(webhook.url)}">${escHtml(webhook.url)}</span>
        </div>
        <div class="webhook-btns">
          <button class="btn-sm btn-outline test-wh-btn" data-wh-id="${webhook.id}">Test</button>
          <button class="btn-sm btn-danger del-wh-btn" data-wh-id="${webhook.id}">Delete</button>
        </div>
      </div>`).join('');

  return `<div class="tab-section">
    <p class="section-title" style="margin-top:24px">Webhooks</p>
    <div class="webhook-list">${webhookRows}</div>
    <div class="create-form">
      <h4 style="font-size:14px;font-weight:600;margin-bottom:12px">Add webhook</h4>
      <div class="form-row">
        <input id="newWhName" class="filter-input" placeholder="Name" style="width:150px">
        <input id="newWhUrl" class="filter-input" placeholder="https://\u2026" style="flex:1;min-width:200px">
        <input id="newWhSecret" class="filter-input" placeholder="Secret (optional)" style="width:160px">
        <button class="btn btn-primary" id="addWhBtn" style="width:auto;padding:6px 18px;margin-top:0">Add</button>
      </div>
    </div>

    <p class="section-title" style="margin-top:32px">New Code Period</p>
    <div class="scan-info" style="grid-template-columns:1fr auto;gap:16px;align-items:center">
      <div>
        <div class="info-label">Current strategy</div>
        <div class="info-value" id="ncpDisplay">${ncpStr}</div>
      </div>
      <div class="form-row" style="justify-content:flex-end">
        <select id="ncpStrategy" class="filter-sel">
          <option value="auto"${(!ncp || ncp.strategy === 'auto') ? ' selected' : ''}>Auto</option>
          <option value="previous_version"${ncp?.strategy === 'previous_version' ? ' selected' : ''}>Previous version</option>
          <option value="number_of_days"${ncp?.strategy === 'number_of_days' ? ' selected' : ''}>Number of days</option>
          <option value="reference_branch"${ncp?.strategy === 'reference_branch' ? ' selected' : ''}>Reference branch</option>
        </select>
        <input id="ncpValue" class="filter-input" placeholder="Value (if needed)" style="width:140px" value="${escAttr(ncp?.value || '')}">
        <button class="btn btn-primary" id="saveNcpBtn" style="width:auto;padding:6px 18px;margin-top:0">Save</button>
      </div>
    </div>
  </div>`;
}

function formatNewCodePeriod(ncp) {
  if (!ncp || !ncp.strategy || ncp.scope === 'inherited') return 'auto (default)';
  const value = ncp.value ? ' \u2014 ' + escHtml(ncp.value) : '';
  return escHtml(ncp.strategy) + value;
}

export async function loadProfilesData() {
  const project = state.currentProject;
  try {
    const requests = [apiFetch('/profiles'), apiFetch('/rules')];
    if (project) {
      const projectKey = encodeURIComponent(project.key);
      requests.push(
        apiFetch('/projects/' + projectKey + '/profiles'),
        apiFetch('/projects/' + projectKey + '/profiles/effective'),
      );
    }
    const [profilesData, rulesData, assignmentsData, effectiveData] = await Promise.all(requests);
    state.profilesData = {
      profiles: profilesData.items || (Array.isArray(profilesData) ? profilesData : []),
      rules: rulesData.items || (Array.isArray(rulesData) ? rulesData : []),
      assignments: assignmentsData?.items || (Array.isArray(assignmentsData) ? assignmentsData : []),
      effective: effectiveData?.items || (Array.isArray(effectiveData) ? effectiveData : []),
    };
  } catch {
    state.profilesData = { profiles: [], rules: [], assignments: [], effective: [] };
  }
  renderView();
}

export function renderProfilesTab() {
  const data = state.profilesData;
  if (data === null) return `<div class="loading-state"><div class="spinner"></div></div>`;
  const profiles = Array.isArray(data) ? data : (data.profiles || []);
  const assignments = Array.isArray(data) ? [] : (data.assignments || []);
  const effective = Array.isArray(data) ? [] : (data.effective || []);
  const rules = Array.isArray(data) ? [] : (data.rules || []);
  if (!profiles.length) return `<div class="empty-state" style="padding:40px 0"><p>No quality profiles found.</p></div>`;

  const byLang = {};
  for (const profile of profiles) {
    if (!byLang[profile.language]) byLang[profile.language] = [];
    byLang[profile.language].push(profile);
  }

  const assignmentByLang = Object.fromEntries(assignments.map(item => [item.language, item]));
  const effectiveByLang = Object.fromEntries(effective.map(item => [item.language, item]));
  const ruleByKey = Object.fromEntries(rules.map(rule => [rule.key, rule]));

  const sections = Object.entries(byLang).map(([lang, profs]) => `
    <div class="profile-lang-section">
      <h4 class="profile-lang-title">${escHtml(lang)}</h4>
      ${renderEffectiveProfileSummary(lang, assignmentByLang[lang], effectiveByLang[lang], ruleByKey)}
      <div class="profile-list">
        ${profs.map(profile => `
          <div class="profile-row">
            <div class="profile-info">
              <span class="profile-name">${escHtml(profile.name)}</span>
              ${profile.is_builtin ? `<span class="badge badge-ok" style="font-size:10px;margin-left:6px">Built-in</span>` : ''}
              ${profile.is_default ? `<span class="badge badge-warn" style="font-size:10px;margin-left:6px">Default</span>` : ''}
              ${profile.parser_only ? `<span class="badge" style="font-size:10px;margin-left:6px">Parser only</span>` : ''}
              <span style="color:var(--text-muted);font-size:12px;margin-left:8px">${profile.rule_count || 0} active rules</span>
            </div>
            <div class="profile-actions">
              <button class="btn-sm btn-outline export-profile-btn" data-profile-id="${profile.id}" data-profile-name="${escAttr(profile.name)}">Export</button>
              <button class="btn-sm btn-outline assign-profile-btn"
                data-profile-id="${profile.id}"
                data-profile-lang="${escAttr(profile.language)}"
                data-profile-name="${escAttr(profile.name)}">Assign to project</button>
            </div>
          </div>`).join('')}
      </div>
    </div>`).join('');

  return `<div class="tab-section">
    <p class="section-title" style="margin-top:24px">Quality Profiles</p>
    <p style="color:var(--text-muted);font-size:13px;margin-bottom:16px">Profiles define which rules are active for each language.</p>
    ${sections}
  </div>`;
}

export async function loadCustomRulesData() {
  try {
    const [rulesData, enginesData, aiData] = await Promise.all([
      apiFetch('/custom-rules'),
      apiFetch('/rule-engines'),
      apiFetch('/custom-rules/ai/models'),
    ]);
    state.customRulesData = rulesData.items || (Array.isArray(rulesData) ? rulesData : []);
    state.customRuleEngines = Array.isArray(enginesData) ? enginesData : [];
    state.customRuleAIProviders = aiData.providers || aiData.items || [];
  } catch {
    state.customRulesData = [];
    state.customRuleEngines = [];
    state.customRuleAIProviders = [];
  }
  renderView();
}

export async function loadAIProvidersData() {
  try {
    const aiData = await apiFetch('/custom-rules/ai/models');
    state.customRuleAIProviders = aiData.providers || aiData.items || [];
  } catch {
    state.customRuleAIProviders = [];
  }
  renderView();
}

export function renderAIProvidersTab() {
  const providers = state.customRuleAIProviders;
  if (providers === null) return `<div class="loading-state"><div class="spinner"></div></div>`;
  const items = providers || [];
  const selectedProvider = state.customRuleAISetupProvider || '';
  const connected = items.filter(provider => provider.status === 'connected' || provider.configured).length;
  const local = items.filter(provider => provider.local).length;
  const cloud = items.length - local;

  return `<div class="tab-section ai-providers-page">
    <div class="rule-studio-hero ai-providers-hero">
      <div class="rule-studio-title-block">
        <span class="rule-studio-eyebrow">Integrations</span>
        <div class="rule-studio-title-row">
          <p class="section-title">AI Providers</p>
          <span>${fmtNum(items.length)} providers</span>
        </div>
      </div>
      <div class="rule-studio-summary">
        ${ruleStudioStat('Connected', connected, connected ? 'ok' : 'warn')}
        ${ruleStudioStat('Local', local)}
        ${ruleStudioStat('Cloud', cloud)}
        ${ruleStudioStat('Models', customRuleAIModels(items).length)}
      </div>
      <button class="btn-sm btn-outline" id="refreshAIProvidersBtn" type="button">Refresh</button>
    </div>
    <section class="rule-builder-card ai-providers-workbench">
      <div class="rule-builder-head">
        <span>Provider setup</span>
        <h4>Connect models for Rule Studio</h4>
      </div>
      <div class="custom-rule-ai-provider-list">
        ${items.length ? items.map(provider => renderCustomRuleAIProviderCard(provider, provider.id === selectedProvider)).join('') : '<div class="empty-state compact"><p>No AI providers are configured.</p></div>'}
      </div>
      <div class="custom-rule-ai-actions">
        <button class="btn-sm btn-outline" id="returnRuleStudioBtn" type="button">Back to Rule Studio</button>
      </div>
    </section>
  </div>`;
}

export function renderCustomRulesTab() {
  const rules = state.customRulesData;
  if (rules === null) return `<div class="loading-state"><div class="spinner"></div></div>`;
  const engines = state.customRuleEngines || [];
  const aiProviders = state.customRuleAIProviders || [];
  const filters = state.customRuleFilters || { search: '', lifecycle: 'all' };
  const allRules = rules || [];
  const visibleRules = allRules.filter(rule => customRuleMatchesFilters(rule, filters));
  const editingId = state.editingCustomRuleId;
  const editingRule = editingId ? allRules.find(rule => rule.id === editingId) : null;
  const builderOpen = editingId != null || state.builderOpen;

  return `<div class="tab-section">
    <p class="section-title" style="margin-top:24px">Rule Studio</p>
    <p style="color:var(--text-muted);font-size:13px;margin-bottom:16px">Create custom rules to enforce project-specific patterns.</p>
    ${renderCustomRuleBuilder(engines, aiProviders, editingRule, builderOpen)}
    <div class="rule-studio-toolbar" style="display:flex;gap:10px;align-items:center;margin-bottom:16px;flex-wrap:wrap">
      <span style="font-weight:600;font-size:13px">${fmtNum(visibleRules.length)} of ${fmtNum(allRules.length)} rules</span>
      <input id="customRuleSearch" class="filter-input" placeholder="Search rules" value="${escAttr(filters.search || '')}" style="width:200px">
      <select id="customRuleLifecycleFilter" class="filter-sel">
        ${customRuleLifecycleOptions(filters.lifecycle || 'all')}
      </select>
      <button class="btn-sm btn-outline" id="refreshCustomRulesBtn" type="button">Refresh</button>
    </div>
    ${visibleRules.length ? visibleRules.map(rule => renderCustomRuleRow(rule)).join('') : renderRuleStudioEmpty(allRules.length)}
  </div>`;
}

function renderRuleStudioEmpty(hasRules) {
  if (hasRules) return '<p style="color:var(--text-muted);padding:20px 0">No rules match the current filters.</p>';
  return `<div class="empty-state" style="padding:40px 0">
    <p>No custom rules yet.</p>
    <button class="btn-sm btn-outline" id="focusRuleBuilderBtn" type="button">Create your first rule</button>
  </div>`;
}

function renderCustomRuleRow(rule) {
  const lifecycle = rule.lifecycle || 'draft';
  const status = rule.validation_status || 'none';
  const expanded = state.expandedCustomRuleId === rule.id;
  const editing = state.editingCustomRuleId === rule.id;
  return `<article class="custom-rule-row${expanded ? ' expanded' : ''}${editing ? ' editing' : ''}" data-rule-id="${rule.id}">
    <div class="custom-rule-row-header">
      <button type="button" class="custom-rule-row-toggle" data-rule-id="${rule.id}" aria-expanded="${expanded ? 'true' : 'false'}">
        <span class="custom-rule-row-chevron" aria-hidden="true">${expanded ? '\u25BE' : '\u25B8'}</span>
        <span class="custom-rule-name">${escHtml(rule.name || rule.key)}</span>
        <span class="mono" style="font-size:11px;color:var(--text-muted)">${escHtml(rule.key)}</span>
      </button>
      <div class="custom-rule-badges">
        <span>${escHtml(rule.language || '-')}</span>
        <span class="badge ${customRuleLifecycleClass(lifecycle)}">${escHtml(lifecycle)}</span>
        <span class="badge ${customRuleStatusClass(status)}">${escHtml(status)}</span>
      </div>
      <div class="custom-rule-row-actions">
        <button class="btn-sm btn-outline edit-custom-rule-btn" data-rule-id="${rule.id}"${editing ? ' disabled' : ''}>Edit</button>
        ${lifecycle === 'draft' && status === 'passed' ? `<button class="btn-sm btn-primary publish-custom-rule-btn" data-rule-id="${rule.id}">Publish</button>` : ''}
        ${expanded ? `<button class="btn-sm btn-ghost disable-custom-rule-btn" data-rule-id="${rule.id}">Disable</button>` : ''}
      </div>
    </div>
    ${expanded ? renderCustomRuleDetails(rule) : ''}
  </article>`;
}

function customRuleLifecycleOptions(selected) {
  return [
    ['all', 'All'],
    ['published', 'Published'],
    ['draft', 'Draft'],
    ['disabled', 'Disabled'],
  ].map(([value, label]) => `<option value="${escAttr(value)}"${selected === value ? ' selected' : ''}>${escHtml(label)}</option>`).join('');
}

function ruleStudioStat(label, value, tone = '') {
  return `<div class="rule-studio-stat ${tone}"><span>${escHtml(label)}</span><strong>${fmtNum(value)}</strong></div>`;
}

function customRuleMatchesFilters(rule, filters) {
  const lifecycle = rule.lifecycle || 'draft';
  if (filters.lifecycle && filters.lifecycle !== 'all' && lifecycle !== filters.lifecycle) {
    return false;
  }
  const query = (filters.search || '').trim().toLowerCase();
  if (!query) {
    return true;
  }
  return [rule.key, rule.name, rule.language, rule.engine, rule.type, rule.severity, rule.pack_name]
    .filter(Boolean)
    .some(value => String(value).toLowerCase().includes(query));
}

function renderCustomRuleDetails(rule) {
  const engineConfig = rule.engine_config || {};
  const examples = Array.isArray(rule.examples) ? rule.examples : [];
  const compliantExample = examples.find(ex => ex.compliant);
  const noncompliantExample = examples.find(ex => !ex.compliant);
  const tags = Array.isArray(rule.tags) ? rule.tags : [];
  const created = rule.created_at ? new Date(rule.created_at).toLocaleString() : '-';
  const updated = rule.updated_at ? new Date(rule.updated_at).toLocaleString() : '-';
  const published = rule.published_at ? new Date(rule.published_at).toLocaleString() : '';
  const matcherRows = Object.entries(engineConfig)
    .filter(([, value]) => value !== undefined && value !== null && value !== '')
    .map(([key, value]) => `<dt>${escHtml(key)}</dt><dd><code class="mono custom-rule-detail-code">${escHtml(String(value))}</code></dd>`)
    .join('');
  return `<div class="custom-rule-detail">
    <div class="custom-rule-detail-grid">
      <dl class="custom-rule-detail-list">
        <dt>Pack</dt><dd>${escHtml(rule.pack_name || '-')}</dd>
        <dt>Version</dt><dd>${escHtml(String(rule.version || 1))}</dd>
        <dt>Message</dt><dd>${escHtml(rule.message || '-')}</dd>
        <dt>Description</dt><dd>${escHtml(rule.description || '-')}</dd>
        <dt>Tags</dt><dd>${tags.length ? tags.map(tag => `<span class="badge badge-soft">${escHtml(tag)}</span>`).join(' ') : '-'}</dd>
        <dt>Created</dt><dd>${escHtml(created)}</dd>
        <dt>Updated</dt><dd>${escHtml(updated)}</dd>
        ${published ? `<dt>Published</dt><dd>${escHtml(published)}</dd>` : ''}
        <dt>Validation hash</dt><dd><code class="mono custom-rule-detail-code">${escHtml(rule.validation_hash || '(none)')}</code></dd>
      </dl>
      <dl class="custom-rule-detail-list">
        <dt>Engine</dt><dd>${escHtml(rule.engine || '-')}</dd>
        ${matcherRows || '<dt>Matcher</dt><dd>-</dd>'}
      </dl>
    </div>
    ${noncompliantExample ? `<div class="custom-rule-detail-block"><p class="custom-rule-detail-label">Noncompliant example</p><pre class="custom-rule-detail-code-block">${escHtml(noncompliantExample.code || '')}</pre></div>` : ''}
    ${compliantExample ? `<div class="custom-rule-detail-block"><p class="custom-rule-detail-label">Compliant example</p><pre class="custom-rule-detail-code-block">${escHtml(compliantExample.code || '')}</pre></div>` : ''}
  </div>`;
}

function renderCustomRuleDiagnostic(item) {
  return `<div class="custom-rule-diagnostic ${escAttr(item.level)}"><span>${escHtml(item.code)}</span>${escHtml(item.message)}</div>`;
}

function renderCustomRuleBuilder(engines, aiProviders, editingRule, open) {
  const engineList = engines.length ? engines : [
    { engine: 'text', name: 'Text pattern' },
    { engine: 'go-ast', name: 'Go AST pattern' },
    { engine: 'tree-sitter', name: 'Tree-sitter query' },
  ];
  const selectedEngine = editingRule?.engine || customRuleDefaultEngine(engineList);
  const engineOptions = engineList
    .map(item => `<option value="${escAttr(item.engine)}"${item.engine === selectedEngine ? ' selected' : ''}>${escHtml(item.name || item.engine)}</option>`).join('');
  const headerTitle = editingRule ? 'Edit rule' : 'New rule';
  const submitLabel = editingRule ? 'Save changes' : 'Create draft';
  const cancelButton = editingRule ? `<button class="btn-sm btn-outline" id="cancelEditCustomRuleBtn" type="button">Cancel</button>` : '';
  const prefill = editingRule || {};

  return `<details class="rule-builder-details" id="customRuleBuilder"${open ? ' open' : ''}>
    <summary class="rule-builder-summary"><h4>${escHtml(headerTitle)}</h4></summary>
    <div class="rule-builder-body">
      <div class="custom-rule-form-grid">
        <label class="custom-rule-field"><span>Name</span><input id="customRuleName" class="filter-input" placeholder="Rule name" value="${escAttr(prefill.name || '')}"></label>
        <label class="custom-rule-field"><span>Rule key</span><input id="customRuleKey" class="filter-input" placeholder="no-console-log" value="${escAttr(prefill.rule_id || '')}"></label>
        <label class="custom-rule-field"><span>Namespace</span><input id="customRuleNamespace" class="filter-input" value="${escAttr(prefill.namespace || 'custom')}"></label>
        <label class="custom-rule-field"><span>Language</span><select id="customRuleLanguage" class="filter-sel">
          <option value="go"${(prefill.language || 'go') === 'go' ? ' selected' : ''}>Go</option>
          <option value="javascript"${prefill.language === 'javascript' ? ' selected' : ''}>JavaScript</option>
          <option value="typescript"${prefill.language === 'typescript' ? ' selected' : ''}>TypeScript</option>
          <option value="python"${prefill.language === 'python' ? ' selected' : ''}>Python</option>
          <option value="rust"${prefill.language === 'rust' ? ' selected' : ''}>Rust</option>
        </select></label>
        <label class="custom-rule-field"><span>Type</span><select id="customRuleType" class="filter-sel">
          <option value="code_smell"${(prefill.type || 'code_smell') === 'code_smell' ? ' selected' : ''}>Code Smell</option>
          <option value="bug"${prefill.type === 'bug' ? ' selected' : ''}>Bug</option>
          <option value="vulnerability"${prefill.type === 'vulnerability' ? ' selected' : ''}>Vulnerability</option>
        </select></label>
        <label class="custom-rule-field"><span>Severity</span><select id="customRuleSeverity" class="filter-sel">
          <option value="major"${(prefill.severity || 'major') === 'major' ? ' selected' : ''}>Major</option>
          <option value="critical"${prefill.severity === 'critical' ? ' selected' : ''}>Critical</option>
          <option value="minor"${prefill.severity === 'minor' ? ' selected' : ''}>Minor</option>
          <option value="info"${prefill.severity === 'info' ? ' selected' : ''}>Info</option>
        </select></label>
        <label class="custom-rule-field"><span>Engine</span><select id="customRuleEngine" class="filter-sel">${engineOptions}</select></label>
        <label class="custom-rule-field full"><span>Message</span><input id="customRuleMessage" class="filter-input" placeholder="Issue message" value="${escAttr(prefill.message || '')}"></label>
      </div>
      <p class="custom-rule-field-hint" id="customRuleLanguageHint" hidden>Go AST rules are Go-only.</p>
      <div class="rule-section-label">Pattern</div>
      <div class="custom-rule-engine-guide" id="customRuleEngineGuide">Regular expression against source text.</div>
      <div class="custom-rule-form-grid">
        <label class="custom-rule-field full" data-engine-field="text"${selectedEngine === 'text' ? '' : ' hidden'}><input id="customRuleTextPattern" class="filter-input" placeholder="debugger|TODO|panic" value="${escAttr((prefill.engine_config || prefill).text_pattern || (prefill.engine_config || {}).pattern || '')}"></label>
        <label class="custom-rule-field" data-engine-field="go-ast"${selectedEngine === 'go-ast' ? '' : ' hidden'}><select id="customRuleGoASTPattern" class="filter-sel">
          <option value="forbidden_call">Forbidden call</option>
          <option value="forbidden_import">Forbidden import</option>
        </select></label>
        <label class="custom-rule-field full" data-engine-field="go-ast"${selectedEngine === 'go-ast' ? '' : ' hidden'}><input id="customRuleTarget" class="filter-input" placeholder="fmt.Println or net/http"></label>
        <label class="custom-rule-field full" data-engine-field="tree-sitter"${selectedEngine === 'tree-sitter' ? '' : ' hidden'}><textarea id="customRuleQuery" class="filter-input" style="min-height:80px" placeholder="(call_expression function: (identifier) @name)"></textarea></label>
      </div>
      <div class="rule-section-label">Examples</div>
      <div class="custom-rule-form-grid">
        <label class="custom-rule-field full"><span>Noncompliant code</span><textarea id="customRuleExample" class="filter-input" style="min-height:60px" placeholder="code that should produce an issue">${escHtml((prefill.examples || []).find(e => !e?.compliant)?.code || prefill.noncompliant_example || '')}</textarea></label>
        <label class="custom-rule-field full"><span>Compliant code</span><textarea id="customRuleCompliantExample" class="filter-input" style="min-height:60px" placeholder="code that should pass">${escHtml((prefill.examples || []).find(e => e?.compliant)?.code || prefill.compliant_example || '')}</textarea></label>
      </div>
      <div class="rule-builder-actions">
        <button class="btn btn-primary" id="createCustomRuleBtn" type="button">${escHtml(submitLabel)}</button>
        ${cancelButton}
        ${!editingRule ? `<button class="btn-sm btn-ghost" id="closeRuleBuilderBtn" type="button">Close</button>` : ''}
      </div>
    </div>
  </details>`;
}

function renderCustomRuleAIAssist(providers) {
  const models = customRuleAIModels(providers);
  const hasModels = models.length > 0;
  const modelOptions = customRuleAIModelOptions(models);
  const selected = models.find(model => model.selected) || models[0] || null;
  const canGenerate = selected?.status === 'connected';
  const status = selected ? customRuleAIModelMessage(selected) : 'No AI providers are available yet.';
  const setupLabel = selected ? customRuleAIStateLabel(selected) || 'setup required' : 'setup required';
  return `<div class="rule-builder-section custom-rule-ai-panel">
    <p>AI assist</p>
    <div class="custom-rule-ai-grid">
      <label class="custom-rule-field"><span>Model</span><select id="customRuleAIModel" class="filter-sel"${hasModels ? '' : ' disabled'}>${modelOptions}</select></label>
      <label class="custom-rule-field full"><span>Intent</span><textarea id="customRuleAIIntent" class="filter-input custom-rule-ai-prompt" placeholder="Flag debug logging in production code"></textarea></label>
    </div>
    <div class="custom-rule-ai-actions">
      <button class="btn-sm btn-outline" id="generateCustomRuleAIBtn" type="button"${canGenerate ? '' : ' disabled'}>Generate draft</button>
      <span id="customRuleAIStatus">${escHtml(status)}</span>
      <a class="custom-rule-ai-setup-link" id="connectCustomRuleAIProviderBtn" href="#custom-rule-ai-provider-setup" role="button" data-provider="${escAttr(selected?.provider || '')}"${canGenerate || !selected ? ' hidden' : ''}>${escHtml(setupLabel)}</a>
    </div>
    ${renderCustomRuleAIProviderSetup(providers)}
  </div>`;
}

function customRuleAIModels(providers) {
  const models = [];
  providers.forEach(provider => {
    const options = provider.model_options || (provider.models || []).map(model => ({ id: model, label: model, status: provider.status || (provider.configured ? 'connected' : 'setup_required') }));
    options.forEach(option => {
      const status = option.status || provider.status || (provider.configured ? 'connected' : 'setup_required');
      models.push({
        provider: provider.id,
        providerLabel: provider.label || provider.id,
        model: option.id || option.label,
        label: option.label || option.id,
        status,
        local: Boolean(option.local || provider.local),
        setupRequired: Boolean(option.setup_required || provider.setup_required || status !== 'connected'),
        setupURL: provider.setup_url || '#custom-rule-ai-provider-setup',
        message: option.message || provider.message || '',
        defaultModel: provider.default_model,
      });
    });
  });
  const firstConnected = models.findIndex(model => model.status === 'connected' && (!model.defaultModel || model.model === model.defaultModel));
  const fallbackConnected = models.findIndex(model => model.status === 'connected');
  let selectedIndex = 0;
  if (firstConnected >= 0) {
    selectedIndex = firstConnected;
  } else if (fallbackConnected >= 0) {
    selectedIndex = fallbackConnected;
  }
  return models.map((model, index) => ({ ...model, selected: index === selectedIndex }));
}

function customRuleAIModelOptions(models) {
  if (!models.length) {
    return '<option value="">No AI providers available</option>';
  }
  return models.map(model => {
    const state = customRuleAIStateLabel(model);
    const selected = model.selected ? ' selected' : '';
    return `<option value="${escAttr(model.provider + '|' + model.model)}" data-provider="${escAttr(model.provider)}" data-provider-label="${escAttr(model.providerLabel)}" data-model="${escAttr(model.model)}" data-status="${escAttr(model.status)}" data-local="${model.local ? 'true' : 'false'}" data-setup-url="${escAttr(model.setupURL)}" data-message="${escAttr(customRuleAIModelMessage(model))}"${selected}>${escHtml(model.providerLabel)} / ${escHtml(model.label)}${state ? ' - ' + escHtml(state) : ''}</option>`;
  }).join('');
}

function customRuleAIStateLabel(model) {
  if (model.status === 'connected') return model.local ? 'local' : 'connected';
  if (model.status === 'setup_required') return 'setup required';
  if (model.status === 'unavailable') return 'unavailable';
  return model.status || '';
}

function customRuleAIModelMessage(model) {
  if (model.message) return model.message;
  if (model.status === 'connected') return model.local ? 'Local model ready.' : 'Model ready.';
  if (model.status === 'setup_required') return 'Set up this provider before generating drafts.';
  if (model.status === 'unavailable') return 'This provider is currently unavailable.';
  return '';
}

function renderCustomRuleAIProviderSetup(providers) {
  if (!providers.length) return '';
  return `<div class="custom-rule-ai-provider-setup" id="customRuleAIProviderSetup" hidden>
    <p class="custom-rule-ai-provider-title">AI provider setup</p>
    <div class="custom-rule-ai-provider-list">
      ${providers.map(renderCustomRuleAIProviderCard).join('')}
    </div>
  </div>`;
}

function renderCustomRuleAIProviderCard(provider, active = false) {
  const status = provider.status || (provider.configured ? 'connected' : 'setup_required');
  const models = provider.models || [];
  const diagnostics = provider.diagnostics || [];
  const modelCountText = models.length === 1 ? '1 model available.' : fmtNum(models.length) + ' models available.';
  return `<article class="custom-rule-ai-provider-card${active ? ' active' : ''}" data-ai-provider-card="${escAttr(provider.id)}">
    <div>
      <strong>${escHtml(provider.label || provider.id)}</strong>
      <span class="badge ${status === 'connected' ? 'badge-ok' : 'badge-warn'}">${escHtml(customRuleAIStateLabel({ status, local: provider.local }))}</span>
    </div>
    <p>${escHtml(provider.message || 'Configure this provider to use it from Rule Studio.')}</p>
    ${provider.base_url ? `<p class="mono">${escHtml(provider.base_url)}</p>` : ''}
    ${models.length ? `<p>${modelCountText}</p><p class="mono">${escHtml(models.join(', '))}</p>` : ''}
    ${diagnostics.length ? `<p class="custom-rule-ai-provider-diagnostic">${escHtml(diagnostics[0])}</p>` : ''}
  </article>`;
}

function customRuleDefaultEngine(engines) {
  if (engines.some(item => item.engine === 'text')) return 'text';
  return engines[0]?.engine || 'text';
}

function customRuleFullKey(namespace, key) {
  const cleanNamespace = (namespace || 'custom').trim().toLowerCase() || 'custom';
  const cleanKey = (key || '').trim().toLowerCase();
  if (!cleanKey) return cleanNamespace + ':<rule-id>';
  if (cleanKey.includes(':')) return cleanKey;
  return cleanNamespace + ':' + cleanKey;
}

function customRuleEngineGuide(engine) {
  if (engine === 'go-ast') return 'Choose a built-in Go AST matcher and the exact call or import to flag.';
  if (engine === 'tree-sitter') return 'Write a structural Tree-sitter query for advanced language-specific matches.';
  return 'Use a regular expression against source text.';
}

function bindCustomRuleBuilderControls() {
  const engineSelect = document.getElementById('customRuleEngine');
  if (!engineSelect) return;
  engineSelect.addEventListener('change', syncCustomRuleBuilder);
  document.getElementById('customRuleAIModel')?.addEventListener('change', syncCustomRuleAIControls);
  document.getElementById('connectCustomRuleAIProviderBtn')?.addEventListener('click', openCustomRuleAIProviderSetup);
  document.getElementById('customRuleNamespace')?.addEventListener('input', syncCustomRuleBuilder);
  document.getElementById('customRuleKey')?.addEventListener('input', syncCustomRuleBuilder);
  syncCustomRuleBuilder();
  syncCustomRuleAIControls();
}

function syncCustomRuleBuilder() {
  const engine = document.getElementById('customRuleEngine')?.value || 'text';
  document.querySelectorAll('[data-engine-field]').forEach(field => {
    const engines = (field.dataset.engineField || '').split(/\s+/).filter(Boolean);
    const visible = engines.includes(engine);
    field.hidden = !visible;
    field.querySelectorAll('input, textarea, select').forEach(control => {
      control.disabled = !visible;
    });
  });

  const language = document.getElementById('customRuleLanguage');
  const languageHint = document.getElementById('customRuleLanguageHint');
  if (language) {
    if (engine === 'go-ast') {
      language.value = 'go';
      language.disabled = true;
      if (languageHint) languageHint.hidden = false;
    } else {
      language.disabled = false;
      if (languageHint) languageHint.hidden = true;
    }
  }

  const guide = document.getElementById('customRuleEngineGuide');
  if (guide) guide.textContent = customRuleEngineGuide(engine);

  const preview = document.getElementById('customRuleKeyPreview');
  if (preview) {
    const namespace = document.getElementById('customRuleNamespace')?.value;
    const key = document.getElementById('customRuleKey')?.value;
    preview.textContent = customRuleFullKey(namespace, key);
  }
}

function syncCustomRuleAIControls() {
  const select = document.getElementById('customRuleAIModel');
  const option = select?.selectedOptions?.[0];
  const generate = document.getElementById('generateCustomRuleAIBtn');
  const connect = document.getElementById('connectCustomRuleAIProviderBtn');
  const status = document.getElementById('customRuleAIStatus');
  const modelStatus = option?.dataset.status || '';
  const canGenerate = modelStatus === 'connected';
  if (generate) generate.disabled = !canGenerate;
  if (connect) {
    connect.hidden = canGenerate || !option?.dataset.provider;
    connect.dataset.provider = option?.dataset.provider || '';
    connect.textContent = customRuleAIStateLabel({ status: modelStatus, local: option?.dataset.local === 'true' }) || 'setup required';
    connect.setAttribute?.('aria-label', 'Open AI provider setup for ' + (option?.dataset.providerLabel || option?.dataset.provider || 'selected model'));
  }
  if (status) status.textContent = option?.dataset.message || '';
}

function openCustomRuleAIProviderSetup(event) {
  event?.preventDefault?.();
  const select = document.getElementById('customRuleAIModel');
  const option = select?.selectedOptions?.[0];
  const provider = option?.dataset.provider || event?.currentTarget?.dataset?.provider;
  if (provider) state.customRuleAISetupProvider = provider;
  const aiProvidersTab = document.querySelector('.tab-btn[data-tab="ai-providers"]');
  if (aiProvidersTab) {
    aiProvidersTab.click();
    return;
  }
  const panel = document.getElementById('customRuleAIProviderSetup');
  if (!panel) return;
  panel.hidden = false;
  panel.querySelectorAll('[data-ai-provider-card]').forEach(card => {
    card.classList.toggle('active', card.dataset.aiProviderCard === provider);
  });
  panel.scrollIntoView?.({ behavior: 'smooth', block: 'nearest' });
}

function readCustomRuleBuilderDraft() {
  return {
    pack_name: document.getElementById('customRulePack')?.value.trim() || 'Rule Studio',
    namespace: document.getElementById('customRuleNamespace')?.value.trim() || 'custom',
    rule_id: document.getElementById('customRuleKey')?.value.trim() || '',
    name: document.getElementById('customRuleName')?.value.trim() || '',
    language: document.getElementById('customRuleLanguage')?.value || 'go',
    type: document.getElementById('customRuleType')?.value || 'code_smell',
    severity: document.getElementById('customRuleSeverity')?.value || 'major',
    engine: document.getElementById('customRuleEngine')?.value || 'text',
    text_pattern: document.getElementById('customRuleTextPattern')?.value.trim() || '',
    go_ast_pattern: document.getElementById('customRuleGoASTPattern')?.value || 'forbidden_call',
    target: document.getElementById('customRuleTarget')?.value.trim() || '',
    tree_sitter_query: document.getElementById('customRuleQuery')?.value.trim() || '',
    noncompliant_example: document.getElementById('customRuleExample')?.value || '',
    compliant_example: document.getElementById('customRuleCompliantExample')?.value || '',
    message: document.getElementById('customRuleMessage')?.value.trim() || '',
  };
}

function applyCustomRuleAISuggestion(suggestion) {
  if (!suggestion) return;
  setInputValue('customRulePack', suggestion.pack_name);
  applyCustomRuleID(suggestion.rule_id || suggestion.key);
  setInputValue('customRuleName', suggestion.name);
  setInputValue('customRuleType', suggestion.type);
  setInputValue('customRuleSeverity', suggestion.severity);
  setInputValue('customRuleEngine', suggestion.engine);
  syncCustomRuleBuilder();
  setInputValue('customRuleLanguage', suggestion.language);
  setInputValue('customRuleTextPattern', suggestion.text_pattern || suggestion.pattern);
  setInputValue('customRuleGoASTPattern', suggestion.go_ast_pattern);
  setInputValue('customRuleTarget', suggestion.target);
  setInputValue('customRuleQuery', suggestion.tree_sitter_query || suggestion.query);
  setInputValue('customRuleExample', suggestion.noncompliant_example);
  setInputValue('customRuleCompliantExample', suggestion.compliant_example);
  setInputValue('customRuleMessage', suggestion.message);
  syncCustomRuleBuilder();
}

function applyCustomRuleID(ruleID) {
  const value = (ruleID || '').trim();
  if (!value) return;
  const separator = value.indexOf(':');
  if (separator > 0) {
    setInputValue('customRuleNamespace', value.slice(0, separator));
    setInputValue('customRuleKey', value.slice(separator + 1));
    return;
  }
  setInputValue('customRuleKey', value);
}

function setInputValue(id, value) {
  if (value === undefined || value === null || value === '') return;
  const input = document.getElementById(id);
  if (input) input.value = value;
}

// forceInputValue mirrors setInputValue but writes empty strings too.
// Used when prefilling the builder from an existing rule, so that fields
// not present in the source are reset rather than left dirty.
function forceInputValue(id, value) {
  const input = document.getElementById(id);
  if (!input) return;
  input.value = value === undefined || value === null ? '' : value;
}

function prefillCustomRuleBuilder(rule) {
  if (!rule) return;
  const fullKey = rule.key || '';
  const separator = fullKey.indexOf(':');
  const namespace = separator >= 0 ? fullKey.slice(0, separator) : 'custom';
  const ruleID = separator >= 0 ? fullKey.slice(separator + 1) : fullKey;
  setInputValue('customRuleNamespace', namespace);
  setInputValue('customRuleKey', ruleID);
  setInputValue('customRuleName', rule.name || '');
  setInputValue('customRuleType', rule.type || 'code_smell');
  setInputValue('customRuleSeverity', rule.severity || 'major');
  setInputValue('customRuleEngine', rule.engine || 'text');
  setInputValue('customRuleLanguage', rule.language || 'go');
  setInputValue('customRuleMessage', rule.message || '');
  const cfg = rule.engine_config || {};
  setInputValue('customRuleTextPattern', cfg.pattern || '');
  setInputValue('customRuleGoASTPattern', cfg.pattern || 'forbidden_call');
  setInputValue('customRuleTarget', cfg.target || '');
  setInputValue('customRuleQuery', cfg.query || '');
  const examples = Array.isArray(rule.examples) ? rule.examples : [];
  setInputValue('customRuleExample', examples.find(ex => !ex.compliant)?.code || '');
  setInputValue('customRuleCompliantExample', examples.find(ex => ex.compliant)?.code || '');
  syncCustomRuleBuilder();
}

function renderCustomRuleImport() {
  return `<details class="rule-import-drawer custom-rule-import">
    <summary>Import pack</summary>
    <label class="custom-rule-field full"><span>Rule pack</span><textarea id="customRuleImportText" class="filter-input custom-rule-import-textarea" placeholder="JSON or YAML rule pack"></textarea></label>
    <button class="btn-sm btn-outline" id="importCustomRulesBtn" type="button">Import</button>
  </details>`;
}

function customRuleLifecycleClass(lifecycle) {
  if (lifecycle === 'published' || lifecycle === 'valid') return 'badge-ok';
  if (lifecycle === 'invalid' || lifecycle === 'disabled') return 'badge-warn';
  return '';
}

function customRuleStatusClass(status) {
  if (status === 'passed') return 'badge-ok';
  if (status === 'failed' || status === 'requires_runtime') return 'badge-warn';
  return '';
}

function renderEffectiveProfileSummary(lang, assignment, effective, ruleByKey) {
  const profile = assignment?.profile;
  const source = effective?.source || assignment?.source || 'default';
  const activeRules = effective?.rules || [];
  const parserOnly = effective?.parser_only || profile?.parser_only;
  const hash = effective?.rules_hash ? effective.rules_hash.slice(0, 10) : '-';
  const profileName = effective?.profile_name || profile?.name || 'No profile';
  const sourceBadge = source === 'assigned' ? 'badge-ok' : 'badge-warn';
  const topRules = activeRules.slice(0, 8);
  const remainingRules = activeRules.length - topRules.length;
  const remainingRow = remainingRules > 0 ? `<div class="profile-rule-row muted">${fmtNum(remainingRules)} more rules</div>` : '';
  const emptyText = parserOnly ? 'Parser available; no bundled rules yet.' : 'No active rules.';
  const rulesBlock = topRules.length
    ? `<div class="profile-rule-table">
      ${topRules.map(rule => renderEffectiveRule(rule, ruleByKey)).join('')}
      ${remainingRow}
    </div>`
    : `<div class="profile-rule-empty">${emptyText}</div>`;
  return `<div class="profile-effective-box">
    <div class="profile-effective-head">
      <div>
        <span class="profile-active-label">Active profile</span>
        <strong>${escHtml(profileName)}</strong>
        <span class="badge ${sourceBadge}" style="font-size:10px;margin-left:6px">${escHtml(source)}</span>
        ${parserOnly ? `<span class="badge" style="font-size:10px;margin-left:6px">Parser only</span>` : ''}
      </div>
      <div class="profile-effective-metrics">
        <span>${fmtNum(effective?.active_rule_count || activeRules.length)} rules</span>
        <span class="mono">${escHtml(hash)}</span>
      </div>
    </div>
    ${rulesBlock}
  </div>`;
}

function renderEffectiveRule(rule, ruleByKey) {
  const meta = ruleByKey[rule.rule_key] || {};
  const tags = Array.isArray(meta.tags) ? meta.tags.slice(0, 3).join(', ') : '';
  return `<div class="profile-rule-row">
    <span class="profile-rule-name">${escHtml(meta.name || rule.rule_key)}</span>
    <span class="profile-rule-meta">${escHtml(rule.severity || meta.severity || '-')}</span>
    <span class="profile-rule-meta">${escHtml(meta.type || '-')}</span>
    <span class="profile-rule-meta">${escHtml(rule.origin || '')}</span>
    <span class="profile-rule-tags">${escHtml(tags)}</span>
  </div>`;
}

const TASK_STATUSES = ['', 'queued', 'running', 'retrying', 'stale', 'failed', 'completed', 'cancelled'];
const TASK_TYPES = ['', 'scan', 'index', 'webhook'];

function currentTaskFilters() {
  const filters = state.backgroundTaskFilters || {};
  const project = state.currentProject;
  return {
    type: filters.type || '',
    status: filters.status || '',
    projectKey: filters.projectKey ?? (project?.key || ''),
    workerId: filters.workerId || '',
    scanId: filters.scanId || '',
    limit: filters.limit || 25,
    offset: filters.offset || 0,
  };
}

function buildTaskQuery(filters) {
  const params = new URLSearchParams();
  if (filters.type) params.set('type', filters.type);
  if (filters.status) params.set('status', filters.status);
  if (filters.projectKey) params.set('project_key', filters.projectKey);
  if (filters.workerId) params.set('worker_id', filters.workerId);
  if (filters.scanId) params.set('scan_id', filters.scanId);
  params.set('limit', String(filters.limit || 25));
  params.set('offset', String(filters.offset || 0));
  return params.toString();
}

export async function loadBackgroundTasksData(options = {}) {
  const filters = { ...currentTaskFilters(), ...options };
  state.backgroundTaskFilters = filters;
  state.loadingBackgroundTasks = true;
  state.backgroundTaskError = '';
  renderView();
  try {
    const query = buildTaskQuery(filters);
    const [list, summary] = await Promise.all([
      apiFetch('/admin/background-tasks?' + query),
      apiFetch('/admin/background-tasks/summary?' + query),
    ]);
    state.backgroundTasksData = list;
    state.backgroundTasksSummary = summary;
  } catch (err) {
    state.backgroundTasksData = { items: [], total: 0, limit: filters.limit, offset: filters.offset };
    state.backgroundTasksSummary = null;
    state.backgroundTaskError = err.message || 'Failed to load background tasks.';
  }
  state.loadingBackgroundTasks = false;
  renderView();
}

export async function loadBackgroundTaskDetail(taskId) {
  if (!taskId) return;
  state.loadingBackgroundTaskDetail = true;
  state.selectedBackgroundTask = null;
  state.backgroundTaskError = '';
  renderView();
  try {
    state.selectedBackgroundTask = await apiFetch('/admin/background-tasks/' + encodeURIComponent(taskId));
  } catch (err) {
    state.backgroundTaskError = err.message || 'Failed to load task details.';
  }
  state.loadingBackgroundTaskDetail = false;
  renderView();
}

export function renderAdminLinksTab() {
  return renderBackgroundTasksPage();
}

export function renderBackgroundTasksPage() {
  const filters = currentTaskFilters();
  const data = state.backgroundTasksData || { items: [], total: 0, limit: filters.limit, offset: filters.offset };
  const items = data.items || [];
  const total = data.total || 0;
  const summary = state.backgroundTasksSummary;
  const selected = state.selectedBackgroundTask;
  const canPrev = (filters.offset || 0) > 0;
  const canNext = (filters.offset || 0) + (filters.limit || 25) < total;
  const globalHeader = state.view === 'background-tasks'
    ? `<div class="page-header background-task-page-title">
        <div>
          <h2>Background Tasks</h2>
          <p>Operational queue for scan intake, indexing, and webhook delivery.</p>
        </div>
        <button class="back-btn" id="backgroundTasksBackBtn">Back to projects</button>
      </div>`
    : '';

  return `${globalHeader}<div class="tab-section background-tasks-page">
    <div class="background-task-header">
      <div>
        <p class="section-title">Queue activity</p>
        <p class="background-task-subtitle">Tasks are grouped by project context first; the technical job id is shown as secondary metadata.</p>
      </div>
      <button class="btn-sm btn-outline" id="refreshBackgroundTasksBtn">Refresh</button>
    </div>
    ${state.backgroundTaskError ? `<div class="error-msg background-task-error">${escHtml(state.backgroundTaskError)}</div>` : ''}
    ${renderTaskSummary(summary)}
    ${renderTaskFilters(filters)}
    <div class="background-task-layout">
      <section class="background-task-table-panel">
        ${state.loadingBackgroundTasks ? `<div class="loading-state"><div class="spinner"></div></div>` : renderTaskTable(items)}
        <div class="background-task-pagination">
          <span>${fmtNum(total)} tasks</span>
          <button class="btn-sm btn-outline" data-task-page="prev"${canPrev ? '' : ' disabled'}>Previous</button>
          <button class="btn-sm btn-outline" data-task-page="next"${canNext ? '' : ' disabled'}>Next</button>
        </div>
      </section>
      <aside class="background-task-detail-panel">
        ${state.loadingBackgroundTaskDetail ? `<div class="loading-state"><div class="spinner"></div></div>` : renderTaskDetail(selected)}
      </aside>
    </div>
  </div>`;
}

function renderTaskSummary(summary) {
  const cards = [
    ['Queued', summary?.queue_depth || 0, 'queued'],
    ['Running', summary?.running_count || 0, 'running'],
    ['Failed', summary?.failed_count || 0, 'failed'],
    ['Stale', summary?.stale_count || 0, 'stale'],
    ['Retrying', summary?.retry_count || 0, 'retrying'],
    ['Recent completions', summary?.recent_completion_count || 0, 'completed'],
  ];
  return `<div class="background-task-summary-grid">
    ${cards.map(([label, value, status]) => `<button class="background-task-summary-card" data-task-status-filter="${escAttr(status)}">
      <span>${escHtml(label)}</span>
      <strong>${fmtNum(value)}</strong>
    </button>`).join('')}
  </div>`;
}

function renderTaskFilters(filters) {
  return `<div class="background-task-filters">
    <label>Type<select class="filter-sel" id="taskTypeFilter">${TASK_TYPES.map(value => `<option value="${escAttr(value)}"${filters.type === value ? ' selected' : ''}>${value ? escHtml(value) : 'All'}</option>`).join('')}</select></label>
    <label>Status<select class="filter-sel" id="taskStatusFilter">${TASK_STATUSES.map(value => `<option value="${escAttr(value)}"${filters.status === value ? ' selected' : ''}>${value ? escHtml(value) : 'All'}</option>`).join('')}</select></label>
    <label>Project<input class="filter-input" id="taskProjectFilter" value="${escAttr(filters.projectKey || '')}" placeholder="project key"></label>
    <label>Scan<input class="filter-input" id="taskScanFilter" value="${escAttr(filters.scanId || '')}" placeholder="scan id"></label>
    <label>Worker<input class="filter-input" id="taskWorkerFilter" value="${escAttr(filters.workerId || '')}" placeholder="worker id"></label>
    <button class="btn-sm btn-primary" id="applyTaskFiltersBtn">Apply</button>
    <button class="btn-sm btn-outline" id="clearTaskFiltersBtn">Clear</button>
  </div>`;
}

function renderTaskTable(items) {
  if (!items.length) return `<div class="empty-state"><p>No background tasks match the current filters.</p></div>`;
  return `<table class="background-task-table">
    <thead><tr><th>Task</th><th>Status</th><th>Type</th><th>Submitted</th><th>Started</th><th>Duration</th><th>Worker</th><th>Scan</th><th>Error</th></tr></thead>
    <tbody>${items.map(task => `<tr class="background-task-row" data-task-id="${escAttr(task.id)}">
      <td>${renderTaskName(task)}</td>
      <td><span class="task-status task-status-${escAttr(task.status)}">${escHtml(task.status)}</span></td>
      <td>${renderTaskType(task)}</td>
      <td>${fmtDate(task.created_at)}</td>
      <td>${fmtDate(task.started_at)}</td>
      <td>${formatSeconds(task.duration_seconds)}</td>
      <td>${escHtml(task.worker_id || '-')}</td>
      <td>${escHtml(task.scan_id || '-')}</td>
      <td class="task-error-cell" title="${escAttr(task.last_error || '')}">${escHtml(task.last_error || '-')}</td>
    </tr>`).join('')}</tbody>
  </table>`;
}

function renderTaskName(task) {
  const primary = task.project_key || (task.project_id ? 'Project #' + task.project_id : task.type + ' task');
  const subtitle = [task.id, task.scan_id ? 'scan #' + task.scan_id : '', task.attempts ? 'attempts ' + task.attempts : ''].filter(Boolean).join(' · ');
  return `<div class="task-name-cell">
    <strong>${escHtml(primary)}</strong>
    <span>${escHtml(subtitle || task.id)}</span>
  </div>`;
}

function renderTaskType(task) {
  const labels = {
    scan: 'Scan processing',
    index: 'Search indexing',
    webhook: 'Webhook delivery',
  };
  return `<span class="task-type-chip task-type-${escAttr(task.type)}">${escHtml(labels[task.type] || task.type)}</span>`;
}

function renderTaskDetail(task) {
  if (!task) return `<div class="empty-state"><p>Select a task to inspect diagnostics and supported actions.</p></div>`;
  const details = task.details || {};
  const detailRows = Object.entries(details).map(([key, value]) => `<div class="info-row"><span>${escHtml(key)}</span><strong>${escHtml(String(value))}</strong></div>`).join('');
  const actions = task.supported_actions || [];
  const scannerParameters = renderScannerParameters(task.scanner_parameters);
  return `<div class="background-task-detail">
    <div class="task-detail-head">
      <div class="task-name-cell">
        <strong>${escHtml(task.project_key || task.id)}</strong>
        <span>${escHtml(task.id)}</span>
      </div>
      <span class="task-status task-status-${escAttr(task.status)}">${escHtml(task.status)}</span>
    </div>
    <div class="scan-info task-detail-grid">
      <div><div class="info-label">Type</div><div class="info-value">${escHtml(task.type)}</div></div>
      <div><div class="info-label">Internal status</div><div class="info-value">${escHtml(task.internal_status)}</div></div>
      <div><div class="info-label">Created</div><div class="info-value">${fmtDate(task.created_at)}</div></div>
      <div><div class="info-label">Started</div><div class="info-value">${fmtDate(task.started_at)}</div></div>
      <div><div class="info-label">Completed</div><div class="info-value">${fmtDate(task.completed_at)}</div></div>
      <div><div class="info-label">Next retry</div><div class="info-value">${fmtDate(task.next_attempt_at)}</div></div>
    </div>
    ${scannerParameters}
    ${task.last_error ? `<div class="task-diagnostic"><span>Last error</span><pre>${escHtml(task.last_error)}</pre></div>` : ''}
    <div class="task-detail-section"><h4>Details</h4>${detailRows || '<p class="muted">No type-specific details.</p>'}</div>
    <div class="task-detail-actions">
      ${actions.map(action => `<button class="btn-sm ${action === 'cancel' ? 'btn-danger' : 'btn-outline'}" data-task-action="${escAttr(action)}" data-task-id="${escAttr(task.id)}">${escHtml(action)}</button>`).join('') || '<span class="muted">No actions available for this state.</span>'}
    </div>
  </div>`;
}

function renderScannerParameters(parameters) {
  if (!parameters || !Object.keys(parameters).length) {
    return `<div class="task-detail-section scanner-params-section">
      <h4>Scanner parameters</h4>
      <p class="muted">Scanner parameters were not captured for this task. Run a new scan with the updated scanner to populate this section.</p>
    </div>`;
  }

  const options = parameters.scanner_options || {};
  const scope = parameters.analysis_scope || {};
  const tests = options.tests || parameters.test_signals || {};
  const rows = [
    ['Config file', options.config_path],
    ['Project directory', options.project_dir],
    ['Project key', options.project_key || scope.project_key],
    ['Sources', options.sources],
    ['Exclusions', options.exclusions],
    ['Format', options.format],
    ['Branch', options.branch || scope.branch],
    ['Commit', options.commit_sha || scope.commit_sha],
    ['Pull request', options.pull_request_key || scope.pull_request_key],
    ['Pull request branch', options.pull_request_branch],
    ['Pull request base', options.pull_request_base || scope.pull_request_base],
    ['Server URL', options.server],
    ['Wait for server job', options.server_wait],
    ['Wait timeout', options.server_wait_timeout],
    ['Wait poll', options.server_wait_poll],
    ['Local UI', options.local_ui],
    ['Local UI bind', options.bind],
    ['Local UI port', options.port],
    ['Debug', options.debug],
  ];

  return `<div class="task-detail-section scanner-params-section">
    <h4>Scanner parameters</h4>
    <div class="scanner-params-grid">${rows.map(([label, value]) => renderParameterRow(label, value)).join('')}</div>
    ${renderTestParameters(tests)}
  </div>`;
}

function renderParameterRow(label, value) {
  return `<div class="info-row"><span>${escHtml(label)}</span><strong>${escHtml(formatParameterValue(value))}</strong></div>`;
}

function renderTestParameters(tests) {
  if (!tests || !Object.keys(tests).length) return '';
  const summary = tests.summary || {};
  const modules = Array.isArray(tests.modules) ? tests.modules : [];
  const rows = [
    ['Tests enabled', tests.enabled ?? summary.enabled],
    ['Mode', tests.mode],
    ['Discover modules', tests.discover],
    ['Run commands', tests.run],
    ['Command policy', tests.command_policy],
    ['Max report age', tests.max_report_age],
    ['Max depth', tests.max_depth],
    ['Max candidates', tests.max_candidates],
    ['Max report bytes', tests.max_report_bytes],
  ];
  const moduleCards = modules.length
    ? `<div class="scanner-test-modules">${modules.map(module => renderTestModuleParameters(module)).join('')}</div>`
    : '';
  return `<div class="scanner-test-params">
    <h5>Test signal parameters</h5>
    <div class="scanner-params-grid">${rows.map(([label, value]) => renderParameterRow(label, value)).join('')}</div>
    ${moduleCards}
  </div>`;
}

function renderTestModuleParameters(module) {
  const rows = [
    ['Root', module.root],
    ['Language', module.language],
    ['Role', module.architecture_role],
    ['Policy', module.test_policy],
    ['Command', module.command],
    ['Artifact root', module.artifact_root],
    ['Report root', module.report_root],
    ['Coverage reports', module.coverage_reports],
    ['Test reports', module.test_reports],
    ['Mutation reports', module.mutation_reports],
    ['Native reports', module.native_reports],
    ['Owner', module.owner],
    ['Team', module.team],
    ['Integration required', module.integration_required],
  ];
  const title = module.name || module.root || 'test module';
  return `<div class="scanner-test-module">
    <strong>${escHtml(title)}</strong>
    <div class="scanner-params-grid">${rows.map(([label, value]) => renderParameterRow(label, value)).join('')}</div>
  </div>`;
}

function formatParameterValue(value) {
  if (value === undefined || value === null || value === '') return '-';
  if (Array.isArray(value)) return value.length ? value.join(', ') : '-';
  if (typeof value === 'boolean') return value ? 'yes' : 'no';
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}

function formatSeconds(value) {
  if (value == null) return '-';
  const seconds = Number(value);
  if (seconds < 60) return seconds + 's';
  if (seconds < 3600) return Math.floor(seconds / 60) + 'm';
  if (seconds < 86400) return Math.floor(seconds / 3600) + 'h';
  return Math.floor(seconds / 86400) + 'd';
}

function operatorLabel(op) {
  const labels = { GT: 'is greater than', LT: 'is less than', GTE: 'is greater than or equal', LTE: 'is less than or equal', EQ: 'equals', NE: 'is not equal' };
  return labels[op] || op;
}

function gateConditionLabel(metric) {
  const labels = { bugs: 'Bugs', vulnerabilities: 'Vulnerabilities', code_smells: 'Code Smells', coverage: 'Coverage', new_bugs: 'New Bugs', new_vulnerabilities: 'New Vulnerabilities', new_code_smells: 'New Code Smells', new_coverage: 'Coverage on New Code', duplicated_lines_density: 'Duplicated Lines (%)', new_duplicated_lines_density: 'Duplicated Lines on New Code (%)', new_security_hotspots_reviewed: 'Security Hotspots Reviewed', new_maintainability_rating: 'Maintainability Rating on New Code', new_reliability_rating: 'Reliability Rating on New Code', new_security_rating: 'Security Rating on New Code', security_hotspots_reviewed: 'Security Hotspots Reviewed', security_review_rating: 'Security Review Rating', reliability_remediation_effort: 'Reliability Remediation Effort', security_remediation_effort: 'Security Remediation Effort' };
  return labels[metric] || metric;
}

function conditionMetricSuffix(value, metric) {
  const pctMetrics = ['coverage', 'new_coverage', 'duplicated_lines_density', 'new_duplicated_lines_density'];
  if (pctMetrics.includes(metric)) return value + '%';
  const ratingMetrics = ['new_maintainability_rating', 'new_reliability_rating', 'new_security_rating', 'security_review_rating'];
  if (ratingMetrics.includes(metric)) return String(value);
  return String(value);
}

function gateMetricOptions() {
  return [
    ['bugs', 'Bugs'],
    ['vulnerabilities', 'Vulnerabilities'],
    ['code_smells', 'Code Smells'],
    ['coverage', 'Coverage'],
    ['new_bugs', 'New Bugs'],
    ['new_vulnerabilities', 'New Vulnerabilities'],
    ['new_code_smells', 'New Code Smells'],
    ['new_coverage', 'Coverage on New Code'],
    ['duplicated_lines_density', 'Duplicated Lines (%)'],
    ['new_duplicated_lines_density', 'Duplicated Lines on New Code (%)'],
  ].map(([value, label]) => `<option value="${escAttr(value)}">${escHtml(label)}</option>`).join('');
}

async function reloadGateConditions(gateId, box) {
  box.innerHTML = '<div class="loading-inline">Loading\u2026</div>';
  box.classList.remove('hidden');
  const expandBtn = document.querySelector('.expand-gate-btn[data-gate-id="' + gateId + '"]');
  if (expandBtn) expandBtn.textContent = 'Conditions \u25B4';
  try {
    const gate = await apiFetch('/quality-gates/' + gateId);
    const conds = gate.conditions || [];
    if (conds.length) {
      box.innerHTML = `<table class="conditions-table">
        <thead><tr><th>Metric</th><th>Operator</th><th>Value</th><th>Warning</th><th>New Code</th><th></th></tr></thead>
        <tbody>${conds.map(condition => `<tr>
          <td>${gateConditionLabel(condition.metric)}</td>
          <td>${escHtml(operatorLabel(condition.operator))}</td>
          <td class="mono">${conditionMetricSuffix(condition.threshold, condition.metric)}</td>
          <td class="mono">${condition.warning_threshold != null ? conditionMetricSuffix(condition.warning_threshold, condition.metric) : '\u2014'}</td>
          <td>${condition.on_new_code ? '\u2713' : ''}</td>
          <td><button class="btn-sm btn-danger del-cond-btn" data-cond-id="${condition.id}" data-gate-id="${gateId}">Remove</button></td>
        </tr>`).join('')}</tbody>
      </table>`;
    } else {
      box.innerHTML = '<p style="color:var(--text-muted);padding:8px 0;font-size:13px">No conditions defined.</p>';
    }
    box.innerHTML += renderAddConditionForm(gateId);
  } catch {
    box.innerHTML = '<p style="color:var(--danger);font-size:13px">Failed to reload conditions.</p>';
  }
}

function renderAddConditionForm(gateId) {
  return `<div class="create-form" style="padding-top:12px;border-top:1px solid var(--border)">
    <div class="form-row" style="gap:6px;flex-wrap:wrap">
      <select class="filter-sel add-cond-metric" data-gate-id="${gateId}" style="width:160px">
        ${gateMetricOptions()}
      </select>
      <select class="filter-sel add-cond-op" data-gate-id="${gateId}">
        <option value="LT">is less than</option>
        <option value="GT">is greater than</option>
        <option value="GTE">is greater than or equal</option>
        <option value="LTE">is less than or equal</option>
        <option value="EQ">equals</option>
        <option value="NE">is not equal</option>
      </select>
      <input class="filter-input add-cond-threshold" placeholder="value" type="number" step="any" style="width:80px" data-gate-id="${gateId}">
      <input class="filter-input add-cond-warning" placeholder="warning (opt)" type="number" step="any" style="width:110px" data-gate-id="${gateId}">
      <label style="font-size:12px;white-space:nowrap"><input type="checkbox" class="add-cond-newcode" data-gate-id="${gateId}"> On new code</label>
      <button class="btn-sm btn-primary add-cond-btn" data-gate-id="${gateId}">Add condition</button>
    </div>
  </div>`;
}

export function bindAdminTabContent() {
  const project = state.currentProject;
  bindBackgroundTasksContent();
  if (!project) return;

  document.querySelectorAll('.expand-gate-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      const id = btn.dataset.gateId;
      const box = document.getElementById('gate-cond-' + id);
      if (!box) return;
      const hidden = box.classList.toggle('hidden');
      btn.textContent = hidden ? 'Conditions \u25BE' : 'Conditions \u25B4';
      if (hidden) {
        return;
      }

      if (box.innerHTML.includes('Loading')) {
        try {
          const gate = await apiFetch('/quality-gates/' + id);
          const conds = gate.conditions || [];
          if (conds.length) {
            box.innerHTML = `<table class="conditions-table">
              <thead><tr><th>Metric</th><th>Operator</th><th>Value</th><th>Warning</th><th>New Code</th><th></th></tr></thead>
              <tbody>${conds.map(condition => `<tr>
                <td>${gateConditionLabel(condition.metric)}</td>
                <td>${escHtml(operatorLabel(condition.operator))}</td>
                <td class="mono">${conditionMetricSuffix(condition.threshold, condition.metric)}</td>
                <td class="mono">${condition.warning_threshold != null ? conditionMetricSuffix(condition.warning_threshold, condition.metric) : '\u2014'}</td>
                <td>${condition.on_new_code ? '\u2713' : ''}</td>
                <td><button class="btn-sm btn-danger del-cond-btn" data-cond-id="${condition.id}" data-gate-id="${id}">Remove</button></td>
              </tr>`).join('')}</tbody>
            </table>`;
          } else {
            box.innerHTML = '<p style="color:var(--text-muted);padding:8px 0;font-size:13px">No conditions defined.</p>';
          }
          box.innerHTML += renderAddConditionForm(id);
        } catch {
          box.innerHTML = '<p style="color:var(--danger);font-size:13px">Failed to load conditions.</p>';
        }
      }
    });
  });

  document.querySelectorAll('.assign-gate-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      const id = btn.dataset.gateId;
      const name = btn.dataset.gateName;
      btn.disabled = true;
      try {
        await apiFetch('/projects/' + encodeURIComponent(project.key) + '/quality-gate', {
          method: 'POST',
          body: JSON.stringify({ gate_id: Number.parseInt(id, 10) }),
        });
        showToastMessage('Gate "' + name + '" assigned.');
      } catch (err) {
        showToastMessage(err.message, 'error');
      }
      btn.disabled = false;
    });
  });

  document.getElementById('createGateBtn')?.addEventListener('click', async () => {
    const name = document.getElementById('newGateName')?.value.trim();
    if (!name) { showToastMessage('Gate name is required.', 'error'); return; }
    try {
      await apiFetch('/quality-gates', {
        method: 'POST',
        body: JSON.stringify({ name, small_changeset_lines: 20 }),
      });
      showToastMessage('Gate "' + name + '" created.');
      state.gateData = null;
      await loadGateData();
    } catch (err) {
      showToastMessage(err.message, 'error');
    }
  });

  document.querySelectorAll('.del-gate-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      const name = btn.dataset.gateName;
      if (!confirm('Delete quality gate "' + name + '" and all its conditions?')) return;
      btn.disabled = true;
      try {
        await apiFetch('/quality-gates/' + btn.dataset.gateId, { method: 'DELETE' });
        showToastMessage('Gate "' + name + '" deleted.');
        state.gateData = null;
        await loadGateData();
      } catch (err) {
        showToastMessage(err.message, 'error');
        btn.disabled = false;
      }
    });
  });

  document.querySelectorAll('.set-default-gate-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      btn.disabled = true;
      try {
        await apiFetch('/quality-gates/' + btn.dataset.gateId + '/set-default', { method: 'POST' });
        showToastMessage('Default gate set.');
        state.gateData = null;
        await loadGateData();
      } catch (err) {
        showToastMessage(err.message, 'error');
        btn.disabled = false;
      }
    });
  });

  document.querySelectorAll('.copy-gate-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      const newName = prompt('Copy "' + btn.dataset.gateName + '" as:');
      if (!newName) return;
      btn.disabled = true;
      try {
        await apiFetch('/quality-gates/' + btn.dataset.gateId + '/copy', {
          method: 'POST',
          body: JSON.stringify({ name: newName }),
        });
        showToastMessage('Gate copied as "' + newName + '".');
        state.gateData = null;
        await loadGateData();
      } catch (err) {
        showToastMessage(err.message, 'error');
        btn.disabled = false;
      }
    });
  });

  // Add condition and remove condition — delegated because they are inside expandable containers
  document.addEventListener('click', async event => {
    const addBtn = event.target.closest('.add-cond-btn');
    if (addBtn) {
      const gateId = addBtn.dataset.gateId;
      const box = document.getElementById('gate-cond-' + gateId);
      const metric = box?.querySelector('.add-cond-metric')?.value;
      const op = box?.querySelector('.add-cond-op')?.value || 'GT';
      const thresholdStr = box?.querySelector('.add-cond-threshold')?.value.trim();
      const warningStr = box?.querySelector('.add-cond-warning')?.value.trim();
      const onNewCode = box?.querySelector('.add-cond-newcode')?.checked || false;
      if (!metric) { showToastMessage('Metric is required.', 'error'); return; }
      if (!thresholdStr) { showToastMessage('Value is required.', 'error'); return; }
      const threshold = Number.parseFloat(thresholdStr);
      if (Number.isNaN(threshold)) { showToastMessage('Value must be a number.', 'error'); return; }
      let warningThreshold = undefined;
      if (warningStr) {
        const w = Number.parseFloat(warningStr);
        if (!Number.isNaN(w)) warningThreshold = w;
      }
      addBtn.disabled = true;
      try {
        await apiFetch('/quality-gates/' + gateId + '/conditions', {
          method: 'POST',
          body: JSON.stringify({ metric, operator: op, threshold, warning_threshold: warningThreshold, on_new_code: onNewCode }),
        });
        showToastMessage('Condition added.');
        await reloadGateConditions(gateId, box);
      } catch (err) {
        showToastMessage(err.message, 'error');
        addBtn.disabled = false;
      }
      return;
    }
    const delBtn = event.target.closest('.del-cond-btn');
    if (delBtn) {
      const gateId = delBtn.dataset.gateId;
      const condId = delBtn.dataset.condId;
      if (!confirm('Remove this condition?')) return;
      delBtn.disabled = true;
      try {
        await apiFetch('/quality-gates/' + gateId + '/conditions/' + condId, { method: 'DELETE' });
        showToastMessage('Condition removed.');
        const box = document.getElementById('gate-cond-' + gateId);
        if (box) await reloadGateConditions(gateId, box);
      } catch (err) {
        showToastMessage(err.message, 'error');
        delBtn.disabled = false;
      }
      return;
    }
  });

  document.getElementById('addWhBtn')?.addEventListener('click', async () => {
    const name = document.getElementById('newWhName')?.value.trim();
    const url = document.getElementById('newWhUrl')?.value.trim();
    const secret = document.getElementById('newWhSecret')?.value.trim();
    if (!name || !url) {
      showToastMessage('Name and URL are required.', 'error');
      return;
    }
    try {
      await apiFetch('/webhooks', {
        method: 'POST',
        body: JSON.stringify({ name, url, secret: secret || '', project_key: project.key }),
      });
      state.webhooksData = null;
      await loadWebhooksData();
    } catch (err) {
      showToastMessage(err.message, 'error');
    }
  });

  document.querySelectorAll('.test-wh-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      btn.disabled = true;
      try {
        await apiFetch('/webhooks/' + btn.dataset.whId + '/test', { method: 'POST' });
        showToastMessage('Test delivery sent.');
      } catch (err) {
        showToastMessage(err.message, 'error');
      }
      btn.disabled = false;
    });
  });

  document.querySelectorAll('.del-wh-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      if (!confirm('Delete this webhook?')) return;
      btn.disabled = true;
      try {
        await apiFetch('/webhooks/' + btn.dataset.whId, { method: 'DELETE' });
        state.webhooksData = null;
        await loadWebhooksData();
      } catch (err) {
        showToastMessage(err.message, 'error');
        btn.disabled = false;
      }
    });
  });

  document.getElementById('saveNcpBtn')?.addEventListener('click', async () => {
    const strategy = document.getElementById('ncpStrategy')?.value;
    const value = document.getElementById('ncpValue')?.value.trim();
    try {
      await apiFetch('/projects/' + encodeURIComponent(project.key) + '/new-code-period', {
        method: 'PUT',
        body: JSON.stringify({ strategy, value: value || '' }),
      });
      state.newCodePeriod = { strategy, value };
      const display = document.getElementById('ncpDisplay');
      if (display) display.textContent = strategy + (value ? ' \u2014 ' + value : '');
      showToastMessage('New code period saved.');
    } catch (err) {
      showToastMessage(err.message, 'error');
    }
  });

  document.querySelectorAll('.assign-profile-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      const id = btn.dataset.profileId;
      const lang = btn.dataset.profileLang;
      const name = btn.dataset.profileName;
      btn.disabled = true;
      try {
        await apiFetch('/projects/' + encodeURIComponent(project.key) + '/profiles', {
          method: 'POST',
          body: JSON.stringify({ profile_id: Number.parseInt(id, 10), language: lang }),
        });
        showToastMessage('Profile "' + name + '" assigned.');
        state.profilesData = null;
        await loadProfilesData();
      } catch (err) {
        showToastMessage(err.message, 'error');
      }
      btn.disabled = false;
    });
  });

  document.querySelectorAll('.export-profile-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      const id = btn.dataset.profileId;
      const name = btn.dataset.profileName;
      btn.disabled = true;
      try {
        const doc = await apiFetch('/profiles/' + encodeURIComponent(id) + '/export');
        const text = JSON.stringify(doc, null, 2);
        await navigator.clipboard?.writeText(text);
        showToastMessage('Profile "' + name + '" exported.');
      } catch (err) {
        showToastMessage(err.message, 'error');
      }
      btn.disabled = false;
    });
  });

  document.getElementById('refreshCustomRulesBtn')?.addEventListener('click', () => loadCustomRulesData());
  document.getElementById('refreshAIProvidersBtn')?.addEventListener('click', () => loadAIProvidersData());
  document.getElementById('returnRuleStudioBtn')?.addEventListener('click', () => {
    document.querySelector('.tab-btn[data-tab="custom-rules"]')?.click();
  });

  document.getElementById('focusRuleBuilderBtn')?.addEventListener('click', () => {
    state.builderOpen = true;
    renderView();
    document.getElementById('customRuleBuilder')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  });

  document.getElementById('closeRuleBuilderBtn')?.addEventListener('click', () => {
    state.builderOpen = false;
    state.editingCustomRuleId = null;
    renderView();
  });

  document.getElementById('customRuleSearch')?.addEventListener('input', event => {
    const filters = { search: '', lifecycle: 'all', ...state.customRuleFilters };
    filters.search = event.target.value;
    state.customRuleFilters = filters;
    renderView();
  });

  document.getElementById('customRuleLifecycleFilter')?.addEventListener('change', event => {
    const filters = { search: '', lifecycle: 'all', ...state.customRuleFilters };
    filters.lifecycle = event.target.value;
    state.customRuleFilters = filters;
    renderView();
  });

  bindCustomRuleBuilderControls();

  document.querySelectorAll('.custom-rule-row-toggle').forEach(toggle => {
    toggle.addEventListener('click', () => {
      const ruleId = Number.parseInt(toggle.dataset.ruleId || '0', 10);
      if (!ruleId) return;
      state.expandedCustomRuleId = state.expandedCustomRuleId === ruleId ? null : ruleId;
      renderView();
    });
  });

  document.querySelectorAll('.edit-custom-rule-btn').forEach(btn => {
    btn.addEventListener('click', event => {
      event.stopPropagation();
      const ruleId = Number.parseInt(btn.dataset.ruleId || '0', 10);
      if (!ruleId) return;
      const rule = (state.customRulesData || []).find(item => item.id === ruleId);
      if (!rule) return;
      state.editingCustomRuleId = ruleId;
      state.expandedCustomRuleId = ruleId;
      state.builderOpen = true;
      renderView();
      requestAnimationFrame(() => {
        prefillCustomRuleBuilder(rule);
        document.getElementById('customRuleBuilder')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
      });
    });
  });

  document.getElementById('cancelEditCustomRuleBtn')?.addEventListener('click', () => {
    state.editingCustomRuleId = null;
    state.builderOpen = false;
    renderView();
  });

  document.getElementById('generateCustomRuleAIBtn')?.addEventListener('click', async event => {
    const button = event.currentTarget;
    const select = document.getElementById('customRuleAIModel');
    const option = select?.selectedOptions?.[0];
    const intent = document.getElementById('customRuleAIIntent')?.value.trim();
    if (!option?.dataset.provider || !option?.dataset.model) {
      showToastMessage('Choose an AI model first.', 'error');
      return;
    }
    if (option.dataset.status !== 'connected') {
      openCustomRuleAIProviderSetup();
      showToastMessage('Set up the selected AI provider before generating a draft.', 'error');
      return;
    }
    if (!intent) {
      showToastMessage('Describe what the rule should detect.', 'error');
      document.getElementById('customRuleAIIntent')?.focus();
      return;
    }
    const status = document.getElementById('customRuleAIStatus');
    button.disabled = true;
    if (status) status.textContent = 'Generating...';
    try {
      const response = await apiFetch('/custom-rules/ai/suggest', {
        method: 'POST',
        body: JSON.stringify({
          provider: option.dataset.provider,
          model: option.dataset.model,
          intent,
          current: readCustomRuleBuilderDraft(),
        }),
      });
      applyCustomRuleAISuggestion(response.suggestion || response);
      if (status) status.textContent = 'Draft generated.';
      showToastMessage('AI draft generated.');
    } catch (err) {
      if (status) status.textContent = err.message;
      showToastMessage(err.message, 'error');
    }
    button.disabled = false;
  });

  document.getElementById('createCustomRuleBtn')?.addEventListener('click', async () => {
    const packName = document.getElementById('customRulePack')?.value.trim() || 'Rule Studio';
    const namespace = document.getElementById('customRuleNamespace')?.value.trim() || 'custom';
    const key = document.getElementById('customRuleKey')?.value.trim();
    const name = document.getElementById('customRuleName')?.value.trim();
    const language = document.getElementById('customRuleLanguage')?.value || 'go';
    const type = document.getElementById('customRuleType')?.value || 'code_smell';
    const severity = document.getElementById('customRuleSeverity')?.value || 'major';
    const engine = document.getElementById('customRuleEngine')?.value || 'text';
    const textPattern = document.getElementById('customRuleTextPattern')?.value.trim();
    const goASTPattern = document.getElementById('customRuleGoASTPattern')?.value || 'forbidden_call';
    const target = document.getElementById('customRuleTarget')?.value.trim();
    const query = document.getElementById('customRuleQuery')?.value.trim();
    const example = document.getElementById('customRuleExample')?.value || '';
    const compliantExample = document.getElementById('customRuleCompliantExample')?.value || '';
    const message = document.getElementById('customRuleMessage')?.value.trim();
    if (!key || !name || !example) {
      showToastMessage('Rule key, name, and noncompliant example are required.', 'error');
      return;
    }
    if (engine === 'text' && !textPattern) {
      showToastMessage('A regexp pattern is required.', 'error');
      return;
    }
    if (engine === 'go-ast' && !target) {
      showToastMessage('A target (e.g. fmt.Println) is required.', 'error');
      return;
    }
    if (engine === 'tree-sitter' && !query) {
      showToastMessage('A tree-sitter query is required.', 'error');
      return;
    }
    const engineConfig = {};
    if (engine === 'tree-sitter') engineConfig.query = query;
    if (engine === 'go-ast') { engineConfig.pattern = goASTPattern; engineConfig.target = target; }
    if (engine === 'text') engineConfig.pattern = textPattern;
    const doc = {
      version: 1,
      pack: { name: packName, namespace },
      rules: [{
        key, name, language, type, severity, engine,
        engine_config: engineConfig,
        message: message || name,
        examples: [
          { name: 'compliant', code: compliantExample, compliant: true },
          { name: 'noncompliant', code: example, compliant: false },
        ],
      }],
    };
    try {
      const editingId = state.editingCustomRuleId;
      if (editingId) {
        const draftRule = doc.rules[0];
        const fullKey = draftRule.key.includes(':') ? draftRule.key : `${namespace}:${draftRule.key}`;
        await apiFetch('/custom-rules/' + encodeURIComponent(editingId), {
          method: 'PUT',
          body: JSON.stringify({ ...draftRule, key: fullKey, pack_name: packName }),
        });
        showToastMessage('Rule updated.');
        state.editingCustomRuleId = null;
        state.builderOpen = false;
      } else {
        await apiFetch('/custom-rules', { method: 'POST', body: JSON.stringify(doc) });
        showToastMessage('Draft created.');
        state.builderOpen = false;
      }
      await loadCustomRulesData();
    } catch (err) {
      showToastMessage(err.message, 'error');
    }
  });

  document.getElementById('importCustomRulesBtn')?.addEventListener('click', async () => {
    const text = document.getElementById('customRuleImportText')?.value.trim();
    if (!text) {
      showToastMessage('Paste a rule pack first.', 'error');
      return;
    }
    try {
      await apiFetch('/custom-rules/import', { method: 'POST', body: text });
      showToastMessage('Custom rule pack imported.');
      await loadCustomRulesData();
    } catch (err) {
      showToastMessage(err.message, 'error');
    }
  });

  document.querySelectorAll('.validate-custom-rule-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      btn.disabled = true;
      try {
        await apiFetch('/custom-rules/' + encodeURIComponent(btn.dataset.ruleId) + '/validate', { method: 'POST' });
        showToastMessage('Custom rule validated.');
        await loadCustomRulesData();
      } catch (err) {
        showToastMessage(err.message, 'error');
      }
      btn.disabled = false;
    });
  });

  document.querySelectorAll('.publish-custom-rule-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      btn.disabled = true;
      try {
        await apiFetch('/custom-rules/' + encodeURIComponent(btn.dataset.ruleId) + '/publish', { method: 'POST' });
        showToastMessage('Custom rule published.');
        state.profilesData = null;
        await loadCustomRulesData();
      } catch (err) {
        showToastMessage(err.message, 'error');
      }
      btn.disabled = false;
    });
  });

  document.querySelectorAll('.disable-custom-rule-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      if (!confirm('Disable this custom rule?')) return;
      btn.disabled = true;
      try {
        await apiFetch('/custom-rules/' + encodeURIComponent(btn.dataset.ruleId) + '/disable', { method: 'POST' });
        showToastMessage('Custom rule disabled.');
        state.profilesData = null;
        await loadCustomRulesData();
      } catch (err) {
        showToastMessage(err.message, 'error');
      }
      btn.disabled = false;
    });
  });

  document.querySelectorAll('.export-custom-rule-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      btn.disabled = true;
      try {
        const doc = await apiFetch('/custom-rules/' + encodeURIComponent(btn.dataset.ruleId) + '/export');
        await navigator.clipboard?.writeText(JSON.stringify(doc, null, 2));
        showToastMessage('Custom rule exported.');
      } catch (err) {
        showToastMessage(err.message, 'error');
      }
      btn.disabled = false;
    });
  });

  document.querySelectorAll('.add-custom-rule-profile-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      const select = document.getElementById('custom-rule-profile-' + btn.dataset.ruleId);
      const profileID = Number.parseInt(select?.value || '0', 10);
      if (!profileID) return;
      btn.disabled = true;
      try {
        await apiFetch('/profiles/' + encodeURIComponent(profileID) + '/rules', {
          method: 'POST',
          body: JSON.stringify({ rule_key: btn.dataset.ruleKey, params: {} }),
        });
        showToastMessage('Custom rule added to profile.');
        state.profilesData = null;
        await loadCustomRulesData();
      } catch (err) {
        showToastMessage(err.message, 'error');
      }
      btn.disabled = false;
    });
  });
}

export function bindBackgroundTasksContent() {
  document.getElementById('refreshBackgroundTasksBtn')?.addEventListener('click', () => loadBackgroundTasksData());
  document.getElementById('applyTaskFiltersBtn')?.addEventListener('click', () => {
    loadBackgroundTasksData({
      type: document.getElementById('taskTypeFilter')?.value || '',
      status: document.getElementById('taskStatusFilter')?.value || '',
      projectKey: document.getElementById('taskProjectFilter')?.value.trim() || '',
      scanId: document.getElementById('taskScanFilter')?.value.trim() || '',
      workerId: document.getElementById('taskWorkerFilter')?.value.trim() || '',
      offset: 0,
    });
  });
  document.getElementById('clearTaskFiltersBtn')?.addEventListener('click', () => loadBackgroundTasksData({ type: '', status: '', projectKey: '', scanId: '', workerId: '', offset: 0 }));
  document.querySelectorAll('[data-task-status-filter]').forEach(btn => {
    btn.addEventListener('click', () => loadBackgroundTasksData({ status: btn.dataset.taskStatusFilter || '', offset: 0 }));
  });
  document.querySelectorAll('[data-task-page]').forEach(btn => {
    btn.addEventListener('click', () => {
      const filters = currentTaskFilters();
      const delta = btn.dataset.taskPage === 'next' ? filters.limit : -filters.limit;
      loadBackgroundTasksData({ offset: Math.max(0, filters.offset + delta) });
    });
  });
  document.querySelectorAll('[data-task-id].background-task-row').forEach(row => {
    row.addEventListener('click', () => loadBackgroundTaskDetail(row.dataset.taskId));
  });
  document.querySelectorAll('[data-task-action]').forEach(btn => {
    btn.addEventListener('click', async () => {
      const action = btn.dataset.taskAction;
      const id = btn.dataset.taskId;
      if (action === 'cancel' && !confirm('Cancel this queued background task?')) return;
      btn.disabled = true;
      try {
        state.selectedBackgroundTask = await apiFetch('/admin/background-tasks/' + encodeURIComponent(id) + '/' + action, { method: 'POST' });
        await loadBackgroundTasksData();
        showToastMessage('Task ' + action + ' accepted.');
      } catch (err) {
        showToastMessage(err.message || 'Task action failed.', 'error');
        btn.disabled = false;
      }
    });
  });
}
