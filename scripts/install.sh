#!/bin/bash
#
# Quick Install Script for RavenForge
# Works on most Linux distributions
#

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Detect distribution
detect_distro() {
    if [[ -f /etc/arch-release ]]; then
        echo "arch"
    elif [[ -f /etc/debian_version ]]; then
        echo "debian"
    elif [[ -f /etc/fedora-release ]]; then
        echo "fedora"
    elif [[ -f /etc/redhat-release ]]; then
        echo "rhel"
    else
        echo "unknown"
    fi
}

install_deps_arch() {
    log_info "Installing dependencies for Arch Linux..."
    sudo pacman -Sy --needed --noconfirm go docker python python-pip sqlite git make gcc
}

install_deps_debian() {
    log_info "Installing dependencies for Debian/Ubuntu..."
    sudo apt update
    sudo apt install -y golang docker.io python3 python3-pip sqlite3 git make gcc
}

install_deps_fedora() {
    log_info "Installing dependencies for Fedora..."
    sudo dnf install -y golang docker python3 python3-pip sqlite git make gcc
}

install_deps_rhel() {
    log_info "Installing dependencies for RHEL/CentOS..."
    sudo yum install -y golang docker python3 python3-pip sqlite git make gcc
}

setup_docker() {
    log_info "Setting up Docker..."
    
    sudo systemctl enable docker 2>/dev/null || true
    sudo systemctl start docker 2>/dev/null || true
    
    if [[ -n "$SUDO_USER" ]]; then
        sudo usermod -aG docker "$SUDO_USER"
    elif [[ -n "$USER" && "$USER" != "root" ]]; then
        sudo usermod -aG docker "$USER"
    fi
}

# Main
main() {
    echo -e "${BLUE}"
    echo "╔═══════════════════════════════════════════════╗"
    echo "║    RavenForge Quick Installer                 ║"
    echo "╚═══════════════════════════════════════════════╝"
    echo -e "${NC}"
    
    distro=$(detect_distro)
    
    log_info "Detected distribution: $distro"
    
    case "$distro" in
        arch)
            install_deps_arch
            ;;
        debian)
            install_deps_debian
            ;;
        fedora)
            install_deps_fedora
            ;;
        rhel)
            install_deps_rhel
            ;;
        *)
            log_warn "Unknown distribution. Please install dependencies manually:"
            echo "  - go (1.22+)"
            echo "  - docker"
            echo "  - python3"
            echo "  - sqlite"
            echo "  - git"
            echo "  - make"
            echo "  - gcc"
            read -p "Continue anyway? [y/N] " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                exit 1
            fi
            ;;
    esac
    
    setup_docker
    
    log_info "Building RavenForge..."
    make deps
    make build
    
    log_info "Installing RavenForge..."
    sudo make install
    
    log_info "Building Docker images..."
    make docker
    
    log_info "Creating ravenforge user..."
    if ! id "ravenforge" &>/dev/null; then
        sudo useradd -r -s /bin/false -d /var/lib/ravenforge ravenforge
    fi
    
    log_info "Setting up directories..."
    sudo mkdir -p /var/lib/ravenforge/artifacts
    sudo mkdir -p /var/lib/ravenforge/tools
    sudo mkdir -p /var/log/ravenforge
    sudo chown -R ravenforge:ravenforge /var/lib/ravenforge
    sudo chown -R ravenforge:ravenforge /var/log/ravenforge
    
    log_info "Installing tool manifests..."
    sudo cp -r tools/* /var/lib/ravenforge/tools/
    sudo chown -R ravenforge:ravenforge /var/lib/ravenforge/tools
    
    echo
    echo -e "${GREEN}Installation complete!${NC}"
    echo
    echo "To start RavenForge:"
    echo "  sudo systemctl start ravenforged"
    echo "  sudo systemctl enable ravenforged"
    echo
    echo "To verify:"
    echo "  ravenforge tool list"
    echo
    
    if [[ -n "$SUDO_USER" || -n "$USER" ]]; then
        echo -e "${YELLOW}NOTE: Log out and back in for docker group membership to take effect${NC}"
    fi
}

main "$@"
