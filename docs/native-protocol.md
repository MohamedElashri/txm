# Native protocol v1

The native server owns one PTY/VT, child process group, listener and bounded client
queues. Ordinary sessions do not need an agent coordinator. Linux/macOS CGO builds
support native; Windows persistence is unsupported.

## Discovery and ownership

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

## Frames and roles

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

## Bounds and shutdown

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
[CLI contracts](contracts.md).
