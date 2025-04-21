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

# GitHub raw content URLs for completion scripts
BASH_COMPLETION_URL="https://raw.githubusercontent.com/MohamedElashri/txm/main/utils/completion.sh"
FISH_COMPLETION_URL="https://raw.githubusercontent.com/MohamedElashri/txm/main/utils/completion.fish"

# Ensure we're in a valid directory
cd "$HOME" || exit 1

# Download and install bash completion
curl -fsSL "$BASH_COMPLETION_URL" > "$HOME/.local/share/bash-completion/completions/txm"

# Download and install fish completion if fish is installed
if command -v fish >/dev/null 2>&1; then
    curl -fsSL "$FISH_COMPLETION_URL" > "$HOME/.config/fish/completions/txm.fish"
fi

# Install zsh completion
if [ "$SHELL_NAME" = "zsh" ]; then
    # Ensure .zshrc sources completions
    if ! grep -q "fpath=(~/.zsh/completion \$fpath)" "$HOME/.zshrc" 2>/dev/null; then
        echo "\n# Add custom completion directory" >> "$HOME/.zshrc"
        echo "fpath=(~/.zsh/completion \$fpath)" >> "$HOME/.zshrc"
        echo "autoload -U compinit && compinit" >> "$HOME/.zshrc"
    fi
    
    # Download and install zsh completion
    curl -fsSL "$BASH_COMPLETION_URL" > "$HOME/.zsh/completion/_txm"
fi

echo "Shell completion installed. You may need to restart your shell or source your shell configuration file."
echo "For bash: source ~/.bashrc"
echo "For zsh: source ~/.zshrc"
echo "For fish: No action needed, completions are automatically loaded."