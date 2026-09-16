# `txm` Roadmap

The following features are planned for future releases of `txm`. 

## 1. TUI Dashboard (Interactive Manager)
A full TUI dashboard to visualize all sessions, windows, and resource usage (CPU/Mem per session) across all backends. This would expand the current simple fuzzy finder into a full "mission control" for terminal multiplexers.

## 2. Smart Command Notifications
Trigger desktop notifications or run scripts when a long-running command finishes in an unattached session, or when specific output is detected. (e.g., `txm notify my-session "npm run build"`).

## 3. Cloud Sync / Portable Configs
Sync `txm` profiles and global configurations across multiple machines seamlessly via GitHub Gists or a custom backend to ensure consistent environments everywhere.

---

## Currently Blocked Features

The following features were considered but are currently blocked by technical limitations in the underlying multiplexer tools.

### State Snapshotting & Resurrect
* **Goal**: Save the active state of all sessions (pane layouts, scrollback, commands) and restore them after a system reboot.
* **Blocker**: While possible with `tmux`, programmatically extracting internal pane layouts, sizes, and running command history from `zellij` and `GNU screen` is currently unsupported or requires extremely brittle text scraping.

### Cross-Backend Migration
* **Goal**: Seamlessly migrate a session from one backend to another (e.g., `txm migrate my-session --to zellij`).
* **Blocker**: Similar to the resurrect feature, migrating a session requires obtaining the exact state of the source backend to replicate in the target backend. The lack of standard state-extraction APIs across tools makes this infeasible at this time.
