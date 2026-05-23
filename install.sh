#!/bin/sh
# install.sh — Install ttag on macOS / Linux
# Usage: curl -fsSL https://raw.githubusercontent.com/dpkay-io/ttag/main/install.sh | sh

set -e

REPO="dpkay-io/ttag"
INSTALL_DIR="$HOME/.terminal_tagger/bin"
BINARY_NAME="ttag"

echo ""
echo "  ttag installer"
echo "  ─────────────────────────────"
echo ""

# ── Create install directory ──────────────────────────────────────────────────
mkdir -p "$INSTALL_DIR"

# ── Detect OS ─────────────────────────────────────────────────────────────────
OS_RAW=$(uname -s)
case "$OS_RAW" in
    Darwin) OS="darwin" ;;
    Linux)  OS="linux"  ;;
    *)
        echo "  Error: Unsupported OS: $OS_RAW"
        exit 1
        ;;
esac

# ── Detect architecture ──────────────────────────────────────────────────────
ARCH_RAW=$(uname -m)
case "$ARCH_RAW" in
    x86_64)
        if [ "$OS" = "darwin" ]; then
            echo "  Error: Mac Intel (darwin/amd64) is no longer supported."
            exit 1
        fi
        ARCH="amd64"
        ;;
    aarch64|arm64)  ARCH="arm64" ;;
    *)
        echo "  Error: Unsupported architecture: $ARCH_RAW"
        exit 1
        ;;
esac

echo "  Platform: ${OS}/${ARCH}"

# ── Download binary ───────────────────────────────────────────────────────────
ASSET_NAME="ttag-${OS}-${ARCH}"
DOWNLOAD_URL="https://github.com/$REPO/releases/latest/download/$ASSET_NAME"
TARGET_PATH="$INSTALL_DIR/$BINARY_NAME"

echo "  Downloading $ASSET_NAME..."
if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$DOWNLOAD_URL" -o "$TARGET_PATH"
elif command -v wget >/dev/null 2>&1; then
    wget -qO "$TARGET_PATH" "$DOWNLOAD_URL"
else
    echo "  Error: Neither curl nor wget found. Please install one and retry."
    exit 1
fi

chmod +x "$TARGET_PATH"
echo "  Saved to $TARGET_PATH"

# ── Add to PATH in shell profile ─────────────────────────────────────────────
add_to_path() {
    local profile_file="$1"
    if [ -f "$profile_file" ] && grep -q "terminal_tagger" "$profile_file" 2>/dev/null; then
        echo "  PATH already configured in $(basename "$profile_file")"
        return
    fi
    {
        echo ""
        echo "# ttag — terminal tagger"
        echo "export PATH=\"\$PATH:$INSTALL_DIR\""
    } >> "$profile_file"
    echo "  Added to PATH in $(basename "$profile_file")"
}

SHELL_NAME=$(basename "${SHELL:-/bin/sh}")
case "$SHELL_NAME" in
    zsh)  add_to_path "$HOME/.zshrc"  ;;
    bash) add_to_path "$HOME/.bashrc" ;;
    *)    add_to_path "$HOME/.profile" ;;
esac

# Make ttag available in the current session
export PATH="$PATH:$INSTALL_DIR"

# ── Install shell hook ───────────────────────────────────────────────────────
echo "  Installing shell hook..."
"$TARGET_PATH" install

echo ""
echo "  ✓ ttag installed successfully!"
echo "  Restart your terminal to activate."
echo ""
