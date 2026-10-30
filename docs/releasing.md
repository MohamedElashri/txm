# Releasing txm

The candidate is **2.0.0-rc.1**. `VERSION` is the build identity;
[CHANGELOG](CHANGELOG.md) supplies release notes and upgrade instructions.

## Prepare and publish

1. Update `VERSION` and its dated changelog section, including upgrade notes.
2. Run `make release-check` and applicable [tests](testing.md). The metadata checker
   verifies version/tag agreement and can write notes to a new `--notes` path.
3. Build and test archives from the final candidate commit; preserve checksums and
   source/artifact-bound reports as build artifacts, outside the source tree.
4. Review and commit the candidate. An approved matching `vVERSION` tag triggers
   shared Linux/macOS CI, then GoReleaser 2.19.0 on macOS 15 using its installed SDK.
5. Review the draft's tag/commit, green jobs, all eight archives, checksums, upgrade
   notes and applicable runtime gates before manually publishing.

The workflow creates drafts; RC tags are prereleases. It tests its packaged
host-compatible macOS ZIP after upload; a failed job may leave an incomplete draft.
Builds are unsigned/unnotarized. Checksums identify bytes, not publisher identity.

`make build` stamps `VERSION`; direct Go builds default to `dev`. Snapshot builds
can set `TXM_SNAPSHOT_VERSION`. Development/prerelease binaries refuse automatic
stable updates. The quick installer and `releases/latest` select stable releases;
choose an exact RC archive or build its source to test the candidate.

## Remaining release gates

- [ ] Cleaned Linux amd64 GNU/static-musl archive/runtime and container rerun.
- [ ] Linux ARM64 hardware/runtime/archive/private-install checks.
- [ ] macOS Intel/older OS runtime/archive/private-install checks. ARM64/Intel
  macOS 15 CI is configured; remote results are not yet recorded.
- [ ] Windows amd64/arm64 external-backend/archive/private-install checks. Local RC
  ZIP cross-builds pass; native persistence/private Unix state/dashboard are unsupported.
- [ ] Actual Darwin foreign-UID filesystem/LOCAL_PEERCRED rejection on an isolated
  runner, without host account changes.
- [ ] Full supported-provider lifecycle with reviewed trust/configuration/credentials.

Local macOS ARM64 coverage includes native/tmux 3.8, profiles, agents/events,
dashboard, tasks/pruning, OpenSSH 10.5p1, Screen 4.00.03 and Zellij 0.45.1, including
race binaries and private archive installation. Linux amd64 GNU/musl and real
foreign-UID/SSH/backend checks have earlier snapshot evidence; rerun the final RC.
An RC may document open gates. Stable support claims require matching evidence.

## Provider support

Generic supervision needs no AI provider. Claude 2.1.308 has isolated offline
SessionStart coverage, not full conversation coverage. Installed Codex 0.166.0 and
OpenCode v2.2.2 were refused by version guards. Codex configuration requires manual
trust review; unsupported versions and automatic trust activation remain refused.

## Upgrade review

Finish legacy native sessions with the old binary and original user/TMPDIR/config.
New txm leaves their sockets/processes untouched. Check automation for nonzero
failure exits, strict backend selection, partial JSON and profile preflight.
Native remains one terminal; Screen/Zellij retain degraded capabilities. Uncertain
resources require inspection. Agent/coordinator/provider workflows stay opt-in.
