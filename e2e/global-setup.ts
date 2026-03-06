export default async function globalSetup(): Promise<void> {
  const runtimeDir = process.env.GOHOUR_E2E_RUNTIME_DIR;
  const dbPath = process.env.GOHOUR_E2E_DB_PATH;
  if (!runtimeDir || !dbPath) {
    throw new Error('GOHOUR_E2E_RUNTIME_DIR and GOHOUR_E2E_DB_PATH must be set');
  }
}
