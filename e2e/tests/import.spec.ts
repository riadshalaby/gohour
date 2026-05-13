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

  await page.getByRole('button', { name: 'Import file' }).first().click();

  await expect(page.locator('#month-import-dialog')).toHaveAttribute('open', '');
  await expect(page.locator('#month-import-billable option')).toHaveCount(2);
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

test('Import dialog prefills fields and shows matched rule on file pick', async ({ page }, testInfo) => {
  const csvPath = testInfo.outputPath('import-smoke-prefill.csv');
  const unmatchedPath = testInfo.outputPath('import-smoke-prefill.txt');
  await writeFile(
    csvPath,
    [
      'description,startdatetime,enddatetime,project,activity,skill',
      'prefill check,2025-01-15 09:00,2025-01-15 10:00,P,A,S',
      '',
    ].join('\n'),
    'utf8',
  );
  await writeFile(
    unmatchedPath,
    [
      'description,startdatetime,enddatetime,project,activity,skill',
      'unmatched check,2025-01-15 10:00,2025-01-15 11:00,P,A,S',
      '',
    ].join('\n'),
    'utf8',
  );

  await page.goto('/month/2025-01');
  await page.getByRole('button', { name: 'Import file' }).first().click();

  await expect(page.locator('#month-import-dialog')).toHaveAttribute('open', '');
  await page.setInputFiles('#month-import-file', csvPath);
  await expect(page.locator('#month-import-rule-match')).toContainText('Matched rule: generic-local');
  await expect(page.locator('#month-import-mapper')).toHaveValue('generic');
  await expect(page.locator('#month-import-project')).toHaveValue('P');
  await expect(page.locator('#month-import-activity')).toHaveValue('A');
  await expect(page.locator('#month-import-skill')).toHaveValue('S');
  await expect(page.locator('#month-import-billable')).toHaveValue('non-billable');

  await page.setInputFiles('#month-import-file', unmatchedPath);
  await expect(page.locator('#month-import-rule-match')).toContainText('No rule matched — using defaults');
  await expect(page.locator('#month-import-mapper')).toHaveValue('epm');
  await expect(page.locator('#month-import-billable')).toHaveValue('billable');
});

test('Update matched rule affordance hidden until override', async ({ page }, testInfo) => {
  const csvPath = testInfo.outputPath('rule-update.csv');
  await writeFile(
    csvPath,
    [
      'description,startdatetime,enddatetime,project,activity,skill',
      'override check,2025-01-16 09:00,2025-01-16 10:00,P,A,S',
      '',
    ].join('\n'),
    'utf8',
  );

  await page.goto('/month/2025-01');
  await page.getByRole('button', { name: 'Import file' }).first().click();
  await expect(page.locator('#month-import-dialog')).toHaveAttribute('open', '');
  await page.setInputFiles('#month-import-file', csvPath);
  await expect(page.locator('#month-import-rule-match')).toContainText('Matched rule: generic-local');
  await page.getByRole('button', { name: 'Upload' }).click();

  await expect(page.locator('#preview-import-btn')).toBeVisible();
  await expect(page.locator('#preview-rule-match')).toContainText('Matched rule: generic-local');
  await expect(page.locator('#preview-update-rule-wrapper')).toBeHidden();
  await expect(page.locator('#preview-update-rule')).toBeDisabled();

  await page.locator('#preview-billable').selectOption('billable');
  await expect(page.locator('#preview-update-rule-wrapper')).toBeVisible();
  await expect(page.locator('#preview-update-rule')).toBeChecked();
  await expect(page.locator('#preview-update-rule')).toBeEnabled();

  await page.locator('#preview-update-rule').uncheck();
  await page.locator('#preview-mapper').selectOption('epm');
  await expect(page.locator('#preview-update-rule-wrapper')).toBeVisible();
  await expect(page.locator('#preview-update-rule')).not.toBeChecked();
});
