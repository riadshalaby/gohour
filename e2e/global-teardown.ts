import { rm } from 'node:fs/promises';

export default async function globalTeardown(): Promise<void> {
  const runtimeDir = process.env.GOHOUR_E2E_RUNTIME_DIR;
  if (!runtimeDir) {
    return;
  }
  await rm(runtimeDir, { recursive: true, force: true });
}
