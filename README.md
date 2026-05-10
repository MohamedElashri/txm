# txm: Terminal Session Manager

`txm` is a unified command-line interface for managing terminal multiplexer sessions. It supports **tmux**, **zellij**, and **GNU Screen** through a single consistent set of commands, with automatic backend detection and native shell completions.

---

## Installation

### Quick Install (recommended)

Downloads and installs the latest release automatically.

**User-local** (installs to `~/.local/bin`):
```bash
curl -s https://raw.githubusercontent.com/MohamedElashri/txm/main/utils/install.sh | bash
```

**System-wide** (installs to `/usr/local/bin`, requires root):
```bash
curl -s https://raw.githubusercontent.com/MohamedElashri/txm/main/utils/install.sh | sudo bash -s -- --system
```

The installer also handles the man page and shell completions automatically.

### Pre-built Binaries

Download the appropriate archive for your platform from the [releases page](https://github.com/MohamedElashri/txm/releases):

| Platform | Archive |
|----------|---------|
| Linux x86_64 | `txm_Linux_x86_64.zip` |
| Linux arm64 | `txm_Linux_arm64.zip` |
| macOS x86_64 | `txm_Darwin_x86_64.zip` |
| macOS arm64 | `txm_Darwin_arm64.zip` |

After downloading, extract and install:
```bash
unzip txm_Linux_x86_64.zip
./txm install          # user-local (~/.local/bin)
# or
./txm install --system # system-wide (/usr/local/bin) — requires sudo
```

### Build from Source

Requirements: Go 1.24+, and at least one multiplexer installed.

```bash
git clone https://github.com/MohamedElashri/txm
cd txm
make build
./bin/txm install      # user-local
# or
sudo ./bin/txm install --system
```

---

## Autocompletion

Shell completions are installed automatically during `txm install` for **bash**, **zsh**, and **fish**.

| Shell | Completion path (user) |
|-------|----------------------|
| Bash | `~/.local/share/bash-completion/completions/txm` |
| Zsh | `~/.zfunc/_txm` |
| Fish | `~/.config/fish/completions/txm.fish` |

> **Zsh only**: add `fpath+=~/.zfunc` to your `~/.zshrc` before the `compinit` call.

For a temporary session, or for **PowerShell**, generate on the fly:
```bash
source <(txm completion bash)   # bash
source <(txm completion zsh)    # zsh
txm completion fish | source    # fish
txm completion powershell       # powershell
```

---

## Quick Start

```bash
# Create and attach to a session
txm create my-project
txm attach my-project

# List sessions
txm list

# Create a window inside the session
txm new-window my-project editor

# Split the window vertically
txm split-window my-project editor v

# Delete the session when done
txm delete my-project
```

---

## Configuration

txm selects a backend in this priority order:

1. `TXM_DEFAULT_BACKEND` environment variable (one-off override)
2. `~/.txm/config` file (persistent)
3. Auto-detection: tmux → zellij → screen

```bash
# Set your preferred backend
txm config set backend zellij

# View current settings
txm config show

# One-off override
TXM_DEFAULT_BACKEND=tmux txm create my-session
```

---

## Commands

### Session Management

| Command | Description |
|---------|-------------|
| `txm create <name>` | Create a new session |
| `txm list` | List all active sessions |
| `txm attach <name>` | Attach to an existing session |
| `txm detach` | Detach from the current session |
| `txm delete <name>` | Delete a session |
| `txm rename-session <old> <new>` | Rename a session |
| `txm nuke` | Kill all sessions |

### Window Management

| Command | Description |
|---------|-------------|
| `txm new-window <session> [name]` | Create a new window/tab |
| `txm list-windows <session>` | List windows in a session |
| `txm kill-window <session> <window>` | Remove a window |
| `txm rename-window <session> <old> <new>` | Rename a window |
| `txm next-window <session>` | Switch to next window |
| `txm prev-window <session>` | Switch to previous window |
| `txm move-window <src-session> <window> <dst-session>` | Move window between sessions |
| `txm swap-window <session> <w1> <w2>` | Swap two windows |
| `txm split-window <session> <window> <v\|h>` | Split a window into panes |

### Pane Management

| Command | Description |
|---------|-------------|
| `txm list-panes <session> <window>` | List panes in a window |
| `txm kill-pane <session> <window> <pane>` | Remove a pane |
| `txm resize-pane <session> <window> <pane> <U\|D\|L\|R> <size>` | Resize a pane |
| `txm send-keys <session> <window> <pane> <keys>` | Send keystrokes to a pane |

### Utility

| Command | Description |
|---------|-------------|
| `txm config set backend <tmux\|zellij\|screen>` | Set default backend |
| `txm config get backend` | Show current backend |
| `txm config show` | Show all configuration |
| `txm install [--system]` | Install binary, man page, and completions |
| `txm uninstall` | Remove txm and all its files |
| `txm version` | Show version |
| `txm completion <bash\|zsh\|fish\|powershell>` | Generate shell completion script |

> **Note**: Session and window names may only contain alphanumeric characters, dashes (`-`), and underscores (`_`).

---

## Backend Support

| Feature | tmux | zellij | GNU Screen |
|---------|:----:|:------:|:----------:|
| Session management | ✓ | ✓ | ✓ |
| Window operations | ✓ | ✓ | ✓ |
| Session rename | ✓ | ✗ | ✗ |
| Move window | ✓ | ✗ | ✗ |
| Swap window | ✓ | ✗ | ✗ |
| Pane split (vertical) | ✓ | ✓ | ✓ |
| Pane split (horizontal) | ✓ | ✓ | ✗ |
| Pane targeting by number | ✓ | ✗ (focus-based) | ✗ |

**Pane targeting notes:**
- **tmux**: Directly targets panes by number (`0`, `1`, `2`, …)
- **zellij**: Navigates to the target pane via focus commands (best-effort)
- **screen**: Basic splitting only; no per-pane addressing

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `TXM_DEFAULT_BACKEND` | Override the active backend (`tmux`, `zellij`, `screen`) |
| `NO_COLOR` | Disable colored output |
| `TERM` | Used for terminal capability detection |

---

## Documentation

- Full command reference: [`docs/user-guide.md`](docs/user-guide.md)
- Changelog: [`docs/CHANGELOG.md`](docs/CHANGELOG.md)
- Man page: `man txm` (installed alongside the binary)

---

## Uninstallation

`txm uninstall` removes the binary, man page, and all shell completion files automatically.

```bash
txm uninstall          # user-local installation
sudo txm uninstall     # system-wide installation
```

---

## Contributing

Contributions are welcome! Please open an issue or submit a pull request on [GitHub](https://github.com/MohamedElashri/txm).

## License

GNU General Public License v3.0 — see [LICENSE](LICENSE)
