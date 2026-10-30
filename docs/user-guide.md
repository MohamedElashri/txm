# txm Documentation

`txm` is a powerful terminal session manager that supports a built-in **native** backend, as well as **tmux**, **zellij**, and **GNU Screen**. This documentation covers all available commands, configuration options, and backend-specific features.

## Table of Contents

- [Configuration System](#configuration-system)
- [Backend Support](#backend-support)
- [Command Line Options](#command-line-options)
- [Session Management](#session-management)
- [Supervised Agents](#supervised-agents)
- [Window Management](#window-management)
- [Pane Operations](#pane-operations)
- [Configuration Commands](#configuration-commands)
- [Environment Variables](#environment-variables)
- [Seamless SSH Workflow](#seamless-ssh-workflow)
- [Backend-Specific Notes](#backend-specific-notes)
- [Troubleshooting](#troubleshooting)

## Configuration System

txm includes a comprehensive configuration system that allows you to set your preferred backend and customize behavior.

### Configuration File

Configuration is stored in `~/.txm/config` using a simple `key=value` format:

```
backend=native
scrollback_size=131072
```

### Backend Selection Priority

1. **Strict selection**: `--backend native|tmux|zellij|screen`, or a qualified
   target such as `tmux/my-session`. Unavailable means an error without fallback.
2. **Preference**: `TXM_DEFAULT_BACKEND`, then the config file's backend.
3. **Fallback**: default preference tmux; configured order tmux → screen → zellij
   → native. Environment/config preferences permit fallback when unavailable.

### Configuration Commands

```bash
# Set default backend
txm config set backend native

# Get specific configuration
txm config get backend

# Show all configuration
txm config show
```

## Backend Support

| Backend | Session Mgmt | Window/Tab Mgmt | Pane/Panel Ops | Advanced Features |
|---------|--------------|-----------------|----------------|-------------------|
| **native** | Create/list/attach/delete | Unsupported | Input to terminal 0/0 | Observer attach, bounded visible snapshot; rename/history unsupported |
| **tmux** | Supported | Exact IDs/names/indexes | Exact pane IDs/indexes | Observer attach, bounded screen/history capture; 3.x CLI required, 3.4 tested |
| **zellij** | Degraded; Linux 0.45.1 basic lifecycle tested | New/tab navigation only | Unsupported | Exact close/rename/split and snapshots unsupported; listing may include resurrectable sessions |
| **screen** | Degraded; Linux 4.09.01 basic lifecycle tested | New/navigation only | Unsupported | Full PID.name identity retained; duplicate short names rejected |

Use `txm backend capabilities --all-backends --json` for explicit capability
levels and errors. Linux amd64 and its GNU/musl snapshots are locally validated;
macOS, other architectures and their archives retain runtime gates. Windows native and private structured state are
unsupported; cross-builds alone do not certify runtime support. Dashboard runtime evidence is Linux amd64 only;
macOS dashboard runtime is disabled pending ancestry/TTY validation; Windows is unsupported.

## Command Line Options

```
txm [command] [arguments] [-v|--verbose]
```

`--backend` selects one backend strictly. `--all-backends` is for list/capability reads and dashboard scope and conflicts with `--backend`. Mutations and `nuke` stay scoped to
one selected backend. Qualified `backend/name` targets are canonical and do not
gain `TXM_SESSION_PREFIX` again. Put flags before positional arguments on
create/attach/exec, where trailing command/input arguments are literal.

The `-v` or `--verbose` flag enables detailed logging, useful for debugging or learning how commands work.

## Autocompletion

`txm` automatically installs native shell autocompletion for `bash`, `zsh`, and `fish` when you run `txm install`. The installer places the generated completion scripts into standard locations dynamically:

- **Bash**: `~/.local/share/bash-completion/completions/txm` (or `/usr/share/bash-completion/` for system installs)
- **Zsh**: `~/.zfunc/_txm` (You must add `fpath+=~/.zfunc` to your `.zshrc`)
- **Fish**: `~/.config/fish/completions/txm.fish`

For a temporary session, or for PowerShell, you can generate it on the fly:
```bash
source <(txm completion bash)   # bash
source <(txm completion zsh)    # zsh
txm completion fish | source    # fish
txm completion powershell       # powershell
```

When you uninstall the app using `txm uninstall`, these autocompletion files are automatically purged from your system.

## Session Management (`txm session`)

Commands for managing sessions. The most common commands (`create`, `list`, `attach`) are available at the root level as aliases.

### Interactive picker
Launch interactive fuzzy-finder picker to select and preview sessions
```bash
txm
```

### create
Create a new session (optionally run a specific command).
**Alias:** `txm create`
```bash
txm session create [session_name] [command...]
```
- `--log`: Native-only opt-in PTY logging with size-based rotation. Existing log files must be owned regular files with mode 0600; symlinks/hard links are rejected.

### list
List all active sessions and display the number of active clients attached.
**Alias:** `txm list`, `txm ls`
```bash
txm session list
```

### Structured inspection and output

```bash
txm --backend tmux session list --json
txm session list --all-backends --json
txm session inspect tmux/my-project --json
txm backend capabilities --all-backends --json
txm session output tmux/my-project --lines 100 --plain
txm session output tmux/my-project --history --lines 200 --json
txm session output native/my-project --raw --json
```

JSON reads emit one `{schema_version:1,data,errors,partial}` envelope, including
expected failures. An empty sessions array is success; partial aggregate results
exit 7 with scoped errors. All-backend reads attempt all four adapters, so an
unavailable adapter is reported even when other adapters succeed. Same-named
sessions stay separate. Native instances have verified instance IDs; unmanaged
sessions have no managed ID or invented child exit code.

Local structured identity is stored privately in
`$XDG_STATE_HOME/txm/v1/host.json`, otherwise `~/.local/state/txm/v1/host.json`.
Corrupt or unsafe identity files return errors. Ordinary mutations and picker
previews do not initialize this state or start an agent coordinator.

Output defaults to the visible screen in safe plain UTF-8, with terminal controls
removed. Limits are 1..10,000 lines (default 100) and 1..1,048,576 bytes (default
1 MiB), enforced during capture. `--history` is explicit and currently tmux-only.
Raw snapshots require `--raw`; their JSON content is base64 with an encoding
field. JSON reports type, returned line/byte counts and truncation. A snapshot is
not a transcript or proof of command completion. Hidden `dump` now has the same
safe limits and plain default; existing raw consumers must request `--raw`.

Exits: 0 success, 1 general/config failure, 2 invalid usage/target, 3 not found,
4 unsupported, 5 conflict/ambiguous, 6 timeout, 7 unavailable/partial, 8 denied,
130 cancelled. See [contracts.md](contracts.md) for complete schemas and error
precedence. JSON diagnostics never share stdout with progress messages.

### attach
Attach to an existing session. If no name is provided, it automatically attaches to the only available session or creates a default one. Can also accept custom startup commands.
**Alias:** `txm attach`
```bash
txm session attach [session_name] [command...]
```
- `-r`, `--read-only`: Native observer attach or tmux enforced read-only attach. A missing session is an error; this mode never creates one. Zellij/Screen reject this request until supported. Native observers cannot write, kill or resize the shared terminal and may detach with `Ctrl+\`. Same-user processes can still open a separate control connection.

### detach
Detach from current session. (Alternatively, use `Ctrl+\` when in a native session to gracefully detach).
**Alias:** `txm detach`
```bash
txm session detach
```

### delete
Delete a session.
**Alias:** `txm delete`
```bash
txm session delete [session_name]
```

### rename
Rename a session.
**Alias:** `txm rename-session`
```bash
txm session rename [old_name] [new_name]
```

### nuke
Remove sessions in the selected backend. Native nuke refuses if incompatible legacy native sockets are present; finish those sessions with the old binary first.
**Alias:** `txm nuke`
```bash
txm session nuke
```

### profile
Validate and apply a profile with tmux 3.4+ or native's single terminal. The profile
backend is strict; a conflicting `--backend` is an error. Without either, normal
backend preferences apply. Screen/Zellij profiles are unsupported before creation.
```bash
txm profile --dry-run dev.toml
txm profile --detach dev.toml
txm profile --attach-existing dev.toml
txm profile --attach-existing --read-only dev.toml
```

Dry-run shows canonical targets, capabilities, cwd and whether commands are
present, without printing command text, discovering sessions or creating runtime
or host identity. Default apply attaches afterward; `--detach` leaves it detached.
An existing session is a conflict unless `--attach-existing` explicitly permits
attaching unchanged. Read-only requires that flag and an existing session; it
never creates/applies a profile. `TXM_READ_ONLY=1` enforces the same restriction.

TOML keys are strict, including nested tables; unknown keys are errors. Limits are
1 MiB, 32 windows and 128 panes. Session/window names use ASCII letters, digits,
dash or underscore, with the session prefix applied once. Omitted windows/panes
mean one default terminal; tmux supplies names `window-1`, `window-2`, etc. Names
must be unique. A split belongs only on additional panes (`v` by default or `h`).
Each new pane splits the preceding returned pane ID, independently of focus.

`dir` is applied before the pane shell starts. Explicit relative paths resolve
against the profile file; omitted cwd inherits invocation cwd. There is no implicit
environment or `~` expansion. Layouts are the named tmux presets `even-horizontal`,
`even-vertical`, `main-horizontal`, `main-vertical` and `tiled`; custom strings are
rejected. Native accepts one unnamed/`0` virtual window and one pane, with cwd and
optional input, but no layouts or named windows.

Profile panes launch the invocation `SHELL` (default `/bin/sh`) interactively,
overriding tmux default-command. `command` remains terminal input followed by
Enter, with no execution/completion receipt. All topology/layouts are built first.
New profile panes have synchronization disabled so input stays on the requested
pane. Ordinary exact `exec` refuses synchronized panes instead of broadcasting.

Failures roll back only a proven owned session. Missing receipts or changed
ownership leave uncertain resources intact and report partial failure (exit 7).
Cancellation still exits 130, with bounded cleanup. Previously accepted input may
have external effects that deleting a session cannot undo. Legacy behavior that
attached/reapplied implicitly or ignored fields is intentionally corrected.

**Example TOML Profile:**
```toml
name = "dev-workspace"
backend = "tmux"

[[window]]
name = "server"
[[window.pane]]
command = "npm run start"

[[window]]
name = "client"
layout = "even-horizontal"
[[window.pane]]
command = "npm run dev"
dir = "."
[[window.pane]]
split = "v"
command = "npm run test"
```

### generate-ssh-config
Automatically generate zmx-style `ControlMaster` SSH configurations for seamless SSH workflows.
```bash
txm generate-ssh-config
```

### update
Update txm to the latest version
```bash
txm update
```

### uninstall
Uninstall txm from your system
```bash
txm uninstall
```

### version
Show version and check for updates
```bash
txm version [--check-update]
```

## Interactive Dashboard

```bash
txm dashboard                       # all four local adapters; per-backend errors
txm dashboard --backend native      # strict scope, no fallback
txm dashboard --backend tmux --preview
txm dashboard --agents --refresh 2s  # explicit M2 service subscription
```

Dashboard defaults to all local adapters independently of the ordinary picker’s
backend preference. It needs terminal stdin/stdout. Use list/status JSON for
scripts; `--json` is unsupported here. `--refresh` accepts 500ms through 1m,
default 2s. Bare `txm` keeps the existing fuzzy picker and has no new refresh or
coordinator overhead. Without `--agents`, dashboard never contacts/starts an
agent coordinator. With it, M2’s existing subscription/start behavior applies;
quitting closes the subscription and leaves sessions and the service intact.

| Key | Action |
| --- | --- |
| Up/Down, j/k, Ctrl-P/Ctrl-N | Move selection |
| PageUp/PageDown, Home/End | Navigate longer lists |
| /, text, Enter | Edit/finish case-insensitive search (maximum 128 runes) |
| Escape | Clear search/query first; quit when no query is active |
| Tab | Cycle all/sessions/agents/needs_input/working/exited/unknown |
| b | Cycle local backend filters |
| Enter / o | Attach / attach read-only to a supported live session row |
| p | Toggle selected-session plain output preview |
| r | Request refresh |
| q / Ctrl-C | Quit / cancel with exit 130 |
| ? / F1 | Show key help |

Session and registration rows are separate observations. Names can be reused;
dashboard does not join an agent to a live session by name. Choose a session row
for attach/preview; choose an agent row for managed/registered lifecycle,
attention source/time, durable receipt and optional turn-completion details.
Client state and capabilities are displayed only where observed. Native launch
instances are authenticated; tmux session IDs are current-server identifiers,
not persistent unmanaged identities. Agent turn completion is not process exit.

Each backend refresh is independent and cancellable. Errors retain stale rows
for context and disable actions. A missing agent heartbeat becomes stale after
five seconds; resync restores fresh data. Output text never supplies agent state.
Previews default off, are limited to 16 KiB/100 visible-screen lines for the
selected native/tmux session, and contain safe plain text. Unsupported/error
messages are placeholders, not successful captures. Nothing is logged or recorded
by dashboard. Retained session observations are bounded; display truncation is
explicit. Extremely large terminals use at most 512 columns by 200 rows.

A Linux ancestry/TTY guard disables attach/preview of the dashboard’s own native
PTY to prevent feedback. Missing /proc context disables native actions; this is
a deny-only safety check, never process/agent status or persistent PID identity.
Non-Linux dashboard runtime stays unsupported pending platform guard/TTY tests.

The dashboard restores the terminal before attaching and resumes on detach or
failure. Native detach uses Ctrl+backslash. tmux uses the server’s normal key
bindings (default Ctrl-b then d); an authorized external `tmux detach-client`
also returns to dashboard. Read-only clients cannot inject input, and unsupported
Screen/Zellij actions remain disabled. Cancellation of external attach restores
terminal attributes and file flags even if its process cannot clean up itself.

### Human workflow examples

```bash
# Shell persistence; native has one terminal, with no tmux layout emulation.
txm --backend native create work
txm attach native/work
# Detach with Ctrl+backslash, then inspect without writing input:
txm attach --read-only native/work
txm dashboard --backend native --preview

# Existing SSH helper: print/review a snippet and use an already configured host.
txm generate-ssh-config
ssh -t YOUR_EXISTING_HOST 'txm attach native/work'
# This is existing SSH attach, not remote dashboard aggregation. No SSH is run by dashboard.

# Two supervised generic workers; no AI provider is required.
# Explicit shell argv is used here to demonstrate lifecycle and hook signals.
txm --backend native agent start --provider generic review-one -- /bin/sh -c \
  'txm agent signal --from-env --state working; sleep 3; txm agent signal --from-env --state needs-input; sleep 60'
txm --backend native agent start --provider generic review-two -- /bin/sh -c \
  'txm agent signal --from-env --state working; sleep 5; exit 42'
txm dashboard --agents
# Use the returned opaque IDs, not the names, in scripts:
txm agent status AGENT_ID --json
txm agent wait AGENT_ID --until needs-input --timeout 30s --json
txm events --jsonl --id AGENT_ID
txm agent wait SECOND_AGENT_ID --until exited --timeout 30s --json
# Observed exit is success even for child 42; opt into propagation explicitly:
txm agent wait SECOND_AGENT_ID --until exited --fail-on-nonzero --json
```

The children’s allowlisted backend environment must have the installed `txm` on
PATH (or use its absolute path in your worker). These examples never turn on
provider permission bypass or infer command success from terminal text. Review
[M2 provider limits](m2-readiness.md) before configuring a real coding agent.

A short [asciicast v2 recording](evidence/phase6-dashboard.cast) demonstrates
synthetic workers, needs-input, separate turn completion, exit 42 and read-only
inspection. It contains fixture labels/output and ephemeral IDs only; no user
project, prompt or credential was recorded. Play locally with
`asciinema play docs/evidence/phase6-dashboard.cast` if that player is installed.
See [M3 readiness](m3-readiness.md) for exact validation and remaining platform gates.

## Supervised Agents

The `agent` namespace opts into a private local coordinator. Ordinary session
commands and the existing picker do not start, contact or wait for it. Initial
managed execution supports Linux/macOS code paths with CGO and native or tmux
3.4+; real Phase 5 validation is Linux amd64 only. Screen/Zellij managed launch
and unverified local transports return unsupported. M1 cross-platform release
gates remain blocked. Signals/events/waits and explicit notifications are available
through the CLI; see [M2 readiness](m2-readiness.md) for provider limitations.

```bash
txm --backend native agent start --cwd /absolute/project build -- go test ./...
txm --backend tmux agent start --env EDITOR editor -- /usr/bin/vim README.md
txm agent list --json
txm agent status AGENT_ID --json
txm agent register tmux/existing-session
txm attach native/build
txm agent stop-coordinator
```

Start creates a new detached session, using exact structured executable argv.
Use `--` between the session name and executable; child flags after it are not
txm flags. There is no implicit shell or project-hook execution. Shell syntax
requires an explicit shell command such as `/bin/sh -c ...`. Names/prefixes follow
ordinary session rules; an existing session or live registration is a conflict.
Agent IDs are opaque; `status` requires the full ID. `--backend` filters agent
lists and must agree with a status target. Attach uses the ordinary session CLI.

The child receives only HOME, PATH, TERM and variable names explicitly added with
repeatable `--env NAME`. These values come from the wrapper's backend environment,
which may predate the current CLI (especially with tmux); this does not synchronize
environment values. Cwd is applied before executing the child. Argv stays in a
private one-use manifest until the wrapper consumes/unlinks it; registry and
receipts contain no argv, environment values or transcripts.

A successful start means a running child or a durable receipt was observed.
It does not mean the child exited successfully: inspect `receipt.outcome` and
`receipt.exit_code`, which can be 42 even when start/status exits 0. An observed
signal has a signal label and no exit code. Failed exec is `start_failed`, with
neither an invented code nor start time; start reports an error while preserving
the registration/receipt for inspection. Partial launch errors include the agent
ID; query it rather than automatically launching again. Cancelling a requesting
CLI does not guarantee a detached launch was undone.

`register` associates an existing exact session without supervising its process.
Its managed/launch IDs and receipt stay null; disappearance does not establish an
exit code. Managed IDs survive rename, while backend instance/ownership bindings
prevent association with a reused name. Lifecycle, attention, source and observation
time are separate. Attention stays unknown with no hook timestamp unless explicit
hooks provide a fresh authenticated observation.
Receipt sources use the actual finish time; live state requires a matching
supervisor handshake and current child observation. Backend session listings do
not implicitly join agent registry data.

The wrapper owns its final immutable exit receipt independently of the CLI and
coordinator. Coordinator restart reloads private registry data and reconciles
receipts/liveness; it never reruns a command. Stopping/crashing the coordinator
leaves sessions/children running. Killing the wrapper/server can prevent a receipt;
status then stays unknown. Wrapper termination requests SIGTERM to the separate
foreground child process group, followed by bounded cleanup of resistant group
members. Deliberately escaped process groups are not a containment guarantee.

State/runtime paths use private `txm/v1/agents` subdirectories: mode 0700 with
owned, single-link, mode-0600 manifests, receipts, registry and stable locks.
Unsafe/corrupt state is rejected rather than repaired or assigned a new identity.
Limits are 256 registrations, a 128 KiB/256-argument manifest, 128 environment
names, 16 short-request clients and finite 15-second requests. Reconciliation runs
during agent requests and active event subscriptions; it stops without subscribers.
Receipts remain available
after service shutdown. There is no automatic registry pruning in this phase.

Use `agent prune FULL_ID --dry-run`, then `agent prune FULL_ID --confirm FULL_ID`
to remove a completed supervised registration with a validated exit receipt.
For an unsupervised registration this removes metadata and leaves its session
running. Prune cleaned task records first with `task prune` (see
[extensions](extensions.md)); receipts, Git branches and launch files remain.
Pruned agent IDs cannot be replayed; the private retired-ID history is bounded
at 16,384. All-agent event subscribers receive a `registry_pruned` resync;
an event stream targeting the removed ID ends with not-found.

## Window Management (`txm window`)

### new
Create a new window with tmux. Screen/Zellij new-window/tab adapters remain
runtime-unverified; native window management is unsupported.
```bash
txm window new [session_name] [window_name]
```

### list
List windows in a session
```bash
txm window list [session_name]
```

### kill
Remove a window
```bash
txm window kill [session_name] [window_name]
```

### next
Switch to next window in session
```bash
txm window next [session_name]
```

### prev
Switch to previous window in session
```bash
txm window prev [session_name]
```

### rename
Rename a window
```bash
txm window rename [session_name] [old_window_name] [new_window_name]
```

### split
Split a window into panes
```bash
txm window split [session_name] [window_name] [v|h]
```
- `v`: vertical split
- `h`: horizontal split

## Pane Operations (`txm pane`)

Exact window mutations and pane operations are supported by tmux. Screen/Zellij
reject unverified exact targets before changing focus or invoking a mutation.

### list
List panes in a window
```bash
txm pane list [session_name] [window_name]
```

### kill
Remove a pane
```bash
txm pane kill [session_name] [window_name] [pane_number]
```

### exec
Send terminal input to an exact pane; success means accepted bytes, without a
command-completion or exit-status promise. tmux resolves IDs within the requested
session/window and sends text literally, then Enter. Duplicate window names or
name/index collisions require an explicit ID. Empty pane targets are accepted
only for a single-pane window.
**Alias:** `txm exec`
```bash
txm --backend tmux pane exec my-project @1 %2 'printf hello'
txm --backend native exec my-project 0 0 'printf hello'
```

## Environment Variables

### TXM_DEFAULT_BACKEND
Override the default backend temporarily:
```bash
export TXM_DEFAULT_BACKEND=native
# or
TXM_DEFAULT_BACKEND=tmux txm create my-session
```

### TXM_SESSION_PREFIX
Automatically prefixes all session names globally.
```bash
export TXM_SESSION_PREFIX=my_env_
```

### NO_COLOR
Disable colored output:
```bash
export NO_COLOR=1
```

### TERM
Terminal type for capability detection. Common values:
- xterm
- xterm-256color
- screen
- screen-256color
- tmux
- tmux-256color
- linux

## Seamless SSH Workflow

Using `txm` with SSH is a first-class citizen. Instead of using SSH to remote into a system with a single terminal and managing `n` `tmux` panes remotely, we can open `n` local terminal tabs and run `ssh` for all of them. This allows us to leverage our local terminal emulator's native tabs, splits, and scrollback, while perfectly preserving the sessions remotely.

To make this a delightful workflow, we can add an SSH config entry for our remote dev server. Generate the recommended setup block instantly by running:
```bash
txm generate-ssh-config
```

The output will look something like this. Add it to our `~/.ssh/config`:
```bash
Host d.*
    HostName 192.168.1.xxx
    
    # Automatically attach to a txm session on the remote server
    # named after the remote host we're connecting to (%h).
    RemoteCommand txm attach %h
    RequestTTY yes

    # Multiplex multiple PTY sessions to a single server over one connection
    ControlMaster auto
    ControlPath ~/.ssh/cm-%r@%h:%p
    ControlPersist 10m
```

Architecturally, SSH supports multiplexing multiple channels of communication within a single TCP connection to a server. `ControlMaster` is the setting that tells SSH to multiplex multiple PTY sessions over that one connection.

Now we can spawn as many terminal sessions as we'd like:
```bash
ssh d.term
ssh d.irc
ssh d.dotfiles
```

Because `txm attach` is an "upsert" command, the first `ssh` command will automatically create the session on the remote host, and subsequent commands to the same host will attach to it (or create entirely new sessions if connecting to different hosts). 

### Auto-Reconnection
We can use the `autossh` tool to make SSH connections auto-reconnect. If we close our laptop lid, it will automatically reconnect all our SSH connections when we reopen it:
```bash
autossh -M 0 -q d.term
```
*(Tip: create an alias/abbr for this, e.g., `alias ash="autossh -M 0 -q"`)*

Now we can set up our OS tiling windows how we like them for our project and have as many windows as we'd like, replicating exactly what `tmux` does but with native windows, tabs, splits, and scrollback!

## Backend-Specific Notes

### native
- **Zero dependencies**: No external multiplexer needed (statically compiled with `txm`)
- **State & Scrollback**: Powered by the cutting-edge **Ghostty** (`libghostty-vt`) terminal emulator core, maintaining a highly accurate VT state and configurable scrollback ring buffer.
- **Lightweight**: Optimized for simple persistent single-pane sessions
- **Graceful Detach**: Keybinding `Ctrl+\` to detach cleanly
- **Portability**: Available as a 100% statically linked `linux-musl` distribution for drop-in use on Alpine Linux and minimal containers without `glibc`.

### tmux
- **Full feature support**: Complete implementation of all window and pane operations
- **Session persistence**: Robust session management with server/client architecture
- **Configuration**: Extensive customization via `.tmux.conf`

### zellij
- **Modern workspace paradigm**: Tab-based workflow with floating panes
- **Built-in layouts**: Predefined and custom layouts
- **Session management**: Contemporary approach to terminal multiplexing

### GNU Screen
- **Legacy compatibility**: Reliable fallback for older systems
- **Basic session management**: Core session operations

## Attention, events and waits

```bash
# The child can call txm agent signal --from-env --state working|needs-input|idle.
txm agent start --provider generic build -- /absolute/path/to/your-worker
# Or associate an existing registration and inspect its public hook generation.
txm agent hooks enable AGENT_ID --provider generic --json
txm agent signal AGENT_ID --instance INSTANCE_ID --generation GENERATION \
  --sequence 1 --time 2026-10-30T12:00:00Z --state needs-input

txm agent wait AGENT_ID --until needs-input --timeout 30s --json
txm agent wait AGENT_ID --until exited --timeout 10m --json
txm agent wait AGENT_ID --until exited --fail-on-nonzero --timeout 10m --json
txm events --jsonl --id AGENT_ID
```

Use the actual current UTC timestamp and an increasing sequence. Only explicit
`--provider` launches receive hook context; default managed commands receive none.
Credentials are mode-0600 local files; token values never go in argv/environment,
status or events. Same-user processes are the trust boundary. Rotation through
`hooks enable` invalidates the old generation; regenerate provider settings and
update any old child hook context explicitly. Attention expires after two minutes;
missing/expired hooks mean unknown. Generic `turn-complete` is a separate signal;
starting new work clears its timestamp. It never establishes process exit.

Wait defaults to 30 seconds (maximum 24 hours). Exit 0 means the condition was
observed, including a nonzero child exit. `--fail-on-nonzero` changes only exit
waiting: unsuccessful receipts return 1. Timeout returns 6, cancellation 130,
unsupported waits 4, and unproven process loss 1. Attention waits need configured
hooks; unsupervised registrations cannot provide exit receipts. A JSON wait
retains the last observed Agent even on failure.

Events begin with `resync_required` carrying a current snapshot. Epoch/sequence
identify ordering; filtering creates gaps. Overflow and reconnect require another
snapshot. No durable replay or exactly-once delivery is promised. Events contain
safe registration/state metadata, never terminal snapshots or provider input.
Names are user-controlled; JSON consumers must render them safely. Optional
`events --timeout D` bounds a stream; cancellation closes its subscription.

```bash
# Explicit Claude opt-in; user/project settings are not edited.
txm agent start --provider claude assistant -- claude
# Codex configuration is printed for manual review and provider trust approval.
txm agent hooks enable AGENT_ID --provider codex
txm agent hooks config AGENT_ID > /path/you/choose/hooks.json
txm agent providers --json
# Optional action, direct argv; payload arrives on stdin. No action runs by default.
txm agent notify -- /absolute/path/to/your-notification-script
```

Provider adapters accept only the exact verified versions in
[M2 readiness](m2-readiness.md). Codex hook trust must be reviewed by the user;
automatic activation/trust bypass is unsupported. OpenCode v2 is unsupported.
Claude/Codex bridge events map SessionStart to unknown, UserPromptSubmit to
working, PermissionRequest to needs_input, and a nonrecursive Stop to idle. These
are provider observations; a Stop does not prove final turn completion. Bridge
stdout is `{}`; rejected input emits a fixed warning and leaves provider decisions
unchanged. No transcript file is opened. Claude has real initialization-hook
coverage; full model/permission conversations and Codex runtime remain unverified.

Notification actions receive only schema version, opaque agent/managed IDs, kind
and time. The local queue is bounded, actions have a two-second deadline and a
five-second per-agent throttle, and snapshot/resync never executes actions.
Notifications are best effort and disabled unless `agent notify` is running.

## Troubleshooting

### Backend Selection Issues

1. **Check available backends**:
```bash
which tmux zellij screen
```

2. **Test backend functionality**:
```bash
# Test each backend individually
TXM_DEFAULT_BACKEND=native txm -v list
TXM_DEFAULT_BACKEND=tmux txm -v list
TXM_DEFAULT_BACKEND=zellij txm -v list
```

3. **Reset configuration**:
```bash
rm -rf ~/.txm
txm config show  # Will recreate with defaults
```

## Native protocol upgrade and validation

The new native runtime uses framed v1 IPC in an owned, mode-0700 directory under
`$XDG_RUNTIME_DIR/txm/v1`, or a private per-user directory under the OS temp dir.
An invalid explicit runtime directory is an error. Socket paths must fit the OS
limit; use a shorter absolute private XDG runtime directory if needed. No public
network listener or agent coordinator is required.

Keep a copy of the old txm executable before upgrading while legacy sessions are
running. Their `txm-NAME.sock` files remain in the old TMPDIR. Use the old binary
with the same user/TMPDIR/config (for example `TXM_DEFAULT_BACKEND=native
/path/to/old-txm attach NAME`) to finish them. New txm reports incompatibility;
it never upgrades, unlinks or kills those sessions automatically.

Native has one terminal per session. `txm exec NAME 0 0 "command"` sends literal
terminal input plus newline and confirms input acceptance, without reporting
command completion or child exit code. Native windows/panes/rename remain
unsupported. Hidden dump is a bounded active-screen snapshot, not history.

Native requires Linux/macOS and CGO; Windows persistence is unsupported. Phase 3
has local Linux amd64 evidence, with macOS runtime validation still pending.
See [testing evidence](testing.md) before making platform/release claims. Failed
commands now return nonzero. Human failures use stderr; JSON reads include errors
in their envelope. Classified exits and public JSON/capability commands are
implemented, including strict profile preflight and exact owned-resource apply.
The M1 release gate remains separate from implemented Linux behavior; consult
the [readiness record](m1-readiness.md) for missing platform/archive verification.

With --agents, stream heartbeats are connection freshness only. Dashboard also
requests existing typed agent list snapshots at most once per two seconds,
with a two-second deadline, so unchanged running-agent observation timestamps
are refreshed without inventing proof. Running observations older than five
seconds become stale/unknown even when heartbeats continue. Completed immutable
receipts retain their original finish timestamp. No new M2 wire schema or
background agent polling is added to ordinary dashboard/picker use.

Backend scope applies to visible session/registration rows and direct M1 reads.
The opted-in M2 coordinator is a per-host service: its existing global subscription
may reconcile other registered backends, even when this dashboard displays one.
It never selects an alternative backend for a requested session action.

## Optional Git tasks and SSH aggregation

Phase 7 adds opt-in task and remote namespaces. See the [extension guide](extensions.md)
for dry-run, worktree ownership and recovery, explicit SSH host pinning, bounded
partial results, and exact remote attachment. Defaults remain independent of both
extensions. Read [extension readiness](extensions-readiness.md) before making
platform or network SSH support claims.
