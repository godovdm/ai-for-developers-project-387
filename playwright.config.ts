import { defineConfig, devices } from '@playwright/test';

const ownerPassword = process.env.E2E_OWNER_PASSWORD ?? 'e2e-owner-password';

export default defineConfig({
  testDir: './e2e',
  use: { baseURL: 'http://127.0.0.1:29386' },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
    { name: 'webkit', use: { ...devices['Desktop Safari'] } },
  ],
  webServer: {
    command: 'cd backend && PORT=29386 go run ./cmd/server',
    url: 'http://127.0.0.1:29386/api/health',
    env: { OWNER_PASSWORD: ownerPassword },
    reuseExistingServer: false,
    timeout: 60_000,
  },
});
