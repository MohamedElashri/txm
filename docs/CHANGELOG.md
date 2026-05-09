# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-05-09

This release marks a major architectural overhaul of the `txm` tool to improve security, extensibility, and user experience. 

### Added
- **Cobra CLI Integration**: Completely replaced the manual argument parsing with the `spf13/cobra` framework.
- **Native Shell Completions**: Added the `txm completion` command to dynamically generate autocompletion scripts for `bash`, `zsh`, `fish`, and `powershell`.
- **Strict Input Validation**: Implemented alphanumeric (`^[a-zA-Z0-9_-]+$`) validation on all user-supplied session and window names to prevent injections.
- **Generic Backend Interface**: Added the `TerminalMultiplexer` interface in `pkg/backend/` allowing for clean encapsulation and extensibility of multiplexer logic.
- **Self-Contained Uninstaller**: Added native intelligence to `txm uninstall` to locate and purge binaries and man pages from the system cleanly.
- **GoReleaser Integration**: Introduced `.goreleaser.yaml` for robust cross-compilation and automated GitHub Releases.
- **Automated CI/CD**: Consolidated all fragmented GitHub Actions into standard `ci.yml` and `release.yml` workflows.
- **Embedded Man Pages**: Compiled the manual (`txm.1`) directly into the Go binary using `//go:embed` for seamless installations.
- **Unit Testing**: Added foundational test coverage for CLI validation logic and config parsing.

### Changed
- Refactored all backend interactions (`tmux`, `zellij`, `screen`) from a massive central file into isolated, structural structs implementing the core backend interface.
- Overhauled the `Makefile` to utilize standard build commands (`build`, `install`, `test`, `lint`).
- Consolidated `docs.md` into `docs/user-guide.md` and moved `txm.1` into the `docs/` folder for better directory organization.

### Removed
- Removed the old, buggy bash/zsh completion scripts in `utils/`.
- Removed `utils.go` containing the legacy auto-updater code.
- Removed custom `txm_test.go` and massive `main.go` switches in favor of standard Cobra routing and smaller unit tests.
