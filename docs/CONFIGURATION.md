# RavenForge Configuration Reference

Complete reference for all configuration options.

## Configuration File Locations

| Platform | Path |
|----------|------|
| Linux | `/etc/ravenforge/ravenforge.yaml` |
| macOS | `~/.config/ravenforge/ravenforge.yaml` |
| Windows | `%APPDATA%\ravenforge\ravenforge.yaml` |

## Full Configuration Example

```yaml
# =============================================================================
# RavenForge Configuration
# =============================================================================

# Tool directories to scan for manifests
# Daemon will recursively search these directories for tool.yaml files
tool_dirs:
  - /var/lib/ravenforge/tools
  - /usr/share/ravenforge/tools
  - ~/.local/share/ravenforge/tools

# =============================================================================
# API Server Configuration
# =============================================================================
server:
  # Bind address
  # Use 127.0.0.1 for local only, 0.0.0.0 for network access
  host: "127.0.0.1"
  
  # Port to listen on
  port: 7433
  
  # Request timeout
  timeout: 30s
  
  # TLS configuration (optional)
  tls:
    enabled: false
    cert_file: /etc/ravenforge/tls/cert.pem
    key_file: /etc/ravenforge/tls/key.pem

# =============================================================================
# Artifact Storage Configuration
# =============================================================================
artifacts:
  # Base directory for artifact storage
  base_dir: /var/lib/ravenforge/artifacts
  
  # Maximum total storage size in bytes (0 = unlimited)
  max_size: 10737418240  # 10GB
  
  # Retention policy
  retention:
    enabled: true
    max_age_days: 90
    min_free_space_gb: 5

# =============================================================================
# Audit Logging Configuration
# =============================================================================
audit:
  # Enable audit logging
  enabled: true
  
  # Path to audit log file (append-only JSONL)
  log_path: /var/log/ravenforge/audit.jsonl
  
  # Maximum size before rotation (MB)
  max_size_mb: 100
  
  # Number of rotated logs to keep
  max_backups: 10
  
  # Maximum age of rotated logs (days)
  max_age_days: 90
  
  # Include cryptographic hash chain
  hash_chain: true

# =============================================================================
# Policy Engine Configuration
# =============================================================================
policy:
  # Path to policy file
  policy_file: /etc/ravenforge/policy.yaml
  
  # Default mode: enforce, audit, or disabled
  default_mode: enforce
  
  # Default capabilities for tools without explicit policy
  default_capabilities:
    - read_input
    - write_output
  
  # Denied capabilities (always blocked)
  denied_capabilities:
    - response_action
    - exfiltrate_data

# =============================================================================
# Job Scheduler Configuration
# =============================================================================
scheduler:
  # Number of concurrent workers
  workers: 4
  
  # Maximum job queue size
  max_queue_size: 1000
  
  # Default job timeout
  default_timeout: 5m
  
  # Job persistence
  persistence:
    enabled: true
    database: /var/lib/ravenforge/jobs.db

# =============================================================================
# Container Sandbox Configuration
# =============================================================================
sandbox:
  # Container runtime: docker, podman
  runtime: docker
  
  # Docker socket path
  # Linux: /var/run/docker.sock
  # macOS: /var/run/docker.sock (or leave empty for Docker Desktop)
  # Windows: npipe:////./pipe/docker_engine (or leave empty)
  docker_socket: /var/run/docker.sock
  
  # Default network mode: none, bridge, host
  # none = completely isolated (recommended for security)
  # bridge = network access through NAT
  # host = direct host network access (not recommended)
  default_network: none
  
  # Default resource limits
  default_limits:
    # CPU limit (1.0 = 1 core)
    cpu_limit: 1.0
    
    # Memory limit in bytes
    memory_limit: 536870912  # 512MB
    
    # Maximum number of processes
    pids_limit: 100
    
    # Execution timeout
    timeout: 5m
  
  # Security options
  security:
    # Run containers as non-root
    no_new_privileges: true
    
    # Drop all capabilities
    drop_capabilities: true
    
    # Read-only root filesystem
    read_only_rootfs: true
    
    # Seccomp profile (empty = default)
    seccomp_profile: ""

# =============================================================================
# Database Configuration
# =============================================================================
database:
  # Database driver: sqlite3, postgres
  driver: sqlite3
  
  # Connection string or path
  # SQLite: /var/lib/ravenforge/ravenforge.db
  # Postgres: postgres://user:pass@localhost/ravenforge?sslmode=disable
  path: /var/lib/ravenforge/ravenforge.db
  
  # Connection pool settings (for postgres)
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 5m

# =============================================================================
# Logging Configuration
# =============================================================================
logging:
  # Log level: debug, info, warn, error
  level: info
  
  # Output format: json, console
  format: json
  
  # Log file path (empty = stdout)
  file: ""
  
  # Log rotation
  rotation:
    max_size_mb: 100
    max_backups: 5
    max_age_days: 30
```

## Environment Variables

Configuration can be overridden with environment variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `RAVENFORGE_CONFIG` | Config file path | `/etc/ravenforge/ravenforge.yaml` |
| `RAVENFORGE_HOST` | Server bind address | `0.0.0.0` |
| `RAVENFORGE_PORT` | Server port | `7433` |
| `RAVENFORGE_LOG_LEVEL` | Log level | `debug` |
| `RAVENFORGE_LOG_FORMAT` | Log format | `console` |
| `DOCKER_HOST` | Docker socket | `unix:///var/run/docker.sock` |

## CLI Flags

The daemon accepts these command-line flags:

```bash
ravenforged [flags]

Flags:
  --config string       Path to config file (default "/etc/ravenforge/ravenforge.yaml")
  --host string         Server bind address
  --port int            Server port
  --log-level string    Log level (debug, info, warn, error)
  --log-format string   Log format (json, console)
  -h, --help            Show help
```

## Minimal Configuration

Minimal working configuration:

```yaml
tool_dirs:
  - /var/lib/ravenforge/tools

server:
  port: 7433

artifacts:
  base_dir: /var/lib/ravenforge/artifacts

audit:
  log_path: /var/log/ravenforge/audit.jsonl

database:
  path: /var/lib/ravenforge/ravenforge.db
```

## Security Recommendations

For production environments:

```yaml
# Bind only to localhost
server:
  host: "127.0.0.1"

# Enable all security options
sandbox:
  default_network: none
  security:
    no_new_privileges: true
    drop_capabilities: true
    read_only_rootfs: true

# Strict policy
policy:
  default_mode: enforce
  denied_capabilities:
    - response_action
    - network_access

# Enable audit
audit:
  enabled: true
  hash_chain: true
```

## See Also

- [Architecture Overview](ARCHITECTURE.md)
- [Tool Development Guide](TOOL_DEVELOPMENT.md)
- [Policy Reference](POLICY_REFERENCE.md)
