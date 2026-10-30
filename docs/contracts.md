# Public contracts

Status: M1 v1 backend/JSON/output/profile contracts plus Phase 4 Linux development,
with Linux evidence in testing.md; platform/release readiness remains separate.
M2 invariants are fixed below, with complete launcher/hook/event schemas frozen
before PR 4A–5B. Phase 2 implements classified exits and one JSON envelope for
supported reads, including expected failures. Native framed IPC is separate.
Protocol framing version and public JSON schema version are independent.

Phase 2 implementation decisions: compatibility action methods remain while
reads use context-aware data interfaces. tmux resolves sessions, windows and panes
from bounded list queries to server IDs before mutation; an empty pane is accepted
only when the window has exactly one pane. Names that are also numeric indexes
must resolve to one identity, otherwise the target is ambiguous. ID selectors
must belong to the requested session/window. Screen retains its full `PID.name`
identity and rejects duplicate names. Screen/Zellij exact pane/layout mutations
are unsupported until verified, without focus changes or force-retry fallbacks.
tmux queries first verify a compatible 3.x version and cache that observation for
the CLI invocation; other major/development versions fail before mutation. Local
runtime certification is 3.4 only. Session/window metadata uses printable field
separators; unrepresentable metadata fails rather than guessing a target.

The default output is the last requested lines of the visible screen. History
is explicit via `--history` and unsupported on native; tmux history uses a bounded
line range, including visible content. Output bounds may omit a suffix at the byte
limit; `truncated` is true when the line or byte limit omits available content.
Plain output is sanitized before final UTF-8-safe byte/line limits. External
capture stops at the collection cap and kills/reaps its query process; stderr is
bounded separately and is not copied into public error messages. Queries default
to five seconds, with a 250 ms pipe-drain limit on cancellation.

Host identity is initialized only for structured-state reads, never picker
previews or ordinary mutations. Unix state paths use descriptor-relative,
no-follow traversal, stable private locks, fsynced temporary files and atomic
publication without replacing existing identity. Platforms without verified
private-state permissions return unsupported for identity-dependent reads; builds
alone do not establish that capability.
The file is `$XDG_STATE_HOME/txm/v1/host.json`, or
`~/.local/state/txm/v1/host.json` when the variable is unset. The base must be
absolute, owned and not writable by group/others; txm/v1 directories are 0700,
identity and stable lock files 0600. Corrupt, linked, symlinked or public identity
files are errors. No host ID is written for invalid command arguments.

## Selection and exact targets

Existing human commands and aliases remain available. Introduce `--backend
native|tmux|zellij|screen` as a strict choice: unavailable means an error and no
fallback. Config and `TXM_DEFAULT_BACKEND` remain preferences with fallback;
preserve default preference tmux and order tmux, Screen, Zellij, native. Invalid
backend values and unreadable config return errors consistently at all verbosity
levels. Version, help and completion generation do not need a backend; dynamic
session completion does.

Local session targets accept a name in the selected backend or `backend/name`.
An explicit target backend conflicting with `--backend` is invalid usage. No
host-string syntax is reserved by M1; E2 must negotiate remote host identity.
Existing name validation/prefix rules apply to the name portion exactly once;
fully qualified output locators are already canonical and must not gain a prefix
again. Destructive operations resolve exactly one session before acting. Within
an adapter use exact names/full backend IDs, never prefix/substring/focus guesses.

Unqualified actions stay scoped to the selected backend. `nuke` continues to
affect that backend only. Aggregate reads require `--all-backends`, incompatible
with `--backend` or a qualified single target. Explicit aggregate discovery
attempts all four adapters; missing/unavailable backends appear as errors and
produce exit 7 even if some data is available. Sort sessions by backend, then name,
then backend identity for stable output. Never merge same-named sessions.

Ordinary attach retains create-if-missing behavior for compatibility when lookup
proves absence. Discovery failure is not absence. Read-only attach never creates
or mutates a missing session; unsupported read-only enforcement is exit 4.

## JSON envelope and compatibility

M1 JSON applies initially to `session list`, `session inspect`, and `backend
capabilities`. `session output --json` uses the same envelope when implemented.
Human output is the default. `--json` emits exactly one UTF-8 JSON object followed
by newline on stdout, including expected argument/backend/permission failures.
Logs and diagnostics go to stderr; no ANSI prefixes or progress messages on JSON
stdout. Do not duplicate the same human error through Cobra and Execute.

All envelopes have exactly these required top-level fields:

```json
{"schema_version":1,"data":{"sessions":[]},"errors":[],"partial":false}
```

- `schema_version`: integer, currently 1. Consumers reject unknown major versions.
- `data`: command-specific object, or null when no result could be obtained.
- `errors`: array of error objects; empty on full success.
- `partial`: true only when some requested backend/data scope succeeded and some
  failed. When all requested scopes fail, data is null and partial is false.

Empty discovery is successful data with an empty sessions array. An empty backend
server is distinguished from executable failure, denied access or malformed output.
Never scrape generic stderr text into success without version-specific tests.

Fields may be added compatibly within schema 1; consumers ignore unknown fields.
Do not change types, remove fields or reinterpret enums within schema 1. New enum
values require documented forward handling; unknown states render as unknown,
never success. IDs are opaque strings, counts are nonnegative integers, absent
optional values are JSON null (not empty-string guesses). Timestamps are UTC
RFC3339 with nanoseconds permitted. Tests freeze representative success, empty,
error and partial envelopes before shipping them.

## Errors and exit status

Error objects require `code`, `message`, `retryable`; optional `backend` and
`locator` are null when not applicable. Messages contain no transcript, secrets,
raw argv or environment values. Error codes are stable snake_case strings.

| Exit | Codes | Meaning |
| --- | --- | --- |
| 0 | none | All requested operations succeeded, or valid empty result |
| 1 | `internal`, `operation_failed`, `protocol_error`, `invalid_config` | General/internal/backend operation or corrupt config/state failure |
| 2 | `invalid_usage`, `invalid_target`, `invalid_profile` | Invalid args, flags or declarative input |
| 3 | `not_found` | Exact requested session/entity absent |
| 4 | `unsupported`, `unsupported_version` | Unsupported capability/protocol/schema |
| 5 | `conflict`, `ambiguous_target` | Duplicate, conflicting ownership or ambiguous target |
| 6 | `timeout` | Deadline elapsed before requested observation/effect |
| 7 | `backend_unavailable`, `partial_failure` | Backend unavailable or aggregate partial result |
| 8 | `denied` | Permission/role/peer authorization failure |
| 130 | `cancelled` | Signal/context cancellation; any available partial data remains explicit |

Cancellation exits 130, including an interrupted aggregate read. For other
aggregate errors use deterministic precedence: any valid partial result exits 7;
otherwise invalid input before side effects wins (2), then denied
(8), conflict (5), timeout (6), unavailable (7), unsupported (4), not-found (3),
general (1). Return every scoped error in stable backend/code order. Signal
cancellation restores TTY and exits with conventional interrupt status 130; it
is not command success. Phase 2 covers the map and schema-level golden tests.

## Session and capability data

Session list data is `{host_id, observed_at, sessions:[Session]}`; inspect data is
`{host_id, observed_at, session:Session}`. A Session requires:

| Field | Type | Semantics |
| --- | --- | --- |
| `locator` | `{host_id,backend,name}` | Exact mutable local locator; host ID required |
| `backend_id` | string or null | Exact adapter-provided identity, not a durable managed ID |
| `managed_id` | string or null | M2 managed identity; null for all unmanaged M1 sessions |
| `instance_id` | string or null | Verified native/managed instance, otherwise null |
| `process_state` | enum | `starting`, `running`, `exited`, `unknown`; shell/process truth only |
| `client_state` | enum | `attached`, `detached`, `unknown`; not process/attention status |
| `attached_clients` | integer or null | Known attachment count, not fabricated from existence |
| `observed_at` | timestamp | Time of successful observation |
| `source` | string | Named adapter/API observation method |
| `capabilities` | capability map | Effective per-session restrictions if different from backend |

The persisted host ID file is `{schema_version:1,host_id:<32 lowercase hex>}`.
Managed ID and per-launch instance ID are independently generated 128-bit values.
No fabricated ID derived from PID/name/hostname. Identity initialization is atomic,
private and errors on corruption; it requires no coordinator.

Capability data is `{host_id, observed_at, backends:[BackendCapabilities]}`.
Each backend entry has `backend`, `available`, `version` (string or null), and
`capabilities`. Each capability has `level` (`supported|degraded|unsupported`),
`reason` (string or null), `constraints` (array of strings), `source`, `observed_at`.
Version probe failures are explicit errors. Unsupported operations must fail before
side effects. Missing/incompatible backends report unsupported capability entries
in aggregate results. `degraded` never permits acting on an unverified target.
Screen/Zellij session/attach APIs are degraded, with real runtime unverified on
this host. Zellij listings may include resurrectable sessions; process state stays
unknown and attachment may invoke upstream resurrection. Working-directory support includes explicit cwd for native/tmux profiles;
Screen/Zellij only inherit invocation cwd for ordinary session creation. M1 never derives a child exit status from these listings.

Capability keys: `session_lifecycle`, `attach`, `read_only_attach`,
`output_snapshot`, `output_history`, `input_injection`, `exact_pane_targeting`,
`window_layout`, `working_directory`, `managed_launch`. Managed launch is
unsupported in the M1-only scope; Phase 4 enables opt-in native/tmux supervision
subject to platform/version limits below. A binary found on PATH is not runtime support evidence; see the
matrix in [architecture.md](architecture.md) and release evidence in testing docs.

## Output and terminal input

`session output TARGET --lines N --plain` defaults to safe plain output; `--raw`
is an explicit opt-in mutually exclusive with `--plain`. Default lines is 100,
valid range 1..10,000; optional bytes is 1..1,048,576, default 1,048,576.
Invalid limits fail before capture. Captures bound memory and runtime during
collection, including native IPC and external process stdout/stderr.

Output data is `{locator,type,format,content,lines,bytes,truncated,observed_at}`.
Type is `screen_snapshot|history`, never an implicit substitution; M1 defaults to
screen snapshot. Plain content is a UTF-8 JSON string; raw content is base64 with
an additional `encoding:"base64"`. `lines`/`bytes` describe returned decoded
content after limits/sanitization. `truncated` discloses omitted content. Safe
plain text removes terminal control/CSI/OSC/DCS/clipboard sequences and handles
incomplete UTF-8. JSON encoding does not itself sanitize terminal controls.

Raw captures preserve snapshot styling where supplied by the backend; they are
not a transcript of every original terminal byte. Human plain mode prints only
content; unsupported snapshots return exit 4. Picker
and dashboard previews use the bounded capture service, adding UI error/unsupported
messages themselves. Legacy hidden dump gets documented safe limits or deprecation;
it cannot remain an unbounded escape hatch for internal callers.

`pane exec` and root `exec` remain terminal input injection. Successful return
means the backend accepted bytes for the exact target, never that the shell
executed or completed them. tmux input uses bounded hex-byte batches, then Enter
separately: its command parser otherwise strips trailing semicolons even in
literal-key mode. Input is capped at 1 MiB and a five-second operation budget.
Place flags before positional arguments on create/attach/exec; trailing argv and
input strings, including leading dashes and `--json`, are literal arguments.
Managed execution is a different M2 operation with actual child exit receipts.

## Declarative profiles

Phase 3 decisions: profiles are bounded to 1 MiB, 32 windows and 128 total panes.
The tmux profile applicator requires 3.4+ (3.4 is locally tested); older versions
fail preflight before creation, even though ordinary tmux reads support 3.x.
Omitted windows/panes mean one default terminal. tmux windows without names receive
`window-1`, `window-2`, etc.; final names must be unique. The first pane cannot
request a split; later panes default to vertical splits of the preceding returned
pane ID. Supported layouts are `even-horizontal`, `even-vertical`,
`main-horizontal`, `main-vertical`, and `tiled`; custom layout strings are rejected.
Native accepts one unnamed/`0` virtual window and one pane, with cwd and optional
terminal input. Profile panes explicitly launch the invocation `SHELL` (default
`/bin/sh`) interactively, overriding tmux default-command so input is addressed
to the requested shell. The executable is validated during preflight. Other native shapes and all Screen/Zellij profiles are unsupported
before any backend command. Commands containing NUL or exceeding backend input
bounds are invalid. Empty cwd inherits invocation cwd; explicit relative cwd is
resolved against the profile file. Cwd is applied at pane creation, never by `cd`.

`profile --dry-run` validates schema, paths, shape and backend availability/version
without session discovery (which could initialize a native runtime), attachment,
host identity or session creation. It prints command presence, not command text.
`--detach` leaves a successful apply detached. `--attach-existing` permits an
unchanged existing session; it does not bypass profile validation. Read-only
profile attachment additionally requires `--attach-existing` and a proven existing
session, and cannot create/apply a profile. Read-only plus detach is invalid.

tmux creation records returned session/window/pane IDs and a random session
ownership token. Subsequent mutations and cleanup check ownership in the tmux
server command queue, avoiding name/PID/ID reuse guesses. Native tracks the exact
random launch instance. Rollback is limited to a proven owned session and uses a
separate finite cleanup context after cancellation. Missing/malformed creation
receipts or failed ownership checks yield `partial_failure` (7), with surviving
or uncertain resource locators, rather than deleting by name. No rollback can undo
external effects of terminal input already accepted. All panes/layouts are built
before input is sent; success means input acceptance, never command completion.
New tmux panes have input synchronization disabled before profile injection;
ordinary exact input refuses synchronized panes instead of broadcasting. The
server queue rechecks synchronization before each input batch/Enter, including
changes made after preflight.

Preserve `name`, `backend`, window `name/layout`, pane `command/dir/split` keys.
Unknown TOML fields are errors. Shell command strings keep shell semantics with
no completion guarantee. Apply `TXM_SESSION_PREFIX` once consistently; validate
all final names, splits, cwd paths, duplicate targets and capability requirements
before mutations. Relative dirs resolve against the profile file's directory;
`~`/environment expansion is not implicit. Layout identifiers are validated against
the selected adapter. Explicit CLI backend and profile backend disagreement is
invalid input; profile backend alone is strict, not a fallback preference.

Dry-run validates and prints intended targets/capabilities without exposing full
commands by default, does not attach, create sockets, start coordinator or write
host/managed identity. `--detach` applies without interactive attach. Existing
session is a conflict unless explicit `--attach-existing` requests attaching
unchanged; never partially reapply or infer equivalence from a matching name.

For tmux capture actual IDs returned at session/window/pane creation. Honor
base-index/pane-base-index and cwd before starting commands; never guess window
0/1 or rely on current focus. Apply only supported layouts; unsupported native,
Screen or Zellij shapes fail in preflight. On partial failure, roll back only
resources whose returned IDs prove this invocation created them. If ownership
or rollback safety cannot be established, leave resources intact and return a
partial_failure error identifying survivors; never destroy a preexisting session.

## M2 supervision, signals and events

### Phase 5 signal/event/wait contract

The new Phase 5 request authorizes Linux implementation and validation while the
earlier platform release gates remain blocked. Stop before Phase 6. Provider
integration never enables permission bypass, reads transcripts, or grants tool
approval. Generic signals work without a provider installed.

`agent hooks enable ID --provider generic|claude|codex` creates a private 0600
credential for one registration, generation and instance. Managed hooks bind the
launch instance; unsupervised registrations get a separate hook association,
not a managed process identity. Rotation invalidates the previous generation.
Tokens are random 256-bit values in private credential files, never CLI argv,
registry, status, logs or event metadata. Same UID is the trust boundary.

`agent signal ID --instance I --generation G --sequence N --time RFC3339
--state unknown|working|needs-input|idle|turn-complete` loads that credential and
submits a normalized authenticated observation. `--from-env` uses the explicit
managed hook context and allocates a sequence under a private client cursor lock.
Intake requires exact registration/generation/instance/token, sequence strictly
greater than the last accepted sequence, nondecreasing time, at most two minutes
old and at most five seconds in the future. State/sequence survive coordinator
restart. Fresh attention expires after two minutes and becomes unknown; process
lifecycle and observed exit receipts never derive from signals. Turn completion
is optional, sourced separately and cleared when new work begins.

`agent start --provider generic|claude ...` opts into a hook context (opaque IDs
and private namespace paths, no token value) in the child's environment. Claude
receives an explicitly generated settings file, not edited user/project config.
Codex hook config is generated for manual trust review; automatic trust bypass
and automatic Codex hook activation are unsupported. Installed-version allowlists
are exact and recorded in provider evidence. OpenCode v2 integration is unsupported
until its actual plugin/session association is verified. `agent hooks config ID`
prints reviewable provider configuration without modifying external settings.

`agent hook ID --generation G --provider claude|codex` is an observational bridge.
It reads at most 64 KiB of provider JSON, selects only event/session identifiers,
and never forwards prompt/tool/transcript/message contents. First authenticated
provider input binds its session; a different session is rejected. SessionStart
reports unknown, UserPromptSubmit working, PermissionRequest needs_input, and
Stop idle only when stop_hook_active is false. Stop is not proof of final turn
completion or process exit. Unsupported events are ignored. Bridge stdout is
always `{}` and failures are nonblocking warnings, never approval/block decisions.

`events --jsonl [--id ID]` emits one safe event per line. Each has schema_version,
coordinator_epoch, sequence, time, managed_id (nullable), agent_id (nullable), kind,
source and typed metadata. The coordinator serializes state and subscription
creation, so the initial resync_required includes an authoritative snapshot with
no snapshot/subscribe gap. Sequence increases within an epoch; filtered streams
may have gaps. At most eight subscriptions, each with 16 queued events, coexist
with bounded short requests. Overflow drops queued notifications and requires a
fresh resync snapshot; reconnect/epoch changes also resnapshot. No replay or
exactly-once promise. Writes have a two-second deadline; one-second heartbeats detect loss.
Reconciliation runs at most every 500 ms only while subscriptions exist, and
stops with the last subscriber. Ordinary commands never poll/contact agents.

`agent wait ID --until exited|needs-input|idle --timeout D` defaults to 30 seconds,
maximum 24 hours. It uses the atomic subscription snapshot, events and bounded
reconciliation after reconnect/uncertainty. Exit 0 means the condition was
observed; exited includes nonzero child outcomes. `--fail-on-nonzero` additionally
returns operation_failed (1) for nonzero/signal/start_failed receipts. Timeout is
6 and cancellation 130. Exited on an unsupervised registration or attention on a
registration without hooks is unsupported (4). Proven loss without a receipt is
unknown, not success; a short reconciliation grace avoids mistaking receipt
publication for loss. Coordinator restart never re-executes a managed command.

Notifications are disabled by default. `agent notify -- EXECUTABLE [ARGS...]`
explicitly opts into an event-driven action using direct argv, sanitized metadata
on stdin, a two-second action deadline and five-second per-agent throttle.
Snapshots/reconnects never replay actions. No transcript, prompt, raw provider
payload or token enters action metadata. No notification configuration is
implicitly enabled by agent start, hooks, events, wait or ordinary session use.

### Phase 4 local development contract

The Phase 4 request authorizes Linux development while M1's unrun platform release
gates remain blocked. This is not M1/M2 release certification. Initial managed
launch and registration support is native with CGO and tmux 3.4+; other backends
and unverified transports remain unsupported. Phase 5 extends this local contract
with the signal/event/wait contract above.

`agent start NAME [--cwd DIR] [--env NAME] -- EXECUTABLE ARGS...` starts a new
supervised session and returns its opaque agent ID. `--backend` is strict; default
selection follows the existing preferences. Argv is executed directly, without a
shell or project hooks. `--env` is a variable-name allowlist applied to the
wrapper's backend launch environment, not an upload of the client's environment
values. Defaults are HOME, PATH and TERM; explicitly named variables extend it.
Backend environments can predate the requesting CLI; no claim of environment
synchronization is made. Cwd is resolved/validated before launch.

`agent register TARGET` allocates an agent registration ID for an existing exact
native/tmux session. It allocates neither a managed-session ID nor a supervised
launch identity/receipt. `agent list` and `agent status ID` reconcile state; all
four commands accept the version-1 JSON envelope. Status targets are full opaque
IDs, never guessed names. `agent stop-coordinator` stops only the matching local
coordinator; managed sessions/children continue. Agent commands alone can start
it. Ordinary commands do not discover/contact/start the service.

Files live below private `txm/v1/agents` state/runtime roots. Directories are 0700;
regular files/locks/receipts/manifests are owned, 0600, single-link and opened
without following symlinks. Manifest `m-INSTANCE.json` has schema_version=1,
managed_id, instance_id, backend, argv, cwd, env_names and receipt filename
`r-INSTANCE.json`. It is bounded to 128 KiB, 256 arguments and 128 variable names.
Only executable/wrapper paths and validated opaque IDs enter backend launch argv;
user argv remains in the private one-use manifest. A per-instance lifetime lock
prevents a second wrapper from consuming it. Consumption securely unlinks it
before executing the child; invalid manifests never run.

Start leaves the session detached; use ordinary `attach backend/name` to interact.
The wrapper owns immutable atomic receipt publication. Receipt v1 contains
managed_id, instance_id, backend, started_at (nullable), finished_at, outcome
(`exited|signaled|start_failed`), exit_code (nullable) and signal (nullable).
Only a reaped child establishes exit_code/signal; failed exec is start_failed
with neither. No raw exec error/argv/environment is persisted. Wrapper
SIGTERM/SIGHUP/SIGINT initiate SIGTERM to the child's separate foreground process
group (terminal-generated interrupts go directly to the foreground child); bounded cleanup
kills resistant descendants before releasing the child PID. Closing the terminal
or killing the wrapper/server can prevent a receipt, which yields unknown.

Registry v1 contains host_id and at most 256 agent records. A supervised record
has agent ID, managed ID, distinct launch instance ID and a verified backend
binding; registration has only agent ID and observation binding. It contains no
argv, environment values or transcripts. Only the locked coordinator writes
atomic registry snapshots. Wrapper status IPC proves live child state with a
matching instance and current kernel observation, never a PID lookup alone.
Receipts are checked against both IDs/backend. A missing receipt after wrapper
loss yields unknown; an unavailable backend cannot itself establish child exit.
A still-authenticated wrapper can independently prove a running child. Recovery never
relaunches. Native instance/tmux ownership bindings survive name changes and
refuse name reuse; observed renames update only locators.

The local coordinator uses same-UID authenticated Unix sockets with a random
epoch, a stable singleton lock and owned-inode cleanup. Framing is a 4-byte
big-endian length followed by strict UTF-8 JSON, at most 1 MiB; schema/epoch must
match. Every request also carries the client's persisted host_id and validated
state-root path; the coordinator rejects a different state identity/location
before accessing the registry or launching, even if a host-ID file was copied.
Changing XDG_STATE_HOME cannot silently use another running registry in the same
runtime directory. Each connection handles one request with finite deadlines, at most 16
concurrent clients. State is serialized by the coordinator; reconciliation runs
for agent requests and active event subscriptions. Slow/malformed clients cannot stall ordinary sessions.
Restart/corrupt metadata never grants permission to unlink an unverified socket
or fabricate identity/state. Lifecycle/attention/source/observation time remain
separate; attention stays unknown with attention_source="no verified hooks" and
attention_observed_at=null unless explicit Phase 5 hooks provide fresh signals. Ordinary
backend session reads do not join registry state or fabricate managed IDs;
`agent` views expose managed associations without a coordinator dependency in M1.

Public agent data is `{agents:[Agent]}` for list and `{agent:Agent}` for
start/register/status. Agent has id, nullable managed_id/instance_id, locator,
supervised, lifecycle, attention, attention_source, nullable attention_observed_at,
source, observed_at, nullable receipt/turn_complete_at and optional public hooks
(provider/version, instance/generation, sequence and session_bound; no session ID/token). Backend binding markers are private
registry data, not part of public Agent. Only full IDs are status targets;
`--backend` filters list and rejects a conflicting status target. Start/register
accept JSON envelopes as mutations, unlike M1's read-only JSON command set.
Partial launch failures retain Agent data and classified errors (exit 7). Query
success does not imply child success, and a receipt with exit 42 remains queryable
with CLI exit 0. Status after unprovable loss is unknown with receipt=null.
Cancellation retains exit 130 and does not promise a detached launch was undone.

The coordinator owns registry and snapshots; wrappers own atomic receipts. A
manifest contains structured argv/cwd and variable-name allowlist, is private
0600 and consumed once. Do not persist command/env secrets in the registry.
Manifest/receipt, registry/reconciliation and Phase 5 hook/event/wait v1 are
specified above. Only explicit hook clients write their private sequence cursor;
accepted state remains coordinator-owned.
Process `exited` requires an observed receipt; missing proof stays unknown.
Registering an existing session cannot retroactively supervise it.

Agent lifecycle, attention and turn completion remain separate, sourced and
timestamped. Generic registrations/launches work with unknown attention when
hooks are absent. See [provider support and M2 readiness](m2-readiness.md) for
verified versions, source documentation and runtime limitations.

## Phase 1 compatibility corrections

- Config read/invalid backend/invalid positive-size values fail consistently;
  version/help/completion generation work independently of config/backend setup.
  Config set does not overwrite an unreadable config with defaults.
- Read-only attach never creates a session. Native enforces an observer connection
  role, including resize denial and a working local detach key. tmux uses its
  enforced read-only attach flag. Zellij/Screen requests are rejected until proven.
- Native transport accepts only the private v1 namespace. Legacy candidates are
  reported and left untouched; native nuke refuses when they are present. Native
  operation failures no longer log a success-equivalent exit code.
- Native `exec NAME 0 0 COMMAND` sends input to its single terminal; other named
  windows/panes are unsupported. Accepted input is not command completion.
- `--log` is native-only and scoped to creation; inherited environment is restored
  afterward. Existing log files must be owned, regular, mode 0600 and not linked.
- Windows/other native runtimes and Linux/macOS builds without CGO report native
  unavailable. Cross-builds do not certify interactive runtime behavior.

## Phase 6 dashboard contract

`txm dashboard [--backend NAME|--all-backends] [--agents] [--preview]
[--refresh D]` is a separate interactive entry point. Bare `txm` retains the
existing picker and never invokes dashboard refresh/subscriptions. Dashboard
requires the verified Linux runtime and terminal stdin/stdout; non-TTY use fails
with invalid_usage (2) before config/state/backend/coordinator access. Other dashboard platforms are unsupported (4) until ancestry/TTY behavior is verified,
independent of cross-build success.

Default dashboard scope is all four local adapters; explicit --backend is strict.
Failures remain per-backend errors; successful reads of other backends continue.
Only explicit --agents starts/subscribes to the opt-in M2 service. Without it the
dashboard does not access the agent registry/runtime or contact a coordinator.
Host identity is the existing private M1 host ID, never a remote host alias.

Session rows use core.Reader data and agent rows use agent.Stream/View data.
They remain separate: a reusable backend/name is not proof that a registration
owns a live session. Session rows show management/attention as unassociated or
not queried, never guessed. Agent rows show supervised/registered status,
lifecycle, attention source/time and receipt/turn information separately. They
have no invented attachment/client/capability facts. Attach/preview require a
live session row, revalidated using its backend ID and name. Native operations
pin its authenticated immutable instance. tmux IDs remain scoped to the current
server, with no durable identity promise for unmanaged sessions. No action
creates a session, injects input, changes configuration or deletes resources.

Arrow/j/k/Ctrl-N/Ctrl-P and page keys navigate; / enters bounded case-insensitive
search; Escape clears search/query first (quits when empty); Tab cycles state/type filters; b cycles backend
filters; p toggles preview; r requests refresh; Enter attaches and o attaches
read-only when supported; q quits; Escape quits outside search with an empty query; Ctrl-C cancels (130).
The selected stable row key survives refresh/reordering. Unsupported actions
are disabled and explained rather than executed optimistically.

Each backend has at most one refresh in flight, with a two-second request budget,
a two-second default interval (allowed 500ms..1m), and at most 2048 retained
sessions. UI render displays at most 2048 session rows plus 256 registrations,
with an explicit truncation notice. Slow/unavailable sources do not hold up
keyboard events or other sources. Failed refreshes retain stale rows for context
and disable actions. Agent disconnect is detected from missing heartbeats after
five seconds, then state is labeled stale/unknown until resync. No event replay
or provider-status inference is added. Refresh and subscription work stops on
quit, cancellation and attach, with bounded cleanup; resumes after detach.

Preview is off by default. Only the selected supported session is captured,
with one cancellable request, at most 16 KiB/100 lines, no history/raw capture,
and no more than one automatic request per two seconds. Selection changes cancel
old requests and reject late results; unsupported/error/stale messages are UI
messages rather than successful snapshots. Metadata and preview pass through
core.Plain before rendering; no raw terminal escapes execute and nothing is
written to a transcript. Search is limited to 128 runes; screen storage/rendering
is limited to 512 columns by 200 rows. CPU/memory agent metrics are omitted.

The screen owns terminal modes only while displaying the dashboard. Attach first
cancels/joins refresh work, stops event input and restores the screen/TTY, then
calls the typed backend action with direct argv. On detach or action failure the
dashboard reinitializes and refreshes. All exit/error/cancellation paths restore
TTY modes, cursor, alternate-screen and file flags. Ordinary picker behavior and
M1/M2 wire schemas remain unchanged.

Pre-run Linux development budgets: with 1000 fixture sessions, input/resize/quit
response within 500ms on a normally draining terminal, and idle dashboard RSS
under 64 MiB. Slow-backend cancellation must return within three seconds. These
are harness budgets, not a cross-platform SLA. Inherited platform release gates
remain blocked and must be listed in the M3 readiness record.

Linux dashboard checks bounded process ancestry and controlling-TTY context only
to disable attach/preview of its own native PTY, preventing output feedback. This
also covers earlier framed native servers without environment markers. It never
uses PID ancestry as lifecycle/managed identity proof, never persists PIDs and
never grants authority. An incomplete /proc check disables native actions while
other backend reads continue. Non-Linux dashboard runtime stays unsupported until
a corresponding guard and TTY behavior are verified.

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

## Phase 7: opt-in extensions

E1 and E2 initialize private state only when invoked. They do not change normal backend selection, start a service during ordinary M1 reads, or require a provider. Linux development validation does not clear inherited platform release blockers.

### E1 task ownership

`task start NAME --repo DIR [--base REV] [--branch NEW] [--dry-run] [--allow-repo-commands] [--env NAME] -- EXEC [ARGS...]` uses a strict native (default) or tmux backend. Names are bounded ASCII. Git resolves a committed base, including detached HEAD; source dirty files are never copied. A fresh branch is mandatory: existing branches are never reset/reused. Worktrees live only in the private state namespace at task-worktrees/FULL_ID. Branches and commits are retained on cleanup and rollback.

Dry-run performs read-only Git/command preflight before creating identity, metadata, worktrees or a coordinator; it redacts argv/env values. All Git routing/config environment variables are stripped. Git hooks/fsmonitor are disabled by default and configured external filters cause refusal. --allow-repo-commands explicitly permits Git hooks/filters; no setup scripts are inferred. Git operations have bounded contexts/output and never invoke a shell.

Task metadata is schema-versioned, private, atomic, writer-locked and bound to host/state path plus repository/common-dir/worktree/.git device+inode identities. A task ID is preallocated as its managed agent ID and persisted before launch; a reused ID is rejected. Recovery observes that exact ID and never replays a launch. Existing managed lifecycle/receipt state is authoritative; task bookkeeping is not another session state machine.

`task list/inspect FULL_ID/stop FULL_ID/cleanup FULL_ID` never resolve names fuzzily. Stop verifies the exact supervised backend binding and native launch instance or tmux ownership marker; it never signals a remembered PID. Cleanup requires a durable exit receipt for a launched task. An orphan is safe only when its task journal proves launch was never requested. Missing coordinator registrations after launch are unknown, never proof of an orphan; a separately pinned instance may recover a validated immutable exit receipt from the same private state. Unknown/live/inconsistent bindings fail closed. Dirty, ignored and untracked files prevent default cleanup. Force requires both --force and --confirm FULL_ID and cannot bypass identity or process proof. Cleanup uses Git worktree remove only, never reset/clean/prune/RemoveAll, and retains branches. Failed or interrupted starts retain inspectable metadata and owned worktrees for explicit cleanup; there is no automatic rollback or launch replay. A partial Git operation without durable ownership proof requires manual inspection.

### E2 SSH transport and targeting

`remote allow ALIAS [--host-id FULL_ID] [--executable txm|/safe/path/txm]`, hosts, pin and forget maintain a private explicit allowlist (max 32). Aliases are bounded ASCII SSH configuration aliases, not user@host/URI/options. Executables are one safe POSIX token. Unpinned entries support probe only. Host IDs are pinned explicitly; duplicate identities across aliases are refused. SSH configuration is never enumerated.

`remote probe ALIAS` negotiates schema/protocol 1 and reports identity. `remote list [ALIAS...]` defaults to native, queries only pinned aliases, runs at most four requests concurrently, and bounds each request to 1..30 seconds (default 10) within a 60-second CLI budget. Failed hosts are scoped errors, never empty successes; partial output exits 7. Strict decoding rejects duplicate/unknown keys, incompatible versions, identity/backend mismatches and duplicate backend IDs.

OpenSSH reuses user authentication and ControlMaster configuration, with BatchMode=yes, StrictHostKeyChecking=yes and finite connection/liveness settings. No remote listener, authentication material or forwarding is installed. SSH remote arguments are joined by a shell, so the entire command is a validated executable token plus fixed internal endpoint and a bounded base64url JSON request token; names/IDs never enter shell text literally. Read requests use -T. Local captured stdout/stderr are bounded; unsafe remote diagnostics are not echoed.

`remote attach ALIAS --host-id ID --backend native|tmux --name NAME --session-id ID [--instance ID] [--read-only]` requires an explicit pinned host, exact observed backend ID/name, and native instance. It requires a TTY, uses -tt and restores local terminal state/descriptor flags after cancellation. The hidden endpoint checks protocol/host identity before backend access and reuses exact observed attachment without creating sessions. Unmanaged tmux IDs retain their existing server-scoped identity limit. Interactive output is terminal data, never mixed with a JSON envelope.

The remote JSON envelope requires all four fields: schema_version, data, errors and partial. Successful captured queries require an explicit non-null `partial: false` and non-null errors array; missing/null flags are malformed protocol responses, not verified complete results. Phase 8 enforces this distinction without changing schema/protocol versions.

Reference: [OpenSSH remote command argument joining](https://man.openbsd.org/ssh.1), [Git worktree operations](https://git-scm.com/docs/git-worktree).

### Phase 8 task recovery hardening

Task records optionally pin the existing supervisor instance ID after a validated
start reply. Old records without that field remain readable. When a launch was
requested and its registration disappears, cleanup fails closed unless that exact
pinned instance has a validated exit receipt in the same private state. A lost
reply without an instance pin requires manual recovery if the registry is lost;
no receipt/manifest/name scan guesses the association. Force never bypasses this
proof. Backend name collisions are preflighted before task state/worktree creation.

### Linux loopback SSH validation follow-up

Interactive remote exit 1 preserves operation_failed/exit 1, rather than being
misreported as a transport outage/exit 7. OpenSSH connection/authentication failures
(exit 255) remain backend_unavailable/exit 7. Actual server tests use fresh fixture
keys, strict host checking and a private -F client configuration in a network-isolated
container; no user SSH config/key is read or changed.

### Explicit metadata pruning

`task prune ID [--dry-run] [--confirm ID]` removes only a cleaned task's private
index record. The worktree must remain absent and launch/exit proof must still
validate. Branches, receipts and agent records remain. Prune tasks before agents.
`agent prune ID [--dry-run] [--confirm ID]` removes only the exact registration;
supervised registrations require a validated exit receipt. Unsupervised removal
is registration removal only and leaves the session running. Actual pruning
requires matching full-ID confirmation; dry-run does not. No automatic pruning,
process termination, worktree removal or receipt/manifest deletion is performed.

The coordinator remains sole registry writer. It writes bounded host-bound ID
tombstones before removal so caller-provided managed IDs cannot replay a normally
pruned launch. Failed publication retains the registration. Corrupt tombstones
block new starts/prunes, not ordinary core sessions. Existing all-agent streams
receive a fresh resync snapshot; a removed-ID subscription ends with not_found.
