# txm - A Terminal Session Manager

`txm` is a command-line utility designed to manage terminal multiplexer sessions efficiently. It primarily works with tmux, with GNU Screen support as a fallback option. This makes it versatile across different environments and setups.

## Features

- Primary support for tmux with automatic fallback to GNU Screen
- Colored output support with automatic terminal capability detection
- Comprehensive session management (create, list, attach, delete)
- Window management (create, rename, kill, swap)
- Advanced tmux-specific features (pane splitting, resizing, key sending)
- Verbose mode for debugging and learning
- Cross-platform compatibility

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
- Either tmux or GNU Screen installed

Steps:

1. Clone the repository:
   ```bash
   git clone https://github.com/MohamedElashri/txm
   ```

2. Navigate to project:
   ```bash
   cd txm
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

For complete documentation, see [docs.md](docs.md).

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