#!/bin/bash
#
# RavenForge Installation Script for Arch Linux
# This script installs all dependencies and builds RavenForge
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
RAVENFORGE_VERSION="1.0.0"
INSTALL_PREFIX="/usr/local"
CONFIG_DIR="/etc/ravenforge"
DATA_DIR="/var/lib/ravenforge"
LOG_DIR="/var/log/ravenforge"

print_banner() {
    echo -e "${BLUE}"
    echo "╔═══════════════════════════════════════════════════════════╗"
    echo "║                                                           ║"
    echo "║   ██████╗  █████╗ ██╗   ██╗███████╗███╗   ██╗            ║"
    echo "║   ██╔══██╗██╔══██╗██║   ██║██╔════╝████╗  ██║            ║"
    echo "║   ██████╔╝███████║██║   ██║█████╗  ██╔██╗ ██║            ║"
    echo "║   ██╔══██╗██╔══██║╚██╗ ██╔╝██╔══╝  ██║╚██╗██║            ║"
    echo "║   ██║  ██║██║  ██║ ╚████╔╝ ███████╗██║ ╚████║            ║"
    echo "║   ╚═╝  ╚═╝╚═╝  ╚═╝  ╚═══╝  ╚══════╝╚═╝  ╚═══╝            ║"
    echo "║                                                           ║"
    echo "║   ███████╗ ██████╗ ██████╗  ██████╗ ███████╗             ║"
    echo "║   ██╔════╝██╔═══██╗██╔══██╗██╔════╝ ██╔════╝             ║"
    echo "║   █████╗  ██║   ██║██████╔╝██║  ███╗█████╗               ║"
    echo "║   ██╔══╝  ██║   ██║██╔══██╗██║   ██║██╔══╝               ║"
    echo "║   ██║     ╚██████╔╝██║  ██║╚██████╔╝███████╗             ║"
    echo "║   ╚═╝      ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝             ║"
    echo "║                                                           ║"
    echo "║   Security Operations Automation Platform v${RAVENFORGE_VERSION}          ║"
    echo "╚═══════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

check_arch_linux() {
    if [[ ! -f /etc/arch-release ]]; then
        log_warn "This script is designed for Arch Linux"
        read -p "Continue anyway? [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
}

install_dependencies() {
    log_info "Installing system dependencies..."
    
    # Update package database
    pacman -Sy --noconfirm
    
    # Core dependencies
    pacman -S --needed --noconfirm \
        go \
        docker \
        python \
        python-pip \
        git \
        make \
        gcc \
        sqlite
    
    # Enable and start Docker
    log_info "Enabling Docker service..."
    systemctl enable docker
    systemctl start docker
    
    log_info "Dependencies installed successfully"
}

install_python_deps() {
    log_info "Installing Python dependencies for tools..."
    
    pip install --break-system-packages \
        pyyaml \
        maxminddb \
        requests
    
    log_info "Python dependencies installed"
}

create_user() {
    log_info "Creating ravenforge system user..."
    
    if ! id "ravenforge" &>/dev/null; then
        useradd -r -s /bin/false -d "$DATA_DIR" ravenforge
        log_info "User 'ravenforge' created"
    else
        log_info "User 'ravenforge' already exists"
    fi
    
    # Add current user to docker group
    if [[ -n "$SUDO_USER" ]]; then
        usermod -aG docker "$SUDO_USER"
        log_info "Added $SUDO_USER to docker group"
    fi
}

create_directories() {
    log_info "Creating directories..."
    
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$CONFIG_DIR/policies"
    mkdir -p "$DATA_DIR/artifacts"
    mkdir -p "$DATA_DIR/tools"
    mkdir -p "$LOG_DIR"
    
    chown -R ravenforge:ravenforge "$DATA_DIR"
    chown -R ravenforge:ravenforge "$LOG_DIR"
    chmod 750 "$DATA_DIR"
    chmod 750 "$LOG_DIR"
    
    log_info "Directories created"
}

build_ravenforge() {
    log_info "Building RavenForge..."
    
    cd "$(dirname "$0")"
    
    # Build Go binaries
    cd core
    
    log_info "Downloading Go modules..."
    go mod download
    go mod tidy
    
    log_info "Building ravenforged daemon..."
    CGO_ENABLED=1 go build -o ravenforged ./cmd/ravenforged
    
    log_info "Building ravenforge CLI..."
    CGO_ENABLED=1 go build -o ravenforge ./cmd/ravenforge
    
    cd ..
    
    log_info "Build completed"
}

install_binaries() {
    log_info "Installing binaries..."
    
    cd "$(dirname "$0")"
    
    install -Dm755 core/ravenforged "$INSTALL_PREFIX/bin/ravenforged"
    install -Dm755 core/ravenforge "$INSTALL_PREFIX/bin/ravenforge"
    
    log_info "Binaries installed to $INSTALL_PREFIX/bin/"
}

install_config() {
    log_info "Installing configuration..."
    
    cd "$(dirname "$0")"
    
    # Install main config if not exists
    if [[ ! -f "$CONFIG_DIR/ravenforge.yaml" ]]; then
        install -Dm644 core/config/ravenforge.linux.yaml "$CONFIG_DIR/ravenforge.yaml"
        log_info "Configuration installed to $CONFIG_DIR/ravenforge.yaml"
    else
        log_warn "Configuration already exists, skipping (backup at ravenforge.yaml.new)"
        install -Dm644 core/config/ravenforge.linux.yaml "$CONFIG_DIR/ravenforge.yaml.new"
    fi
}

install_systemd_service() {
    log_info "Installing systemd service..."
    
    cd "$(dirname "$0")"
    
    install -Dm644 scripts/ravenforged.service /etc/systemd/system/ravenforged.service
    
    systemctl daemon-reload
    
    log_info "Systemd service installed"
}

install_tools() {
    log_info "Installing tool manifests..."
    
    cd "$(dirname "$0")"
    
    # Copy tool directories
    cp -r tools/* "$DATA_DIR/tools/"
    chown -R ravenforge:ravenforge "$DATA_DIR/tools"
    
    log_info "Tool manifests installed to $DATA_DIR/tools/"
}

build_docker_images() {
    log_info "Building Docker images for tools..."
    
    cd "$(dirname "$0")"
    
    local tools=(
        "ingest:ingest-jsonlines"
        "detect:detect-simple-rules"
        "enrich:enrich-geoip"
        "correlate:correlate-events"
        "report:report-generate"
        "triage:triage-prioritize"
    )
    
    for tool in "${tools[@]}"; do
        IFS=':' read -r category name <<< "$tool"
        local tool_dir="tools/$category/$name"
        
        if [[ -f "$tool_dir/Dockerfile" ]]; then
            log_info "Building ravenforge/$name:1.0.0..."
            docker build -t "ravenforge/$name:1.0.0" -f "$tool_dir/Dockerfile" .
        fi
    done
    
    log_info "Docker images built successfully"
}

print_completion() {
    echo
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║           RavenForge Installation Complete!               ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════╝${NC}"
    echo
    echo -e "${BLUE}Quick Start:${NC}"
    echo
    echo "  1. Start the daemon:"
    echo -e "     ${YELLOW}sudo systemctl start ravenforged${NC}"
    echo
    echo "  2. Enable on boot:"
    echo -e "     ${YELLOW}sudo systemctl enable ravenforged${NC}"
    echo
    echo "  3. Check status:"
    echo -e "     ${YELLOW}ravenforge tool list${NC}"
    echo
    echo -e "${BLUE}Manual start (for debugging):${NC}"
    echo -e "     ${YELLOW}ravenforged --config /etc/ravenforge/ravenforge.yaml${NC}"
    echo
    echo -e "${BLUE}Configuration:${NC}"
    echo "     $CONFIG_DIR/ravenforge.yaml"
    echo
    echo -e "${BLUE}Logs:${NC}"
    echo "     journalctl -u ravenforged -f"
    echo "     $LOG_DIR/audit.jsonl"
    echo
    echo -e "${BLUE}Documentation:${NC}"
    echo "     https://github.com/yourusername/ravenforge"
    echo
    
    if [[ -n "$SUDO_USER" ]]; then
        echo -e "${YELLOW}NOTE: Log out and back in for docker group membership to take effect${NC}"
    fi
}

# Main installation flow
main() {
    print_banner
    
    check_root
    check_arch_linux
    
    echo
    log_info "Starting RavenForge installation..."
    echo
    
    install_dependencies
    install_python_deps
    create_user
    create_directories
    build_ravenforge
    install_binaries
    install_config
    install_systemd_service
    install_tools
    build_docker_images
    
    print_completion
}

# Parse arguments
case "${1:-}" in
    --deps-only)
        check_root
        install_dependencies
        install_python_deps
        ;;
    --build-only)
        build_ravenforge
        ;;
    --docker-only)
        build_docker_images
        ;;
    --help|-h)
        echo "RavenForge Installation Script"
        echo
        echo "Usage: $0 [OPTION]"
        echo
        echo "Options:"
        echo "  --deps-only     Install only system dependencies"
        echo "  --build-only    Build binaries only (no install)"
        echo "  --docker-only   Build Docker images only"
        echo "  --help, -h      Show this help message"
        echo
        echo "Without options, performs full installation."
        ;;
    *)
        main
        ;;
esac
