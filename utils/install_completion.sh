#!/bin/bash

# Script to install txm shell completion

set -e

echo "Installing txm shell completion..."

# Determine the shell
SHELL_NAME=$(basename "$SHELL")

# Create completion directories if they don't exist
mkdir -p "$HOME/.local/share/bash-completion/completions" 2>/dev/null || true
mkdir -p "$HOME/.config/fish/completions" 2>/dev/null || true
mkdir -p "$HOME/.zsh/completion" 2>/dev/null || true

# Get the directory where txm is installed
TXM_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/.."

# Install bash completion
cp "$TXM_DIR/utils/completion.sh" "$HOME/.local/share/bash-completion/completions/txm"

# Install fish completion if fish is installed
if command -v fish >/dev/null 2>&1; then
    cp "$TXM_DIR/utils/completion.fish" "$HOME/.config/fish/completions/txm.fish"
fi

# Install zsh completion
if [ "$SHELL_NAME" = "zsh" ]; then
    # Ensure .zshrc sources completions
    if ! grep -q "fpath=(~/.zsh/completion \$fpath)" "$HOME/.zshrc" 2>/dev/null; then
        echo "\n# Add custom completion directory" >> "$HOME/.zshrc"
        echo "fpath=(~/.zsh/completion \$fpath)" >> "$HOME/.zshrc"
        echo "autoload -U compinit && compinit" >> "$HOME/.zshrc"
    fi
    
    # Copy completion script
    cp "$TXM_DIR/utils/completion.sh" "$HOME/.zsh/completion/_txm"
fi

echo "Shell completion installed. You may need to restart your shell or source your shell configuration file."
echo "For bash: source ~/.bashrc"
echo "For zsh: source ~/.zshrc"
echo "For fish: No action needed, completions are automatically loaded."