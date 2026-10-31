# Getting started

[Documentation](README.md) / Getting started

txm keeps terminal sessions running after you detach. The built-in native backend
is enough for a persistent shell; use tmux when you need multiple windows and
panes. Additional workflows build on those sessions: profiles reproduce a layout,
agent commands supervise work, and the dashboard helps you find it.

## Installation

These instructions describe **v2.0.0**. Select its release archive or build the
matching source. The quick installer follows the latest published stable release.

### Release archives

Choose a published version on [GitHub Releases](https://github.com/MohamedElashri/txm/releases)
and download the archive for your operating system and processor:

| Platform | x86_64 / Intel | ARM64 / Apple Silicon |
| --- | --- | --- |
| Linux | `txm_Linux_x86_64.zip` | `txm_Linux_arm64.zip` |
| Linux, static musl | `txm_Linux_x86_64-musl.zip` | `txm_Linux_arm64-musl.zip` |
| macOS | `txm_Darwin_x86_64.zip` | `txm_Darwin_arm64.zip` |
| Windows | `txm_Windows_x86_64.zip` | `txm_Windows_arm64.zip` |

Static musl builds are available for systems such as Alpine Linux. Windows
archives have a limited external-backend scope; native sessions, private Unix
state, and the dashboard are unsupported. An archive target is not a runtime
validation claim; see the [platform matrix](releasing.md#platforms-and-backends).

Compare the archive with the release's checksums, then extract it into a new
directory. On Linux or macOS, run from that directory:

```sh
./txm install
export PATH="$HOME/.local/bin:$PATH"
txm version
```

The user installer copies the binary to `~/.local/bin`, installs `man txm`, and
sets up completions for the detected shell. It may update your shell startup
file. Use `sudo ./txm install --system` for an intentional system installation.

### Stable installer

For the latest stable release on Linux or macOS, use the repository's installer:

```sh
curl -fsSL https://raw.githubusercontent.com/MohamedElashri/txm/main/utils/install.sh | bash
```

It requires `curl` and `unzip`, installs under `~/.local/bin`, and can update
shell startup files to put that directory on PATH. It does not select release
candidates. To install a particular version, use its archive instead.

### Build from source

From a checkout of the version you want to run, install Go 1.26+, Zig 0.16.0,
a C compiler, `pkg-config`, and Git. macOS also needs Command Line Tools.
Then run:

```sh
make build
./bin/txm install
txm version
```

The build helper obtains the pinned Ghostty source and builds the native terminal
library. It rejects an existing dirty or mismatched dependency checkout rather
than resetting it. [Building and testing](testing.md#dependency-configuration)
explains private checkout, prefix, and compiler overrides. `go install` alone
does not build the required native library.

## Your first session

Create a session with the native backend, then attach:

```sh
txm --backend native create work
txm attach native/work
```

You now have a persistent shell. Start an editor or a long-running command, then
press **Ctrl+backslash** to detach. Your session continues running.

```sh
txm --backend native list
txm attach --read-only native/work
txm session output native/work --plain --lines 40
```

Read-only attachment lets you observe without sending input or resizing the
shared terminal. Detach with the same key. When you are finished with the session,
exit its shell or delete it explicitly:

```sh
txm delete native/work
```

## Find your work

Run `txm` to open the session picker for the selected backend, or choose the
native backend explicitly:

```sh
txm --backend native
txm dashboard --backend native
```

In the dashboard, `/` searches, Enter attaches, `o` observes read-only, and `q`
quits. The [user guide](user-guide.md#dashboard) covers filters and previews.

For repeatable layouts, continue with [profiles](user-guide.md#profiles).
For commands with recorded exit results, start with
[supervised jobs](automation.md#supervised-jobs).
