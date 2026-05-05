import test from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const apiPrefix = '/api/v1';
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const app = await import(pathToFileURL(path.join(__dirname, 'app/main.js')).href);

function createHarness(options = {}) {
  const search = options.search || '';
  const storage = new Map();
  const requests = [];
  const historyCalls = [];
  const elements = new Map();

  function getElement(id) {
    if (!elements.has(id)) {
      const element = {
        id,
        innerHTML: '',
        value: '',
        dataset: {},
        hidden: false,
        disabled: false,
        selectedOptions: [],
        listeners: {},
        addEventListener(type, handler) { this.listeners[type] = handler; },
        click() { this.listeners.click?.({ currentTarget: this, preventDefault() {} }); },
        setAttribute(name, value) { this[name] = String(value); },
        classList: { add() {}, remove() {} },
      };
      elements.set(id, element);
    }
    return elements.get(id);
  }

  app.replaceState(app.createInitialState());

  globalThis.history = {
    pushState(_state, _title, url) { historyCalls.push({ type: 'push', url }); },
    replaceState(_state, _title, url) { historyCalls.push({ type: 'replace', url }); },
  };
  globalThis.location = { search, pathname: '/ui' };
  globalThis.window = { addEventListener() {} };
  globalThis.localStorage = {
    getItem(key) { return storage.has(key) ? storage.get(key) : null; },
    setItem(key, value) { storage.set(key, String(value)); },
    removeItem(key) { storage.delete(key); },
  };
  globalThis.document = {
    readyState: 'loading',
    addEventListener() {},
    getElementById(id) { return getElement(id); },
    querySelector(selector) { return options.querySelector ? options.querySelector(selector) : null; },
    querySelectorAll() { return []; },
    createElement() { return { classList: { add() {}, remove() {} }, remove() {}, textContent: '' }; },
    body: { appendChild() {} },
  };
  globalThis.fetch = async (url, requestOptions) => {
    const requestUrl = normalizeRequestUrl(url);
    requests.push(requestUrl);
    const response = options.fetchHandler ? await options.fetchHandler(requestUrl, requestOptions) : { body: {} };
    return {
      status: response.status ?? 200,
      ok: response.ok ?? true,
      json: async () => response.body ?? {},
    };
  };
  globalThis.setTimeout = () => 0;
  globalThis.clearTimeout = () => {};
  globalThis.requestAnimationFrame = callback => { callback(); };
  globalThis.confirm = () => true;

  return {
    app,
    requests,
    historyCalls,
    elements,
  };
}

function normalizeRequestUrl(url) {
  if (typeof url === 'string') return url;
  if (url instanceof URL) return url.href;
  if (url && typeof url === 'object' && typeof url.url === 'string') return url.url;
  throw new TypeError('unsupported fetch URL type in test harness');
}

test('normalizeScope maps snake_case scope fields', () => {
  const { app: harnessApp } = createHarness();
  const scope = harnessApp.normalizeScope({
    type: 'pull_request',
    branch: 'feature/login',
    pull_request: '42',
    pull_request_base: 'main',
    default_branch: 'main',
  });

  assert.equal(scope.type, 'pull_request');
  assert.equal(scope.pullRequestKey, '42');
  assert.equal(scope.pullRequestBase, 'main');
  assert.equal(scope.defaultBranch, 'main');
});

test('parseProjectRoute reads project, tab and branch scope from query string', () => {
  const { app: harnessApp } = createHarness({ search: '?project=demo&tab=code&branch=release%2F1.2' });
  const route = { ...harnessApp.parseProjectRoute() };

  assert.deepEqual(route, {
    project: 'demo',
    tab: 'code',
    branch: 'release/1.2',
    pullRequest: '',
  });
});

test('buildProjectRoute preserves pull request scope in the URL', () => {
  const { app: harnessApp } = createHarness();
  const route = harnessApp.buildProjectRoute('demo', 'information', {
    type: 'pull_request',
    pullRequestKey: '42',
    branch: 'feature/login',
  });

  assert.equal(route, '?project=demo&tab=information&pull_request=42');
});

test('render outputs the unified login card before authentication', () => {
  const { app: harnessApp, elements } = createHarness();

  harnessApp.render();

  const html = elements.get('app').innerHTML;
  assert.match(html, /login-card-unified/);
  assert.match(html, /login-card-header/);
  assert.match(html, /login-brand-name/);
  assert.match(html, /login-features/);
  assert.match(html, /Welcome back/);
  assert.match(html, /Sign in/);
});

test('render shows API docs and only configured observability links in the global nav', () => {
  const { app: harnessApp, elements } = createHarness();

  harnessApp.state.view = 'projects';
  harnessApp.state.user = { login: 'admin', name: 'Administrator' };
  harnessApp.state.uiSettings = { observabilityLinks: [{ label: 'Datadog', href: 'https://app.datadoghq.com/dashboard/abc' }] };

  harnessApp.render();

  const html = elements.get('app').innerHTML;
  assert.match(html, /aria-label="Admin shortcuts"/);
  assert.match(html, /id="apiDocsBtn"/);
  assert.match(html, /id="tagsBtn"/);
  assert.match(html, /id="backgroundTasksBtn"/);
  assert.doesNotMatch(html, /href="\/api\/v1\/system\/info"/);
  assert.match(html, /href="\/metrics"/);
  assert.match(html, /https:\/\/app\.datadoghq\.com\/dashboard\/abc/);
  assert.doesNotMatch(html, /http:\/\/localhost:9091\/targets/);
  assert.doesNotMatch(html, /http:\/\/localhost:16686/);
  assert.doesNotMatch(html, /http:\/\/localhost:3100\/ready/);
});

test('renderApiDocsPage documents Ollanta API groups', () => {
  const { app: harnessApp } = createHarness();

  const html = harnessApp.renderApiDocsPage();

  assert.match(html, /Ollanta API/);
  assert.match(html, /Authorization: Bearer/);
  assert.match(html, /<details class="api-endpoint-row">/);
  assert.match(html, /api-example-toggle/);
  assert.match(html, /curl -X GET/);
  assert.match(html, /curl -X POST/);
  assert.match(html, /Authorization: Bearer \$OLLANTA_TOKEN/);
  assert.match(html, /project_key/);
  assert.match(html, /\/api\/v1\/auth\/login/);
  assert.match(html, /\/api\/v1\/projects\/{key}\/issues/);
  assert.match(html, /Tag Governance/);
  assert.match(html, /\/api\/v1\/tags\/bulk\/preview/);
  assert.match(html, /\/api\/v1\/saved-filters/);
  assert.match(html, /\/api\/v1\/system\/info/);
  assert.match(html, /\/metrics/);
});

test('renderTagsPage shows catalog, tag detail, bulk edit and saved filters', () => {
  const { app: harnessApp } = createHarness();

  harnessApp.state.tagCatalogData = {
    total: 1,
    items: [{ key: 'team-api', display_name: 'Team API', color: '#0ea5e9', description: 'API ownership', owner_name: 'Platform', status: 'active', usage: { issue_count: 2 } }],
  };
  harnessApp.state.selectedTagKey = 'team-api';
  harnessApp.state.tagDetailData = {
    tag: {
      key: 'team-api',
      display_name: 'Team API',
      color: '#0ea5e9',
      description: 'API ownership',
      owner_name: 'Platform',
      status: 'active',
      aliases: [{ alias: 'api' }],
      usage: { issue_count: 2, project_count: 1, rule_count: 0, custom_rule_count: 1, saved_filter_count: 1 },
    },
    audit: [{ action: 'created', created_at: '2026-05-01T12:00:00Z' }],
  };
  harnessApp.state.savedFiltersData = { items: [{ id: 7, name: 'API review', criteria: { tag: 'team-api', severity: 'critical' } }] };
  harnessApp.state.tagBulkPreview = { target_count: 2, changes: [{ target_key: 'ISSUE-1', before: ['legacy'], after: ['team-api'] }] };

  const html = harnessApp.renderTagsPage();

  assert.match(html, /Create Tag/);
  assert.match(html, /Catalog/);
  assert.match(html, /Team API/);
  assert.match(html, /tag-swatch/);
  assert.match(html, /tag-status-active/);
  assert.match(html, /Bulk Edit/);
  assert.match(html, /ISSUE-1/);
  assert.match(html, /Saved Filters/);
  assert.match(html, /API review/);
  assert.match(html, /Open issues/);
});

test('renderScopeToolbar shows the real default branch name in the branch selector', () => {
  const { app: harnessApp } = createHarness();

  harnessApp.state.scope = harnessApp.normalizeScope({ type: 'branch', branch: 'main', defaultBranch: 'main' });
  harnessApp.state.branchesData = [
    { name: '', is_default: false },
    { name: 'main', is_default: true },
    { name: 'release', is_default: false },
  ];
  harnessApp.state.pullRequestsData = [];

  const html = harnessApp.renderScopeToolbar();
  assert.match(html, /<option value="main">main<\/option>/);
  assert.doesNotMatch(html, /<option value="">Default branch<\/option>/);
});

test('renderBackgroundTasksPage renders filters, summary and project-first task rows', () => {
  const { app: harnessApp } = createHarness();

  harnessApp.state.view = 'background-tasks';
  harnessApp.state.backgroundTasksSummary = { queue_depth: 2, running_count: 1, failed_count: 3, stale_count: 1, retry_count: 0, recent_completion_count: 4 };
  harnessApp.state.backgroundTasksData = {
    total: 1,
    limit: 25,
    offset: 0,
    items: [{ id: 'index:7', type: 'index', status: 'failed', project_key: 'demo', scan_id: 99, attempts: 2, worker_id: 'worker-1', last_error: 'boom' }],
  };

  const html = harnessApp.renderBackgroundTasksPage();

  assert.match(html, /Background Tasks/);
  assert.match(html, /id="backgroundTasksBackBtn"/);
  assert.match(html, /id="taskProjectFilter" value=""/);
  assert.match(html, /<strong>demo<\/strong>/);
  assert.match(html, /index:7/);
  assert.match(html, /Search indexing/);
  assert.match(html, /task-status-failed/);
  assert.match(html, /boom/);
});

test('renderProjectDetail does not duplicate Background Tasks in project tabs', () => {
  const { app: harnessApp, elements } = createHarness();

  harnessApp.state.currentProject = { key: 'demo', name: 'Demo' };
  harnessApp.state.user = { login: 'admin' };
  harnessApp.state.view = 'project';
  harnessApp.state.projectTab = 'overview';
  harnessApp.state.overviewData = { last_scan: { total_issues: 0 } };
  harnessApp.state.branchesData = [
    { name: 'main', is_default: true },
    { name: 'release', is_default: false },
  ];
  harnessApp.state.pullRequestsData = [];

  harnessApp.render();
  const html = elements.get('app').innerHTML;
  const tabsHtml = html.match(/<div class="proj-tabs">[\s\S]*?<\/div>/)?.[0] || '';

  assert.match(html, /id="backgroundTasksBtn"/);
  assert.match(tabsHtml, /data-tab="scopes"/);
  assert.doesNotMatch(tabsHtml, /data-tab="admin"/);
  assert.doesNotMatch(tabsHtml, /data-tab="branches"/);
  assert.doesNotMatch(tabsHtml, /data-tab="information"/);
  assert.doesNotMatch(tabsHtml, /Background Tasks/);
});

test('renderRuleStudio shows summary stats and filters rule rows', () => {
  const { app: harnessApp, elements } = createHarness();

  harnessApp.state.currentProject = { key: 'demo', name: 'Demo' };
  harnessApp.state.user = { login: 'admin' };
  harnessApp.state.view = 'project';
  harnessApp.state.projectTab = 'custom-rules';
  harnessApp.state.profilesData = { profiles: [] };
  harnessApp.state.customRuleEngines = [{ engine: 'text', name: 'Text' }];
  harnessApp.state.customRuleAIProviders = [
    { id: 'openai', label: 'OpenAI', status: 'setup_required', models: ['gpt-5.5'], default_model: 'gpt-5.5', configured: false, setup_required: true, message: 'Connect an OpenAI-compatible provider before using these models.' },
    { id: 'mock', label: 'Mock AI', status: 'connected', models: ['deterministic'], default_model: 'deterministic', configured: true, local: true },
  ];
  harnessApp.state.customRulesData = [
    { id: '1', key: 'custom:forbidden-print', name: 'Forbidden print', language: 'go', engine: 'text', type: 'code_smell', severity: 'major', lifecycle: 'published', validation_status: 'passed' },
    { id: '2', key: 'custom:banned-debug', name: 'Banned debug', language: 'javascript', engine: 'text', type: 'bug', severity: 'critical', lifecycle: 'draft', validation_status: 'failed' },
  ];
  harnessApp.state.customRuleFilters = { search: 'debug', lifecycle: 'draft' };

  harnessApp.render();
  const html = elements.get('app').innerHTML;

  assert.match(html, /Rule Studio/);
  assert.match(html, /rule-studio-hero/);
  assert.match(html, /rule-studio-catalog/);
  assert.match(html, /rule-builder-card/);
  assert.match(html, /Validation/);
  assert.match(html, /Rule ID/);
  assert.match(html, /Full key:/);
  assert.match(html, /AI assist/);
  assert.match(html, /id="customRuleAIModel"/);
  assert.match(html, /OpenAI \/ gpt-5\.5 - setup required/);
  assert.match(html, /Mock AI \/ deterministic - local/);
  assert.match(html, /AI provider setup/);
  assert.match(html, /id="connectCustomRuleAIProviderBtn"/);
  assert.match(html, /Rule tests/);
  assert.match(html, /Compliant example/);
  assert.ok(html.indexOf('Compliant example') < html.indexOf('Noncompliant example'));
  assert.match(html, /id="customRuleTextPattern"/);
  assert.match(html, /data-engine-field="tree-sitter" hidden/);
  assert.doesNotMatch(html, /id="customRulePattern"/);
  assert.match(html, /Banned debug/);
  assert.doesNotMatch(html, /Forbidden print/);
  assert.match(html, /id="customRuleSearch"/);
});

test('Rule Studio setup-required AI status opens provider setup', () => {
  let aiProvidersTabClicked = false;
  const { app: harnessApp, elements } = createHarness({
    querySelector(selector) {
      if (selector === '.tab-btn[data-tab="ai-providers"]') {
        return { click() { aiProvidersTabClicked = true; harnessApp.state.projectTab = 'ai-providers'; } };
      }
      return null;
    },
  });

  harnessApp.state.currentProject = { key: 'demo', name: 'Demo' };
  harnessApp.state.user = { login: 'admin' };
  harnessApp.state.view = 'project';
  harnessApp.state.projectTab = 'custom-rules';
  harnessApp.state.profilesData = { profiles: [] };
  harnessApp.state.customRuleEngines = [{ engine: 'text', name: 'Text' }];
  harnessApp.state.customRuleAIProviders = [
    { id: 'openai', label: 'OpenAI', status: 'setup_required', models: ['gpt-5.5'], default_model: 'gpt-5.5', configured: false, setup_required: true, message: 'Connect an OpenAI-compatible provider before using these models.' },
  ];
  harnessApp.state.customRulesData = [];

  harnessApp.render();
  const html = elements.get('app').innerHTML;
  assert.match(html, /id="connectCustomRuleAIProviderBtn"[^>]*href="#custom-rule-ai-provider-setup"[^>]*>setup required<\/a>/);

  elements.get('customRuleAIModel').selectedOptions = [{ dataset: { provider: 'openai', providerLabel: 'OpenAI', model: 'gpt-5.5', status: 'setup_required', local: 'false' } }];
  elements.get('connectCustomRuleAIProviderBtn').dataset.provider = 'openai';
  elements.get('connectCustomRuleAIProviderBtn').click();

  assert.equal(aiProvidersTabClicked, true);
  assert.equal(harnessApp.state.projectTab, 'ai-providers');
  assert.equal(harnessApp.state.customRuleAISetupProvider, 'openai');
});

test('renderRuleStudio shows profile activation controls for published rules', () => {
  const { app: harnessApp, elements } = createHarness();

  harnessApp.state.currentProject = { key: 'demo', name: 'Demo' };
  harnessApp.state.user = { login: 'admin' };
  harnessApp.state.view = 'project';
  harnessApp.state.projectTab = 'custom-rules';
  harnessApp.state.profilesData = { profiles: [{ id: 10, name: 'Team Go', language: 'go' }] };
  harnessApp.state.customRuleEngines = [{ engine: 'text', name: 'Text' }];
  harnessApp.state.customRulesData = [
    { id: '1', key: 'custom:forbidden-print', name: 'Forbidden print', language: 'go', engine: 'text', type: 'code_smell', severity: 'major', lifecycle: 'published', validation_status: 'passed' },
  ];
  harnessApp.state.customRuleFilters = { search: '', lifecycle: 'all' };

  harnessApp.render();
  const html = elements.get('app').innerHTML;

  assert.match(html, /Forbidden print/);
  assert.match(html, /custom-rule-profile-1/);
  assert.match(html, /Team Go/);
  assert.match(html, /Add to profile/);
  assert.match(html, /validate-custom-rule-btn/);
  assert.match(html, /publish-custom-rule-btn/);
  assert.match(html, /export-custom-rule-btn/);
  assert.match(html, /disable-custom-rule-btn/);
});

test('loadCustomRulesData hydrates Rule Studio dependencies', async () => {
  const { app: harnessApp, requests, elements } = createHarness({
    fetchHandler(url) {
      switch (url) {
        case apiPrefix + '/custom-rules':
          return { body: { items: [{ id: '7', key: 'custom:no-debug', name: 'Loaded rule', language: 'go', lifecycle: 'draft', validation_status: 'none' }] } };
        case apiPrefix + '/rule-engines':
          return { body: [{ engine: 'text', name: 'Text' }] };
        case apiPrefix + '/custom-rules/ai/models':
          return { body: { providers: [{ id: 'mock', label: 'Mock AI', configured: true, models: ['deterministic'] }] } };
        case apiPrefix + '/profiles':
          return { body: [{ id: 10, name: 'Team Go', language: 'go' }] };
        default:
          throw new Error(`unexpected request ${url}`);
      }
    },
  });

  harnessApp.state.currentProject = { key: 'demo', name: 'Demo' };
  harnessApp.state.user = { login: 'admin' };
  harnessApp.state.view = 'project';
  harnessApp.state.projectTab = 'custom-rules';

  await harnessApp.loadCustomRulesData();

  assert.deepEqual(requests, [
    apiPrefix + '/custom-rules',
    apiPrefix + '/rule-engines',
    apiPrefix + '/custom-rules/ai/models',
    apiPrefix + '/profiles',
  ]);
  assert.equal(harnessApp.state.customRulesData[0].key, 'custom:no-debug');
  assert.equal(harnessApp.state.customRuleEngines[0].engine, 'text');
  assert.equal(harnessApp.state.customRuleAIProviders[0].id, 'mock');
  assert.equal(harnessApp.state.profilesData.profiles[0].name, 'Team Go');
  assert.match(elements.get('app').innerHTML, /Loaded rule/);
});

test('renderAIProvidersTab shows provider setup states', () => {
  const { app: harnessApp, elements } = createHarness();

  harnessApp.state.currentProject = { key: 'demo', name: 'Demo' };
  harnessApp.state.user = { login: 'admin' };
  harnessApp.state.view = 'project';
  harnessApp.state.projectTab = 'ai-providers';
  harnessApp.state.customRuleAISetupProvider = 'ollama';
  harnessApp.state.customRuleAIProviders = [
    { id: 'ollama', label: 'Ollama', status: 'unavailable', local: true, configured: false, setup_required: true, base_url: 'http://host.docker.internal:11434', message: 'Start Ollama and pull a model.' },
    { id: 'openai', label: 'OpenAI', status: 'setup_required', local: false, configured: false, setup_required: true, models: ['gpt-5.5'], message: 'Connect an OpenAI-compatible provider.' },
    { id: 'anthropic', label: 'Anthropic Claude', status: 'setup_required', local: false, configured: false, setup_required: true, models: ['claude-sonnet-4-6'], message: 'Connect Anthropic Claude.' },
    { id: 'kimi', label: 'Kimi K2', status: 'setup_required', local: false, configured: false, setup_required: true, models: ['kimi-k2.6', 'kimi-k2-thinking'], message: 'Connect Kimi.' },
    { id: 'qwen', label: 'Qwen', status: 'setup_required', local: false, configured: false, setup_required: true, models: ['qwen3.6-max-preview', 'qwen3.6-plus'], message: 'Connect Qwen.' },
  ];

  harnessApp.render();
  const html = elements.get('app').innerHTML;

  assert.match(html, /AI Providers/);
  assert.match(html, /Provider setup/);
  assert.match(html, /Ollama/);
  assert.match(html, /unavailable/);
  assert.match(html, /http:\/\/host\.docker\.internal:11434/);
  assert.match(html, /OpenAI/);
  assert.match(html, /setup required/);
  assert.match(html, /Anthropic Claude/);
  assert.match(html, /claude-sonnet-4-6/);
  assert.match(html, /Kimi K2/);
  assert.match(html, /kimi-k2\.6/);
  assert.match(html, /Qwen/);
  assert.match(html, /qwen3\.6-max-preview/);
  assert.match(html, /Back to Rule Studio/);
  assert.match(html, /custom-rule-ai-provider-card active/);
});

test('renderBackgroundTasksPage shows task details and guarded actions', () => {
  const { app: harnessApp } = createHarness();

  harnessApp.state.backgroundTasksData = { total: 0, limit: 25, offset: 0, items: [] };
  harnessApp.state.selectedBackgroundTask = {
    id: 'scan:12',
    type: 'scan',
    status: 'queued',
    internal_status: 'accepted',
    project_key: 'demo',
    attempts: 0,
    created_at: '2026-05-02T12:00:00Z',
    supported_actions: ['cancel'],
    scanner_parameters: {
      scanner_options: {
        project_dir: '/workspace/ollanta',
        project_key: 'demo',
        sources: ['./...'],
        format: 'all',
        tests: { enabled: true, mode: 'run', modules: [{ name: 'api', root: 'ollantaweb', command: 'go test ./api' }] },
      },
      analysis_scope: { branch: 'main', commit_sha: 'abc123' },
    },
    details: { source_job_type: 'scan_job', project_key: 'demo' },
  };

  const html = harnessApp.renderBackgroundTasksPage();

  assert.match(html, /scan:12/);
  assert.match(html, /task-status-queued/);
  assert.match(html, /data-task-action="cancel"/);
  assert.match(html, /Scanner parameters/);
  assert.match(html, /\/workspace\/ollanta/);
  assert.match(html, /\.\/\.\.\./);
  assert.match(html, /go test \.\/api/);
  assert.match(html, /source_job_type/);
});

test('changeScope refreshes scoped data and persists the branch in the URL', async () => {
  const { app: harnessApp, requests, historyCalls } = createHarness({
    fetchHandler(url) {
      if (url === apiPrefix + '/projects/demo/overview?branch=release') {
        return { body: { scope: { type: 'branch', branch: 'release', default_branch: 'main' } } };
      }
      if (url === apiPrefix + '/projects/demo/branches') {
        return { body: { items: [{ name: 'main', is_default: true }, { name: 'release', is_default: false }] } };
      }
      if (url === apiPrefix + '/projects/demo/pull-requests') {
        return { body: { items: [] } };
      }
      if (url === apiPrefix + '/projects/demo/information?branch=release') {
        return { body: { project: { key: 'demo', name: 'Demo' }, scope: { type: 'branch', branch: 'release', default_branch: 'main' } } };
      }
      throw new Error('unexpected fetch ' + url);
    },
  });

  harnessApp.state.currentProject = { key: 'demo' };
  harnessApp.state.projectTab = 'overview';
  harnessApp.state.scope = harnessApp.normalizeScope({ type: 'branch', branch: 'main', defaultBranch: 'main' });

  await harnessApp.changeScope({ type: 'branch', branch: 'release', defaultBranch: 'main' });

  assert.deepEqual(requests, [
    apiPrefix + '/projects/demo/overview?branch=release',
    apiPrefix + '/projects/demo/branches',
    apiPrefix + '/projects/demo/pull-requests',
    apiPrefix + '/projects/demo/information?branch=release',
  ]);
  assert.equal(harnessApp.state.scope.branch, 'release');
  assert.equal(historyCalls.at(-1).url, '?project=demo&tab=overview&branch=release');
});

test('loadProject normalizes project information links into overview details', async () => {
  const { app: harnessApp, requests, historyCalls } = createHarness({
    fetchHandler(url) {
      if (url === apiPrefix + '/projects/demo') {
        return { body: { key: 'demo', name: 'Demo', main_branch: 'main' } };
      }
      if (url === apiPrefix + '/projects/demo/overview?branch=release') {
        return {
          body: {
            scope: { type: 'branch', branch: 'release', default_branch: 'main' },
            last_scan: { analysis_date: '2026-04-21T12:00:00Z' },
          },
        };
      }
      if (url === apiPrefix + '/projects/demo/branches') {
        return { body: { items: [{ name: 'main', is_default: true }, { name: 'release', is_default: false }] } };
      }
      if (url === apiPrefix + '/projects/demo/pull-requests') {
        return { body: { items: [] } };
      }
      if (url === apiPrefix + '/projects/demo/information?branch=release') {
        return {
          body: {
            project: { key: 'demo', name: 'Demo', main_branch: 'main' },
            scope: { type: 'branch', branch: 'release', default_branch: 'main' },
            measures: { files: 4, lines: 24, ncloc: 18, issues: 2 },
            code_snapshot: { stored_files: 1, total_files: 1, truncated_files: 1, omitted_files: 0, stored_bytes: 256, max_file_bytes: 128, max_total_bytes: 512 },
          },
        };
      }
      throw new Error('unexpected fetch ' + url);
    },
  });

  await harnessApp.loadProject('demo', { project: 'demo', tab: 'information', branch: 'release', pullRequest: '' });

  assert.equal(harnessApp.state.projectTab, 'overview');
  assert.equal(harnessApp.state.scope.branch, 'release');
  assert.ok(requests.includes(apiPrefix + '/projects/demo/overview?branch=release'));
  assert.ok(requests.includes(apiPrefix + '/projects/demo/information?branch=release'));
  assert.equal(historyCalls.at(-1).url, '?project=demo&tab=overview&branch=release');
  const html = harnessApp.renderOverviewTab();
  assert.match(html, /Project Details/);
  assert.match(html, /Current scope/);
  assert.match(html, /Measures/);
  assert.match(html, /Code snapshot/);
  assert.match(html, /Stored bytes/);
});

test('changeScope refreshes overview project details for a selected pull request', async () => {
  const { app: harnessApp, requests, historyCalls } = createHarness({
    fetchHandler(url) {
      if (url === apiPrefix + '/projects/demo/overview?pull_request=128') {
        return { body: { scope: { type: 'pull_request', branch: 'feature/login', pull_request_key: '128', pull_request_base: 'main', default_branch: 'main' } } };
      }
      if (url === apiPrefix + '/projects/demo/branches') {
        return { body: { items: [{ name: 'main', is_default: true }] } };
      }
      if (url === apiPrefix + '/projects/demo/pull-requests') {
        return { body: { items: [{ key: '128', branch: 'feature/login', base_branch: 'main' }] } };
      }
      if (url === apiPrefix + '/projects/demo/information?pull_request=128') {
        return {
          body: {
            project: { key: 'demo', name: 'Demo', main_branch: 'main' },
            scope: { type: 'pull_request', branch: 'feature/login', pull_request_key: '128', pull_request_base: 'main', default_branch: 'main' },
            measures: { files: 2, lines: 12, ncloc: 10, issues: 1 },
            code_snapshot: { stored_files: 1, total_files: 1, max_file_bytes: 128, max_total_bytes: 512 },
          },
        };
      }
      throw new Error('unexpected fetch ' + url);
    },
  });

  harnessApp.state.currentProject = { key: 'demo' };
  harnessApp.state.projectTab = 'information';
  harnessApp.state.scope = harnessApp.normalizeScope({ type: 'branch', branch: 'main', defaultBranch: 'main' });

  await harnessApp.changeScope({ type: 'pull_request', pullRequestKey: '128', branch: 'feature/login', pullRequestBase: 'main', defaultBranch: 'main' });

  assert.ok(requests.includes(apiPrefix + '/projects/demo/information?pull_request=128'));
  assert.equal(harnessApp.state.scope.pullRequestKey, '128');
  assert.equal(historyCalls.at(-1).url, '?project=demo&tab=overview&pull_request=128');
  assert.match(harnessApp.renderOverviewTab(), /Pull request/i);
  assert.match(harnessApp.renderOverviewTab(), /128/);
});

test('loadCodeTreeData fetches the scoped file and renderCoverageTab shows issue markers', async () => {
  const { app: harnessApp, requests } = createHarness({
    fetchHandler(url) {
      if (url === apiPrefix + '/projects/demo/code/tree?branch=release') {
        return {
          body: {
            code_snapshot: { stored_files: 1, total_files: 1 },
            items: [{ path: 'src/app.go', line_count: 2, size_bytes: 18, is_omitted: false }],
          },
        };
      }
      if (url === apiPrefix + '/projects/demo/code/file?path=src%2Fapp.go&branch=release') {
        return {
          body: {
            file: {
              path: 'src/app.go',
              language: 'go',
              content: 'line one\nline two',
              line_count: 2,
              size_bytes: 18,
            },
            issues: [{ rule_key: 'go:no-large-functions', severity: 'major', line: 2, message: 'Too large' }],
          },
        };
      }
      throw new Error('unexpected fetch ' + url);
    },
  });

  harnessApp.state.currentProject = { key: 'demo' };
  harnessApp.state.scope = harnessApp.normalizeScope({ type: 'branch', branch: 'release', defaultBranch: 'main' });
  harnessApp.state.overviewData = {
    summary: {
      coverage: {
        coverage: 50,
        files: [{ component_path: 'src/app.go', lines_to_cover: 2, covered_lines: 1, covered_line_numbers: [1], uncovered_line_numbers: [2], coverage: 50 }],
      },
    },
  };

  await harnessApp.loadCodeTreeData();

  assert.deepEqual(requests, [
    apiPrefix + '/projects/demo/code/tree?branch=release',
    apiPrefix + '/projects/demo/code/file?path=src%2Fapp.go&branch=release',
  ]);
  const html = harnessApp.renderCoverageTab();
  assert.match(html, /src\/app\.go/);
  assert.match(html, /go:no-large-functions/);
  assert.match(html, /has-issue/);
  assert.match(html, /sev-major/);
  assert.match(html, /not covered/);
});

test('loadIssues includes tracking_state when lifecycle filter is active', async () => {
  const { app: harnessApp, requests } = createHarness({
    fetchHandler(url) {
      if (url.startsWith(apiPrefix + '/issues?')) {
        return { body: { items: [], total: 0 } };
      }
      if (url.startsWith(apiPrefix + '/issues/facets?')) {
        return { body: { by_lifecycle: { reopened: 3 } } };
      }
      throw new Error('unexpected fetch ' + url);
    },
  });

  harnessApp.state.currentProject = { key: 'demo' };
  harnessApp.state.scope = harnessApp.normalizeScope({ type: 'branch', branch: 'release', defaultBranch: 'main' });
  harnessApp.state.issueFilter.trackingState = 'reopened';

  await harnessApp.loadIssues();

  assert.equal(requests.length, 2);
  assert.match(requests[0], /\/issues\?/);
  assert.match(requests[0], /tracking_state=reopened/);
  assert.match(requests[0], /branch=release/);
  assert.match(requests[1], /\/issues\/facets\?/);
  assert.match(requests[1], /tracking_state=reopened/);
  assert.match(requests[1], /branch=release/);
});

test('renderIssuesSection shows lifecycle badges for tracked issues', () => {
  const { app: harnessApp, elements } = createHarness();

  harnessApp.state.issues = [{
    id: 7,
    severity: 'major',
    type: 'bug',
    tracking_state: 'new',
    rule_key: 'go:new-bug',
    component_path: 'internal/service.go',
    line: 12,
    message: 'new bug',
    status: 'open',
  }];
  harnessApp.state.issuesTotal = 1;
  harnessApp.state.issueFacets = { by_lifecycle: { new: 1, unknown: 2 } };

  harnessApp.renderIssuesSection();

  const html = elements.get('issues-section').innerHTML;
  assert.match(html, /Lifecycle/);
  assert.match(html, />Current</);
  assert.match(html, /issue-track track-new/);
  assert.match(html, />New</);
});

test('renderIssuesSection shows issue quality and testability facets', () => {
  const { app: harnessApp, elements } = createHarness();

  harnessApp.state.issues = [];
  harnessApp.state.issuesTotal = 0;
  harnessApp.state.issueFacets = {
    by_quality: { security: 2, testability: 1 },
    by_security_category: { injection: 2 },
    by_language: { go: 3 },
  };

  harnessApp.renderIssuesSection();

  const html = elements.get('issues-section').innerHTML;
  assert.match(html, /Software Quality/);
  assert.match(html, /Security/);
  assert.match(html, /Testability/);
  assert.match(html, /Security Category/);
  assert.match(html, /injection/);
  assert.match(html, /Language/);
});

test('renderIssuesSection exposes search and expand controls for long facet groups', () => {
  const { app: harnessApp, elements } = createHarness();
  const rules = {};
  for (let i = 0; i < 10; i += 1) rules[`go:rule-${i}`] = 10 - i;

  harnessApp.state.issueFacets = { by_rule: rules };
  harnessApp.renderIssuesSection();

  const html = elements.get('issues-section').innerHTML;
  assert.match(html, /data-facet-search="rule"/);
  assert.match(html, /Show more \(2\)/);
});

test('renderOverviewTab keeps overview sections intact after helper extraction', () => {
  const { app: harnessApp } = createHarness();

  harnessApp.state.overviewData = {
    last_scan: {
      version: '1.2.3',
      branch: 'main',
      commit_sha: 'abcdef123456',
      status: 'completed',
      analysis_date: '2026-04-21T12:00:00Z',
      elapsed_ms: 1200,
    },
    quality_gate: { status: 'OK' },
    measures: {
      bugs: 1,
      vulnerabilities: 0,
      code_smells: 3,
      coverage: 82.1,
      duplicated_lines_density: 2.5,
      mutation_score: 72.5,
      mutants_total: 40,
      mutants_killed: 29,
      mutants_survived: 11,
      ncloc: 200,
    },
    facets: {
      by_severity: { major: 2 },
      by_type: { bug: 1, code_smell: 2 },
      by_file: { 'src/foo/bar.js': 3, 'src/app.js': 1 },
    },
    new_code: { new_issues: 2, closed_issues: 1 },
  };

  const html = harnessApp.renderOverviewTab();
  assert.match(html, /Quality Gate/);
  assert.match(html, /New Code/);
  assert.match(html, /Hotspot Files/);
  assert.match(html, /Mutation Testing/);
  assert.match(html, /72\.5%/);
  assert.match(html, /src\/foo\/bar\.js/);
  assert.match(html, /Latest Scan/);
  assert.match(html, /1\.2s/);
});

test('renderOverviewTab prefers review summary when summary payload is available', () => {
  const { app: harnessApp } = createHarness();

  harnessApp.state.overviewData = {
    last_scan: {
      version: '2.0.0',
      branch: 'feature/payments',
      commit_sha: 'abcdef123456',
      status: 'completed',
      analysis_date: '2026-05-01T12:00:00Z',
      elapsed_ms: 2400,
    },
    summary: {
      review: {
        status: 'blocked',
        headline: 'Review blocked by new vulnerabilities',
        reasons: [
          { metric: 'new_vulnerabilities', label: 'New vulnerabilities', actual: 1, threshold: 0, operator: 'GT' },
        ],
      },
      new_code: {
        baseline: { label: 'Reference branch: main' },
        metrics: {
          new_issues: 4,
          new_bugs: 0,
          new_vulnerabilities: 1,
          new_code_smells: 3,
          new_coverage: 54,
          changed_mutation_score: 66,
          changed_mutants_survived: 2,
          new_duplications: 2,
          closed_issues: 1,
        },
      },
      must_fix_now: [
        {
          component_path: 'internal/auth/repo.go',
          line: 84,
          message: 'SQL query uses unsanitized input',
          severity: 'critical',
          why_selected: 'matches failing quality gate',
        },
      ],
      impacted_files: [
        {
          component_path: 'internal/auth/repo.go',
          issue_count: 2,
          coverage: 45,
        },
      ],
      overall_code: {
        metrics: {
          bugs: 2,
          vulnerabilities: 1,
          code_smells: 8,
          coverage: 71.2,
          duplicated_lines_density: 1.8,
          changed_mutation_score: 66,
          changed_mutants_total: 10,
          changed_mutants_killed: 8,
          changed_mutants_survived: 2,
          ncloc: 58240,
        },
      },
    },
  };

  const html = harnessApp.renderOverviewTab();
  assert.match(html, /Review Summary/);
  assert.match(html, /New Code Focus/);
  assert.match(html, /Changed Mutation/);
  assert.match(html, /Survived Mutants/);
  assert.match(html, /Must Fix Now/);
  assert.match(html, /Impacted Files/);
  assert.match(html, /Overall Snapshot/);
  assert.match(html, /Reference branch: main/);
  assert.match(html, /internal\/auth\/repo\.go/);
});

test('renderOverviewTab shows the empty state when there is no overview data', () => {
  const { app: harnessApp } = createHarness();

  harnessApp.state.overviewData = null;

  const html = harnessApp.renderOverviewTab();
  assert.match(html, /No scans yet for this project/);
});

const activityModuleUrl = pathToFileURL(path.join(__dirname, 'app/features/activity.js')).href;
const activity = await import(activityModuleUrl);

test('pickTestSignal prioritises failing tests', () => {
  const signal = activity.pickTestSignal({ test_failures: 2, test_errors: 1, coverage: 90 });
  assert.equal(signal.kind, 'failing');
  assert.equal(signal.value, 3);
});

test('pickTestSignal falls back to mutation score when no failures', () => {
  const signal = activity.pickTestSignal({ mutation_score: 76.5 });
  assert.equal(signal.kind, 'mutation');
});

test('pickTestSignal falls back to coverage when only coverage present', () => {
  const signal = activity.pickTestSignal({ coverage: 64 });
  assert.equal(signal.kind, 'coverage');
});

test('pickTestSignal returns none when no test data is available', () => {
  const signal = activity.pickTestSignal({});
  assert.equal(signal.kind, 'none');
});

test('applyActivityFilters returns all entries when no filter is set', () => {
  const items = [
    { analysis_date: new Date().toISOString(), events: [{ category: 'ANALYSIS', name: 'a' }] },
    { analysis_date: new Date().toISOString(), events: [{ category: 'QUALITY_GATE', name: 'b' }] },
  ];
  assert.equal(activity.applyActivityFilters(items, { categories: [], window: 'all' }).length, 2);
});

test('applyActivityFilters keeps entries matching a selected category', () => {
  const items = [
    { analysis_date: new Date().toISOString(), events: [{ category: 'QUALITY_GATE', name: 'gate change' }] },
    { analysis_date: new Date().toISOString(), events: [{ category: 'VERSION', name: 'v1.2' }] },
    { analysis_date: new Date().toISOString(), events: [] },
  ];
  const filtered = activity.applyActivityFilters(items, { categories: ['QUALITY_GATE', 'ANALYSIS'], window: 'all' });
  assert.equal(filtered.length, 2);
});

test('applyActivityFilters drops entries outside the time window', () => {
  const old = new Date(Date.now() - 100 * 24 * 60 * 60 * 1000).toISOString();
  const fresh = new Date().toISOString();
  const items = [
    { analysis_date: old, events: [] },
    { analysis_date: fresh, events: [] },
  ];
  const filtered = activity.applyActivityFilters(items, { categories: [], window: '7d' });
  assert.equal(filtered.length, 1);
  assert.equal(filtered[0].analysis_date, fresh);
});
