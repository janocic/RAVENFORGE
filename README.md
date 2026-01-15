# RavenForge

<p align="center">
  <img src="docs/assets/logo.png" alt="RavenForge Logo" width="200"/>
</p>

<p align="center">
  <strong>A comprehensive, open-source, tool-based cybersecurity platform</strong>
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#installation">Installation</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#documentation">Documentation</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License"/>
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go" alt="Go Version"/>
  <img src="https://img.shields.io/badge/Platform-Linux%20|%20macOS%20|%20Windows-lightgrey" alt="Platform"/>
</p>

---

## Overview

RavenForge is an extensible security operations platform that enables teams to build, deploy, and orchestrate security tools in isolated containers. It provides a unified framework for ingesting, detecting, enriching, correlating, and reporting on security events.

### Key Principles

- **Tool-Based Architecture**: Security capabilities are packaged as containerized tools with well-defined interfaces
- **Security by Default**: All tools run in OCI containers with minimal privileges and network isolation
- **Full Auditability**: Every action is logged with cryptographic chaining for SOC compliance
- **Policy-Driven**: Fine-grained control over tool capabilities (network, AI, response actions)
- **Pipeline Orchestration**: DAG-based execution of multi-stage security workflows

---

## Installation

### Arch Linux (Recommended)

#### Quick Install

```bash
git clone https://github.com/YOURUSERNAME/ravenforge.git
cd ravenforge
sudo ./scripts/install-arch.sh
```

#### Manual Install

```bash
# Install dependencies
sudo pacman -S go docker python python-pip sqlite git make

# Enable Docker
sudo systemctl enable --now docker
sudo usermod -aG docker $USER

# Clone and build
git clone https://github.com/YOURUSERNAME/ravenforge.git
cd ravenforge
make deps
make build

# Install system-wide
sudo make install

# Build Docker images for tools
make docker

# Start the service
sudo systemctl enable --now ravenforged
```

#### Using Makefile

```bash
make deps          # Install Go dependencies
make build         # Build binaries
sudo make install  # Install to /usr/local
make docker        # Build Docker images
make test          # Run tests
make help          # Show all targets
```

### Debian/Ubuntu

```bash
# Install dependencies
sudo apt update
sudo apt install -y golang docker.io python3 python3-pip sqlite3 git make gcc

# Enable Docker
sudo systemctl enable --now docker
sudo usermod -aG docker $USER
newgrp docker

# Clone and build
git clone https://github.com/YOURUSERNAME/ravenforge.git
cd ravenforge
make deps && make build
sudo make install
make docker
```

### Fedora/RHEL

```bash
# Install dependencies
sudo dnf install -y golang docker python3 python3-pip sqlite git make gcc

# Enable Docker
sudo systemctl enable --now docker
sudo usermod -aG docker $USER

# Clone and build
git clone https://github.com/YOURUSERNAME/ravenforge.git
cd ravenforge
make deps && make build
sudo make install
make docker
```

### macOS

```bash
# Install dependencies (using Homebrew)
brew install go docker sqlite python

# Start Docker Desktop, then:
git clone https://github.com/YOURUSERNAME/ravenforge.git
cd ravenforge
make deps && make build
make docker
```

---

## Features

### Core Platform

- 🛠️ **Tool Registry** - Discover, register, and manage security tools
- 📦 **Artifact Store** - Content-addressable storage with SHA256 verification
- 📝 **Audit Logger** - Append-only log with cryptographic hash chain
- 🔒 **Policy Engine** - Evaluate and enforce tool permissions
- 🐳 **Sandbox Runner** - Secure OCI container execution
- ⏰ **Job Scheduler** - Async job queue with persistence
- 🔄 **Pipeline Executor** - DAG-based workflow orchestration
- 🌐 **REST API** - Full-featured API with OpenAPI specification
- 💻 **CLI** - Comprehensive command-line interface

### Reference Tools

| Tool | Category | Description |
|------|----------|-------------|
| `ingest-jsonlines` | Ingest | Normalize JSON/JSONL logs to ECS format |
| `detect-simple-rules` | Detect | Rule-based detection engine |
| `enrich-geoip` | Enrich | GeoIP enrichment for IP addresses |
| `correlate-events` | Correlate | Event correlation and incident grouping |
| `report-generate` | Report | Security report generation |
| `triage-prioritize` | Triage | Incident prioritization and queuing |

## Quick Start

### Prerequisites

- Go 1.22+
- Docker 24+
- SQLite3

### Installation

```bash
# Clone the repository
git clone https://github.com/ravenforge/ravenforge.git
cd ravenforge

# Build the daemon and CLI
cd core
go build -o bin/ravenforged ./cmd/ravenforged
go build -o bin/ravenforge ./cmd/ravenforge

# Create configuration
cp config/ravenforge.example.yaml config/ravenforge.yaml
```

### Running the Daemon

```bash
# Start the daemon
./bin/ravenforged --config config/ravenforge.yaml

# In another terminal, check status
./bin/ravenforge status
```

### Basic Usage

```bash
# Discover tools
ravenforge tool discover ./tools

# List available tools
ravenforge tool list

# Run a tool
ravenforge run ingest-jsonlines \
  --input events=./data/raw-logs.jsonl \
  --output ./output

# Create and run a pipeline
ravenforge pipeline run ./pipelines/detection-pipeline.yaml \
  --input events=./data/raw-logs.jsonl
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                           RavenForge                                 │
├─────────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌────────────┐ │
│  │   REST API  │  │     CLI     │  │   Go SDK    │  │ Python SDK │ │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └─────┬──────┘ │
├─────────┴────────────────┴────────────────┴───────────────┴─────────┤
│                            Daemon Core                               │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐ │   │
│  │  │ Registry │ │ Artifact │ │  Audit   │ │  Policy Engine   │ │   │
│  │  │          │ │  Store   │ │  Logger  │ │                  │ │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘ │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────────────────────────┐  │   │
│  │  │ Scheduler│ │ Pipeline │ │       Sandbox Runner         │  │   │
│  │  │          │ │ Executor │ │    (Docker Integration)      │  │   │
│  │  └──────────┘ └──────────┘ └──────────────────────────────┘  │   │
│  └──────────────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────────────┤
│                          Tool Containers                             │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────────┐    │
│  │  ingest-*  │ │  detect-*  │ │  enrich-*  │ │  correlate-*   │    │
│  └────────────┘ └────────────┘ └────────────┘ └────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
```

### Components

| Component | Description |
|-----------|-------------|
| **Registry** | Tool discovery, registration, and metadata management |
| **Artifact Store** | Content-addressable storage for inputs/outputs |
| **Audit Logger** | Append-only audit log with hash chaining |
| **Policy Engine** | Permission evaluation for tool capabilities |
| **Sandbox Runner** | Docker-based isolated tool execution |
| **Scheduler** | Async job queue with worker pool |
| **Pipeline Executor** | DAG-based workflow orchestration |

## Tools

### Tool Manifest

Every tool is defined by a `tool.yaml` manifest:

```yaml
name: my-tool
version: "1.0.0"
description: Description of what the tool does
author: Your Name
license: MIT
category: detect

inputs:
  events:
    type: stream
    format: jsonl
    description: Input events
    required: true

outputs:
  results:
    type: stream
    format: jsonl
    description: Output results

parameters:
  threshold:
    type: integer
    description: Detection threshold
    default: 10

gates:
  network: false
  ai: false
  response_action: false
  secrets: false

resources:
  cpu: "1.0"
  memory: "512M"

timeout: 300
```

### Creating a Tool

1. Create directory structure:
```
tools/
└── my-category/
    └── my-tool/
        ├── tool.yaml
        ├── main.py
        └── Dockerfile
```

2. Use the SDK:
```python
from rfsdk import Tool, main_wrapper

@main_wrapper
def main(tool: Tool):
    tool.logger.info("Starting processing")
    
    events = tool.read_input_jsonl("events")
    results = []
    
    for event in events:
        # Process event
        results.append(process(event))
    
    tool.write_output_jsonl("results", results)
    return 0
```

3. Build container:
```dockerfile
FROM python:3.11-slim
WORKDIR /app
COPY sdk/libs/python /app/sdk
COPY tools/my-category/my-tool/main.py /app/
RUN useradd -r -u 1000 ravenforge
USER ravenforge
ENTRYPOINT ["python", "/app/main.py"]
```

## Pipelines

Pipelines define multi-stage security workflows:

```yaml
name: detection-pipeline
description: Full detection and triage pipeline

stages:
  - name: ingest
    tool: ingest-jsonlines
    inputs:
      raw_data: $INPUT.events

  - name: detect
    tool: detect-simple-rules
    depends_on: [ingest]
    inputs:
      events: $STAGE.ingest.normalized_events

  - name: correlate
    tool: correlate-events
    depends_on: [detect]
    inputs:
      events: $STAGE.detect.detections

  - name: triage
    tool: triage-prioritize
    depends_on: [correlate]
    inputs:
      incidents: $STAGE.correlate.incidents

  - name: report
    tool: report-generate
    depends_on: [triage]
    inputs:
      incidents: $STAGE.triage.prioritized
```

## API Reference

The REST API follows OpenAPI 3.0 specification. Key endpoints:

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/tools` | List all registered tools |
| GET | `/v1/tools/{name}` | Get tool details |
| POST | `/v1/runs` | Execute a tool |
| GET | `/v1/jobs` | List jobs |
| GET | `/v1/jobs/{id}` | Get job status |
| POST | `/v1/pipelines` | Create pipeline |
| POST | `/v1/pipelines/{id}/run` | Execute pipeline |
| GET | `/v1/artifacts` | List artifacts |
| GET | `/v1/audit` | Query audit log |

Full API documentation: [sdk/spec/openapi.yaml](sdk/spec/openapi.yaml)

## Configuration

```yaml
server:
  host: "0.0.0.0"
  port: 8080

artifacts:
  store_path: "/var/lib/ravenforge/artifacts"
  max_size_bytes: 10737418240
  retention_days: 30

audit:
  log_path: "/var/log/ravenforge/audit.jsonl"
  max_size_bytes: 104857600
  rotate_count: 10

policy:
  path: "/etc/ravenforge/policies"
  default_allow: false

sandbox:
  docker_host: "unix:///var/run/docker.sock"
  network_mode: "none"
  memory_limit: "1G"
  cpu_limit: "2.0"
  default_timeout: 300

database:
  path: "/var/lib/ravenforge/ravenforge.db"

logging:
  level: "info"
  format: "json"
```

## Security

RavenForge is designed with security as a first-class concern:

- **Container Isolation**: All tools run in Docker containers with:
  - Read-only root filesystem
  - No network by default
  - All capabilities dropped
  - No privilege escalation
  - Non-root user

- **Policy Engine**: Fine-grained control over:
  - Network access
  - AI/ML capabilities
  - Response actions
  - Secret access

- **Audit Logging**: Every action logged with:
  - Cryptographic hash chain
  - Tamper-evident design
  - SOC compliance ready

See [SECURITY.md](SECURITY.md) for security policy and vulnerability reporting.

## Development

### Building

```bash
cd core
go build ./...
```

### Testing

```bash
# Unit tests
go test ./...

# Integration tests
go test -tags=integration ./test/integration/...
```

### Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

RavenForge is licensed under the MIT License. See [LICENSE](LICENSE) for details.

## Acknowledgments

Built with:
- [Go](https://golang.org/)
- [Docker](https://www.docker.com/)
- [Chi Router](https://github.com/go-chi/chi)
- [Cobra](https://github.com/spf13/cobra)
- [Zap](https://github.com/uber-go/zap)
