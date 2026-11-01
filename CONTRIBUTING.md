# Contributing to txm

Report bugs and discuss substantial changes in
[GitHub Issues](https://github.com/MohamedElashri/txm/issues).

## Work on the project

1. Branch from your PR's target and keep the change focused.
2. Follow [Development and releases](docs/development.md) for dependencies,
   isolated fixtures and checks appropriate to your change.
3. Update the [user guide](docs/user-guide.md) for workflows or
   [contracts](docs/contracts.md) for integration guarantees.
4. Describe the problem, resulting behavior and checks in your PR. Include
   relevant platform/backend versions and anything you could not test.

## Validate safely

Use private sessions, repositories, homes, sockets and SSH keys. Never target
personal sessions or the default backend server. Keep credentials, prompts and
transcripts out of reports; keep generated artifacts out of source control.

A cross-build proves compilation, not runtime support. Preserve exact targeting,
capability reporting, terminal restoration and failure exits. Terminal output
does not prove a supervised child's result.

Keep docs short and current. Check links and examples against `txm COMMAND --help`.
Keep the README focused on first use; link to the guide that owns the detail.
Read `VERSION` and dependency pins instead of copying versions into examples.
Put user-facing changes and upgrade notes in the changelog; keep evidence in CI.

Release metadata and publishing steps are in the
[release process](docs/development.md#release-process).
