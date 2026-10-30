# txm user guide

Use `txm COMMAND --help` for the complete flags and aliases. See
[CLI contracts](contracts.md) for machine-readable behavior and
[Releasing](releasing.md) for support limits and native-session migration.

## Sessions and backends

```sh
txm                         # existing interactive picker
txm create work /bin/sh
txm list
txm attach work
txm attach --read-only work
txm delete work
txm --backend tmux list
txm session inspect tmux/work --json
txm backend capabilities --all-backends --json
```

Explicit `--backend` is strict; `backend/name` qualifies a target. Config/environment
settings are preferences with fallback. Unqualified mutations stay in the selected
backend. `nuke` removes that backend's sessions. Normal attach can create a missing
session after successful discovery; read-only attach never creates one.

| Backend | Supported scope |
| --- | --- |
| Native | One persistent terminal, read-only attach, bounded visible snapshots and terminal input; no rename, windows, layouts or history |
| tmux | Exact sessions/windows/panes, layouts, cwd, read-only attach and bounded screen/history; profiles require 3.4+ |
| Screen/Zellij | Degraded basic session lifecycle/attach; exact pane/layout/capture/read-only operations are unsupported |

Native detaches with Ctrl+backslash. tmux retains its configured detach binding
(default Ctrl-b, then d). Input acceptance is never a command-completion result.

## Output and exact pane operations

```sh
txm session output native/work --lines 100 --plain
txm session output tmux/work --history --lines 1000 --json
txm session output native/work --raw --json
txm --backend tmux window new work editor
txm --backend tmux window list work
txm --backend tmux pane list work editor
txm --backend tmux pane exec work editor PANE_ID 'printf hello'
```

Plain capture strips terminal controls and discloses truncation. Raw is explicit;
JSON raw content is base64. Native output is the visible screen, not history.
Native `txm exec work 0 0 'command'` sends literal input plus Enter. Exact tmux
input refuses synchronized panes rather than broadcasting. Prefer returned IDs
to reusable names/indexes; ambiguity and unsupported targets fail before mutation.

## Profiles

```sh
txm profile --dry-run dev.toml
txm profile --detach dev.toml
txm profile --attach-existing dev.toml
txm profile --attach-existing --read-only dev.toml
```

```toml
name = "dev"
backend = "tmux"
[[window]]
name = "editor"
[[window.pane]]
dir = "."
command = "vim"
```

Strict preflight validates cwd/topology/capabilities before creation. Relative cwd
resolves against the profile file; no implicit `~` or environment expansion occurs.
Default apply attaches; `--detach` leaves it detached. Existing sessions are
unchanged and require `--attach-existing`. Read-only requires an existing session.
Native profiles permit one terminal without named windows/layouts; Screen/Zellij
profiles are unsupported. Commands remain terminal input, without exit receipts.
Rollback deletes only proven owned resources; uncertain survivors are reported.

## Supervised agents

```sh
txm --backend native agent start build -- go test ./...
txm --backend tmux agent start --cwd /absolute/project worker -- /path/to/worker
txm agent list --json
txm agent status AGENT_ID --json
txm agent wait AGENT_ID --until exited --timeout 30s --json
txm agent wait AGENT_ID --until exited --fail-on-nonzero --json
txm agent register tmux/existing-session
txm agent stop-coordinator
```

Agent workflows opt into a private local coordinator; ordinary sessions and the
picker do not contact it. Native/tmux managed launches execute structured argv
and publish immutable receipts. Registration is unsupervised. Missing proof is
unknown; output text is never proof of exit. Stopping the coordinator leaves
sessions and children running. `--env NAME` allowlists names from the launch
environment; it does not upload the invoking client's values.

Default wait success means the condition was observed, including a nonzero child
exit. `--fail-on-nonzero` propagates an unsuccessful receipt. Default timeout is
30 seconds; cancellation returns 130. Use full returned IDs in automation.

## Signals, events and notifications

```sh
txm agent hooks enable AGENT_ID --provider generic
txm events --jsonl --id AGENT_ID
txm agent wait AGENT_ID --until needs-input --timeout 30s --json
txm agent notify -- /absolute/path/to/notification-handler
```

Within an opted-in managed worker, `txm agent signal --from-env --state working`
can submit authenticated attention; states also include needs-input and idle.
Lifecycle, attention and turn completion are separate observations. Attention
expires, credentials rotate and replay is rejected. Event gaps/restarts require
resync; there is no replay log. Notifications default off and receive bounded safe
metadata on stdin, never transcripts or raw commands.

Claude hook setup is explicit; Codex config requires manual review and provider
trust. Version guards reject unsupported providers. See
[provider support](releasing.md#provider-support) before enabling an integration.

## Dashboard

```sh
txm dashboard --backend native
txm dashboard --agents --preview
```

Search with `/`, filter with Tab/b, attach with Enter, inspect read-only with o,
toggle bounded preview with p and quit with q. Stale and unsupported actions are
disabled. Agent rows require explicit `--agents`; reusable names never join
registrations to sessions. Linux/macOS guards prevent feedback into the dashboard's
own native terminal. Terminal state restores across exit, cancellation and attach.

## Git tasks

```sh
txm task start build --repo /absolute/repo --dry-run -- go test ./...
txm task start build --repo /absolute/repo -- go test ./...
txm task list --json
txm task inspect TASK_ID --json
txm task stop TASK_ID --json
txm task cleanup TASK_ID --dry-run --json
txm task cleanup TASK_ID --json
txm task prune TASK_ID --dry-run
txm task prune TASK_ID --confirm TASK_ID
```

Tasks use fresh owned worktrees/branches and supervised IDs. Defaults are native;
explicit tmux is supported. Repository hooks/filters require explicit
`--allow-repo-commands`. Failed/uncertain launches retain resources for inspection.
Cleanup requires ownership and exit proof. Dirty/untracked/ignored removal needs
force plus full-ID confirmation; force cannot bypass ownership. Branches and
receipts remain. After task cleanup/pruning, `agent prune ID --confirm ID` can
remove eligible registration metadata. Pruned managed IDs cannot be reused.

## SSH aggregation

```sh
txm remote allow devbox
txm remote probe devbox --json
txm remote pin devbox VERIFIED_HOST_ID
txm remote list devbox --json
txm --backend native remote attach devbox --host-id HOST_ID --name work \
  --session-id SESSION_ID --instance INSTANCE_ID --read-only
txm remote forget devbox
```

OpenSSH handles authentication and host keys. txm separately requires an explicit
alias allowlist and verified host pin. Use the executable override when the
remote txm is not on PATH. Requests check versions and exact observed identities;
partial failures stay explicit. Native attach requires the launch instance; tmux
IDs are server-scoped. No remote session is created by attach. Interactive attach
uses a real terminal and restores its settings. Remote names remain encoded data.
`generate-ssh-config` remains available for the simpler existing SSH workflow.

## Configuration and installation

Config search order: `~/.txm/config`, `~/.txm/config.txt`, then `~/.txmrc`.

```sh
txm config set backend tmux
txm config get backend
txm config show
txm install                 # private user installation
txm install --system        # system installation, requires privilege
txm uninstall
txm version
```

`TXM_DEFAULT_BACKEND` sets a fallback preference; `TXM_SESSION_PREFIX` prefixes
names once; `NO_COLOR` disables color. New private state honors absolute
`XDG_STATE_HOME`; sockets honor private `XDG_RUNTIME_DIR`. Invalid ownership,
permissions and explicit paths fail rather than silently switching namespaces.
`TERM` should describe the actual terminal.

Stable builds support `txm update`. Development/prerelease builds refuse automatic
stable updates; install a selected release explicitly. For legacy native sessions,
retain the old binary and original user/TMPDIR/configuration until they finish.
