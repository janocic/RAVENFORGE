#!/bin/bash
#
# RavenForge Uninstall Script for Linux
#

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}[ERROR]${NC} This script must be run as root"
    exit 1
fi

echo "RavenForge Uninstaller"
echo "======================"
echo
echo "This will remove:"
echo "  - /usr/local/bin/ravenforged"
echo "  - /usr/local/bin/ravenforge"
echo "  - /etc/systemd/system/ravenforged.service"
echo
echo "This will NOT remove:"
echo "  - /etc/ravenforge/ (configuration)"
echo "  - /var/lib/ravenforge/ (data)"
echo "  - /var/log/ravenforge/ (logs)"
echo "  - Docker images"
echo

read -p "Continue? [y/N] " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    exit 0
fi

log_info "Stopping service..."
systemctl stop ravenforged 2>/dev/null || true
systemctl disable ravenforged 2>/dev/null || true

log_info "Removing binaries..."
rm -f /usr/local/bin/ravenforged
rm -f /usr/local/bin/ravenforge

log_info "Removing systemd service..."
rm -f /etc/systemd/system/ravenforged.service
systemctl daemon-reload

echo
log_info "RavenForge uninstalled successfully"
echo
echo "To completely remove all data:"
echo "  sudo rm -rf /etc/ravenforge"
echo "  sudo rm -rf /var/lib/ravenforge"
echo "  sudo rm -rf /var/log/ravenforge"
echo "  sudo userdel ravenforge"
echo
echo "To remove Docker images:"
echo "  docker rmi \$(docker images 'ravenforge/*' -q)"
