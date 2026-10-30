# Linux follow-up and review handoff

This follow-up implements the remaining work that can be validated on this
Linux amd64 machine without user accounts, credentials, trusted provider runs,
real remote hosts, or additional platform hardware. It follows the original
Phases 0–8 goals. The historical phase evidence is preserved.

## Changes

- Actual UID 65534 socket clients test both ordinary filesystem denial and
  kernel peer-credential rejection for native IPC and the coordinator. Only
  private fixture permissions are temporarily relaxed to reach the peer check;
  owner access, running children and registry contents must survive.
- Real OpenSSH client/server tests cover successful authentication, host-key and
  client-key refusal, host pins, partial results, protocol mismatch, timeout,
  cancellation, ControlMaster reuse/reconnection and real native/tmux PTYs.
  A remote operation's exit 1 now retains that classification instead of being
  reported as SSH unavailability. SSH exit 255 remains transport failure.
- Real GNU Screen 4.09.01 and Zellij no-web 0.45.1 tests cover basic session
  lifecycle, exact-name collisions, typed IDs, namespace-scoped nuke with a
  second protected namespace, and attach/input/detach/TTY restoration. The
  adapters remain degraded; pane/layout/read-only capabilities are not expanded.
- Explicit `task prune` and `agent prune` reclaim bounded index capacity. Dry-run
  performs checks; actual removal requires the full matching ID. Cleaned task
  records require absent worktrees and exit proof; supervised agents require a
  validated exit receipt. Branches, commits, receipts and launch/hook files
  remain. Unsupervised pruning leaves its terminal alive. Host-bound retired IDs
  prevent explicit-ID launch replay; corrupt history fails closed. Event
  subscribers resync after registry removal. See [contracts](contracts.md).

## Container isolation and reproduction

`scripts/validate-linux.py` builds the test-only
[`Dockerfile.linux`](../tests/containers/Dockerfile.linux) from a pinned Ubuntu
base digest and records the resulting image ID and installed tool versions.
Apt package resolution is not a package lock; future repository updates may
produce a different image, which must be recorded and revalidated.

Each runtime container has `--network none`, a read-only root, private `/tmp`,
`/run` and `/root` tmpfs mounts, and an init process to reap children. Only a copy
of the Python fixtures and explicitly supplied binaries are mounted read-only;
reports are the sole writable host mount. No host HOME, SSH agent, credentials,
Docker socket, user backend socket or project Git/config directory is mounted.
The disposable container root account accepts only the test server's fresh
public key over loopback, with password login disabled. No host service/account
is changed. Owned subprocesses, private socket credentials and live pidfds guard
cleanup; Docker cleanup checks a random name and ownership label.

Acquire Zellij's official `v0.45.1/zellij-no-web-x86_64-unknown-linux-musl.tar.gz`
release asset separately and verify the release asset SHA-256:
`d7bda1e18c30a688833ae7627f1d6a253bbba5349a4bc48e4f0ec008aaf75ed1`.
The runner verifies the extracted executable SHA-256:
`0ec6ef07b63c6355c02ce18343d40ef5ef5af19e25313ea9009c8fceda29e94f`.
It does not download or install a backend on the host.

```sh
python3 scripts/validate-linux.py \
  --binary /absolute/txm --race-binary /absolute/txm-race \
  --archive /absolute/txm_Linux_x86_64.zip \
  --archive /absolute/txm_Linux_x86_64-musl.zip \
  --zellij /absolute/verified-zellij \
  --claude /absolute/claude-2.1.308 \
  --output /tmp/new-txm-linux-report
```

The Claude argument is optional and runs only the existing offline `--init-only`
SessionStart test with private configuration, no model/auth and container network
access disabled. The full Linux gate requires all nine groups without skips for
each binary, checks race instrumentation and binds source/artifact fingerprints.
Run the ordinary full runtime/ZIP/performance matrix separately as described in
[testing](testing.md#phase-8-reproducible-final-validation). The direct root-only
suite refuses host execution; use the container runner.

## Provider and platform limits

Installed versions were rechecked with private HOME/configuration: Claude
2.1.308, Codex 0.166.0 and OpenCode v2.2.2. The latter two remain outside the
verified adapter allowlist; no version pin was relaxed. Claude initialization
proves SessionStart only, not permission handling, full turns or conversations.
Codex hooks require review of the hook configuration; automatic approval or
trust bypass is outside this task. See [official Codex hook trust documentation](https://learn.chatgpt.com/docs/hooks).
No provider authentication or model calls were used.

macOS runtime/archive/install, native ARM64 hardware, and Windows external
backend/archive/install validation still require appropriate isolated runners.
Cross-compilation and containers on an amd64 kernel do not close those gates.
Trusted/full provider lifecycle validation requires an explicitly reviewed
provider setup. No whole-project release is certified.

## Review boundaries

The tree contains the previous phases and this follow-up as uncommitted work.
No files were staged and no commit, PR, push, tag or release was created.
Review the phases in the PR boundaries in `plan.md`; the current follow-up can
be reviewed in these groups:

1. Remote exit classification and its real SSH regression cases.
2. Metadata pruning, replay prevention, event resync and refusal/preservation
   tests; public help, man page and contracts.
3. Linux container runner, foreign-UID/SSH/backend/provider evidence, updated
   support matrix and completion notes.

These are proposed review boundaries, not existing commit references. Generated
binaries, dependency checkouts and private test keys remain outside the source
change. Existing ignored `bin`, `dist` and Ghostty checkout are preserved.


## Completed validation

[The follow-up manifest](evidence/linux-followup-validation.json) binds exact
commands, source/harness/build hashes and artifact hashes to all 44 retained
runtime/install/performance checks, 36 real Linux container groups, offline
Claude initialization, version guards, 13 Go/build checks and four fuzz targets.
All pass. The privacy scan covers known credential patterns and synthetic fixture
provenance; it is not universal secret detection. The cleanup report verifies
owned container/process teardown, unchanged HEAD, nothing staged and the original
ignored Ghostty revision. Historical Phase 0–8 evidence is unchanged.
