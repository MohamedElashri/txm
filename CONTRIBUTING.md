# Contributing to txm

Bug reports, focused fixes, documentation improvements, and backend contributions
are welcome. Use [GitHub Issues](https://github.com/MohamedElashri/txm/issues) to
discuss substantial changes before opening a pull request.

## Work on the project

1. Start from the branch targeted by your pull request. Keep changes focused on
   one problem and preserve unrelated local work.
2. Follow [Building and testing](docs/testing.md) for dependencies and isolated
   fixtures. Run the checks appropriate to the change.
3. Update the relevant [documentation](docs/README.md). Put user workflows in the
   guides and machine-readable guarantees in [CLI contracts](docs/contracts.md).
4. Explain the problem, resulting behavior, and validation in the pull request.
   State the OS, architecture, backend versions, and any checks you could not run.

## Validate safely

Use private fixture sessions, repositories, homes, sockets, and SSH keys. Do not
run destructive tests against your own active sessions or default backend server.
Never include credentials, raw provider prompts, or user transcripts in reports.
Keep generated binaries, archives, recordings, and validation reports outside
source control; attach relevant evidence to CI or the pull request.

A cross-build proves compilation, not runtime support. Backend changes should
preserve exact targeting, truthful capability reporting, terminal restoration,
and failure exits. Changes to supervision must retain the distinction between
observed terminal output and a validated child exit result.

For documentation changes, check relative links, examples, and agreement with
`txm COMMAND --help`. Keep the README focused on the project and first use;
put detailed instructions in the guide that owns the topic.

Release metadata and publishing steps are in [Compatibility and releases](docs/releasing.md).
