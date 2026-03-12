#!/bin/bash
set -e

REPO="shachiku-ai/shachiku"
BINARY_DIR="/usr/local/bin"
SERVICE_NAME="shachiku"

# Function to check command existence
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Ensure running with root privileges for installation
if [ "$EUID" -ne 0 ]; then
    echo "Error: Please run this script as root or with sudo."
    exit 1
fi

echo "================================================="
echo "   Installing $SERVICE_NAME as a system service  "
echo "================================================="

# Determine OS and Architecture
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
    Linux)
        OS_SUFFIX="linux"
        ;;
    Darwin)
        OS_SUFFIX="darwin"
        ;;
    *)
        echo "Unsupported OS: $OS"
        exit 1
        ;;
esac

case "$ARCH" in
    x86_64|amd64)
        ARCH_SUFFIX="amd64"
        ;;
    aarch64|arm64)
        ARCH_SUFFIX="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

BINARY_NAME="shachiku-$OS_SUFFIX-$ARCH_SUFFIX"

echo "Detected Platform: $OS_SUFFIX ($ARCH_SUFFIX)"
echo "Target Binary: $BINARY_NAME"

# Check dependencies
if ! command_exists curl && ! command_exists wget; then
    echo "Error: curl or wget is required to download the binary."
    exit 1
fi

# Get the latest release tag
echo "Fetching latest release information..."
if command_exists curl; then
    LATEST_RELEASE=$(curl -s "https://api.github.com/repos/$REPO/releases/latest")
else
    LATEST_RELEASE=$(wget -qO- "https://api.github.com/repos/$REPO/releases/latest")
fi

TAG=$(echo "$LATEST_RELEASE" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$TAG" ]; then
    echo "Error: Could not retrieve the latest release tag. Rate limit maybe?"
    echo "$LATEST_RELEASE"
    exit 1
fi

echo "Latest release tag: $TAG"

DOWNLOAD_URL="https://github.com/$REPO/releases/download/$TAG/$BINARY_NAME"
INSTALL_PATH="$BINARY_DIR/$SERVICE_NAME"

echo "Downloading from $DOWNLOAD_URL ..."

# Download the file to a temporary location first
TMP_DOWNLOAD="/tmp/$BINARY_NAME"

if command_exists curl; then
    curl -L -f -o "$TMP_DOWNLOAD" "$DOWNLOAD_URL" || { echo "Download failed"; exit 1; }
else
    wget -q --show-progress -O "$TMP_DOWNLOAD" "$DOWNLOAD_URL" || { echo "Download failed"; exit 1; }
fi

if [ ! -f "$TMP_DOWNLOAD" ]; then
    echo "Error: Downloaded file not found."
    exit 1
fi

echo "Installing executable to $INSTALL_PATH..."
mv "$TMP_DOWNLOAD" "$INSTALL_PATH"
chmod +x "$INSTALL_PATH"

echo "================================================="
echo "               Setting up service                "
echo "================================================="

if [ "$OS_SUFFIX" = "linux" ]; then
    echo "================================================="
    echo "       Installing Playwright Dependencies        "
    echo "================================================="
    if ! command_exists npm; then
        echo "npm not found. Installing Node.js..."
        if command_exists apt-get; then
            apt-get update
            apt-get install -y nodejs npm
        elif command_exists yum; then
            yum install -y nodejs npm
        elif command_exists dnf; then
            dnf install -y nodejs npm
        else
            echo "Warning: Could not install npm automatically. Please install Node.js manually."
        fi
    fi

    if command_exists npx; then
        echo "Running npx playwright install --with-deps chromium..."
        npx -y playwright@latest install --with-deps chromium
    else
        echo "Notice: 'npx' not found. Playwright might fail to run if system dependencies are missing."
        echo "Please install Node.js and run 'npx playwright install --with-deps chromium' manually."
    fi

    # systemd handling
    if command_exists systemctl; then
        echo "Installing systemd service for Linux..."
        SERVICE_FILE="/etc/systemd/system/$SERVICE_NAME.service"
        cat <<EOF > "$SERVICE_FILE"
[Unit]
Description=Shachiku System Service
After=network.target

[Service]
Type=simple
ExecStart=$INSTALL_PATH
Restart=on-failure
RestartSec=5
User=root
# If you want to run as a specific user, change 'root' to that username
# Environment="PORT=8080"
Environment="IS_PUBLIC=true"

[Install]
WantedBy=multi-user.target
EOF
        echo "Reloading systemd daemon..."
        systemctl daemon-reload
        echo "Enabling $SERVICE_NAME service to start at boot..."
        systemctl enable "$SERVICE_NAME"
        echo "Starting $SERVICE_NAME service..."
        systemctl restart "$SERVICE_NAME"
        echo "Service status can be checked with: systemctl status $SERVICE_NAME"
    else
        echo "Notice: 'systemctl' not found. You will need to start '$INSTALL_PATH' manually or configure your own init script."
    fi

elif [ "$OS_SUFFIX" = "darwin" ]; then
    # launchd handling
    echo "Installing launchd service for macOS..."
    PLIST_FILE="/Library/LaunchDaemons/com.$SERVICE_NAME.service.plist"
    cat <<EOF > "$PLIST_FILE"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.$SERVICE_NAME.service</string>
    <key>ProgramArguments</key>
    <array>
        <string>$INSTALL_PATH</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>EnvironmentVariables</key>
    <dict>
        <key>IS_PUBLIC</key>
        <string>true</string>
    </dict>
    <key>StandardOutPath</key>
    <string>/var/log/$SERVICE_NAME.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/$SERVICE_NAME.error.log</string>
</dict>
</plist>
EOF
    
    echo "Unloading any existing service..."
    launchctl unload "$PLIST_FILE" 2>/dev/null || true
    echo "Loading new service..."
    launchctl load -w "$PLIST_FILE"
    echo "Service installed. Logs are available at /var/log/$SERVICE_NAME.log"
    echo "You can manage the service with: sudo launchctl list | grep $SERVICE_NAME"
fi

echo "================================================="
echo "                Install Complete                 "
echo "================================================="
