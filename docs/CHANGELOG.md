# Changelog

Changes follow [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Documentation

- Consolidate installation, interactive, and automation workflows in the user
  guide, development and release instructions in one maintainer guide, and
  native wire details in the contracts reference. Routine documentation uses
  repository version metadata; release-specific evidence stays in the changelog.

### Fixed

- Publish native fixture identity receipts atomically and tolerate temporary
  connection-pool refusals only in the bounded recovery probe. Rejected test
  handshakes close their sockets.

### Release engineering

- Skip expensive automatic Linux/macOS build, runtime, and lint jobs when only
  Markdown or the man page changes. Keep lightweight scope/metadata checks and
  full manual CI for exact-commit release validation.
- Reserve full package validation for tags and explicit dry runs. Reuse successful
  CI of the exact release commit, build Linux/Windows archives on Linux alongside
  a separate Darwin build, and cache verified native dependencies before tests.
- Retain independently retryable build partitions and the complete package bundle,
  bound job runtime, and distinguish package reports by run attempt. All eight
  target runtime/CLI gates remain required before draft creation.
- Run Linux foreign-UID checks in a pinned minimal Python image without building
  a full SSH/backend test image for each archive.

## [2.0.0] - 2026-10-31

txm 2.0 brings persistent terminal workspaces, supervised commands, a dashboard,
Git worktree tasks, and SSH hosts into one CLI. See the [compatibility matrix](user-guide.md#platforms-and-backends)
for backend scope and platform validation.

### Upgrade notes

- Native sessions use private framed protocol v1. Keep the old binary and its
  original user/TMPDIR/config to finish existing legacy sessions. New clients
  leave their sockets and processes untouched; native nuke refuses while
  incompatible legacy candidates are present.
- Command failures now return nonzero and write human diagnostics to stderr.
  Scripts should use the versioned JSON envelopes and documented exit codes.
- Explicit `--backend` is strict. Exact target operations reject ambiguity and
  unsupported pane/layout behavior before mutation. Invalid profiles fail preflight.
- Native provides one terminal, bounded visible snapshots and literal input to
  terminal 0/0. Input acceptance is not command completion. Logging is opt-in.
- Ordinary sessions and the picker remain independent of the optional coordinator.
  Provider hooks require explicit opt-in; existing provider trust is unchanged.

### Added

- Typed session inspection, capability data, versioned JSON and bounded plain/raw
  output, including tmux history and exact session/window/pane identities.
- Declarative profile preflight/dry-run, cwd/layout handling and cleanup limited
  to resources whose creation and ownership are proven.
- Supervised native/tmux agents with immutable exit receipts, authenticated
  signals, bounded JSONL events, deterministic waits and explicit notifications.
- A separate keyboard dashboard with search/filtering, freshness/capability
  details, bounded previews, read-only attach and explicit agent subscriptions.
- Opt-in owned Git worktrees and tasks with conservative recovery and cleanup;
  explicit task/agent metadata pruning retains receipts and Git branches.
- Allowlisted, host-pinned OpenSSH discovery and exact native/tmux attach with
  protocol checks, partial results, cancellation and ControlMaster reuse.

### Fixed

- Stable updates compare semantic versions and retain a newer installed binary
  when GitHub still reports an older stable release; invalid/prerelease metadata
  cannot trigger an update download.

- Development/prerelease builds refuse automatic stable updates, preventing a
  development build from being replaced by an older stable release.
- Native socket permissions/peer checks, duplicate startup, bounded IPC queues,
  slow peers, process-group shutdown, detach and TTY/file-flag restoration.
- Exact tmux input/profile targeting and literal keys, synchronized-pane refusal,
  private host identity publication and corrupt-state handling.
- Darwin concurrent locks, exclusive publication, zombie-only group shutdown,
  dashboard feedback guards and Apple Screen listing/version exit statuses.
- Zellij background readiness without repeated creation or name-based rollback.
- Missing remote completeness fields and uncertain task recovery now fail closed.

### Platform support

- Linux amd64/ARM64 GNU/static-musl and macOS ARM64/Intel pass archive installation
  and runtime checks on matching CI runners, including actual foreign-UID
  rejection. Windows amd64/ARM64 pass CLI smoke checks on matching runners;
  external backends and Windows installers remain unverified. The tag workflow
  repeats validation of the final release archives before creating a draft.
- Screen/Zellij retain degraded basic lifecycle/attach support for the recorded
  versions; exact panes/layouts/capture/read-only remain unsupported.
- Windows native persistence/private Unix state/dashboard are unsupported.
- Generic supervision needs no AI provider. Claude has isolated initialization
  coverage; full provider lifecycle needs reviewed setup. Codex hooks require
  manual trust review; unsupported provider versions are refused.

### Documentation and release engineering

- A concise README, shared documentation index, getting-started and automation
  guides, and contributor instructions organized around user workflows.
- Stable 2.0.0 identity and release notes, checksum/archive validation, and native
  package checks before draft creation. Publishing remains a separate step.

### Validation evidence

These are pre-publication snapshot results for this version, not proof of the
final tagged release bytes. The tag workflow must validate its own archives
before draft creation and publication review.

| Platform | Recorded runtime coverage |
| --- | --- |
| Linux amd64 | GNU/static-musl archives pass private installation, native/tmux runtime suites, profiles, supervision/events, dashboard, tasks/pruning, and actual foreign-UID rejection. Normal/race/fuzz CI and additional local SSH/container checks also pass. |
| macOS ARM64 | Archive installation and native/tmux runtime suites pass, including profiles, supervision/events, dashboard, tasks/pruning, real Screen/Zellij, loopback SSH, and actual foreign-UID rejection. Normal and race CI also pass. |
| macOS Intel | The same archive installation, runtime, backend, SSH, and foreign-UID gates as macOS ARM64 pass on an Intel runner; normal and race CI also pass. |
| Linux ARM64 | GNU/static-musl archives pass private installation, native/tmux runtime suites, profiles, supervision/events, dashboard, tasks/pruning, and actual foreign-UID rejection on ARM64 runners. |
| Windows amd64/ARM64 | Both archives pass checksum/header validation and CLI version/help/completion/unsupported-refusal smoke checks on matching runners. External backends and Windows installers are unverified; native persistence, private Unix state, and dashboard are unsupported. |

Basic Zellij 0.45.1 and GNU Screen lifecycle/attach have Linux/macOS runtime
coverage; macOS Screen is 4.00.03. Their adapters remain degraded: exact panes,
layouts, output capture, and read-only attachment are unsupported.

The matrix records the successful [PR CI](https://github.com/MohamedElashri/txm/actions/runs/41153986716)
and [eight-target package validation](https://github.com/MohamedElashri/txm/actions/runs/41153986791)
at commit `0bb3ef3`, plus local coverage recorded here. These PR archives
are snapshots stamped 2.0.0. Official release assets are rebuilt and must pass
the exact-tag workflow before publication. Archive names alone do not establish
runtime support.

| Adapter | Recorded validation |
| --- | --- |
| Generic | Authenticated worker signals; no AI provider required |
| Claude | Exact 2.1.308 adapter with offline SessionStart coverage; full conversations remain unverified |
| Codex | 0.165.0 schema and manual configuration only; runtime/trusted lifecycle remains unverified. Installed 0.166.0 is refused. |
| OpenCode | Installed v2.2.2 is unsupported; no v1 examples are applied to v2 |

Local Linux amd64 GNU/static-musl checks passed 40 runtime/install/performance
checks and 36 container groups across normal/race and actual archived binaries;
25 harness tests, 13 Go/build checks, and four fuzz targets also passed. Reports
remain outside the source tree. Local macOS ARM64 coverage included OpenSSH
10.5p1. Windows external backends/installers and full trusted provider
conversations remained outside the verified scope.

## [1.3.1] - 2026-09-17

### Added
- **Auto-Update Enhancements**: The `txm update` command now automatically updates the bundled man page and shell completion scripts, instead of just replacing the binary.

## [1.3.0] - 2026-09-17

### Added 
- **Verbosity Levels**: Added verbosity levels to the CLI, allowing users to see more detailed information about what the CLI is doing.
- **Color Support**: Added color support to the CLI, allowing users to see colors in the terminal.
- **Profile Management**: Added a new command `txm profile` to create and manage terminal session profiles declaratively from TOML configuration files.

### Changed
- **CLI Reorganization**: Refactored the command-line interface to a concrete noun-verb structure (`txm session <cmd>`, `txm window <cmd>`, `txm pane <cmd>`).
- **Code Refactoring**: Centralized argument parsing and validation into helper functions, significantly reducing logic duplication across commands.
- **man page**: Update the man page to accompany changes for v1.2.5.
- **Root Commands**: Old root-level commands (`txm detach`, `txm delete`, `txm rename-session`, `txm nuke`, `txm exec`) have been retained as aliases to their new equivalents to ensure backward compatibility and a fast UX.

## [1.2.2] - 2026-07-10

### Fixed
- **libghostty Thread Safety**: Fixed a critical race condition in the native backend where concurrent connections could cause segmentation faults when accessing the VT terminal state.
- **Scrollback Layout Restoration**: Fixed a bug where attaching to a native session from a window with a different size would restore scrollback text using the default 80x24 layout, causing text garbling. PTY resize events are now properly propagated to the internal VT core.
- **Server Broadcast Freezing**: Fixed a severe head-of-line blocking vulnerability where a single frozen or maliciously paused client could permanently hang the native background daemon and all other attached users.
- **Unix Socket Hijacking**: Fixed a local privilege escalation (LPE) vulnerability where the native backend's IPC Unix socket in `/tmp` inherited default OS umask permissions, potentially allowing other local users to connect, sniff terminal output, and inject arbitrary commands into the session. Sockets are now strictly locked down to `0600` permissions.
- **Log File Information Disclosure**: Fixed a vulnerability where session log files created with `--log` were initialized with world-readable `0644` permissions, exposing raw terminal output (including passwords and sensitive data) to other local users. New log files are now securely created with `0600` permissions.

### Changed
- **man page**: Update the man page to accompany changes since 1.2.0 releases.

## [1.2.1] - 2026-07-09

### Added
- **libghostty Integration**: Supercharged the `native` backend by replacing the naive byte-ring buffer with Ghostty's highly-performant C/Zig terminal emulator core (`libghostty-vt`). This guarantees 100% accurate VT terminal state parsing, resizing, and rendering for native sessions.
- **Fully Static Musl Builds**: Added dedicated `linux-musl` static cross-compilation targets (`txm_Linux_x86_64-musl.zip`) to the automated GitHub release pipeline, providing drop-in compatibility for Alpine Linux and minimal Docker containers without requiring `glibc`.

## [1.2.0] - 2026-07-09

### Added
- **Native Backend**: Implemented a built-in lightweight native backend for persistent terminal sessions without relying on external dependencies like `tmux` or `zellij`.
- **Auto-Attach**: Automatically attaches to the only available session when running `txm attach` without arguments, or creates a default session if none exist.
- **Remote Execution**: Added `txm exec` to remotely execute commands inside background panes.
- **Native State & Scrollback Restoration**: The native backend now maintains a configurable scrollback ring buffer (default 64KB) to instantly restore terminal state and history upon reattachment.
- **Graceful Detach Keybinding**: Added `Ctrl+\` as a native detach sequence, allowing users to safely disconnect from a native session without terminating it.
- **Custom Startup Commands**: `txm create` and `txm attach` now accept custom commands to run upon session initialization instead of defaulting to the system shell.
- **Interactive Session Picker**: Running `txm` without arguments now brings up a native interactive fuzzy-finder to preview and select sessions.
- **Session Prefixing**: Added support for the `TXM_SESSION_PREFIX` environment variable to automatically namespace session names across all commands.
- **Read-Only Attach Mode**: Added `-r` / `--read-only` flag to `txm attach` to allow safe, interference-free session monitoring.
- **Session Output Logging**: Added `--log` flag to `txm create` to mirror PTY output to a persistent file, complete with automatic size-based log rotation.
- **Active Client Tracking**: `txm list` now natively pings background sessions and actively displays the number of attached clients.
- **SSH Workflow Helper**: Added the `txm generate-ssh-config` command to automatically generate zmx-style `ControlMaster` SSH configurations.

### Changed
- **CLI Reorganization**: Grouped window and pane commands into intuitive subcommands (`txm window <cmd>` and `txm pane <cmd>`).
- Renamed `send-keys` to `exec` to better reflect remote command execution capabilities.

### Removed
- **Layout Commands Deprecated**: Dropped support for backend-specific window layout commands (`move-window`, `swap-window`, `resize-pane`) to align with a more minimalist session persistence philosophy.

## [1.1.1] - 2026-05-09

### Added
- **Native Auto-Updater**: Fully overhauled `txm update` to natively fetch, download, and extract the latest release binaries directly from GitHub using the GoReleaser artifacts naming convention. Includes atomic binary replacement to bypass "text file busy" errors on Linux.

## [1.1.0] - 2026-05-09

### Added
- **Dynamic Session Autocomplete**: Shell autocomplete now intelligently suggests active session names for commands taking a session argument (e.g., `attach`, `delete`, `move-window`, `list-panes`, etc.). Works dynamically across all supported backends (`tmux`, `zellij`, and `screen`).
- **File Completion Suppression**: Improved autocomplete experience by suppressing irrelevant local file path completions where they are not applicable (like session names).

## [1.0.7] - 2026-05-09

### Added
- **Automated Shell Configuration**: `txm install` now automatically detects your shell (`bash`, `zsh`, or `fish`) and configures your RC files (`.bashrc` or `.zshrc`) for completions.
- **Completion --install Flag**: Added a dedicated `--install` flag to the `txm completion` subcommands for a one-step setup of shell completions.
- **Smart Shell Detection**: The installer and completion commands now use the `$SHELL` environment variable to provide shell-specific instructions.

### Fixed
- **Installation "Text File Busy"**: Resolved a critical bug where `txm install` would fail if the binary was already running from the destination path.
- **Zsh Completion Conflicts**: Improved Zsh installation logic to prioritize native Zsh completions and prevent conflicts with Bash completion helpers.
- **Linting Compliance**: Fixed all remaining `errcheck` and `staticcheck` issues to ensure a clean CI/CD pipeline.
- **Bash Path Quoting**: Fixed an issue where the automated Bash completion source line would fail on paths containing spaces.

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
