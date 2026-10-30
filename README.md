# txm: Terminal Session Manager

txm manages native, tmux, Zellij and GNU Screen sessions through one CLI. It also
provides optional supervision, a dashboard, Git worktree tasks and SSH aggregation.

The next candidate is **2.0.0-rc.1**. See the [upgrade notes](docs/CHANGELOG.md#200-rc1---2026-10-30)
and [release checklist](docs/releasing.md) before upgrading existing sessions.

## Installation

Install the latest stable release to `~/.local/bin`:

```sh
curl -s https://raw.githubusercontent.com/MohamedElashri/txm/main/utils/install.sh | bash
```

For a system installation, run the installer with `sudo bash -s -- --system`.
The quick installer selects stable releases; choose an exact published archive
or build its source to test a release candidate.

[Prebuilt releases](https://github.com/MohamedElashri/txm/releases) use these names:

| Target | Archive |
| --- | --- |
| Linux x86_64 / ARM64 | `txm_Linux_x86_64.zip` / `txm_Linux_arm64.zip` |
| Static Linux x86_64 / ARM64 | `txm_Linux_x86_64-musl.zip` / `txm_Linux_arm64-musl.zip` |
| macOS Intel / Apple Silicon | `txm_Darwin_x86_64.zip` / `txm_Darwin_arm64.zip` |
| Windows x86_64 / ARM64 | `txm_Windows_x86_64.zip` / `txm_Windows_arm64.zip` |

Archive targets do not imply runtime certification. See [tested scope and open
gates](docs/releasing.md#remaining-release-gates). Windows native persistence and
dashboard are unsupported; Screen/Zellij retain degraded capabilities.

## Build

Use Go 1.26+, Zig 0.16.0, a C compiler, pkg-config and Git. The helper builds the
Ghostty revision pinned in `scripts/dependencies.env` and rejects dirty/mismatched
source without resetting it. See [Testing](docs/testing.md) for private overrides.

```sh
git clone https://github.com/MohamedElashri/txm
cd txm
make build
./bin/txm install
```

## Quick start

```sh
txm                                  # interactive picker
txm create work /bin/sh
txm list
txm attach work
txm attach --read-only work
txm session inspect native/work --json
txm session output native/work --lines 100 --plain
txm backend capabilities --all-backends --json
txm profile --dry-run dev.toml
txm --backend native agent start build -- go test ./...
txm agent wait AGENT_ID --until exited --timeout 30s --json
txm dashboard --agents --preview
txm task start build --repo /absolute/repo --dry-run -- go test ./...
txm remote allow devbox
txm remote probe devbox --json
```

Explicit `--backend` selects strictly. Native provides one terminal; tmux supplies
exact windows/panes/layouts. Ordinary sessions and the picker do not need an agent
coordinator or AI provider. Supervised exit receipts establish completion; terminal
input/output never proves success. SSH hosts require explicit allowlisting and
verified pins. Provider hooks require opt-in and reviewed trust.

## Documentation

- [User guide](docs/user-guide.md) and `txm COMMAND --help`
- [CLI/state contracts](docs/contracts.md) and [native protocol](docs/native-protocol.md)
- [Build and tests](docs/testing.md)
- [Release process and support](docs/releasing.md)
- [Changelog](docs/CHANGELOG.md)
- `man txm`, installed with the binary

Contributions are welcome through [GitHub](https://github.com/MohamedElashri/txm).
Licensed under [GNU GPL v3](LICENSE).
