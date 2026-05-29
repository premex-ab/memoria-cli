#!/bin/sh
set -e

REPO=premex-ab/memoria-cli
DEST="${HOME}/.local/bin/memoria"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo "Unsupported arch: $ARCH" >&2; exit 1 ;;
esac
ASSET="memoria-${OS}-${ARCH}"

# Resolve the release to install.
#
# We deliberately avoid the api.github.com JSON API here: unauthenticated
# requests to it are capped at 60/hr per IP and return HTTP 403 once the budget
# is spent. That budget is shared across everyone behind the same egress IP, so
# CI runners and cloud/sandbox environments hit it constantly. github.com's own
# release redirects carry no such limit, so we resolve through those instead.
#
# Set MEMORIA_VERSION (e.g. cli/v0.1.0) to pin a version and skip the lookup.
if [ -n "${MEMORIA_VERSION:-}" ]; then
  VERSION="${MEMORIA_VERSION}"
else
  # /releases/latest 302-redirects to /releases/tag/<tag>; read the tag from
  # the Location header without following it (and without the rate limit).
  VERSION="$(curl -fsS -o /dev/null -w '%{redirect_url}' \
    "https://github.com/${REPO}/releases/latest" 2>/dev/null \
    | sed -n 's#.*/releases/tag/##p')" || true
fi

if [ -n "${VERSION}" ]; then
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
else
  # Couldn't read the tag (offline header parse failed) — fall back to the
  # latest-asset redirect, which still resolves without the API.
  VERSION="latest"
  URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
fi

mkdir -p "$(dirname "$DEST")"
if ! curl -fsSL "$URL" -o "$DEST"; then
  echo "Failed to download ${ASSET} from:" >&2
  echo "  ${URL}" >&2
  echo "Pin a known version and retry, e.g.:" >&2
  echo "  curl -fsSL https://api.memoria.premex.se/install.sh | MEMORIA_VERSION=cli/v0.1.0 sh" >&2
  exit 1
fi
chmod +x "$DEST"

echo "Installed memoria ${VERSION} to ${DEST}"
echo
echo "Next steps:"
echo "  1. Make sure ${HOME}/.local/bin is in your PATH."
echo "  2. memoria init <your-api-key>"
echo "     Mint a key at https://memoria.premex.se/dashboard."
