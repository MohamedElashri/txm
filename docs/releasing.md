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
| Linux amd64 | Fresh 2.0.0-stamped GNU/static-musl builds pass normal/race runtime suites, private installation/completions, native/tmux, supervision, profiles, dashboard, tasks/pruning, real SSH, and foreign-UID rejection. Exact-tag CI is still required. |
| macOS ARM64 | Earlier local development builds covered native/tmux 3.8, profiles, supervision/events, dashboard, tasks/pruning, private archive installation, and real loopback SSH; normal and race binaries. Exact-tag CI is still required. |
| macOS Intel | Build/CI configuration; runtime and archive/private-install gates remain open. |
| Linux ARM64 | Build target; native hardware/runtime/archive/private-install gates remain open. |
| Windows amd64/ARM64 | ZIP cross-builds; external-backend/runtime/install gates remain open. Native persistence, private Unix state, and dashboard are unsupported. |

Basic Zellij 0.45.1 and GNU Screen lifecycle/attach have Linux/macOS runtime
coverage; macOS Screen is 4.00.03. Their adapters remain degraded: exact panes,
layouts, output capture, and read-only attachment are unsupported. See the
[user guide's capability table](user-guide.md#choose-a-backend) or inspect the
installed backend with `txm backend capabilities --all-backends --json`.

Configured CI runs are not a substitute for recorded results. Published archive
names describe build targets; runtime support requires matching evidence.
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

Pull requests into `main` run normal CI and the complete package matrix below.
The package workflow uses `--snapshot` with the version from `VERSION`, checks
all eight archives, and never creates a draft or release for a PR. A version tag
rebuilds and revalidates the exact release artifacts before draft creation.

1. Update `VERSION` and its dated changelog section, including upgrade notes.
   `make release-check` requires a stable version and matching metadata.
2. Run the applicable [tests](testing.md), including actual archive installation
   and runtime checks. Save source/artifact hashes and reports outside the tree.
3. Commit the reviewed release preparation and merge it into the intended release
   branch. Tag that exact commit `v2.0.0`; the tag must match `VERSION`.
4. The tag workflow runs shared CI, then GoReleaser 2.19.0 on macOS 15 with its
   installed SDK. It builds all eight archives with `--skip=publish`; no GitHub
   release exists at this stage.
5. `scripts/check-release-artifacts.py` checks the complete archive inventory,
   checksums, packaged documentation, binary OS/CPU headers, and static-musl
   linkage. The same bundle is checked after each artifact download.
6. Each Unix archive runs on its matching Linux/macOS architecture through private
   installation and the runtime suites. Mac jobs additionally test real backends
   and loopback SSH. Both platforms test actual different-UID rejection using
   private fixtures: Linux in a disposable container, macOS as root only on the
   disposable CI runner. No accounts or host services are changed.
7. Windows x86_64/ARM64 runners check their executable's version, help,
   completions, and explicit unsupported refusals. These are CLI smoke checks,
   not external-backend or Windows installer certification.
8. Only after every package job passes does a write-enabled job create a
   **stable draft**, with eight ZIPs, checksums, and the canonical release notes.
   Review the tag/commit, jobs, reports, upgrade notes, and compatibility scope
   before manually publishing. Existing releases are not overwritten on rerun.

The bundle and package reports are retained as Actions artifacts for 14 days.
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

## Remaining release gates

- [x] Stable 2.0.0 metadata, dated upgrade notes, and prerelease refusal check.
- [x] Complete-bundle validation and build-before-upload release workflow.
- [x] Fresh Linux amd64 GNU/static-musl runtime/install/container checks: 40
  runtime/install/performance checks and 36 container groups across normal/race
  and actual archived binaries; 25 harness tests, 13 Go/build checks, and four
  fuzz targets also pass. Reports remain outside the source tree.
- [ ] Execute the tag workflow on the reviewed final commit, including all six
  Unix package jobs, both Windows CLI jobs, and Linux/macOS foreign-UID checks.
- [ ] Review the resulting stable draft and publish v2.0.0.

Local macOS ARM64 coverage includes OpenSSH 10.5p1. Previous Linux snapshots and
macOS results remain useful development evidence; they do not replace checks of
changed final release bytes. ARM64 Linux/macOS Intel gates are configured, not
claimed to have run. Windows external backends/installers and full trusted
provider conversations remain outside the verified release scope. Providers keep
explicit version and trust guards; generic supervision does not require them.
