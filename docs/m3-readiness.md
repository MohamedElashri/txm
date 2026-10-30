# M3 readiness

Phase 6 implements a separate Linux development dashboard on the existing M1/M2
contracts. The explicit request authorizes this work while preserving earlier
release blockers. **M3 release remains Blocked** by inherited unrun M1/M2 platform
gates. No optional worktree/SSH aggregation, commit, tag, push or publication is
part of this task.

`dashboard` uses the already pinned tcell v2.13.10 and uniseg v0.4.7 directly;
versions and module checksums are unchanged. Bare `txm` remains the existing
fuzzyfinder picker. Default dashboard refresh reads all local adapters; explicit
--backend is strict. Only --agents starts/subscribes to M2. Session and
registration rows remain separate, with no name-based ownership inference.

| Platform/backend | Dashboard evidence and supported actions |
| --- | --- |
| Linux amd64, native with CGO | Actual PTY/VT reads, bounded preview, control/read-only attach, instance validation and TTY restoration |
| Linux amd64, tmux 3.4 | Actual isolated server list/preview, exact ID attach/read-only and return/cancellation; IDs remain current-server scoped |
| Linux, Screen/Zellij | Typed list/error and unsupported-action fixtures; real installed runtimes unverified, dashboard actions disabled |
| Linux without CGO | Buildable; native/coordinator unavailable, external adapters retain their own limits; non-TTY refusal verified |
| macOS | Common UI implementation; dashboard runtime disabled pending ancestry/TTY validation; native CGO/runtime/archives/install unverified |
| Linux arm64 | Runtime/archive/install unverified |
| Windows amd64/arm64 | Build-only; dashboard and native/private runtime explicitly unsupported; external backend runtime unverified |

Agent capability/provider limits remain those of [M2 readiness](m2-readiness.md):
generic operation needs no provider, real Claude coverage is initialization only,
Codex has a manual schema/config bridge with unrun trusted runtime, and OpenCode
v2/other versions are unsupported. A dashboard cannot increase those guarantees.

The dashboard displays lifecycle, sourced attention, optional turn completion and
receipts independently. Stale rows become unknown and cannot authorize actions.
Missing heartbeats are stale after five seconds. Session action targets are
revalidated and native instances pinned. Screen/Zellij and registration-only rows
cannot authorize attach or preview. No create/delete/input-injection action,
provider permission bypass, raw output log or new terminal renderer is added.

Refreshes are independent/cancellable, with two-second request budgets. Preview
is selected-session only, plain, at most 16 KiB/100 visible lines, single-worker
and generation checked. Display/storage are bounded with explicit truncation.
Search, keys, resize and quit stay responsive during backend delays. UI workers
stop before attach, and the screen/TTY resumes after return. A separate terminal
attribute/file-flag guard covers external client cancellation as well as normal
screen cleanup. CPU/memory attribution for agents remains omitted.

Final checks, source/artifact hashes, commands, measured budgets and the synthetic
recording are indexed in [Phase 6 evidence](evidence/phase6-validation.json) and
[testing](testing.md). The [short recording](evidence/phase6-dashboard.cast) uses
only fixture labels/output and ephemeral IDs; no real project, prompt, credential
or provider model request is captured. Existing SSH helper examples are
illustrative only; no remote host was contacted.

Release blockers are unchanged: unrun macOS, Linux arm64 and Windows external
runtime/archive/install checks and foreign-UID end-to-end rejection remain in
[M1 readiness](m1-readiness.md). Configured CI, local snapshots and cross-builds
are not substitutes. No M1/M2/M3 release is certified by Linux development alone.

Linux ancestry/controlling-TTY inspection is a deny-only guard against attaching
or previewing the dashboard’s own native PTY. It is not an agent state parser,
PID-based recovery, or an identity grant. Incomplete /proc visibility disables
native actions. Non-Linux runtime is conservatively disabled until its matching
TTY guard is verified; this does not waive the required macOS release gate.

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

## Validation result

Linux implementation/validation passes: Go 1.26.10/1.27.3, Go race, zero-issue
lint and module verification; 10 isolation guards, all retained M1/M2 suites,
and 14 dashboard PTY groups normally/race and from actual GNU/musl ZIPs. Both
ZIPs pass private install/content/completion checks. Windows and macOS
amd64/arm64 CGO-disabled builds pass with build-only scope; no runtime support
is inferred. Three unchanged framing/plain-output fuzz targets pass 30/10/15 s.

The fixed 1000-session budgets pass: normal search/resize/quit
33.57/283.47/20.76 ms and 30,604 KiB RSS (limits 500 ms / 65,536 KiB). Race
latency checks pass with memory measured diagnostically. Isolated retained
1/10/100-session budgets also pass: startup 6.91 ms, list100 103.10 ms and
25,504 KiB maximum idle server RSS. These samples do not establish an agent
resource-attribution or production SLA. The final recording is synthetic only.

One final dashboard-model-only backend display filter change followed the full
integration pass. Go/minimum/race/lint/build checks and all 14 dashboard groups
normal/race/GNU/musl were repeated, as were final ZIP install/completion checks.
M1/M2 sources stayed unchanged. Exact sequencing is in the evidence index.

Latest Linux audit, recovery/protocol hardening and final validation are recorded
in [the Phase 8 audit](final-audit.md). Historical results above retain their
original scope; unrun release gates remain Blocked.
