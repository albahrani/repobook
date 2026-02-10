# Agent Guide (repobook)
Local web app that turns a folder (usually a git repo) into a fast Markdown "book".
Backend: Go. Frontend: vanilla HTML/CSS/JS embedded into the Go binary.

No Cursor rules found in `.cursor/rules/` or `.cursorrules`. No Copilot rules found in `.github/copilot-instructions.md`.

## Key Layout
- Entry: `cmd/repobook/main.go`
- HTTP server + API: `internal/server/server.go`
- Markdown render + sanitize: `internal/render/*`
- Tree scan: `internal/scan/tree.go`
- Search (optional rg): `internal/search/rg.go`
- Live reload: `internal/watch/*`
- Embedded assets: `internal/web/assets.go`, `internal/web/static/*`
- UI tests: `ui-tests/*`, `playwright.config.ts`

## Build / Lint / Test
### Go

Linux/macOS (bash):

- Build: `go build ./cmd/repobook`
- Run: `go run ./cmd/repobook --no-open /path/to/repo`
- Run (fixed addr): `go run ./cmd/repobook --host 127.0.0.1 --port 32123 --no-open testdata/repo`
- Format (all Go files): `gofmt -w $(git ls-files '*.go')`
- Format check (CI): `test -z "$(gofmt -l $(git ls-files '*.go'))"`
- Vet (CI): `go vet ./...`
- Tests: `go test ./...`
- Single package: `go test ./internal/scan`
- Single test: `go test ./internal/scan -run '^TestBuildTree$'`
- Subtest: `go test ./internal/scan -run '^TestBuildTree$/case$'`
- No cache: `go test ./internal/scan -run '^TestBuildTree$' -count=1`

Windows (PowerShell):

Note: Some docs/commands in this repo use bash-isms (`$(...)`, `test -z`, etc.). On Windows, prefer the PowerShell equivalents below (or run the bash versions in Git Bash / WSL).

- Build: `go build ./cmd/repobook`
- Run: `go run ./cmd/repobook --no-open C:\path\to\repo`
- Run (fixed addr): `go run ./cmd/repobook --host 127.0.0.1 --port 32123 --no-open testdata\repo`
- Format (all Go files):

```powershell
gofmt -w (git ls-files '*.go')
```

- Format check (CI-style):

```powershell
$files = git ls-files '*.go'
$bad = gofmt -l $files
if ($bad) { $bad; exit 1 }
```

- Vet (CI): `go vet ./...`
- Tests: `go test ./...`
- Single package: `go test ./internal/scan`
- Single test: `go test ./internal/scan -run '^TestBuildTree$'`
- Subtest: `go test ./internal/scan -run '^TestBuildTree$/case$'`
- No cache: `go test ./internal/scan -run '^TestBuildTree$' -count=1`

### UI (Playwright)
Config starts server via `playwright.config.ts`:
`go run ./cmd/repobook --no-open --host 127.0.0.1 --port 32123 testdata/repo`.

Windows (PowerShell path form):
`go run ./cmd/repobook --no-open --host 127.0.0.1 --port 32123 testdata\repo`.

- Setup (Linux CI): `npm ci`; `npx playwright install --with-deps`
- Setup (Windows/macOS): `npm ci`; `npx playwright install`
- All: `npm run test:ui`
- One file: `npx playwright test ui-tests/repobook.spec.ts`
- One test: `npx playwright test -g "search finds results"`
- Debug/headed: `npm run test:ui:debug -- ui-tests/repobook.spec.ts`; `npm run test:ui:headed -- ui-tests/repobook.spec.ts`

Search uses `rg`. If missing, `/api/search` returns `501` and UI tests may skip.

## Code Style
### Defaults
- Small, focused changes; avoid new deps (see `CONTRIBUTING.md`).
- Treat repo content as untrusted input (paths, markdown, raw assets).
- If vendoring third-party code/assets, update `THIRD_PARTY_NOTICES.md`.

### Go formatting / imports
- Run `gofmt` on touched files.
- Imports: stdlib; blank; third-party; blank; local (`repobook/internal/...`) (gofmt order).
- Keep imports minimal; no unused.

### Go naming / types
- Packages: short, lower-case.
- Exported: `PascalCase`; unexported: `camelCase`.
- JSON structs: explicit tags; use `omitempty` when needed.

### Errors / control flow
- Wrap with context: `fmt.Errorf("x: %w", err)`.
- Preserve identity when useful; use `errors.Is/As`.
- Sentinel errors: `var ErrX = errors.New("...")` (see `ErrRipgrepNotFound`).
- Put timeouts around external work (see `internal/search/rg.go`, shutdown in `cmd/repobook/main.go`).

### Paths / security invariants (do not weaken)
- OS paths: `filepath.*`. URL/repo-relative: forward slashes + `path.*`.
- Convert with `filepath.ToSlash` / `filepath.FromSlash`.
- Never allow `..` to escape repo root (see `internal/util/path.go:ResolveRepoPath`).
- Keep raw repo assets separate-origin: `/repo/...` redirects to asset server (`internal/server/server.go`).
- Markdown output must be sanitized (goldmark renders unsafe HTML; bluemonday sanitizes in `internal/render/render.go`).

### HTTP handlers
- Check method first; return `405` via `http.Error`.
- Status codes: `404` missing/ignored, `501` optional feature missing (rg), `500` internal.
- Avoid leaking host filesystem paths in errors.

### Frontend (embedded)
- No bundler: files under `internal/web/static/*` embedded via `//go:embed`.
- Vanilla JS; prefer `const`, `async/await`, small helpers (`fetchJSON`).
- Escape user-controlled strings when building HTML (see `internal/web/static/app.js:esc`).

### Tests
- Go tests: `*_test.go` near code.
- UI: prefer Playwright assertions; avoid sleeps.

## Repo Hygiene
- Do not commit generated/local artifacts (`node_modules/`, Playwright reports, OS junk).
- Vendor changes (`internal/web/static/vendor/*`) require license review + notices update.
