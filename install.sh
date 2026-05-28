#!/bin/sh
set -e

REPO=premex-ab/memoria-cli
DEST="${HOME}/.local/bin/memoria"

# Clean up the temp file on any exit (success or failure).
trap 'rm -f "${DEST}.tmp"' EXIT

LATEST="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | head -1 | cut -d'"' -f4)"

if [ -z "$LATEST" ]; then
  echo "Could not resolve latest release tag from GitHub API." >&2
  exit 1
fi

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo "Unsupported arch: $ARCH" >&2; exit 1 ;;
esac

URL="https://github.com/${REPO}/releases/download/${LATEST}/memoria-${OS}-${ARCH}"

mkdir -p "$(dirname "$DEST")"
curl -fsSL "$URL" -o "${DEST}.tmp"
mv "${DEST}.tmp" "$DEST"
chmod +x "$DEST"

echo "Installed memoria ${LATEST} to ${DEST}"
echo
echo "Next steps:"
echo "  1. Make sure ${HOME}/.local/bin is in your PATH."
echo "  2. memoria init <your-api-key>"
echo "     Mint a key at https://memoria.premex.se/dashboard."
