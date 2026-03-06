import { expect, test } from '@playwright/test';

test.describe.configure({ mode: 'serial' });

test('Day page loads', async ({ page }) => {
  await page.goto('/day/2025-01-02');

  await expect(page.locator('.day-page .stat-card')).toHaveCount(4);
  await expect(page.locator('#day-entries')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Add new worklog entry' })).toBeVisible();
});

test('Day navigation', async ({ page }) => {
  await page.goto('/day/2025-01-02');

  await page.keyboard.press('ArrowRight');
  await expect(page).toHaveURL(/\/day\/2025-01-03$/);
});

test('Add entry dialog opens', async ({ page }) => {
  await page.goto('/day/2025-01-02');

  await page.getByRole('button', { name: 'Add new worklog entry' }).click();
  await expect(page.locator('#edit-dialog')).toHaveAttribute('open', '');
  await expect(page.locator('#edit-dialog-title')).toContainText('Add entry');
  await expect(page.locator('#edit-project option')).toHaveCount(1);
  await expect(page.locator('#edit-activity option')).toHaveCount(1);
  await expect(page.locator('#edit-skill option')).toHaveCount(1);
});

test('Create entry', async ({ page }, testInfo) => {
  const description = `playwright-created-entry-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  const startHour = 13 + testInfo.retry;
  const endHour = startHour + 1;

  await page.goto('/day/2025-01-02');

  await page.getByRole('button', { name: 'Add new worklog entry' }).click();
  await page.fill('#edit-start', `${String(startHour).padStart(2, '0')}:00`);
  await page.fill('#edit-end', `${String(endHour).padStart(2, '0')}:00`);
  await page.fill('#edit-billable-hours', '1');
  await page.fill('#edit-description', description);
  await page.getByRole('button', { name: 'Save' }).click();

  await expect(page.locator('.toast')).toContainText('Entry created.');
  await expect(page.locator('#day-entries tr').filter({ hasText: description })).toHaveCount(1);
});

test('Edit entry', async ({ page }) => {
  await page.goto('/day/2025-01-02');

  const row = page.locator('#day-entries tr').filter({ hasText: 'seed-entry' }).first();
  await row.getByRole('button', { name: 'Edit entry' }).click();

  await expect(page.locator('#edit-dialog')).toHaveAttribute('open', '');
  await expect(page.locator('#edit-start')).toHaveValue('09:00');
  await expect(page.locator('#edit-description')).toHaveValue(/seed-entry/);
});

test('Delete entry', async ({ page }, testInfo) => {
  const description = `delete-me-entry-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  const startHour = 15 + testInfo.retry;
  const endHour = startHour + 1;

  await page.goto('/day/2025-01-02');

  await page.getByRole('button', { name: 'Add new worklog entry' }).click();
  await page.fill('#edit-start', `${String(startHour).padStart(2, '0')}:00`);
  await page.fill('#edit-end', `${String(endHour).padStart(2, '0')}:00`);
  await page.fill('#edit-billable-hours', '1');
  await page.fill('#edit-description', description);
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.locator('#day-entries tr').filter({ hasText: description })).toHaveCount(1);

  const row = page.locator('#day-entries tr').filter({ hasText: description }).first();
  await row.getByRole('button', { name: 'Delete entry' }).click();
  await page.locator('#confirm-ok').click();

  await expect(page.locator('.toast')).toContainText('Entry deleted.');
  await expect(page.locator('#day-entries tr').filter({ hasText: description })).toHaveCount(0);
});

test('Edit dialog clears stale error', async ({ page }) => {
  await page.goto('/day/2025-01-02');

  await page.getByRole('button', { name: 'Add new worklog entry' }).click();
  await page.fill('#edit-start', '09:00');
  await page.fill('#edit-end', '10:00');
  await page.fill('#edit-billable-hours', '1');
  await page.fill('#edit-description', 'duplicate-check');
  await page.getByRole('button', { name: 'Save' }).click();

  await expect(page.locator('#edit-dialog-error')).toContainText('Entry already exists');
  await page.getByRole('button', { name: 'Cancel' }).click();
  await expect(page.locator('#edit-dialog')).not.toHaveAttribute('open', '');

  await page.getByRole('button', { name: 'Add new worklog entry' }).click();
  await expect(page.locator('#edit-dialog-error')).toHaveText('');
});
