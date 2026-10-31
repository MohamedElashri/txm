# Compatibility and releases

[Documentation](README.md) / Compatibility and releases

`VERSION` identifies the build in this checkout: **2.0.0**.
The [changelog](CHANGELOG.md) supplies versioned release notes and migration
instructions. [Getting started](getting-started.md) covers installation.

## Platforms and backends

Linux and macOS are the primary runtime platforms. The native backend includes
its terminal library; tmux, Zellij, and Screen adapters require the corresponding
executable. Capabilities describe individual operations, not backend parity.

| Platform | Runtime coverage and current scope |
| --- | --- |
| Linux amd64 | GNU/static-musl archives pass private installation, native/tmux runtime suites, profiles, supervision/events, dashboard, tasks/pruning, and actual foreign-UID rejection. Normal/race/fuzz CI and additional local SSH/container checks also pass. |
| macOS ARM64 | Archive installation and native/tmux runtime suites pass, including profiles, supervision/events, dashboard, tasks/pruning, real Screen/Zellij, loopback SSH, and actual foreign-UID rejection. Normal and race CI also pass. |
| macOS Intel | The same archive installation, runtime, backend, SSH, and foreign-UID gates as macOS ARM64 pass on an Intel runner; normal and race CI also pass. |
| Linux ARM64 | GNU/static-musl archives pass private installation, native/tmux runtime suites, profiles, supervision/events, dashboard, tasks/pruning, and actual foreign-UID rejection on ARM64 runners. |
| Windows amd64/ARM64 | Both archives pass checksum/header validation and CLI version/help/completion/unsupported-refusal smoke checks on matching runners. External backends and Windows installers are unverified; native persistence, private Unix state, and dashboard are unsupported. |

Basic Zellij 0.45.1 and GNU Screen lifecycle/attach have Linux/macOS runtime
coverage; macOS Screen is 4.00.03. Their adapters remain degraded: exact panes,
layouts, output capture, and read-only attachment are unsupported. See the
[user guide's capability table](user-guide.md#choose-a-backend) or inspect the
installed backend with `txm backend capabilities --all-backends --json`.

The matrix records the successful [PR CI](https://github.com/MohamedElashri/txm/actions/runs/41153986716)
and [eight-target package validation](https://github.com/MohamedElashri/txm/actions/runs/41153986791)
at commit `0bb3ef3`, plus the local coverage described below. These PR archives
are snapshots stamped 2.0.0. Official release assets are rebuilt and must pass
the exact-tag workflow before publication. Archive names alone do not establish
runtime support.
The release workflow builds unsigned, unnotarized archives. Checksums identify
bytes, not publisher identity.

## Provider support

Any supported executable can be supervised as a process without a provider
adapter. Attention and turn signals require an explicit integration.

| Adapter | Current validation |
| --- | --- |
| Generic | Authenticated worker signals; no AI provider required |
| Claude | Exact 2.1.308 adapter with offline SessionStart coverage; full conversations remain unverified |
| Codex | 0.165.0 schema and manual configuration only; runtime/trusted lifecycle remains unverified. Installed 0.166.0 is refused. |
| OpenCode | Installed v2.2.2 is unsupported; no v1 examples are applied to v2 |

Use `txm agent providers --json` to check the installed versions. Provider
configuration requires review and applicable trust; txm does not bypass provider
permissions or activate trust automatically. [Automation](automation.md#coding-agents-and-attention)
explains the distinction between running a process and interpreting its hooks.

## Upgrade review

Finish legacy native sessions using the old binary and original user, TMPDIR,
and configuration. The framed native protocol leaves legacy sockets and
processes untouched; it does not migrate running sessions.

Before updating automation, review strict backend selection, nonzero failure
exits, partial JSON results, and profile preflight. Native remains one terminal;
Screen/Zellij have the capability limits above. Keep uncertain resources for
inspection. Coordinator and provider workflows remain opt-in.

Stable builds support `txm update`, which downloads only a newer stable version
and refuses downgrade or malformed/prerelease metadata. Development and prerelease builds refuse
automatic stable updates; install a selected release explicitly. The quick
installer and GitHub's latest-release endpoint select stable releases.

## Release process

Pull requests into `main` run normal CI. Full archive builds and the eight-target
package matrix run only for version tags or an explicit Release workflow dispatch.
Branch dispatches use snapshots stamped from `VERSION` and never create a draft.
Tag runs build and validate the official archives before draft creation.

1. Update `VERSION` and its dated changelog section, including upgrade notes.
   `make release-check` requires a stable version and matching metadata.
2. Run the applicable [tests](testing.md), including actual archive installation
   and runtime checks. Save source/artifact hashes and reports outside the tree.
3. Commit the reviewed release preparation and merge it into `main`. Wait for
   that exact merge commit's CI to pass, then tag it with the version from
   `VERSION` (for example `v2.0.0`).
4. A short Linux preflight checks stable metadata and the latest CI run for that
   exact commit on `main`, including successful Linux, macOS ARM64/Intel, and
   lint jobs. Missing, pending, failed, cancelled, or skipped validation stops
   the release before compilation. Existing CI is reused rather than repeated.
5. Two parallel GoReleaser 2.19.0 jobs build without publishing: Linux builds the
   four Linux and two Windows archives; macOS 15 builds the two Darwin archives
   using its SDK. Each job builds only its required Ghostty libraries and retains
   its partition. An inexpensive Linux job assembles the complete bundle using
   the original build checksums.
6. `scripts/check-release-artifacts.py` checks the complete archive inventory,
   checksums, packaged documentation, binary OS/CPU headers, and static-musl
   linkage. The same bundle is checked after each artifact download.
7. Each Unix archive runs on its matching Linux/macOS architecture through private
   installation and the runtime suites. Mac jobs additionally test real backends
   and loopback SSH. Both platforms test actual different-UID rejection using
   private fixtures: Linux in a disposable container, macOS as root only on the
   disposable CI runner. The Linux peer probe uses a pinned multi-architecture
   Python image directly, avoiding an unrelated SSH/tmux/Screen/Git image build.
   No accounts or host services are changed.
8. Windows x86_64/ARM64 runners check their executable's version, help,
   completions, and explicit unsupported refusals. These are CLI smoke checks,
   not external-backend or Windows installer certification.
9. Only after every package job passes does a write-enabled job create a
   **stable draft**, with eight ZIPs, checksums, and the canonical release notes.
   Review the tag/commit, jobs, reports, upgrade notes, and compatibility scope
   before manually publishing. Existing releases are not overwritten on rerun.

The bundle and package reports are retained as Actions artifacts for 14 days.
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
then run `gh workflow run release.yml --ref main`. For a development branch,
run `gh workflow run ci.yml --ref dev`, wait for that exact commit's manual CI
to pass, then run `gh workflow run release.yml --ref dev`. Pushes or new commits
require matching fresh CI. A dry run on `main` also seeds default-branch release
caches that later tags can restore. Neither branch dispatch publishes a release.

No release or draft is uploaded when a required build/package job fails. Drafts
remain unpublished until reviewed; `release.prerelease` is explicitly false.
The workflow uses native [GitHub-hosted runners](https://docs.github.com/en/actions/how-tos/write-workflows/choose-where-workflows-run/choose-the-runner-for-a-job)
for each architecture. Older macOS versions and untested external backends are
not certified by these jobs.

`make build` stamps `VERSION`; local snapshot builds can set
`TXM_SNAPSHOT_VERSION=2.0.0`. Direct Go builds default to `dev`. Snapshot output
is test material, not a replacement for the exact tagged release artifacts.
Builds are unsigned and unnotarized; checksums establish integrity, not publisher
identity.

## Validation record and publication gates

- [x] Stable 2.0.0 metadata, dated upgrade notes, and prerelease refusal check.
- [x] Complete-bundle validation and build-before-upload release workflow.
- [x] PR CI on Linux and macOS ARM64/Intel, lint, and CodeQL.
- [x] All eight PR package jobs: six Unix runtime/private-install jobs and two
  Windows CLI smoke jobs, including Linux/macOS actual foreign-UID rejection.
- [x] Fresh Linux amd64 GNU/static-musl runtime/install/container checks: 40
  runtime/install/performance checks and 36 container groups across normal/race
  and actual archived binaries; 25 harness tests, 13 Go/build checks, and four
  fuzz targets also pass. Reports remain outside the source tree.

For publication, the tag workflow must repeat CI and all eight package jobs on
the reviewed release commit. Review its resulting stable draft, assets, checksums,
reports, and canonical upgrade notes before publishing v2.0.0. The PR results
above do not replace validation of those final release bytes.

Local macOS ARM64 coverage includes OpenSSH 10.5p1. Previous Linux snapshots and
macOS results remain useful development evidence; they do not replace checks of
changed final release bytes. Windows external backends/installers and full trusted
provider conversations remain outside the verified release scope. Providers keep
explicit version and trust guards; generic supervision does not require them.
