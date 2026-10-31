# txm documentation

Use txm to keep interactive sessions running, reproduce a development workspace,
supervise background jobs, or manage work across Git repositories and SSH hosts.

## Start here

| You want to… | Read |
| --- | --- |
| Install txm and run your first session | [Getting started](getting-started.md) |
| Choose a backend, apply a profile, or use the dashboard | [User guide](user-guide.md) |
| Run jobs or coding agents, automate tasks, or connect hosts | [Automation](automation.md) |
| Check platform support or upgrade an existing installation | [Compatibility and releases](releasing.md) |

## Reference

- [CLI and state contracts](contracts.md): JSON fields, exit codes, identities,
  lifecycle, event delivery, and cleanup semantics.
- [Native protocol](native-protocol.md): framing, peer authentication, connection
  roles, and server limits for native-backend contributors.
- [Changelog](CHANGELOG.md): changes and migration instructions by version.
- `txm COMMAND --help` and `man txm`: command and flag reference.

## Development

[Contributing](../CONTRIBUTING.md) explains the development workflow.
[Building and testing](testing.md) covers dependency overrides, isolated fixtures,
race and fuzz tests, and archive validation. Maintainers use the
[release process](releasing.md#release-process) to prepare and publish builds.

These pages describe the version in this checkout. Match your binary with
`txm version`; the current version and remaining validation gates are recorded
in [release status](releasing.md).
