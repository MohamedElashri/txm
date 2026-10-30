# txm roadmap

The implementation ledger is [plan.md](../plan.md); supported behavior and release
limits are recorded in [contracts](contracts.md) and [testing](testing.md).

## M1: reliable programmable sessions

Native framed IPC/lifecycle, exact backend targeting, truthful errors, capabilities,
versioned JSON, bounded output and strict profiles form the first milestone.
Normal session use stays independent of an agent coordinator. Linux evidence does
not certify macOS, other architectures or every archive; M1 release readiness
requires those explicit platform and packaging gates.

## M2: opt-in agent awareness

Phases 4–5 implement structured supervision, durable receipts, private registry,
authenticated attention signals, bounded JSONL events, deterministic waits and
explicit notification actions for Linux development. Generic workflows need no
provider. Claude initialization hooks have isolated real-CLI coverage; Codex
manual configuration has schema coverage, and OpenCode v2 remains unsupported.
No output text implies success or turn completion. M1 cross-platform gates remain
blocked; [M2 readiness](m2-readiness.md) records the evidence and limits.

## M3: dashboard

Phase 6 implements a separate `txm dashboard` on M1/M2 data APIs, with keyboard
search/filtering, capability-gated attach/read-only inspection, cancellable
refresh, bounded opt-in preview and explicit `--agents` subscriptions. Session
and registration observations stay separate; stale/unsupported data is visible.
Bare `txm` retains its picker. Resource metrics remain omitted until attributable
and tested; the TUI is not required for M2. [M3 readiness](m3-readiness.md) records
Linux evidence and inherited unrun platform/release gates.

## Optional extensions

Phase 7 implements separate opt-in Git task and SSH command namespaces, with
Linux development coverage. Tasks reuse supervised agent IDs; SSH aliases require
explicit allowlisting and host pins. See [usage and recovery](extensions.md) and
[extension readiness](extensions-readiness.md). Actual isolated Linux loopback SSH server/client smoke now passes; inherited
platform release gates remain unrun. Explicit full-ID task/agent metadata pruning
reclaims record capacity after verified cleanup/exit, retaining branches and
receipts. See the [Linux follow-up and handoff](linux-followup.md). Cloud sync is deferred. Process migration between backends,
restoring arbitrary running processes after reboot and recreating mux layouts in
the native backend are outside the current scope. Backend-specific resurrection
metadata does not establish supervised process completion or portable live state.
