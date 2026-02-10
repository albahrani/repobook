# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog (https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Fixed

- **Windows deadlock**: Fixed application crash ("fatal error: all goroutines are asleep - deadlock!") when running on Windows with large directory trees. The file watcher event loop now starts before adding directory watches to prevent fsnotify from blocking on synchronous event sends.
- **Paths with spaces**: Command-line argument parsing now properly handles paths containing spaces, even when not quoted (e.g., `repobook C:\path with spaces`).

## [0.1.0] - 2026-01-31

Initial open-source release.

Added:

- Local web UI for browsing a repository as a Markdown "book" (nav tree, breadcrumbs, TOC)
- GitHub-flavored-ish Markdown rendering with syntax highlighting
- Optional full-text search via ripgrep (`rg`)
- Mermaid diagrams via `mermaid` fenced code blocks
