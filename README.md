<div align="center">

<h1>txm</h1>
<p><strong>Persistent terminals. Supervised jobs. One CLI.</strong></p>

<p>
  <a href="https://github.com/MohamedElashri/txm/releases"><img src="https://img.shields.io/github/v/release/MohamedElashri/txm?style=flat-square&amp;color=22c55e" alt="Latest release"></a>
  <a href="https://github.com/MohamedElashri/txm/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/MohamedElashri/txm/ci.yml?branch=main&amp;style=flat-square&amp;label=CI" alt="CI status"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-GPL--3.0-2563eb?style=flat-square" alt="GPL-3.0 license"></a>
</p>

<p>
  <a href="docs/user-guide.md#installation">Get started</a> ·
  <a href="docs/README.md">Documentation</a> ·
  <a href="https://github.com/MohamedElashri/txm/releases">Downloads</a> ·
  <a href="docs/CHANGELOG.md">Changelog</a>
</p>

</div>

txm manages interactive terminals and background work across projects and SSH
hosts. Use its built-in native backend or your existing tmux, Zellij, or GNU
Screen setup.

## What you can do

| Capability | What you get |
| :--- | :--- |
| **[Persistent sessions](docs/user-guide.md#your-first-session)** | Keep shells running with built-in Ghostty terminal emulation. Detach and reconnect. |
| **[Workspace profiles](docs/user-guide.md#profiles)** | Recreate commands, working directories, and tmux pane layouts from TOML. |
| **[Jobs & coding agents](docs/user-guide.md#supervised-jobs)** | Supervise any command, record its exit result, and add opt-in attention hooks. |
| **[Keyboard dashboard](docs/user-guide.md#dashboard)** | Search sessions, preview output, and attach without leaving the terminal. |
| **[Git worktree tasks](docs/user-guide.md#git-worktree-tasks)** | Run project tasks in dedicated worktrees with tracked jobs and explicit cleanup. |
| **[SSH hosts](docs/user-guide.md#ssh-hosts)** | Discover and attach to remote sessions through configured OpenSSH hosts. |
| **[Automation](docs/user-guide.md#json-for-scripts)** | Query versioned JSON, stream live events, and wait on job state. |

Linux and macOS are the primary platforms. See
[platform and backend support](docs/user-guide.md#platforms-and-backends) for the full scope.

## Get started

Install the latest stable release on Linux or macOS (`curl` and `unzip` required):

```sh
curl -fsSL https://raw.githubusercontent.com/MohamedElashri/txm/main/utils/install.sh | bash
export PATH="$HOME/.local/bin:$PATH"
```

[Release archives](https://github.com/MohamedElashri/txm/releases/latest) ·
[Build from source](docs/user-guide.md#build-from-source)

```sh
# Start a persistent shell. Detach with Ctrl+backslash.
txm --backend native create work
txm attach native/work

# Find sessions and preview their output.
txm dashboard --backend native

# Supervise a test run from a Go project.
txm --backend native agent start tests -- go test ./...
txm agent wait AGENT_ID --until exited --timeout 5m --fail-on-nonzero
```

Use the ID returned by `agent start` in place of `AGENT_ID`.

## Contributing

[Contributing guide](CONTRIBUTING.md) ·
[Report an issue](https://github.com/MohamedElashri/txm/issues) ·
[GPL-3.0 license](LICENSE)
