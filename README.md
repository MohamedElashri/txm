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

### Pre-built Binaries

Download the appropriate archive for your platform from the [releases page](https://github.com/MohamedElashri/txm/releases) and extract. For maximum portability on Alpine Linux or minimal Docker containers, use the `-musl` binaries which are 100% statically linked.
```bash
unzip txm_Linux_x86_64.zip
# or unzip txm_Linux_x86_64-musl.zip for fully static binary
./txm install          # user-local (~/.local/bin)
# or
./txm install --system # system-wide (/usr/local/bin) — requires sudo
```

### Build from Source

Requirements: Go 1.25.11+
```bash
git clone https://github.com/MohamedElashri/txm
cd txm
make build
./bin/txm install
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

For a comprehensive guide covering configuration, window/pane management, backends, environment variables, and SSH workflows, see the **[User Guide](docs/user-guide.md)**.

---

## Documentation

- Full command reference & guide: **[User Guide](docs/user-guide.md)**
- Changelog: [`docs/CHANGELOG.md`](docs/CHANGELOG.md)
- Man page: `man txm` (installed alongside the binary)

---

## Contributing

Contributions are welcome! Please open an issue or submit a pull request on [GitHub](https://github.com/MohamedElashri/txm).

## License

GNU General Public License v3.0 — see [LICENSE](LICENSE)
