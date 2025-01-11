#!/bin/bash

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# GitHub repository information
REPO_OWNER="MohamedElashri"
REPO_NAME="txm-go"
LATEST_RELEASE_URL="https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/latest"

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

# Set up installation directories
USER_BIN_DIR="$HOME/.local/bin"
USER_MAN_DIR="$HOME/.local/share/man/man1"
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

# Check if curl is installed
if ! command -v curl &> /dev/null; then
    error_exit "curl is not installed. Please install curl and try again."
fi

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    error_exit "jq is not installed. Please install jq and try again."
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
    INSTALL_MAN_DIR="$SYS_MAN_DIR"
    echo -e "${BLUE}Performing system-wide installation...${NC}"
else
    INSTALL_BIN_DIR="$USER_BIN_DIR"
    INSTALL_MAN_DIR="$USER_MAN_DIR"
    echo -e "${BLUE}Performing user-local installation...${NC}"
fi

# Create necessary directories
create_dir_if_needed "$INSTALL_BIN_DIR"
create_dir_if_needed "$INSTALL_MAN_DIR"

# Add ~/.local/bin to PATH if it's not already there and doing user installation
if [ "$SYSTEM_INSTALL" = false ]; then
    if [[ ":$PATH:" != *":$USER_BIN_DIR:"* ]]; then
        echo -e "${YELLOW}Adding $USER_BIN_DIR to your PATH...${NC}"
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.bashrc"
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.zshrc" 2>/dev/null || true
        echo -e "${YELLOW}Please restart your shell or run 'source ~/.bashrc' to update your PATH.${NC}"
    fi
fi

# Check if txm is already installed
if command -v txm &> /dev/null; then
    CURRENT_VERSION=$(txm --version 2>/dev/null || echo "unknown")
    LATEST_VERSION=$(curl -s "$LATEST_RELEASE_URL" | jq -r ".tag_name")

    if [ "$CURRENT_VERSION" == "$LATEST_VERSION" ]; then
        echo -e "${YELLOW}txm is already installed and up to date (version $CURRENT_VERSION).${NC}"
        read -p "Do you want to re-install txm? (y/n): " REINSTALL
        if [ "$REINSTALL" != "y" ]; then
            echo -e "${BLUE}Installation aborted. Exiting.${NC}"
            exit 0
        fi
    else
        echo -e "${YELLOW}txm is already installed (version $CURRENT_VERSION), but a newer version ($LATEST_VERSION) is available.${NC}"
        read -p "Do you want to upgrade txm? (y/n): " UPGRADE
        if [ "$UPGRADE" != "y" ]; then
            echo -e "${BLUE}Upgrade aborted. Exiting.${NC}"
            exit 0
        fi
    fi
fi

# Fetch the latest release information
echo -e "${BLUE}Fetching the latest release information...${NC}"
RELEASE_INFO=$(curl -s "$LATEST_RELEASE_URL")

# Extract the download URL for the current platform
DOWNLOAD_URL=$(echo "$RELEASE_INFO" | jq -r ".assets[] | select(.name | contains(\"$OS.zip\")) | .browser_download_url")

if [ -z "$DOWNLOAD_URL" ]; then
    error_exit "No compatible binary found for $OS in the latest release."
fi

# Create temporary directory for downloads
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR" || error_exit "Failed to create temporary directory"

# Download the binary
echo -e "${BLUE}Downloading the txm binary...${NC}"
curl -LO "$DOWNLOAD_URL" || error_exit "Failed to download the txm binary."

# Extract the downloaded ZIP file
echo -e "${BLUE}Extracting the downloaded ZIP file...${NC}"
unzip -o "txm-$OS.zip" || error_exit "Failed to extract the downloaded ZIP file."

# Move the binary to installation directory
echo -e "${BLUE}Moving the txm binary to $INSTALL_BIN_DIR...${NC}"
if [ "$SYSTEM_INSTALL" = true ]; then
    sudo mv "txm-$OS-latest" "$INSTALL_BIN_DIR/txm" || error_exit "Failed to move the txm binary"
    sudo chmod 755 "$INSTALL_BIN_DIR/txm"
else
    mv "txm-$OS-latest" "$INSTALL_BIN_DIR/txm" || error_exit "Failed to move the txm binary"
    chmod 755 "$INSTALL_BIN_DIR/txm"
fi

# Download and install man page
echo -e "${BLUE}Downloading the txm man page...${NC}"
curl -LO "https://raw.githubusercontent.com/$REPO_OWNER/$REPO_NAME/main/txm.1" || error_exit "Failed to download the txm man page."

if [ "$SYSTEM_INSTALL" = true ]; then
    sudo mv "txm.1" "$INSTALL_MAN_DIR/txm.1" || error_exit "Failed to move the txm man page"
    sudo chmod 644 "$INSTALL_MAN_DIR/txm.1"
else
    mv "txm.1" "$INSTALL_MAN_DIR/txm.1" || error_exit "Failed to move the txm man page"
    chmod 644 "$INSTALL_MAN_DIR/txm.1"
fi

# Update the man page index
echo -e "${BLUE}Updating the man page index...${NC}"
if [ "$OS" == "macOS" ]; then
    if [ "$SYSTEM_INSTALL" = true ]; then
        sudo /usr/libexec/makewhatis "$INSTALL_MAN_DIR"
    else
        /usr/libexec/makewhatis "$INSTALL_MAN_DIR" 2>/dev/null || true
    fi
else
    if [ "$SYSTEM_INSTALL" = true ]; then
        sudo mandb
    else
        mandb --user-db "$HOME/.local/share/man" 2>/dev/null || true
    fi
fi

# Clean up
cd - >/dev/null
rm -rf "$TMP_DIR"

echo -e "${GREEN}Installation completed successfully!${NC}"
if [ "$SYSTEM_INSTALL" = false ]; then
    echo -e "${GREEN}txm has been installed to $INSTALL_BIN_DIR/txm${NC}"
    echo -e "${YELLOW}Make sure $USER_BIN_DIR is in your PATH.${NC}"
    echo -e "${YELLOW}If not, restart your shell or run: source ~/.bashrc${NC}"
fi
echo -e "${GREEN}You can now run 'txm' from the command line and access its man page with 'man txm'.${NC}"