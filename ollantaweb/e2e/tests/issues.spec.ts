import { test, expect } from '@playwright/test';

test.describe('issues', () => {
  test('list loads', async ({ page }) => {
    await page.goto('/');
    await page.locator('.project-card').first().click();
    await page.locator('.tab-btn[data-tab="issues"]').click();

    await expect(page.locator('.issues-table tbody tr[data-issue-idx]')).toHaveCount(3);
  });

  test('filter by severity', async ({ page }) => {
    await page.goto('/');
    await page.locator('.project-card').first().click();
    await page.locator('.tab-btn[data-tab="issues"]').click();

    // Wait for facet sidebar and table
    await expect(page.locator('.issue-facet-sidebar')).toBeVisible();
    await expect(page.locator('.issues-table tbody tr')).toHaveCount(3);

    // Click the "major" severity facet
    const majorFacet = page.locator('.facet-option[data-facet-field="severity"][data-facet-value="major"]');
    await expect(majorFacet).toBeVisible();
    await majorFacet.click();

    // Wait for the facet to become active and the table to update
    await expect(majorFacet).toHaveClass(/active/);
    await expect(page.locator('.issues-table tbody tr')).toHaveCount(1);
  });

  test('click issue opens details', async ({ page }) => {
    await page.goto('/');
    await page.locator('.project-card').first().click();
    await page.locator('.tab-btn[data-tab="issues"]').click();

    await expect(page.locator('.issues-table tbody tr').first()).toBeVisible();

    const firstRow = page.locator('.issues-table tbody tr').first();
    await firstRow.click();

    // Detail panel should open
    await expect(page.locator('#detail-panel')).toBeVisible();
    await expect(page.locator('.detail-title')).toBeVisible();
    await expect(page.locator('#detailClose')).toBeVisible();

    // Close detail
    await page.click('#detailClose');
    await expect(page.locator('#detail-panel')).toBeHidden();
  });
});
