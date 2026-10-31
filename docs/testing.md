# Building and testing

[Documentation](README.md) / Building and testing

Start with [Contributing](../CONTRIBUTING.md) for pull request expectations.
[Installation](getting-started.md#build-from-source) lists the build prerequisites.
This guide covers dependency configuration and validation without touching active
sessions or personal configuration.

## Dependency configuration

`scripts/dependencies.env` pins Zig 0.16.0 and the Ghostty revision compatible
with the Go binding. `make build` builds the native library before compiling txm
and stamps the version from `VERSION`. A direct Go build defaults to `dev`.

The helper accepts these overrides:

| Variable | Purpose |
| --- | --- |
| `GHOSTTY_SOURCE_DIR` | Dedicated source checkout at the pinned revision |
| `GHOSTTY_HOST_DIR` | Native library installation prefix |
| `ZIG` | Path to the pinned Zig executable |

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

Keep regression tests for shipped behavior. Save logs, recordings, manifests, and
archive outputs outside the repository or as CI/release artifacts. Include exact
commands, source/artifact hashes, OS, architecture, backend versions, and skip
reasons. A configured CI job or cross-build is not evidence of an executed runtime
gate.

## Validate packaged builds

The runtime runner takes existing binaries and archives. It verifies race
instrumentation, refuses an existing output directory, and binds results to the
source and artifact hashes. It tests at most two flavors concurrently and
measures performance after the workers finish.

```sh
python3 scripts/validate-runtime.py \
  --binary /absolute/check/txm --race-binary /absolute/check/txm-race \
  --archive /absolute/check/txm_Darwin_arm64.zip \
  --expected-version 2.0.0 --macos-backends --loopback-ssh \
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

Use `scripts/audit-artifacts.py` on explicit outputs for known credential forms;
it reports redacted findings. Pair the scan with a review of fixture provenance.
[Compatibility and releases](releasing.md) tracks the platform matrix and gates.


Pull requests run the full package matrix using non-publishing snapshot builds.
Both macOS package jobs install checksum-pinned Zellij 0.45.1 with
`scripts/install-ci-zellij.sh` to avoid differing Homebrew package snapshots.
The tag workflow rebuilds the exact release artifacts. Both verify the complete eight-ZIP
bundle with `scripts/check-release-artifacts.py`, and tests archives on matching
runners before the tag workflow can create a stable draft. PR runs never create releases. `tests/peer_checks.py` shares the actual
foreign-UID regressions between Linux containers and disposable macOS CI; it
refuses ordinary host-root execution. Windows checks intentionally cover only
version/help/completions and unsupported refusals. See the
[release process](releasing.md#release-process) for the complete gate sequence.
