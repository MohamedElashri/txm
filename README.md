# txm: Terminal Session Manager

`txm` is a unified command-line interface for managing terminal multiplexer sessions. It supports a built-in **native** backend for lightweight session persistence, as well as **tmux**, **zellij**, and **GNU Screen** through a single consistent set of commands.

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

Requirements: Go 1.25.11+, and at least one multiplexer installed.

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
# Launch interactive session picker (fuzzy finder with previews)
txm

# Create and attach to a session
txm create my-project
txm attach my-project

# Or create a session running a specific command
txm create my-server npm run dev

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
3. Auto-detection: tmux → screen → zellij → native

```bash
# Set a preferred backend
txm config set backend native

# Advanced: Edit ~/.txm/config manually to change native scrollback size:
# scrollback_size=131072

# View current settings
txm config show

# One-off override
TXM_DEFAULT_BACKEND=native txm create my-session
```

---

## Commands

### Session Management

| Command | Description |
|---------|-------------|
| `txm` | Launch interactive session picker |
| `txm create <name> [command...]` | Create a new session (optionally running a specific command) |
| `txm list` | List all active sessions (and client counts) |
| `txm attach [name] [command...]` | Attach to a session (auto-attaches if only 1 exists, or creates it) |
| `txm detach` | Detach from the current session |
| `txm delete <name>` | Delete a session |
| `txm rename-session <old> <new>` | Rename a session |
| `txm exec <session> <win> <pane> <cmd>` | Execute a command in a session |
| `txm nuke` | Kill all sessions |
| `txm generate-ssh-config` | Output recommended config for seamless SSH workflows |

### Window Management (`txm window`)

| Command | Description |
|---------|-------------|
| `txm window new <session> [name]` | Create a new window/tab |
| `txm window list <session>` | List windows in a session |
| `txm window kill <session> <window>` | Remove a window |
| `txm window rename <session> <old> <new>` | Rename a window |
| `txm window next <session>` | Switch to next window |
| `txm window prev <session>` | Switch to previous window |
| `txm window split <session> <window> <v\|h>` | Split a window into panes |

### Pane Management (`txm pane`)

| Command | Description |
|---------|-------------|
| `txm pane list <session> <window>` | List panes in a window |
| `txm pane kill <session> <window> <pane>` | Remove a pane |

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

| Feature | Native | tmux | zellij | GNU Screen |
|---------|:------:|:----:|:------:|:----------:|
| Session management | ✓ | ✓ | ✓ | ✓ |
| Window operations | ✗ | ✓ | ✓ | ✓ |
| Session rename | ✗ | ✓ | ✗ | ✗ |
| Pane split (vertical) | ✗ | ✓ | ✓ | ✓ |
| Pane split (horizontal) | ✗ | ✓ | ✓ | ✗ |
| Pane targeting by number | ✗ | ✓ | ✗ (focus-based) | ✗ |

**Pane targeting notes:**
- **tmux**: Directly targets panes by number (`0`, `1`, `2`, …)
- **zellij**: Navigates to the target pane via focus commands (best-effort)
- **screen**: Basic splitting only; no per-pane addressing

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `TXM_DEFAULT_BACKEND` | Override the active backend (`tmux`, `zellij`, `screen`) |
| `TXM_SESSION_PREFIX` | Automatically prefixes all session names globally |
| `NO_COLOR` | Disable colored output |
| `TERM` | Used for terminal capability detection |

---

## Seamless SSH Workflow

Using `txm` with SSH is a first-class citizen. Instead of using SSH to remote into a system with a single terminal and managing `n` `tmux` panes remotely, we can open `n` local terminal tabs and run `ssh` for all of them. This allows us to leverage our local terminal emulator's native tabs, splits, and scrollback, while perfectly preserving the sessions remotely.

To make this a delightful workflow, we can add an SSH config entry for our remote dev server. Generate the recommended setup block instantly by running:
```bash
txm generate-ssh-config
```

The output will look something like this. Add it to our `~/.ssh/config`:
```bash
Host d.*
    HostName 192.168.1.xxx
    
    # Automatically attach to a txm session on the remote server
    # named after the remote host we're connecting to (%h).
    RemoteCommand txm attach %h
    RequestTTY yes

    # Multiplex multiple PTY sessions to a single server over one connection
    ControlMaster auto
    ControlPath ~/.ssh/cm-%r@%h:%p
    ControlPersist 10m
```

Architecturally, SSH supports multiplexing multiple channels of communication within a single TCP connection to a server. `ControlMaster` is the setting that tells SSH to multiplex multiple PTY sessions over that one connection.

Now we can spawn as many terminal sessions as we'd like:
```bash
ssh d.term
ssh d.irc
ssh d.dotfiles
```

Because `txm attach` is an "upsert" command, the first `ssh` command will automatically create the session on the remote host, and subsequent commands to the same host will attach to it (or create entirely new sessions if connecting to different hosts). 

### Auto-Reconnection
We can use the `autossh` tool to make SSH connections auto-reconnect. If we close our laptop lid, it will automatically reconnect all our SSH connections when we reopen it:
```bash
autossh -M 0 -q d.term
```
*(Tip: create an alias/abbr for this, e.g., `alias ash="autossh -M 0 -q"`)*

Now we can set up our OS tiling windows how we like them for our project and have as many windows as we'd like, replicating exactly what `tmux` does but with native windows, tabs, splits, and scrollback!

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
