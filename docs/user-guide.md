# User guide

[Documentation](README.md) / User guide

Install txm, keep sessions running and supervise jobs with recorded exit results.
Use `txm COMMAND --help` or `man txm` for the complete command reference.

- [Installation](#installation) and [your first session](#your-first-session)
- [Platforms and backends](#platforms-and-backends) and [backend selection](#choose-a-backend)
- [Sessions and targets](#sessions-and-targets), [output and panes](#output-windows-and-panes)
- [Profiles](#profiles), [dashboard](#dashboard), and [configuration](#configuration)
- [Supervised jobs](#supervised-jobs) and [JSON for scripts](#json-for-scripts)
- [Local MCP](#mcp-for-local-clients) and [optional metrics](#optional-process-metrics)
- [Coding agents](#coding-agents-and-attention), [events and waits](#events-waits-and-notifications)
- [Git worktree tasks](#git-worktree-tasks) and [metadata retention](#metadata-retention)
- [SSH hosts](#ssh-hosts)
- [Updates and migration](#updates-and-migration)

## Installation

The installer selects the latest stable release. For a specific version, use
its archive or source tag. Check your build with `txm version`.

### Release archives

Choose a published version on [GitHub Releases](https://github.com/MohamedElashri/txm/releases)
and download the archive for your operating system and processor:

| Platform | x86_64 / Intel | ARM64 / Apple Silicon |
| --- | --- | --- |
| Linux | `txm_Linux_x86_64.zip` | `txm_Linux_arm64.zip` |
| Linux, static musl | `txm_Linux_x86_64-musl.zip` | `txm_Linux_arm64-musl.zip` |
| macOS | `txm_Darwin_universal.zip` | `txm_Darwin_universal.zip` |
| Windows | `txm_Windows_x86_64.zip` | `txm_Windows_arm64.zip` |

macOS uses one universal binary for Intel and Apple Silicon. The installer also
handles older releases with separate Mac archives. Use static musl for Alpine
and similar systems. See [platform limits](#platforms-and-backends) for Windows.

Compare the archive with the release's checksums, then extract it into a new
directory. On Linux or macOS, run from that directory:

```sh
./txm install
export PATH="$HOME/.local/bin:$PATH"
txm version
```

Installation adds the binary to `~/.local/bin`, the man page and shell
completions; it may edit shell startup files. For system-wide installation,
use `sudo ./txm install --system`.

### Stable installer

For the latest stable release on Linux or macOS, use the repository's installer:

```sh
curl -fsSL https://raw.githubusercontent.com/MohamedElashri/txm/main/utils/install.sh | bash
```

Requires `curl` and `unzip`. Installs under `~/.local/bin` and may update your
shell PATH. Release candidates are excluded.

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

The helper builds the pinned Ghostty library and refuses dirty/mismatched
dependencies. `go install` alone is insufficient. See
[dependency overrides](development.md#dependency-configuration) for custom paths.

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

Runtime evidence stays with each release's CI reports. Universal Mac slices are
checked structurally; current CI executes on Apple Silicon. Archives are unsigned
and unnotarized; checksums identify bytes, not publisher identity.

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

Inspect the installed backend's capabilities:

```sh
txm --backend tmux backend capabilities --json
txm backend capabilities --all-backends --json
```

Aggregate queries report missing/failed backends and may return useful data
with a nonzero partial-failure exit.

## Sessions and targets

Inspect an exact session with `txm session inspect native/work --json`.

`--backend` is strict: unavailable means error. `native/work` selects an exact
backend; an unqualified name uses the selected backend without searching others.
Names allow ASCII letters, digits, underscores and dashes, up to 128 characters.

Preference order is `TXM_DEFAULT_BACKEND`, configuration, then tmux. Fallback
tries tmux, Screen, Zellij, native. Set a preference with
`txm config set backend tmux`; use explicit selection in scripts.

Native detaches with Ctrl+backslash; tmux defaults to Ctrl-b, then d.
`txm detach` delegates to the current backend. Ordinary attach can create a
missing session; read-only attach requires an existing one.

`create`, `attach`, `delete` and `list` alias their `session` subcommands.
`rename-session` aliases `session rename`. `nuke` affects only the selected backend.

Put options **before positional arguments** on `create`, `attach`, and `exec`.
Subsequent arguments are interpreted as the command or literal terminal input.

## Output, windows, and panes

Capture the native visible screen, or request bounded tmux history explicitly:

```sh
txm session output native/work --plain --lines 100
txm session output tmux/work --history --lines 1000 --json
```

Plain output strips terminal controls; explicit `--raw` preserves them and uses
base64 in JSON. `--lines`/`--bytes` bound capture and disclose truncation.
Native output is a screen snapshot, without history.

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

Dry-run checks the plan without creating sessions/state. Directories are relative
to the profile; `~` and environment variables are not expanded. Its backend must
agree with `--backend`.

Apply attaches unless `--detach` is given. Existing sessions require
`--attach-existing` and stay unchanged; add `--read-only` to observe. Native
profiles allow one unnamed window/pane. Commands are terminal input, not
supervised jobs. Rollback removes only proven owned resources.

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
| [ / ] | Previous / next fresh needs-input agent |
| g / c / C | Toggle repository/task grouping / collapse / expand |
| q / Ctrl-C | Quit / cancel |

Requires a real Linux/macOS terminal. Preview is bounded and off by default.
Fresh needs-input agents come first; stale/unsupported actions are disabled.
Lifecycle and attention stay separate. Screen/Zellij sessions can be listed,
but dashboard actions are disabled. A permission event alone does not prove
an interactive dialog is open.

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

Unsafe ownership, permissions or explicit paths fail. Keep the original state
directory; see [identity and recovery](contracts.md#identity-supervision-and-events).

## Supervised jobs

Run a command from your project directory:

```sh
txm --backend native agent start build -- go test ./...
```

Native and tmux launch structured argv in a new detached session. Keep the
full opaque agent ID returned by `agent start`:

```sh
txm agent list --json
txm agent status AGENT_ID --json
txm agent wait AGENT_ID --until exited --timeout 5m --fail-on-nonzero --json
```

Read `receipt.outcome` and `receipt.exit_code` for the result. Start success
only confirms launch or an observed exit. Waiting for exit alone accepts a failed
child; `--fail-on-nonzero` propagates its failure.

Set cwd with `--cwd /absolute/project`. Repeat `--env NAME` to allow variables
from the launch environment; HOME, PATH and TERM are allowed by default. Values
are not forwarded into an already running backend wrapper.

Agent workflows start an optional coordinator; ordinary sessions do not.
`txm agent stop-coordinator` leaves children/sessions running, and restart never
replays commands. Missing exit proof remains unknown.

`txm agent register tmux/existing-session` tracks an existing session without
supervising it or supplying a retroactive exit result.

For metadata reads that neither initialize state nor start a coordinator,
select the backend and exact managed IDs:

```sh
txm --backend native agent list --existing --read-id AGENT_ID --json
txm --backend native agent status AGENT_ID --existing --json
```

These read registrations/receipts, not unmanaged sessions or backend liveness.
Ordinary queries may reconcile state; `--existing` does not.

## Optional process metrics

Metrics are off by default. Opt in when starting a supervised native/tmux job:

```sh
txm --backend native agent start --metrics --metrics-interval 2 build -- go test ./...
txm agent metrics AGENT_ID --json
```

CPU uses 100% per logical CPU and may exceed 100%; RSS is bytes. Scope is only
the exact direct child (`child_pid`), excluding descendants. First CPU is unknown;
missing samples are unavailable, not zero. Process identity is checked.

Intervals are 2–60 seconds, with at most 32 active samplers. Histories expire
within 24 hours and have bounded storage. Capacity/observer failure leaves the
job running. Reads never start sampling. See [metrics contracts](contracts.md#process-metrics)
for quotas and recovery rules.

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

## MCP for local clients

Configure an MCP client to launch txm over stdio with an explicit read scope:

```sh
txm mcp serve --read-backend native --read-id AGENT_ID
```

Repeat selectors as needed. IDs and `--read-repo /owned/canonical/repo` intersect;
`--read-host` restricts local identity, not SSH discovery. No scope means denied.
Omit `--json`: stdout is the MCP transport.

Read tools cover managed agents, sessions, tasks, capabilities and finite waits.
They do not discover unmanaged sessions, capture output, access remote hosts,
start services or repair state. Session backend liveness remains unverified.

Creation needs a separate reviewed mode-0600 `--create-policy` binding fixed
argv, executable identity and cwd to named templates. Task templates also bind
a committed base and disable repository commands. Children get an empty
environment; an interpreter template grants its exact code.

Creation calls require `template_id` and a unique 32-hex `operation_id`.
Retry a lost response with the same ID/request; inspect uncertain outcomes
instead of starting again with a new ID. Keep the namespace journal for retry
safety. Stop, cleanup and merge remain human CLI handoffs.

See [MCP contracts](contracts.md#scoped-mcp-over-stdio) for protocol eras, policy
schema and limits. Grants do not sandbox arbitrary same-UID programs.

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

For existing work, `txm agent hooks enable AGENT_ID --provider generic`
creates/rotates credentials. Signals are authenticated, generation-bound,
freshness-checked and replay-protected. Attention expires.

Inspect discovery separately from explicit executable/version probing:

```sh
txm agent doctor --json
txm agent doctor --probe --json
txm agent providers --json
```

Doctor reports installed version, compatibility, validation references and
trust separately. Compatibility is not live-instance verification. Unknown
versions refuse hooks while generic process supervision remains available.

Claude can opt in with `--provider claude`; generated settings stay private.
Existing `--settings` requires manual composition. Review inherited hooks,
policies and permissions first. `txm agent hooks config AGENT_ID` emits settings
for review; omit `--json`.

Codex requires manual configuration and normal provider trust. Associate with
`txm agent hooks enable AGENT_ID --provider codex`, inspect the generated config,
and review it through the intended private configuration and `/hooks` flow.
Use plain supervision if you only need exit receipts.

| Adapter | Integration scope |
| --- | --- |
| Generic | Authenticated worker signals; no AI provider required |
| Claude | Exact-version neutral lifecycle hooks; authorized live reference covers only the recorded Linux/amd64 interactive mode |
| Codex | Exact-version neutral lifecycle hooks with manual configuration/trust; authorized live reference covers only the recorded Linux/amd64 interactive mode |
| Other tools, including OpenCode | Generic process supervision and explicit worker signals |

Any executable can use generic process supervision. Attention/turn signals need
an explicit integration; inspect capabilities with doctor.

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

Tasks run supervised commands in fresh Git branches/worktrees. They start from
a committed base (`HEAD` by default); uncommitted changes are not copied.

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

Review committed changes against an exact local target branch before applying
anything:

```sh
txm task setup-status TASK_ID --json
txm task review TASK_ID --target refs/heads/main --json
txm task review TASK_ID --target refs/heads/main --diff --json
txm task merge TASK_ID --target refs/heads/main --json
```

Review/merge dry-run create no state or services. `--diff` opts into a
control-stripped 16 KiB diff without secret redaction. Merge defaults to a
read-only plan with exact OIDs and a review digest.

A human may apply that exact reviewed fast-forward only after stopping all
writers and external Git clients:

```sh
txm task merge TASK_ID --target refs/heads/main --dry-run=false --quiescent \
  --confirm TASK_ID --expected-task TASK_OID --expected-target TARGET_OID \
  --review-digest REVIEW_DIGEST --json
```

Use full values from the dry run. The target needs a distinct clean checked-out
worktree; task/setup must have succeeded and registered workers must have exit
receipts. Ref-only targets, sparse worktrees, submodules, changed config and
non-fast-forwards are refused. `--quiescent` includes editors, external Git and
escaped descendants. No push, branch deletion or cleanup follows. Uncertain
outcomes require manual reconciliation; retries never reapply or roll back.

Optional `--setup-policy` needs a reviewed mode-0600 JSON file binding task,
base, argv, environment and executable/config identities. Review dry-run and
supply its `--accept-setup` digest. Failed/timed-out setup blocks main launch;
inspect `setup-status` before cleanup.

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

Inspect failed/uncertain starts and keep the original state directory. Missing
registrations, moved worktrees or lost exit proof never justify guessed cleanup.

## Metadata retention

Completed records count toward the 128-task and 256-agent index limits. To reclaim
index capacity, clean the task first, then prune its record and registration:

```sh
txm task prune TASK_ID --dry-run
txm task prune TASK_ID --confirm TASK_ID
txm agent prune AGENT_ID --dry-run
txm agent prune AGENT_ID --confirm AGENT_ID
```

Pruning needs an absent task worktree and applicable launch/exit proof.
Unsupervised registration pruning leaves its session alive. Branches, receipts,
manifests and hook files remain; this reclaims index capacity, not disk space.
Retired IDs cannot replay; see [contracts](contracts.md#dashboard-tasks-and-ssh).

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

Read the target release's [upgrade notes](CHANGELOG.md). Keep the previous binary
for sessions that need it; replacing the executable does not migrate sessions.

`txm update` installs only a newer stable release. Development/prerelease builds
require explicit installation. For scripts, review [CLI contracts](contracts.md)
for backend selection, failure exits and partial JSON. Keep original state and
inspect uncertain resources before cleanup.
