#!/bin/sh
set -e

REPO="original-flipster69/koko"
INSTALL_DIR="${KOKO_INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="koko"

# Default mirror (Codeberg)
MIRROR="${KOKO_MIRROR:-codeberg}"

# Usage information
usage() {
  echo "Usage: $0 [OPTIONS]" >&2
  echo "" >&2
  echo "Options:" >&2
  echo "  --version <version>    Install a specific version (default: latest)" >&2
  echo "  --mirror <mirror>      Select mirror: github or codeberg (default: codeberg)" >&2
  echo "  --help                 Show this help message" >&2
  exit 1
}

# Parse command line arguments
while [ $# -gt 0 ]; do
  case "$1" in
    --version)
      KOKO_VERSION="$2"
      shift 2
      ;;
    --mirror)
      MIRROR="$2"
      shift 2
      ;;
    --help|-h)
      usage
      ;;
    *)
      echo "Error: Unknown option: $1" >&2
      usage
      ;;
  esac
done

require() {
  for cmd in "$@"; do
    command -v "$cmd" >/dev/null 2>&1 || {
      echo "Error: '$cmd' is required" >&2
      exit 1
    }
  done
}

require_one_of() {
  for cmd in "$@"; do
    if command -v "$cmd" >/dev/null 2>&1; then
      return 0
    fi
  done
  echo "Error: one of [$*] is required" >&2
  exit 1
}

detect_platform() {
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  ARCH=$(uname -m)

  case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    i386|i686) ARCH="386" ;;
    *)
      echo "Error: unsupported architecture: $ARCH" >&2
      exit 1
      ;;
  esac

  case "$OS" in
    darwin|linux|windows) ;;
    *)
      echo "Error: unsupported OS: $OS" >&2
      exit 1
      ;;
  esac
}

fetch_latest_tag() {
  if [ -n "${KOKO_VERSION:-}" ]; then
    TAG="$KOKO_VERSION"
    return
  fi

  # Determine the base URL based on the mirror
  if [ "$MIRROR" = "github" ]; then
    BASE_URL="https://github.com/${REPO}/releases/latest"
  else
    BASE_URL="https://codeberg.org/${REPO}/releases/latest"
  fi

  url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
    "$BASE_URL")
  TAG=${url##*/tag/}

  case "$TAG" in
    v*) ;;
    *)
      echo "Error: unexpected release URL: $url" >&2
      exit 1
      ;;
  esac
}

sha256() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    sha256sum "$1" | awk '{print $1}'
  fi
}

download_and_verify() {
  VERSION=${TAG#v}
  
  # Handle Windows binary naming
  if [ "$OS" = "windows" ]; then
    ASSET="${BINARY_NAME}_${VERSION}_${OS}_${ARCH}.zip"
  else
    ASSET="${BINARY_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"
  fi
  
  # Determine the base URL based on the mirror
  if [ "$MIRROR" = "github" ]; then
    ASSET_URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"
    CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${TAG}/checksums.txt"
  else
    ASSET_URL="https://codeberg.org/${REPO}/releases/download/${TAG}/${ASSET}"
    CHECKSUMS_URL="https://codeberg.org/${REPO}/releases/download/${TAG}/checksums.txt"
  fi

  TMPDIR=$(mktemp -d)
  trap 'rm -rf "$TMPDIR"' EXIT

  echo "Downloading ${ASSET} (${TAG}) from ${MIRROR}..."
  curl -fsSL -o "${TMPDIR}/${ASSET}" "$ASSET_URL"
  curl -fsSL -o "${TMPDIR}/checksums.txt" "$CHECKSUMS_URL"

  EXPECTED=$(awk -v a="${ASSET}" '$2 == a {print $1}' "${TMPDIR}/checksums.txt")
  if [ -z "$EXPECTED" ]; then
    echo "Error: ${ASSET} not listed in checksums.txt" >&2
    exit 1
  fi

  ACTUAL=$(sha256 "${TMPDIR}/${ASSET}")
  if [ "$EXPECTED" != "$ACTUAL" ]; then
    echo "Error: checksum mismatch for ${ASSET}" >&2
    echo "  expected: $EXPECTED" >&2
    echo "  actual:   $ACTUAL" >&2
    exit 1
  fi
  echo "Checksum verified."

  if [ "$OS" = "windows" ]; then
    unzip -o "${TMPDIR}/${ASSET}" -d "$TMPDIR"
  else
    tar -xzf "${TMPDIR}/${ASSET}" -C "$TMPDIR"
  fi
}

install_binary() {
  # Handle Windows binary naming
  if [ "$OS" = "windows" ]; then
    BINARY_FILE="${BINARY_NAME}.exe"
  else
    BINARY_FILE="${BINARY_NAME}"
  fi
  
  chmod +x "${TMPDIR}/${BINARY_FILE}"
  if [ -w "$INSTALL_DIR" ]; then
    mv "${TMPDIR}/${BINARY_FILE}" "${INSTALL_DIR}/${BINARY_FILE}"
  else
    echo "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mv "${TMPDIR}/${BINARY_FILE}" "${INSTALL_DIR}/${BINARY_FILE}"
  fi
  
  # Remove macOS quarantine flag to avoid Gatekeeper warnings
  if [ "$OS" = "darwin" ]; then
    xattr -d com.apple.quarantine "${INSTALL_DIR}/${BINARY_FILE}" 2>/dev/null || true
  fi
}

install_config() {
  CONFIG_DIR="${HOME}/.koko"
  CONFIG_FILE="${CONFIG_DIR}/config.toml"
  if [ -f "$CONFIG_FILE" ]; then
    echo "Keeping existing config at ${CONFIG_FILE}."
    return
  fi
  
  # Determine the base URL based on the mirror
  if [ "$MIRROR" = "github" ]; then
    BLUEPRINT_URL="https://raw.githubusercontent.com/${REPO}/${TAG}/docs/examples/config.toml"
  else
    BLUEPRINT_URL="https://codeberg.org/${REPO}/raw/tag/${TAG}/docs/examples/config.toml"
  fi
  
  mkdir -p "$CONFIG_DIR"
  if curl -fsSL -o "$CONFIG_FILE" "$BLUEPRINT_URL"; then
    echo "Wrote starter config to ${CONFIG_FILE}."
  else
    echo "Warning: could not fetch starter config (skipping)." >&2
  fi
}

require curl tar awk
require_one_of shasum sha256sum

detect_platform

# Windows requires unzip
if [ "$OS" = "windows" ]; then
  require unzip
fi
fetch_latest_tag
download_and_verify
install_binary
install_config

echo
if [ "$OS" = "windows" ]; then
  echo "Installed ${BINARY_NAME} ${TAG} to ${INSTALL_DIR}\${BINARY_NAME}.exe"
  echo "Run '${BINARY_NAME}.exe' to start a session."
else
  echo "Installed ${BINARY_NAME} ${TAG} to ${INSTALL_DIR}/${BINARY_NAME}"
  echo "Run '${BINARY_NAME}' to start a session."
fi
