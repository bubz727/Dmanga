#!/usr/bin/env bash
# Dmanga Linux Installer Script
# Usage: curl -fsSL https://raw.githubusercontent.com/bubz727/Dmanga/main/install.sh | bash

set -e

echo "=== Dmanga Installer (Linux) ==="

# 1. Architecture Detection
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)
    TARGET_ARCH="amd64"
    ;;
  aarch64|arm64)
    TARGET_ARCH="arm64"
    ;;
  *)
    echo "Error: Unsupported architecture $ARCH"
    exit 1
    ;;
esac

BINARY_NAME="Dmanga_linux_${TARGET_ARCH}"
echo "[*] Detected architecture: ${TARGET_ARCH} (${BINARY_NAME})"

# 2. Fetch Latest Release Info
API_URL="https://api.github.com/repos/bubz727/Dmanga/releases/latest"
echo "[*] Fetching latest release information..."
RELEASE_JSON=$(curl -fsSL -H "User-Agent: Dmanga-Installer" "$API_URL" || true)

if [ -z "$RELEASE_JSON" ]; then
  echo "Error: Failed to fetch latest release from GitHub."
  exit 1
fi

TAG=$(echo "$RELEASE_JSON" | grep -m 1 '"tag_name":' | sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/')
echo "[+] Latest version: $TAG"

DOWNLOAD_URL=$(echo "$RELEASE_JSON" | grep -E "browser_download_url.*${BINARY_NAME}\"" | head -n 1 | sed -E 's/.*"browser_download_url":\s*"([^"]+)".*/\1/')
CHECKSUM_URL=$(echo "$RELEASE_JSON" | grep -E "browser_download_url.*SHA256SUMS\.txt\"" | head -n 1 | sed -E 's/.*"browser_download_url":\s*"([^"]+)".*/\1/')

if [ -z "$DOWNLOAD_URL" ]; then
  echo "Error: Asset '$BINARY_NAME' not found in release $TAG."
  exit 1
fi

# 3. Installation Directory
if [ "$(id -u)" -eq 0 ]; then
  INSTALL_DIR="/usr/local/bin"
else
  INSTALL_DIR="${HOME}/.local/bin"
fi

mkdir -p "$INSTALL_DIR"
DEST_PATH="${INSTALL_DIR}/Dmanga"
TEMP_PATH="${INSTALL_DIR}/Dmanga.tmp"

# 4. Download Binary
echo "[*] Downloading ${BINARY_NAME}..."
curl -fsSL -o "$TEMP_PATH" "$DOWNLOAD_URL"

# 5. Checksum Verification
if [ -n "$CHECKSUM_URL" ]; then
  echo "[*] Verifying SHA256 checksum..."
  CHECKSUMS=$(curl -fsSL "$CHECKSUM_URL" || true)
  EXPECTED_HASH=$(echo "$CHECKSUMS" | grep "${BINARY_NAME}" | awk '{print $1}')
  if [ -n "$EXPECTED_HASH" ]; then
    ACTUAL_HASH=$(sha256sum "$TEMP_PATH" | awk '{print $1}')
    if [ "$ACTUAL_HASH" != "$EXPECTED_HASH" ]; then
      rm -f "$TEMP_PATH"
      echo "Error: Checksum mismatch! Expected $EXPECTED_HASH, got $ACTUAL_HASH"
      exit 1
    fi
    echo "[OK] Checksum verified successfully."
  fi
fi

# 6. Install Binary
chmod +x "$TEMP_PATH"
mv -f "$TEMP_PATH" "$DEST_PATH"

# 7. Check PATH
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo "[!] Notice: '$INSTALL_DIR' is not in your current PATH."
    SHELL_PROFILE=""
    if [ -n "$BASH_VERSION" ]; then
      SHELL_PROFILE="${HOME}/.bashrc"
    elif [ -n "$ZSH_VERSION" ]; then
      SHELL_PROFILE="${HOME}/.zshrc"
    fi
    if [ -n "$SHELL_PROFILE" ] && [ -f "$SHELL_PROFILE" ]; then
      echo "    Adding '$INSTALL_DIR' to $SHELL_PROFILE"
      echo "export PATH=\"\$PATH:$INSTALL_DIR\"" >> "$SHELL_PROFILE"
      echo "    Run: source $SHELL_PROFILE"
    fi
    ;;
esac

echo ""
echo "[OK] Dmanga $TAG was successfully installed to: $DEST_PATH"
echo "You can now run 'Dmanga' in your terminal!"
