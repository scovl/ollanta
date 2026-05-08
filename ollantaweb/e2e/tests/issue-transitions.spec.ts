import { test, expect } from '@playwright/test';

test.describe('issue transitions', () => {
  test('transition to false_positive', async ({ page }) => {
    await page.goto('/');
    await page.locator('.project-card').first().click();
    await page.locator('.tab-btn[data-tab="issues"]').click();

    await expect(page.locator('.issues-table tbody tr')).toHaveCount.greaterThan(0);

    const firstRow = page.locator('.issues-table tbody tr').first();

    // Verify the row is open (has FP/WF/OK buttons)
    await expect(firstRow.locator('.itbtn.fp-btn')).toBeVisible();

    // Click False Positive
    await firstRow.locator('.itbtn.fp-btn').click();

    // Wait for the row to update: closed rows have the "row-closed" class
    // and show the reopen button instead
    await expect(firstRow).toHaveClass(/row-closed/);
    await expect(firstRow.locator('.itbtn.re-btn')).toBeVisible();
    await expect(firstRow.locator('.itbtn.fp-btn')).not.toBeVisible();
  });
});
