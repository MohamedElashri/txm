#!/usr/bin/env fish

# txm fish shell completion script

function __txm_commands
    echo "create list attach detach delete new-window list-windows kill-window next-window prev-window nuke rename-session rename-window move-window swap-window split-window list-panes kill-pane resize-pane send-keys version update uninstall help"
end

function __txm_get_sessions
    # Get session names from tmux or screen
    if command -v tmux >/dev/null 2>&1
        tmux list-sessions 2>/dev/null | cut -d ':' -f 1
    else if command -v screen >/dev/null 2>&1
        screen -ls 2>/dev/null | grep -o '[0-9]\+\.\([^\t]\+\)' | cut -d '.' -f 2
    end
end

# Global options
complete -c txm -s h -l help -d "Show help message and exit"
complete -c txm -s v -l verbose -d "Enable verbose output"

# Define completions for txm commands
complete -c txm -f -n "__fish_is_first_arg" -a "(__txm_commands)"

# Define completions for commands that take session names
complete -c txm -f -n "__fish_seen_subcommand_from attach delete list-windows next-window prev-window rename-session" -a "(__txm_get_sessions)"
complete -c txm -f -n "__fish_seen_subcommand_from new-window kill-window" -a "(__txm_get_sessions)"
complete -c txm -f -n "__fish_seen_subcommand_from rename-window move-window swap-window split-window list-panes" -a "(__txm_get_sessions)"
complete -c txm -f -n "__fish_seen_subcommand_from kill-pane resize-pane send-keys" -a "(__txm_get_sessions)"

# Add descriptions for main commands
complete -c txm -f -n "__fish_is_first_arg" -a "create" -d "Create a new tmux or screen session"
complete -c txm -f -n "__fish_is_first_arg" -a "list" -d "List all tmux or screen sessions"
complete -c txm -f -n "__fish_is_first_arg" -a "attach" -d "Attach to a tmux or screen session"
complete -c txm -f -n "__fish_is_first_arg" -a "detach" -d "Detach from current session"
complete -c txm -f -n "__fish_is_first_arg" -a "delete" -d "Delete a tmux or screen session"
complete -c txm -f -n "__fish_is_first_arg" -a "help" -d "Display help information"