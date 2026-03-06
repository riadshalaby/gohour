import { expect, test } from '@playwright/test';

test('Submit day dry-run', async ({ page }) => {
  await page.goto('/day/2025-01-02');

  await page.getByRole('button', { name: 'Submit day' }).click();
  await page.locator('#submit-dry-run').check();
  await page.locator('#submit-dialog-run').click();

  await expect(page.locator('#submit-dialog-result')).toContainText('Preview only');
  await expect(page.locator('#submit-dialog-result')).toContainText('Would add');
});
