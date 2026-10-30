# M2 readiness

Phase 5 implements the Linux CLI workflow: generic supervision, authenticated
attention signals, events, deterministic waits and explicit notification actions.
M2 release status remains **Blocked** by inherited unrun M1 platform gates. No
Phase 6/TUI, release tag, commit or publication is authorized by this work.
The validation record is [Phase 5 evidence](evidence/phase5-validation.json).

## Provider support (reviewed 2026-10-30)

| Provider | Installed version | Enabled behavior | Runtime evidence / limits |
| --- | --- | --- | --- |
| Generic | schema 1; no provider needed | Explicit authenticated unknown/working/needs-input/idle/turn-complete signals; managed context opt-in | Real native/tmux supervised fixtures; source/freshness/exit semantics remain separate |
| Claude Code | `2.1.308 (Claude Code)` | `--provider claude` generates a private settings file; manual configuration also available | Real `--init-only` SessionStart with private HOME, no model/auth; full prompt/tool/permission conversations unrun |
| Codex | `codex-cli 0.165.0`; installed `hooks` feature stable/enabled | Observational hook schema and generated config for manual review/trust | Translation fixtures and installed version/feature inventory; actual trusted provider lifecycle unrun; automatic activation/trust bypass unsupported |
| OpenCode | `opencode v2.2.1` | Unsupported | Reviewed v2 plugin/server docs; session association and plugin runtime unverified; older v1 examples are not used |
| Other versions | Any version outside the exact allowlist | Unsupported | Reverify primary docs, payload schema and runtime before expanding support |

`agent providers --json` reports installed versions and adapter schema availability;
`supported=true` for Codex means the reviewed schema/config bridge is available,
not that a full trusted model conversation was tested. Provider absence never
blocks generic signals, exit waits or ordinary sessions.

Primary sources: [Claude hooks](https://code.claude.com/docs/en/hooks),
[Codex hooks](https://learn.chatgpt.com/docs/hooks),
[OpenCode v2 plugins](https://opencode.ai/v2/docs/build/plugins/) and
[OpenCode v2 server](https://opencode.ai/v2/docs/build/plugins/server/).
The installed CLI `--version`/`--help` inventory and Codex feature listing used a
constructed private environment; no user configuration or credential was read.

Both implemented provider bridges select only event name, session ID and Stop's
recursion flag from bounded JSON input. SessionStart reports unknown;
UserPromptSubmit reports working; PermissionRequest reports needs_input; Stop
reports idle only with explicit `stop_hook_active=false`. PermissionRequest can
precede a prompt or an automatic decision: it is a sourced observation, not proof
that a user must intervene. Other hook payloads are ignored. A Stop may be
continued by other provider hooks, so it never reports final turn completion or
process exit. First accepted input binds a provider session. Different sessions,
stale generations, replayed sequences and expired timestamps cannot update it.

Bridge stdout is `{}`; failures emit a fixed warning with exit 0, leaving provider
decisions unchanged. Generated settings contain no approval/block output, auth
material or permission bypass. They do not edit project/user provider settings.
Codex requires its normal manual hook trust review before use. Tokens remain
in private local files, with same UID as the trust boundary. No transcript is
opened or forwarded. Events omit prompts, provider payloads and terminal content;
user-controlled names still require safe rendering by JSON consumers.

## Release limits

All final Linux checks pass: Go 1.26.10/1.27.3, Go race, zero-issue lint,
dependency verification, 10 harness guards and all retained M1/Phase 4 suites;
14 new Phase 5 groups normally and with a race executable; both actual GNU/musl
snapshot ZIPs with private install/completion/content checks and every runtime
suite, including real Claude initialization. Native/plain/local-frame fuzz pass
30/10/15-second runs (1,520,064 / 45,361 / 587,795 executions). Windows amd64/arm64
cross-builds and CGO-disabled runtime refusal checks pass. Fixed 1/10/100 idle
session budgets pass; these are host observations, not a coordinator-scale SLA.

Required
macOS, Linux arm64, Windows external-backend runtime/archive/install checks and
foreign-UID end-to-end rejection are still unrun as recorded in
[M1 readiness](m1-readiness.md). Windows cross-builds only prove compilation.
Configured CI is not runtime evidence. Provider model conversations and desktop
notifications were not exercised; optional action tests use fixture scripts.
Generic M2 remains usable without those provider-specific optional capabilities.

CLI/coordinator development IPC requires a matching schema/build. Stop an older
experimental coordinator using its matching executable before upgrading; managed
children and immutable receipts remain independent. No automatic protocol
fallback, migration or relaunch is claimed.

Latest Linux audit, recovery/protocol hardening and final validation are recorded
in [the Phase 8 audit](final-audit.md). Historical results above retain their
original scope; unrun release gates remain Blocked.
