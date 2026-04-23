#!/usr/bin/env bash
set -euo pipefail

REPO="ACM-Dev/gpu-finder"
VERSION="v1.0.0"
BIN_NAME="gpu-finder"
INSTALL_DIR="${HOME}/.local/bin"
BIN_PATH="${INSTALL_DIR}/${BIN_NAME}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux|darwin) ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

if command -v ${BIN_NAME} &>/dev/null; then
  echo "✅ ${BIN_NAME} already installed: $(which ${BIN_NAME})"
  echo ""
  read -p "Re-download ${VERSION}? [y/N]: " -n 1 -r
  echo
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    ${BIN_NAME} "$@"
    exit 0
  fi
fi

if [[ -f "./${BIN_NAME}" ]]; then
  echo "✅ Found local binary: ./gpu-finder"
  echo ""
  read -p "Use local? [Y/n]: " -n 1 -r
  echo
  if [[ ! $REPLY =~ ^[Nn]$ ]]; then
    chmod +x "./${BIN_NAME}"
    ./${BIN_NAME} "$@"
    exit 0
  fi
fi

FILENAME="${BIN_NAME}-${VERSION}-${OS}-${ARCH}.tar.gz"
echo "📦 Downloading ${FILENAME}..."
curl -fSL --progress-bar "${DOWNLOAD_URL}/${FILENAME}" -o "/tmp/${FILENAME}"

echo "📂 Extracting to ${INSTALL_DIR}..."
mkdir -p "${INSTALL_DIR}"
tar xzf "/tmp/${FILENAME}" -C "${INSTALL_DIR}"
chmod +x "${BIN_PATH}"
rm -f "/tmp/${FILENAME}"

echo ""
echo "✅ Installed to ${BIN_PATH}"
echo ""
${BIN_PATH} "$@"
