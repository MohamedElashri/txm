# Testing, isolation and release evidence

Updated through the Linux follow-up, 2026-10-30. Read this before running lifecycle, cleanup or
release tests. Phase sections below are historical evidence, not current release
certification. See [final audit and support matrix](final-audit.md) for the latest
status. Historical Phase 0 results describe the original executable;
Phase 1/2 results describe implemented behavior. Normative contracts are in [contracts.md](contracts.md) and
[native-protocol.md](native-protocol.md).

## Toolchain and dependency builds

`scripts/dependencies.env` is the shared lock for local, CI and release builds.
Zig is exactly 0.16.0; Ghostty is
`36ec90a07fcb58e3033296c06c82438bf30d2e42`, matching the existing Go binding's
`CMakeLists.txt` at `c1ee3183994d21df2b9eae4f11336aca2da1e8e6`. The original
`d4c88d806` library builds but does not provide all C symbols that binding needs.
This is a dependency alignment, not a Go module upgrade.

Requirements: Go 1.26+, CGO/C compiler, pkg-config, the locked Zig version, Git,
Python 3.10+ for the harness, and tmux for its real integration. Bash is required
for build helpers; scripts use macOS-compatible Bash features. CI's installer
downloads exact Zig metadata from ziglang.org, verifies SHA-256 and uses an exact
extracted directory. Local downloads should verify the same official checksum.

Build in a disposable clone. Do not reset/clean existing ignored dependency
checkouts or binaries. A preexisting Ghostty source tree must be clean, standalone
and at the lock revision; otherwise the helper fails without changing it. Set
`GHOSTTY_SOURCE_DIR` to a separate matching checkout. `GHOSTTY_HOST_DIR` selects
the private host install prefix, and `ZIG` selects a verified executable.

```sh
# Run from the disposable txm checkout. Paths below are fixture-specific.
export GOCACHE=/tmp/txm-build-cache
export ZIG_GLOBAL_CACHE_DIR=/tmp/txm-zig-cache
export GHOSTTY_SOURCE_DIR=/tmp/txm-dependency-checkout
export GHOSTTY_HOST_DIR=/tmp/txm-host-prefix
export ZIG=/path/to/zig-0.16.0/zig
make build
make test
make test-harness
make test-integration
make test-race
make test-fuzz
make test-integration-race
make lint
git diff --check
```

The helper always invokes Zig's content-based incremental build, even when the
prefix exists, then checks the static pkg-config file/archive and writes an
informational `.txm-build-info`. The stamp is not a cache-validity check. Source
revision and tool version are checked before each invocation. Release builds
use the same helper and place target prefixes inside `.ghostty-release/`, matching
GoReleaser; they no longer share predictable `/tmp/ghostty-*` paths. Release
cross-targets still require their own packaging/runtime gates.

Network-restricted environments may need approved dependency downloads. In the
Phase 0 environment Zig's downloader stalls with inherited proxy settings;
curl fetched locked archives, then `zig fetch /absolute/archive` verified each
package hash against `build.zig.zon` before compilation. Do not change hashes or
disable TLS to get a build. Do not commit dependency caches or generated binaries.

## One guarded fixture

`tests/sandbox.py` creates a canonical `/tmp/txm-test-*` root mode 0700, remembers
its device/inode and random ownership marker, and validates them before operations
and cleanup. Changed marker, symlinked marker, changed root permissions/ownership
or a substituted tmux socket causes refusal, leaving evidence for inspection.
This guards trusted tests; it is not an OS sandbox against malicious same-UID code.

Each fixture constructs HOME, XDG config/state/runtime, TMPDIR, SCREENDIR and PATH
from scratch. It inherits no `TMUX`, `STY`, Zellij context, `TXM_*` overrides,
SSH agent, credentials or user executable PATH. Only an explicit system shell/tool
set is symlinked into private PATH. txm's baseline uses HOME config and TMPDIR
sockets, so XDG variables alone are insufficient. Fixture commands run in a temp
repository directory. Build caches and module/toolchain paths are set separately
from the runtime fixture, avoiding downloads after HOME isolation.

### tmux

The fixture resolves the real executable before isolation, then runs a foreground
server with `tmux -D -S <private socket> -f /dev/null`. It creates a uniquely named
guard session and retains a live Popen process handle. A private PATH wrapper
adds that exact `-S` to **every** baseline adapter invocation: create, lookup,
list, capture, attach and cleanup. It rejects alternate global socket/config
selectors. No fallback to the real user's socket is permitted.

Before kill-server, verify root marker, socket device/inode, live server handle,
server-reported PID and exact guard-session presence. Guard tests use a second
disposable server as the protected stand-in and assert it survives cleanup. They
never connect to the user's default tmux server, even for a read-only safety check.
Availability's absolute-path existence checks do not execute another binary;
all current adapter executions use the wrapped bare name. Reaudit that assumption
when PR 2A changes command construction.

### Native

Start a built txm executable as foreground `server --instance <random ID>` under
private HOME/XDG/TMPDIR. The default fixture uses the v1 runtime namespace and
framed handshake; the original executable uses an explicit `legacy=True` fixture
only with `integration.py --baseline`. Teardown retains a real process handle.
The child shell records its PID and execs a bounded fixture command. Cleanup
verifies the live server's parent relationship before signaling the PTY child,
then reaps the server; it never uses discovery or an unverified PID as a cleanup
fallback. The descendant-kill test has its own bounded, cooperative fallback.
Direct protocol tests connect only to that fixture's random-instance socket.
Server stderr is collected; a race report or exit 66 fails the test.

The default smoke suite creates a real PTY for CLI attach, sends Ctrl-backslash,
compares terminal attributes before/after and checks the server survives. A
separate socket attach measures protocol latency; those are different metrics.
The gated output child emits about 256 KiB and an end sentinel, measuring PTY,
VT parsing and client fanout with a draining observer. It is one sample, not a
benchmark threshold or proof against slow-reader failures.

### Other adapters and future extensions

Zellij and Screen commands are disabled by fixture PATH stubs unless explicitly
replaced with a fake recorder. Fake executable tests check argv/control flow but
do not certify upstream runtime behavior. Zellij is absent on the Phase 0 host;
verify its version-specific socket/data namespace before any destructive suite.
Screen is absent; verify owned mode-0700 SCREENDIR with its installed version
before enabling real tests. These historical Phase 0 limits are superseded for basic Linux lifecycle/attach
by the separate [container follow-up](linux-followup.md); unsupported pane/layout
operations remain unsupported. No test may fall back to the default backend server.

Git extensions use temp repositories with hooks disabled by default; SSH extensions use
fake transport and a second private endpoint. Phase 7 implements both opt-in
extensions; actual loopback SSH is now covered by the separate container follow-up. Arbitrary project setup hooks,
install/update/uninstall commands and real worktree cleanup are not part of this
harness. Process cleanup targets only recorded live children/groups, never PIDs
discovered by name or unverified sockets. Fixture root removal uses only the
verified private root, never a path supplied by a backend response.

## Commands and interpretation

```sh
# Build guard and fixture safety tests; no txm binary required.
python3 -m unittest discover -s tests -p 'test_*.py' -v

# Passing smoke suite; native real PTY and isolated tmux.
python3 tests/integration.py --binary /absolute/path/to/txm --report /tmp/txm-smoke.json

# Desired-behavior probes on baseline. Exit 1 is expected while defects exist.
# FAIL is never renamed PASS merely because it reproduces a known bug.
python3 tests/integration.py --binary /absolute/path/to/saved-old-txm --baseline --report /tmp/txm-baseline.json

# Current CLI and adversarial native/runtime checks (not the old executable).
python3 tests/cli_errors.py --binary /absolute/path/to/txm
python3 tests/native_checks.py --binary /absolute/path/to/txm
python3 tests/structured_checks.py --binary /absolute/path/to/txm
# One focused case, if needed:
python3 tests/native_checks.py --binary /absolute/path/to/txm NativeChecks.test_concurrent_attach_resize_and_output
```

Require local Unix socket and PTY permissions. If the host sandbox denies socket
operations, request the narrow execution permission; never weaken assertions or
switch to a default socket. Scripts use finite subprocess/socket deadlines. Do
not treat skipped/unavailable adapters as successful runtime coverage.

## Regression mapping and first implementation gates

| Regression | Phase 0 reproduction / next procedure |
| --- | --- |
| R01 | Baseline sends native delete, then checks foreground server/socket actually terminate; PR 1C adds process-group/reaping assertions |
| R02 | Baseline launches two owned servers with one name and compares socket inode; PR 1B/1C must reject duplicate without replacement |
| R03 | Baseline unsupported native window command checks exit/stderr; PR 1A turns this into failure-code/alias tests |
| R04 | Fake tmux argv probe checks read-only attach flag. Native observer role does not exist; PR 1B must forge write/resize/kill on an actual observer connection and verify denial plus local detach |
| R05 | Fake Zellij/Screen output with `foobar`, then create/delete `foo`; PR 2A adds exact real tmux and available-adapter tests |
| R06 | Invalid cwd/profile/backend is checked for zero create mutations through fake recorder; real multi-pane sentinel tests in PR 3A |
| R07 | Unverified dynamically: no framed parser exists. PR 1B tests each header/payload split, coalescing, oversized/truncated frames and rejected side effects, then fuzzes the CGO-free parser |
| R08 | Socket attach/disconnect and real CLI PTY detach/TTY restoration; coordinator failure extension waits for M2 |
| R09 | Native standalone input checks PTY echo after accepted CLI write; PR 2A adds literal `Enter`, `C-c`, dashes and Unicode on exact tmux pane IDs |
| R10 | Config path is a directory, causing deterministic read failure even as root; check default verbosity for panic; PR 1A adds verbosity/config/version/alias policy tests |
| R11 | Fake profile preflight catches ignored backend/cwd/prefix assumptions; real tmux indexes 0/1, focus changes and returned IDs wait for PR 3A |
| R12 | Create missing executable must return nonzero; PR 1C adds identity/readiness/duplicate races and short-lived children; durable receipts wait for M2 |

The table records Phase 0 procedures; the Phase 1 evidence below supersedes its
native/CLI implementation status. `FuzzNativeFrame` now exercises the CGO-free
frame decoder/encoder; strict JSON has unit/adversarial coverage, not a claimed
JSON fuzz pass. M1 does not depend on provider CLIs or coordinator recovery tests.

## CI and platform gates

Ubuntu CI builds/tests with the locked Zig and module-minimum Go, runs guard tests,
native/tmux integration, Go race tests, bounded frame fuzz and native lifecycle
integration with a race-enabled executable. A macOS 14 job builds/tests and
runs the same integration with tmux. CI configuration is not evidence that a
remote job has run: do not push merely to produce a passing badge.

Linux amd64 local evidence does not certify arm64, musl, macOS or Windows. The
M1 release needs real macOS runtime evidence and actual archive smoke tests for
every advertised target. Windows native remains unsupported until tested with
real transport/PTY support. `goreleaser check` validates configuration only; no
release or publication is authorized by Phase 0.

Before each milestone: save exact commands/results, SHA/worktree, OS/arch and
backend/tool versions; run applicable golden, race/fuzz, adversarial and manual
TTY checks; verify archive names/completions/man page and supported capabilities.
Record unsupported hosts and missing tools. Expand startup/list/attach/RSS/PTy
measurements to 10/100-session scale only after M1 isolation and lifecycle fixes.

## Phase 0 execution evidence

Baseline application SHA: `f2606d240912ffa2269569cd642a7f83e5457f74`. Linux
amd64, kernel `7.0.0-38-generic`, Go 1.27.3 and 1.26.10, Zig 0.16.0,
tmux 3.4, libghostty Go binding unchanged. Zellij/Screen unavailable.
Builds used `/tmp/txm-p0-Yso5Rk/repo`, separate dependency checkout
`/tmp/txm-p0-Yso5Rk/ghostty`, and prefix `prefix-compatible` under that root.
The original project `ghostty/` remains clean at `d4c88d806`; its `bin/` was not
used or replaced. No real sessions, remote hosts or worktrees were manipulated.

| Check | Observed result |
| --- | --- |
| Old Ghostty pin + Zig 0.16.0 | Library builds; txm CGO fails on missing C API symbols; baseline incompatibility confirmed |
| Corrected pin + shared helper | Pass; source/version/artifact checks, including repeated incremental build |
| `make build test` in disposable checkout, Go 1.27.3 | Pass; all three original Go test files pass |
| `go test ./...`, Go 1.26.10 with explicit matching GOROOT | Pass; initial attempt inherited Go 1.27 GOROOT and failed, corrected before reporting pass |
| `go test -race ./...`, Go 1.27.3 | Pass on existing tests; does not cover native concurrency defects |
| `make test-integration` | Pass: build, guard suite and five real tmux/native smoke checks |
| Final Python guard suite | Nine tests: source/cache/version failures, marker/socket replacement, symlink refusal, and two-server isolation |
| `integration.py --baseline` | Expected exit 1: five smoke passes, ten desired-behavior failures; details below |
| `make lint`, golangci-lint 2.15.0 | Existing failure: unchecked receive from errChan at `pkg/backend/native.go:204`; PR 1A/1C |
| `goreleaser check` | Exit 2: structurally valid config but deprecated snapshot/archive keys; packaging gate remains open |
| Shell syntax, Python compilation, `git diff --check` | Pass; no new Go production changes |
| macOS job / other release targets / frame fuzz | Configured but not run / not validated / parser not implemented; no passing claim |

Exact successful build/test invocation, from the disposable checkout:

```sh
env -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY -u http_proxy -u https_proxy -u all_proxy \
  GOCACHE=/tmp/txm-plan-review-go-cache GOTOOLCHAIN=local GOPROXY=off \
  ZIG_GLOBAL_CACHE_DIR=/tmp/txm-p0-Yso5Rk/zig-cache GHOSTTY_BUILD_JOBS=2 \
  make build test \
  GHOSTTY_SOURCE_DIR=/tmp/txm-p0-Yso5Rk/ghostty \
  GHOSTTY_HOST_DIR=/tmp/txm-p0-Yso5Rk/prefix-compatible \
  ZIG=/tmp/txm-p0-Yso5Rk/zig-x86_64-linux-0.16.0/zig
```

The same environment/overrides were used for `make test-integration` and `make
lint`. Race tests used the same PKG_CONFIG_PATH/GOCACHE/CGO settings. Go 1.26.10
used its installed module toolchain executable, matching GOROOT and a separate
`/tmp/txm-p0-Yso5Rk/go126-cache`. Official Zig archive SHA-256 was
`70e49664a74374b48b51e6f3fdfbf437f6395d42509050588bd49abe52ba3d00`.

Machine-readable evidence: [smoke](evidence/phase0-smoke.json) and
[baseline](evidence/phase0-baseline.json). The baseline failures are R01, R02,
R03, R04 tmux argv, R05 Zellij, R05 Screen, R06/R11 profile preflight, R09 native
input, R10 config panic, R12 startup failure. Fake adapter results prove control
flow defects, not installed provider behavior. Native observer/framing/full pane
matrix tests remain explicitly unverified with procedures above.

Small-fixture smoke measurements (Debug Ghostty build, Go 1.27.3; no thresholds):

| Measurement | Sample |
| --- | --- |
| CLI `version` startup | Median 6.45 ms over 10 subprocesses |
| Native list, one session | 6.15 ms, one sample |
| Native socket attach | 20.41 ms, includes 20 ms observation polling granularity |
| CLI attach on real PTY | 21.16 ms, includes the same polling granularity |
| Native server RSS | 21,888 KiB before throughput workload |
| PTY/VT/fanout throughput | 262,292 bytes at 0.0353 MiB/s, one sample |

A later make-target smoke repeated successfully with startup 3.89 ms, list
4.88 ms, attach 20.38/21.12 ms, RSS 21,088 KiB and throughput 0.0518 MiB/s.
These noisy single-workload results establish observations, not acceptable
performance. Resolve throughput/resource limits against measured thresholds in
M1; do not claim the lightweight/performance release goal has passed.

Phase 0 prep is complete; its final guard/diff review passed. Known baseline
correctness, lint and packaging failures remain mandatory later work; M1 is not
release-ready. No commits, pushes, PR publication, merges or releases were made.

## Phase 1 execution evidence

Phase 1 is implemented and validated on Linux amd64, against the same baseline
SHA, locked Ghostty/Zig and installed Go/tmux versions above. Changes remain in
the working tree, including the earlier Phase 0 work; there is no implementation
commit or published PR. The original ignored dependency checkout and binaries
remain untouched. Machine-readable results are in
[phase1-validation.json](evidence/phase1-validation.json) and the final
[smoke report](evidence/phase1-smoke.json).

| Check | Observed result |
| --- | --- |
| Disposable `make build test` / Go 1.27.3 | Pass with locked dependencies |
| `go test ./...`, Go 1.26.10 with matching GOROOT | Pass |
| `go test -race ./...`, Go 1.27.3 | Pass, including framing, VT/queue/deadline, runtime-path, process-wait and CLI unit tests |
| Guard suite | Nine pass; private paths, build failures and protected second tmux server |
| `integration.py` | Five pass; real native PTY attach/detach/TTY restore, output and isolated tmux lifecycle |
| `cli_errors.py` | Four groups pass; session/window/pane aliases, one stderr diagnostic, bad config at default/verbose levels, independent help/version/completion, empty-vs-failed discovery and read-only flags |
| `native_checks.py`, normal executable | 11 pass, final run 25.528 s |
| `native_checks.py`, race executable | 11 pass, final run 47.881 s; fixture stderr checked for race reports |
| `FuzzNativeFrame`, 30 s, two workers | Pass, 1,609,524 executions; frame codec only |
| `golangci-lint run` 2.15.0 | Pass, zero issues; the baseline native lint failure is resolved |
| Windows amd64 and arm64, `CGO_ENABLED=0 go build` | Pass, build-only; native availability is false |
| Man-page rendering, Python compilation, shell syntax, gofmt and diff whitespace | Pass |
| macOS runtime / other architecture runtime / release archives | Unavailable or not run; no passing platform/release claim |

Native adversarial coverage includes fragmented/coalesced/invalid/version-skewed
frames and duplicate JSON fields; observer input/resize/kill denial; real PTY
resize and acknowledged input; exact kill and resistant descendant process-group
cleanup; duplicate starts and early exit/readiness; stale sockets, symlinks and
unsafe runtime permissions; legacy socket preservation and nuke refusal; bounded
connections and cancellation during a partial handshake; read-only detach and
signal-time TTY restoration; and four simultaneous viewers with concurrent
resize, UTF-8 output and snapshot requests. Real tmux verifies the attached
client's read-only flag. Unit tests additionally exercise independent slow-client
queues, heartbeat expiry, capture byte/line limits, resize rollback, secure log
files and cancelled lock/input waits. No claim is made for a foreign-UID
end-to-end test, all possible same-user races, or downstream provider behavior.

Use the build environment from the Phase 0 command above. The native unit/race
and lint commands use:

```sh
export PKG_CONFIG_PATH=/tmp/txm-p0-Yso5Rk/prefix-compatible/share/pkgconfig
export CGO_ENABLED=1 GOCACHE=/tmp/txm-plan-review-go-cache
export GOTOOLCHAIN=local GOPROXY=off
go test -race ./...
golangci-lint run
go test ./pkg/nativeprotocol -run='^$' -fuzz=FuzzNativeFrame -fuzztime=30s -parallel=2
# Run in the disposable checkout to build both executables and all fixtures:
make test-integration test-integration-race \
  GHOSTTY_SOURCE_DIR=/tmp/txm-p0-Yso5Rk/ghostty \
  GHOSTTY_HOST_DIR=/tmp/txm-p0-Yso5Rk/prefix-compatible \
  ZIG=/tmp/txm-p0-Yso5Rk/zig-x86_64-linux-0.16.0/zig
```

The Make integration targets passed before the last concurrency case was added;
both final 11-case suites were then run directly using the binaries at
`/tmp/txm-p0-Yso5Rk/repo/bin/txm` and `txm-race`. Final CLI/smoke checks used
`/tmp/txm-phase1/txm`, which also includes the completion-generation error fix;
that fix is covered by CLI unit/race tests. Artifact hashes and source fingerprints
are in the validation JSON. No native implementation changed after those final
native suite runs.

R01/R02/R03/R04/R07/R08/R10/R12 and native R09 now have passing Phase 1 coverage
for the Linux cases described above. This does not complete the all-backend
R05/R09 or profile R06/R11 matrix, public JSON/error classification, M2 receipt
and recovery tests, or the M1 release gate. Exact Screen cleanup and an error-aware
compatibility lookup moved forward from Phase 2 to avoid wrong-target cleanup or
creation following a failed probe; full typed targeting remains Phase 2 work.
Native contexts, bounded snapshots and platform stubs are prerequisites of the
Phase 1 transport/lifecycle work, not completion of the Phase 2 output API.

Final small-fixture sample: startup median 4.46 ms (10 runs), native one-session
list 5.75 ms, socket attach 0.71 ms, CLI PTY attach 21.24 ms (20 ms observation
polling floor), RSS 25,020 KiB, and 262,292 bytes at 0.0481 MiB/s through the Debug
VT/fanout workload. These are noisy observations without acceptance thresholds;
the scale/performance and actual release-archive gates remain open. M1 macOS
runtime validation is blocked on an available host/runner; configuring CI does
not satisfy it. Windows native, Zellij/Screen real-runtime capabilities and other
unrun platforms retain the conservative support limits above. The existing
GoReleaser deprecation warning remains packaging work for the release gate.

Phase 1 stops here, before Phase 2. No real sessions, remote hosts or worktrees
were changed; no commits, pushes, merges or releases were made.

## Phase 2 execution evidence

Phase 1's Go-source fingerprint and baseline SHA were revalidated before Phase 2.
All work remains uncommitted on baseline `f2606d240912ffa2269569cd642a7f83e5457f74`,
with the Phase 0/1 work preserved. Phase 2 implements typed backend observations,
exact ID resolution, strict selection, capabilities, public JSON/classified exits,
private host identity and bounded capture/preview. It stops before Phase 3 profiles
and the M1 release gate. The old original ignored `ghostty/` and `bin/` remain
untouched; final integration used a separate `/tmp/txm-phase2/repo` checkout and
the same locked dependency checkout/prefix as Phase 0/1.

| Final gate | Observed result |
| --- | --- |
| `make test-integration` in disposable checkout | Pass: locked build, nine guards, five smoke checks, four CLI groups, 11 native cases (25.519 s), 15 Phase 2 groups (18.240 s) |
| `make test-integration-race` | Pass: shared nine-guard prerequisite; race executable passes the same 11 native cases (47.936 s) and 15 Phase 2 groups (62.308 s), with server race stderr checked |
| `go test -race ./...`, Go 1.27.3 | Pass, including new core goldens/errors/sanitizer, private identity and external-runner tests |
| `go test ./...`, Go 1.26.10 / matching GOROOT | Pass |
| `golangci-lint run` 2.15.0 | Pass, zero issues |
| `make test-fuzz` | Pass: frame codec 30 s / 1,731,055 executions; plain sanitizer 10 s / 294,221 executions, two workers each |
| Windows amd64/arm64, `CGO_ENABLED=0 go build` | Pass, build-only; native and unverified private structured state remain unsupported |
| Man rendering, Python compilation, shell syntax, gofmt, whitespace and evidence review | Pass |
| macOS/other runtime hosts, actual release archives and M1 scale thresholds | Not run; release gates remain open |

The 15 Phase 2 groups exercise empty/success/error/partial JSON, argument failures
before identity creation, concurrent first host-ID creation and corrupt-file
refusal, canonical prefix handling and strict unavailable/conflicting selection,
same-named sessions in aggregate results, deterministic error precedence, version
and capability failures, Screen full-ID/duplicate-name targeting, exact Zellij
lookup without force retries, zero backend calls for unsupported exact operations,
dynamic backend/qualified completions, malformed discovery without creation,
timeout and signal cancellation, collection caps against an infinite output
producer, safe native output/raw/history rejection, and real tmux targeting/input
and snapshot/history. Real tmux uses base-index and pane-base-index 1, duplicate
window names, current-focus changes and pane IDs from another session. Literal
input checks key names, leading dashes, spaces, Unicode, semicolons, `--json`,
`--` and synthetic shell-like text against a PTY byte reader. No user shell
command is executed to infer input success.

Unit tests additionally freeze representative envelopes and exit precedence,
strip CSI/OSC/DCS/APC/C1 and truncated UTF-8, check path/permission/corrupt-state
refusal, cancel/reap fake external queries, cap stderr without disclosing its
contents, retain the same resolved executable path, and reject implicit current-
directory executables (`exec.ErrDot`). Integration fixtures provide the private
HOME/XDG/PATH/socket/process boundaries documented above. Fake Screen/Zellij
results validate control flow, not installed-provider runtime support.

Implementation findings and deviations:

- tmux replaces control separators in pane-list formatting, so metadata uses a
  printable separator with strict field validation. Unsupported metadata fails
  before mutation. Its literal-key mode also strips trailing semicolons; bounded
  hex-byte batches plus separate Enter preserve the literal bytes on a real PTY.
- Compatibility action/read wrappers remain while CLI reads use `core.Reader`;
  the profile's old boolean existence probe was replaced with an error-aware
  lookup. Full profile preflight/cwd/layout/rollback is still Phase 3.
- The retained short-lived-child regression initially expected only general exit
  1. Phase 2 may instead classify a handshake observing exit as not-found (3).
  Its updated assertion verifies that diagnostic, empty stdout and socket/process
  cleanup; a surviving session is never treated as successful cleanup.
- Cancellation takes exit 130 even when an aggregate already has partial data.
  Unavailable/incompatible capability entries do not advertise support. Zellij
  live process state remains unknown because listings may include resurrection
  metadata; exact pane operations stay unsupported.

Exact final build/integration environment (run from `/tmp/txm-phase2/repo`):

```sh
env -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY -u http_proxy -u https_proxy -u all_proxy \
  GOCACHE=/tmp/txm-plan-review-go-cache GOTOOLCHAIN=local GOPROXY=off \
  ZIG_GLOBAL_CACHE_DIR=/tmp/txm-p0-Yso5Rk/zig-cache GHOSTTY_BUILD_JOBS=2 \
  make test-integration \
  GHOSTTY_SOURCE_DIR=/tmp/txm-p0-Yso5Rk/ghostty \
  GHOSTTY_HOST_DIR=/tmp/txm-p0-Yso5Rk/prefix-compatible \
  ZIG=/tmp/txm-p0-Yso5Rk/zig-x86_64-linux-0.16.0/zig
# Same environment/overrides for make test-integration-race.
# Workspace unit/race/lint commands use the Phase 1 PKG_CONFIG_PATH environment.
make test-fuzz
```

Evidence: [validation/fingerprints](evidence/phase2-validation.json),
[normal integration output](evidence/phase2-integration.txt),
[race integration output](evidence/phase2-integration-race.txt),
[fuzz output](evidence/phase2-fuzz.txt), and
[small-fixture smoke sample](evidence/phase2-smoke.json).
The sample measured startup median 4.79 ms, one-session native list 7.46 ms,
socket attach 0.59 ms, CLI PTY attach 21.02 ms, RSS 24,972 KiB and 262,292 bytes at
0.0450 MiB/s through the Debug VT/fanout workload. These noisy samples have no
release thresholds and were not 10/100-session scale benchmarks.

Linux amd64 and real tmux 3.4 are the validated runtime scope. macOS remains
blocked on a host/runner; no native Windows, musl/arm64, Screen/Zellij real runtime,
foreign-UID end-to-end, actual release-archive, profile or M2/M3 pass is claimed.
Existing GoReleaser deprecations and M1 platform/performance acceptance remain
release-gate work. No coordinator was introduced, no real user sessions or
worktrees were changed, and no commits, pushes, merges or releases were made.


## Phase 3 provisional resource budgets (set before measurement)

The Phase 0 Debug-build baseline was startup median 6.45 ms, one-session list
6.15 ms and RSS 21,888 KiB. For the M1 Linux comparison, provisional upper bounds
are startup median 20 ms, native list at 1/10/100 idle sessions 50/250/2000 ms,
and maximum server RSS 32,768 KiB. These permit moderate host noise and growth
while catching large regressions; they are review budgets, not a release SLA or
proof that Debug VT throughput is acceptable for every workload. Throughput is
compared with the Phase 0 sample separately. Real release artifacts and unrun
platforms retain their gates. `tests/performance.py` requires Linux /proc and at
least 4 GiB available memory before attempting 100 fixture-owned idle sessions.
It never measures or terminates a user's existing server.

## Phase 3 implementation and M1 gate (2026-10-30)

PR 3A is implemented on Linux amd64 with tmux 3.4. The M1 release gate is
**Blocked**, as recorded in [readiness](m1-readiness.md). Phase 2's three recorded
source/harness/build fingerprints and baseline SHA were verified before changing
code. Existing Phase 0/1/2 work and evidence were preserved, with no dependency
version changes. Final source fingerprints, artifact hashes, commands and limits
are in [phase3-validation.json](evidence/phase3-validation.json).

| Check | Available-host result |
| --- | --- |
| Go 1.26.10 and 1.27.3 tests; Go race; dependency verification | Pass |
| golangci-lint 2.15.0 | Pass, zero issues |
| Guarded `make test-integration test-integration-race` in disposable checkout | 10 isolation/build guards, 5 smoke checks, 4 CLI groups, 11 native cases, 15 structured groups, 16 profile groups pass; native/structured/profile suites also pass with race executable |
| Actual GNU and static musl Linux amd64 archive executables | Each passes all 5 smoke, 4 CLI, 11 native, 15 structured and 16 profile checks |
| Archive contents, completions, version and private user install | Both ZIPs pass; exactly 8 expected files; installed binary/man verified and generated profile completions exercised |
| Frame fuzz / plain-output fuzz | Pass: 30 s / 727,266 executions; 10 s / 160,218 executions (wall time includes fuzz shutdown) |
| Windows amd64/arm64 | CGO-disabled cross-builds pass; build-only, no native persistence/runtime/install claim |
| Linux resource comparison | Fixed provisional startup/list/RSS budgets pass with 1/10/100 private idle sessions; exact samples in the performance evidence |
| Other hosts and all-platform release workflow | Not executed; required release gates remain open |

R01/R02/R04/R07/R08/R09/R12 retain native lifecycle/framing/observer/startup and
real PTY tests, including enforced tmux read-only attachment. R03/R10 retain CLI,
config, aliases, classified errors and backend-independent version/completion
checks. R05 retains exact-name/ambiguity and refusal tests; Screen/Zellij runtime
support is not inferred from fakes. New R06/R11 checks verify returned window/pane
IDs at both index bases, focus changes, spaces/quotes/format characters/trailing
semicolons in cwd, actual layout geometry, per-pane input, prefix-once behavior,
existing-session conflict/unchanged attachment and read-only noncreation.
They also exercise the bare native picker and default tmux profile attachment on
controlling PTYs, with detach and terminal restoration.

Profile failure tests verify no input before full topology/layout construction,
owned-session rollback with protected neighboring sessions, missing creation
receipts and changed ownership (partial error 7 without deletion by name), and
SIGINT cancellation (130 with a fresh cleanup context). A concurrent fixture
client enables pane synchronization immediately before input: ordinary exec
refuses and profile apply safely rolls back. Each write/Enter rechecks the option
in the tmux server queue. Commands are accepted terminal input, never completion
receipts. Native is limited to one terminal; older tmux profile versions and all
Screen/Zellij profiles fail before mutation.

The final regression command is the retained Make target pair, using the same
pinned source, Zig, cache and host prefix as Phase 2. It includes the new
`tests/profile_checks.py` in normal and race runs. Archive verification uses
`python3 tests/archive_checks.py --archive PATH --report REPORT`, followed by the
same five runtime suites with `--binary` pointing to the extracted executable.
`python3 tests/performance.py --binary PATH --report REPORT` runs the finite scale
comparison. All runtime commands use guarded private HOME/XDG/PATH/socket roots
and fixture-owned process handles; sandbox permission was granted only for those
local PTYs/sockets. No original ignored `bin/` or `ghostty/` was rebuilt or cleaned.

The two real snapshot ZIPs were built with GoReleaser 2.19.0 using a disposable
copy of production configuration narrowed only to Linux amd64 in each build.
Matching pinned Ghostty GNU/musl libraries were built explicitly using the
release helper's target flags; the skipped before-hooks were replaced by those
builds and explicit completion generation. Snapshot mode skips publication.
Version is `1.3.2-next`; this is no tag, release or version bump in source.
`linux-smoke.yml`, build/install/runtime logs, archive reports and hashes are
recorded as evidence. The full eight-target release hook and workflow were not
executed. `goreleaser check` validates the production configuration without the
previous deprecated properties.

Archive testing found two concrete packaging issues: relative pkg-config paths
were resolved from Go module directories, and the host VT library did not match
the release GNU target ABI. Absolute `TXM_RELEASE_ROOT` paths and target-specific
libraries solve those failures; a fresh release GOCACHE avoids cached Cgo linker
paths from the earlier prefix. No ABI floor, compiler pin or dependency version
was changed to get a passing artifact. Completion installation differs from
standalone Bash generation only in Cobra's lazy placement of built-in help flags;
the archive check normalizes those four entries while comparing the rest.

Evidence: [integration/race](evidence/phase3-integration.txt),
[GNU archive runtime](evidence/phase3-archive-gnu-runtime.txt),
[musl archive runtime](evidence/phase3-archive-musl-runtime.txt),
[archive build](evidence/phase3-archive-build.txt),
[fuzz](evidence/phase3-fuzz.txt), and
[resource comparison](evidence/phase3-performance.json).
The final Debug-host scale sample measured startup median 5.24 ms, list at
1/10/100 sessions 7.37/15.74/116.52 ms and maximum idle server RSS 22,088 KiB.
In the retained small fixture, startup was 5.48 ms, list 9.67 ms, socket attach
1.20 ms, CLI PTY attach 21.76 ms, RSS 24,984 KiB and 262,292 bytes at
0.0374 MiB/s. Phase 0's corresponding attach was 21.16 ms and throughput
0.0353 MiB/s; these single Debug VT/fanout samples include host noise and do not
establish a production throughput or attach SLA. GNU/musl archive smoke reports
retain their separate samples rather than combining unlike builds.
macOS, Linux arm64, Windows archive/install/runtime and foreign-UID end-to-end
checks remain unrun. Configured CI is not host evidence. Phase 4/M2, coordinator,
managed receipts and provider integrations remain pending; no user resources,
remote hosts, commits, pushes, tags or published releases were changed.

## Phase 4 supervisor and coordinator (2026-10-30)

The explicit Phase 4 request authorizes Linux development while M1's unrun
platform release gates remain blocked. PR 4A/4B are implemented for native and
tmux 3.4+ on the tested Linux amd64 host. No M1/M2 release certification, Phase 5
provider/hook/event/wait work or publication is implied. Phase 3's baseline and
three source/harness/build fingerprints were verified before editing.

| Check | Result and boundary |
| --- | --- |
| Go 1.27.3/1.26.10 tests, Go race, dependency verification | Pass |
| golangci-lint 2.15.0 | Pass, zero issues |
| Retained M1 normal/race integration | Pass: 10 guards, 5 smoke checks, 4 CLI groups, 11 native cases, 15 structured groups, 16 profile groups; native/structured/profile also pass with race executable |
| Final Phase 4 agent suite | 19 groups pass normally and with race executable; per-fixture GORACE logs cover relaunched coordinator/wrapper/native processes |
| Final actual GNU/static-musl Linux amd64 snapshot ZIPs | Each passes private install/content/completion checks plus all 5 smoke, 4 CLI, 11 native, 15 structured, 16 profile and 19 agent groups |
| Native frame / plain-output / local IPC fuzz | Pass: 30 s / 1,324,136; 10 s / 24,694; 15 s / 607,628 executions respectively; shutdown can extend wall time |
| Windows amd64/arm64 | CGO-disabled cross-builds pass; no Windows runtime/archive/install claim |
| Linux CGO-disabled runtime | Agent commands reject with unsupported (4) before runtime/state creation |
| Fixed resource budgets | Pass at 1/10/100 private idle native sessions; no agent service involved |

New coverage verifies direct argv (quotes, Unicode, leading dashes and shell-looking
strings), real foreground PTYs/cwd, environment-name filtering, and one-use
mode-0600 manifests. Native/tmux exit 0/42/signal receipts retain identical IDs,
outcomes and timestamps after service restart. Failed exec records start_failed
without an invented child exit code. Coordinator SIGKILL/stop leaves children
running; exit receipts remain queryable with no re-execution. Killing a wrapper
without an exit observation produces unknown, not success. Group cleanup handles
a resistant descendant before releasing the direct child identity.

Coordinator checks include concurrent automatic startup, singleton/state-writer
locks, duplicate starts, exact registration, rename/name reuse, strict framing,
epoch mismatch, slow clients, SIGINT/SIGTERM cancellation (130), torn/corrupt
registry and mismatched receipts. Changing XDG_STATE_HOME, including copying the
host-ID file, cannot select the wrong running registry: both host ID and validated
state location are checked before any operation. Interrupted launch intents stay
unknown and unused manifests are discarded under a free lifetime lock; recovery
never replays them. Unsupervised registration has no managed ID, launch identity
or exit receipt. Ordinary M1 reads/aliases/completions work without the service,
even with corrupt coordinator metadata present.

The full Make target pair initially exercised the retained M1 suites and the
then-16 agent groups. After adding restart/state-location cases and fixing the
state-identity contract, the final 19 agent groups were rerun against fresh normal
and race executables; both final archives reran every runtime suite. The retained
M1 code paths did not change in that final coordinator-only fix. Exact commands,
source/artifact fingerprints and this sequencing are preserved in
[phase4-validation.json](evidence/phase4-validation.json).

Final scale samples: startup median 4.77 ms, list at 1/10/100 sessions
7.11/15.20/106.12 ms, maximum idle server RSS 25,300 KiB. The Phase 3 budgets
(20 ms startup, 50/250/2000 ms list, 32,768 KiB RSS) were retained unchanged.
These host/Debug-VT samples are not a production SLA, coordinator scale test or
claim of zero overhead. Archive smoke keeps separate attach/throughput samples.

Evidence: [final normal agents](evidence/phase4-agents.txt),
[final race agents](evidence/phase4-agents-race.txt),
[retained integration/race](evidence/phase4-integration.txt),
[GNU archive](evidence/phase4-archive-gnu-runtime.txt),
[musl archive](evidence/phase4-archive-musl-runtime.txt),
[archive build](evidence/phase4-archive-build.txt),
[fuzz](evidence/phase4-fuzz.txt), and
[resource comparison](evidence/phase4-performance.json).
Snapshot packaging follows Phase 3's narrowed production Linux configuration,
matching pinned target libraries, explicit completion generation, fresh release
Go cache, and no publication. Generated agent flags are checked in the archive.
The complete all-platform release workflow was not executed.

Initial failed test runs exposed fixture permissions and Linux per-thread child
enumeration; fixtures now set private modes and retain pidfds/Popen identities
before signals. A test-client connection-failure socket leak was also closed.
The Phase 2 capability expectation intentionally changes managed_launch to
supported for verified native/tmux builds; other adapter/target guarantees remain.
No dependency versions, original ignored bin/ghostty trees, real sessions or
worktrees were changed. macOS/other-architecture runtime/archive gates and the
foreign-UID end-to-end scenario remain unverified. State has a 256-registration
limit with no automatic pruning, child environments may predate the client, and
escaped process groups are not contained. M2's release gate remains Phase 5 work.

## Phase 5 signals, events, waits and optional actions (2026-10-30)

The new request authorizes Linux development while preserving the unrun M1
platform release gates. Phase 4 source/harness/build/documentation fingerprints
were checked before editing. Exact commands, source/artifact fingerprints, logs,
provider versions and outstanding limits are in
[phase5-validation.json](evidence/phase5-validation.json). The release decision is
[M2 readiness](m2-readiness.md); Phase 6 is not started.

The 14 new private-fixture groups cover authenticated token/instance/generation
association, replay/stale/future rejection, credential rotation and private modes;
separate lifecycle/attention/turn state; explicit generic child context on native
and tmux; atomic initial snapshots plus future transitions; timeout 6/cancellation
130/nonzero child handling; unsupported unsupervised/missing-hook waits; missing
receipts and stale attention; coordinator crash/reconnect with a new epoch and no
re-execution; subscriber limits/disconnect; conservative Claude/Codex translation
and provider session binding; provider absence/version refusal and ordinary M1
independence; opt-in safe action payloads, throttling/no snapshot replay, slow
action deadlines and cancellation; and real Claude `--init-only` SessionStart.

Go tests additionally exercise deterministic queue overflow/resync without
blocking a fast subscriber, attention expiry without lifecycle changes, bounded
provider JSON validation/translation and secret-free generated configuration.
Overflow is verified at the broker level rather than by relying on nondeterministic
OS socket-buffer sizes. A silent subscriber's writes have a two-second deadline.
No desktop popup or actual model/tool conversation is triggered by these checks.

Real-provider verification uses installed Claude 2.1.308 with a constructed
private HOME/settings namespace and disabled nonessential traffic, no credentials,
no prompt and no permission bypass. Its initialization hook binds a real session;
its actual process exit remains a separate supervisor receipt. Codex 0.165.0 has
installed version/feature inventory and observational schema/config fixtures;
normal user hook trust review and full provider execution remain unrun. OpenCode
2.2.1 is explicitly unsupported after review of its v2 docs. Optional unsupported
providers do not block the generic CLI workflow.

The first generic fixture used `signal.py`, shadowing Python's standard library;
it was renamed. The initial Go 1.26 command inherited the host Go 1.27 GOROOT;
the corrected minimum-version check sets its own matching GOROOT/cache. Lint's
cache was redirected into /tmp. These failed setup attempts are retained separately
from final passing evidence. One automatic approval review for the private runtime
suite timed out; its allowed retry succeeded. No approval was bypassed.

Normal/race tests and archive checks use only guarded fixture resources. Local
snapshot packaging narrows the production matrix to Linux amd64 GNU/static-musl,
builds matching pinned target VT libraries, generates completions, checks actual
ZIP installation/content, and publishes nothing. Other platform runtime/archive
checks remain unrun. Notification scripts are fixture programs; registry pruning,
process-group escape containment and arbitrary provider-version compatibility are
not claimed.

Final results: Go 1.26.10/1.27.3, Go race, dependency verification and zero-issue
lint pass. Ten harness guards, five smoke checks, four CLI groups, 11 native
cases, 15 structured groups, 16 profile groups, 19 retained agent groups and 14
new Phase 5 groups pass; every runtime suite passes normally and with the race
executable. Both actual GNU/musl ZIPs pass private install/content/completion
checks and every runtime suite, including real Claude initialization. Windows
amd64/arm64 builds are compilation-only; Linux CGO-disabled agent start/wait/events
return unsupported (4). Native/plain/local-frame fuzz pass 30/10/15-second runs
with 1,520,064 / 45,361 / 587,795 executions.

The fixed resource budgets remain unchanged. Final samples are recorded in
[phase5-performance.json](evidence/phase5-performance.json): list at 1/10/100
sessions took 7.37/17.60/102.23 ms and maximum idle server RSS was 25,344 KiB.
These samples ran after archive/fuzz work finished, with no agent coordinator.
Exact startup timing, budgets and scope are in that report. Current and archived
Go/embedded sources and harness/golden fingerprints match. No final code change
followed the passing checks; only documentation/evidence/ledger were finalized.

Evidence: [new normal groups](evidence/phase5-signals.txt),
[new race groups](evidence/phase5-signals-race.txt),
[retained integration](evidence/phase5-integration.txt),
[GNU archive](evidence/phase5-archive-gnu-runtime.txt),
[musl archive](evidence/phase5-archive-musl-runtime.txt),
[archive build](evidence/phase5-archive-build.txt), and
[fuzz](evidence/phase5-fuzz.txt). All exact commands, result codes, hashes and
provider inventory are indexed by [phase5-validation.json](evidence/phase5-validation.json).

## Phase 6 dashboard (2026-10-30)

The explicit request authorizes Linux dashboard development while preserving the
M1/M2 release blockers. Phase 5 source/harness/build/documentation fingerprints
were verified before edits. Phase 6 adds `pkg/dashboard`, a Cobra adapter and
small typed backend action/capture helpers; it preserves public M1/M2 schemas,
provider limitations and the existing bare picker. The exact result index is
[phase6-validation.json](evidence/phase6-validation.json); release status is
[M3 readiness](m3-readiness.md).

Six model/simulated-screen groups cover stable selection after reordering,
backend/type/state/search filters and bounds, unassociated registrations,
source/per-agent freshness, control sanitization/Unicode/small screens, a blocked
backend with prompt quit, single-worker generation-checked previews, repeated
read-only attach/error/resume cleanup, and deny-only native ancestry parsing.

Fourteen real PTY groups render through the existing native VT, without a second
terminal parser. They check non-TTY/invalid-option preflight with no backend/state
access; native refresh/search/plain preview and resize; repeated read-only and
control attach with no input leaks; actual isolated tmux role/attach/detach;
SIGINT/SIGTERM/raw Ctrl-C restoration, including cancellation during native/tmux
attach; independent slow/unavailable backends and worker shutdown; disabled
unsupported actions; stale native selection refusing a replacement with the same
name; self-PTY attach/preview prevention; sourced separate agent process,
attention, turn and exit-42 receipt state; missing-heartbeat uncertainty and
recovery without relaunch; unchanged live-agent proof refreshed through typed M2
snapshots; and fixed 1000-session response/RSS budgets.

Final Go 1.27.3/1.26.10 tests, Go race, dependency verification and zero-issue lint
pass. Windows amd64/arm64 and macOS amd64/arm64 CGO-disabled cross-builds pass,
with compilation-only scope. Linux CGO-disabled agent start/wait/events refuse
unsupported; dashboard non-TTY use returns 2 without state/runtime creation.
Non-Linux dashboard runtime is explicitly disabled pending equivalent ancestry
and TTY verification. This does not waive the required macOS release gate.

Ten harness guards, five smoke checks, four CLI groups, 11 native cases, 15
structured groups, 16 profile groups, 19 retained agent groups, 14 Phase 5 groups
and 14 dashboard groups pass normally and with the race executable. Actual
GNU/static-musl Linux amd64 ZIPs pass private installation, archive content,
completion checks and every runtime suite. The full all-platform release workflow
was not executed. Three retained fuzz targets pass 30/10/15-second runs with
1,438,409 / 165,638 / 456,033 executions; fuzz target sources are unchanged by the
final dashboard-only fixes.

After the complete integration run, final review tightened only the dashboard
model's visible backend filter. M1/M2 production sources were unchanged. All Go,
minimum-version, race, lint and build checks were repeated; rebuilt ZIPs reran
installation/completions, and all 14 dashboard groups reran normally, under race,
and from both final ZIPs. The result index records this sequencing rather than
attributing earlier runs to the final model hash.

Setup failures are kept separate from passing evidence. A simulated attach test
initially matched retained stale data before a new refresh; it now waits for fresh
data and bounds its waits. Real PTYs exposed clipped stale text and an ambiguous
Escape/query interaction, now corrected. Review caught self-attachment feedback
and heartbeat-only agent freshness; dedicated regressions cover both. Only
verified checkpoint process handles/private fixture resources were stopped; no
real user process was targeted. An existing native-capacity test assumed an
excess client could write HELLO before refusal. EPIPE is now accepted alongside
EOF/reset as refusal; all capacity, live-peer and recovery assertions remain.
The affected complete suite was rerun successfully.

The [synthetic asciicast](evidence/phase6-dashboard.cast) is an explicit test-only
recording of fixture workers, needs-input, turn completion, exit 42, bounded plain
preview and read-only inspection. It contains ephemeral IDs and synthetic labels,
with no real project, prompt, credential or model conversation. Dashboard itself
never records output. Generated archive completions expose dashboard flags and
match the installed binary. Existing SSH examples are illustrative; no SSH host,
worktree extension, real desktop popup, commit, tag or publication was involved.

Evidence: [dashboard normal](evidence/phase6-dashboard.txt),
[dashboard race](evidence/phase6-dashboard-race.txt),
[retained integration](evidence/phase6-integration.txt),
[GNU archive](evidence/phase6-archive-gnu-runtime.txt),
[musl archive](evidence/phase6-archive-musl-runtime.txt),
[archive build](evidence/phase6-archive-build.txt), and
[normal metrics](evidence/phase6-dashboard-metrics.json).

Final normal dashboard samples with 1000 sessions: search 33.57 ms, resize
283.47 ms, quit 20.76 ms, RSS 30,604 KiB; all are within the predeclared 500 ms /
65,536 KiB budgets. Race input/resize/quit also pass; its 72,304 KiB RSS is
instrumentation diagnostics, not the normal memory gate. GNU/musl samples are
indexed separately. After all runtime work finished, an isolated retained M1
sample passed unchanged budgets: startup 6.91 ms, list 1/10/100 at
9.23/20.48/103.10 ms, maximum idle server RSS 25,504 KiB. The earlier concurrent
sample also passed but had higher timings; both reports are retained. These are
host/fixture observations, not a cross-platform or coordinator-scale SLA.
The final synthetic recording is about 5.55 seconds with 91 output events.

## Phase 7 extensions

Run make test-extensions (also included in normal/race integration targets).
The suite provisions disposable Git repositories, fixture-specific native/tmux
sockets and fake SSH; it never contacts a real configured host. Git and tmux are
required for the full Linux gate. Remote decoder fuzzing is in make test-fuzz.
See [extension readiness](extensions-readiness.md) and
[the evidence manifest](evidence/phase7-validation.json).

## Phase 8 reproducible final validation

Build normal/race binaries and GNU/static-musl archives in a disposable checkout
with the pinned dependency/toolchain, using the build commands above. Then run:

```sh
python3 scripts/validate-runtime.py \
  --binary /tmp/txm-build/txm --race-binary /tmp/txm-build/txm-race \
  --archive /tmp/txm-build/txm_Linux_x86_64.zip \
  --archive /tmp/txm-build/txm_Linux_x86_64-musl.zip \
  --output /tmp/txm-final-new-directory
python3 scripts/audit-artifacts.py /tmp/txm-build/txm \
  /tmp/txm-build/txm_Linux_x86_64.zip \
  --report /tmp/txm-privacy-new-report.json
```

The runtime runner requires Linux, Git and tmux, refuses existing output paths,
checks actual race build metadata, runs guard tests and all nine native/tmux/
agent/dashboard/extension suites per supplied binary, and tests ZIP installation,
man page and all shell completions. Captured output stays in separate fixture logs;
source/harness and artifact hashes must stay unchanged throughout. Performance
runs alone after the concurrent test workers finish. No provider CLI, model call,
real SSH alias, system installation or publication is invoked. A 900-second
outer timeout is a failure requiring inspection, not a guessed process cleanup.
The built-in harness has its own shorter deadlines and owned teardown.

The privacy scanner bounds ZIP members and scans across chunk boundaries, emits
rule/offset/path only, and checks compressed members as well as the outer ZIP.
It detects selected known credential forms, not arbitrary sensitive prose. Pair
it with review of synthetic fixture provenance, private launch-manifest handling,
explicit logging opt-in, safe event fields and artifact allowlists. Passing this
scanner is not a general data-loss-prevention guarantee. Exact final commands,
results, performance comparisons and remaining gates are recorded in
[evidence](evidence/phase8-validation.json) and the [audit](final-audit.md).

## Linux container follow-up

See [the dedicated reproduction and isolation guide](linux-followup.md). It
requires Docker and an explicit verified Zellij artifact; it adds actual
foreign-UID, OpenSSH loopback and Screen/Zellij runtime gates without installing
host services or reading user configuration. `tests/prune_checks.py` is now part
of the ordinary normal/race/GNU/musl runtime matrix and Makefile integration gates.
