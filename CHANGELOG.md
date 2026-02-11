# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog (https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Fixed

- **Windows deadlock**: Fixed application crash ("fatal error: all goroutines are asleep - deadlock!") when running on Windows with large directory trees. The file watcher event loop now starts before adding directory watches.
- **Paths with spaces**: Command-line argument parsing now properly handles paths containing spaces, even when not quoted (e.g., `repobook C:\path with spaces`).

### Added

- Server-side mermaid handling: fenced code blocks with language `mermaid` are converted to `<div class="mermaid">...</div>` during render (`internal/render/diagrams.go`).
- Client-side rendering: lazy load + render logic for Mermaid in `internal/web/static/app.js` (`loadScriptOnce`, `renderMermaidElements`) so diagrams render after the document is inserted.
- Vendored Mermaid runtime: `internal/web/static/vendor/mermaid.min.js` (vendored copy available and served at `/app/vendor/mermaid.min.js`).
- Test coverage: new Playwright UI test `ui-tests/mermaid.spec.ts` verifies the mermaid block renders to an SVG and there are no console/network errors; added to the existing UI suite.
- Test data: sample mermaid diagram added to `testdata/repo/docs/guide.md` to exercise rendering end-to-end.

## [0.1.1] - 2026-02-10

Small release with dependency bumps, CI improvements, and several fixes.

### Fixed

- **Windows deadlock & path handling**: backported fixes to avoid a deadlock on Windows and improve handling of paths with spaces (see PR #5).

### Changed

- CI/workflows: updated Playwright config to support prebuilt binaries and improved release workflow; removed bundled binary from the repository and updated `.gitignore` / docs.
- Dependencies: bumped `golang.org/x/net`, `github.com/alecthomas/chroma`, and the Playwright test dependency.

## [0.1.0] - 2026-01-31

Initial open-source release.

Added:

- Local web UI for browsing a repository as a Markdown "book" (nav tree, breadcrumbs, TOC)
- GitHub-flavored-ish Markdown rendering with syntax highlighting
- Optional full-text search via ripgrep (`rg`)
- Mermaid diagrams via `mermaid` fenced code blocks
