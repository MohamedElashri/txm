# User guide

[Documentation](README.md) / User guide

txm keeps terminal sessions running after you detach and supervises commands with
recorded exit results. This guide covers installation, interactive work, and
automation. Use `txm COMMAND --help` or `man txm` for command and flag reference.

- [Installation](#installation) and [your first session](#your-first-session)
- [Platforms and backends](#platforms-and-backends) and [backend selection](#choose-a-backend)
- [Sessions and targets](#sessions-and-targets), [output and panes](#output-windows-and-panes)
- [Profiles](#profiles), [dashboard](#dashboard), and [configuration](#configuration)
- [Supervised jobs](#supervised-jobs) and [JSON for scripts](#json-for-scripts)
- [Coding agents](#coding-agents-and-attention), [events and waits](#events-waits-and-notifications)
- [Git worktree tasks](#git-worktree-tasks) and [metadata retention](#metadata-retention)
- [SSH hosts](#ssh-hosts)
- [Updates and migration](#updates-and-migration)

## Installation

The quick installer follows the latest published stable release. To install a
particular version, choose its release archive or check out its source tag.
Use `txm version` to identify the installed build.

### Release archives

Choose a published version on [GitHub Releases](https://github.com/MohamedElashri/txm/releases)
and download the archive for your operating system and processor:

| Platform | x86_64 / Intel | ARM64 / Apple Silicon |
| --- | --- | --- |
| Linux | `txm_Linux_x86_64.zip` | `txm_Linux_arm64.zip` |
| Linux, static musl | `txm_Linux_x86_64-musl.zip` | `txm_Linux_arm64-musl.zip` |
| macOS | `txm_Darwin_x86_64.zip` | `txm_Darwin_arm64.zip` |
| Windows | `txm_Windows_x86_64.zip` | `txm_Windows_arm64.zip` |

Static musl builds are available for systems such as Alpine Linux. Windows
archives have a limited external-backend scope; native sessions, private Unix
state, and the dashboard are unsupported. For platform scope, see
[Platforms and backends](#platforms-and-backends).

Compare the archive with the release's checksums, then extract it into a new
directory. On Linux or macOS, run from that directory:

```sh
./txm install
export PATH="$HOME/.local/bin:$PATH"
txm version
```

The user installer copies the binary to `~/.local/bin`, installs `man txm`, and
sets up completions for the detected shell. It may update your shell startup
file. Use `sudo ./txm install --system` for an intentional system installation.

### Stable installer

For the latest stable release on Linux or macOS, use the repository's installer:

```sh
curl -fsSL https://raw.githubusercontent.com/MohamedElashri/txm/main/utils/install.sh | bash
```

It requires `curl` and `unzip`, installs under `~/.local/bin`, and can update
shell startup files to put that directory on PATH. It does not select release
candidates. To install a particular version, use its archive instead.

### Build from source

From a checkout of the version you want to run, install the Go version required
by [go.mod](../go.mod), the Zig version pinned in
[scripts/dependencies.env](../scripts/dependencies.env), a C compiler,
`pkg-config`, and Git. macOS also needs Command Line Tools. Then run:

```sh
make build
./bin/txm install
txm version
```

The build helper obtains the pinned Ghostty source and builds the native terminal
library. It rejects an existing dirty or mismatched dependency checkout rather
than resetting it. [Development](development.md#dependency-configuration)
explains private checkout, prefix, and compiler overrides. `go install` alone
does not build the required native library.

## Your first session

Create a session with the native backend, then attach:

```sh
txm --backend native create work
txm attach native/work
```

You now have a persistent shell. Start an editor or a long-running command, then
press **Ctrl+backslash** to detach. Your session continues running.

```sh
txm --backend native list
txm attach --read-only native/work
txm session output native/work --plain --lines 40
```

Read-only attachment lets you observe without sending input or resizing the
shared terminal. Detach with the same key. When you are finished with the session,
exit its shell or delete it explicitly:

```sh
txm delete native/work
```

Use the [dashboard](#dashboard) to find sessions, [profiles](#profiles) for
repeatable layouts, and [supervised jobs](#supervised-jobs) for recorded exit results.

## Platforms and backends

Linux and macOS are the primary runtime platforms. The native backend includes
its terminal library; tmux, Zellij, and Screen require the corresponding executable.

| Platform | Scope |
| --- | --- |
| Linux amd64 / ARM64 | Native and external backends; GNU and static-musl archives |
| macOS Intel / Apple Silicon | Native and external backends |
| Windows amd64 / ARM64 | Limited external-backend CLI scope; native persistence, private Unix state, and dashboard unsupported; external backends and installer unverified |

An archive target is not a runtime validation claim. For evidence tied to a
particular version, read its [changelog entry](CHANGELOG.md) and release workflow
reports. Official archives must pass the [package gates](development.md#release-process)
on matching runners before draft creation. Archives are unsigned and unnotarized;
checksums identify bytes, not publisher identity.

## Choose a backend

Use the native backend for a persistent terminal without installing another
multiplexer. Choose tmux for exact window and pane control, layouts, and history.
Zellij and GNU Screen provide basic session management through the same CLI.
Creating Zellij sessions with a custom command requires Zellij 0.45.1 or newer.

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
See [Platforms and backends](#platforms-and-backends) for platform scope.

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
exit result. Use [agent start](#supervised-jobs) when completion matters.

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

Run `txm` without a subcommand to open the session picker for the selected
backend. Use `txm dashboard` for the broader session and agent view:

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

## Supervised jobs

Run a command from your project directory:

```sh
txm --backend native agent start build -- go test ./...
```

The command starts in a new detached session. `agent start` returns an opaque
agent ID; use that full ID in subsequent calls. The launch uses structured argv
rather than a shell command string. Native and tmux support supervision.

```sh
txm agent list --json
txm agent status AGENT_ID --json
txm agent wait AGENT_ID --until exited --timeout 5m --fail-on-nonzero --json
```

Inspect `receipt.outcome` and `receipt.exit_code` for the child's result.
Successful `agent start` means the launch or a recorded exit was observed; it
does not mean the child succeeded. Likewise, a default `wait --until exited`
succeeds when exit is observed, even if the child failed. `--fail-on-nonzero`
makes an unsuccessful child receipt fail the wait command.

Use `--cwd /absolute/project` to select a working directory. Repeated `--env NAME`
options allow additional variable names from the launch environment; HOME, PATH,
and TERM are allowed by default. This is not a mechanism to forward values from
the invoking client into an already running backend wrapper.

The optional coordinator maintains registrations and observations. It starts for
agent workflows; ordinary sessions and the picker do not contact it. Stopping
it with `txm agent stop-coordinator` leaves sessions and children running.
Restart does not replay commands. Missing exit proof remains unknown.

`txm agent register tmux/existing-session` tracks an existing session without
supervising it; registration cannot provide a retroactive child exit result.

## JSON for scripts

Use `--json` for supported session, agent, task, and remote queries. Responses
have versioned `data`, `errors`, and `partial` fields. Aggregate results can
contain useful data while returning a nonzero failure exit, so check both the
status and envelope. [CLI contracts](contracts.md#json-and-exits) define them.

For example, this script uses `jq` to capture the returned ID and wait for a build:

```sh
#!/bin/sh
set -eu
launch="$(txm --backend native agent start --json build -- go test ./...)"
agent_id="$(printf '%s\n' "$launch" | jq -er '.data.agent.id')"
txm agent wait "$agent_id" --until exited --timeout 5m --fail-on-nonzero --json
```

## Coding agents and attention

Process lifecycle, attention, and turn completion answer different questions:

| Observation | Meaning |
| --- | --- |
| Lifecycle | Whether the supervised process is running, exited, or unproven |
| Attention | A fresh authenticated signal such as working, needs-input, or idle |
| Turn completion | An explicit hook signal for a completed turn, independent of process exit |

Terminal text is never parsed to infer those signals. To use a generic worker
that reports attention, opt in at launch:

```sh
txm --backend native agent start --provider generic worker -- /absolute/path/to/worker
```

Inside that worker, the inherited private hook context enables:

```sh
txm agent signal --from-env --state working
txm agent signal --from-env --state needs-input
txm agent signal --from-env --state turn-complete
```

For existing associations, `txm agent hooks enable AGENT_ID --provider generic`
creates or rotates credentials. Hook input is authenticated, generation-bound,
and checked for freshness and replay. Attention expires when it is no longer
fresh; it does not remain a permanent success label.

`txm agent providers --json` reports installed adapter availability. Claude
launches can opt in with `--provider claude`; generated settings stay private and
do not edit user or project settings. `txm agent hooks config AGENT_ID` prints
provider configuration for review. Exact version guards apply; check the command's reported support before enabling
hooks. Accepted versions are defined in [the adapter implementation](../pkg/agent/hooks.go).
Provider configuration requires review and applicable trust; txm does not bypass
provider permissions or activate trust automatically.

| Adapter | Integration scope |
| --- | --- |
| Generic | Authenticated worker signals; no AI provider required |
| Claude | Opt-in private generated settings; isolated initialization coverage, full conversations unverified |
| Codex | Manual configuration and trust review; runtime/trusted lifecycle unverified |
| OpenCode | Unsupported; no older-version examples are applied automatically |

Any supported executable can be supervised as a process without a provider
adapter. Attention and turn signals require an explicit integration. Historical
tested versions and limitations belong to the relevant [changelog entry](CHANGELOG.md).

## Events, waits, and notifications

```sh
txm events --jsonl --id AGENT_ID
txm agent wait AGENT_ID --until needs-input --timeout 30s --json
txm agent notify -- /absolute/path/to/notification-handler
```

Events begin with an atomic snapshot. Gaps, overflow, and coordinator restarts
require resynchronization; there is no durable replay log or exactly-once
promise. Waits consume the same observations. Timeouts exit 6; cancellation
exits 130. `exited`, `needs-input`, and `idle` are supported wait conditions.

Notifications are off until you run `agent notify`. Your handler receives bounded
JSON metadata on stdin for future attention, turn-complete, and exit events.
Actions have deadlines and per-agent throttling; initial snapshots and resyncs
do not replay actions. Registry and event payloads exclude prompts, transcripts,
and tool contents. The [CLI contracts](contracts.md) define the delivery semantics.

## Git worktree tasks

A task combines a fresh Git branch and worktree with a supervised command. Use it
for a build, an isolated project experiment, or a worker that needs its own files.
The command starts from a committed base, `HEAD` by default; uncommitted source
changes are not copied into the task.

```sh
txm task start build --repo /absolute/repo --dry-run -- go test ./...
txm task start build --repo /absolute/repo -- go test ./...
txm task list --json
txm task inspect TASK_ID --json
```

Dry-run validates the plan without creating task state or launching work. The
default backend is native; `--backend tmux` selects tmux explicitly. `--base REV`
chooses the starting revision; `--branch NEW_BRANCH` names a fresh branch.
Repository hooks and filters require explicit `--allow-repo-commands`; no setup
script is inferred. The task ID is also its managed agent ID, so `agent wait`
can observe its exit.

Inspect the task before cleaning up:

```sh
txm task stop TASK_ID
txm task cleanup TASK_ID --dry-run
txm task cleanup TASK_ID
```

`stop` targets the exact supervised process binding. Cleanup requires exit and
ownership proof and refuses modified, untracked, or ignored files by default.
To deliberately discard those worktree files, both `--force` and
`--confirm TASK_ID` are required; neither bypasses process or ownership checks.
Cleanup retains the Git branch and committed work.

Failed or uncertain starts retain resources for inspection. Missing registrations,
moved worktrees, changed filesystem identities, or lost exit proof are not
permission to guess a cleanup target. Keep the original state directory.

## Metadata retention

Completed records count toward the 128-task and 256-agent index limits. To reclaim
index capacity, clean the task first, then prune its record and registration:

```sh
txm task prune TASK_ID --dry-run
txm task prune TASK_ID --confirm TASK_ID
txm agent prune AGENT_ID --dry-run
txm agent prune AGENT_ID --confirm AGENT_ID
```

Task pruning requires an absent worktree and validated launch/exit proof where
applicable. Supervised registrations require a validated exit receipt;
unsupervised registration pruning leaves the terminal session alive. Pruning
retains branches, receipts, manifests, and hook files, so it is not disk garbage
collection. Retired agent IDs are bounded at 16,384 and cannot be replayed.

## SSH hosts

Configure working OpenSSH authentication and host-key verification first. Install
a compatible txm on the remote host, then allow its SSH configuration alias:

```sh
txm remote allow devbox
txm remote probe devbox --json
txm remote pin devbox VERIFIED_HOST_ID
txm remote list devbox --json
```

Verify the probed host ID through a trusted channel before replacing
`VERIFIED_HOST_ID` and pinning it. OpenSSH's host key and txm's host pin are
separate identity checks. Unpinned aliases permit probing, not listing or attach.
If needed, `remote allow --executable /absolute/path/to/txm devbox` selects the
remote executable. `remote hosts` lists the allowlist; `remote forget devbox`
removes its entry without changing SSH configuration or remote sessions.

`remote list` defaults to native; use `--backend tmux` for tmux. Results are grouped
by host, with exact session and native instance IDs for attachment:

```sh
txm --backend native remote attach devbox --host-id HOST_ID --name work \
  --session-id SESSION_ID --instance INSTANCE_ID --read-only
```

Substitute the verified host ID and exact observed session/instance fields.
Attachment requires a real terminal and never creates a remote session.
OpenSSH handles credentials and ControlMaster reuse; txm sends encoded requests
and checks protocol and target identity. Per-host list timeouts and partial
failures remain explicit. No public txm network service is installed.

For scripts, use the [JSON and exit contracts](contracts.md#json-and-exits)
rather than parsing human-readable session or agent listings.

## Updates and migration

Read the target release's [upgrade notes](CHANGELOG.md) before replacing your
binary, especially for changes to native protocols, persistent state, CLI exits,
and JSON contracts. Keep the previous binary available to finish sessions that
require it; updating the executable does not migrate running sessions.

Stable builds support `txm update`, which downloads only a newer stable version
and refuses downgrades or malformed/prerelease metadata. Development and
prerelease builds refuse automatic stable updates; install a selected release
explicitly. The quick installer and GitHub's latest-release endpoint select
stable releases.

Before updating automation, review strict backend selection, nonzero failure
exits, partial JSON results, and profile preflight in the [CLI contracts](contracts.md).
Keep the original state directory and retain uncertain resources for inspection.
