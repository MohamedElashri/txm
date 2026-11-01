# txm documentation

Start with the user guide; use contracts for automation and development for builds.

## Guides and reference

| You want to… | Read |
| --- | --- |
| Install and use sessions, jobs, profiles, tasks or SSH hosts | [User guide](user-guide.md) |
| Check platform support or upgrade an existing installation | [Platform scope](user-guide.md#platforms-and-backends) and [updates](user-guide.md#updates-and-migration) |
| Integrate with JSON, exit codes, state, events, or native IPC | [Contracts reference](contracts.md) |
| Connect a local MCP client or inspect opt-in process metrics | [MCP](user-guide.md#mcp-for-local-clients) and [metrics](user-guide.md#optional-process-metrics) |
| Build, test, validate packages, or prepare a release | [Development and releases](development.md) |
| Review version-specific changes and migration notes | [Changelog](CHANGELOG.md) |

Use `txm COMMAND --help` and `man txm` for the command and flag reference.
[Contributing](../CONTRIBUTING.md) explains the pull request workflow.

## Versions and documentation

These pages describe this checkout. Run `txm version`; use the matching Git tag's
docs for older builds. [VERSION](../VERSION) owns the release number, and the
changelog records upgrade notes. Evidence stays in CI. Dependency pins live in
[go.mod](../go.mod) and [scripts/dependencies.env](../scripts/dependencies.env).
