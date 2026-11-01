# Development and releases

[Documentation](README.md) / Development and releases

See [Contributing](../CONTRIBUTING.md) for PR expectations and
[build from source](user-guide.md#build-from-source) for prerequisites.

## Dependency configuration

`make build` builds the pinned Ghostty library, then txm, stamping
[VERSION](../VERSION). Direct Go builds default to `dev`. Pins live in
[scripts/dependencies.env](../scripts/dependencies.env) and [go.mod](../go.mod).

| Override | Purpose |
| --- | --- |
| `GHOSTTY_SOURCE_DIR` | Dedicated clean checkout at the pinned revision |
| `GHOSTTY_HOST_DIR` | Native library installation prefix |
| `ZIG` | Pinned Zig executable |
| `TXM_RELEASE_PLATFORM` | `linux` or `darwin`; unset builds all release targets |

The helper refuses dirty or mismatched dependencies without resetting them.
Inspect custom paths before `make clean`, which removes the configured host
prefix and build outputs. Direct Go tests need CGO and pkg-config metadata:

```sh
PKG_CONFIG_PATH="$PWD/.ghostty-host/share/pkgconfig" CGO_ENABLED=1 go test ./...
```

Use your actual prefix if overridden. Integration tests need Python 3 and tmux;
`make lint` also needs golangci-lint.

## Checks by change type

| Change | Checks |
| --- | --- |
| Go behavior | `make test test-race lint` |
| CLI, backend or lifecycle | `make test-integration test-integration-race` |
| Test/build harness | `make test-harness` |
| Protocol or decoder | `make test-fuzz` and relevant regression tests |
| Performance | `make test-performance` |
| Release metadata or packaging | `make release-check` and native archive checks |
| Documentation | Local links, examples/help and `python3 scripts/check-release.py` |

Focus a regression with its unittest selector:

```sh
python3 tests/native_checks.py --binary "$PWD/bin/txm" \
  NativeChecks.test_concurrent_attach_resize_and_output
```

Update `docs/txm.1` when command semantics change; it is embedded in the binary.
To test an existing race binary without rebuilding, use
`make test-runtime TEST_BINARY=/absolute/txm-race`.

**Verify & Build** runs Scope → Lint → Tests → OS builds → Validation → CI ready.

| Profile | Work |
| --- | --- |
| Docs-only push/PR | Scope and release metadata; expensive stages skip |
| Code PR | Harness, race tests, short fuzzing, native Linux/macOS builds and runtime checks, archive configuration |
| Main or full manual run | Full fuzzing, seven release archives and native package checks in addition to source checks |

Only Markdown and `docs/txm.1` qualify for docs-only scope; code under `docs/`
does not. Manual CI defaults to full readiness; `release_readiness=false`
selects the lighter profile. Linux/macOS runtime suites run once per OS, and
macOS backend/SSH checks run once. Pinned dependency caches are revalidated;
Go objects are scoped to native library bytes, SDK and source.

## Fixture isolation

Use `tests/sandbox.py` for private HOME, XDG, runtime and temporary directories.
Backends use explicit owned sockets/configuration; cleanup verifies ownership
and process identity. Never target personal sessions or run foreign-UID probes
as root on an ordinary host. Fixtures do not sandbox malicious same-UID code
or escaping descendants.

Keep reports, recordings and archives outside source control. Record commands,
source/artifact hashes, platform/backend versions and skips. A cross-build or
configured CI job does not prove native execution. Use
`scripts/audit-artifacts.py` on explicit outputs for known credential patterns,
then review fixture provenance.

Live-provider runs require separate account, cost and trust authorization.
Ordinary fixtures do not use personal configuration or inference. SDK/loader
checks do not establish a complete provider lifecycle.

- `make test-opencode-runtime` needs `OPENCODE_BINARY` and a new
  `OPENCODE_REPORT`; it checks the exact SDK/loader privately without inference.
- `python3 tests/record_demo.py --binary /absolute/txm --destination /absolute/new-demo`
  records about a minute of generic CLI/PTY activity without a provider account.

## Validate packaged builds

The runner consumes existing binaries/archives and creates a new report directory:

```sh
python3 scripts/validate-runtime.py \
  --binary /absolute/check/txm --race-binary /absolute/check/txm-race \
  --archive /absolute/check/txm_Darwin_universal.zip \
  --expected-version "$(cat VERSION)" --macos-backends --loopback-ssh \
  --output /absolute/new-runtime-report
```

On macOS, backend checks need Screen and Zellij 0.45.1+; loopback SSH uses fresh
keys, a private home and an unprivileged sshd. It does not enable Remote Login.
Use `make test-ssh` for SSH alone. On Linux, see
`python3 scripts/validate-linux.py --help` for disposable-container checks.

`tests/archive_checks.py` checks private installation, version, man page,
completions and short runtime behavior. Match archives to native runners.
Windows checks cover CLI smoke only, not external backends or installers.

The 100-session performance fixture needs 4 GiB available memory. If refused,
rerun on a suitable idle host. Smaller runs do not replace that gate. Compare
with the same tools, dependencies and harness, without concurrent workers;
investigate repeatable regressions above 20%.

## Release process

**Ship txm** reuses the exact source commit's validated CI bundle. It never
rebuilds or repeats runtime/package tests. A branch dispatch is a rehearsal;
a version-tag push creates a draft.

1. Update `VERSION` and its dated changelog entry, including upgrade notes.
   Run `make release-check` and the checks relevant to the change.
2. Merge the reviewed preparation into `main`. Wait for full CI on that exact
   commit. If docs-only scope skipped it, run `gh workflow run ci.yml --ref main`
   and wait for full readiness.
3. Optionally rehearse with `gh workflow run release.yml --ref main`.
4. Create a signed tag matching `VERSION` at the verified commit, then push it.
   For example, from the verified checkout: `git tag -s "v$(cat VERSION)"`.
5. Review the resulting draft's commit, seven ZIPs, checksums and concise notes,
   then publish. Existing releases are never overwritten by rerunning the workflow.

Release preflight requires every named full CI stage, `CI ready`, and unexpired
source-bound bundle/evidence artifacts. Missing, skipped, failed or pending
checks stop the release. Changed commits need fresh full readiness.

CI builds four Linux archives, two Windows archives and one universal macOS
archive. All Mac jobs use `macos-latest` on Apple Silicon; both binary slices
are checked structurally, with native runtime checks on Apple Silicon.
Unix packages get installation, child/signal/receipt/metrics and foreign-UID
checks; Windows gets CLI smoke. Archive inventory, checksums, source identity,
binary headers and static-musl linkage are checked before draft creation.

The release workflow verifies downloaded bytes against CI's original evidence
and privacy scan. Reports and bounded hash proofs stay with Actions, not the
changelog. Re-run failed jobs with `gh run rerun RUN_ID --failed`; successful
build partitions and packages are reused. Expired artifacts require fresh CI.

For another branch, dispatch full `ci.yml --ref BRANCH`, wait for success, then
rehearse `release.yml --ref BRANCH`. Branch rehearsals never publish.
Build pins and archive definitions live in
[CI](../.github/workflows/ci.yml),
[build setup](../.github/actions/setup-build/action.yml) and
[GoReleaser](../.goreleaser.yaml); publishing lives in
[Ship txm](../.github/workflows/release.yml).
