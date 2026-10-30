# Phase 7 extension readiness

E1/E2 are implemented as independent opt-in namespaces/packages in the existing
executable. They add no dependency, background service or ordinary-path Git/SSH
lookup. This is Linux development work, with no release, commit or publication.

The [extension guide](extensions.md) describes ownership, preflight, retention,
manual recovery, host pins and attachment. The [contracts](contracts.md) specify
public semantics. Checks/source fingerprints are recorded in
[Phase 7 evidence](evidence/phase7-validation.json).

Inherited M1/M2/M3 platform, foreign-UID and provider runtime validation gates
remain Blocked. Windows/macOS CGO-disabled builds are compilation evidence only.
Native/tmux runtime validation is Linux amd64 and tmux 3.4. Screen/Zellij
task/remote actions are unsupported. No providers/model calls are required.

Git 2.43.0 and OpenSSH client 9.6p1 are installed; sshd is absent. Fake SSH checks
the actual endpoint in a second private environment, including PTYs, encoded
argv, identity/version errors, timeouts, cancellation and partial results. Actual
network authentication, host keys and ControlMaster/server interoperability
remain unrun; no real SSH alias was contacted.

Failures retain worktrees for explicit cleanup; no automatic rollback or launch
replay occurs. A crash before ownership was saved, missing/moved paths, replaced
repositories or corrupt/copied metadata fail closed and require manual inspection.
Completed records remain, with 128-task/256-agent limits and no automatic pruning.
Branches/commits always survive cleanup. Same-UID processes are not a sandbox.

## Validation result

Eight new unit groups and 12 disposable Git/fake-SSH integration groups pass
normally, under the race detector, and from actual GNU/static-musl ZIPs. All
retained M1/M2/M3 runtime suites pass for those four binaries, including 14
dashboard groups per binary. Go 1.27.3 and minimum 1.26.10, Go race, lint, module
verification, four fuzz targets, fixture guards, ZIP install/completions and
CGO-disabled refusal checks pass. Windows/macOS amd64/arm64 cross-builds pass
as compilation-only evidence. Real provider initialization was skipped in this
phase; prior provider evidence/limitations remain.

Isolated unchanged M1 budgets pass: startup 6.96 ms (20 ms), list100 109.76 ms
(2000 ms), peak native server RSS 22,912 KiB (32,768 KiB). These are fixture
measurements, not a cross-platform or coordinator-scale SLA. Exact commands,
outputs, hashes, dashboard metrics and skip reasons are in the evidence manifest.

Latest Linux audit, recovery/protocol hardening and final validation are recorded
in [the Phase 8 audit](final-audit.md). Historical results above retain their
original scope; unrun release gates remain Blocked.

## Subsequent Linux follow-up

Actual different-UID rejection and real loopback SSH server/client gates are now
covered in isolated Linux containers. Explicit `task prune`/`agent prune`
reclaim index capacity after verified cleanup/exit, retaining Git and receipt
data. See [current follow-up](linux-followup.md) and the [current support matrix](final-audit.md).
The original Phase 7 evidence and unrun platform/provider gates above remain
historical and are not relabeled released.
