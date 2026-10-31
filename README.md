# txm

**Terminal workspaces for people, scripts, and coding agents.**

txm brings persistent terminal sessions, supervised commands, Git worktrees, and
SSH hosts into one CLI. Use its built-in native backend or manage existing tmux,
Zellij, and GNU Screen sessions. Work interactively, watch jobs in a dashboard,
or build automation with structured output and events.

[Get started](docs/getting-started.md) · [Documentation](docs/README.md) ·
[Releases](https://github.com/MohamedElashri/txm/releases) · [Changelog](docs/CHANGELOG.md)

## What you can do

- **Keep work running.** Create persistent sessions, detach, reconnect, and
  inspect terminal output. Native sessions need no external multiplexer.
- **Describe your workspace.** Apply TOML profiles with commands, working
  directories, and tmux window and pane layouts.
- **Supervise jobs and agents.** Launch commands with recorded exit results,
  wait for completion, and follow explicit attention signals through events.
- **See your sessions in one place.** Search and filter the keyboard-driven
  dashboard, preview output, and attach to the session you need.
- **Work across projects and hosts.** Run tasks in fresh Git worktrees and
  discover or attach to sessions on explicitly configured SSH hosts.

Sessions work independently of the optional agent coordinator. You can supervise
ordinary commands without an AI provider; coding-agent attention hooks are opt-in.

## Get started

Download a published archive from [Releases](https://github.com/MohamedElashri/txm/releases),
then extract it and run `./txm install` on Linux or macOS. The
[installation guide](docs/getting-started.md#installation) covers platform
selection, the stable installer, and source builds.

These docs describe **v2.0.0**. Use the matching release archive or
[build this checkout](docs/getting-started.md#build-from-source).

```sh
# Start a persistent terminal using the built-in backend.
txm --backend native create work
txm attach native/work

# Detach with Ctrl+backslash, then find your session again.
txm --backend native list
txm dashboard --backend native
```

To supervise a command in a Go project:

```sh
txm --backend native agent start build -- go test ./...
txm agent wait AGENT_ID --until exited --timeout 5m --fail-on-nonzero
```

Replace `AGENT_ID` with the ID returned by `agent start`.
[Automation](docs/automation.md) covers receipts, coding-agent hooks, worktree
tasks, and remote sessions.

Linux and macOS are the primary runtime platforms. Backend capabilities vary;
see the [user guide](docs/user-guide.md) and
[compatibility matrix](docs/releasing.md#platforms-and-backends).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for builds, tests, and pull requests.
Report bugs or propose features through [GitHub Issues](https://github.com/MohamedElashri/txm/issues).

Licensed under [GPL-3.0](LICENSE).
