# txm - A Terminal Session Manager

`txm` is a powerful command-line utility designed to manage terminal multiplexer sessions efficiently. It supports multiple backends including **tmux**, **zellij**, and **GNU Screen**, making it versatile across different environments and setups.

## Features

- **Multi-backend support**: tmux (primary), zellij, and GNU Screen
- **Configuration system**: Choose your preferred backend with persistent settings
- **Environment variable override**: Temporary backend switching via `TXM_DEFAULT_BACKEND`
- **Intelligent fallback**: Automatically detects and uses available backends
- Colored output support with automatic terminal capability detection
- Comprehensive session management (`create`, `list`, `attach`, `delete`)
- Window management for all supported backends
- Advanced backend-specific features (tmux pane operations, zellij workspaces)
- Cross-compatible command interface with backend-specific optimizations

## Backend Support Matrix

| Feature | tmux | zellij | GNU Screen |
|---------|------|--------|------------|
| Session Management | ✓ | ✓ | ✓ |
| Window Operations | ✓ | ✓ | ✓ |
| Pane/Panel Operations | ✓ (full targeting) | ✓ (focus-based) | ✓* |
| Advanced Window Mgmt | ✓ | ✓ | ✗ |
| Configuration Support | ✓ | ✓ | ✓ |
| Workspace/Tab Paradigm | Windows | Tabs | Windows |

**Note**: *GNU Screen supports basic splitting but with limited pane management

## Pane Operation Differences

**tmux**: Full pane targeting by number - you can specify which pane to operate on
```bash
txm kill-pane my-session window-name 2    # Kills pane number 2
txm resize-pane my-session window-name 1 R 10  # Resizes pane 1
txm send-keys my-session window-name 0 "echo hello"  # Sends to pane 0
```

**zellij**: Focus-based operations with automatic pane navigation
```bash
# zellij attempts to navigate to the target pane number before operations
txm kill-pane my-session tab-name 2    # Navigates to pane 2, then kills it
txm resize-pane my-session tab-name 1 R 10  # Navigates to pane 1, then resizes
txm send-keys my-session tab-name 3 "echo hello"  # Navigates to pane 3, then sends keys

# Note: Navigation is best-effort - if pane doesn't exist, operates on last reachable pane
```

**GNU Screen**: Limited pane support - basic splitting only

## Configuration

### Setting Your Preferred Backend

```bash
# Set default backend persistently
txm config set backend zellij

# View current configuration
txm config show

# Get specific configuration value
txm config get backend

# Temporarily override via environment variable
TXM_DEFAULT_BACKEND=tmux txm create my-session
```

### Backend Selection Priority

1. **Environment Variable**: `TXM_DEFAULT_BACKEND` (temporary override)
2. **Config File**: `~/.txm/config` (persistent setting)
3. **Default**: tmux (if available, otherwise first available backend)


## Available Commands

| Command | Description | tmux | zellij | GNU Screen |
|---------|-------------|------|--------|------------|
| `create` | Create a new session | ✓ | ✓ | ✓ |
| `list` | List all active sessions | ✓ | ✓ | ✓ |
| `attach` | Attach to an existing session | ✓ | ✓ | ✓ |
| `detach` | Detach from current session | ✓ | ✓ | ✓ |
| `delete` | Delete a session | ✓ | ✓ | ✓ |
| `nuke` | Remove all sessions | ✓ | ✓ | ✓ |
| `new-window` | Create a new window/tab | ✓ | ✓ | ✓ |
| `list-windows` | List windows in a session | ✓ | ✓ | ✓ |
| `kill-window` | Remove a window | ✓ | ✓ | ✓ |
| `next-window` | Switch to next window | ✓ | ✓ | ✓ |
| `prev-window` | Switch to previous window | ✓ | ✓ | ✓ |
| `rename-session` | Rename an existing session | ✓ | ✓ | ✗ |
| `rename-window` | Rename a window | ✓ | ✓ | ✓ |
| `move-window` | Move window between sessions | ✓ | ✓ | ✗ |
| `swap-window` | Swap window positions | ✓ | ✗ | ✗ |
| `split-window` | Split a window into panes | ✓ | ✓ | ✓* |
| `list-panes` | List panes in a window | ✓ | ✓ | ✗ |
| `kill-pane` | Remove a pane | ✓ | ✓ | ✗ |
| `resize-pane` | Resize a pane | ✓ | ✓ | ✗ |
| `send-keys` | Send keystrokes to a pane | ✓ | ✓ | ✗ |
| `config` | Configuration management | ✓ | ✓ | ✓ |
| `update` | Update txm to the latest version | ✓ | ✓ | ✓ |
| `uninstall` | Uninstall txm | ✓ | ✓ | ✓ |
| `version` | Show version and check for updates | ✓ | ✓ | ✓ |

### Configuration Commands

| Command | Description |
|---------|-------------|
| `txm config set backend <backend>` | Set default backend (tmux/zellij/screen) |
| `txm config get <key>` | Get configuration value |
| `txm config show` | Show all configuration |

**Note**: 
- Basic session and window management commands are supported across all backends
- *GNU Screen only supports vertical splitting
- Advanced features automatically adapt to backend capabilities
- zellij uses tab-based workflow while tmux/screen use windows

## Installation

### Using Pre-built Binaries

Download pre-built binaries from the [releases page](https://github.com/MohamedElashri/txm/releases):

- `txm-ubuntu.zip`: Ubuntu (Linux)
- `txm-macOS.zip`: macOS

Installation steps:

1. Download the appropriate ZIP file
2. Extract it:
   ```bash
   unzip txm-<platform>.zip
   ```
3. Move to PATH:
   ```bash
   sudo mv txm /usr/local/bin/
   ```

Quick install using the installation script:

For user-local installation (default)

```bash
curl -s https://raw.githubusercontent.com/MohamedElashri/txm/main/utils/install.sh | bash
```
 

For system-wide installation

```bash
curl -s https://raw.githubusercontent.com/MohamedElashri/txm/main/utils/install.sh | sudo bash -s -- --system
```

### Building from Source

Requirements:
- Go 1.17 or later
- At least one terminal multiplexer: tmux, zellij, or GNU Screen

Steps:

1. Clone the repository:
   ```bash
   git clone https://github.com/MohamedElashri/txm
   ```

2. Navigate to project:
   ```bash
   cd txm
   ```

3. Build the binary:
   ```bash
   cd src
   go build -o txm
   ```

4. (Optional) Move to PATH:
   ```bash
   sudo mv txm /usr/local/bin/
   ```

## Usage Examples

### Backend Configuration

```bash
# Set zellij as your default backend
txm config set backend zellij

# View current configuration
txm config show

# Temporarily use tmux for a single command
TXM_DEFAULT_BACKEND=tmux txm create dev-session
```

### Session Management

```bash
# Create a new session (uses configured backend)
txm create my-project

# List all sessions
txm list

# Attach to session
txm attach my-project

# Create session with specific backend
TXM_DEFAULT_BACKEND=zellij txm create zellij-session
```

### Window/Tab Management

```bash
# Create new window/tab
txm new-window my-project development

# Navigate between windows
txm next-window my-project
txm prev-window my-project

# List windows in session
txm list-windows my-project
```

### Advanced Operations

```bash
# Pane operations
txm split-window my-project development v  # vertical split (all backends)
txm split-window my-project development h  # horizontal split (tmux only)

# tmux: Target specific panes by number
txm list-panes my-project development      # Lists all panes with numbers
txm resize-pane my-project development 0 R 10  # Resize pane 0
txm send-keys my-project development 1 "npm start"  # Send to pane 1

# zellij: Operations work on focused pane (pane numbers ignored)
txm resize-pane my-project tab-name 0 R 10  # Resizes focused pane
txm send-keys my-project tab-name 0 "npm start"  # Sends to focused pane
```

3. Initialize module:
   ```bash
   go mod init github.com/MohamedElashri/txm
   ```

4. Build:
   ```bash
   go build -o txm
   ```

5. Install (optional):
   ```bash
   sudo mv txm /usr/local/bin/
   ```

## Basic Usage

- Create session:
  ```bash
  txm create mysession
  ```

- List sessions:
  ```bash
  txm list
  ```

- Attach to session:
  ```bash
  txm attach mysession
  ```

- Delete session:
  ```bash
  txm delete mysession
  ```

- Create window (tmux only):
  ```bash
  txm new-window mysession windowname
  ```

## Advanced Features (tmux only)

- Split window:
  ```bash
  txm split-window mysession 0 v  # vertical split
  txm split-window mysession 0 h  # horizontal split
  ```

- Move window:
  ```bash
  txm move-window source-session 1 target-session
  ```

- Resize pane:
  ```bash
  txm resize-pane mysession 0 1 "-D 10"  # resize down 10 units
  ```

For complete documentation, see [docs.md](docs.md). Or you can run `txm help` to see the available commands. There is an old fashioned `man` page available as well, run `man txm` to see

## Environment Variables

- `NO_COLOR`: Disable colored output
- `TERM`: Used for terminal capability detection

## Update 

To update txm to the latest version, you simply need to run the following command:

```bash
txm update
```

If you have installed it as system-wide, you need to run the following command:

```bash
sudo txm update
```


## Uninstallation

Remove txm and its configurations:

There is `uninstall` option to uninstall txm that can be used to uninstall txm .

If something went wrong during the this process, you can uninstall txm using the following script:

For user-local uninstallation
```bash
curl -s https://raw.githubusercontent.com/MohamedElashri/txm/main/utils/uninstall.sh | bash
```

For system-wide uninstallation

```bash
curl -s https://raw.githubusercontent.com/MohamedElashri/txm/main/utils/uninstall.sh | sudo bash
```


## Contributing

Contributions welcome! Please submit issues and pull requests on GitHub.

## License

GNU General Public License v3.0 - see [LICENSE](LICENSE)
