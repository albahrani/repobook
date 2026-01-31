# Contributing

Thanks for considering contributing.

## Development Setup

Requirements:

- Go (see `go.mod`)
- Node.js (for Playwright UI tests)

### Run tests

Go unit tests:

```bash
go test ./...
```

UI tests:

```bash
npm ci
npx playwright install --with-deps
npm run test:ui
```

Notes:

- Search features use ripgrep (`rg`). If `rg` is missing, some UI tests will skip.

## Code Style

- Keep changes small and focused.
- Prefer straightforward, dependency-light solutions.
- If you vendor new third-party code, add an entry to `THIRD_PARTY_NOTICES.md`.
