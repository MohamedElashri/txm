# Final implementation audit and release status

Phase 8 validates and cleans up the existing M1/M2/M3 and E1/E2 implementation.
The audit is scoped to Linux amd64 development. Release certification remains
**Blocked** until required platform/provider gates have actual
evidence. No commit, PR, push, tag, release or publication is part of this work.

## Audit scope and findings

| Area | Reviewed implementation and proof | Result / limit |
| --- | --- | --- |
| Framing / concurrency | nativeprotocol, localipc, nativeserver; strict bounds/JSON, fragmented frames, connection/queue limits, cancellation and worker/VT shutdown; unit/fuzz/real PTYs | Retain bounded observer/control roles and backpressure; no new wire schema |
| Permissions / identity | runtimepath, supervisor, agent; private mode/UID/no-follow checks, instance handshakes, ownership markers, atomic publication and receipts | Missing proof stays unknown; actual foreign-UID filesystem/peer rejection now covered in disposable Linux containers |
| Exact targeting / shutdown | backend profiles, managed bindings, dashboard actions, task cleanup | Native instance or tmux exact ID/ownership required; no remembered PID authorizes an action; unmanaged tmux IDs remain server scoped |
| Terminal output / TTY | core.Plain, native attach, dashboard, remote guard | Bounded safe previews, explicit raw opt-in, read-only input blocking and TTY/file-flag restoration; non-Linux dashboard disabled |
| Recovery / state | agent registry, supervisor receipts, task metadata | Fixed missing-registration orphan inference: task pins supervisor instance, requires that exact receipt after registry loss, and refuses unknown/lost-reply cases even with force |
| Remote envelopes | remote.Query, strict decoder regression | Fixed missing/null partial flag acceptance; require explicit completeness and data fields; existing schema/protocol 1 retained |
| Fixture isolation | sandbox.py and build-helper tests | Fixed inherited HOME/Git/environment routing in build-helper fixture; explicit environment now prevents real-repository redirection |
| Privacy / artifacts | hook/event fields, optional logging, synthetic fixtures, ZIP allowlist and bounded credential-pattern scan | No real prompt/provider data introduced; scan is a known-pattern check, not a universal secrecy proof |
| Documentation / packaging | README, testing guide, man page, generated bash/zsh/fish/PowerShell completions | Corrected stale phase/deferred statements; archive smoke now checks task/remote completion flags |

The protocol, recovery and fixture-environment fixes have regression coverage. Normal CLI/session behavior and M1/M2
schemas remain unchanged; only malformed remote responses become errors. The
validation and privacy tools require explicit input paths, write new output
locations, and do not discover real sessions/hosts or download dependencies.
Earlier phase evidence is retained with its original fingerprints; final evidence
binds this working tree and actual artifacts separately.

## Support and unresolved gates

| Platform / capability | Local validation scope | Release gate |
| --- | --- | --- |
| Linux amd64 native + tmux 3.4 | Normal/race unit and real PTY workflows; actual GNU/static-musl ZIP install/runtime | Linux development evidence only |
| Linux arm64 | Earlier compilation-only scope; no native hardware runtime/archive certification | Blocked / unrun |
| macOS amd64/arm64 native + external backends | CGO-disabled cross-build only; no macOS host/SDK/runtime archive smoke | Blocked / unrun |
| Windows amd64/arm64 | CGO-disabled cross-build only; native/private runtime/dashboard unsupported | External backend/install/archive gate unrun |
| Screen / Zellij | Real Linux Screen 4.09.01 and Zellij no-web 0.45.1 basic lifecycle, exact targeting, namespace isolation and attach/TTY tests | Degraded; pane/layout/read-only parity remains unsupported |
| SSH aggregation | Real isolated OpenSSH 9.6p1 loopback server/client, keys, ControlMaster, partial results, timeout/cancel and native/tmux PTYs | No external host/WAN or other OS claim |
| Generic agent | Supervised receipts, authenticated signals/events/wait and local action fixtures | Linux evidence; not provider success inference |
| Providers | Offline real Claude 2.1.308 initialization; installed Codex 0.166.0/OpenCode 2.2.2 are outside the adapter allowlist | Trusted/full provider lifecycle gates unchanged |
| Foreign UID | Actual UID 65534 filesystem denial and separately reachable SO_PEERCRED rejection for native/coordinator; owner services survive | Linux container evidence; no macOS credential-path certification |

QEMU availability is not native arm64 hardware evidence. A compiled executable or
configured CI job does not replace an executed platform gate. These gaps must be
closed on isolated runners before advertising or publishing a whole-project
release; they do not justify changing permissions, approving providers or testing
against a user's real sessions/worktrees/SSH configuration.

Task branches/commits and completed records remain. Missing/moved worktrees,
crashes before ownership persistence and copied/corrupt state require manual
inspection. Limits remain 128 task records and 256 agents with no automatic
pruning. Explicit exact-ID task/agent pruning now reclaims index capacity after
validated cleanup/exit; receipts, branches and launch files remain, with a
bounded retired-ID replay guard. Escaped process groups are not containment; same UID is not a sandbox.

## Evidence and reproduction

Use [the final validation instructions](testing.md#phase-8-reproducible-final-validation).
Exact commands, hashes, results, skip reasons and baseline/performance comparisons
are in [Phase 8 evidence](evidence/phase8-validation.json). Historical milestone
records remain independently linked from the M1/M2/M3 and extension readiness
notes. This final run does not retroactively turn unrun historical gates into
passing milestone releases.

## Historical Phase 8 Linux validation result

All 40 source/artifact-bound runtime, guard, ZIP install/completion and isolated
performance checks pass across normal, race, GNU ZIP and static-musl ZIP binaries.
This includes 13 extension groups and 14 dashboard groups per binary, 19 managed
runtime and 14 hook/event/wait groups (real provider initialization skipped).
Go normal/race/minimum-version, lint, module verification, tidy diff and four fuzz
targets pass. Fifteen guard/validator tests pass. No dependency change was needed.
Actual ZIP ELF inspection confirms static musl linkage. Original ignored build
outputs/dependency checkout and real user resources remain untouched.

| Fixed budget | Observed | Budget |
| --- | ---: | ---: |
| CLI startup median | 4.70 ms | 20 ms |
| List 100 sessions | 98.06 ms | 2000 ms |
| Peak native server RSS | 25,848 KiB | 32,768 KiB |
| Dashboard search / resize / quit, 1000 rows | 22.72 / 249.31 / 20.66 ms | 500 ms each |
| Dashboard RSS, 1000 rows | 31,248 KiB | 65,536 KiB |

Historical Phase 0 startup/list/PTY samples and the isolated final sample are
retained for comparison in the evidence index. PTY throughput is descriptive:
no earlier frozen throughput SLA existed. Race RSS stays diagnostic only.
Earlier Phase 7 registry-absence-as-orphan reasoning is superseded by the
instance/receipt recovery fix; historical evidence is preserved rather than
rewritten. The superseded initial Phase 8 run is explicitly failed/interrupted,
not passing evidence. Release criteria above remain open despite passing Linux
checks.

## Latest Linux follow-up

See [Linux follow-up and review handoff](linux-followup.md) for container
isolation, explicit metadata pruning, provider limits, reproduction and proposed
review boundaries. The new evidence is separate from the historical Phase 8
manifest; no historical gate is retroactively certified.


The follow-up passes 44 full runtime/install/performance checks across normal,
race, GNU ZIP and static-musl ZIP binaries, plus 36 real container groups and one
offline Claude initialization group. Thirteen Go/current/minimum-version/race/
lint/module/build checks, four fuzz targets and 15 harness guards pass. Actual
ZIP linkage and pruning completions are verified. Source, harness and build
fingerprints match the disposable archive checkout. All original phase evidence
remains unchanged. Exact results are in
[Linux follow-up evidence](evidence/linux-followup-validation.json).

| Fixed budget | Latest observed | Budget |
| --- | ---: | ---: |
| CLI startup median | 6.10 ms | 20 ms |
| List 100 sessions | 111.66 ms | 2000 ms |
| Peak native server RSS | 25,700 KiB | 32,768 KiB |
| Dashboard search / resize / quit, 1000 rows | 33.49 / 206.73 / 20.59 ms | 500 ms each |
| Dashboard RSS, 1000 rows | 30,992 KiB | 65,536 KiB |

The CLI/native performance sample ran after all concurrent runtime and container
workers finished. Dashboard values are per-suite diagnostics under the recorded
budgets. Cleanup found no remaining exact validation binary processes or owned
containers; HEAD/staging and the original ignored Ghostty checkout are preserved.
No milestone release, staging, commit, PR or publication occurred.
