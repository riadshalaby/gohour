
import { defineConfig } from '@playwright/test';
import os from 'node:os';
import path from 'node:path';

const runtimeDir = process.env.GOHOUR_E2E_RUNTIME_DIR || path.join(os.tmpdir(), 'gohour-e2e-runtime');
const dbPath = process.env.GOHOUR_E2E_DB_PATH || path.join(runtimeDir, 'test.db');
const baseURL = process.env.GOHOUR_BASE_URL || 'http://localhost:9876';
const listenURL = new URL(baseURL);
const port = Number(listenURL.port || (listenURL.protocol === 'https:' ? '443' : '80'));
const runServerScript = path.resolve(__dirname, 'run-server.sh');

process.env.GOHOUR_E2E_RUNTIME_DIR = runtimeDir;
process.env.GOHOUR_E2E_DB_PATH = dbPath;
process.env.GOHOUR_E2E_PORT = String(port);

export default defineConfig({
  testDir: './tests',
  timeout: 15_000,
  retries: 1,
  workers: 1,
  globalSetup: require.resolve('./global-setup'),
  globalTeardown: require.resolve('./global-teardown'),
  use: {
    baseURL,
    headless: true,
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium' },
    },
  ],
  webServer: {
    command: `sh "${runServerScript}"`,
    reuseExistingServer: false,
    timeout: 15_000,
    url: baseURL,
    cwd: path.resolve(__dirname, '..'),
  },
});
