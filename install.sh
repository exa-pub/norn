#!/bin/sh
set -e

REPO="exa-pub/norn"
BINARY_NAME="norn"

# --- Colors (if terminal supports them) ---
if [ -t 1 ]; then
  BOLD='\033[1m'
  RED='\033[0;31m'
  GREEN='\033[0;32m'
  YELLOW='\033[0;33m'
  RESET='\033[0m'
else
  BOLD='' RED='' GREEN='' YELLOW='' RESET=''
fi

info()  { printf "${BOLD}${GREEN}==> %s${RESET}\n" "$*"; }
warn()  { printf "${BOLD}${YELLOW}==> %s${RESET}\n" "$*"; }
error() { printf "${BOLD}${RED}Error: %s${RESET}\n" "$*" >&2; exit 1; }

# --- Downloader detection ---
DOWNLOADER=""
if command -v curl >/dev/null 2>&1; then
  DOWNLOADER="curl"
elif command -v wget >/dev/null 2>&1; then
  DOWNLOADER="wget"
else
  error "curl or wget is required but neither is installed"
fi

download() {
  local url="$1" output="$2"
  if [ "$DOWNLOADER" = "curl" ]; then
    if [ -n "$output" ]; then
      curl -fsSL -o "$output" "$url"
    else
      curl -fsSL "$url"
    fi
  else
    if [ -n "$output" ]; then
      wget -q -O "$output" "$url"
    else
      wget -q -O - "$url"
    fi
  fi
}

# --- Platform detection ---
detect_os() {
  case "$(uname -s)" in
    Linux)  echo "linux" ;;
    *)      error "Unsupported OS: $(uname -s). Only Linux is supported." ;;
  esac
}

OS="$(detect_os)"

detect_arch() {
  local arch
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)   arch="amd64" ;;
    aarch64|arm64)   arch="arm64" ;;
    *)               error "Unsupported architecture: $arch" ;;
  esac

  echo "$arch"
}

ARCH="$(detect_arch)"

# --- Install directory ---
# Strategy:
#   1. Use NORN_INSTALL_DIR if set
#   2. Try /usr/local/bin (needs root or writable)
#   3. Fall back to ~/.local/bin (no root needed)
determine_install_dir() {
  if [ -n "$NORN_INSTALL_DIR" ]; then
    echo "$NORN_INSTALL_DIR"
    return
  fi

  # If running as root or /usr/local/bin is writable, use it
  if [ "$(id -u)" = "0" ] || [ -w "/usr/local/bin" ]; then
    echo "/usr/local/bin"
    return
  fi

  # Try to use sudo if available
  if command -v sudo >/dev/null 2>&1; then
    # Test if sudo is available without a password (cached credentials or NOPASSWD)
    if sudo -n true 2>/dev/null; then
      echo "/usr/local/bin"
      return
    fi

    # Ask the user
    printf "${BOLD}${YELLOW}==> /usr/local/bin is not writable.${RESET}\n"
    printf "    [1] Use sudo to install to /usr/local/bin (may prompt for password)\n"
    printf "    [2] Install to ~/.local/bin (no root required)\n"
    printf "    Choice [1]: "
    read -r choice </dev/tty
    case "$choice" in
      2)
        echo "$HOME/.local/bin"
        return
        ;;
      *)
        echo "/usr/local/bin"
        return
        ;;
    esac
  fi

  # No sudo available, fall back
  echo "$HOME/.local/bin"
}

INSTALL_DIR="$(determine_install_dir)"

# --- Determine if we need sudo ---
# Check writability of the target dir, or its closest existing parent.
# This avoids false positives for paths like ~/.local/bin that don't exist yet
# but are owned by the user.
needs_sudo() {
  if [ "$(id -u)" = "0" ]; then
    return 1  # already root
  fi

  local dir="$INSTALL_DIR"
  # Walk up to the nearest existing directory
  while [ ! -d "$dir" ]; do
    dir="$(dirname "$dir")"
  done

  if [ -w "$dir" ]; then
    return 1  # writable without sudo
  fi
  return 0    # needs sudo
}

run_cmd() {
  if needs_sudo; then
    sudo "$@"
  else
    "$@"
  fi
}

# --- Version resolution ---
if [ -n "$NORN_VERSION" ]; then
  VERSION="$NORN_VERSION"
else
  VERSION=$(download "https://api.github.com/repos/${REPO}/releases/latest" "" \
    | grep '"tag_name"' | cut -d'"' -f4) || true
  if [ -z "$VERSION" ]; then
    error "Failed to fetch latest version. Set NORN_VERSION to install a specific version."
  fi
fi

# --- Download & verify ---
ASSET="norn-${OS}-${ARCH}"
ARCHIVE="${ASSET}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

info "Installing ${BINARY_NAME} ${VERSION} (${OS}/${ARCH})..."
info "From: ${URL}"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

download "$URL" "${TMP}/${ARCHIVE}"

# Checksum verification (optional: only if checksums.txt is published)
if download "$CHECKSUMS_URL" "${TMP}/checksums.txt" 2>/dev/null; then
  expected=$(grep "${ARCHIVE}" "${TMP}/checksums.txt" | awk '{print $1}')
  if [ -n "$expected" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
      actual=$(sha256sum "${TMP}/${ARCHIVE}" | awk '{print $1}')
    else
      warn "sha256sum not found, skipping checksum verification"
      actual=""
    fi

    if [ -n "$actual" ]; then
      if [ "$actual" != "$expected" ]; then
        error "Checksum verification failed! Expected: ${expected}, Got: ${actual}"
      fi
      info "Checksum verified"
    fi
  fi
else
  warn "checksums.txt not found, skipping verification"
fi

# --- Extract & install ---
tar -xzf "${TMP}/${ARCHIVE}" -C "$TMP"

# Ensure install directory exists
run_cmd mkdir -p "$INSTALL_DIR"
run_cmd install -m 755 "${TMP}/${ASSET}" "${INSTALL_DIR}/${BINARY_NAME}"

info "${BINARY_NAME} installed to ${INSTALL_DIR}/${BINARY_NAME}"

# --- PATH hint ---
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    warn "${INSTALL_DIR} is not in your PATH."
    echo "    Add it by running:"
    echo ""
    echo "      export PATH=\"${INSTALL_DIR}:\$PATH\""
    echo ""
    echo "    To make it permanent, add the line above to your ~/.bashrc or ~/.zshrc"
    ;;
esac
