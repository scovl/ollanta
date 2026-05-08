import { request } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

const PROJECT_KEY = 'e2e-test-project';
const AUTH_DIR = path.resolve(__dirname, 'playwright/.auth');
const AUTH_FILE = path.join(AUTH_DIR, 'user.json');
const META_FILE = path.join(AUTH_DIR, 'project.json');

async function waitForScanJob(apiContext: any, jobId: number, maxAttempts = 60) {
  for (let i = 0; i < maxAttempts; i++) {
    const res = await apiContext.get(`/api/v1/scan-jobs/${jobId}`);
    if (res.status() === 200) {
      const job = await res.json();
      if (job.status === 'completed') return job;
      if (job.status === 'failed') {
        throw new Error(`Scan job ${jobId} failed: ${job.last_error || 'unknown error'}`);
      }
    }
    await new Promise((r) => setTimeout(r, 1000));
  }
  throw new Error(`Scan job ${jobId} did not complete within ${maxAttempts}s`);
}

export default async function globalSetup() {
  fs.mkdirSync(AUTH_DIR, { recursive: true });

  const apiContext = await request.newContext({
    baseURL: 'http://localhost:8080',
  });

  // 1. Login as default admin
  const loginRes = await apiContext.post('/api/v1/auth/login', {
    data: { login: 'admin', password: 'admin' },
  });
  if (!loginRes.ok()) {
    const body = await loginRes.text().catch(() => '');
    throw new Error(`Global setup login failed (${loginRes.status()}): ${body}`);
  }
  const loginData = await loginRes.json();
  const token: string = loginData.access_token;

  // 2. Create / update test project
  const projectRes = await apiContext.post('/api/v1/projects', {
    headers: { Authorization: `Bearer ${token}` },
    data: { key: PROJECT_KEY, name: 'E2E Test Project' },
  });
  if (!projectRes.ok() && projectRes.status() !== 201) {
    const body = await projectRes.text().catch(() => '');
    throw new Error(`Global setup project creation failed (${projectRes.status()}): ${body}`);
  }

  // 3. Submit a scan with sample issues
  const scanPayload = {
    metadata: {
      project_key: PROJECT_KEY,
      analysis_date: new Date().toISOString(),
      version: '1.0.0',
      elapsed_ms: 1000,
      branch: 'main',
    },
    measures: {
      files: 10,
      lines: 500,
      ncloc: 300,
      comments: 50,
      bugs: 1,
      code_smells: 2,
      vulnerabilities: 1,
      by_language: { go: 300 },
    },
    issues: [
      {
        rule_key: 'go:no-large-functions',
        message: 'Function has too many lines',
        type: 'code_smell',
        severity: 'major',
        component_path: 'main.go',
        line: 10,
        language: 'go',
        quality_domain: 'maintainability',
      },
      {
        rule_key: 'go:missing-doc',
        message: 'Exported function lacks documentation',
        type: 'code_smell',
        severity: 'minor',
        component_path: 'main.go',
        line: 20,
        language: 'go',
        quality_domain: 'maintainability',
      },
      {
        rule_key: 'go:err-check',
        message: 'Unchecked error return',
        type: 'bug',
        severity: 'critical',
        component_path: 'main.go',
        line: 30,
        language: 'go',
        quality_domain: 'reliability',
      },
    ],
  };

  const scanRes = await apiContext.post('/api/v1/scans', {
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    data: scanPayload,
  });
  if (!scanRes.ok()) {
    const body = await scanRes.text().catch(() => '');
    throw new Error(`Global setup scan ingest failed (${scanRes.status()}): ${body}`);
  }
  const scanJob = await scanRes.json();

  // 4. Wait for the background worker to process the job
  await waitForScanJob(apiContext, scanJob.id);

  // 5. Persist auth state for tests
  const state = {
    cookies: [],
    origins: [
      {
        origin: 'http://localhost:8080',
        localStorage: [
          { name: 'olt_token', value: token },
          { name: 'olt_user', value: JSON.stringify(loginData.user) },
        ],
      },
    ],
  };
  fs.writeFileSync(AUTH_FILE, JSON.stringify(state, null, 2));
  fs.writeFileSync(META_FILE, JSON.stringify({ projectKey: PROJECT_KEY }, null, 2));

  await apiContext.dispose();
}
