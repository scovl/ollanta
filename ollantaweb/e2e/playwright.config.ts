import { defineConfig, devices } from '@playwright/test';
import * as path from 'path';

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  workers: 4,
  timeout: 30000,
  globalSetup: require.resolve('./global-setup'),
  use: {
    baseURL: 'http://localhost:8080',
    storageState: path.join(__dirname, 'playwright/.auth/user.json'),
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  reporter: [
    ['list'],
    ['html', { open: 'never', outputFolder: 'playwright-report' }],
  ],
});
