# Implementation Plan

Private implementation ledger for the current task. Keep this file current, specific, and executable. The checkboxes and evidence below are the authority for progress; do not report a phase complete because code compiles or because a feature exists behind an untested flag. Update Status, Completion notes, and Deviation notes after each PR. Keep detailed protocol/API specifications in dedicated repository documents (identified below), rather than allowing the ledger to become a second implementation.

Repository: https://github.com/MohamedElashri/txm\
Planning baseline: `main` at `f2606d240912ffa2269569cd642a7f83e5457f74`, inspected on 2026-10-29. **Revalidate SHA and working tree before implementation.**\
Template: https://github.com/MohamedElashri/agentic/blob/main/templates/plan.md\
Execution mode: autonomous, incremental implementation; no push, merge, release, or destructive action on real user resources without authorization.

**Current task:** Completed explicitly requested Linux follow-up: close locally feasible foreign-UID IPC, real isolated loopback SSH and backend runtime gates; reconcile handoff and preserve unavailable macOS/native ARM64/Windows and provider limits. Use disposable containers with read-only source/binary mounts and private fixtures; no host account/service changes, real user resources, staging, commits or publication.

## Goal

Make **txm** a reliable, lightweight, programmable terminal session manager with **opt-in agent awareness**, preserving its native, tmux, Zellij, and GNU Screen strategy. Provide trustworthy state, structured machine access, session observation, managed process completion, agent attention notifications, and a fast TUI without building a new terminal emulator or replacing users' existing tools.

**Positioning:** Universal terminal session management for shells, remote workflows, and coding agents.

**Shipping milestones:**

| Milestone | Required outcomes | Release rule |
| --- | --- | --- |
| **M1 - Reliable programmable txm** | Secure native IPC and lifecycle; correct CLI errors; typed backend contracts; capabilities, strict targeting, versioned JSON, bounded output; profile correctness | Independently releasable, useful without agents or daemon |
| **M2 - Agent-aware txm** | Opt-in supervised command runtime, agent registration/launch, durable identity and exit receipts, explicit local state owner, verified hooks, events, wait, optional notifications | Independently releasable after M1; **no TUI dependency** |
| **M3 - Agent dashboard** | Efficient interactive session and agent TUI, built on M1/M2 contracts; documentation and ergonomics | Separately releasable; preserve existing `txm` picker |
| **E1 - Git worktrees** | Optional isolated task/worktree lifecycle | Separate opt-in extension; must not delay M1-M3 |
| **E2 - Remote aggregation** | Optional SSH-based discovery and attach | Separate opt-in extension; must not delay M1-M3 |

The **mandatory project goal** covers M1-M3. E1/E2 are explicitly deferred unless the requested implementation scope includes them. An agent must stop at each release gate to validate and record evidence, not silently push on to the next milestone.

## Scope

**In**

- Correct the native PTY server's protocol framing, socket lifecycle, permissions, concurrent client behavior, read-only enforcement, child termination, and attach/resize behavior.
- Make errors and exit codes truthful. Preserve conventional commands, aliases, session picker, shell completions and behavior except for documented corrections.
- Keep existing four backend adapters, but explicitly model unsupported, degraded, and version-dependent operations; do not promise uniform pane/window semantics.
- Separate backend data models from CLI formatting. Provide stable session locators, durable managed IDs, `--json`, safe bounded output inspection, and deterministic backend selection.
- Make declarative TOML profiles verifiable and predictable, including requested backend, working directories, layouts, command semantics, preflight validation, partial failures, and dry-run.
- Add **opt-in** supervised process launching and a local agent-state coordinator, provider-neutral integration, supported Claude Code/Codex/OpenCode hooks, safe live events, waiting and optional notifications.
- Provide a responsive TUI consuming the same structured state as CLI.
- Add cross-platform/backend integration tests, release smoke tests, migration documentation, and clear advertised feature support.
- If independently authorized, add safe task/worktree and SSH aggregation extensions.

**Out**

- Reimplementing tmux/Zellij layouts inside native; changing terminal rendering; a desktop/web application; integrated LLM chat, prompt orchestration, inference, token billing or PR-review UI.
- Inferring successful command execution from injected keystrokes, shell prompts, regexes or terminal colors.
- Migrating running PTY processes across backends, or resurrecting arbitrary processes from output snapshots.
- Default transcript retention, background polling when agent features are unused, automatic dangerous agent permissions, or public network listeners.
- Automatic commit, push, merge, deletion of dirty worktrees, or running repository-supplied setup scripts without explicit opt-in.
- Promising parity on Screen, Zellij, or Windows where upstream APIs or platform runtime do not support it.

## Constraints

1. **Compatibility and minimalism.** Preserve existing CLI paths (`txm`, `create`, `attach`, `list`, `session`, `window`, `pane`, `exec`, `profile`, `config`) and default human outputs where practical. Correct misleading success semantics even if scripts that relied on bugs change. Add tests and migration notes for intentional changes. Prefer standard Go library and the existing Cobra, PTY, libghostty-vt and TOML stack.
2. **One concern per PR.** Changes must be independently reviewable and testable. Avoid broad refactors, dependency churn, hidden feature expansion, or wholesale GUI/multiplexer rewrites. Finish each gate before implementing dependent work.
3. **Backend honesty.** The new explicit `--backend` must never silently fall back; it does not exist in the baseline. Unsafe exact pane/window targeting is `unsupported`, not best-effort success. Screen/Zellij limitations must be represented as data. Preserve default auto-selection precedence; document whether `TXM_DEFAULT_BACKEND` and config remain preferences that permit fallback.
4. **Security.** Use owned, permission-restricted local runtime paths; validate socket peers/permissions where the OS permits; reject malformed frames and requests; avoid symlink races. Same-UID processes are not a sandbox or security boundary. Keep control and observer permissions distinct; never log prompts, environment secrets, or raw terminal output unless explicitly enabled. Do not expose an HTTP listener.
5. **Truthful semantics.** Distinguish session existence, client attachment, process running/exited, agent attention, agent turn completion and command success. Unknown or stale data must remain `unknown`. `pane exec` is terminal input injection, **not** supervised subprocess execution.
6. **Concurrency.** Context/cancellation and finite deadlines on requests. Per-client bounded queues; no PTY fanout blocked indefinitely by a slow reader. Make disk updates atomic, versioned and crash-recoverable. Never use process ID alone as a durable identifier.
7. **Portability.** Linux and macOS are the initial required native runtime targets. Keep Windows builds and external-backend support truthful; either implement native Windows transport + ConPTY/PTY path with real tests or mark native Windows unsupported. Builds are not equivalent to runtime evidence.
8. **Safety of autonomous tests.** All tests use isolated HOME, XDG dirs, runtime, repository, sockets, processes and backend instances. Never test `nuke`/cleanup against the user's default tmux socket or real repositories. If an adapter cannot be isolated, skip its destructive integration suite and record the blocker.
9. **No hidden permissions.** Agent launch does not imply sandbox bypass, unreviewed approvals, access to secrets, or arbitrary hooks. Provider events require verifiable local association to an opted-in agent.
10. **No fabricated passing gates.** Unsupported hosts, unavailable tools, mismatched dependencies and skipped tests must be recorded. If the environment blocks required validation, mark the milestone Blocked, provide reproduction steps, and stop release work.

### Fixed architectural decisions (do not redesign without recorded evidence)

**A. Two operating modes.** Existing session CLI remains **daemonless except for its native per-session PTY servers**. M1 works without a new always-on service. M2 introduces an **opt-in, per-user, local-only coordinator** (`txm internal coordinator`, actual CLI implementation may be hidden) that runs only while agent management is enabled/needed. It owns normalized live agent state, authenticated hook intake, and event fanout. Never require it to list or attach to ordinary sessions.

**B. Supervised execution.** A small **`txm internal supervised-run`** executable wrapper runs *inside* a newly created session's PTY, launches a child from a structured argv manifest, waits/reaps it, and atomically records a durable exit receipt (`exit_code` or termination signal, instance ID, timestamp). All managed backend launchers use it where proven supported; a coordinator restart cannot erase a recorded exit. Never inject a command into an arbitrary shell and then pretend it was supervised. For tmux/similar interfaces that require a command string, use a securely shell-quoted, fixed launcher path plus a validated opaque manifest ID; user arguments reside only in a mode-0600 manifest, never in interpolated shell text. If this cannot be guaranteed for a backend, mark `managed_launch=unsupported` there.

**C. State ownership and recovery.** The coordinator is the **sole writer of agent registry/state snapshots** under a versioned, private state directory. The supervised wrapper is the **sole writer of its exit receipt**, using an atomic rename; it attempts best-effort notification to the coordinator. Provider hook clients submit signals to the coordinator only. On restart, the coordinator reloads the registry and exit receipts, validates instance identity/liveness, and reconciles uncertainty to `unknown`; it never fabricates an exit code. Coordinator crashes must not terminate a managed PTY process. Do not automatically relaunch a terminated agent.

**D. Event semantics.** In M2, events are **ephemeral, best-effort notifications**, not a durable audit log. The coordinator emits monotonically ordered sequence numbers within a running coordinator epoch and bounded per-subscriber queues. An epoch change, queue overflow or reconnect requires `resync_required`, then a fresh `status`/`list` snapshot. `txm events --jsonl` streams only safe metadata, never transcripts. No exactly-once or historical replay claim. `wait` uses an initial authoritative snapshot plus event subscription, with resynchronization and bounded periodic reconciliation to avoid races; no busy-spin.

**E. Identities.** Each **managed session/agent** gets a random, opaque, persisted 128-bit ID generated with `crypto/rand`, plus a separate mutable locator `{host_id, backend, name}`. A rename changes the locator, not the managed ID. An **unmanaged** session has a locator, not a fabricated durable identity; name reuse can create a different process. Persist a random `host_id` at first M1 structured-state use (no coordinator required). Short names are accepted only when unambiguous; destructive commands require resolution to a single exact backend/session target. Remote host names/aliases do not replace host IDs.

**F. Schema and process contracts.** Freeze M1 schemas and error codes in `docs/contracts.md` before shipping JSON. Specify and freeze M2 launcher/hook/event/wait additions before their respective implementation PRs; Phase 0 need not freeze unverified provider APIs. Preserve human-oriented CLI defaults. A managed process `exited` state is not the same as an agent `turn_complete` or `needs_input`. The agent's status source and observation timestamp are required whenever an agent state is reported.

**G. Platform support.** Prioritize Linux and macOS. Continue releasing other target binaries only after at least appropriate build checks, and do not advertise native Windows persistence without actual runtime validation. Future platform work must not weaken Unix protocol correctness.

### Required design documentation generated during implementation

- `docs/architecture.md`: diagrams/ownership, coordinator vs native PTY server, supervisor lifecycle, ID/locator model, recovery, trust boundaries, resource limits.
- `docs/native-protocol.md`: framing/encoding/version negotiation, socket discovery, connection privileges, request IDs, errors, migration from legacy sockets, per-client backpressure.
- `docs/contracts.md`: normative JSON envelopes and field definitions; capability and status enums; CLI exit codes; managed launcher/hook/event/wait contracts; schema compatibility rules.
- `docs/testing.md`: isolated fixture design, exact backend launch flags, process teardown, CI matrix, manual smoke steps, release evidence.

These are implementation deliverables, not prerequisites the user must author separately. None of these four documents exists at the reviewed SHA. Phase 0 establishes their M1 content and M2 ownership decisions; later phases extend the relevant contracts before dependent code. Keep detailed specifications in those documents rather than duplicating them here.

## Current State

The following findings were checked against source at the baseline SHA on 2026-10-29. **Source-confirmed means the code path was inspected, not that an integration reproduction passed.** No real session was created, attached, killed or queried during this review.

| Area | Relevant files | Source-confirmed state / required validation |
| --- | --- | --- |
| Backends | `pkg/backend/backend.go`, `{native,tmux,zellij,screen}.go` | A broad `TerminalMultiplexer` interface mixes actions that are unsupported on some backends with methods that print directly. No typed, introspectable capability model. |
| Selection/config | `pkg/config/config.go`, `pkg/backend/backend.go`, `pkg/cmd/root.go` | Default preference is tmux; configured fallback order is tmux, Screen, Zellij, native. Ultimate fallback differs: tmux, Zellij, Screen, native. Config searches `~/.txm/config`, `~/.txm/config.txt`, `~/.txmrc`; XDG config is not used. `getManager` can pass nil config to `NewManager` after a load error at verbosity 0. |
| Native transport/startup | `pkg/backend/native.go`, `pkg/cmd/server.go` | Both use `os.TempDir()/txm-<name>.sock`; byte-leading protocol has no framing. Server blindly removes the path. Creation waits only for path existence for about one second, without readiness/instance acknowledgement or child-exit check. Hidden `server` also accepts a name without ordinary CLI validation. |
| Native control/input | `pkg/backend/native.go`, `pkg/cmd/server.go` | `KillSession` sends initial `0x03`; `Exec` sends initial `0x01`. Server initially accepts only `0x04` status, `0x05` dump or `0x00` attach, so both requests are rejected. Kill ignores write errors; Exec can report a successful write without PTY input. Kill removes a socket on dial failure without verifying staleness. |
| Native concurrency/lifecycle | `pkg/cmd/server.go`, `pkg/backend/native_unix.go` | Broadcast serializes all client writes under a mutex with 50 ms per-client deadlines; initial screen dump has no write deadline. PTY output goroutine is not joined before terminal/log teardown; kill targets the direct child only. SIGWINCH registration/goroutine has no stop path; attach input goroutine can remain blocked on stdin; read-only attach has no local detach-key reader. |
| CLI errors/output | `pkg/cmd/{root,session,window,pane}.go`, `pkg/logger/logger.go` | Session/window/pane handlers often log errors then return nil. Logger errors/warnings go to stdout; `Execute` prints errors to stdout in addition to Cobra's error handling. Picker hides discovery failures as help. Global manager/flags and `TXM_LOG_FILE`/`TXM_READ_ONLY` changes require isolation in in-process tests. |
| Exact targets/input | `pkg/backend/{tmux,zellij,screen}.go` | tmux constructs `session:window.` for an empty pane; targets use backend name matching and `send-keys` lacks literal-text mode. Zellij close-tab/split ignore requested window and pane operations use best-effort focus. Screen rename/split/Exec ignore some target arguments; session lookup/kill uses substring matching. |
| Discovery/availability | `pkg/backend/{native,tmux,zellij,screen}.go` | Native treats matching paths as live sessions and always reports available, including Windows. Zellij matches session substrings. Screen discards discovery/kill failures. tmux/Zellij availability accepts absolute fallback paths but execution still uses the bare executable name from PATH. |
| Read-only | `pkg/cmd/session.go`, `pkg/backend/{native,tmux,zellij,screen}.go` | Flag sets a client environment variable only; native suppresses stdin but still sends resize. Server has no observer role; other adapters ignore the variable. Attach may create a missing session even when read-only was requested. |
| TOML profiles | `pkg/cmd/profile.go`, `pkg/config/profile.go` | `backend`, `dir`, `layout` are parsed and unused; loader uses permissive TOML unmarshal. Profile bypasses ordinary name/prefix handling, guesses initial window by renaming both `0` and `1`, ignores rename errors and only warns on split/Exec failure. Existing session attaches immediately; successful apply always attaches. |
| Output/picker | `pkg/cmd/root.go`, `pkg/backend/{native,tmux,zellij,screen}.go` | Picker synchronously calls `DumpSession`. Native reads unbounded socket output with ignored read errors/no deadline; tmux captures a pane; Zellij/Screen return placeholder text with nil error. No structured output/history API exists. |
| Build/releases | `go.mod`, `Makefile`, `.github/scripts/build-ghostty.sh`, workflows, `.goreleaser.yaml` | Go 1.26.0 minimum, CGO/libghostty required for command package. Makefile pins Ghostty `d4c88d806` only on first clone and trusts prefix-directory existence. Release script clones without pinning/checking out that revision and writes eight target prefixes under `/tmp`. Workflows select Zig 0.16.0; CI tests only Ubuntu. |
| Coverage/new features | `pkg/cmd/root_test.go`, `pkg/config/{config,profile}_test.go` | Exactly three test files: name validation, backend parsing, TOML parsing. No backend tests or integration harness; no JSON/capability/coordinator/supervisor/agent/event/wait/dashboard implementation. |
| Existing docs | `README.md`, `docs/{user-guide.md,ROADMAP.md,txm.1}` | Guide says nuke spans all backends, but code affects only the selected one; read-only is overstated. Roadmap includes cloud sync and output-triggered notifications beyond this plan. Reconcile with M1-M3; cloud sync is deferred and output heuristics must not establish command/agent success. |

**Existing command surface:** bare `txm` picker; `session create/list/attach/detach/delete/rename/nuke` and hidden `dump`; root aliases `create/list/attach/detach/delete/rename-session/nuke/exec/dump`; `window new` (alias `create`), `list/kill/next/prev/rename/split`; `pane list/kill/exec`; `profile <file>`; `config set/get/show`; completion, version, installation/update/removal and `generate-ssh-config`; hidden native `server`. No `--backend`, `--json`, `inspect`, `output`, `agent`, `events` or `dashboard` yet. Keep installer/updater outside feature scope except relevant packaging/documentation corrections.

**Review evidence and limits (2026-10-29):**

The bullets below preserve the initial planning review. Phase 0 subsequently
resolved the build blocker through verified dependency alignment; current evidence
and remaining baseline failures are in `docs/testing.md` and `docs/evidence/`.

- Host: Linux amd64; Go `1.27.3` (module minimum `1.26.0`), Zig `0.17.0`, tmux `3.4`. Zellij and Screen are absent from PATH. golangci-lint, GoReleaser and pkg-config are available; their presence is not a passing check.
- Existing ignored `ghostty/` is clean at `d4c88d8069912b653d707191388ca98e24751f12`; its `build.zig.zon` declares minimum Zig `0.16.0`. `.ghostty-host/` is absent. Existing ignored `bin/` is not assumed to match HEAD and was not executed.
- **Pass:** `GOCACHE=/tmp/txm-plan-review-go-cache GOTOOLCHAIN=local GOPROXY=off go test ./pkg/config ./pkg/backend`. Config tests pass; backend package has **no test files**. This is compilation evidence only for adapters.
- **Blocked:** `PKG_CONFIG_PATH=/home/melashri/projects/txm/.ghostty-host/share/pkgconfig GOCACHE=/tmp/txm-plan-review-go-cache GOTOOLCHAIN=local GOPROXY=off go test ./pkg/cmd`: pkg-config cannot find `libghostty-vt-static`. Full build/tests, lint, race/fuzz, PTY and release checks were not run. Installed Zig differs from CI; compatibility has not been tested. Establish a verified Ghostty/Zig pair in Phase 0 before runtime claims.
- Repository instructions search found no applicable `AGENTS.md`. Source inspection, config tests and tool inventory are complete for this planning review; Phase 0 and M1-M3 remain unfinished.

**Explicit source-driven regression list:**

- R01: native kill actually kills the server/child and removes only its own socket; other sessions survive.
- R02: native duplicate creation cannot unlink, replace or hijack an active socket; stale socket recovery does not kill a live process.
- R03: unsupported/failed commands return nonzero CLI exit codes and do not print success.
- R04: requests on a native observer connection cannot write, kill or perform disallowed resize; local detach remains usable; supported tmux read-only attach is properly enforced. This does not prevent the owning UID from opening a separately authorized control connection.
- R05: Zellij and Screen name `foo` does not match `foobar`; duplicate Screen names and tmux prefix matches cannot cause wrong-session actions.
- R06: multi-pane profile commands run in the intended pane or fail before creation; never silently target a different pane.
- R07: native stream accepts fragmented/coalesced frames, never interprets partial payloads as commands, and bounds allocation.
- R08: M1 attach/detach preserves sessions and legacy runtime paths remain untouched; M2 extends this with coordinator failure/restart independence. The coordinator portion is not an M1 prerequisite.
- R09: native Exec reaches the PTY through an acknowledged input request; command strings such as `Enter`, `C-c` and leading dashes are sent literally by tmux. Input accepted is never reported as child command completion.
- R10: unreadable config at verbosity 0/1 returns a documented error or safe fallback without panic; aliases/completion/version paths do not accidentally depend on a live backend. Invalid config preferences and strict backend requests have explicit, distinct policies.
- R11: profile honors name/prefix policy and captures actual window/pane IDs; tmux `base-index`/`pane-base-index` 0 or 1, focus changes and preexisting session policy cannot redirect input or silently ignore fields.
- R12: native startup distinguishes readiness, duplicate-instance races, failed exec and short-lived child exit; no launcher claims a live session from socket existence alone. M2 additionally records short-lived child receipts even when the session vanishes before discovery.

R01-R12 are source-driven test requirements, **not passing regressions**. Build the reproductions under the guarded harness in Phase 0 and corresponding implementation PRs; never reproduce the unsafe legacy operations on default runtime paths.

## Commands

**Execution discipline:** Recorded Phase 0 commands/results are in `docs/testing.md`; the examples below are not an assertion that every gate passed. First use the isolated sandbox and inspect dependency recipes. `make build-deps` now checks the shared source/Zig pins and asks Zig to validate artifacts on every invocation. Never repoint or clean the existing ignored `ghostty/` or `bin/` blindly; use a disposable checkout and prefix. Tests of optional features only run after implementation. Never run `nuke`, installer/updater, `make clean`, untrusted setup hooks, or release commands on active user resources.

```bash
# orientation
pwd
git status --short
git rev-parse HEAD
go version
zig version || true
command -v tmux || true
command -v zellij || true
command -v screen || true

# dependency/build: ONLY in a disposable checkout after verifying
# Ghostty revision, compatible Zig version, private prefix and temp paths.
# Local and release scripts share scripts/dependencies.env after Phase 0.
make build-deps
make build
./bin/txm version
./bin/txm --help

# standard project tests and static checks
make test
make lint
git diff --check

# after introducing targeted test packages, run in that same sandbox
# with the verified prefix (prefer explicit Makefile integration/race targets).
# Phase 0 added test-harness, test-integration and test-race targets.
# Add the framing fuzz target with the parser in PR 1B.
# PKG_CONFIG_PATH="$GHOSTTY_HOST_DIR/share/pkgconfig" CGO_ENABLED=1 go test ./pkg/... -run 'TestNative|TestProfile|TestCLI' -count=1
# PKG_CONFIG_PATH="$GHOSTTY_HOST_DIR/share/pkgconfig" CGO_ENABLED=1 go test -race ./pkg/... -count=1
# go test ./pkg/nativeprotocol -fuzz=FuzzNativeFrame -fuzztime=30s

# planned CLI contract smoke tests (unavailable until corresponding PRs)
# ./bin/txm backend capabilities --json
# ./bin/txm session list --all-backends --json
# ./bin/txm session output <exact-target> --lines 100 --plain
# ./bin/txm agent status <managed-id> --json
# ./bin/txm events --jsonl
# ./bin/txm dashboard --help

# after all applicable release checks pass and only in an isolated checkout
# goreleaser check
# goreleaser release --snapshot --clean
```

In `docs/testing.md`, define a **single guarded test harness** with disposable `HOME`, `XDG_CONFIG_HOME`, `XDG_STATE_HOME`, `XDG_RUNTIME_DIR`, `TMPDIR`, a controlled executable `PATH`, isolated native socket namespace, and uniquely named child processes. Set separate build caches/toolchain paths explicitly so an isolated HOME does not trigger unexpected downloads. Clear inherited `TMUX`, `STY`, Zellij context and all `TXM_*` overrides; then set only fixture-owned values. HOME matters because baseline config ignores XDG paths; TMPDIR matters because legacy native sockets use `os.TempDir()`. Do not rely on HOME alone for backend isolation:

- **tmux:** dedicated `-L txm-test-<random>` or `-S <temp-socket>` plus `-f /dev/null` at test server startup; socket selection on **every** invocation, including availability/probe/discovery/capture/cleanup helpers. Baseline hardcodes `exec.Command("tmux", ...)` in multiple paths; add a narrow executable/argument injection seam or guarded wrapper covering every path before integration. Availability checks of absolute paths must not bypass isolation. Verify server identity before calling `kill-server`.
- **Zellij:** use an independently verified isolated socket/data dir mechanism for the specific version. If not demonstrably isolated, run only non-destructive tests and record blocker; never delete default user sessions.
- **Screen:** verify isolated `SCREENDIR` behavior/permissions for installed version, or skip destructive tests; guard every target.
- **Native:** private TMPDIR for legacy reproductions, private runtime root and immutable opaque test IDs for the new protocol; build an actual txm executable because `CreateSession` relaunches `os.Executable()` as `server` (a Go test binary is not a substitute). Verify socket ownership and daemon PID/instance before cleanup.
- **Git/SSH extensions:** temp Git repositories and fake/loopback SSH fixtures; no real remote host required for destructive tests.
- **CLI fixtures:** reset global Cobra flags/manager/logger and restore environment between in-process tests; prefer subprocess tests for exit codes, raw TTY behavior and native self-launch. Introduce a command constructor only if needed for isolation, without a wholesale CLI rewrite.
- **Cleanup:** test-owned resources only; process groups + timeouts; never remove an unverified socket or orphan blindly. CI must include at least a native real-PTY and isolated tmux integration job; test config and flags must never reach the default mux server.

## Acceptance Criteria

The checked conditions are independently verifiable gates. A release claim requires concrete output, test fixture or an observed manual run with platform/version details.

### M1 - Reliable programmable txm

Individual checks below apply to the tested Linux amd64/native and tmux 3.4 scope
(and verified unsupported adapter contracts), as detailed in `docs/m1-readiness.md`.
They do not certify unrun platforms or close the release gate.

- [ ] M1 portions of R01-R12 pass; all existing human workflows/aliases either pass unchanged or have documented intentional fixes. R08 coordinator-loss and R12 durable receipt checks belong to M2.
- [x] Native framed protocol is versioned, handles partial/combined/truncated/malformed frames, bounds memory and clients, and resists slow-peer head-of-line blocking.
- [x] Secure runtime path, socket permissions/peer validation where available, observer authorization, stale-file handling, and daemon/child exit semantics pass tests.
- [x] Old native daemon socket paths are **never unlinked or hijacked** by new clients; old sessions may be finished with a legacy binary. Incompatible versions produce a clear migration error, not unsafe fallback.
- [x] Backend capabilities are tested, truthful and exposed; explicit backend selects only the requested backend; exact-target operations refuse ambiguity/unsafe focus semantics.
- [x] Versioned `--json` envelopes work for success, empty, partial and expected failures. Error diagnostics never contaminate JSON stdout. Exit codes are documented and verified.
- [x] Bounded `session output` distinguishes rendered screen, raw control bytes and true history; unsupported capture fails with explicit error.
- [x] Profiles honor supported `backend`, `dir`, layout and command semantics or reject the profile in preflight; dry-run is truthful; partial creations are disclosed and cleaned up safely.
- [x] M1 works without an agent coordinator or any agent harness installed. Existing session picker remains functional.


### M2 - Agent-aware txm

Checks below apply to tested Linux amd64/native and tmux 3.4, with exact provider
limitations in `docs/m2-readiness.md`. They do not certify unrun M1 platforms or
close the M2 release gate. Claude has real initialization-hook coverage; Codex is
a manually reviewed schema/config bridge with unrun trusted runtime; OpenCode v2
and other versions are unsupported.

- [x] Agent coordinator starts **only on opt-in agent workflows**, runs over owned local IPC only, never exposes a network port, and does not affect basic sessions if stopped.
- [x] Managed launcher records true termination status even when coordinator restarts; cannot mistake PID reuse, external session termination or vanished receipts for success.
- [x] Managed IDs survive rename and avoid collisions; unmanaged locators are not falsely advertised as persistent IDs.
- [x] Lifecycle (`starting|running|exited|unknown`), attention (`working|needs_input|idle|unknown`) and optional turn completion are distinct, sourced and timestamped.
- [x] Generic agent workflow works without provider hooks. Claude/Codex/OpenCode adapters are individually tested on documented, actually supported hook versions or marked unsupported.
- [x] `agent wait` handles initial-state races, exit with nonzero status, missing signals, timeout, lost session and cancellation deterministically.
- [x] `events --jsonl` is bounded, private, cancellable and emits `resync_required` on stream gaps/reconnect/epoch changes; event history is not falsely claimed.
- [x] Notifications default off; no unapproved prompts, transcripts, raw command lines or credentials are persisted or broadcast.

### M3 - Agent dashboard

Checks apply to tested Linux amd64/native and tmux 3.4 only, with explicit
unsupported/degraded cases. They do not close the M3 release gate. Non-Linux
dashboard runtime remains disabled pending equivalent ancestry/TTY validation;
see `docs/m3-readiness.md` for all inherited platform and provider limits.

- [x] TUI consumes exactly the same session/agent state contracts as CLI; shows `unknown`/`unsupported` without optimistic badges.
- [x] Keyboard navigation, search/filter, attach/read-only attach, bounded output preview and responsive refresh work under slow/unavailable backends.
- [x] Existing `txm` default picker behavior remains usable, with a separate `txm dashboard` entry point initially.
- [x] TTY restores correctly after interrupt, resize, detach, subprocess failure and non-TTY invocation.
- [x] README/man page/completions/CHANGELOG/support matrix match actual tested functionality; demonstrations contain no invented feature claims.

### Optional extensions (only if selected)

- [x] E1 (Linux): no dirty/untracked worktree is deleted by default; receipt/instance-pinned cleanup and journal-proven never-launched recovery tested. Failed/uncertain operations retain resources for explicit/manual cleanup; no automatic rollback/replay.
- [ ] E2: SSH remote discovery and attach work through existing OpenSSH with strict host/target identity, version checks and no public listener. **Fake SSH/actual endpoint plus real-client configuration checks pass; actual SSH server/network gate remains unrun.**

### Release-quality gates

- [ ] Every required CI, isolated integration, supported race test, schema golden test and manual PTY smoke test has recorded evidence; skipped cases say why.
- [ ] Linux and macOS native runtime claims have genuine testing; Linux static and Windows artifacts are advertised only to the level verified.
- [x] Trusted test fixture isolation and negative/destructive review pass on Linux, including inherited Git-routing refusal; not an OS sandbox against same-UID code.
- [x] Linux final audit fixes evidenced unsafe cleanup; synthetic provenance/known-credential scans, scoped resource checks and documented contracts pass. No real user data, provider conversation or publication; privacy and platform limits remain explicit.

## Test Plan

1. **Orientation and baseline.** Record git SHA, Go/Zig/Ghostty versions, OS/arch, `make build`, `make test`, lint results, current behavior and existing failures before implementation edits. Use a fresh disposable environment. Validate both module minimum Go and current development Go; record Zig compatibility rather than assuming a newer version works. Ensure local/release Ghostty builds use the same revision.
2. **Native framing unit/fuzz.** Split every frame at every byte boundary; concatenate frames; truncated header/payload; invalid version/type/length; zero and oversized frames; malicious input; abrupt EOF; repeated resize; bounded allocations and no command side effects from malformed frames.
3. **Native real-PTY integration.** Detached child, startup readiness/failed exec/short-lived command (R12), resize, alternate screen, UTF-8 boundaries, bracketed paste, Ctrl-key detach (including read-only), concurrent viewers, slow observers, server death, duplicate names, socket ownership, process-group termination, zombie/resource cleanup. Include R01/R02/R04/R07/R08/R09 and use real TTY when required. Coordinate PTY reader shutdown before closing VT/log resources.
4. **CLI golden contracts.** Assert stdout/stderr, JSON schema, missing fields, null/unknown, exact exit codes, partial discovery behavior, aliases and human-output compatibility. Validate R03/R10 at default and verbose settings, Cobra error output once, config failures, and scoped environment restoration. Pure argument/version/completion-generation paths must not probe a live mux unnecessarily.
5. **Backend adapter matrix.** Isolated tmux, native, supported Zellij/Screen versions. Verify exact operation effects, name collision targeting (R05), literal input (R09), read-only, output behavior, accurate capability probe and cancellation. Use fake executables for argv/error tests on absent adapters; these do not establish runtime support. Never count a successful subprocess exit without checking the resulting session/pane state.
6. **Profiles.** Validation and dry-run, backend selection, spaces in cwd/path, prefix rules, base-index 0/1, multiple windows/panes, duplicate targets, unsupported operations, existing session policy, partial rollback. Confirm R06/R11 with a visible sentinel in the exact requested pane, including focus changes. `dir` must apply before the command starts; do not inject `cd` and assume it worked.
7. **Managed runtime.** Fake child exits 0/42, signal kill, failed exec, missing executable, coordinator gone/restart, orphan receipts, stale PID reused, simultaneous launches, duplicate names, renamed session, session killed externally. Tests assert actual exit receipts, not shell prompt detection.
8. **Agent hooks/events.** Fake provider adapters first, then verified installed harness versions. Reject stale/duplicate/cross-agent/unauthenticated hook messages; test bounded fanout, disconnect/reconnect, sequence/epoch, `resync_required`, no transcript leaks and event source timestamps.
9. **Wait semantics.** Immediate-exited before subscribe, transition during subscribe, unknown/stale state, exits nonzero, timeout, SIGINT/cancellation, coordinator reconnect and lost session; no busy loops, CPU spikes or false success.
10. **Dashboard.** Empty/error/large datasets; rapid events; inaccessible/slow backends; keyboard-only interactions; resize and SIGINT; redirects/non-TTY; TTY restoration. Check rendering from public state APIs only.
11. **Optional Git/SSH.** Disposable Git repos (dirty/untracked, symlinks, pre-existing branches, rollback) and fake SSH (quoting, errors, version skew, host identity, interruption). No user's real repo or server used for destructive cases.
12. **Release.** Artifact smoke tests, static/musl checks, man page and completions, archive naming, least-privilege runtime paths, exact documented support. Re-run baseline compatibility suite and adversarial failure cases before every milestone release.

**Platform support gate**

| Target | Minimum required verification before claim |
| --- | --- |
| Linux x86_64 native + tmux | Automated integration + manual real-TTY smoke |
| Linux arm64 native | Runner or explicit manual host smoke for claimed runtime support |
| Linux musl artifacts | Execute actual archived binary and native attach/detach (both architectures if both advertised) |
| macOS arm64 native + tmux | Real runtime/PTTY/reattach tests + release archive smoke |
| macOS x86_64 native | Real runtime smoke if advertised |
| Zellij/Screen | Per-version isolated integration for each advertised capability; otherwise explicit unverified/unsupported |
| Windows x86_64/arm64 | Build and install smoke for advertised binaries; native persistence unsupported until proven with actual PTY/IPC tests |

**Performance:** After build/isolation is available in Phase 0, capture CLI startup, list and attach latency, native PTY throughput and daemon memory on a small fixture; defer the 10/100-session scale runs to M1 integration. Set thresholds from measured baseline before declaring performance acceptable. Ordinary commands must not start, contact or wait for the agent coordinator; avoid claiming mathematically zero overhead. No agent polling when unused. TUI refresh must be bounded and cancellable, including the existing picker preview path.

## Phases

Each PR has a distinct acceptance gate. Status vocabulary: `Pending`, `In progress`, `Blocked`, `Done`, `Deferred`. Keep completion notes short: SHA/PR, exact tests, observed platform, remaining limits. When architecture changes, update this file **and** relevant normative docs and golden tests before writing dependent code.

### Phase 0: Orientation, decisions and safe test foundation

Deliverables:

- [x] Revalidated SHA/worktree and source findings; runtime baseline and conservative support matrix recorded in `docs/testing.md`. Dependency/header mismatch found and resolved for the prep build.
- [x] Shared verified full Ghostty pin and Zig 0.16.0 in `scripts/dependencies.env`, used by Makefile/CI/release helper. Clean source/version/artifact checks replace directory-only cache hits; builds use a disposable checkout and prefix. `go.mod` unchanged.
- [x] Added guarded HOME/XDG/TMPDIR/PATH/native fixtures and tmux wrapper with `-S` on every invocation. Nine guard tests pass, including protected second server, replaced socket/marker and symlink refusal; no default user server is contacted.
- [x] Captured ten desired-behavior baseline failures and five smoke passes in `docs/evidence/phase0-baseline.json`; R01-R12 mapping and procedures in `docs/testing.md`. Native observer/framing and full per-adapter/index matrices explicitly remain unverified until their implementation PRs; M2 portions remain deferred.
- [x] Added `docs/architecture.md`, `docs/native-protocol.md`, `docs/contracts.md`, `docs/testing.md` with M1 contracts, ownership, limits, migration and testing rules. Provider schemas remain scheduled for 5A/5B.
- [x] Captured small-fixture startup/list/socket and real-PTY attach/RSS/throughput baseline. Added macOS integration CI job; it is configured, not run. Linux amd64 only is validated here; release/platform claims remain gated.
- [x] Defined package/PR interfaces, legacy socket coexistence, strict backend/target resolution, JSON fields, error/exit precedence and profile policy before feature implementation.

Acceptance criteria:

- [x] Ownership, supervisor receipts, IPC migration, identity and conservative backend capabilities agree across the four design documents and ledger.
- [x] Fixture isolation is proven for current trusted test paths; known defects have guarded baseline probes or explicitly unverified procedures. Prep changes are ready for review; this is not an M1 release gate.

Status: Done\
Completion notes: 2026-10-29, baseline `f2606d240912ffa2269569cd642a7f83e5457f74`; disposable `make build test`, Go 1.26.10/1.27.3 tests, Go race, nine guard tests and five PTY/tmux smoke checks pass. Baseline probes intentionally exit 1 with ten failures. Exact commands/metrics/limits in `docs/testing.md`; no commit/PR created. Stop before PR 1A.\
Deviation notes: Old Ghostty `d4c88d806` compiles but is incompatible with the existing Go binding. Aligned to its own pinned `36ec90a07fcb58e3033296c06c82438bf30d2e42`, retaining Zig 0.16.0 and all Go dependencies. Proxy required approved curl downloads plus verified Zig package hashes. Baseline lint fails at `native.go:204`; GoReleaser check flags existing deprecated properties. These and reproduced correctness defects keep M1/release gates open, not prep. macOS/other artifacts remain untested; providers and scale benchmarks retain their later phase assignments.

### Phase 1: Correct errors and native protocol/lifecycle (M1)

**PR 1A - error propagation and truthful CLI behavior**

- [x] Fix log-and-return-`nil`, silent backend errors, premature success messages across session/window/pane handlers and aliases; propagate errors swallowed by Screen/native/Zellij discovery/cleanup. Empty sessions must be distinguished from unavailable backend/error, without suppressing failures.
- [x] Fix `getManager` nil-config failure independent of verbosity; define config-load/invalid-preference behavior consistently across `root.go`, `config.go` and `misc.go`. Avoid manager initialization for backend-independent version/help/config/completion generation; dynamic session completion may still query a backend.
- [x] Establish stdout/stderr policy with errors emitted once through Cobra/Execute; logger warnings/errors go to stderr. Preserve human success output and test aliases plus environment restoration for read-only/log flags. First guarantee nonzero failures; implement the full classified exit map in PR 2B.
- [x] Read-only attach must refuse a missing session without creating it. Apply tmux's verified read-only attach option; reject the request on adapters without enforced support. Native connection-role enforcement follows in PR 1B. Test flag aliases and local detach behavior (R04).

**PR 1B - new framed native protocol and migration**

- [x] Specify and implement framed protocol with fixed header `{magic, version, message_type, request_id, payload_len}` (exact width/endian/encoding fixed in `docs/native-protocol.md`), `io.ReadFull` for header/payload, maximum payload/connection limits, and typed replies.
- [x] Separate attach/data/resize from acknowledged control/input requests, including standalone native Exec/Kill (R09/R01); serialize writes per connection; bounded fanout; close slow clients without blocking PTY; maintain consistent VT state and controlled terminal resizing.
- [x] Use secure private runtime directory, authenticated peer checks where supported, server-enforced connection roles; an observer cannot write/kill or resize shared terminal state. Keep local detach-key handling. Same UID can still request a separate control role; do not present observer mode as a sandbox against its owner. Reject unauthorized operations and avoid pathname/symlink races and blind unlink.
- [x] New protocol must use a **new socket namespace**. Legacy paths are `os.TempDir()/txm-*.sock`, not necessarily `/tmp`; report owned legacy candidates conservatively without treating arbitrary sockets as txm. Do not unlink, send new protocol to, or kill legacy sessions. Preserve the existing hidden `server` launch path or migrate it explicitly; new runtime paths must be shared by CLI and server. Validate server names even when invoked directly. Document how to finish an old session with the old binary.

**PR 1C - lifecycle and platform implementation**

- [x] Fix R01/R02/R09/R12: authenticated readiness/instance handshake, live discovery vs stale pathname, failed exec/short-lived commands, natural child exit, process-group kill/interrupt and reaping. Prevent duplicate startup from claiming another server's socket and timeouts from leaving unowned children.
- [x] Stop SIGWINCH registration and attach goroutines on detach/cancellation (including blocked stdin), propagate transport errors, restore raw terminal, and join PTY/output goroutines before closing libghostty/log state. Verify concurrent attach/resize/output ordering and consistent initial snapshot/fanout handoff.
- [x] Make Windows-native capability false if unimplemented; test Linux/macOS runtime where available. Correct capability/help/release claims without disabling independently verified external backends.
- [x] Run framing fuzz/PTY/security/race suites. Gate next PR on tests rather than optimism.

Acceptance criteria:

- [x] Applicable R01/R02/R03/R04/R07/R08/R10/R12 checks and the native-input portion of R09 pass; requests cannot exceed their connection role or corrupt stream framing. R09 tmux literal input belongs to PR 2A; M2-only recovery/receipt checks remain deferred.
- [x] Old native sessions are not disrupted. Expected IPC mismatch has clear error. New protocol handles version skew.
- [x] M1 feature development can rely on safe native runtime and correct CLI exit status.

Status: Done\
Completion notes: 2026-10-29, Phase 1 implementation and available-host validation complete on Linux amd64; baseline SHA unchanged, no commit/PR created. Go 1.26.10/1.27.3 tests, Go race, zero-issue lint, nine isolation guards, five PTY/tmux smoke checks, four CLI groups and 11 native adversarial/concurrency checks pass; the same 11 checks pass with a race-enabled executable. Frame fuzz passes 30 s / 1,609,524 executions. Windows amd64/arm64 cross-builds pass with native unavailable. See `docs/testing.md` and `docs/evidence/phase1-validation.json` for exact evidence, fingerprints, limits and migration notes. Stop before Phase 2; this does not close M1 or any release/platform gate.\
Deviation notes: Exact Screen cleanup matching and an error-aware compatibility lookup moved forward from 2A to prevent wrong-target cleanup or creation after a failed probe. Native invocation contexts, bounded snapshots and platform stubs are transport/lifecycle prerequisites; the general typed backend/output APIs remain Phase 2. A small CGO waitid/WNOWAIT helper reserves the child PID/PGID until group teardown; stable name locks are retained to avoid inode races. macOS runtime validation remains blocked on an available host/runner; no macOS, other-architecture, musl, Windows-native or archive runtime pass is claimed. Existing GoReleaser deprecations and performance thresholds remain release-gate work. No real user resources were mutated.

### Phase 2: Backend model, identity, capabilities and JSON (M1)

**PR 2A - typed adapters and targeting**

- [x] Introduce typed `Session`, `SessionLocator`, `ManagedID`, `BackendCapability`, `Snapshot`, `OperationError` models in a small `pkg/core` (or similarly cohesive) package independent of CLI rendering.
- [x] Refactor adapter reads to return data; move formatting into Cobra/TUI. Replace `SessionExists` boolean internally with an error-aware exact lookup so permission/probe failure cannot trigger accidental session creation. Preserve compatibility wrappers where needed. Add request contexts/deadlines and batch queries; interactive attach uses cancellation and handshake deadlines, not a finite lifetime cap.
- [x] Resolve executable paths consistently for availability and execution; centralize external command construction enough to apply isolated server flags, context and captured diagnostics to every path. Preserve genuine empty-list outcomes without hiding invocation/parse failures; retain Screen full server identity for exact targeting rather than losing it when parsing names.
- [x] Expose `backend capabilities` for `session_lifecycle`, `attach`, `read_only_attach`, `output_snapshot`, `output_history`, `input_injection`, `exact_pane_targeting`, `window_layout`, `working_directory`, `managed_launch`. Each has `supported|degraded|unsupported`, constraints and reasons.
- [x] Add strict `--backend`, `--all-backends`, collision disambiguation and exact targets. Keep unqualified mutations scoped to the selected backend; enumerate all backends only for explicit aggregate reads. Reject ambiguous names within that backend and across requested aggregates; do not silently turn existing `nuke` into cross-backend deletion.
- [x] Use stable backend window/pane IDs where available and exact session resolution; reject focus-based Zellij/Screen target operations before side effects when requested identity cannot be proven. Cover window close/rename/split as well as pane input/kill. Probe real version support before expanding capabilities. Verify R05 and the backend ID-targeting prerequisites for R11; full profile R11 runs in PR 3A.
- [x] Send tmux input as literal text with separate Enter delivery and safe option handling; test `Enter`, `C-c`, leading dashes, spaces and Unicode. Native input is acknowledged but cannot establish command success. Preserve/document `pane exec` and root `exec` as input injection (R09).

**PR 2B - stable CLI JSON and output API**

- [x] Implement `--json` for `session list`, `session inspect`, `backend capabilities`, with identical versioned success/error envelope (`schema_version`, `data`, `errors`, `partial`). Add other read commands only when justified.
- [x] Success/empty: exit 0; expected failure: valid JSON envelope and nonzero code; multi-backend partial success: valid data + errors + `partial=true`, exit **7**. No free text on JSON stdout, even on error.
- [x] Document `error.code`, `message`, optional `backend/locator`, `retryable`; freeze CLI exits: `0 success`, `1 internal/general`, `2 invalid usage`, `3 not found`, `4 unsupported`, `5 conflict/ambiguous`, `6 timeout`, `7 partial/unavailable backend`, `8 denied/unauthorized`. Define tests for precedence when multiple failures occur.
- [x] Persist a private, atomic random host ID at first structured-state use and include required `{host_id, backend, name}` in local locators; report initialization failures instead of silently dropping identity. Unmanaged sessions have no managed ID; M1 defines the type but only M2 allocates managed identities. Define separate managed entity ID vs per-launch instance ID before receipts/state are written. Handle same names across backends; reconcile managed renames in M2 through coordinator ownership.
- [x] Implement bounded `session output --lines N --plain` plus explicit `--raw` and optional `--bytes`, with output type (`screen_snapshot` or `history`), max 1 MiB per request (initial design; revise with evidence), and safe terminal sanitization. Enforce capture bounds/timeouts during collection, not after unbounded `io.ReadAll`/`cmd.Output`; reject unsupported history. Cover CSI/OSC/DCS/clipboard escapes, control bytes and truncated UTF-8. Placeholder previews are not successful output snapshots.
- [x] Route ordinary picker previews through the bounded/cancellable snapshot service; render unsupported/unavailable text in the UI without reporting placeholder text as backend data. Keep hidden `dump` compatible only where safe; document limits/deprecation and test raw output remains explicit.

Acceptance criteria:

- [x] CLI golden schemas and error exits pass in native/tmux/available adapters; JSON is valid on expected errors and partial discovery.
- [x] Capabilities match observed backend behavior; no operation can address wrong pane/session due to ambiguous name.

Status: Done\
Completion notes: 2026-10-29, Phase 1 source/evidence and baseline SHA revalidated; Phase 2 implemented and validated on Linux amd64 with real tmux 3.4. Typed reads/exact lookup, capabilities, strict/aggregate backend selection, JSON/error exits, private host identity, bounded output and safe previews are complete. Disposable sequential `make test-integration test-integration-race` passes nine guards, five smoke checks, four CLI groups, 11 native cases and 15 Phase 2 groups; both latter suites also pass with the race executable. Go 1.26.10/1.27.3 tests, Go race, zero-issue lint, Windows amd64/arm64 build-only checks, frame fuzz (30 s / 1,731,055 executions) and plain sanitizer fuzz (10 s / 294,221 executions) pass. Exact logs, fingerprints, limits and migration notes are in `docs/testing.md` and `docs/evidence/phase2-validation.json`. No commit/PR created; stop before Phase 3.\
Deviation notes: Action/read compatibility wrappers remain while CLI reads use `core.Reader`; only the profile existence probe changed early to preserve discovery errors. Real tmux pane formatting replaced control separators and literal-key argv stripped trailing semicolons, requiring printable metadata separators and bounded hex-byte input plus separate Enter. The retained early-exit test now verifies classified not-found (3) as well as startup/general outcomes and mandatory cleanup. Cancellation takes 130 even for partial aggregates. Unverified private structured-state permissions on Windows return unsupported; Screen/Zellij exact pane/layout operations remain unsupported and their lifecycle is degraded/unverified. macOS/other runtime hosts, real release archives, scale thresholds, profiles and existing GoReleaser deprecations remain open M1/Phase 3 gates. No new coordinator, real user resource mutation, commit, push or release.

### Phase 3: Correct declarative profiles and M1 release gate

**PR 3A - profiles**

- [x] Implement strict schema/preflight and dry-run (do not echo full command arguments by default). Honor `backend`, `dir`, `layout` and requested initial pane/window or reject before creation; never silently ignore fields.
- [x] Preserve existing TOML keys `name`, `backend`, `[[window]]`, `name/layout`, `[[window.pane]]`, `command/dir/split`; decode unknown fields as errors with migration notes. Define legacy `command` strings as shell commands sent to the intended shell, with no completion/exit-status promise. Structured managed argv manifests belong to PR 4A; adding profile argv syntax is optional and must not delay M1. Validate names/prefix application, split values, cwd/path existence, layouts and duplicate names before side effects.
- [x] Compile a backend-specific apply plan before creation. Initially implement exact multi-window/pane profiles for tmux; capture returned initial/new window and pane IDs (including base-index/pane-base-index 0 or 1), cwd and layout application. Replace blind renames of both `0` and `1` and active-pane assumptions. Native may accept only a supported single-terminal profile; reject unsupported Screen/Zellij layouts or command targets during preflight until an exact API is verified.
- [x] Define preexisting session policy (attach unchanged only when explicitly requested; otherwise conflict), `--detach`, and a documented rollback strategy limited to resources definitely created by this invocation. Return clear partial-error with surviving resources if cleanup is unsafe.
- [x] Run multi-pane tmux tests including spaces in cwd, focus changes and nondefault indexes; fake-command tests assert unsupported profiles make zero mutations. Run Zellij/Screen runtime profile tests only after exact targeting/isolation is established; otherwise mark unsupported. Verify R06/R11. Read-only attach must never create a session or apply a profile implicitly.

**M1 release gate**

- [ ] Run all M1 acceptance/tests, human compatibility checks, race/fuzz tests where feasible, baseline resource comparison, and platform support smoke tests.
- [x] Update relevant docs, completions and CHANGELOG. Document legacy protocol migration, backend matrix, new errors, and any intentional profile changes.
- [x] Record M1 release readiness and blockers. **Do not proceed to M2 if mandatory M1 correctness/security gates fail.** No publish without approval.

Acceptance criteria:

- [x] M1 alone can manage real persistent sessions, inspect them as JSON, and reject unsafe/unavailable operations honestly (tested Linux amd64/native and tmux 3.4 scope).
- [x] All M1 portions of R01-R12 pass where claimed on the validated Linux host; see the regression mapping and unverified boundaries in `docs/testing.md`. No agent coordinator is needed to use M1; R08 coordinator-loss and R12 receipt checks remain M2 work.

Status: Blocked (M1 release gate); PR 3A implementation complete on Linux amd64\
Completion notes: Phase 2 source/harness/build fingerprints and baseline SHA revalidated. Strict profile compilation, redacted dry-run, tmux returned-ID topology/cwd/layout/input, native single-terminal cwd, explicit existing/read-only policy and ownership-checked rollback are implemented. Go 1.26.10/1.27.3, Go race, zero-issue lint, 10 guards, 5 smoke checks, 4 CLI groups, 11 native cases, 15 structured groups and 16 profile groups pass; native/structured/profile also pass with the race executable. Both actual local GNU/musl Linux amd64 snapshot ZIPs pass all five runtime suites and private install/content checks. Frame/plain fuzz pass (30 s/727,266 and 10 s/160,218 executions); Windows amd64/arm64 cross-builds pass with native unavailable. Predeclared startup/list/RSS budgets pass at 1/10/100 private idle sessions (5.24 ms startup, 116.52 ms list100, 22,088 KiB max RSS). Final Linux validation/readiness evidence is recorded in `docs/testing.md`, `docs/evidence/phase3-validation.json` and `docs/m1-readiness.md`; the full release checkbox stays open for required unrun platforms. Stop before Phase 4/M2.\
Deviation notes: tmux profiles require 3.4+ while ordinary reads retain 3.x; profile panes explicitly launch the invocation shell instead of inheriting tmux default-command. Exact input checks pane synchronization in the server queue; new profile panes disable it. Native rejects relative TMPDIR so cwd cannot redirect runtime discovery. Actual archive testing fixed absolute pkg-config paths and required matching GNU/musl dependency libraries and a fresh release Go cache, without changing dependency pins. Local narrowed Linux snapshots do not execute/certify the all-platform release workflow. macOS, Linux arm64 and Windows runtime/archive/install gates remain blocked on actual hosts/evidence; foreign-UID end-to-end security remains unverified. No coordinator, M2 work, real user session mutation, commit, push or publication.

### Phase 4: Opt-in supervisor, registry and coordinator (M2)

**PR 4A - managed launcher with durable receipts**

- [x] Implement structured manifest (`schema_version`, random instance/managed ID, executable argv, cwd, an environment-variable *name* allowlist, selected backend, receipt path) stored mode 0600. Consume the manifest once and securely unlink it after the wrapper reads it; do not retain argv or environment secrets in the agent registry or logs. Any transient argv values remain private to the local user while the manifest exists. Do not embed user arguments in shell launch strings.
- [x] Implement `internal supervised-run` in the session PTY. Execute argv without shell by default; capture exit/status/signal and write a mode-0600, atomic exit receipt. Handle failed exec, SIGTERM, parent CLI exit and coordinator absence.
- [x] For tmux/native, verify managed launch runs inside the requested session PTY with correct cwd and exit semantics. Handle children that exit before session discovery/readiness: a durable receipt, not surviving socket/session presence, proves completion. A SIGKILL of the wrapper/server may prevent any receipt; report unknown/lost rather than inventing an exit code. Add other backend launchers only after proving equivalent behavior; otherwise capability unsupported.
- [x] Distinguish process group kill from closing the terminal, and define cleanup. Do not run untrusted project hooks by default.

**PR 4B - agent-state owner**

- [x] Implement optional local coordinator using private Unix domain socket (or separately tested Windows local transport if supported), start on agent command only, not on normal session reads. One active coordinator per user runtime with safe lock/instance ownership; prevent socket hijack or competing writers.
- [x] Coordinator owns registry/live states; supervise/store atomic JSON snapshots and versioned state files without transcripts/secrets. Wrapper owns final exit receipt. Reconcile on startup without PID-only assumptions; unknown when unprovable.
- [x] Provide `agent start`, `agent register` (existing-session attachment **without** supervision claim), `agent list`, `agent status`. Registering an existing session allocates an agent registration ID, not a retroactive durable backend-session identity or exit receipt. First-class generic harness; support agent `cwd`, clear sources/timestamps, explicit backend, controlled launch arguments. Reconcile rename/delete through coordinator requests or later discovery; the daemonless M1 CLI never becomes a second registry writer.
- [x] Test coordinator kill/restart, concurrent starts, duplicate names, rename, torn writes, corrupt/stale metadata, session deletion, and PID reuse. Existing non-agent sessions still work while coordinator is stopped.

Acceptance criteria:

- [x] Receipts for observed managed child exit 0/42/signal remain queryable after CLI and coordinator exit/restart; missing receipts after wrapper/server loss yield unknown. R08/R12 M2 recovery checks pass; no re-execution occurs during recovery.
- [x] Unsupervised/unmanaged sessions never receive invented exit codes. Agent metadata stays private, minimal, consistent and versioned.

Status: Done (Linux amd64 development/validation; M2 release gate remains Phase 5)\
Completion notes: 2026-10-30, Phase 3 baseline and source/harness/build fingerprints revalidated. PR 4A/4B implemented: one-use structured manifests, direct foreground-PTY argv/cwd/environment-name filtering, immutable observed exit receipts, private opt-in coordinator, sole-writer registry, exact native/tmux bindings, restart reconciliation and agent start/register/list/status/stop-coordinator. Go 1.26.10/1.27.3 tests, Go race, zero-issue lint, retained M1 normal/race suites and 19 final agent groups normal/race pass. Both actual GNU/musl Linux amd64 snapshot archives pass install/completion/content checks and all M1 plus 19 agent groups. Exit 0/42/signal receipts survive CLI/coordinator loss/restart; missing proof remains unknown and no recovery re-executes. Native/plain/local-frame fuzz pass 30 s/1,324,136, 10 s/24,694 and 15 s/607,628 executions. Windows amd64/arm64 build-only and Linux CGO-disabled refusal checks pass. Existing 1/10/100 idle-session budgets pass (4.77 ms startup, 106.12 ms list100, 25,300 KiB max RSS). Evidence, exact hashes, sequencing and limits: `docs/testing.md` and `docs/evidence/phase4-validation.json`. Stop before Phase 5.\
Deviation notes: The new explicit Phase 4 request authorizes Linux development while the previous cross-platform M1 release gate remains Blocked; no M1/M2 release claim or publication. Initial managed launch/registration supports native with CGO and tmux 3.4+, not Screen/Zellij/Windows. Environment allowlists filter the wrapper/backend environment, which may predate a client; values are not uploaded/persisted. Added explicit coordinator stop, separate foreground child process groups, wrapper liveness IPC and state-root/host-ID request binding; a copied ID at a different state root cannot use the wrong registry. Linux uses atomic no-replace publication; macOS path remains unverified. Registry has 256-record limit without automatic pruning; missing receipts after wrapper/server loss stay unknown. Phase 2 managed_launch capability expectations advance intentionally; a failed-connect test socket leak and Linux thread-child fixture were corrected. No dependency upgrades, coordinator requirement on ordinary commands, real user resource mutation, commit, push, tag or publication.

### Phase 5: Agent hooks, events, waiting, notifications and M2 gate

**PR 5A - provider-neutral, then verified provider adapters**

- [x] At implementation time verify Claude Code, Codex and OpenCode supported hook/event mechanisms against primary docs and installed CLI versions; this review did not verify their current APIs. Record supported signals and unavailable hooks explicitly; provider absence must not block generic M2 use or M1.
- [x] Implement authenticated/associated `agent signal` intake to coordinator, with per-registration token in mode-0600 local file, instance ID, timestamp/sequence and freshness checks. Same-UID access is the trust boundary; do not claim isolation from all same-user processes.
- [x] Generic adapter first. Implement provider-specific Claude Code, Codex and OpenCode support **only after** confirming real supported hook/event API and version. Record exact translation and missing statuses in contracts and tests.
- [x] Keep process lifecycle, attention, and turn completion distinct; forbid inferred `done` from regex-based output. Missing hooks -> `unknown` attention.

**PR 5B - events and deterministic wait**

- [x] Define `schema_version`, `coordinator_epoch`, `sequence`, `time`, `managed_id`, `kind`, `source`, and safe metadata for JSONL events. No prompts/terminal data by default.
- [x] Bounded event fanout and snapshot resync (`resync_required`) on overflow/reconnect/epoch change. Document no replay and no exactly-once guarantee. Streams close promptly on cancellation.
- [x] Implement `agent wait ID --until exited|needs-input|idle --timeout D`, default finite timeout; initial atomic status + subscription handshake (or snapshot/subscribe/resnapshot) avoids missed transitions. On `--until exited`, exit 0 means **observed exit**, regardless of child status; child `exit_code` remains available through `status`. Optional `--fail-on-nonzero` is a separate, tested mode.
- [x] Timeout exit 6; unknown/lost/unsupported condition returns appropriate nonzero. Coordinator crash/restart triggers resync. No busy-waiting or privileged escalation.

**PR 5C - optional notifications**

- [x] Notifications are disabled by default. Event-driven desktop notifications or user-approved script actions only, with throttling/deduplication and transcript-free payloads.

**M2 release gate**

- [x] Demonstrate generic supervised process + real agent CLI integration on tested harness; verify timeout/status/source semantics, privacy, recovery and normal-session independence.
- [x] Update docs/contracts/CHANGELOG and support matrix; explicitly document unsupported provider versions and adapter limitations. Stop before TUI if M2 gates fail.

Acceptance criteria:

- [x] Events/waits remain truthful under missing signals, nonzero child exit, crash/restart, stale hooks and dropped subscribers.
- [x] M2 is useful entirely through CLI/JSONL; no TUI required.

Status: Done (Linux amd64 development/validation); M2 release gate Blocked by inherited M1 platform gaps\
Completion notes: 2026-10-30, Phase 4 baseline and source/harness/build/docs fingerprints verified. PR 5A/5B/5C implemented: authenticated rotating per-instance/generation signals, conservative provider bridge, sourced expiring attention and separate turn completion, bounded epoch/sequence JSONL fanout with atomic snapshots/resync, deterministic finite waits, explicit bounded/throttled notification actions. All 14 new groups and retained 19 agent groups pass normally/race and from both actual GNU/musl Linux amd64 snapshot ZIPs. Retained M1 10 guards, 5 smoke, 4 CLI, 11 native, 15 structured and 16 profile groups pass, including all runtime suites with race. Go 1.26.10/1.27.3, Go race, zero-issue lint, dependency verification, three fuzz targets, Windows amd64/arm64 build-only and CGO-disabled refusal checks pass. Fixed 1/10/100 session budgets pass. Actual Claude 2.1.308 --init-only SessionStart works in private HOME without auth/model requests; Codex 0.165.0 schema/config/session-binding fixtures pass. Evidence, exact hashes, resource samples and commands: `docs/testing.md`, `docs/evidence/phase5-validation.json`, `docs/m2-readiness.md`. No Phase 6.\
Deviation notes: Explicit request authorizes Linux development while inherited M1 platform release gates stay Blocked; M2 is not released. Codex requires manual trust review and its real provider lifecycle is unrun; no bypass or auto-activation. OpenCode 2.2.1 v2 session/plugin runtime and other provider versions remain unsupported. Claude full model/tool/permission conversations and desktop popups are unrun; optional actions use fixture scripts. Overflow is verified deterministically at broker level. No dependency changes, original ignored bin/ghostty changes, user provider settings, real sessions/worktrees, commits or publication.

### Phase 6: Dashboard and M3 release gate

**PR 6A - dashboard UI**

- [x] Build `txm dashboard` with session/backend/host, managed status, freshness, attention, client state where known, capabilities and optional bounded output preview. Use M1/M2 data APIs only, not a second status parser. Hide/disable unsupported actions; external backend preview placeholders remain UI messages, not successful snapshots. CPU/memory metrics from the older roadmap are optional until attributable and tested; they do not expand mandatory scope.
- [x] Keyboard-first search, filters, attach/read-only attach, resize support, error/unknown indicators, graceful non-TTY failure, safe TTY restoration and cancellable refresh. Consider reuse of `go-fuzzyfinder`; add TUI dependency only if justified.
- [x] Preserve bare `txm` fuzzy-picker behavior initially. No polls or coordinator overhead for ordinary picker unless agent feature explicitly chosen.

**PR 6B - UX and release readiness**

- [x] Provide human examples: shell persistence, remote SSH attach with existing helper, two managed agents, JSON scripts, needs-input events/wait, read-only inspection. Make a short recording with redacted sample data.
- [x] Run dashboard stress tests under slow/unavailable backends, many sessions, terminal resizes, SIGINT, repeated attach/detach, and non-TTY usage.
- [x] Update README, `docs/user-guide.md`, `docs/ROADMAP.md`, `docs/txm.1`, completions and `docs/CHANGELOG.md`, supported platform matrix. Correct stale nuke/read-only/backend-selection/build requirements as their implementation lands; make roadmap cloud sync explicitly deferred and replace output-based success heuristics with supervised/hook-backed semantics. Record M3 test evidence and unresolved limitations.

Acceptance criteria:

- [ ] Dashboard is responsive and truthful; ordinary users keep the original picker; full M1-M3 integration and release checks pass. **Linux subcriteria pass; required cross-platform release checks remain Blocked. Keep this combined release criterion unchecked.**
- [x] No backend or OS is advertised as supported for an untested capability.

Status: Done (Linux amd64 development/validation); M3 release gate Blocked\
Completion notes: 2026-10-30, Phase 5 baseline and all four fingerprints verified. PR 6A/6B implemented: separate dashboard with typed session/registration rows, host/backend/source/freshness/capability details, search/type/state/backend filters, capability-gated native/tmux attach/read-only actions, cancellable independent refresh, selected bounded plain preview, stable selection and TTY/file-flag restoration. Bare picker unchanged. Linux deny-only native ancestry/TTY guard prevents self-attachment feedback; missing context disables native actions. Heartbeats plus bounded typed M2 list refresh and per-view TTL preserve truthful running-state freshness. Six model/screen unit groups and 14 real PTY groups pass normally/race and from both GNU/musl ZIPs. Retained M1/M2 suites, Go 1.26.10/1.27.3, Go race, zero-issue lint, module verification, three fuzz targets, Windows/macOS amd64/arm64 CGO-disabled cross-builds and refusal checks pass. Final model-only display-scope fix reran all Go checks, actual ZIP install/completions and all 14 dashboard groups; M1/M2 sources unchanged. Normal 1000-session search/resize/quit 33.57/283.47/20.76 ms, RSS 30,604 KiB pass fixed budgets; retained isolated M1 startup/list100/RSS 6.91 ms/103.10 ms/25,504 KiB pass. Synthetic 5.55-second asciicast, examples, docs/support matrix and exact hashes/commands are in `docs/evidence/phase6-validation.json`, `docs/testing.md`, `docs/m3-readiness.md`.\
Deviation notes: Explicit request authorizes Linux development despite inherited M1/M2 gaps; no M1/M2/M3 release certified. Non-Linux dashboard runtime is explicitly unsupported until equivalent native feedback guard and TTY behavior are verified; macOS native/external and Linux arm64/Windows external runtime/archive/install plus foreign-UID E2E remain unrun. Screen/Zellij dashboard actions disabled, metrics attribution omitted. Existing tcell/uniseg promoted to direct use without version/checksum changes. Agent/session rows remain separate rather than guessing a binding by name. M2 is per-host, so --backend scopes visible rows/direct M1 reads but does not stop the coordinator's other reconciliation. Native capacity fixture now accepts valid EPIPE rejection as well as EOF/reset without weakening capacity/liveness assertions. Provider limitations unchanged. No original ignored bin/ghostty or real user resource/provider-setting changes, optional extension work, commit, push, tag or publication.

### Phase 7: Optional extensions (E1/E2; explicitly requested)

**E1 - Worktrees (separate PR/release)**

- [x] `task start/list/inspect/stop/cleanup`, backed by Git and existing managed session IDs; no independent session state machine.
- [x] Dry-run before side effects; verify branch/repo/worktree state; never run repo setup commands without opt-in; rollback only txm-created clean worktrees; refuse dirty/untracked cleanup by default.
- [x] Test temporary repositories, collision/restart/orphan handling, symlinks, detached HEAD, dirty/untracked files, force-confirmation boundaries.

**E2 - SSH aggregation (separate PR/release)**

- [x] Allowlist OpenSSH host aliases, reuse user's SSH authentication/ControlMaster and remote `txm --json` through explicitly verified quoting/argv transport.
- [x] Remote `list/attach`, strict `{host_id,backend,name}` targeting, schema/version negotiation, bounded timeouts and clear partial failures; no exposed socket/network service.
- [x] Test with fake/loopback SSH, failure/cancellation, version mismatches and identity collisions before real remote smoke.

Acceptance criteria:

- [x] No remote/session/worktree action can accidentally target, delete or overwrite a user's unrelated resources.
- [x] Extensions are independently installable/usable; failures never degrade M1-M3 core behavior.

Status: Done (Linux amd64 development/validation); inherited release gates and actual SSH network/server validation remain Blocked/unrun.\
Completion notes: 2026-10-30, verified Phase 6 prerequisite fingerprints and baseline HEAD. E1 implements task start/list/inspect/stop/cleanup on existing managed IDs, with redacted preflight, fresh branches, pinned repo/common-dir/worktree/.git/admin identities, journaled launch intent, strict native/tmux selection and ownership-checked stop. Default no-checkout plus conditional-filter checks prevent Git hooks/filters from running without opt-in. Cleanup requires exit receipts or authoritative same-state orphans; dirty/untracked/ignored files require force plus exact ID. Branches/commits remain. E2 implements private SSH alias allowlisting and explicit host pinning, schema/protocol 1 probing, bounded concurrent list with scoped partial errors, encoded request tokens and exact observed native/tmux attach with read-only and TTY restoration. Eight new unit groups and 12 disposable integration groups pass normally/race and from both actual GNU/static-musl ZIPs; all retained M1/M2/M3 suites, Go 1.26.10/1.27.3, race/lint/module verification, four fuzz targets, fixture guards, ZIP install/completions, CGO-disabled refusal and compilation-only Windows/macOS amd64/arm64 builds pass. Isolated startup/list100/RSS 6.96 ms/109.76 ms/22,912 KiB pass unchanged budgets. Commands/hashes/outputs are in docs/evidence/phase7-validation.json; usage and limits in docs/extensions.md and docs/extensions-readiness.md.\
Deviation notes: Explicit Phase 7 request authorizes E1/E2 Linux development despite inherited M1/M2/M3 platform/provider gates; none is relabeled released. Extensions are separate packages/namespaces in the existing executable rather than separately distributed plugin binaries; Git/SSH resolve lazily and ordinary core commands do not initialize extension state. Failed starts retain inspectable records/worktrees for explicit cleanup rather than automatic rollback; missing/moved paths or a crash before ownership persistence require manual inspection, with no replay or guessed cleanup. Records/branches remain (128 tasks/256 agents; no automatic pruning). Git 2.43.0 and OpenSSH 9.6p1 tested; sshd unavailable. Actual endpoint/PTY behavior is tested over fake SSH plus a real client configuration-only check; actual SSH authentication/host-key/ControlMaster server/network smoke remains unrun, no real host contacted. Provider real-CLI initialization skipped this phase; prior evidence/limits retained. Corrected Makefile fuzz dollar escaping after snapshot build, verified with make -n; compiled/harness/embedded sources match actual snapshots. No new dependency, original ignored bin/ghostty changes, real user resource changes, staging, commit, PR, push, tag, release or publication. Phase 8 remains Pending.

### Phase 8: Validation and Cleanup

Deliverables:

- [ ] At **each milestone**, run the corresponding complete Test Plan subset; preserve exact command outputs, host/arch, backend versions and skip reasons. Final integration is not a substitute for earlier gates. **Linux evidence retained/revalidated; required cross-platform milestone gates remain Blocked.**
- [x] Audit protocol framing, concurrency, connection permissions, terminal escape/render behavior, target identity, failure signaling, metadata atomicity, shutdown/kill, and OS claims.
- [x] Verify security properties with adversarial tests; confirm no secrets or raw prompts in committed fixtures, example outputs, logs or binary artifacts.
- [x] Compare observed startup, memory, PTY throughput and TUI responsiveness against baseline and recorded thresholds; resolve significant regressions or document tradeoffs.
- [ ] Validate clean-room install and archives, GoReleaser/static-musl requirements, shell completions and docs on supported hosts. Do not claim unsupported Windows runtime behavior. **Linux GNU/static-musl ZIPs pass; remaining platform install/archive gates stay unchecked.**
- [x] Run complete existing human workflow suite. Check aliases, profile migration, legacy native socket coexistence, coordinator disabled path, and startup without internet/LLM tools.
- [x] Review final diff and remove unused code/dependencies, temporary debugging, misleading comments, generated binaries and unrelated edits. `git diff --check` clean.
- [x] Complete the ledger with exact passing checks, commits/PR references if any, supported platform matrix, change notes, decisions, deviations, known limitations and any deferred E1/E2 tasks.

Acceptance criteria:

- [ ] Each shipped milestone has evidence and no open critical correctness/security issue. `Blocked` is never relabeled `Done` without actual passing evidence. **No milestone release is certified; unrun release gates remain Blocked.**
- [x] No push, merge, publication, live session deletion or real worktree cleanup occurred without authorization.

Status: Done (Linux amd64 audit/validation/cleanup); whole-project release gate Blocked\
Completion notes: 2026-10-30, verified Phase 7 source/harness/build/docs/evidence fingerprints and baseline HEAD before edits. Audited framing/backpressure, permissions, identity, terminal sanitization/TTY restoration, metadata/receipts, recovery and OS claims. Fixed remote missing/null completeness flag and missing data acceptance (11 envelope regression cases); fixed unsafe task orphan inference after registry loss by pinning supervisor instances and requiring exact exit receipts, with force/mismatch/lost-pin/corrupt-receipt/never-launched cases (13 extension groups). Added task name-collision preflight before worktree creation, isolated build-helper HOME/Git environment and a routing/canary regression. Added reproducible source/artifact-bound Linux runtime runner and bounded credential-pattern artifact scanner with redacted findings; archive smoke checks task/remote completions. Final normal/race/minimum Go 1.26.10/current 1.27.3, lint, mod verify/tidy-diff, four fuzz targets, 15 fixture/validator guards and all nine runtime suites on normal/race/actual GNU/static-musl ZIPs pass (40 runtime/install/performance checks). Actual musl ELF has no interpreter/dynamic section. Compilation-only Windows/macOS amd64/arm64 and CGO-disabled refusal pass. Fixed budgets pass: startup 4.70 ms, list100 98.06 ms, native RSS 25,848 KiB; 1000-row dashboard search/resize/quit 22.72/249.31/20.66 ms, RSS 31,248 KiB. Isolated PTY/throughput baseline comparison, exact commands/hashes/logs and privacy scope are in docs/evidence/phase8-validation.json and docs/final-audit.md. Earlier phase evidence preserved; final current/archive source, harness and build fingerprints match. git diff --check clean; no dependency changes, staging, commit/PR or publication.\
Deviation notes: Linux development audit is complete, not whole-project release certification. Required macOS/native arm64 hardware/Windows external runtime and archive/install, actual different-UID peer, provider trusted/full lifecycle and SSH server/network gates remain Blocked/unrun. QEMU and cross-compiles are not hardware/runtime proof; UID-switch probe fails in this environment and sshd is absent. Existing Screen/Zellij/provider limitations remain. Older task records load, but registry loss without an instance pin/validated receipt requires manual inspection; missing registration is never orphan proof, even with force. Historical Phase 7 absence-as-orphan assumption is superseded by this fix. Worktree failures retain records/branches; no automatic replay/rollback/pruning. Initial Phase 8 run was interrupted after the recovery audit found unsafe cleanup; only its exact owned process group was stopped, fixture teardown verified, and fresh final binaries/archives/full matrix rerun. Privacy scan covers known credential forms and synthetic fixture provenance, not arbitrary secret detection. Original ignored bin/ghostty and real user sessions/worktrees/SSH/provider settings untouched. No push, tag, release or publication.

### Linux follow-up after Phase 8 (explicitly requested)

- [x] Actual different-UID native/coordinator IPC rejection, separately testing filesystem permissions and reachable kernel peer-credential checks; legitimate owner/children/registry survive.
- [x] Real OpenSSH loopback server/client: authentication, strict host keys, wrong client key, host pins, partial results, version mismatch, timeout/cancel, ControlMaster reuse/reconnect, actual native/tmux read-only/control PTYs and TTY restoration.
- [x] Real Linux GNU Screen 4.09.01 and Zellij no-web 0.45.1 basic lifecycle, exact targets/name collisions, protected second namespace and attach/input/detach/TTY checks. Portable capability restrictions retained.
- [x] Explicit metadata-only task/agent pruning with dry-run, exact-ID confirmation, cleanup/exit checks, branch/receipt retention, bounded retired-ID replay prevention and event resync; no automatic pruning.
- [x] Offline real Claude 2.1.308 SessionStart initialization with no model/auth/network; installed Codex 0.166.0 and OpenCode v2.2.2 version guards refuse activation before starting a coordinator.
- [x] Final normal/race/GNU/static-musl runtime (44 checks), archive/install/completion, performance and evidence reconciliation; source/artifact-bound results in docs/evidence/linux-followup-validation.json.
- [ ] macOS runtime/archive/install, native ARM64 hardware/runtime, Windows external-backend/archive/install and trusted/full provider lifecycle: require other runners or explicit reviewed provider setup; remain Blocked.

Status: Done for all locally feasible Linux work without provider authentication/trust changes; whole-project release gate remains Blocked.\
Completion notes: New container runner uses a pinned Ubuntu base and records image/tool versions, no external runtime network, read-only copied fixtures and explicit binaries, private HOME/keys/backend namespaces and ownership-checked cleanup. All nine Linux groups pass for normal/race/actual GNU/static-musl ZIPs (36 groups); offline real Claude initialization also passes. Real SSH exposed and fixed remote operation exit 1 being misclassified as transport exit 7; OpenSSH 255 remains transport failure. All 44 retained runtime/install/performance checks pass, including four pruning groups on each binary; 13 Go/minimum-version/race/lint/module/build checks, four fuzz targets, 15 harness guards, actual static-musl linkage and generated/dynamic pruning completion checks pass. Fixed budgets: startup 6.10 ms, list100 111.66 ms, peak native RSS 25,700 KiB; normal dashboard search/resize/quit 33.49/206.73/20.59 ms and RSS 30,992 KiB. Exact commands, fingerprints, artifacts, version guards and privacy/cleanup results are in docs/evidence/linux-followup-validation.json. See docs/linux-followup.md for reproduction and review boundaries.\
Deviation notes: Host lacks sshd/Screen/Zellij; disposable containers close those Linux gates without host accounts/services/settings changes. Zellij release asset and executable hashes verified; apt packages are recorded, not claimed as a package lock. Containers share Linux amd64 kernel and do not certify other platforms/hardware. Full provider lifecycle/trust not inferred from initialization; no allowlist relaxation or trust bypass. Task/agent pruning reclaims index capacity, not disk storage; receipts/manifests/credentials remain and retired IDs are bounded at 16,384. Original ignored build/dependency directories and real resources preserved. No staging, commits, PR, push, tag, release or publication.

### PR boundaries and stop conditions

| PR | Milestone | Work | Must pass before next dependent PR |
| --- | --- | --- | --- |
| 0 | prep | Verified dependency pins/build, isolation seams/harness, baseline, M1 design docs | Reproducible build and proven isolation; source review alone is insufficient |
| 1A | M1 | CLI/config errors, stdout/stderr and aliases | Nonzero failures, no config panic or duplicate error output; classified codes finalized in 2B |
| 1B | M1 | New native IPC/security/migration | Frame fuzz, isolation and read-only tests |
| 1C | M1 | Native lifecycle/platform correctness | Real PTY and applicable R01/R02/R04/R07/R08/R12 plus native R09 |
| 2A | M1 | Typed backend models/capabilities/targets/literal input | Adapter matrix incl. R05/R09 and ID-targeting prerequisites for R11 |
| 2B | M1 | JSON/output/host identity and bounded picker preview | Schema/exit/partial-output golden tests; bounded capture and cancellation |
| 3A | M1 | Profiles + release gate | R06/R11, rollback and M1 portions of R01-R12 |
| 4A | M2 | Supervised runner and receipts | Truthful exit after parent/coordinator loss |
| 4B | M2 | Coordinator/registry/agent CLI | Crash, concurrency, stale-state tests |
| 5A | M2 | Generic and verified harness adapters | Hook provenance/freshness tests |
| 5B | M2 | Events and wait | Gap, race, exit, timeout tests |
| 5C | M2 | Notifications + release gate | Privacy, disabled-by-default, M2 tests |
| 6A | M3 | Dashboard | TTY, responsiveness, data-source truth tests |
| 6B | M3 | Docs and UX/release gate | M3 test/packaging matrix |
| E1 | optional | Worktree management | Disposable Git and cleanup safety tests |
| E2 | optional | SSH management | Quoting/version/host identity safety tests |

**Stop the affected test/PR or release gate and mark it Blocked when:** a test may reach a real user's terminal server; a native protocol change could unlink/disconnect an existing legacy session; a security or framing test fails; required state cannot be established truthfully; a supposedly supported exact target is unreliable; a required gate cannot be tested on the claimed platform; persistent metadata is corrupted or non-atomic; a PR introduces a critical regression; or an action exceeds authorized access. Fix defects within the authorized repository scope and rerun the gate; blocked release validation does not prevent unrelated safe planning/unit work. Expected `unknown`/`unsupported` outcomes on declared unsupported capabilities are valid contracts, not reasons to fabricate support or stop the whole project. Do not ignore failing tests or weaken assertions merely to proceed.

### Autonomous agent execution protocol

1. Read this ledger, repository instructions, actual current files and `git status`; reconcile baseline SHA. Never assume the plan's observations remain current.
2. Select the first unfinished **mandatory** PR within the user's requested scope whose prerequisites passed. The current request explicitly authorizes the Linux follow-up despite inherited release blockers; preserve all unrun gates and record exact final evidence. Stop before publication. If an extension is deferred, do not opportunistically implement it.
3. Before code changes, state the behavior/contract, changed files, risks, intended tests and compatibility impact. Modify docs/contracts first when public semantics change.
4. Make the smallest implementation that passes targeted unit/integration/security tests; then run relevant full project tests, lint/race/release checks as available. Check actual effects, not subprocess exit alone.
5. Record test evidence, failures and environment limitations. Do not claim pass for an unrun test, a different OS, or a merely cross-compiled binary.
6. Update this document after the PR, including Status/Completion notes/Deviation notes. Maintain a short decision log in `docs/architecture.md` for substantive design changes.
7. Continue only after the current gate passes. At each milestone prepare a concise release-readiness summary; publishing and merging require separate explicit approval.
8. Never expose secrets through logs/events/notifications, change security posture silently, manipulate real tmux/SSH/Git state during testing, or automatically grant coding agents dangerous execution permissions.

### Final handoff checklist

Completed for the recorded Linux scope; macOS/native ARM64/Windows and trusted/full provider gates are explicitly Blocked, not certified. No commit/PR references exist because nothing was staged, committed or published.


- [x] M1, M2, M3 evidence complete (or explicitly blocked); E1/E2 explicitly deferred or separately validated.
- [x] Test sandbox proven isolated; no real sessions/worktrees damaged.
- [x] Native protocol/migration docs and actual implementations agree; old active sessions untouched.
- [x] JSON schemas, errors, identities, lifecycle/attention/wait/event semantics frozen and covered by fixtures.
- [x] Every claimed backend/platform combination supported by real tests; unsupported cases labeled accurately.
- [x] No silent false success, wrong-pane targeting, stale PID exit claims, uncontrolled transcript persistence or secrets in output.
- [x] Normal `txm` workflows still work without a coordinator; agent features remain opt-in.
- [x] Updated README, user guide, man page, completions, roadmap and CHANGELOG match tested commands.
- [x] Final diff reviewed; exact commands/results, decisions, deviations, remaining risks, and PR/commit references recorded.
