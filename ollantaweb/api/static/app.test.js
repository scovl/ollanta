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
      elements.set(id, {
        id,
        innerHTML: '',
        value: '',
        addEventListener() {},
        classList: { add() {}, remove() {} },
      });
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
    querySelectorAll() { return []; },
    createElement() { return { classList: { add() {}, remove() {} }, remove() {}, textContent: '' }; },
    body: { appendChild() {} },
  };
  globalThis.fetch = async (url, requestOptions) => {
    const requestUrl = typeof url === 'string' ? url : url instanceof URL ? url.href : String(url);
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
  assert.doesNotMatch(html, /href="\/api\/v1\/system\/info"/);
  assert.match(html, /href="\/metrics"/);
  assert.match(html, /https:\/\/app.datadoghq.com\/dashboard\/abc/);
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
  assert.match(html, /\/api\/v1\/system\/info/);
  assert.match(html, /\/metrics/);
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
  assert.match(html, /<option value="main">main Ã‚Â· default<\/option>/);
  assert.doesNotMatch(html, /<option value="">Default branch<\/option>/);
});

test('renderAdminLinksTab exposes API and observability shortcuts', () => {
  const { app: harnessApp } = createHarness();

  harnessApp.state.currentProject = { key: 'demo' };
  harnessApp.state.uiSettings = { observabilityLinks: [{ label: 'Grafana', href: 'https://grafana.example.com' }] };

  const html = harnessApp.renderAdminLinksTab();

  assert.match(html, /data-admin-api="\/system\/info"/);
  assert.match(html, /data-admin-api="\/projects\/demo\/permissions"/);
  assert.match(html, /https:\/\/grafana.example.com/);
  assert.doesNotMatch(html, /http:\/\/localhost:9091\/targets/);
  assert.doesNotMatch(html, /http:\/\/localhost:16686/);
  assert.match(html, /\/metrics/);
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
  ]);
  assert.equal(harnessApp.state.scope.branch, 'release');
  assert.equal(historyCalls.at(-1).url, '?project=demo&tab=overview&branch=release');
});

test('loadProject preserves branch scope in reloads and refreshes project information', async () => {
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
            code_snapshot: { stored_files: 1, total_files: 1, max_file_bytes: 128, max_total_bytes: 512 },
          },
        };
      }
      throw new Error('unexpected fetch ' + url);
    },
  });

  await harnessApp.loadProject('demo', { project: 'demo', tab: 'information', branch: 'release', pullRequest: '' });

  assert.equal(harnessApp.state.projectTab, 'information');
  assert.equal(harnessApp.state.scope.branch, 'release');
  assert.ok(requests.includes(apiPrefix + '/projects/demo/overview?branch=release'));
  assert.ok(requests.includes(apiPrefix + '/projects/demo/information?branch=release'));
  assert.equal(historyCalls.at(-1).url, '?project=demo&tab=information&branch=release');
  assert.match(harnessApp.renderProjectInformationTab(), /release/);
});

test('changeScope refreshes project information for a selected pull request', async () => {
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
  assert.equal(historyCalls.at(-1).url, '?project=demo&tab=information&pull_request=128');
  assert.match(harnessApp.renderProjectInformationTab(), /Pull request/i);
});

test('loadCodeTreeData fetches the scoped file and renderCodeTab shows issue markers', async () => {
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

  await harnessApp.loadCodeTreeData();

  assert.deepEqual(requests, [
    apiPrefix + '/projects/demo/code/tree?branch=release',
    apiPrefix + '/projects/demo/code/file?path=src%2Fapp.go&branch=release',
  ]);
  const html = harnessApp.renderCodeTab();
  assert.match(html, /src\/app\.go/);
  assert.match(html, /go:no-large-functions/);
  assert.match(html, /has-issue/);
  assert.match(html, /sev-major/);
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
          ncloc: 58240,
        },
      },
    },
  };

  const html = harnessApp.renderOverviewTab();
  assert.match(html, /Review Summary/);
  assert.match(html, /New Code Focus/);
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