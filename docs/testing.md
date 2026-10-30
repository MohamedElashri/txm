# Testing

Use Go 1.26+, Zig 0.16.0, a C compiler, pkg-config and Git. Native VT requires CGO;
macOS Command Line Tools suffice. `scripts/dependencies.env` pins the matching
Ghostty/Zig pair. Dirty/mismatched dependency source is rejected without reset.
Python 3 and tmux are required for the integration suites.

```sh
make build
make test test-race lint
make test-harness
make test-integration test-integration-race
make test-fuzz test-performance
make release-check
```

Override `GHOSTTY_SOURCE_DIR`, `GHOSTTY_HOST_DIR` and `ZIG` for private dependency
checkouts/prefixes. Direct Go tests need `CGO_ENABLED=1` and the verified prefix's
`share/pkgconfig` in `PKG_CONFIG_PATH`. Inspect custom paths before `make clean`.

## Isolation

`tests/sandbox.py` constructs private HOME/XDG/runtime/temp paths, environment and
backend wrappers. tmux uses an owned foreground server and explicit socket/config.
Native fixtures use owned process handles and framed IPC. Cleanup validates
ownership, namespace entries and process identity; it never discovers real
sessions, worktrees, SSH config or provider credentials.

PTY tests consume output and check terminal/file-flag restoration. Darwin
comparisons exclude only kernel bookkeeping bits. Same UID is a trust boundary;
these fixtures are not a sandbox against malicious code or escaping descendants.

The feature suites cover native lifecycle/protocol, exact CLI/backend behavior,
profiles, supervision, signals/events, dashboard, tasks/pruning and SSH. Unit tests
cover build/isolation guards, release identity, decoding, ownership and recovery.
Keep regression tests for shipped behavior; generated reports, demonstrations and
one-time development probes do not belong in source control.

## Artifact matrix

The runner checks existing binaries/archives, verifies race instrumentation,
refuses reused output directories and binds results to source/artifact hashes.
It runs at most two flavors concurrently, then measures performance alone.

```sh
python3 scripts/validate-runtime.py \
  --binary /private/tmp/check/txm --race-binary /private/tmp/check/txm-race \
  --archive /private/tmp/check/txm_Darwin_arm64.zip \
  --expected-version 2.0.0-rc.1 --macos-backends --loopback-ssh \
  --output /private/tmp/runtime-new
```

The macOS flags require installed Screen/Zellij and an unprivileged loopback sshd.
SSH uses fresh keys in private TMPDIR, strict known hosts and a forced fixture
endpoint with private HOME/ZDOTDIR. It does not modify Remote Login or accounts.
`make test-ssh` runs that suite alone.

`python3 scripts/validate-linux.py --help` describes the separate Linux amd64
container matrix. Root/foreign-UID probes run only in disposable containers with
no external runtime network and read-only source/artifact mounts; never as host
root. `tests/archive_checks.py` checks ZIP inventory, expected version and private
install/completions. Cross-builds do not establish runtime support.

Keep generated logs, archives and validation manifests outside the repository or
as CI/release artifacts. `scripts/audit-artifacts.py` scans explicit paths for
bounded known credential forms without echoing matches. Review fixture provenance
too. Release steps and remaining platform gates are in [Releasing](releasing.md).
