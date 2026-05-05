const seedCSV = [
  'description,startdatetime,enddatetime,project,activity,skill',
  'seed-entry,2025-01-02 09:00,2025-01-02 10:00,P,A,S',
  '',
].join('\n');

export default async function globalSetup(): Promise<void> {
  const runtimeDir = process.env.GOHOUR_E2E_RUNTIME_DIR;
  const dbPath = process.env.GOHOUR_E2E_DB_PATH;
  const baseURL = process.env.GOHOUR_BASE_URL || 'http://localhost:9876';
  if (!runtimeDir || !dbPath) {
    throw new Error('GOHOUR_E2E_RUNTIME_DIR and GOHOUR_E2E_DB_PATH must be set');
  }

  const form = new FormData();
  form.append('mapper', 'generic');
  form.append('file', new Blob([seedCSV], { type: 'text/csv' }), 'seed-day.csv');

  const response = await fetch(new URL('/api/import', baseURL), {
    method: 'POST',
    body: form,
  });
  if (!response.ok) {
    throw new Error(`seed import failed: ${response.status} ${await response.text()}`);
  }
}
