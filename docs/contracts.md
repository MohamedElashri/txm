# CLI, state, and native protocol contracts

[Documentation](README.md) / Contracts

Reference for scripts and integrations. See the [user guide](user-guide.md) for
recipes and [platform scope](user-guide.md#platforms-and-backends) for availability.

- [Targets](#backend-selection-and-exact-targets) and [JSON/exits](#json-and-exits)
- [Output/profiles](#output-and-profiles) and [identity/events](#identity-supervision-and-events)
- [MCP](#scoped-mcp-over-stdio) and [metrics](#process-metrics)
- [Dashboard/tasks/SSH](#dashboard-tasks-and-ssh) and [native IPC](#native-protocol-v1)

## Backend selection and exact targets

`--backend native|tmux|zellij|screen` is strict. Configuration and
`TXM_DEFAULT_BACKEND` are preferences with fallback. Targets use a selected-backend
name or `backend/name`; conflicting selectors fail. Aggregate reads require
`--all-backends` and keep same-named sessions separate.

Mutations require exact identities; names, indexes, focus and remembered PIDs
are insufficient. `nuke` affects only the selected backend. Ordinary attach may
create a session after discovery proves it absent; read-only attach never does.
Native/tmux support read-only attach. Screen/Zellij reject unsupported observer,
exact pane/layout and output operations before mutation.

## JSON and exits

Supported JSON commands emit one UTF-8 object plus newline, including expected
failures. Diagnostics use stderr. All four envelope fields are required:

```json
{"schema_version":1,"data":{"sessions":[]},"errors":[],"partial":false}
```

Empty discovery succeeds. `partial` means some requested scope succeeded and
some failed; total failure has null data. Accept additive schema-1 fields and
handle unknown states conservatively. IDs are opaque, optional values nullable,
and timestamps UTC RFC3339.

| Exit | Meaning |
| --- | --- |
| 0 | Success, including empty discovery |
| 1 | Operation, protocol, internal or configuration failure |
| 2 | Invalid arguments, target or profile |
| 3 | Not found |
| 4 | Unsupported capability, version or schema |
| 5 | Conflict or ambiguous identity |
| 6 | Timeout |
| 7 | Unavailable backend or partial failure |
| 8 | Permission, role or peer denial |
| 130 | Cancellation |

Precedence: cancellation, partial failure, invalid input, denied, conflict,
timeout, unavailable, unsupported, not-found, general failure. Errors retain
backend scope. Field definitions live in `pkg/core`, `pkg/agent` and golden tests.

## Output and profiles

Output distinguishes visible `screen_snapshot` from requested tmux history.
Native has no history API. Plain output strips terminal controls before
UTF-8-safe limits; explicit raw output uses base64 in JSON. Truncation is
reported. Accepted terminal input never proves execution or child exit.

Profiles use strict TOML: at most 1 MiB, 32 windows and 128 panes. Preflight checks
names, cwd, topology and capabilities. Dry-run creates no runtime/identity state.
Native supports one terminal; tmux 3.4+ supports exact layouts and pane cwd.
Existing sessions require `--attach-existing`. Rollback affects only proven
owned resources; uncertainty retains resources and returns partial failure.

## Identity, supervision and events

Private directories are owned mode 0700; state, locks and receipts are 0600.
Unsafe ownership, symlinks, hard links and corrupt state are refused. Host identity
is random and atomically published. Preserve returned opaque IDs.

| Identity | Scope |
| --- | --- |
| `host_id` | Persisted private local-state identity |
| `locator` | Host, backend and observed session name |
| `backend_id` | Backend session; unmanaged tmux IDs are server-scoped |
| `instance_id` | One native/supervised launch, independent of a reusable name |
| `managed_id` | Supervised work, independent of terminal text |
| Agent `id` | Registration; unsupervised entries may lack a managed ID |

The optional coordinator owns registrations; ordinary sessions/picker do not
contact it. A supervised wrapper executes structured argv, owns/reaps its child
and publishes an immutable receipt. Missing receipts mean unknown. Registration
cannot supervise retroactively. Coordinator restart neither relaunches work nor
kills sessions.

Lifecycle, attention and turn completion are separate sourced facts. Hooks bind
registration, instance and generation; sequence/time checks reject replay.
Attention expires. Provider observations additionally bind session/thread and
optional turn identity; stale, foreign or out-of-order events cannot replace
newer observations. Resume/fork does not silently rebind an instance.

Terminal text never establishes success or attention. Hook bridges never approve,
deny or continue provider operations. Registry/events exclude prompts,
transcripts, tool contents and credentials. Discovery reports availability,
exact schema compatibility, validation, trust and capabilities separately;
legacy `supported` means compatibility only. Unknown versions refuse hooks;
generic supervision remains available. A live reference is not per-instance
attestation.

Strict v1 registry, receipt and native IPC formats remain unchanged. New
observations and Claude/Codex associations use separate sidecars, including
schema-2 association/credential files. Legacy credential rotation revokes the
new credential. Older coordinators reject new operations such as metrics
creation before launching work.

Events are ephemeral, with atomic initial snapshots and resync after gaps,
overflow or restart; no replay or exactly-once promise. Wait success means the
condition was observed; `--fail-on-nonzero` propagates an unsuccessful child
receipt. Notifications are explicit, bounded and off by default.

`agent list/status --existing` require explicit backend and exact managed IDs.
These scoped reads create no paths, locks, identity, services or workers and
never repair/rewrite state. A bounded read-only query subprocess is not a managed
worker. Ordinary reconciliation queries do not have this pure-read guarantee.

## Scoped MCP over stdio

Opt-in newline-delimited JSON-RPC uses the pinned Go SDK, not native IPC framing.
No network listener is opened. Stdout is protocol-only; diagnostics use stderr.
`mcp serve --json` is invalid.

| Protocol era | Lifecycle |
| --- | --- |
| `2025-11-25` | `initialize`, then `notifications/initialized` |
| `2026-07-28` | Namespaced per-request `_meta`, optional `server/discover`; no legacy handshake |

Do not mix eras. Unsupported versions are refused. Limits: 1 MiB messages,
16 concurrent requests, four waits, 60 seconds per wait, 8192 unique request IDs
per process. Reconnect before exhausting IDs. Lists are paginated; cursors expire
on inventory changes/restart.

Read scope defaults to deny. Backend, exact IDs and/or canonical owned repository
selectors intersect; rejected identities/paths are not probed. Reads never
create/repair state or start services, samplers, providers or jobs. Session tools
expose managed metadata, with backend liveness unverified. No unmanaged discovery,
remote reads or terminal preview. Missing proof remains unknown.

Creation needs a separately reviewed owned regular mode-0600 policy, at most
64 KiB. Example schema:

```json
{
  "schema_version": 1,
  "namespace": "11111111111111111111111111111111",
  "templates": [{
    "id": "worker", "kind": "agent", "name": "reviewed-worker",
    "backend": "native", "directory": "/owned/canonical/repo",
    "argv": ["/usr/bin/true"]
  }]
}
```

Replace the path and namespace. At most 32 templates bind fixed argv, canonical
cwd identity and executable digest. Task templates use `"kind":"task"` plus a
full frozen `base_commit`; repository commands stay disabled. The child
environment is empty and bindings are revalidated before launch. Interpreter
allowlisting grants its exact code; same-UID code remains trusted.

`start_agent`/`start_task` accept only `operation_id` (32 lowercase hex) and
`template_id`. The namespace journal permanently binds an operation ID to request
hash and reserved resource ID. Identical retries return the recorded accepted
or uncertain result; conflicting reuse fails. Lost responses never launch
another job. Limits: 1024 entries/1 MiB; capacity refuses creation. Deleting or
replacing the namespace cannot preserve retry safety.

Stop, cleanup and merge are human CLI handoffs, never executable MCP tools.
Tool annotations describe behavior and grant no authority.

## Process metrics

Opt-in sampling covers only the supervisor's unreaped direct child (`child_pid`).
Samples validate kernel start and parent identity. CPU uses 100% per logical CPU
and may exceed 100%; RSS is bytes. Darwin converts Mach ticks with the kernel
timebase. First CPU is null; missing/failed observations are unavailable, not zero.

| Bound | Limit |
| --- | --- |
| Interval / active samplers | 2–60 seconds / 32 |
| Storage including publication headroom | 1 MiB per instance / 32 MiB overall |
| Retained history | 512 KiB or 2048 samples per instance / 30 MiB overall |
| Maximum age | 24 hours, subject to tighter quotas |

Eviction selects oldest inactive history with deterministic identity tie-breaks;
active histories are protected. Capacity/observer failure leaves the job running.
Sampling survives coordinator restart, but supervisor failure does not trigger
PID-based recovery. Disabled jobs allocate no metrics files, leases or timers.
Reads never start or repair sampling.

## Dashboard, tasks and SSH

Dashboard keeps session and registration rows separate, disables stale/unsupported
actions and bounds opt-in preview. Ancestry/TTY checks prevent feedback into its
own native terminal. Attach/exit restore terminal attributes and file flags.

Tasks bind intent and filesystem identity; receipts establish lifecycle.
Cleanup preserves dirty/untracked/ignored files unless explicitly forced with
full-ID confirmation. Force never bypasses ownership/exit proof. Pruning retains
branches and receipts; up to 16,384 retired IDs remain reserved against replay.
Recovery never guesses by name.

Review/merge dry-run are bounded existing-state reads with explicit target refs.
Opt-in diff is control-stripped and capped at 16 KiB; secrets are not redacted.
Setup requires a mode-0600 policy binding argv, environment names, task/worktree,
executable/config identities and the accepted dry-run digest. Failure/timeout
retains uncertain state and blocks main launch.

Merge applies `git merge --ff-only` only to a distinct clean checked-out target.
It binds reviewed OIDs/configuration/worktree identity and requires successful
task/setup receipts, registered-worker exit proof and explicit human quiescence
of external writers. Ref-only targets, sparse worktrees, submodules and
non-fast-forwards are refused. Durable intent precedes mutation; replay returns
the recorded outcome or uncertainty, never retry/rollback, push or cleanup.

SSH requires alias allowlisting and a verified host pin plus OpenSSH credentials
and host-key checks. Encoded requests bind protocol, host and exact session/
instance. Results are bounded, cancellable and possibly partial, with explicit
completeness fields. Attach restores TTY/file flags; tmux IDs stay server-scoped.
No public txm listener is installed.

## Native protocol v1

One server owns one PTY/VT, child process group, listener and bounded client
queues. Ordinary sessions need no coordinator. Native requires Linux/macOS CGO;
Windows persistence is unsupported.

### Discovery and ownership

Use private `$XDG_RUNTIME_DIR/txm/v1`, or a canonical per-user temporary directory
when unset. Invalid explicit paths fail. Require owner UID, 0700 directories,
0600 files/sockets, no-follow access and socket path limits.

Sockets are `s-INSTANCE.sock` with 128-bit random lowercase-hex instance IDs.
`n-SHA256(name).lock` serializes names. Readiness requires owned lock/listener,
child startup and a matching instance handshake; a pathname alone is insufficient.
Stale/incompatible entries are retained. Legacy `txm-NAME.sock` is never contacted
or removed; finish those sessions with the old binary and original environment.
Native nuke refuses incompatible legacy candidates.

### Frames and roles

Public JSON and native framing versions are independent. All header integers
use network byte order; the header is 20 bytes:

| Offset | Bytes | Field |
| --- | --- | --- |
| 0 | 4 | `TXM1` |
| 4 | 2 | Version 1 |
| 6 | 1 | Message type |
| 7 | 1 | Flags, zero |
| 8 | 8 | Request ID |
| 16 | 4 | Payload length, at most 1 MiB |

Types 1–10: HELLO, HELLO_OK, REQUEST, RESPONSE, DATA, RESIZE, CLOSE, ERROR, PING,
PONG. Read full frames and validate before allocation. Unknown/duplicate fields,
malformed/trailing JSON, invalid UTF-8 and frame ordering are refused. JSON bytes
use base64. Implementation: `pkg/nativeprotocol`; adversarial fixtures: `tests/wire.py`.

Authenticate peer UID before accepting HELLO role. HELLO_OK binds protocol,
instance, backend, name, role, process state, capabilities and observation time.
Observers may status/snapshot/attach/ping/close; input/resize/kill check control
on every request. Same UID remains the trust boundary.

One request is outstanding per connection. Attach transfers the initial snapshot
and subsequent DATA without gaps/duplicates. Queue overflow disconnects the slow
client. Input acknowledgement proves PTY acceptance only. CLI kill waits for the
exact instance to disappear after bounded shutdown.

### Bounds and shutdown

- Connections: 32. Handshake/request/startup: five seconds. Socket writes: two
  seconds. Interactive attach has no overall lifetime deadline.
- Output queues: 256 KiB or 64 frames. Input: 16 items, each at most 16 KiB,
  expiring after five seconds. PTY draining never waits on a client writer.
- Resize: 1–4096 per axis, at most 1,048,576 cells. Failed PTY resize rolls back
  VT geometry; observers cannot resize.
- Idle attached peers receive PING after 30 seconds and must reply within five.
- Shutdown retains child identity through group signaling, waits two seconds
  after SIGTERM, then SIGKILL/reaps where supported and joins workers.

Detach/cancellation restore terminal attributes and file flags without ending
sessions. Detached/observer terminal queries use libghostty's callback; attached
control terminals retain raw query/reply. Raw streams are unsanitized; plain
capture follows [Output and profiles](#output-and-profiles).
