# txm documentation

Use txm to keep interactive sessions running, reproduce a development workspace,
supervise background jobs, or manage work across Git repositories and SSH hosts.

## Guides and reference

| You want to… | Read |
| --- | --- |
| Install, manage sessions, use profiles/dashboard, or automate jobs, tasks, and SSH hosts | [User guide](user-guide.md) |
| Check platform support or upgrade an existing installation | [Platform scope](user-guide.md#platforms-and-backends) and [updates](user-guide.md#updates-and-migration) |
| Integrate with JSON, exit codes, state, events, or native IPC | [Contracts reference](contracts.md) |
| Connect a local MCP client or inspect opt-in process metrics | [MCP](user-guide.md#mcp-for-local-clients) and [metrics](user-guide.md#optional-process-metrics) |
| Build, test, validate packages, or prepare a release | [Development and releases](development.md) |
| Review version-specific changes, migration notes, and validation evidence | [Changelog](CHANGELOG.md) |

Use `txm COMMAND --help` and `man txm` for the command and flag reference.
[Contributing](../CONTRIBUTING.md) explains the pull request workflow.

## Versions and documentation

These pages describe the source in this checkout. Run `txm version` to identify
your binary; for an older release, read the docs at its matching Git tag.

[VERSION](../VERSION) owns the build/release version. The changelog owns dated
release notes, upgrade instructions, and historical validation evidence. Guides
use version-independent examples or read `VERSION` when a version is required.
Dependency pins live in [go.mod](../go.mod),
[scripts/dependencies.env](../scripts/dependencies.env), and the build/workflow
configuration. Protocol and schema numbers are compatibility contracts and stay
explicit in the reference.
