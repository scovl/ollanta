import { test, expect } from '@playwright/test';

test.describe('auth', () => {
  test.use({ storageState: { cookies: [], origins: [] } });

  test('valid login', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('#loginBtn')).toBeVisible();

    await page.fill('#loginUser', 'admin');
    await page.fill('#loginPass', 'admin');
    await page.click('#loginBtn');

    await expect(page.locator('#logoutBtn')).toBeVisible();
    await expect(page.locator('.projects-grid, .empty-state')).toBeVisible();
  });

  test('invalid login shows error', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('#loginBtn')).toBeVisible();

    await page.fill('#loginUser', 'admin');
    await page.fill('#loginPass', 'wrongpassword');
    await page.click('#loginBtn');

    await expect(page.locator('#loginError')).not.toBeEmpty();
    await expect(page.locator('#loginBtn')).toBeEnabled();
  });
});

test.describe('logout', () => {
  test('logout returns to login', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('#logoutBtn')).toBeVisible();

    await page.click('#logoutBtn');

    await expect(page.locator('#loginBtn')).toBeVisible();
    await expect(page.locator('#loginUser')).toBeVisible();
    await expect(page.locator('#loginPass')).toBeVisible();
  });
});
