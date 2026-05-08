export const ISSUE_PAGE = 50;

export const SEV_ORDER = ['blocker', 'critical', 'major', 'minor', 'info'];
export const SEV_COLOR = {
  blocker: '#ef4444',
  critical: '#f97316',
  major: '#eab308',
  minor: '#22c55e',
  info: '#64748b',
};
export const SEV_BG = {
  blocker: 'rgba(239,68,68,.12)',
  critical: 'rgba(249,115,22,.10)',
  major: 'rgba(234,179,8,.09)',
  minor: 'rgba(34,197,94,.09)',
  info: 'rgba(100,116,139,.09)',
};
export const SEV_LABEL = {
  blocker: 'Blocker',
  critical: 'Critical',
  major: 'Major',
  minor: 'Minor',
  info: 'Info',
};
export const TYPE_ICON = {
  bug: '\uD83D\uDC1B',
  code_smell: '\uD83C\uDF3F',
  vulnerability: '\uD83D\uDD12',
  security_hotspot: '\uD83D\uDD0E',
};
export const TYPE_COLOR = {
  bug: '#ef4444',
  code_smell: '#22c55e',
  vulnerability: '#f97316',
  security_hotspot: '#eab308',
};
export const TYPE_LABEL = {
  bug: 'Bug',
  code_smell: 'Code Smell',
  vulnerability: 'Vulnerability',
  security_hotspot: 'Security Hotspot',
};

export const QUALITY_LABEL = {
  security: 'Security',
  reliability: 'Reliability',
  maintainability: 'Maintainability',
  testability: 'Testability',
};

export function emptyScope() {
  return { type: 'branch', branch: '', pullRequestKey: '', pullRequestBase: '', defaultBranch: '' };
}

export function createInitialState() {
  return {
    user: null,
    uiSettings: { observabilityLinks: [] },
    view: 'login',
    projects: [],
    currentProject: null,
    currentScan: null,
    scope: emptyScope(),
    overviewData: null,
    issues: [],
    issueFacets: null,
    issuesTotal: 0,
    issueOffset: 0,
    issueFilter: { quality: 'all', severity: 'all', type: 'all', status: 'all', trackingState: 'all', language: 'all', rule: 'all', tag: 'all', securityCategory: 'all', directory: 'all', file: 'all', search: '', scanId: '' },
    issueFacetExpanded: {},
    issueFacetSearch: {},
    loading: false,
    loadingIssues: false,
    projectTab: 'overview',
    gateData: null,
    webhooksData: null,
    profilesData: null,
    customRulesData: null,
    customRuleEngines: null,
    customRuleAIProviders: null,
    customRuleAISetupProvider: '',
    customRuleFilters: { search: '', lifecycle: 'all' },
    expandedCustomRuleId: null,
    editingCustomRuleId: null,
    activityData: null,
    activityFilters: { categories: [], window: 'all' },
    activityPage: { offset: 0, hasMore: false },
    activityCompareSelection: [],
    loadingMoreActivity: false,
    branchesData: null,
    pullRequestsData: null,
    projectInfoData: null,
    codeTreeData: null,
    codeFileData: null,
    codeSelectedPath: '',
    newCodePeriod: null,
    selectedIssue: null,
    backgroundTasksData: null,
    backgroundTasksSummary: null,
    backgroundTaskFilters: null,
    selectedBackgroundTask: null,
    loadingBackgroundTasks: false,
    loadingBackgroundTaskDetail: false,
    backgroundTaskError: '',
    tagCatalogData: null,
    tagDetailData: null,
    selectedTagKey: '',
    savedFiltersData: null,
    loadingTags: false,
    tagSearch: '',
    tagBulkPreview: null,
    tagsTab: 'catalog',
    tagCreatorOpen: false,
    tagSourceFilter: 'all',
    projectFilter: '',
    paletteOpen: false,
    paletteQuery: '',
    paletteResults: null,
    paletteLoading: false,
    paletteIndex: 0,
    survivedMutantsData: null,
    survivedMutantsError: '',
    survivedMutantsFilters: { module: '', changedOnly: false },
    survivedMutantsSort: { field: 'file', dir: 'asc' },
    loadingSurvivedMutants: false,
  };
}

export const state = createInitialState();

export function replaceState(nextState) {
  Object.keys(state).forEach(key => { delete state[key]; });
  Object.assign(state, nextState);
  return state;
}

export function resetProjectState() {
  state.currentProject = null;
  state.currentScan = null;
  state.scope = emptyScope();
  state.overviewData = null;
  state.issues = [];
  state.issueFacets = null;
  state.issuesTotal = 0;
  state.issueOffset = 0;
  state.issueFilter = { quality: 'all', severity: 'all', type: 'all', status: 'all', trackingState: 'all', language: 'all', rule: 'all', tag: 'all', securityCategory: 'all', directory: 'all', file: 'all', search: '', scanId: '' };
  state.issueFacetExpanded = {};
  state.issueFacetSearch = {};
  state.projectTab = 'overview';
  state.gateData = null;
  state.webhooksData = null;
  state.profilesData = null;
  state.customRulesData = null;
  state.customRuleEngines = null;
  state.customRuleAIProviders = null;
  state.customRuleAISetupProvider = '';
  state.customRuleFilters = { search: '', lifecycle: 'all' };
  state.expandedCustomRuleId = null;
  state.editingCustomRuleId = null;
  state.activityData = null;
  state.activityFilters = { categories: [], window: 'all' };
  state.activityPage = { offset: 0, hasMore: false };
  state.activityCompareSelection = [];
  state.loadingMoreActivity = false;
  state.branchesData = null;
  state.pullRequestsData = null;
  state.projectInfoData = null;
  state.codeTreeData = null;
  state.codeFileData = null;
  state.codeSelectedPath = '';
  state.newCodePeriod = null;
  state.selectedIssue = null;
  state.backgroundTasksData = null;
  state.backgroundTasksSummary = null;
  state.backgroundTaskFilters = null;
  state.selectedBackgroundTask = null;
  state.loadingBackgroundTasks = false;
  state.loadingBackgroundTaskDetail = false;
  state.backgroundTaskError = '';
  state.tagDetailData = null;
  state.selectedTagKey = '';
  state.survivedMutantsData = null;
  state.survivedMutantsError = '';
  state.survivedMutantsFilters = { module: '', changedOnly: false };
  state.survivedMutantsSort = { field: 'file', dir: 'asc' };
  state.loadingSurvivedMutants = false;
}
