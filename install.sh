#!/bin/sh
set -e

REPO="exa-pub/norn"
INSTALL_DIR="/usr/local/bin"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)      echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  *)              echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Determine version
if [ -n "$NORN_VERSION" ]; then
  VERSION="$NORN_VERSION"
else
  VERSION=$(curl -fsS "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
  if [ -z "$VERSION" ]; then
    echo "Failed to fetch latest version. Set NORN_VERSION to install a specific version." >&2
    exit 1
  fi
fi

BINARY="norn-${OS}-${ARCH}"
ARCHIVE="${BINARY}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

echo "Installing norn ${VERSION} (${OS}/${ARCH})..."

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "$URL" -o "${TMP}/${ARCHIVE}"
tar -xzf "${TMP}/${ARCHIVE}" -C "$TMP"

install -m 755 "${TMP}/${BINARY}" "${INSTALL_DIR}/norn"

echo "norn installed to ${INSTALL_DIR}/norn"
