# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.2.2] - 2026-07-10

### Fixed
- **libghostty Thread Safety**: Fixed a critical race condition in the native backend where concurrent connections could cause segmentation faults when accessing the VT terminal state.
- **Scrollback Layout Restoration**: Fixed a bug where attaching to a native session from a window with a different size would restore scrollback text using the default 80x24 layout, causing text garbling. PTY resize events are now properly propagated to the internal VT core.

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
