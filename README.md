# txm: Terminal Session Manager

**txm** is a unified command-line interface for managing terminal multiplexer sessions. It supports a built-in **native** backend for lightweight session persistence, as well as **tmux**, **zellij**, and **GNU Screen** through a single consistent set of commands.

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

| Platform | Architecture | Download Link (Latest) |
|----------|--------------|------------------------|
| Linux    | x86_64       | [txm_Linux_x86_64.zip](https://github.com/MohamedElashri/txm/releases/latest/download/txm_Linux_x86_64.zip) |
| Linux    | ARM64        | [txm_Linux_arm64.zip](https://github.com/MohamedElashri/txm/releases/latest/download/txm_Linux_arm64.zip) |
| Linux (Static) | x86_64 | [txm_Linux_x86_64-musl.zip](https://github.com/MohamedElashri/txm/releases/latest/download/txm_Linux_x86_64-musl.zip) |
| Linux (Static) | ARM64  | [txm_Linux_arm64-musl.zip](https://github.com/MohamedElashri/txm/releases/latest/download/txm_Linux_arm64-musl.zip) |
| macOS    | x86_64       | [txm_Darwin_x86_64.zip](https://github.com/MohamedElashri/txm/releases/latest/download/txm_Darwin_x86_64.zip) |
| macOS    | Apple Silicon| [txm_Darwin_arm64.zip](https://github.com/MohamedElashri/txm/releases/latest/download/txm_Darwin_arm64.zip) |
| Windows  | x86_64       | [txm_Windows_x86_64.zip](https://github.com/MohamedElashri/txm/releases/latest/download/txm_Windows_x86_64.zip) |
| Windows  | ARM64        | [txm_Windows_arm64.zip](https://github.com/MohamedElashri/txm/releases/latest/download/txm_Windows_arm64.zip) |

These links name release targets; the uncommitted Phases 0–8 work is not published.
Current local validation covers Linux amd64 GNU/musl snapshots. Other platform
runtime/archive gates remain open; consult the [final audit](docs/final-audit.md).

```bash
unzip txm_Linux_x86_64.zip
# or unzip txm_Linux_x86_64-musl.zip for fully static binary
./txm install          # user-local (~/.local/bin)
# or
./txm install --system # system-wide (/usr/local/bin) — requires sudo
```

### Build from Source

Requirements: Go 1.26+, Zig 0.16.0, a C compiler (CGO), pkg-config, and Git.
The build helper fetches the Ghostty revision pinned in `scripts/dependencies.env`.
It rejects an existing mismatched or dirty Ghostty checkout without modifying it;
use `GHOSTTY_SOURCE_DIR` for a separate checkout. See [build and test isolation](docs/testing.md)
for dependency overrides, Python/tmux integration-test requirements, and validation limits.
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

# Create and attach to a session (using short aliases)
txm create my-project
txm attach my-project

# Or create a session running a specific command
txm session create my-server npm run dev

# List sessions
txm list

# Inspect exact sessions and bounded output as structured data
txm --backend tmux session list --json
txm session inspect tmux/my-project --json
txm session output tmux/my-project --lines 100 --plain
txm backend capabilities --all-backends --json

# Review and apply a declarative profile (tmux 3.4+ or native single terminal)
txm profile --dry-run dev.toml
txm profile --detach dev.toml

# Opt in to supervised execution (native or tmux 3.4+)
txm --backend native agent start build -- go test ./...
txm agent list --json
txm agent status AGENT_ID --json
txm agent wait AGENT_ID --until exited --timeout 30s --json
txm events --jsonl --id AGENT_ID

# Interactive dashboard; agent subscriptions and previews are explicit
txm dashboard --backend native
txm dashboard --agents --preview

# Optional Git task: review before creating a worktree and supervised command
txm task start build --repo /path/to/repo --dry-run -- go test ./...
txm task start build --repo /path/to/repo -- go test ./...
txm task list --json

# Optional SSH: allow an alias, inspect identity, then explicitly pin it
txm remote allow devbox
txm remote probe devbox --json
# txm remote pin devbox VERIFIED_HOST_ID
# txm remote list devbox --json

# Create a window inside the session
txm window create my-project editor

# Split the window vertically
txm window split my-project editor v

# Delete the session when done
txm session delete my-project
```

For a comprehensive guide covering configuration, window/pane management, backends, environment variables, and SSH workflows, see the **[User Guide](docs/user-guide.md)**.

Use `--backend` for strict selection; environment/config settings remain fallback
preferences. Native is a single terminal; tmux supplies exact window/pane targets.
Unverified Screen/Zellij pane operations return unsupported. See the
[contracts](docs/contracts.md) for JSON/error semantics and the
[validation record](docs/testing.md) for platform and release limits.

Agent start leaves the session detached. Inspect its durable receipt for the
child exit code; input/output text never establishes success. Registration of an
existing session is unsupervised. Explicit agent workflows (including events and dashboard --agents) can start the
optional local coordinator; `txm agent stop-coordinator` stops it without killing sessions.
Phase 5 adds authenticated signals, bounded JSONL events, deterministic waits and
explicit notification actions. Generic operation needs no installed AI provider.
Claude initialization hooks have isolated real-CLI coverage; Codex configuration
requires manual review/trust, and OpenCode v2 is unsupported. See
[agent usage](docs/user-guide.md#supervised-agents) and
[M2 readiness](docs/m2-readiness.md). Earlier cross-platform M1 release gates remain blocked.

`txm dashboard` shows local backend/session observations; `--agents` adds separate
registered-agent rows with sourced lifecycle, attention and receipt state. It
leaves bare `txm` unchanged. Use `/` to search, Tab/b to filter, Enter/o to attach
or inspect read-only, p for bounded plain preview, and q to quit. Unsupported
and stale actions are disabled. No registration is joined by a reusable name.
See [dashboard usage](docs/user-guide.md#interactive-dashboard), the
[synthetic recording](docs/evidence/phase6-dashboard.cast) and
[M3 readiness](docs/m3-readiness.md). Linux development validation does not close
the inherited cross-platform release gates.

---

## Documentation

- Full command reference & guide: **[User Guide](docs/user-guide.md)**
- Changelog: [`docs/CHANGELOG.md`](docs/CHANGELOG.md)
- Future Plans : **[Roadmap](docs/ROADMAP.md)**
- Man page: `man txm` (installed alongside the binary)

---

## Contributing

Contributions are welcome! Please open an issue or submit a pull request on [GitHub](https://github.com/MohamedElashri/txm).

## License

GNU General Public License v3.0 - see [LICENSE](LICENSE)
