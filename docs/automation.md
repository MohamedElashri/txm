# Automation and agent workflows

[Documentation](README.md) / Automation

txm can supervise a build or worker, expose its state to scripts, and run project
work in a dedicated Git worktree. Coding-agent hooks add attention signals to the
same model. SSH aggregation extends session discovery and attachment to hosts
you configure explicitly.

A session is a terminal you can reconnect to. A supervised job additionally has
a managed identity and a recorded process exit result. You can use either
without an AI provider.

## Supervised jobs

Run a command from your project directory:

```sh
txm --backend native agent start build -- go test ./...
```

The command starts in a new detached session. `agent start` returns an opaque
agent ID; use that full ID in subsequent calls. The launch uses structured argv
rather than a shell command string. Native and tmux support supervision.

```sh
txm agent list --json
txm agent status AGENT_ID --json
txm agent wait AGENT_ID --until exited --timeout 5m --fail-on-nonzero --json
```

Inspect `receipt.outcome` and `receipt.exit_code` for the child's result.
Successful `agent start` means the launch or a recorded exit was observed; it
does not mean the child succeeded. Likewise, a default `wait --until exited`
succeeds when exit is observed, even if the child failed. `--fail-on-nonzero`
makes an unsuccessful child receipt fail the wait command.

Use `--cwd /absolute/project` to select a working directory. Repeated `--env NAME`
options allow additional variable names from the launch environment; HOME, PATH,
and TERM are allowed by default. This is not a mechanism to forward values from
the invoking client into an already running backend wrapper.

The optional coordinator maintains registrations and observations. It starts for
agent workflows; ordinary sessions and the picker do not contact it. Stopping
it with `txm agent stop-coordinator` leaves sessions and children running.
Restart does not replay commands. Missing exit proof remains unknown.

`txm agent register tmux/existing-session` tracks an existing session without
supervising it; registration cannot provide a retroactive child exit result.

## JSON for scripts

Use `--json` for supported session, agent, task, and remote queries. Responses
have versioned `data`, `errors`, and `partial` fields. Aggregate results can
contain useful data while returning a nonzero failure exit, so check both the
status and envelope. [CLI contracts](contracts.md#json-and-exits) define them.

For example, this script uses `jq` to capture the returned ID and wait for a build:

```sh
#!/bin/sh
set -eu
launch="$(txm --backend native agent start --json build -- go test ./...)"
agent_id="$(printf '%s\n' "$launch" | jq -er '.data.agent.id')"
txm agent wait "$agent_id" --until exited --timeout 5m --fail-on-nonzero --json
```

## Coding agents and attention

Process lifecycle, attention, and turn completion answer different questions:

| Observation | Meaning |
| --- | --- |
| Lifecycle | Whether the supervised process is running, exited, or unproven |
| Attention | A fresh authenticated signal such as working, needs-input, or idle |
| Turn completion | An explicit hook signal for a completed turn, independent of process exit |

Terminal text is never parsed to infer those signals. To use a generic worker
that reports attention, opt in at launch:

```sh
txm --backend native agent start --provider generic worker -- /absolute/path/to/worker
```

Inside that worker, the inherited private hook context enables:

```sh
txm agent signal --from-env --state working
txm agent signal --from-env --state needs-input
txm agent signal --from-env --state turn-complete
```

For existing associations, `txm agent hooks enable AGENT_ID --provider generic`
creates or rotates credentials. Hook input is authenticated, generation-bound,
and checked for freshness and replay. Attention expires when it is no longer
fresh; it does not remain a permanent success label.

`txm agent providers --json` reports installed adapter availability. Claude
launches can opt in with `--provider claude`; generated settings stay private and
do not edit user or project settings. `txm agent hooks config AGENT_ID` prints
provider configuration for review. Exact version guards apply; Codex requires
manual trust review and OpenCode v2 is unsupported. Read the
[provider support matrix](releasing.md#provider-support) before enabling hooks.

## Events, waits, and notifications

```sh
txm events --jsonl --id AGENT_ID
txm agent wait AGENT_ID --until needs-input --timeout 30s --json
txm agent notify -- /absolute/path/to/notification-handler
```

Events begin with an atomic snapshot. Gaps, overflow, and coordinator restarts
require resynchronization; there is no durable replay log or exactly-once
promise. Waits consume the same observations. Timeouts exit 6; cancellation
exits 130. `exited`, `needs-input`, and `idle` are supported wait conditions.

Notifications are off until you run `agent notify`. Your handler receives bounded
JSON metadata on stdin for future attention, turn-complete, and exit events.
Actions have deadlines and per-agent throttling; initial snapshots and resyncs
do not replay actions. Registry and event payloads exclude prompts, transcripts,
and tool contents. The [CLI contracts](contracts.md) define the delivery semantics.

## Git worktree tasks

A task combines a fresh Git branch and worktree with a supervised command. Use it
for a build, an isolated project experiment, or a worker that needs its own files.
The command starts from a committed base, `HEAD` by default; uncommitted source
changes are not copied into the task.

```sh
txm task start build --repo /absolute/repo --dry-run -- go test ./...
txm task start build --repo /absolute/repo -- go test ./...
txm task list --json
txm task inspect TASK_ID --json
```

Dry-run validates the plan without creating task state or launching work. The
default backend is native; `--backend tmux` selects tmux explicitly. `--base REV`
chooses the starting revision; `--branch NEW_BRANCH` names a fresh branch.
Repository hooks and filters require explicit `--allow-repo-commands`; no setup
script is inferred. The task ID is also its managed agent ID, so `agent wait`
can observe its exit.

Inspect the task before cleaning up:

```sh
txm task stop TASK_ID
txm task cleanup TASK_ID --dry-run
txm task cleanup TASK_ID
```

`stop` targets the exact supervised process binding. Cleanup requires exit and
ownership proof and refuses modified, untracked, or ignored files by default.
To deliberately discard those worktree files, both `--force` and
`--confirm TASK_ID` are required; neither bypasses process or ownership checks.
Cleanup retains the Git branch and committed work.

Failed or uncertain starts retain resources for inspection. Missing registrations,
moved worktrees, changed filesystem identities, or lost exit proof are not
permission to guess a cleanup target. Keep the original state directory.

## Metadata retention

Completed records count toward the 128-task and 256-agent index limits. To reclaim
index capacity, clean the task first, then prune its record and registration:

```sh
txm task prune TASK_ID --dry-run
txm task prune TASK_ID --confirm TASK_ID
txm agent prune AGENT_ID --dry-run
txm agent prune AGENT_ID --confirm AGENT_ID
```

Task pruning requires an absent worktree and validated launch/exit proof where
applicable. Supervised registrations require a validated exit receipt;
unsupervised registration pruning leaves the terminal session alive. Pruning
retains branches, receipts, manifests, and hook files, so it is not disk garbage
collection. Retired agent IDs are bounded at 16,384 and cannot be replayed.

## SSH hosts

Configure working OpenSSH authentication and host-key verification first. Install
a compatible txm on the remote host, then allow its SSH configuration alias:

```sh
txm remote allow devbox
txm remote probe devbox --json
txm remote pin devbox VERIFIED_HOST_ID
txm remote list devbox --json
```

Verify the probed host ID through a trusted channel before replacing
`VERIFIED_HOST_ID` and pinning it. OpenSSH's host key and txm's host pin are
separate identity checks. Unpinned aliases permit probing, not listing or attach.
If needed, `remote allow --executable /absolute/path/to/txm devbox` selects the
remote executable. `remote hosts` lists the allowlist; `remote forget devbox`
removes its entry without changing SSH configuration or remote sessions.

`remote list` defaults to native; use `--backend tmux` for tmux. Results are grouped
by host, with exact session and native instance IDs for attachment:

```sh
txm --backend native remote attach devbox --host-id HOST_ID --name work \
  --session-id SESSION_ID --instance INSTANCE_ID --read-only
```

Substitute the verified host ID and exact observed session/instance fields.
Attachment requires a real terminal and never creates a remote session.
OpenSSH handles credentials and ControlMaster reuse; txm sends encoded requests
and checks protocol and target identity. Per-host list timeouts and partial
failures remain explicit. No public txm network service is installed.

For scripts, use the [JSON and exit contracts](contracts.md#json-and-exits)
rather than parsing human-readable session or agent listings.
