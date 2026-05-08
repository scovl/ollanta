import { test, expect } from '@playwright/test';

test.describe('command palette', () => {
  test('Ctrl+K opens palette', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('#logoutBtn')).toBeVisible();

    // Palette should not be visible initially
    await expect(page.locator('#paletteOverlay')).not.toBeVisible();

    // Open with keyboard shortcut
    await page.keyboard.press('ControlOrMeta+k');
    await expect(page.locator('#paletteOverlay')).toBeVisible();
    await expect(page.locator('#paletteInput')).toBeVisible();

    // Static pages should appear in the empty query state
    await expect(page.locator('.palette-item').first()).toBeVisible();

    // Type a query
    await page.fill('#paletteInput', 'Projects');
    await expect(page.locator('.palette-item')).toBeVisible();

    // Close with Escape
    await page.keyboard.press('Escape');
    await expect(page.locator('#paletteOverlay')).not.toBeVisible();
  });
});
