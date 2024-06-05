# txm - A tmux Helper Tool

`txm` is a command-line utility designed to make working with tmux more efficient and user-friendly. It provides a set of commands to manage tmux sessions, windows, panes, and perform various actions. This is the Go port (and extension) of the original [txm](https://github.com/MohamedElashri/txm) written in Python. Another bash version can be found [here](https://github.com/MohamedElashri/txm-bash).

## Features

- Create, list, attach to, detach from, rename, and kill tmux sessions
- Create, rename, close, and switch between windows
- Split panes vertically and horizontally
- Navigate between panes using arrow keys
- Resize panes
- Close panes
- Zoom in/out of panes
- Execute commands in panes
- Save and restore session layouts
- Set tmux options
- Execute scripts in specific panes
- Broadcast input to all panes

## Installation

### Using Pre-built Binaries

You can download pre-built binaries for your operating system from the [releases page](https://github.com/MohamedElashri/txm-go/releases). Look for the latest release and download the appropriate ZIP file for your platform:

- `txm-ubuntu-latest.zip`: Ubuntu (Linux)
- `txm-macOS-latest.zip`: macOS

Once downloaded, extract the ZIP file to obtain the `txm` binary. You can then move the binary to a directory in your system's `PATH` for easy access from the command line.

For example, on Unix-based systems (Linux and macOS), you can follow these steps:

1. Download the appropriate ZIP file for your platform.
2. Extract the ZIP file:

   ```bash
   unzip txm-<platform>.zip
   ```
3. Move the `txm` binary to a directory in your `PATH`, such as `/usr/local/bin`:

   ```bash
   sudo mv txm-<platform> /usr/local/bin/txm
   ```
   Replace `<platform>` with the appropriate platform name (`ubuntu-latest`, or `macOS-latest`).


### Building from Source

If you prefer to build `txm` from the source code, follow these steps:

1. Ensure you have [Go](https://golang.org/) installed on your system (version 1.17 or later).

2. Clone the `txm-go` repository:

   ```bash
   git clone https://github.com/MohamedElashri/txm-go
   ```

3. Navigate to the project directory:

   ```bash
   cd txm-go
   ```

4. Initialize the Go module:

   ```bash
   go mod init github.com/MohamedElashri/txm-go
   ```

5. Build the `txm` binary:
   
   ```bash
   go build -o txm
   ```

   This will create a binary named `txm` in the current directory.

6. (Optional) Move the `txm` binary to a directory in your system's `PATH` for easy access from the command line.

   For example, on Unix-based systems (Linux and macOS):
   ```
   sudo mv txm /usr/local/bin/
   ```

After completing the installation steps, you can run `txm` from the command line to start using the tmux manager.


## Usage

`txm` provides a wide range of commands to interact with tmux. Here are a few examples:

- Create a new session:
  ```bash
  txm new my-session
  ```

- List all sessions:
  ```bash
  txm list
  ```

- Attach to a session:
  ```bash
  txm attach my-session
  ```

- Split pane vertically:
  ```bash
  txm vsplit
  ```

- Resize pane up by 5 units:
  ```bash
  txm resize U 5
  ```

- Execute a script in a specific pane:
  ```bash
  txm execute-script 0.1 script.sh
  ```

For a complete list of available commands and their usage, refer to the man page:

```bash
man txm
```

Or you can consult the [documentation](docs.md)

## Configuration

`txm` can be configured using a configuration file located at `$HOME/.txm/config`. The configuration file allows you to set default options and customize the behavior of `txm`.

## Contributing

Contributions are welcome! If you find any issues or have suggestions for improvements, please open an issue or submit a pull request on the GitHub repository.

## License

This project is licensed under the [GNU 3.0 License](LICENSE).

