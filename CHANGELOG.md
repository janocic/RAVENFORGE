# Changelog

All notable changes to RavenForge will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-01-15

### Added

#### Core Platform
- Tool Registry with automatic discovery from configured directories
- Artifact Store with content-addressable storage (SHA256)
- Audit Logger with cryptographic hash chain for SOC compliance
- Policy Engine for fine-grained capability control
- Docker-based Sandbox Runner for secure tool execution
- Job Scheduler with persistent queue
- Pipeline Executor for DAG-based workflows
- REST API server with OpenAPI specification
- CLI for daemon management and tool execution

#### Security Tools
- `ingest-jsonlines` - JSONL log ingestion and normalization
- `detect-simple-rules` - Rule-based detection engine
- `enrich-geoip` - GeoIP enrichment for IP addresses
- `correlate-events` - Event correlation and grouping
- `report-generate` - Security report generation
- `triage-prioritize` - Incident prioritization

#### SDK
- Python SDK for tool development
- Go SDK for tool development
- Tool manifest specification (tool.yaml)

#### Documentation
- Architecture documentation
- Tool development guide
- Configuration reference
- Installation guides for multiple platforms

#### DevOps
- Systemd service file
- Makefile for build automation
- PKGBUILD for Arch Linux
- GitHub Actions CI/CD workflows
- Docker image build automation

### Security
- Container isolation with network restrictions
- Drop all Linux capabilities by default
- Read-only root filesystem in containers
- Process and memory limits
- Audit logging with tamper detection

---

## Version History

- **1.0.0** - Initial release (2026-01-15)
