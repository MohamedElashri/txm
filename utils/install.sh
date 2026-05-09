#!/bin/bash

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# GitHub repository information
REPO_OWNER="MohamedElashri"
REPO_NAME="txm"
LATEST_RELEASE_URL="https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/latest"

# Detect the operating system and architecture
OS=""
ARCH=""

case "$(uname -s)" in
    Darwin*)
        OS="Darwin"
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

case "$(uname -m)" in
    x86_64|amd64)
        ARCH="x86_64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo -e "${RED}Unsupported architecture: $(uname -m). Only x86_64 and arm64 are supported.${NC}"
        exit 1
        ;;
esac

# The archive name as produced by GoReleaser, e.g. txm_Linux_x86_64.zip
ARCHIVE_NAME="txm_${OS}_${ARCH}.zip"

# Set up installation directories
USER_BIN_DIR="$HOME/.local/bin"
SYSTEM_BIN_DIR="/usr/local/bin"

# Function to print error messages and exit
error_exit() {
    echo -e "${RED}Error: $1${NC}"
    exit 1
}

# Function to create directory if it doesn't exist
create_dir_if_needed() {
    if [ ! -d "$1" ]; then
        mkdir -p "$1" || error_exit "Failed to create directory: $1"
    fi
}

# Check dependencies
if ! command -v curl &> /dev/null; then
    error_exit "curl is not installed. Please install curl and try again."
fi

if ! command -v unzip &> /dev/null; then
    error_exit "unzip is not installed. Please install unzip and try again."
fi

# Parse command line arguments
SYSTEM_INSTALL=false
while [[ $# -gt 0 ]]; do
    case $1 in
        --system)
            SYSTEM_INSTALL=true
            shift
            ;;
        *)
            error_exit "Unknown option: $1"
            ;;
    esac
done

# Set installation directories based on installation type
if [ "$SYSTEM_INSTALL" = true ]; then
    if [ "$EUID" -ne 0 ]; then
        error_exit "System-wide installation requires root privileges. Please run with sudo."
    fi
    INSTALL_BIN_DIR="$SYSTEM_BIN_DIR"
    echo -e "${BLUE}Performing system-wide installation...${NC}"
else
    INSTALL_BIN_DIR="$USER_BIN_DIR"
    echo -e "${BLUE}Performing user-local installation...${NC}"
fi

# Create necessary directories
create_dir_if_needed "$INSTALL_BIN_DIR"

# Add ~/.local/bin to PATH if needed
if [ "$SYSTEM_INSTALL" = false ]; then
    if [[ ":$PATH:" != *":$USER_BIN_DIR:"* ]]; then
        echo -e "${YELLOW}Adding $USER_BIN_DIR to your PATH...${NC}"
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.bashrc"
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.zshrc" 2>/dev/null || true
        echo -e "${YELLOW}Please restart your shell or run 'source ~/.bashrc' to update your PATH.${NC}"
    fi
fi

# Fetch the latest release information
echo -e "${BLUE}Fetching the latest release information...${NC}"
RELEASE_INFO=$(curl -sf "$LATEST_RELEASE_URL") || error_exit "Failed to fetch release information from GitHub."

LATEST_VERSION=$(echo "$RELEASE_INFO" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')

if [ -z "$LATEST_VERSION" ]; then
    error_exit "Could not determine the latest release version."
fi

echo -e "${BLUE}Latest version: ${LATEST_VERSION}${NC}"

# Check if already installed
if command -v txm &> /dev/null; then
    CURRENT_VERSION=$(txm version 2>/dev/null | grep -o '[0-9]\+\.[0-9]\+\.[0-9]\+' | head -1 || echo "unknown")
    if [ "$CURRENT_VERSION" = "$(echo "$LATEST_VERSION" | sed 's/^v//')" ]; then
        echo -e "${YELLOW}txm $CURRENT_VERSION is already installed and up to date.${NC}"
        read -rp "Do you want to re-install txm? (y/n): " REINSTALL
        if [ "$REINSTALL" != "y" ]; then
            echo -e "${BLUE}Installation aborted.${NC}"
            exit 0
        fi
    fi
fi

# Find the download URL for the platform archive
DOWNLOAD_URL=$(echo "$RELEASE_INFO" | grep '"browser_download_url"' | grep "$ARCHIVE_NAME" | sed 's/.*"browser_download_url": *"\([^"]*\)".*/\1/')

if [ -z "$DOWNLOAD_URL" ]; then
    error_exit "No compatible binary found for ${OS} ${ARCH} (looking for ${ARCHIVE_NAME}) in release ${LATEST_VERSION}."
fi

echo -e "${BLUE}Downloading ${ARCHIVE_NAME}...${NC}"

# Create temporary directory
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT
cd "$TMP_DIR" || error_exit "Failed to create temporary directory"

curl -Lf "$DOWNLOAD_URL" -o "$ARCHIVE_NAME" || error_exit "Failed to download $ARCHIVE_NAME."

echo -e "${BLUE}Extracting archive...${NC}"
unzip -q "$ARCHIVE_NAME" || error_exit "Failed to extract $ARCHIVE_NAME."

# The binary inside the archive is named 'txm' (or 'txm.exe' on Windows)
if [ ! -f "txm" ]; then
    error_exit "Extracted archive does not contain a 'txm' binary. Contents: $(ls)"
fi

echo -e "${BLUE}Installing binary to $INSTALL_BIN_DIR/txm...${NC}"
if [ "$SYSTEM_INSTALL" = true ]; then
    sudo mv txm "$INSTALL_BIN_DIR/txm" || error_exit "Failed to install binary."
    sudo chmod 755 "$INSTALL_BIN_DIR/txm"
else
    mv txm "$INSTALL_BIN_DIR/txm" || error_exit "Failed to install binary."
    chmod 755 "$INSTALL_BIN_DIR/txm"
fi

# Verify the binary works
echo -e "${BLUE}Verifying installation...${NC}"
if ! "$INSTALL_BIN_DIR/txm" version > /dev/null 2>&1; then
    error_exit "Installed binary failed to execute. Please check your system."
fi

echo -e "${GREEN}Binary verification successful!${NC}"

# Use the binary's native install for man page and completions
echo -e "${BLUE}Installing man page and shell completions...${NC}"
if [ "$SYSTEM_INSTALL" = true ]; then
    sudo "$INSTALL_BIN_DIR/txm" install --system || echo -e "${YELLOW}Warning: man page/completion install failed (binary already installed).${NC}"
else
    "$INSTALL_BIN_DIR/txm" install || echo -e "${YELLOW}Warning: man page/completion install failed (binary already installed).${NC}"
fi

echo ""
echo -e "${GREEN}✓ txm ${LATEST_VERSION} installed successfully!${NC}"
if [ "$SYSTEM_INSTALL" = false ]; then
    echo -e "${GREEN}  Binary:       $INSTALL_BIN_DIR/txm${NC}"
    echo -e "${YELLOW}  Ensure $USER_BIN_DIR is in your PATH.${NC}"
    echo -e "${YELLOW}  If not, run: source ~/.bashrc${NC}"
fi
echo -e "${GREEN}  Run 'txm --help' to get started, or 'man txm' for the manual.${NC}"