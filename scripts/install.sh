#!/usr/bin/env bash
set -euo pipefail

REPO="ACM-Dev/gpu-finder"
VERSION="v1.0.0"
BIN_NAME="gpu-finder"
INSTALL_DIR="${HOME}/.local/bin"
BIN_PATH="${INSTALL_DIR}/${BIN_NAME}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}"

# Uninstall mode
if [[ "${1:-}" == "--uninstall" ]]; then
  if [[ ! -f "${BIN_PATH}" ]]; then
    echo "❌ gpu-finder not found at ${BIN_PATH}"
    exit 1
  fi

  echo "🗑️  Uninstalling gpu-finder..."
  rm -f "${BIN_PATH}"
  echo "✅ Removed ${BIN_PATH}"
  echo ""
  echo "🔄 To complete removal:"
  echo "   • Remove from PATH if added: edit ~/.bashrc, ~/.zshrc, or ~/.profile"
  echo "   • Remove this line if present: export PATH=\"${INSTALL_DIR}:\$PATH\""
  echo "   • Or run:  sed -i '/gpu-finder/d' ~/.bashrc ~/.zshrc ~/.profile 2>/dev/null || true"
  exit 0
fi

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
ARCHIVE_BIN="${BIN_NAME}-${OS}-${ARCH}"
echo "📦 Downloading ${FILENAME}..."
curl -fSL --progress-bar "${DOWNLOAD_URL}/${FILENAME}" -o "/tmp/${FILENAME}"

echo "📂 Extracting..."
mkdir -p "${INSTALL_DIR}"
tar xzf "/tmp/${FILENAME}" -C "/tmp"

echo "🔧 Renaming to ${BIN_NAME}..."
mv "/tmp/${ARCHIVE_BIN}" "${BIN_PATH}"
chmod +x "${BIN_PATH}"
rm -f "/tmp/${FILENAME}"

if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
  echo "🛤️  Adding ${INSTALL_DIR} to PATH for this session..."
  export PATH="${INSTALL_DIR}:${PATH}"
fi

echo ""
echo "✅ Installed to ${BIN_PATH}"
echo ""
echo "🔄 To use gpu-finder:"
echo "   • Restart your shell, or run:  export PATH=\"${INSTALL_DIR}:\$PATH\""
echo "   • Then run:                    gpu-finder"
echo "   • To uninstall:                bash <(curl -fsSL https://github.com/ACM-Dev/gpu-finder/raw/main/scripts/install.sh) --uninstall"
echo ""
${BIN_PATH} "$@"
