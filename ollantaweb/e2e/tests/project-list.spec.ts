import { test, expect } from '@playwright/test';
import { apiCall } from './helpers';

test.describe('project list', () => {
  test('create project appears in list', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('.project-card').first()).toBeVisible();

    const before = await page.locator('.project-card').count();

    const result = await apiCall(page, 'POST', '/projects', {
      key: 'e2e-new-project',
      name: 'E2E New Project',
    });
    expect(result.status).toBe(201);

    await page.reload();
    await expect(page.locator('.project-card[data-key="e2e-new-project"]')).toBeVisible();

    const after = await page.locator('.project-card').count();
    expect(after).toBe(before + 1);
  });
});
