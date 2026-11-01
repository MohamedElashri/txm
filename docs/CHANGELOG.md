# Changelog

Changes follow [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.1.0] - 2026-11-01

### Upgrade notes

- JSON schema 1 and existing private state remain compatible. Restart an older
  coordinator before using new operations; running sessions remain intact.
- macOS uses one universal archive for Intel and Apple Silicon.
- Provider hooks and MCP remain opt-in. Review existing settings and trust before
  enabling integrations.

### Added

- Claude/Codex lifecycle hooks with separate process, attention and turn state.
- Scoped stdio MCP with reviewed task-creation templates.
- Git task setup, review and explicitly authorized fast-forward merges.
- Attention-first dashboard and optional Linux/macOS child-process CPU/RSS metrics.

### Fixed

- Correct macOS CPU accounting and provider resume/session identity handling.

## [2.0.0] - 2026-10-31

### Upgrade notes

- Native sessions use private protocol v1. Keep the previous binary and its
  original environment to finish legacy sessions.
- Failures return nonzero exit codes; scripts should use the JSON envelope.
- Explicit backend selection is strict and rejects unsupported operations.

### Added

- Persistent native terminals, supervised jobs and a keyboard dashboard.
- Workspace profiles, isolated Git worktree tasks and SSH host access.
- Structured JSON, event streams, process receipts and agent notifications.

### Fixed

- Improve terminal restoration, process ownership, shutdown and backend isolation.

## [1.3.1] - 2026-09-17

- Update bundled man pages and completions with `txm update`.

## [1.3.0] - 2026-09-17

- Add verbosity, color and TOML workspace profiles.
- Group session, window and pane commands; retain root-command aliases.

## [1.2.2] - 2026-07-10

- Fix native terminal concurrency, scrollback restoration and slow-client blocking.
- Restrict native sockets and session logs to their owner.

## [1.2.1] - 2026-07-09

- Add Ghostty terminal emulation and static Linux musl archives.

## [1.2.0] - 2026-07-09

- Add native sessions, detach/reattach, read-only views, logging and session picker.
- Add terminal input, custom startup commands, prefixes and SSH config generation.
- Group window/pane commands; remove backend-specific window layout commands.

## [1.1.1] - 2026-05-09

- Add stable-release updates with atomic binary replacement.

## [1.1.0] - 2026-05-09

- Add backend-aware session completion and suppress unrelated file suggestions.

## [1.0.7] - 2026-05-09

- Add shell-aware installation and completion setup.
- Fix running-binary replacement, Zsh completion conflicts and Bash path quoting.

## [1.0.0] - 2026-05-09

- Add the Cobra CLI, strict name validation and backend interfaces.
- Add shell completions, install/uninstall, release archives and embedded man pages.
- Consolidate build and test commands; reorganize documentation.
