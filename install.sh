#!/bin/sh
set -e

REPO="original-flipster69/koko"
INSTALL_DIR="${KOKO_INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="koko"

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
    *)
      echo "Error: unsupported architecture: $ARCH" >&2
      exit 1
      ;;
  esac

  case "$OS" in
    darwin|linux) ;;
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

  url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
    "https://github.com/${REPO}/releases/latest")
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
  ASSET="${BINARY_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"
  ASSET_URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"
  CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${TAG}/checksums.txt"

  TMPDIR=$(mktemp -d)
  trap 'rm -rf "$TMPDIR"' EXIT

  echo "Downloading ${ASSET} (${TAG})..."
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

  tar -xzf "${TMPDIR}/${ASSET}" -C "$TMPDIR"
}

install_binary() {
  chmod +x "${TMPDIR}/${BINARY_NAME}"
  if [ -w "$INSTALL_DIR" ]; then
    mv "${TMPDIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
  else
    echo "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mv "${TMPDIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
  fi
}

install_config() {
  CONFIG_DIR="${HOME}/.koko"
  CONFIG_FILE="${CONFIG_DIR}/config.toml"
  if [ -f "$CONFIG_FILE" ]; then
    echo "Keeping existing config at ${CONFIG_FILE}."
    return
  fi
  BLUEPRINT_URL="https://raw.githubusercontent.com/${REPO}/${TAG}/config.example.toml"
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
fetch_latest_tag
download_and_verify
install_binary
install_config

echo
echo "Installed ${BINARY_NAME} ${TAG} to ${INSTALL_DIR}/${BINARY_NAME}"
echo "Run '${BINARY_NAME}' to start a session."
