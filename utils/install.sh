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

# Function to remove txm and its man page
remove_txm() {
  echo -e "${BLUE}Removing txm binary...${NC}"
  sudo rm -f /usr/local/bin/txm

  echo -e "${BLUE}Removing txm man page...${NC}"
  sudo rm -f "$MAN_DIR/txm.1"

  echo -e "${GREEN}txm and its man page have been removed successfully.${NC}"
  exit 0
}

# Check if the -rm flag is passed
if [ "$1" == "-rm" ]; then
  remove_txm
fi

# Check if curl is installed
if ! command -v curl &> /dev/null; then
  error_exit "curl is not installed. Please install curl and try again."
fi

# Check if jq is installed
if ! command -v jq &> /dev/null; then
  error_exit "jq is not installed. Please install jq and try again."
fi

# Check if txm is already installed
if command -v txm &> /dev/null; then
  CURRENT_VERSION=$(txm --version)
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

# Download the binary
echo -e "${BLUE}Downloading the txm binary...${NC}"
curl -LO "$DOWNLOAD_URL" || error_exit "Failed to download the txm binary."

# Extract the downloaded ZIP file
echo -e "${BLUE}Extracting the downloaded ZIP file...${NC}"
unzip -o "txm-$OS.zip" || error_exit "Failed to extract the downloaded ZIP file."

# Move the binary to /usr/local/bin
echo -e "${BLUE}Moving the txm binary to /usr/local/bin...${NC}"
sudo mv "txm-$OS-latest" /usr/local/bin/txm || error_exit "Failed to move the txm binary to /usr/local/bin."

# Clean up the downloaded ZIP file
echo -e "${BLUE}Cleaning up the downloaded ZIP file...${NC}"
rm "txm-$OS-latest.zip"

# Download the man page
echo -e "${BLUE}Downloading the txm man page...${NC}"
curl -LO "https://raw.githubusercontent.com/$REPO_OWNER/$REPO_NAME/main/txm.1" || error_exit "Failed to download the txm man page."

# Create the man directory if it doesn't exist
sudo mkdir -p "$MAN_DIR"

# Move the man page to the appropriate directory
echo -e "${BLUE}Moving the txm man page to $MAN_DIR...${NC}"
sudo mv "txm.1" "$MAN_DIR/txm.1" || error_exit "Failed to move the txm man page to $MAN_DIR."

# Update the man page index
echo -e "${BLUE}Updating the man page index...${NC}"
if [ "$OS" == "macOS" ]; then
  sudo /usr/libexec/makewhatis "$MAN_DIR"
else
  sudo mandb
fi

echo -e "${GREEN}Installation completed successfully!${NC}"
echo -e "${GREEN}You can now run 'txm' from the command line and access its man page with 'man txm'.${NC}"
