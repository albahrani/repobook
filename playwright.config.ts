import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: 'ui-tests',
  timeout: 30_000,
  use: {
    baseURL: 'http://127.0.0.1:32123',
  },
  webServer: {
    command: 'go run ./cmd/repobook --no-open --host 127.0.0.1 --port 32123 testdata/repo',
    url: 'http://127.0.0.1:32123/',
    reuseExistingServer: !process.env.CI,
    timeout: 30_000,
  },
})
