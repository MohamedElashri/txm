# txm Documentation

`txm` is a command-line tool for managing tmux sessions, windows, and panes. It provides a convenient way to create, navigate, and manipulate tmux sessions and layouts.

## Table of Contents

- [txm Documentation](#txm-documentation)
  - [Table of Contents](#table-of-contents)
  - [Usage](#usage)
  - [Commands](#commands)
    - [new](#new)
    - [list](#list)
    - [attach](#attach)
    - [detach](#detach)
    - [rename](#rename)
    - [kill](#kill)
    - [switch](#switch)
    - [new-window](#new-window)
    - [rename-window](#rename-window)
    - [close-window](#close-window)
    - [switch-window](#switch-window)
    - [vsplit](#vsplit)
    - [hsplit](#hsplit)
    - [navigate](#navigate)
    - [resize](#resize)
    - [close-pane](#close-pane)
    - [zoom](#zoom)
    - [run](#run)
    - [save-layout](#save-layout)
    - [restore-layout](#restore-layout)
    - [set-option](#set-option)
    - [execute-script](#execute-script)
    - [broadcast](#broadcast)
    - [help](#help)


## Usage

To use `txm`, open a terminal and run the `txm` command followed by the desired subcommand and options. The general syntax is:

```bash
$ txm [COMMAND] [OPTIONS]
```

For example, to create a new tmux session named "mysession", you can run:

```bash
$ txm new mysession
```

## Commands

### new

Create a new tmux session.

```bash
$ txm new [SESSION_NAME]
```

- `SESSION_NAME`: The name of the session to create (default: "my_session").

Example:
```bash
$ txm new mysession
```

### list

List all tmux sessions.

```bash
$ txm list
```

### attach

Attach to a tmux session.

```bash
$ txm attach [SESSION_NAME]
```

- `SESSION_NAME`: The name of the session to attach to.

Example:
```bash
$ txm attach mysession
```

### detach

Detach from the current tmux session.

```bash
$ txm detach
```

### rename

Rename a tmux session.

```bash
$ txm rename [OLD_NAME] [NEW_NAME]
```

- `OLD_NAME`: The current name of the session.
- `NEW_NAME`: The new name for the session.

Example:
```bash
$ txm rename mysession newsession
```

### kill

Kill a tmux session.

```bash
$ txm kill [SESSION_NAME]
```

- `SESSION_NAME`: The name of the session to kill.

Example:
```bash
$ txm kill mysession
```

### switch

Switch to a different tmux session.

```bash
$ txm switch [SESSION_NAME]
```

- `SESSION_NAME`: The name of the session to switch to.

Example:
```bash
$ txm switch mysession
```

### new-window

Create a new window in the current tmux session.

```bash
$ txm new-window [WINDOW_NAME]
```

- `WINDOW_NAME`: The name of the window to create.

Example:
```bash
$ txm new-window mywindow
```

### rename-window

Rename a window in the current tmux session.

```bash
$ txm rename-window [OLD_NAME] [NEW_NAME]
```

- `OLD_NAME`: The current name of the window.
- `NEW_NAME`: The new name for the window.

Example:
```bash
$ txm rename-window mywindow newwindow
```

### close-window

Close a window in the current tmux session.

```bash
$ txm close-window [WINDOW_NAME]
```

- `WINDOW_NAME`: The name of the window to close.

Example:
```bash
$ txm close-window mywindow
```

### switch-window

Switch to a different window in the current tmux session.

```bash
$ txm switch-window [WINDOW_NAME]
```

- `WINDOW_NAME`: The name of the window to switch to.

Example:
```bash
$ txm switch-window mywindow
```

### vsplit

Split the current pane vertically.

```bash
$ txm vsplit
```

### hsplit

Split the current pane horizontally.

```bash
$ txm hsplit
```
### navigate

Navigate between panes in the current window.

```bash
$ txm navigate [DIRECTION]
```

- `DIRECTION`: The direction to navigate (U: Up, D: Down, L: Left, R: Right).

Example:
```bash
$ txm navigate U
```

### resize

Resize the current pane.

```bash
$ txm resize [DIRECTION] [AMOUNT]
```

- `DIRECTION`: The direction to resize (U: Up, D: Down, L: Left, R: Right).
- `AMOUNT`: The amount to resize the pane by.

Example:
```bash
$ txm resize D 5
```

### close-pane

Close the current pane.

```bash
$ txm close-pane
```

### zoom

Zoom in/out of the current pane.

```bash
$ txm zoom
```

### run

Execute a command in the current pane.

```bash
$ txm run [COMMAND]
```

- `COMMAND`: The command to execute.

Example:
```bash
$ txm run "ls -l"
```

### save-layout

Save the current session layout.

```bash
$ txm save-layout [LAYOUT_NAME]
```

- `LAYOUT_NAME`: The name to save the layout as.

Example:
```bash
$ txm save-layout mylayout
```

### restore-layout

Restore a previously saved session layout.

```bash
$ txm restore-layout [LAYOUT_NAME]
```

- `LAYOUT_NAME`: The name of the layout to restore.

Example:
```bash
$ txm restore-layout mylayout
```

### set-option

Set a tmux option.

```bash
$ txm set-option [OPTION] [VALUE]
```

- `OPTION`: The tmux option to set.
- `VALUE`: The value to set the option to.

Example:
```bash
$ txm set-option status-bg red
```

### execute-script

Execute a script in a specific pane.

```bash
$ txm execute-script [PANE_ID] [SCRIPT_FILE]
```

- `PANE_ID`: The ID of the pane to execute the script in.
- `SCRIPT_FILE`: The path to the script file to execute.

Example:
```bash
$ txm execute-script 1 myscript.sh
```

### broadcast

Broadcast input to all panes.

```bash
$ txm broadcast [INPUT]
```

- `INPUT`: The input to broadcast to all panes.

Example:
```bash
$ txm broadcast "echo Hello, World!"
```

### help

Display the help information.

```bash
$ txm help
```