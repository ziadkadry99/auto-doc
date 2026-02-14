#!/bin/sh
# Auto-doc installer script
# Usage: curl -fsSL https://raw.githubusercontent.com/ziadkadry99/auto-doc/main/scripts/install.sh | sh
set -e

REPO="ziadkadry99/auto-doc"
BINARY="autodoc"

log() { printf '%s\n' "$@"; }
err() { log "ERROR: $*" >&2; exit 1; }

# --- Detect download tool ---
DOWNLOAD=""
if command -v curl >/dev/null 2>&1; then
    DOWNLOAD="curl -fsSL"
elif command -v wget >/dev/null 2>&1; then
    DOWNLOAD="wget -qO-"
else
    err "curl or wget is required but neither was found"
fi

# --- Detect OS ---
OS="$(uname -s)"
case "$OS" in
    Linux*)  OS=linux  ;;
    Darwin*) OS=darwin ;;
    *)       err "Unsupported operating system: $OS (only linux and macOS are supported)" ;;
esac

# --- Detect architecture ---
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64)  ARCH=amd64 ;;
    aarch64|arm64)  ARCH=arm64 ;;
    *)              err "Unsupported architecture: $ARCH (only amd64 and arm64 are supported)" ;;
esac

log "Detected platform: ${OS}/${ARCH}"

# --- Fetch latest release tag ---
log "Fetching latest release..."
RELEASE_URL="https://api.github.com/repos/${REPO}/releases/latest"
RELEASE_JSON="$($DOWNLOAD "$RELEASE_URL")" || err "Failed to fetch release info from GitHub"

TAG="$(printf '%s' "$RELEASE_JSON" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')"
if [ -z "$TAG" ]; then
    err "Could not determine latest release tag. Are there any releases published?"
fi

VERSION="${TAG#v}"
log "Latest version: ${VERSION}"

# --- Build download URL ---
ARCHIVE="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"

# --- Download and extract ---
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

log "Downloading ${DOWNLOAD_URL}..."
$DOWNLOAD "$DOWNLOAD_URL" > "${TMPDIR}/${ARCHIVE}" || err "Download failed. Check that the release exists for ${OS}/${ARCH}."

log "Extracting..."
tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR" || err "Failed to extract archive"

if [ ! -f "${TMPDIR}/${BINARY}" ]; then
    err "Binary '${BINARY}' not found in archive"
fi

chmod +x "${TMPDIR}/${BINARY}"

# --- Install binary ---
INSTALL_DIR="/usr/local/bin"
if [ "$(id -u)" -ne 0 ]; then
    INSTALL_DIR="${HOME}/.local/bin"
    mkdir -p "$INSTALL_DIR"
fi

cp "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}" || err "Failed to install binary to ${INSTALL_DIR}"

log ""
log "autodoc ${VERSION} installed to ${INSTALL_DIR}/${BINARY}"

# Check if install dir is in PATH
case ":$PATH:" in
    *":${INSTALL_DIR}:"*) ;;
    *) log "NOTE: ${INSTALL_DIR} is not in your PATH. Add it with:"
       log "  export PATH=\"${INSTALL_DIR}:\$PATH\"" ;;
esac
