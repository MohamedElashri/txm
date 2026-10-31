# Changelog

Changes follow [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.1.0] - 2026-10-31

Release preparation; no tag or published release is implied by this entry.
The final signed-source and matching-platform archive gates remain required.

### Upgrade notes

- Public JSON schema 1, exit codes, native protocol v1 and strict private v1
  registry/receipt formats remain compatible. Canonical provider observations
  and current-version associations use separate private sidecars. Actual tagged
  v2.0/current coordinator fixtures cover both directions and credential rotation.
- Restart an older coordinator explicitly before requesting new operations;
  `agent stop-coordinator` leaves sessions and children running. Old coordinators
  refuse unsupported metrics creation before launching work. Keep the original
  state directory and previous binary for existing work.
- Provider compatibility, validation and trust are separate. Unknown versions
  refuse hook activation. No provider account, trust setting or permission is
  enabled by upgrading. Claude automatic setup refuses an existing `--settings`
  argument; manually review composition instead. Codex configuration/trust stays
  manual. Other inherited-policy combinations and platforms remain unverified.
- Merge apply is limited to a clean checked-out target and an explicitly reviewed
  fast-forward, with operator quiescence and immutable worker exit proof.
  Ref-only targets, sparse worktrees and submodules are refused. Retained uncertain
  intents require inspection; retry never reapplies or rolls back a merge.
- MCP reads and creation are independent default-deny grants. Preserve permanent
  operation-ID journals after dropped responses or uncertainty. Stop, cleanup
  and merge remain human CLI handoffs, not executable MCP tools.
- Metrics are off by default and cover only the exact supervised child PID.
  CPU 100% means one logical CPU; RSS is bytes. No descendant totals are claimed.

### Added

- Version-specific neutral Claude/Codex lifecycle observations with independent
  process, attention and turn identity; stale/replay/foreign-session rejection,
  natural expiry and restart preservation. Discovery reports capability,
  compatibility, validation and nullable trust/enablement separately.
- Strict existing-state CLI reads and scoped stdio MCP through the pinned Go SDK,
  independently tested legacy initialization and modern per-request protocol
  eras, bounded pagination/waits and reviewed fixed creation templates.
- Config/executable-bound task setup acceptance, retained setup receipts, pure
  bounded task review and checked-out fast-forward merge with frozen OIDs,
  review digest and durable uncertain/applied outcomes.
- Attention-first dashboard navigation, repository/task grouping and stable
  selection; explicit sourced fresh/unknown observations and bounded previews.
- Opt-in Linux/macOS child-PID CPU/RSS samples with kernel identity checks,
  bounded sampler/storage/retention quotas and existing-only reads. Disabled jobs
  allocate no sampling artifacts; observer failure leaves the job running.
- Provider-free quick start, reviewed CLI/man/completion examples and a real
  roughly 60-second generic CLI/PTY demonstration on Linux and macOS.

### Fixed

- Convert Darwin libproc CPU Mach ticks with the kernel timebase instead of
  treating them as nanoseconds. Native Apple Silicon regression tests compare
  CPU against getrusage and RSS against ps; the previous CPU value was about
  42 times too small on the tested host.
- Preserve provider prompt/session identity, resumed state and real failure/end
  observations without treating provider turn completion as process exit.
- Canonicalize owned temporary fixture roots on macOS, use portable sleep paths,
  retain bounded race-test task deadlines and verify recorder TTY restoration
  before Darwin revokes its controlling session.
- Publish native fixture identity receipts atomically and tolerate temporary
  connection-pool refusals only in the bounded recovery probe. Rejected test
  handshakes close their sockets.

### Provider validation scope

| Provider | Evidence and limits |
| --- | --- |
| Generic | Offline authenticated worker signals and immutable native/tmux exit receipts; no provider account |
| Claude 2.1.309 | Authorized Linux/amd64 interactive restricted/default-permission reference: eight callback kinds, turns, permission cancellation, tool success/failure, resume, API failure/recovery, exit, restart/expiry and negative identities. Direct user-cancellation signaling and other modes/platforms remain unverified or unsupported. |
| Codex CLI 0.166.0 | Authorized Linux/amd64 interactive no-daemon reference with normally trusted neutral hooks: seven callback kinds, turns, permission cancellation, interrupt, resume, tool/turn/session end, restart/expiry and fork-rebind refusal. Other modes/platforms remain unverified. |
| Claude 2.1.308 / Codex CLI 0.165.0 | Schema/fixture compatibility retained; no equivalent full live-lifecycle claim |
| OpenCode 2.2.5 | Linux/macOS arm64 SDK/loader/auth/session-metadata/crash-restart fixtures pass, including a retry against the upgraded installed v2 CLI. Signed-out free-model attempts emitted real execution-started/failed events and returned provider free-limit HTTP 429 at zero reported cost. Successful completion, permission/resume lifecycle and canonical activation remain unverified/disabled. A separate earlier v1.19.5 free-model attempt timed out; it does not establish v2 compatibility. |

Exact reference executable hashes/platforms are checked by diagnostics; matching
only a version string does not attest a running instance or its trust settings.
OpenCode's final release disposition remains a maintainer gate.

### Validation and remaining release gates

- Linux reference evidence covers the development checkpoint's Go/race/fuzz,
  runtime, MCP/Git/metrics, provider lifecycles and GNU/static-musl build partition.
  It does not certify changed final release bytes or native ARM64/Windows runners.
- Native macOS 27.2 arm64 passes Go 1.27.3 unit/race, Go 1.26.10 minimum-toolchain
  tests, lint, module checks, five fuzz targets, all applicable normal/race runtime
  suites, exact tagged compatibility, child metrics, isolated tmux 3.8,
  Zellij 0.45.1, Apple Screen 4.00.03 and real private OpenSSH loopback checks.
- Both Darwin architectures build and pass archive structural checks. The arm64
  2.1.0 snapshot also passes private installation and the complete functional
  runtime matrix. Intel runtime, final signed-source archive validation,
  macOS foreign-UID CI, native Linux
  ARM64, Windows CLI smoke and the complete final eight-archive gate remain open.
- Three alternating full-scale macOS/v2.0 comparisons pass every fixed and 20%
  relative budget: median startup +15.7%, list10 +19.9%, list100 -3.6%, and peak
  server RSS +1.9%. The final arm64 snapshot runtime check passes the 100-session
  fixed budgets. Earlier attempts below the unchanged 4 GiB available-memory
  guard remain recorded as refusals. These fixture measurements are not a
  performance SLA; historical Linux evidence retains its original scope.
- A separate authorized live-provider demonstration and a final signed candidate
  source/CI run remain pending. No new macOS provider inference or trust approval
  is claimed by generic runtime tests.

### Documentation and release engineering

- Consolidate operator workflows in the existing guides and publish the actual
  MCP, setup/review/merge, metrics and provider validation limits. No installed
  agent-skill package is supplied.
- Keep lightweight CI scope/metadata checks for documentation-only changes and
  full manual CI for exact-source release validation. Retain independently
  retryable build partitions and all eight matching-runner archive gates before
  draft creation; no required check is disabled by local validation.

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
