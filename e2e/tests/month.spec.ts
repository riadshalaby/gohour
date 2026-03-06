import { expect, test } from '@playwright/test';

test('Month page loads', async ({ page }) => {
  await page.goto('/month/2025-01');

  await expect(page).toHaveTitle(/2025-01/);
  await expect(page.locator('.stat-card')).toHaveCount(6);
  await expect(page.locator('#month-rows tr')).toHaveCount(31);
});

test('Month navigation', async ({ page }) => {
  await page.goto('/month/2025-01');

  await page.getByLabel('Next month').click();
  await expect(page).toHaveURL(/\/month\/2025-02$/);
});

test('Refresh remote shows error and clears spinner', async ({ page }) => {
  await page.goto('/month/2025-01');

  await page.getByRole('button', { name: /actions/i }).click();
  await page.getByRole('menuitem', { name: 'Refresh remote' }).click();

  await expect(page.locator('#month-rows .dialog-error')).toBeVisible();
  await expect(page.locator('.toast')).toContainText('Failed to refresh remote data.');
  await expect(page.locator('#month-refresh-head')).not.toHaveClass(/htmx-request/);
});
