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
    MAN_DIR="/usr/local/share/man/man1"
    ;;
  Linux*)
    OS="Linux"
    MAN_DIR="/usr/local/share/man/man1"
    ;;
  *)
    echo -e "${RED}Unsupported operating system. This script only supports macOS and Linux.${NC}"
    exit 1
    ;;
esac

# Function to print error messages and exit
error_exit() {
  echo -e "${RED}Error: $1${NC}"
  exit 1
}

# Check if txm is installed
if ! command -v txm &> /dev/null; then
  echo -e "${YELLOW}txm is not installed on this system.${NC}"
  exit 0
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

# Remove the txm binary
echo -e "${BLUE}Removing txm binary...${NC}"
sudo rm -f /usr/local/bin/txm || error_exit "Failed to remove txm binary."

# Remove the txm man page
echo -e "${BLUE}Removing txm man page...${NC}"
sudo rm -f "$MAN_DIR/txm.1" || error_exit "Failed to remove txm man page."

# Update the man page index
echo -e "${BLUE}Updating the man page index...${NC}"
if [ "$OS" == "macOS" ]; then
  sudo /usr/libexec/makewhatis "$MAN_DIR"
else
  sudo mandb
fi

# Remove any configuration files or directories (if applicable)
echo -e "${BLUE}Removing txm configuration files...${NC}"
rm -rf ~/.txm 2>/dev/null
rm -f ~/.txmrc 2>/dev/null

# Remove any cache files or directories (if applicable)
echo -e "${BLUE}Removing txm cache files...${NC}"
rm -rf ~/.cache/txm 2>/dev/null

# Remove any logs (if applicable)
echo -e "${BLUE}Removing txm log files...${NC}"
rm -rf ~/.local/share/txm/logs 2>/dev/null

# Check if uninstallation was successful
if ! command -v txm &> /dev/null && [ ! -f "$MAN_DIR/txm.1" ]; then
  echo -e "${GREEN}txm has been successfully uninstalled from your system.${NC}"
else
  echo -e "${RED}Uninstallation may have been incomplete. Please check manually.${NC}"
fi

# Remind user about possible system-wide configurations
echo -e "${YELLOW}Note: If you have made any system-wide configurations related to txm, you may need to remove them manually.${NC}"

# Suggest shell configuration cleanup
echo -e "${YELLOW}Don't forget to remove any txm-related lines from your shell configuration files (e.g., .bashrc, .zshrc) if you added any.${NC}"