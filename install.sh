#!/bin/bash
set -e

REPO="upWatchly/metrics-agent"
INSTALL_DIR="/usr/local/bin"
SERVICE_NAME="upwatchly-agent"
BINARY_NAME="upwatchly-agent"

# --- Check root ---
if [ "$(id -u)" -ne 0 ]; then
  echo "Error: run as root (sudo bash install.sh)" >&2
  exit 1
fi

# --- Check API key (not required for updates) ---
API_KEY="$1"
IS_UPDATE=false
if [ -f /etc/systemd/system/${SERVICE_NAME}.service ]; then
  IS_UPDATE=true
fi

if [ -z "$API_KEY" ] && [ "$IS_UPDATE" = false ]; then
  echo "Usage: sudo bash install.sh <API_KEY>" >&2
  exit 1
fi

if [ -n "$API_KEY" ] && [ ${#API_KEY} -lt 8 ]; then
  echo "Error: API key seems too short (min 8 characters)" >&2
  exit 1
fi

# --- Detect arch ---
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)
    echo "Error: unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

# --- Get latest release URL ---
echo "Detecting latest release..."
DOWNLOAD_URL=$(curl -sS "https://api.github.com/repos/${REPO}/releases/tags/latest" \
  | grep "browser_download_url.*linux-${ARCH}" \
  | cut -d '"' -f 4)

# Fallback to latest stable release
if [ -z "$DOWNLOAD_URL" ]; then
  DOWNLOAD_URL=$(curl -sS "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep "browser_download_url.*linux-${ARCH}" \
    | cut -d '"' -f 4)
fi

if [ -z "$DOWNLOAD_URL" ]; then
  echo "Error: could not find binary for linux-${ARCH}" >&2
  exit 1
fi

# --- Stop service before replacing binary ---
if [ "$IS_UPDATE" = true ]; then
  echo "Stopping agent..."
  systemctl stop ${SERVICE_NAME}
fi

# --- Download and install binary ---
echo "Downloading ${BINARY_NAME} for linux/${ARCH}..."
rm -f "${INSTALL_DIR}/${BINARY_NAME}"
curl -sSL "$DOWNLOAD_URL" -o "${INSTALL_DIR}/${BINARY_NAME}"

# Verify download succeeded
if [ ! -s "${INSTALL_DIR}/${BINARY_NAME}" ]; then
  echo "Error: downloaded binary is empty or missing" >&2
  exit 1
fi

chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
echo "Installed to ${INSTALL_DIR}/${BINARY_NAME}"

# --- Create or update systemd service ---
if [ "$IS_UPDATE" = true ] && [ -z "$API_KEY" ]; then
  echo "Starting agent..."
  systemctl start ${SERVICE_NAME}
else
  cat > /etc/systemd/system/${SERVICE_NAME}.service <<EOF
[Unit]
Description=Upwatchly Metrics Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${BINARY_NAME}
Environment=UW_API_KEY=${API_KEY}
Environment=UW_LOG_LEVEL=info
Environment=UW_DISABLE_KEEP_ALIVE=false
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable --now ${SERVICE_NAME}
fi

echo ""
echo "Upwatchly agent installed and running."
echo "  Status:  systemctl status ${SERVICE_NAME}"
echo "  Logs:    journalctl -u ${SERVICE_NAME} -f"
echo "  Config:  /etc/systemd/system/${SERVICE_NAME}.service"
echo "  Stop:    systemctl stop ${SERVICE_NAME}"
echo "  Remove:  systemctl stop ${SERVICE_NAME} && systemctl disable ${SERVICE_NAME} && rm ${INSTALL_DIR}/${BINARY_NAME} /etc/systemd/system/${SERVICE_NAME}.service"
echo ""
echo "To change settings, edit the service file and restart:"
echo "  sudo nano /etc/systemd/system/${SERVICE_NAME}.service"
echo "  sudo systemctl daemon-reload && sudo systemctl restart ${SERVICE_NAME}"
