# Native protocol v1

Status: implemented by Phase 1 in `pkg/nativeprotocol`, `pkg/runtimepath`,
`pkg/nativeserver` and the native backend. Linux amd64 framing/fuzz/PTY evidence
is recorded in testing docs. macOS implementation awaits its real runtime gate;
Windows and other native runtimes are unsupported. Phase 2 public JSON and typed
backend capability contracts are documented separately in contracts.md.

Phase 4's coordinator and supervisor status sockets use a separate private
`agents` child directory and bounded JSON framing, specified in contracts.md.
They are not native PTY sessions and are never enumerated by ordinary native
discovery. Native framing/protocol version remains 1; old sockets and the native
observer/control boundary are unchanged.

## Discovery and coexistence

Use `<XDG_RUNTIME_DIR>/txm/v1` when XDG_RUNTIME_DIR is absolute, user-owned and
private (0700). If unset, use `<canonical OS temp>/txm-<uid>/v1`, validating and
creating the private user directory with 0700. Invalid explicit XDG runtime is an
error, not a silent fallback. A relative `TMPDIR` is rejected: changing the pane
cwd must not change the native runtime namespace. Use directory-relative no-follow filesystem
operations where the OS supports them. Reject paths exceeding the Unix socket
path limit before starting a child; never truncate names to make them fit.

Server socket filenames are `s-<instance_id>.sock`, where instance ID is 32
lowercase hexadecimal characters from 16 random bytes. A session-name lock is
`n-<sha256(name)>.lock`; hold an OS advisory exclusive lock for the entire server
lifetime. Validate names at both CLI and direct hidden-server entry points using
the existing alphanumeric/dash/underscore rule and a native limit of 128 bytes.
Name-lock files remain reusable across lifetimes: unlinking a lock can let two
different inodes protect the same name. Enumeration streams in bounded batches,
permits at most 1024 socket candidates and honors request cancellation;
accumulated name locks do not count toward that limit. Prefix application is a CLI
operation and happens exactly once before launch. The server locks the final name.

Discovery scans only the validated private v1 directory, dials candidates with
deadlines and checks the handshake. It returns names/instance IDs from the live
server, not filenames or PIDs. A dead socket is not a session. Stale cleanup
requires the name lock and verified socket owner/type/instance/inode; a dial
failure alone never authorizes unlink. Disable automatic UnixListener unlinking
and remove only the same socket inode owned by the shutting-down instance.
Phase 1 retains unresponsive stale sockets, skips connection-refused/disappeared
candidates and allocates a fresh random socket on the next start. It never
automatically removes a socket whose former name/instance cannot be established.

Legacy sockets use `os.TempDir()/txm-<name>.sock`. New clients may report owned
legacy candidates as incompatible, without claiming liveness. They must never
unlink them, send new protocol traffic, terminate their children, or silently
downgrade to the unsafe legacy protocol. Finish legacy sessions with the old
binary. Changing TMPDIR can hide legacy candidates; it is not a migration.
Native nuke refuses before any mutation if owned legacy candidates are present;
it cannot promise to remove incompatible sessions. Listing reports candidates
from a bounded scan of the legacy temp directory.

Creation passes an expected instance ID to the hidden native launcher, which
returns readiness only after lock acquisition, listener creation and successful
child start. The parent verifies the matching ID via handshake within five
seconds. A live socket belonging to another instance means conflict. An exec
failure or child exit before readiness is an explicit startup failure; M1 does
not invent a durable exit result. M2 receipts handle short-lived managed launches.
The launcher uses a private inherited readiness pipe (hidden `--ready-fd 3`),
then authenticates the matching socket/instance handshake. Hidden `--instance`
precedes the session name. Pipe errors/timeouts terminate and reap only that
launcher-owned server. Success proves a running child at handshake, not that it
will remain alive after the call returns; it may exit immediately afterward.

## Framing

Every message in both directions has a 20-byte header followed by exactly the
declared payload. All multi-byte integers are unsigned network byte order.

| Offset | Bytes | Meaning |
| --- | --- | --- |
| 0 | 4 | ASCII magic `TXM1` |
| 4 | 2 | Protocol version, currently 1 |
| 6 | 1 | Message type below |
| 7 | 1 | Flags, zero in v1 |
| 8 | 8 | Request ID |
| 16 | 4 | Payload length, maximum 1,048,576 bytes |

Read headers and payloads with `io.ReadFull`; validate before allocating. Reject
unknown magic/version/type/flags, oversized length, truncated header/payload and
invalid message-specific lengths. EOF at a frame boundary is a disconnect; EOF
inside a frame is a protocol error. Frames may be arbitrarily fragmented or
coalesced. All writes serialize through one writer per connection and handle
short writes. Never interpret partial payload bytes as a new control opcode.

| Type | Name | Payload and request ID |
| --- | --- | --- |
| 1 | HELLO | UTF-8 JSON `{role:"control"|"observer"}`; ID 1 |
| 2 | HELLO_OK | JSON identity/capabilities below; echo ID 1 |
| 3 | REQUEST | UTF-8 JSON operation; increasing nonzero ID, starting at 2 |
| 4 | RESPONSE | JSON `{data, error}`; echo request ID |
| 5 | DATA | Raw PTY bytes; ID 0, nonempty, maximum 16 KiB |
| 6 | RESIZE | Exactly 4 bytes: uint16 columns, uint16 rows; nonzero request ID |
| 7 | CLOSE | Empty payload; ID 0 |
| 8 | ERROR | JSON error `{code,message}`; request ID or 0 for transport failure |
| 9 | PING | Empty payload; nonzero ID |
| 10 | PONG | Empty payload; echo PING ID |

JSON table fragments are notation, not literal JSON fixtures. JSON payloads must
be objects with strict field validation, valid UTF-8, no trailing values and no
duplicate keys. A malformed request causes no operation side effect. Protocol
errors close the connection after a bounded best-effort ERROR; do not keep reading
an untrusted framing stream. `unsupported_version`, `protocol_error`, `denied`,
`not_found`, `conflict`, `timeout`, `limit_exceeded`, `internal` are wire errors.
Map them to public CLI errors in [contracts.md](contracts.md).
JSON field names match exact case; Go's case-insensitive aliases are rejected.
Byte fields (`data`, `content`) are base64 JSON strings; empty content is `""`.

## Handshake and privileges

The first frame must be HELLO within five seconds. Authenticate socket peer UID
before accepting a role, using platform-supported Unix credentials (Linux/macOS
implementations require platform tests). The owning UID may request control or
observer. A different UID is denied, even if socket access was accidentally
possible. The private directory and socket permissions remain required.

HELLO_OK contains `protocol_version:1`, `instance_id`, `backend:"native"`, `name`,
`role`, `process_state`, `capabilities`, `observed_at`. `process_state` is
`starting|running|exited|unknown`, not agent attention. This handshake binds
readiness/discovery to the actual server instance. No token or transcript is logged.

Observer permits status, snapshot, attach, ping and close. It denies input,
resize, kill and all other mutations, on **every** request/frame. A control
connection may perform those operations. This is a connection capability, not
isolation from another same-UID process opening a control connection. Native
read-only attach retains local detach-key handling without forwarding key bytes.

## Requests and ordered attach

One request is outstanding at a time per connection. Reused/out-of-order request
IDs are rejected. A client wanting concurrent snapshots/control uses another
bounded connection. Supported REQUEST operations:

- `status`: return instance, name, process state and actual attachment count as
  a full integer (not the legacy one-byte counter).
- `snapshot`: request `{format:"plain"|"vt",lines:N,max_bytes:B}`; validate bounds,
  capture under VT synchronization and return `{type:"screen_snapshot",encoding:
  "base64",content,lines,truncated,observed_at,position}`. B bounds decoded content
  at 1 MiB; the entire JSON response also fits the 1 MiB frame cap, so base64 and
  metadata overhead may reduce the returned count below B. Formatting streams
  into a bounded writer and stops at its cap. Attach rejects truncation rather
  than restoring partial VT state. Line limits also disclose omitted viewport
  rows through `truncated`. Unsupported history is an error. The Phase 2 capture
  service sanitizes public plain output, including hidden dump. Native raw attach
  retains terminal-stream semantics.
- `input`: control-only base64 bytes, at most 16 KiB decoded. A RESPONSE confirms
  bytes accepted by the PTY, never shell command execution or success.
- `attach`: response carries a bounded initial VT snapshot and output position;
  subsequent server DATA frames continue from that position without a gap or
  duplicate snapshot bytes. Buffer intervening output while writing the snapshot;
  disconnect on queue overflow, requiring a fresh attach/snapshot.
- `kill`: control-only; acknowledge acceptance, then bounded process-group
  termination/reaping. CLI confirms instance disappearance rather than treating
  the acknowledgement alone as completed termination.

DATA from client to server is permitted only after successful control attach;
server-to-client DATA is allowed only after attach. A control RESIZE validates
1..4096 rows/columns, at most 1,048,576 cells in total, and receives RESPONSE; observers cannot resize. The most
recent accepted control resize defines the shared PTY/VT dimensions. A failed PTY resize rolls VT back; failed rollback
terminates the server rather than pretending geometry is consistent. Session
rename/window/pane operations remain unsupported on native M1.

## Limits and cancellation

- Maximum 32 concurrent connections per server, including unattached control.
- Five-second handshake/request and startup deadlines; two-second socket write
  deadline. Interactive attach has no overall lifetime deadline.
- Per-attached-client queue: at most 256 KiB of PTY data and at most 64 frames,
  whichever bound is reached first. Slow clients are closed; they never stall
  PTY draining or other clients. Snapshot replies use the same finite writer but
  may exceed the queue's DATA limit within the 1 MiB frame cap.
- Heartbeat PING every 30 seconds on an otherwise idle attached connection;
  missing PONG after five seconds disconnects it. Cancel promptly on local detach,
  context cancellation, server exit or transport EOF.
- Input queue: 16 items of at most 16 KiB, each with a five-second deadline;
  expired input is not written later. Input and resize processing do not block
  PTY output draining. Only one worker writes PTY input.
- Native scrollback configuration: 1..1,000,000 lines, default 65,536.
- Child shutdown grace period: two seconds for process-group SIGTERM, then
  SIGKILL/reap where supported. Waitid with WNOWAIT observes exit without freeing
  the child PID/PGID before the last group signal. All PTY/client workers join
  before VT/log teardown. Report failure if completion cannot be proved.

These are initial limits to measure, not performance claims. Changes require
updated docs and boundary tests. Ordinary client DATA has no acknowledgement;
standalone input uses REQUEST when acceptance must be reported. Neither proves
child execution completion. Only future supervisor receipts can prove that.

Terminal query replies use the existing libghostty callback when detached or
observer-only. With a live control terminal attached, raw query passthrough lets
that terminal reply; the server does not inject an additional emulator response.
No new renderer or escape-sequence parser is introduced. A connection role is not
a promise to sanitize raw terminal output. Phase 2 public capture adds safe plain
sanitization and explicit raw opt-in without changing this wire framing version.
