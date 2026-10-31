# CLI, state, and native protocol contracts

[Documentation](README.md) / Contracts

This is the reference for scripts and integrations. For command recipes and
interactive use, read the [user guide](user-guide.md).
The guarantees below apply to supported operations. Check installed capabilities
and the [platform scope](user-guide.md#platforms-and-backends) before assuming an
operation is available.

- [Backend selection and exact targets](#backend-selection-and-exact-targets)
- [JSON and exits](#json-and-exits)
- [Output and profiles](#output-and-profiles)
- [Identity, supervision and events](#identity-supervision-and-events)
- [Scoped MCP over stdio](#scoped-mcp-over-stdio)
- [Process metrics](#process-metrics)
- [Dashboard, tasks and SSH](#dashboard-tasks-and-ssh)
- [Native protocol v1](#native-protocol-v1)

## Backend selection and exact targets

`--backend native|tmux|zellij|screen` is strict: unavailable means failure.
Configuration and `TXM_DEFAULT_BACKEND` are preferences with fallback. Targets
accept a selected-backend name or `backend/name`; conflicting selectors fail.
Aggregate reads require `--all-backends` and never merge same-named sessions.
Mutations resolve exact identities; names, indexes, focus and remembered PIDs do
not establish ownership. `nuke` affects only the selected backend.

Ordinary attach may create a missing session only after successful discovery
proves absence. Read-only attach never creates a session. Native enforces an
observer role; tmux uses read-only attach. Screen/Zellij reject unsupported
read-only, exact pane/layout and output operations before mutation.

## JSON and exits

Supported JSON commands emit one UTF-8 object and newline, including expected
failures. Diagnostics go to stderr. The envelope is:

```json
{"schema_version":1,"data":{"sessions":[]},"errors":[],"partial":false}
```

All four fields are required. Empty discovery is success. `partial` means some
requested scope succeeded and some failed; total failure has null data. Consumers
accept additive fields within schema 1 and handle unknown states conservatively.
IDs are opaque, optional values are null, and timestamps are UTC RFC3339.

| Exit | Meaning |
| --- | --- |
| 0 | Successful operation/observation, including empty discovery |
| 1 | Operation, protocol, internal or configuration failure |
| 2 | Invalid arguments, targets or profile |
| 3 | Not found |
| 4 | Unsupported capability, version or schema |
| 5 | Conflict or ambiguous identity |
| 6 | Timeout |
| 7 | Unavailable backend or partial failure |
| 8 | Permission/role/peer denial |
| 130 | Cancellation |

Cancellation takes precedence. Other aggregate errors use partial failure first,
then invalid input, denied, conflict, timeout, unavailable, unsupported,
not-found and general failure. Errors remain scoped to the relevant backend.
Authoritative field types are in `pkg/core`, `pkg/agent` and their golden tests.

## Output and profiles

Output distinguishes visible `screen_snapshot` from explicitly requested history.
Native has no history API; tmux history uses a bounded range. Plain output strips
terminal controls before UTF-8-safe limits. Raw output requires explicit opt-in
and uses base64 in JSON. Truncation is disclosed. Input acceptance never proves
execution, completion or child exit.

Profiles use strict TOML, bounded to 1 MiB, 32 windows and 128 panes. Preflight
checks names, cwd, topology and backend capabilities before creation. Dry-run
creates no runtime/identity state. Native supports one terminal; tmux 3.4+
supports exact layouts and pane cwd. Existing sessions require explicit
`--attach-existing`. Rollback affects only proven owned resources; uncertainty
retains resources and returns partial failure.

## Identity, supervision and events

Private directories are owned and mode 0700; state/locks/receipts are 0600.
Reject unsafe ownership, symlinks, hard links and corrupt state rather than
repairing foreign paths. Host identity is random and atomically published.
Names and PIDs are observations; managed and launch instance IDs are distinct.

| Identity | Scope |
| --- | --- |
| `host_id` | Persisted random identity of the private local state |
| `locator` | Host, backend, and observed session name |
| `backend_id` | Backend session identity; unmanaged tmux IDs remain server-scoped |
| `instance_id` | One native or supervised launch, distinct from a reusable name |
| `managed_id` | Supervised work identity, independent of terminal text |
| Agent `id` | Registration identity; unsupervised registrations need not have a managed ID |

Preserve returned opaque IDs rather than deriving them from names or PIDs.

The optional coordinator owns the registry. Ordinary sessions and the picker do
not start or contact it. A supervised wrapper executes structured argv, owns and
reaps its child, and publishes an immutable exit receipt. Missing receipts mean
unknown, never exit 0. Registration cannot retroactively supervise a process.
Coordinator restart does not relaunch work or kill existing sessions.

Lifecycle, attention and turn completion are separate sourced observations.
Authenticated hook credentials bind registration, instance and generation;
sequence/time checks prevent replay. Attention expires. Terminal text is never
used to infer success or attention. Provider payloads exclude transcripts,
prompts, tool contents and credentials from registry/events.

Provider observations additionally bind provider session/thread and optional
turn identity. Process exit, a completed turn, a permission request and confirmed
needs-input are independent facts. Foreign sessions, stale generations and
out-of-order events cannot replace fresher observations; coordinator restart
does not refresh expired attention. Resumed/forked sessions are not silently
rebound to another managed instance. Hook bridges emit neutral output and never
approve, deny or continue provider operations.

Provider discovery reports availability, schema compatibility, validation level,
enablement/trust and event capabilities separately. The legacy `supported` field
means exact schema compatibility only. A live-reference record is not attestation
of an arbitrary running instance. Unknown versions fail closed for hook
activation, leaving generic process supervision available.

The strict v1 registry/receipt/native IPC contracts remain unchanged. New
canonical observations use private sidecars. Current Claude/Codex associations
use separate schema-2 association/credential sidecars rather than values an old
strict v1 reader cannot decode. Tagged v2.0/current coordinator fixtures verify
process-state compatibility and observation restoration; legacy credential
rotation explicitly revokes the newer credential. Older coordinators refuse
new operation kinds such as metrics creation before launching the job.

Events are bounded ephemeral notifications, with atomic initial snapshots and
resync on gaps, overflow or restart. They promise neither replay nor exactly-once
delivery. Wait consumes these observations: exit 0 means the requested condition
was observed; use `--fail-on-nonzero` to propagate an unsuccessful child receipt.
Notifications are explicit, bounded and off by default.

`agent list/status --existing` require an explicit backend and exact managed IDs
and use the same scoped existing-state reads as MCP. They do not create paths,
lock files, host identity, services or workers, repair unsafe state, or rewrite
existing metadata. A bounded read-only query subprocess is distinct from a
managed worker. Ordinary agent reconciliation is not a pure-read guarantee.

## Scoped MCP over stdio

MCP is opt-in local stdio, using newline-delimited JSON-RPC through the pinned
Go SDK. It does not use native IPC framing and opens no HTTP/TCP listener.
Stdout contains protocol messages only; diagnostics use stderr. `--json` is
invalid for `mcp serve`.

| Protocol era | Lifecycle |
| --- | --- |
| `2025-11-25` | `initialize`, then `notifications/initialized` |
| `2026-07-28` | Namespaced per-request `_meta`, optional `server/discover`; no legacy initialization handshake |

Do not mix the two lifecycle paths. Unsupported protocol versions are refused.
Messages are limited to 1 MiB, with 16 concurrent requests, four waits and a
60-second maximum wait. At most 8192 unique request IDs are accepted per process;
reconnect before exhausting the budget. Lists are bounded/paginated; cursors
expire on inventory changes or server restart.

Read scopes default to deny and bind local backend plus exact managed IDs and/or
canonical owned repository roots. Supplied selectors intersect. Disallowed
identities/paths are rejected before probing them. Reads never create/repair
state or start a coordinator, sampler, provider or job. Missing proof remains
unknown. Session tools expose managed registration metadata, not live backend
attachment proof, unmanaged discovery or terminal text. Remote reads and output
preview are absent.

Creation is independently disabled unless an operator loads an owned regular
mode-0600 policy file of at most 64 KiB. The schema is:

```json
{"schema_version":1,"namespace":"11111111111111111111111111111111","templates":[{"id":"worker","kind":"agent","name":"reviewed-worker","backend":"native","directory":"/owned/canonical/repo","argv":["/usr/bin/true"]}]}
```

Replace the example path/namespace with reviewed values. Up to 32 templates bind
fixed argv, canonical directory identity and a verified executable digest. Task
templates use `"kind":"task"` and additionally require `base_commit` as a full
frozen commit OID; repository commands remain disabled. The child environment
is empty. Revalidation precedes creation. A shell/interpreter template grants
its exact code; executable allowlisting alone cannot constrain arbitrary code.
Same-UID code remains the trust boundary.

`start_agent`/`start_task` accept only `operation_id` (32 lowercase hex) and
`template_id`. A permanent namespace journal binds the operation ID to an
immutable request hash and reserved resource ID. Identical retries return the
recorded accepted/uncertain outcome; conflicting reuse fails. Neither a dropped
response nor an uncertain outcome starts another job. The journal retains up
to 1024 entries within 1 MiB, refuses new requests at capacity and has no automatic
expiry/eviction. Namespace deletion or replacement is not a safe retry mechanism.

`stop_agent`, `cleanup_task` and `merge_task` are never executable MCP tools in
this release. Capability results provide human CLI handoffs only. Tool
annotations describe behavior and do not grant authority.

## Process metrics

Sampling is opt-in and local to a supervisor's unreaped direct child. Every
sample validates kernel process start identity and parent identity; scope is
`child_pid`, never inferred descendant aggregation. CPU normalizes 100% to one
logical CPU and permits values above 100%; RSS is bytes. Darwin CPU ticks are
converted using the kernel Mach timebase. The first CPU value is null, and
missing/failed observations are unavailable rather than zero.

Intervals are 2–60 seconds. There are at most 32 active samplers, 1 MiB per
instance and 32 MiB aggregate, including publication headroom. Actual retained
history limits are 512 KiB/2048 samples per instance and 30 MiB aggregate. The
tighter quota wins over the maximum 24-hour age. Eviction is oldest inactive
history first with deterministic identity tie-breaks; another sampler cannot
evict an active history. Capacity/observer failure leaves the job running.

The supervisor owns sampling across coordinator restart. Supervisor failure does
not trigger PID-based sampler recovery. Disabled jobs allocate no metrics state,
leases or timers. Explicit reads consume existing private histories without
starting or repairing anything.

## Dashboard, tasks and SSH

Dashboard uses the same typed state APIs. Session and registration rows remain
separate; stale/unsupported actions are disabled. Preview is bounded and opt-in.
Linux/macOS ancestry/TTY checks deny feedback into the dashboard's own native
terminal. Exit and attachment restore the terminal and file flags.

Tasks record intent and filesystem identity, while agent receipts establish
lifecycle. Cleanup preserves dirty/untracked/ignored work unless explicitly
forced with full-ID confirmation; force cannot bypass ownership or exit proof.
Pruning removes eligible metadata, retaining branches and receipts. Retired
managed IDs cannot be replayed. Recovery never guesses associations from names.

Task review/merge dry-run are bounded existing-state reads with explicit local
target refs. Diff content is opt-in, control-stripped and limited to 16 KiB;
secrets are not redacted. Setup execution requires a mode-0600 policy binding
exact argv, environment names, task/worktree, executable digests and configuration
provenance, plus acceptance of the exact reviewed dry-run digest. Failure/timeout
retains uncertain setup state and prevents main launch.

Merge apply supports only a distinct clean checked-out target worktree and
`git merge --ff-only`. It binds reviewed OIDs/configuration/worktree identity,
requires successful task/setup receipts and registered-worker exit proof, and
requires the human's explicit external-writer quiescence assertion. Ref-only
updates, sparse worktrees, submodules and non-fast-forward operations are refused.
A durable uncertain intent precedes mutation. Replays return recorded outcomes
or uncertainty, never retry/rollback, push, delete branches or clean up.

SSH requires an explicit alias allowlist and verified host pin, in addition to
OpenSSH authentication/host-key checks. Encoded requests bind protocol, host and
exact session/instance identity. Results are bounded, cancellable and may be
partial. Remote envelopes require explicit completeness fields. Attach restores
TTY/file flags; tmux IDs remain server-scoped. No public txm listener is added.

## Native protocol v1

The native server owns one PTY/VT, child process group, listener and bounded client
queues. Ordinary sessions do not need an agent coordinator. Linux/macOS CGO builds
support native; Windows persistence is unsupported.

### Discovery and ownership

Use private `$XDG_RUNTIME_DIR/txm/v1`, or a canonical per-user temporary directory
when unset. Invalid explicit paths fail. Enforce owner UID, mode 0700 directories,
0600 files/sockets, no-follow access and socket path limits.

Socket names are `s-INSTANCE.sock`; instance IDs are 128-bit random lowercase hex.
Stable `n-SHA256(name).lock` files serialize names. Readiness requires the owned
lock, listener, successful child startup and matching instance handshake. A socket
pathname alone is never proof. Stale/incompatible entries are not blindly removed.

Legacy `txm-NAME.sock` paths are never contacted, unlinked or taken over. Finish
those sessions using the old binary with the same user/TMPDIR/config. Native
nuke refuses while incompatible legacy candidates are present.

Framing and public JSON schema versions are independent.

### Frames and roles

All integers use network byte order. The fixed header is 20 bytes:

| Offset | Bytes | Field |
| --- | --- | --- |
| 0 | 4 | `TXM1` |
| 4 | 2 | Version 1 |
| 6 | 1 | Message type |
| 7 | 1 | Flags, zero |
| 8 | 8 | Request ID |
| 16 | 4 | Payload length, at most 1 MiB |

Types 1–10 are HELLO, HELLO_OK, REQUEST, RESPONSE, DATA, RESIZE, CLOSE, ERROR,
PING and PONG. Read complete frames and validate before allocation. Reject unknown
fields, duplicate keys, malformed/trailing JSON, invalid UTF-8 and invalid frame
ordering. JSON byte fields use base64. Framing is implemented in
`pkg/nativeprotocol`; raw/adversarial fixtures are in `tests/wire.py`.

Authenticate peer UID before HELLO role acceptance. HELLO_OK binds protocol,
instance, backend, name, role, process state, capabilities and observation time.
Observers may status/snapshot/attach/ping/close; input/resize/kill require control
on every request. Same UID remains the trust boundary.

Requests support status, snapshot, input, attach and kill. Only one request is
outstanding per connection. Attach hands off an initial snapshot and subsequent
DATA without a gap or duplicate. Queue overflow disconnects the slow client.
Input acknowledgement proves PTY acceptance only. Kill acknowledgement is followed
by bounded shutdown; CLI waits for the exact instance to disappear.

### Bounds and shutdown

- At most 32 connections; handshake/request/startup deadlines five seconds,
  socket writes two seconds. Interactive attach has no overall lifetime deadline.
- Attached output queues: 256 KiB or 64 frames. Input: 16 queued items of at most
  16 KiB, with five-second expiry. PTY draining never waits on a client writer.
- Resize: 1–4096 per axis and at most 1,048,576 cells. Failed PTY resize rolls back
  VT geometry; observers cannot resize.
- Idle attached peers receive PING after 30 seconds and must reply within five.
- Shutdown retains child identity through final group signaling, waits two seconds
  after SIGTERM, then SIGKILL/reaps where supported and joins PTY/client workers.

Detach/cancellation restores terminal attributes and file flags while preserving
persistent sessions. Detached/observer-only terminal queries use libghostty's
existing callback; attached control terminals retain raw query/reply behavior.
Raw streams are not sanitized; public plain capture follows
[Output and profiles](#output-and-profiles).
