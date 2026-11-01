# Development and releases

[Documentation](README.md) / Development and releases

Start with [Contributing](../CONTRIBUTING.md) for pull request expectations.
[Installation](user-guide.md#build-from-source) lists the build prerequisites.
This guide covers dependency configuration and validation without touching active
sessions or personal configuration.

- [Dependency configuration](#dependency-configuration)
- [Checks by change type](#checks-by-change-type) and [fixture isolation](#fixture-isolation)
- [Validate packaged builds](#validate-packaged-builds)
- [Release process](#release-process)

## Dependency configuration

[scripts/dependencies.env](../scripts/dependencies.env) pins Zig and the Ghostty
revision compatible with the Go binding. `make build` builds the native library
before compiling txm and stamps the version from `VERSION`. A direct Go build
defaults to `dev`. Run the examples below from the repository root.

The helper accepts these overrides:

| Variable | Purpose |
| --- | --- |
| `GHOSTTY_SOURCE_DIR` | Dedicated source checkout at the pinned revision |
| `GHOSTTY_HOST_DIR` | Native library installation prefix |
| `ZIG` | Path to the pinned Zig executable |
| `TXM_RELEASE_PLATFORM` | `linux` or `darwin` to build only that release partition; unset builds all targets |

Existing dependency checkouts must be clean and at the pinned revision. The
helper fails rather than resetting a different or dirty checkout. Build in a
separate checkout or use private paths when preserving existing build artifacts.
Inspect custom paths before running `make clean`, which removes build outputs
and the configured host prefix.

Direct Go tests need the native library's pkg-config metadata:

```sh
PKG_CONFIG_PATH="$PWD/.ghostty-host/share/pkgconfig" CGO_ENABLED=1 go test ./...
```

Use the actual verified prefix if you override `GHOSTTY_HOST_DIR`. Building the
native library needs CGO, a C compiler, and pkg-config. Go installation alone is
not sufficient. Python 3 and tmux are required for the integration harness;
`make lint` additionally requires golangci-lint.

## Checks by change type

| Change | Run |
| --- | --- |
| Go behavior | `make test test-race lint` |
| CLI, backend, or lifecycle behavior | `make test-integration test-integration-race` |
| Fixture/build/validation harness | `make test-harness` |
| Protocol or decoder | `make test-fuzz` plus relevant unit/integration checks |
| Performance | `make test-performance` |
| Release metadata or packaging | `make release-check` and actual archive checks |

For a complete local pass:

```sh
make build
make test test-race lint
make test-harness
make test-integration test-integration-race
make test-fuzz test-performance
make release-check
```

To focus an existing regression, pass its unittest selector:

```sh
python3 tests/native_checks.py --binary "$PWD/bin/txm" \
  NativeChecks.test_concurrent_attach_resize_and_output
```

For documentation, check local links and anchors, compare commands with help,
and run the release metadata checker. Update the man page when command semantics
change; it is embedded in the binary.

CI runs the Python harness once, then race-instrumented Go and full runtime
regressions once per OS. `make build-race` prepares the runtime binary;
`make test-runtime TEST_BINARY=/absolute/txm-race` runs the shared suite without
rebuilding dependencies or repeating the harness. Fuzzing runs once on Linux;
macOS backend and loopback SSH checks run once in source CI.

## Fixture isolation

`tests/sandbox.py` builds private HOME, XDG, runtime, temp, and environment
namespaces. tmux uses an owned foreground server with an explicit socket and
configuration. Native fixtures retain process handles and use framed IPC.
Cleanup checks ownership, namespace entries, and process identity before acting.
It never discovers personal sessions, worktrees, SSH configuration, or credentials.

PTY tests drain output and check terminal and descriptor restoration. Darwin
comparisons exclude only kernel bookkeeping bits. Fixtures protect against
accidental cross-test access; same-UID malicious code and escaping descendants
are outside their isolation guarantee.

Fixture readiness receipts are published by atomic replacement, so file presence
means complete JSON. After deliberately saturating native connection slots, only
the recovery probe retries temporary EOF/reset/broken-pipe refusals within a fixed
budget. A dead server, invalid protocol response, or pool that never recovers
still fails. Failed protocol handshakes close their client sockets.

Keep regression tests for shipped behavior. Save logs, recordings, manifests, and
archive outputs outside the repository or as CI/release artifacts. Include exact
commands, source/artifact hashes, OS, architecture, backend versions, and skip
reasons. A configured CI job or cross-build is not evidence of an executed runtime
gate.

Provider fixtures and live-provider validation are separate. The ordinary matrix
does not authorize authentication, inference cost, workspace/hook trust changes,
or use of personal provider configuration. Keep exact executable hashes,
platforms, versions and modes with a separately authorized live run. An SDK
loader test or a parsed callback fixture does not verify a provider lifecycle.
One-off live-run controllers and account-specific profiles are not part of the
regression harness; keep them with the private evidence for that authorized run.

`make test-opencode-runtime` additionally requires an explicit exact OpenCode
executable and a new report path through `OPENCODE_BINARY`/`OPENCODE_REPORT`.
It runs a private loopback SDK/loader/session-metadata fixture with provider use
denied and no inference. Version mismatch is a blocker, not a passing skip.
Canonical OpenCode lifecycle activation remains disabled.

`tests/record_demo.py --binary /absolute/txm --destination /absolute/new-demo`
records roughly 60 seconds of real generic CLI/PTY activity after installation.
It needs no provider account. Recordings and visual-review artifacts stay outside
the source tree.

## Validate packaged builds

The runtime runner takes existing binaries and archives. It verifies race
instrumentation, refuses an existing output directory, and binds results to the
source and artifact hashes. It tests at most two flavors concurrently and
measures performance after the workers finish.

```sh
python3 scripts/validate-runtime.py \
  --binary /absolute/check/txm --race-binary /absolute/check/txm-race \
  --archive /absolute/check/txm_Darwin_universal.zip \
  --expected-version "$(cat VERSION)" --macos-backends --loopback-ssh \
  --output /absolute/new-runtime-report
```

The example is for macOS. `--macos-backends` requires installed Screen and
Zellij 0.45.1 or newer (the fixture launches an explicit child command);
`--loopback-ssh` uses an unprivileged local sshd with fresh keys, strict known
hosts, a forced fixture endpoint, and private HOME/ZDOTDIR. It does not change
Remote Login or user accounts. `make test-ssh` runs the SSH suite alone on macOS.

For Linux amd64 foreign-UID, backend, and loopback SSH coverage, use
`python3 scripts/validate-linux.py --help`. That matrix runs root/foreign-UID
probes only inside disposable containers, with no external runtime network and
read-only fixture/artifact mounts. Never run those probes as host root.

`tests/archive_checks.py` validates ZIP inventory, version identity, private
installation, the man page, and completions. Match the archive to the runtime
host; cross-compilation does not replace a native runtime check.

The full 100-session performance fixture requires at least 4 GiB available
memory. If that preflight refuses, preserve the failure and rerun on a suitable
idle host; do not reduce the guard or report the scale gate as passing.
Smaller matched startup/1/10-session measurements must be labeled with their
actual scope and do not replace the full gate. Relative comparisons should use
the same toolchain, native dependency and harness, without concurrent runtime
workers; investigate repeatable regressions above 20%.

Use `scripts/audit-artifacts.py` on explicit outputs for known credential forms;
it reports redacted findings. Pair the scan with a review of fixture provenance.

The [release process](#release-process) defines the complete package gate sequence.
The macOS source job installs checksum-pinned Zellij using
`scripts/install-ci-zellij.sh`. `tests/peer_checks.py` shares actual foreign-UID
regressions between Linux containers and disposable macOS CI and refuses ordinary
host-root execution. Windows checks cover version/help/completions and explicit
unsupported refusals; they do not certify external backends or installers.

## Release process

Automatic pushes and pull requests into `main` run a lightweight scope and
release-metadata check. Changes limited to Markdown and `docs/txm.1` skip the
expensive Linux, macOS, and lint jobs. Code under `docs/`, mixed code/docs changes,
and changes whose scope cannot be determined still run the full suite. Manual
CI dispatches and reusable calls run the full suite by default.

Full archive builds and the seven-runner package matrix run only for version
tags or an explicit Release workflow dispatch.
Branch dispatches use snapshots stamped from `VERSION` and never create a draft.
Tag runs build and validate the official archives before draft creation.

1. Update `VERSION` and its dated changelog section, including upgrade notes.
   `make release-check` requires a stable version and matching metadata.
2. Run the applicable [tests](#checks-by-change-type), including actual archive
   installation and runtime checks. Save source/artifact hashes and reports
   outside the tree.
3. Commit the reviewed release preparation and merge it into `main`. Wait for
   that exact merge commit's full CI to pass (dispatch CI manually if automatic
   CI skipped documentation-only changes), then tag it with the version from
   `VERSION`: `git tag "v$(cat VERSION)"`.
4. A short Linux preflight checks stable metadata and the latest CI run for that
   exact commit on `main`, including successful Linux, Apple Silicon macOS, and
   lint jobs. Missing, pending, failed, cancelled, or skipped validation stops
   the release before compilation. It also requires an unexpired universal Mac
   artifact bound to that source commit; rerun full CI if it is unavailable.
5. The single Apple Silicon source job on `macos-latest` builds both Darwin
   slices and retains one universal archive. Release downloads that exact CI
   artifact and its original checksums rather than rebuilding it. A Linux
   GoReleaser job builds the four Linux and two Windows archives without
   publishing; an inexpensive Linux job assembles both partitions. All Mac
   jobs require ARM64 on the latest stable hosted image; there is no Intel
   runner or architecture-specific Mac archive.
6. `scripts/check-release-artifacts.py` checks the complete archive inventory,
   checksums, packaged documentation, binary OS/CPU headers, both bounded
   nonoverlapping universal Mac slices, and static-musl
   linkage. The same bundle is checked after each artifact download.
7. Each Unix archive passes private installation and a short packaged-code smoke
   check: native child startup, authenticated signals, exit receipts after
   coordinator shutdown, and child-PID CPU/RSS. Full regressions reuse the exact
   source CI result rather than running again for every ZIP. The universal Mac
   file runs on Apple Silicon; both slices are checked structurally. This is not
   a native Intel runtime claim. Both platforms test actual different-UID rejection using
   private fixtures: Linux in a disposable container, macOS as root only on the
   disposable CI runner. The Linux peer probe uses a pinned multi-architecture
   Python image directly, avoiding an unrelated SSH/tmux/Screen/Git image build.
   No accounts or host services are changed.
8. Windows x86_64/ARM64 runners check their executable's version, help,
   completions, and explicit unsupported refusals. These are CLI smoke checks,
   not external-backend or Windows installer certification.
9. Only after every package job passes does a write-enabled job create a
   **stable draft**, with seven ZIPs, checksums, and the canonical release notes.
   Review the tag/commit, jobs, reports, upgrade notes, and compatibility scope
   before manually publishing. Existing releases are not overwritten on rerun.

The bundle and package reports are retained as Actions artifacts.
Bundle privacy checks run once. Bounded annotations expose source/archive hashes
and verified report hashes, so evidence remains inspectable through the API when
an artifact-download host is unavailable.
Reports include the run attempt in their artifact names, so a successful retry
is distinct from an earlier incomplete report. On failure, use GitHub's **Re-run
failed jobs** or `gh run rerun RUN_ID --failed`. A package failure reuses the
existing bundle and other successful jobs; it does not repeat compilation.
If one build partition fails, its successful sibling is retained. Job timeouts
bound stalled work; unrelated package jobs finish so retries need not repeat them.

CI and release builds cache pinned Ghostty sources, native outputs, and Zig's
incremental cache by OS, CPU, SDK version/build, and dependency lock. Every restore
still runs the source/version guards and `zig build`; directory presence never
proves validity. Validated dependencies are saved before application compilation
and tests, so those failures do not discard the expensive dependency work. Go's
cache is scoped to the verified library bytes as well as the dependency lock and
SDK, preventing stale external-library links after a restore. Cache misses
remain valid cold builds; GitHub's branch/tag cache scope can limit reuse.

For an explicit pre-release package check on `main`, first wait for normal CI,
then run `gh workflow run release.yml --ref main`. To validate another branch,
run `gh workflow run ci.yml --ref BRANCH`, wait for that exact commit's manual CI
to pass, then run `gh workflow run release.yml --ref BRANCH`, replacing `BRANCH`
with the reviewed branch name. Pushes or new commits
require matching fresh CI. A dry run on `main` also seeds default-branch release
caches that later tags can restore. Neither branch dispatch publishes a release.

No release or draft is uploaded when a required build/package job fails. Drafts
remain unpublished until reviewed; `release.prerelease` is explicitly false.
The workflow uses native [GitHub-hosted runners](https://docs.github.com/en/actions/how-tos/write-workflows/choose-where-workflows-run/choose-the-runner-for-a-job)
for each architecture. Older macOS versions and untested external backends are
not certified by these jobs.

`make build` stamps `VERSION`; GoReleaser snapshot builds can set
`TXM_SNAPSHOT_VERSION="$(cat VERSION)"`. Direct Go builds default to `dev`.
Snapshot output is test material, not a replacement for the exact tagged release
artifacts.
Use the pinned tools and runner configuration in the
[release workflow](../.github/workflows/release.yml). The build/dependency pins,
archive inventory, and job definitions live there and in
[GoReleaser configuration](../.goreleaser.yaml).

For each release, keep validation reports with the CI run and record relevant
compatibility evidence in that version's [changelog entry](CHANGELOG.md). A prior
snapshot's passing checks do not validate changed source or final release bytes.
