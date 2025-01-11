#!/bin/bash

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Detect the operating system
OS=""
case "$(uname -s)" in
    Darwin*)
        OS="macOS"
        SYS_MAN_DIR="/usr/local/share/man/man1"
        ;;
    Linux*)
        OS="Linux"
        SYS_MAN_DIR="/usr/local/share/man/man1"
        ;;
    *)
        echo -e "${RED}Unsupported operating system. This script only supports macOS and Linux.${NC}"
        exit 1
        ;;
esac

# Set up directories
USER_BIN_DIR="$HOME/.local/bin"
USER_MAN_DIR="$HOME/.local/share/man/man1"
SYSTEM_BIN_DIR="/usr/local/bin"

# Function to print error messages and exit
error_exit() {
    echo -e "${RED}Error: $1${NC}"
    exit 1
}

# Function to remove a file with appropriate privileges
remove_file() {
    local file="$1"
    local is_system="$2"

    if [ -f "$file" ]; then
        if [ "$is_system" = true ]; then
            sudo rm -f "$file" || error_exit "Failed to remove $file"
        else
            rm -f "$file" || error_exit "Failed to remove $file"
        fi
    fi
}

# Function to update man database
update_man_db() {
    local is_system="$1"
    echo -e "${BLUE}Updating the man page index...${NC}"
    
    if [ "$OS" == "macOS" ]; then
        if [ "$is_system" = true ]; then
            sudo /usr/libexec/makewhatis "$SYS_MAN_DIR"
        else
            /usr/libexec/makewhatis "$USER_MAN_DIR" 2>/dev/null || true
        fi
    else
        if [ "$is_system" = true ]; then
            sudo mandb
        else
            mandb --user-db "$HOME/.local/share/man" 2>/dev/null || true
        fi
    fi
}

# Function to remove shell configuration
remove_shell_config() {
    local config_files=(".bashrc" ".zshrc")
    for config in "${config_files[@]}"; do
        if [ -f "$HOME/$config" ]; then
            echo -e "${BLUE}Removing txm PATH from $config...${NC}"
            sed -i.bak '/export PATH="\$HOME\/.local\/bin:\$PATH"/d' "$HOME/$config"
            rm -f "$HOME/$config.bak"
        fi
    done
}

# Check if txm is installed
TXM_PATH=$(which txm 2>/dev/null)
if [ -z "$TXM_PATH" ]; then
    echo -e "${YELLOW}txm is not installed on this system.${NC}"
    exit 0
fi

# Determine installation type
IS_SYSTEM_INSTALL=false
if [[ "$TXM_PATH" == "/usr/local/bin/"* ]]; then
    IS_SYSTEM_INSTALL=true
    if [ "$EUID" -ne 0 ]; then
        error_exit "System-wide uninstallation requires root privileges. Please run with sudo."
    fi
fi

# Check if running in non-interactive mode
if [ -t 0 ]; then
    # Running interactively
    read -p "Are you sure you want to uninstall txm? (y/n): " CONFIRM
    if [ "$CONFIRM" != "y" ]; then
        echo -e "${BLUE}Uninstall aborted. Exiting.${NC}"
        exit 0
    fi
else
    # Running non-interactively (e.g., piped from curl)
    echo -e "${YELLOW}Running in non-interactive mode. Proceeding with uninstallation.${NC}"
fi

# Remove binary
echo -e "${BLUE}Removing txm binary...${NC}"
if [ "$IS_SYSTEM_INSTALL" = true ]; then
    remove_file "$SYSTEM_BIN_DIR/txm" true
else
    remove_file "$USER_BIN_DIR/txm" false
fi

# Remove man page
echo -e "${BLUE}Removing txm man page...${NC}"
if [ "$IS_SYSTEM_INSTALL" = true ]; then
    remove_file "$SYS_MAN_DIR/txm.1" true
else
    remove_file "$USER_MAN_DIR/txm.1" false
fi

# Update man database
update_man_db "$IS_SYSTEM_INSTALL"

# Remove configuration files
echo -e "${BLUE}Removing txm configuration files...${NC}"
rm -rf ~/.txm 2>/dev/null
rm -f ~/.txmrc 2>/dev/null

# Remove cache files
echo -e "${BLUE}Removing txm cache files...${NC}"
rm -rf ~/.cache/txm 2>/dev/null

# Remove logs
echo -e "${BLUE}Removing txm log files...${NC}"
rm -rf ~/.local/share/txm 2>/dev/null

# Remove PATH from shell configuration if it's a user installation
if [ "$IS_SYSTEM_INSTALL" = false ]; then
    remove_shell_config
fi

# Verify uninstallation
if ! command -v txm &> /dev/null; then
    echo -e "${GREEN}txm has been successfully uninstalled from your system.${NC}"
    if [ "$IS_SYSTEM_INSTALL" = true ]; then
        echo -e "${YELLOW}System-wide installation was removed.${NC}"
    else
        echo -e "${YELLOW}User-local installation was removed.${NC}"
    fi
else
    echo -e "${RED}Uninstallation may have been incomplete. Please check manually.${NC}"
fi

# Additional cleanup suggestions
echo -e "${YELLOW}Note: If you have made any additional customizations or configurations, you may need to remove them manually.${NC}"