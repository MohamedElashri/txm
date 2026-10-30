# Optional tasks and SSH workflows

These commands are opt-in and independent of the picker, dashboard and normal
session commands. Tasks need Git and the native CGO runtime or tmux 3.4+. Remote
commands need OpenSSH locally and compatible txm remotely. Current evidence is
Linux amd64 development; see [readiness](extensions-readiness.md).

## Git worktree tasks

Start with read-only preflight, then run a foreground executable:

~~~sh
txm task start build --repo /path/to/repo --dry-run -- go test ./...
txm --backend native task start build --repo /path/to/repo -- go test ./...
txm --backend tmux task start review --repo /path/to/repo --branch txm/review -- /bin/sh
txm task list --json
txm task inspect FULL_TASK_ID --json
txm agent wait FULL_TASK_ID --until exited --timeout 30s --json
txm task stop FULL_TASK_ID --json
txm task cleanup FULL_TASK_ID --dry-run --json
txm task cleanup FULL_TASK_ID --json
~~~

Task names share the backend session namespace. Tasks default strictly to native,
independent of configured backend preferences.
Explicit --backend selects native or tmux without fallback. Names use 1..64 ASCII
letters/digits/dash/underscore, beginning with a letter or digit; actions require
full opaque task IDs. The task ID is also the managed agent ID.

Tasks always create a fresh branch (default txm/NAME-ID) from a committed
revision. Detached HEAD and dirty source repositories are allowed: uncommitted
source changes are never copied. Existing branches are never reused/reset.
Worktrees live only under the private XDG state's txm/v1/task-worktrees/ID.
No arbitrary destination or automatic repository setup script is supported.
The command after -- is your explicit foreground executable and literal argv.
Use --env NAME for additional allowed variable names. Values and argv are omitted
from task metadata and dry-run output; existing supervised manifest privacy and
retention rules still apply.

Git routing/config environment overrides are stripped. Default Git operations
disable hooks/fsmonitor and refuse configured external clean/smudge/process
filters, including conditional configuration that applies only in the new
worktree. Review repository commands before explicitly opting into Git hooks and
filters with --allow-repo-commands. No generic setup script is inferred.

Cleanup requires a supervisor exit receipt for a launched task. A never-launched
orphan is safe only when the task journal says launch was not requested. A missing
registration is unknown; recovery then requires the separately pinned supervisor
instance and its validated exit receipt. A lost reply without an instance pin
requires manual inspection if the registry is also lost.
Running/unknown processes, changed repository/worktree/.git identities, symlink
paths or a changed worktree branch cause refusal. Stop verifies the exact native
instance or tmux ownership marker, including after a session rename.

Modified, untracked **and ignored** files prevent default cleanup. To discard
those files deliberately, provide both flags with the same full task ID:

~~~sh
txm task cleanup FULL_TASK_ID --force --confirm FULL_TASK_ID --dry-run
txm task cleanup FULL_TASK_ID --force --confirm FULL_TASK_ID
~~~

Force cannot bypass process or ownership checks. Git branches and committed work
are retained. There is no automatic reset/clean/prune or restart launch replay.
Failed starts retain inspectable records/worktrees for explicit cleanup. A crash
before ownership was saved, a moved/missing path, corrupt/copied state, or
repository replacement requires manual inspection; txm refuses to guess or repair
ownership. Keep the original state directory. Completed records remain (128-task
limit plus the existing 256-agent limit); automatic metadata pruning is absent.

To reclaim index capacity explicitly, clean the task first, then prune its task
record before its agent registration:

~~~sh
txm task prune FULL_TASK_ID --dry-run
txm task prune FULL_TASK_ID --confirm FULL_TASK_ID
txm agent prune FULL_AGENT_ID --dry-run
txm agent prune FULL_AGENT_ID --confirm FULL_AGENT_ID
~~~

Pruning refuses running/unknown supervised agents or a present worktree path.
It retains branches, commits, exit receipts and private launch/hook files;
it is not disk garbage collection. Unsupervised registration pruning leaves its
terminal session alive. Agent pruning stores up to 16,384 host-bound retired IDs
to prevent explicit-ID launch replay; a full or corrupt history refuses further
pruning. Do not delete state files to bypass these checks.

## SSH host allowlist and discovery

Configure authentication, known hosts and optional ControlMaster in your ordinary
OpenSSH configuration first. txm reuses these settings with batch authentication
and strict host-key verification. It does not accept new keys automatically,
install credentials or start a network service. Aliases must be simple SSH
configuration names; user@host, URI, port syntax, whitespace and options are rejected.

~~~sh
txm remote allow devbox
txm remote hosts --json
txm remote probe devbox --json
# Verify the returned identity against the intended host before pinning it:
txm remote pin devbox VERIFIED_FULL_HOST_ID
txm --backend native remote list devbox --timeout 10s --json
txm --backend tmux remote list devbox another_allowed_alias --json
txm remote forget devbox
~~~

For txm outside PATH, allow --executable /absolute/safe/path/txm. The path must
be a single POSIX token containing letters/digits/underscore/dot/slash/dash.
An unpinned alias permits only probe. Duplicate host identities across aliases
are refused; repinning is explicit. At most 32 entries are kept in the private
remotes/hosts.json file. SSH config wildcard entries are never enumerated.

List defaults to native and uses only the allowlist when aliases are omitted.
Each host has a 1..30 second deadline (default 10); at most four SSH requests run
concurrently, within the CLI overall 60-second budget. Successful hosts remain in partial JSON output, with alias-scoped
errors for failures. Partial output exits 7, timeout-only failures 6, identity
conflicts 5, incompatible versions 4, and cancellation 130. A failed host is never
a successful empty list. Captured queries do not echo raw remote diagnostics.
Schema/protocol 1 is required on both ends.

## Exact remote attachment

Copy fields from an observed remote list row, including its pinned host ID and
backend ID. Native also needs the observed launch instance:

~~~sh
txm --backend native remote attach devbox --host-id FULL_HOST_ID --name EXACT_NAME --session-id BACKEND_ID --instance NATIVE_INSTANCE --read-only
txm --backend tmux remote attach devbox --host-id FULL_HOST_ID --name EXACT_NAME --session-id '$7'
~~~

Attach requires a local TTY and explicit --backend, --host-id, --name and
--session-id. Native also requires --instance matching its backend ID.
Host identity is checked remotely before session access. The endpoint revalidates
name/ID/native instance without creating sessions or selecting another backend.
Unmanaged tmux IDs remain server scoped, as in M1 operations.

Encoded JSON requests prevent shell metacharacters in names from becoming
command syntax. Attachment uses ssh -tt and restores local TTY settings and
descriptor flags on exit/cancellation. --read-only uses the backend observer
contract. Interactive terminal data is not a JSON response; --json is refused.

Tests use fake SSH and a second private txm environment. Actual SSH server/network
smoke was not run because this environment has no sshd. Fixture coverage does
not establish verified deployment or cross-platform support.
