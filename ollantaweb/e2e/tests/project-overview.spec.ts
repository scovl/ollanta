import { test, expect } from '@playwright/test';

test.describe('project overview', () => {
  test('click project and tabs load', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('.project-card').first()).toBeVisible();

    // Click the first project card
    await page.locator('.project-card').first().click();

    // Wait for project header and tabs
    await expect(page.locator('.detail-header')).toBeVisible();
    await expect(page.locator('.proj-tabs')).toBeVisible();

    // Overview tab is active by default
    await expect(page.locator('.tab-btn[data-tab="overview"]')).toHaveClass(/active/);

    // Click Issues tab
    await page.locator('.tab-btn[data-tab="issues"]').click();
    await expect(page.locator('#issues-section')).toBeVisible();

    // Click Coverage tab
    await page.locator('.tab-btn[data-tab="coverage"]').click();
    await expect(page.locator('.coverage-tree, .empty-state')).toBeVisible();

    // Click Activity tab
    await page.locator('.tab-btn[data-tab="activity"]').click();
    await expect(page.locator('.activity-list, .empty-state')).toBeVisible();
  });
});
