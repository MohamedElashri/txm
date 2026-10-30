# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Validation and hardening

- Added disposable Linux container gates for actual foreign-UID IPC rejection,
  real loopback SSH/PTY/ControlMaster and Screen 4.09.01/Zellij 0.45.1 basic
  lifecycle/attach, preserving their degraded capability limits.
- Remote operation exit 1 retains its operation-failure classification; OpenSSH
  authentication/network failures remain transport failures.
- Added explicit full-ID-confirmed `task prune` and `agent prune`, dry-run,
  supervised exit/cleaned-worktree checks, retained receipts/branches, bounded
  retired-ID replay prevention and event subscriber resync.

- Final Linux validation runner records exact source/artifact hashes, command
  results and fixture logs, and rejects uninstrumented race binaries or reused
  output directories. Added bounded credential-pattern checks with redacted findings.
- Remote JSON now rejects missing/null completeness flags and missing data fields;
  malformed envelopes cannot appear as complete results. Schema remains version 1.
- Task cleanup no longer treats a missing managed registration as proof of an
  orphan. Tasks pin supervisor instances and require exact exit receipts after
  registry loss; unknown/lost replies fail closed even with force. Name collisions
  are preflighted before creating worktrees.
- Build-helper tests use a private HOME and constructed environment; inherited
  Git routing variables or credentials cannot redirect their temporary repositories.
- Archive smoke verifies task/remote completions as well as existing workflows.
  Unrun platform/provider/actual SSH gates remain explicit release blockers.

### Added
- Opt-in Git task start/list/inspect/stop/cleanup, private owned worktrees,
  redacted dry-run, fresh branches, exact managed IDs and receipt-gated cleanup.
  Dirty/untracked/ignored removal requires force plus full-ID confirmation;
  branches/commits remain, and interrupted launches are never replayed.
- Explicit SSH alias allowlist/pinning, protocol probe, bounded partial remote
  listing and exact native/tmux attachment through encoded requests.
  Reuses OpenSSH authentication/configuration; ordinary workflows need no Git/SSH.
  Fake SSH coverage plus isolated real OpenSSH loopback/PTY checks; no external-host or other-platform claim.
- Separate keyboard-first `dashboard` with typed session/agent observations,
  search/backend/state filters, freshness/source/capability details, independent
  cancellable refresh, safe bounded preview and native/tmux attach/read-only actions.
- Explicit dashboard `--agents` subscriptions; ordinary dashboard and bare picker
  remain independent of the coordinator. Registrations are never joined by name.
- Original TTY/file-flag restoration across dashboard exit, cancellation and
  external attach failures, plus a synthetic recording and workflow examples.
- Authenticated per-instance/generation attention and turn signals, private hook
  credentials, replay/freshness checks and explicit provider associations.
- Bounded `events --jsonl` with atomic initial snapshots, overflow/reconnect
  resync and epoch/sequence metadata; deterministic `agent wait` with finite
  deadlines and optional child failure propagation.
- Explicit `agent notify -- EXECUTABLE [ARGS...]` actions with safe stdin metadata,
  deadlines, throttling and no snapshot replay. Notifications default off.
- Generic managed hook context and isolated real Claude initialization-hook
  coverage. Codex hooks require manual trust review; OpenCode v2 is unsupported.
- Opt-in `agent start/register/list/status` and `stop-coordinator`, with private
  single-writer registry, exact backend bindings and restart reconciliation.
- Native/tmux structured supervised launches with one-use argv/cwd manifests,
  filtered child environment, foreground PTY process groups and immutable exit
  receipts. Missing proof remains unknown; registration is unsupervised.
- Strict profile preflight, redacted `--dry-run`, `--detach`, explicit
  `--attach-existing` and read-only existing-profile attachment.
- Exact tmux 3.4+ profile application using returned IDs, per-pane cwd, named
  layouts and ownership-checked rollback; native single-terminal cwd profiles.
- Strict `--backend`, explicit `--all-backends` read aggregation, qualified
  `backend/name` targets, `session inspect`, and `backend capabilities`.
- Versioned `--json` envelopes for session list/inspect/output and capabilities,
  classified failure exits, explicit partial results, and private local host IDs.
- Bounded `session output` with safe plain text, explicit raw snapshots and
  tmux history. Picker previews and hidden dump use the same capture service.

### Fixed
- Profile commands no longer depend on focus or guessed indexes, ignore cwd and
  layout, or reapply automatically to an existing session. Partial/uncertain
  creations are disclosed and left intact when cleanup ownership cannot be proven.
- Exact tmux input refuses pane synchronization; profiles disable it only on
  newly created panes. Profile shells override tmux default-command explicitly.
- Relative native TMPDIR is rejected before runtime creation.
- Release pkg-config paths are absolute; archives include man/completion files
  and use current GoReleaser keys. Linux archive checks remain separate from
  unverified platform/release gates.
- CLI/backend failures return nonzero and print diagnostics to stderr once;
  invalid config no longer panics or silently overwrites configuration.
- Native framed IPC validates complete messages, roles, peer UID and bounds;
  standalone input/kill requests are acknowledged and kill waits for shutdown.
- Native duplicate startup cannot replace a live socket; private runtime paths,
  stable name locks and instance readiness replace pathname-only discovery.
- PTY process-group cleanup reserves the child identity until reaping. Client
  queues, input cancellation, resize rollback and attach/detach TTY restoration
  prevent slow viewers and abandoned input goroutines from stalling the session.
- Read-only attach is enforced for native/tmux and never creates sessions.
- tmux resolves exact session/window/pane IDs before mutations, including
  duplicate names, nondefault indexes and focus changes. Literal input preserves
  key names, leading dashes, Unicode and semicolons without claiming completion.

### Changed
- Native protocol v1 coexists with untouched legacy sessions; keep the old binary
  to finish them. Native nuke refuses incompatible legacy candidates.
- Native input uses terminal 0/0; input acceptance never claims command completion.
  Native logging is opt-in, native-only and requires private regular files.
- Native Windows persistence is explicitly unavailable. Linux amd64 validation is
  recorded; macOS runtime evidence is still required before its release claim.
- Shared pinned Ghostty/Zig builds and guarded integration/race/fuzz test targets.
- Screen/Zellij unsafe exact pane, close/rename/split and placeholder-preview
  operations now return unsupported; Screen retains full session IDs. Their
  real-runtime support remains unverified. Their profiles return unsupported.
- Human tmux lists render typed session names and attachment counts. Hidden dump
  defaults to plain text; raw snapshot sequences require `--raw` explicitly.

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
