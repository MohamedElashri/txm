#!/bin/bash

# txm shell completion script

_txm_commands() {
    local commands="create list attach detach delete new-window list-windows kill-window next-window prev-window nuke rename-session rename-window move-window swap-window split-window list-panes kill-pane resize-pane send-keys version update uninstall help"
    echo "$commands"
}

_txm_get_sessions() {
    # Get session names from tmux or screen
    if command -v tmux >/dev/null 2>&1; then
        tmux list-sessions 2>/dev/null | cut -d ':' -f 1
    elif command -v screen >/dev/null 2>&1; then
        screen -ls 2>/dev/null | grep -oP '\d+\.\K[^\t]+' || true
    fi
}

_txm_completion() {
    local cur prev words cword
    _init_completion || return

    # Handle command completion
    if [ "$cword" -eq 1 ]; then
        COMPREPLY=($(compgen -W "$(_txm_commands)" -- "$cur"))
        return 0
    fi

    # Handle arguments based on command
    local command="${words[1]}"
    case "$command" in
        attach|delete|list-windows|next-window|prev-window|rename-session)
            # Commands that take a session name as first argument
            if [ "$cword" -eq 2 ]; then
                COMPREPLY=($(compgen -W "$(_txm_get_sessions)" -- "$cur"))
                return 0
            fi
            ;;
        new-window|kill-window)
            # Commands that take a session name as first argument
            if [ "$cword" -eq 2 ]; then
                COMPREPLY=($(compgen -W "$(_txm_get_sessions)" -- "$cur"))
                return 0
            fi
            ;;
        rename-window|move-window|swap-window|split-window|list-panes)
            # Commands that take a session name as first argument
            if [ "$cword" -eq 2 ]; then
                COMPREPLY=($(compgen -W "$(_txm_get_sessions)" -- "$cur"))
                return 0
            fi
            ;;
        kill-pane|resize-pane|send-keys)
            # Commands that take a session name as first argument
            if [ "$cword" -eq 2 ]; then
                COMPREPLY=($(compgen -W "$(_txm_get_sessions)" -- "$cur"))
                return 0
            fi
            ;;
    esac

    return 0
}

# Register the completion function
complete -F _txm_completion txm

# ZSH compatibility
if [ -n "$ZSH_VERSION" ]; then
    autoload -U +X bashcompinit && bashcompinit
    complete -F _txm_completion txm
fi