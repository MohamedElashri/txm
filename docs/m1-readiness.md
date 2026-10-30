# M1 release readiness

Status: **Blocked for release**. Phase 3 profile implementation is complete on the
tested Linux amd64 host. This records a local validation gate, not a published
release or approval to proceed to M2. See the [ledger](../plan.md),
[validation evidence](evidence/phase3-validation.json) and [test record](testing.md).

| Gate | Evidence and boundary |
| --- | --- |
| Native framing, lifecycle, observer authorization and migration | Retained real PTY/adversarial checks, Go race and frame fuzz on Linux. Legacy runtime paths remain untouched. macOS execution and a foreign-UID end-to-end check are unverified. |
| Backend targets, capabilities, JSON and errors | Retained structured/error suites and real tmux 3.4 checks on Linux. Screen/Zellij exact pane/profile operations remain unsupported; fake commands prove refusal, not upstream runtime support. |
| Declarative profiles, R06/R11 | Strict preflight, redacted dry-run, exact returned IDs, index 0/1, cwd/layout, shell input, focus changes, existing/read-only policy, owned rollback, partial failure and cancellation tested on Linux. Native supports one terminal; tmux profiles require 3.4+. |
| Human workflows | Retained aliases/CLI checks, real native picker attach/detach and terminal restoration, real tmux default profile attach/detach, isolated user installation and generated completions. |
| Resource comparison | Fixed provisional startup/list/RSS budgets pass at 1/10/100 private idle native sessions. Attach/Debug PTY throughput samples are comparisons, not a production throughput SLA. |
| Packaging | Current GoReleaser configuration validates; actual local Linux amd64 GNU/musl snapshot ZIPs include binary, man page, README/license and four completions. Archive binaries are tested, including private user installation. These are narrowed local snapshot builds, not an execution of the all-platform release workflow. |
| Other claimed platforms/archives | **Blocked**: macOS amd64/arm64 real runtime and archives, Linux arm64 GNU/musl runtime and archives, Windows archive/install/runtime smoke. CGO-disabled Windows cross-builds only establish compilation with native unavailable. |

The mandatory unrun platform/archive gates prevent a whole-M1 release claim.
The configured macOS CI job is a procedure awaiting execution, not evidence.
Before release, run those hosts/artifacts through their applicable native PTY,
tmux, structured/profile and installation smoke checks, record versions and exact
artifact hashes, and keep declared unsupported capabilities explicit. A Windows
pass must demonstrate the documented native-unavailable behavior rather than
imply native persistence. Also close the unrun foreign-UID end-to-end credential
check before claiming that security scenario passed.

Phase 4 has subsequently been explicitly requested for Linux development; its
supervisor/coordinator evidence is recorded separately in
[phase4-validation.json](evidence/phase4-validation.json). This does not close the
M1 platform release gate. Phase 5 hooks/events/wait, the M2 release gate, M3 and
optional extensions remain pending. No coordinator or publication was introduced
in Phase 3; no release or platform pass is implied by the later development work.

Latest Linux audit, recovery/protocol hardening and final validation are recorded
in [the Phase 8 audit](final-audit.md). Historical results above retain their
original scope; unrun release gates remain Blocked.
