import { defineConfig } from '@playwright/test'

// Centralized configuration for host, port, and test repository
const HOST = '127.0.0.1'
const PORT = 32123
const TEST_REPO = 'testdata/repo'
const BASE_URL = `http://${HOST}:${PORT}`

// Determine server command based on environment
// In CI, use the prebuilt binary downloaded from artifacts
// Locally, use `go run` for convenience
const isCI = !!process.env.CI
const serverCommand = isCI
  ? `./repobook --no-open --host ${HOST} --port ${PORT} ${TEST_REPO}`
  : `go run ./cmd/repobook --no-open --host ${HOST} --port ${PORT} ${TEST_REPO}`

export default defineConfig({
  testDir: 'ui-tests',
  timeout: 30_000,
  use: {
    baseURL: BASE_URL,
  },
  webServer: {
    command: serverCommand,
    url: BASE_URL,
    reuseExistingServer: !isCI,
    timeout: 30_000,
  },
})
