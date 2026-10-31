# User guide

[Documentation](README.md) / User guide

Use this guide for interactive sessions, workspace profiles, and the dashboard.
[Getting started](getting-started.md) covers installation;
[Automation](automation.md) covers supervised jobs, Git tasks, and SSH hosts.
Use `txm COMMAND --help` for the full command reference.

## Choose a backend

Use the native backend for a persistent terminal without installing another
multiplexer. Choose tmux for exact window and pane control, layouts, and history.
Zellij and GNU Screen provide basic session management through the same CLI.

| Capability | Native | tmux | Zellij / GNU Screen |
| --- | --- | --- | --- |
| Create, list, attach, delete sessions | Yes | Yes | Yes, with adapter limits |
| Read-only attachment | Yes | Yes | Unsupported |
| Capture visible output | Yes | Yes | Unsupported |
| Capture history | Unsupported | Yes | Unsupported |
| Exact window/pane operations and layouts | One terminal only | Yes | Unsupported |
| Profiles | One terminal | Full layouts, tmux 3.4+ | Unsupported |
| Supervised jobs and Git tasks | Yes | Yes | Unsupported |

Available capabilities depend on the installed backend. Inspect them directly:

```sh
txm --backend tmux backend capabilities --json
txm backend capabilities --all-backends --json
```

Aggregate queries attempt every backend and report missing or failed adapters
explicitly. They can return useful data with a nonzero partial-failure exit.
[Compatibility and releases](releasing.md#platforms-and-backends) records tested
platforms and versions.

## Sessions and targets

```sh
txm --backend native create work
txm attach native/work
txm attach --read-only native/work
txm session inspect native/work --json
txm delete native/work
```

`--backend` is strict: an unavailable backend is an error. A qualified target such
as `native/work` selects that backend and avoids ambiguity. An unqualified name
uses the selected backend; it does not search all backends. Names use ASCII
letters, digits, underscores, and dashes, up to 128 characters.

Without explicit selection, txm uses `TXM_DEFAULT_BACKEND`, then the configured
preference, then its default preference of tmux. Preferences can fall back to an
available adapter, in the order tmux, Screen, Zellij, native. Set a preference with
`txm config set backend tmux`; use explicit selectors in scripts.

Native detaches with Ctrl+backslash. Other backends retain their own bindings;
tmux defaults to Ctrl-b, then d. `txm detach` delegates to the current backend.
Ordinary attach can create a missing session after discovery proves it absent.
Read-only attach requires an existing session and never creates one.

Root commands such as `create`, `attach`, `delete`, and `list` are shortcuts for
`session create`, `session attach`, `session delete`, and `session list`.
`rename-session` aliases `session rename`; renaming is backend-dependent.
`nuke` removes sessions only from the selected backend.

Put options **before positional arguments** on `create`, `attach`, and `exec`.
Subsequent arguments are interpreted as the command or literal terminal input.

## Output, windows, and panes

Capture the native visible screen, or request bounded tmux history explicitly:

```sh
txm session output native/work --plain --lines 100
txm session output tmux/work --history --lines 1000 --json
```

Plain output strips terminal controls. `--raw` explicitly preserves them and uses
base64 when combined with JSON. Truncation is disclosed. Native capture is a
screen snapshot; it has no history API. `--lines` and `--bytes` bound collection.

For multiple windows and panes, use tmux:

```sh
txm --backend tmux create dev
txm --backend tmux window new dev editor
txm --backend tmux window split dev editor v
txm --backend tmux pane list dev editor
txm --backend tmux pane exec dev editor PANE_ID 'printf hello'
```

Replace `PANE_ID` with an exact ID returned by `pane list`; quote IDs containing
shell metacharacters. txm rejects ambiguous targets and synchronized tmux input
rather than broadcasting. The native backend accepts input only at window `0`,
pane `0`: `txm exec native/work 0 0 'printf hello'`.

Terminal input is not a supervised job: accepting it does not establish a child
exit result. Use [agent start](automation.md#supervised-jobs) when completion matters.

## Profiles

A TOML profile describes a workspace to create. Save this as `dev.toml`:

```toml
name = "dev"
backend = "tmux"

[[window]]
name = "editor"
layout = "even-horizontal"

[[window.pane]]
dir = "."
command = "vim"

[[window.pane]]
dir = "."
split = "h"
```

Preview the plan, then apply it:

```sh
txm profile --dry-run dev.toml
txm profile --detach dev.toml
txm profile --attach-existing dev.toml
```

Dry-run checks names, directories, topology, and capabilities without creating
sessions or runtime state. Relative directories resolve against the profile
file; `~` and environment variables are not expanded. The profile backend is
strict and must agree with `--backend` if both are given.

Applying attaches by default; `--detach` leaves the workspace running. An
existing session requires `--attach-existing` and is attached unchanged. Combine
it with `--read-only` to observe. Native profiles allow one unnamed window and one
pane. Profile commands are terminal input, with the same completion limits as
`exec`. Rollback removes only resources whose ownership can be proven.

## Dashboard

```sh
txm dashboard
txm dashboard --backend native
txm dashboard --agents --preview
```

The dashboard lists local sessions across backends by default. Use `--backend`
to narrow the view and `--agents` to include agent registrations. The latter
opts into the local coordinator; a session-only dashboard does not start it.

| Key | Action |
| --- | --- |
| Arrows / j / k, Page Up / Page Down | Move through rows |
| / | Search |
| Tab / b | Cycle state/type filters / backend filters |
| Enter / o | Attach / attach read-only |
| p / r | Toggle output preview / refresh |
| q / Ctrl-C | Quit / cancel |

Preview captures bounded plain output for the selected session and is off by
default. Stale and unsupported actions are disabled. Session and agent rows remain
separate, with lifecycle and attention shown independently. Screen/Zellij actions
are disabled in the dashboard even though their sessions can be listed.
The dashboard requires a real terminal on Linux or macOS.

## Configuration

```sh
txm config set backend tmux
txm config get backend
txm config show
txm completion zsh --install
```

Config lookup uses the first existing file: `~/.txm/config`,
`~/.txm/config.txt`, then `~/.txmrc`. Files use `key=value` entries.

| Variable | Purpose |
| --- | --- |
| `TXM_DEFAULT_BACKEND` | Backend preference; explicit selection takes precedence |
| `TXM_SESSION_PREFIX` | Prefix unqualified session names once; qualified targets are already canonical |
| `NO_COLOR` | Disable colored output |
| `XDG_STATE_HOME` | Absolute path for private persistent state |
| `XDG_RUNTIME_DIR` | Private runtime directory for sockets |
| `TERM` | Describe the actual terminal's capabilities |

Unsafe ownership, permissions, or explicit state paths cause errors rather than
silently switching to another namespace. Keep the original state directory for
managed work; [CLI contracts](contracts.md#identity-supervision-and-events)
explain identity and recovery.

For installation, updates, and migration, see
[Getting started](getting-started.md) and [upgrade review](releasing.md#upgrade-review).
