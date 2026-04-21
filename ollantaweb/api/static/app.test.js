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
  assert.match(html, /<option value="main">main · default<\/option>/);
  assert.doesNotMatch(html, /<option value="">Default branch<\/option>/);
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

test('renderOverviewTab shows the empty state when there is no overview data', () => {
  const { app: harnessApp } = createHarness();

  harnessApp.state.overviewData = null;

  const html = harnessApp.renderOverviewTab();
  assert.match(html, /No scans yet for this project/);
});