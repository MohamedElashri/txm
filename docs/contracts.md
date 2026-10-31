# CLI and state contracts

[Documentation](README.md) / CLI and state contracts

This is the reference for scripts and integrations. For command recipes, read
[Automation](automation.md); for interactive use, read the [user guide](user-guide.md).
The guarantees below apply to supported operations. Check installed capabilities
and the [platform matrix](releasing.md#platforms-and-backends) before assuming an
operation is available.

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

Events are bounded ephemeral notifications, with atomic initial snapshots and
resync on gaps, overflow or restart. They promise neither replay nor exactly-once
delivery. Wait consumes these observations: exit 0 means the requested condition
was observed; use `--fail-on-nonzero` to propagate an unsuccessful child receipt.
Notifications are explicit, bounded and off by default.

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

SSH requires an explicit alias allowlist and verified host pin, in addition to
OpenSSH authentication/host-key checks. Encoded requests bind protocol, host and
exact session/instance identity. Results are bounded, cancellable and may be
partial. Remote envelopes require explicit completeness fields. Attach restores
TTY/file flags; tmux IDs remain server-scoped. No public txm listener is added.
