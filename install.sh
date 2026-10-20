#!/usr/bin/env sh
# Bound Universal Installer (macOS & Linux)
# Usage: curl -fsSL https://raw.githubusercontent.com/404kernel/bound/main/install.sh | sh

set -e

REPO="404kernel/bound"
BIN_DIR="/usr/local/bin"

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "❌ Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

case "$OS" in
  darwin) PLATFORM="darwin" ;;
  linux) PLATFORM="linux" ;;
  *)
    echo "❌ Unsupported operating system: $OS"
    exit 1
    ;;
esac

echo "=========================================================="
echo "  BOUND // Programmable Spend Vault for Autonomous AI Agents"
echo "=========================================================="
echo "Detecting latest release from GitHub..."

TAG=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$TAG" ]; then
  TAG="v0.2.1"
fi

BINARY_NAME="bound-${PLATFORM}-${ARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${BINARY_NAME}"

echo "Downloading Bound ${TAG} (${PLATFORM}/${ARCH})..."
TMP_DIR=$(mktemp -d)
curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/bound"
chmod +x "${TMP_DIR}/bound"

echo "Installing binary to ${BIN_DIR}/bound..."
if [ -w "$BIN_DIR" ]; then
  mv "${TMP_DIR}/bound" "${BIN_DIR}/bound"
else
  sudo mv "${TMP_DIR}/bound" "${BIN_DIR}/bound"
fi

rm -rf "$TMP_DIR"

echo "✅ Bound successfully installed to ${BIN_DIR}/bound"
echo ""
echo "Run 'bound help' or 'bound proxy' to start governing agent spend."
