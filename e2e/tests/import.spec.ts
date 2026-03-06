import { expect, test } from '@playwright/test';
import { writeFile } from 'node:fs/promises';

test('Import file flow', async ({ page }, testInfo) => {
  const description = `browser-import-smoke-${testInfo.retry}`;
  const csvPath = testInfo.outputPath('import-smoke.csv');
  await writeFile(
    csvPath,
    [
      'description,startdatetime,enddatetime,project,activity,skill',
      `${description},2025-01-15 13:00,2025-01-15 14:00,P,A,S`,
      '',
    ].join('\n'),
    'utf8',
  );

  await page.goto('/month/2025-01');

  await page.getByRole('button', { name: /actions/i }).click();
  await page.getByRole('menuitem', { name: 'Import file' }).click();

  await expect(page.locator('#month-import-dialog')).toHaveAttribute('open', '');
  await expect(page.locator('#month-import-project option')).toHaveCount(1);

  await page.selectOption('#month-import-mapper', 'generic');
  await page.setInputFiles('#month-import-file', csvPath);
  await page.getByRole('button', { name: 'Upload' }).click();

  await expect(page.locator('#preview-import-btn')).toBeVisible();
  await expect(page.locator('#preview-summary')).toContainText('1 entries');
  await page.locator('#preview-import-btn').click();

  await expect(page.locator('#preview-import-btn')).not.toBeVisible();
  await page.waitForLoadState('networkidle');
  await page.goto('/day/2025-01-15');
  await expect(page.locator('#day-entries')).toContainText(description);
});
