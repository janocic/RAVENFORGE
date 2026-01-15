# RavenForge Arch Linux Installation Guide

Complete installation guide for Arch Linux.

## Prerequisites

- Arch Linux (or Arch-based distro like Manjaro, EndeavourOS)
- Root access (sudo)
- Internet connection

## Quick Installation

```bash
# Clone the repository
git clone https://github.com/YOURUSERNAME/ravenforge.git
cd ravenforge

# Run the installer (requires root)
sudo ./scripts/install-arch.sh
```

The installer will:
1. Install all required packages (go, docker, python, sqlite, etc.)
2. Create ravenforge system user
3. Create necessary directories
4. Build and install binaries
5. Install systemd service
6. Build Docker images for all tools

## Manual Installation

### Step 1: Install Dependencies

```bash
# Update system
sudo pacman -Syu

# Install required packages
sudo pacman -S --needed \
    go \
    docker \
    python \
    python-pip \
    git \
    make \
    gcc \
    sqlite
```

### Step 2: Setup Docker

```bash
# Enable and start Docker
sudo systemctl enable docker
sudo systemctl start docker

# Add your user to docker group
sudo usermod -aG docker $USER

# Apply group membership (or log out and back in)
newgrp docker

# Verify Docker works
docker run hello-world
```

### Step 3: Clone Repository

```bash
git clone https://github.com/YOURUSERNAME/ravenforge.git
cd ravenforge
```

### Step 4: Build

```bash
# Using make
make deps
make build

# Or manually
cd core
go mod download
go mod tidy
CGO_ENABLED=1 go build -o ravenforged ./cmd/ravenforged
CGO_ENABLED=1 go build -o ravenforge ./cmd/ravenforge
cd ..
```

### Step 5: Install

```bash
# Using make
sudo make install

# Or manually
sudo install -Dm755 core/ravenforged /usr/local/bin/ravenforged
sudo install -Dm755 core/ravenforge /usr/local/bin/ravenforge
sudo install -Dm644 core/config/ravenforge.linux.yaml /etc/ravenforge/ravenforge.yaml
sudo install -Dm644 scripts/ravenforged.service /etc/systemd/system/ravenforged.service
```

### Step 6: Create Directories

```bash
# Create data directories
sudo mkdir -p /var/lib/ravenforge/artifacts
sudo mkdir -p /var/lib/ravenforge/tools
sudo mkdir -p /var/log/ravenforge

# Create system user
sudo useradd -r -s /bin/false -d /var/lib/ravenforge ravenforge

# Set permissions
sudo chown -R ravenforge:ravenforge /var/lib/ravenforge
sudo chown -R ravenforge:ravenforge /var/log/ravenforge
```

### Step 7: Install Tool Manifests

```bash
sudo cp -r tools/* /var/lib/ravenforge/tools/
sudo chown -R ravenforge:ravenforge /var/lib/ravenforge/tools
```

### Step 8: Build Docker Images

```bash
make docker

# Or manually build each one
docker build -t ravenforge/ingest-jsonlines:1.0.0 -f tools/ingest/ingest-jsonlines/Dockerfile .
docker build -t ravenforge/detect-simple-rules:1.0.0 -f tools/detect/detect-simple-rules/Dockerfile .
docker build -t ravenforge/enrich-geoip:1.0.0 -f tools/enrich/enrich-geoip/Dockerfile .
docker build -t ravenforge/correlate-events:1.0.0 -f tools/correlate/correlate-events/Dockerfile .
docker build -t ravenforge/report-generate:1.0.0 -f tools/report/report-generate/Dockerfile .
docker build -t ravenforge/triage-prioritize:1.0.0 -f tools/triage/triage-prioritize/Dockerfile .
```

### Step 9: Start Service

```bash
# Reload systemd
sudo systemctl daemon-reload

# Start service
sudo systemctl start ravenforged

# Enable on boot
sudo systemctl enable ravenforged

# Check status
sudo systemctl status ravenforged
```

## Verification

```bash
# Check if daemon is running
sudo systemctl status ravenforged

# List registered tools
ravenforge tool list

# Should output:
# ingest-jsonlines     1.0.0    Ingest JSONL files
# detect-simple-rules  1.0.0    Rule-based detection
# enrich-geoip         1.0.0    GeoIP enrichment
# correlate-events     1.0.0    Event correlation
# report-generate      1.0.0    Report generation
# triage-prioritize    1.0.0    Incident prioritization
```

## Configuration

Main configuration file: `/etc/ravenforge/ravenforge.yaml`

### Key Settings

```yaml
# Server configuration
server:
  host: "127.0.0.1"  # Bind address (use 0.0.0.0 for network access)
  port: 7433         # API port

# Tool directories
tool_dirs:
  - /var/lib/ravenforge/tools
  - /usr/share/ravenforge/tools

# Docker sandbox
sandbox:
  docker_socket: /var/run/docker.sock
  default_network: none  # none=isolated, bridge=network access
```

### Logging

```bash
# View service logs
journalctl -u ravenforged -f

# Audit log location
/var/log/ravenforge/audit.jsonl
```

## Troubleshooting

### Docker Permission Denied

```bash
# Add user to docker group
sudo usermod -aG docker $USER

# Apply without logout
newgrp docker
```

### Cannot Connect to Docker

```bash
# Check Docker is running
sudo systemctl status docker

# Start Docker
sudo systemctl start docker
```

### Build Errors

```bash
# Ensure CGO is enabled
export CGO_ENABLED=1

# Install gcc if missing
sudo pacman -S gcc
```

### Service Won't Start

```bash
# Check logs
journalctl -u ravenforged -n 50

# Common issues:
# - Docker not running
# - Permission issues on directories
# - Configuration syntax error
```

## Uninstallation

```bash
# Using script
sudo ./scripts/uninstall.sh

# Or manually
sudo systemctl stop ravenforged
sudo systemctl disable ravenforged
sudo rm /usr/local/bin/ravenforged
sudo rm /usr/local/bin/ravenforge
sudo rm /etc/systemd/system/ravenforged.service
sudo systemctl daemon-reload

# Optionally remove data
sudo rm -rf /etc/ravenforge
sudo rm -rf /var/lib/ravenforge
sudo rm -rf /var/log/ravenforge
sudo userdel ravenforge
```

## AUR Package (Future)

Once submitted to AUR:

```bash
# Using yay
yay -S ravenforge

# Or using paru
paru -S ravenforge
```

## Support

- [GitHub Issues](https://github.com/YOURUSERNAME/ravenforge/issues)
- [Documentation](https://github.com/YOURUSERNAME/ravenforge/tree/main/docs)
