# RavenForge Architecture

This document describes the architecture of the RavenForge cybersecurity platform.

## Overview

RavenForge is a tool-based security operations platform that enables teams to build, deploy, and orchestrate security tools in isolated containers. The platform provides a unified framework for security operations with a focus on auditability, security, and extensibility.

## Design Principles

### 1. Tool-Based Architecture

Security capabilities are packaged as containerized tools with well-defined interfaces:

- **Inputs**: Data streams (JSONL) or files (JSON, binary)
- **Outputs**: Produced data streams or files
- **Parameters**: Configuration options
- **Gates**: Required capabilities (network, AI, response_action, secrets)

### 2. Security by Default

All tools run in OCI containers with minimal privileges:

- Read-only root filesystem
- No network access by default
- All Linux capabilities dropped
- No privilege escalation allowed
- Non-root user execution

### 3. Full Auditability

Every action is logged with cryptographic chaining:

- Append-only log file
- Each entry includes hash of previous entry
- Tamper-evident design
- SOC/compliance ready

### 4. Policy-Driven Execution

Fine-grained control over tool capabilities:

- Tool-level permissions
- User/role-based access
- Gate requirements enforcement
- Default-deny posture option

## Component Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              External Interfaces                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌─────────────┐  │
│  │   REST API   │  │     CLI      │  │   Go SDK     │  │ Python SDK  │  │
│  │   (Chi)      │  │   (Cobra)    │  │              │  │             │  │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────┬──────┘  │
└─────────┼─────────────────┼─────────────────┼─────────────────┼─────────┘
          │                 │                 │                 │
          └────────────┬────┴────────────┬────┴────────────┬────┘
                       ▼                 ▼                 ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                            Daemon Core                                   │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                         Request Router                              │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                   │                                      │
│         ┌───────────┬─────────────┼─────────────┬───────────┐           │
│         ▼           ▼             ▼             ▼           ▼           │
│  ┌───────────┐┌───────────┐┌───────────┐┌───────────┐┌───────────┐      │
│  │  Registry ││  Artifact ││   Audit   ││  Policy   ││ Scheduler │      │
│  │           ││   Store   ││  Logger   ││  Engine   ││           │      │
│  └─────┬─────┘└─────┬─────┘└─────┬─────┘└─────┬─────┘└─────┬─────┘      │
│        │            │            │            │            │             │
│        │            │            │            │            │             │
│  ┌─────┴────────────┴────────────┴────────────┴────────────┴──────┐     │
│  │                        SQLite Database                          │     │
│  └─────────────────────────────────────────────────────────────────┘     │
│                                   │                                      │
│                                   ▼                                      │
│  ┌─────────────────────────────────────────────────────────────────┐     │
│  │                      Pipeline Executor                           │     │
│  │                    (DAG Orchestration)                          │     │
│  └───────────────────────────┬─────────────────────────────────────┘     │
│                              │                                           │
│                              ▼                                           │
│  ┌─────────────────────────────────────────────────────────────────┐     │
│  │                      Sandbox Runner                              │     │
│  │                  (Docker Integration)                           │     │
│  └───────────────────────────┬─────────────────────────────────────┘     │
└──────────────────────────────┼──────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                        Docker Runtime                                    │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐         │
│  │  Tool A    │  │  Tool B    │  │  Tool C    │  │  Tool D    │         │
│  │ Container  │  │ Container  │  │ Container  │  │ Container  │         │
│  └────────────┘  └────────────┘  └────────────┘  └────────────┘         │
└─────────────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Tool Registry

**Purpose**: Discover, register, and manage security tools.

**Responsibilities**:
- Parse and validate tool manifests
- Store tool metadata in database
- Provide tool lookup and search
- Track tool versions

**Key Files**:
- `internal/manifest/manifest.go` - Manifest parsing and validation
- `internal/registry/registry.go` - Tool registration and lookup

**Data Model**:
```
Tool
├── Name (unique identifier)
├── Version
├── Description
├── Author
├── License
├── Category
├── Inputs[]
├── Outputs[]
├── Parameters[]
├── Gates
├── Resources
└── Timeout
```

### 2. Artifact Store

**Purpose**: Content-addressable storage for inputs and outputs.

**Responsibilities**:
- Store artifacts with SHA256 hashing
- Track metadata (source, type, timestamps)
- Verify artifact integrity
- Manage retention policies

**Key Files**:
- `internal/artifact/store.go` - Artifact storage and retrieval

**Features**:
- Content-addressable by SHA256
- Deduplication of identical content
- Integrity verification on retrieval
- Configurable retention policies

### 3. Audit Logger

**Purpose**: Append-only audit log with cryptographic chaining.

**Responsibilities**:
- Log all security-relevant actions
- Maintain hash chain integrity
- Support log verification
- Provide query interface

**Key Files**:
- `internal/audit/audit.go` - Audit logging implementation

**Entry Format**:
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "sequence": 12345,
  "action": "tool.run",
  "actor": "user@example.com",
  "resource": "detect-simple-rules",
  "details": { ... },
  "prev_hash": "abc123...",
  "hash": "def456..."
}
```

### 4. Policy Engine

**Purpose**: Evaluate and enforce tool permissions.

**Responsibilities**:
- Evaluate policy rules
- Check gate requirements
- Enforce access controls
- Provide policy decisions

**Key Files**:
- `internal/policy/policy.go` - Policy evaluation

**Policy Structure**:
```yaml
rules:
  - name: allow-network-tools
    subjects: ["admin"]
    resources: ["enrich-*"]
    actions: ["run"]
    gates:
      network: true
    effect: allow
```

### 5. Sandbox Runner

**Purpose**: Secure OCI container execution.

**Responsibilities**:
- Create isolated containers
- Mount inputs/outputs
- Enforce resource limits
- Collect execution results

**Key Files**:
- `internal/sandbox/runner.go` - Docker integration

**Security Hardening**:
```go
HostConfig{
    ReadonlyRootfs: true,
    NetworkMode:    "none",
    CapDrop:        []string{"ALL"},
    SecurityOpt:    []string{"no-new-privileges"},
    Resources: container.Resources{
        Memory:   memLimit,
        NanoCPUs: cpuLimit,
    },
}
```

### 6. Job Scheduler

**Purpose**: Async job queue with persistence.

**Responsibilities**:
- Queue job submissions
- Manage worker pool
- Track job status
- Handle job recovery

**Key Files**:
- `internal/scheduler/scheduler.go` - Job scheduling

**Job States**:
```
pending → running → completed
                 → failed
```

### 7. Pipeline Executor

**Purpose**: DAG-based workflow orchestration.

**Responsibilities**:
- Parse pipeline definitions
- Build execution DAG
- Execute stages in order
- Handle stage dependencies

**Key Files**:
- `internal/pipeline/pipeline.go` - Pipeline execution

**Pipeline Format**:
```yaml
stages:
  - name: ingest
    tool: ingest-jsonlines
    inputs:
      raw_data: $INPUT.events

  - name: detect
    tool: detect-simple-rules
    depends_on: [ingest]
    inputs:
      events: $STAGE.ingest.normalized
```

## Data Flow

### Single Tool Execution

```
1. Client submits run request
   │
2. Policy engine evaluates permissions
   │
3. Artifact store receives inputs
   │
4. Scheduler queues job
   │
5. Worker picks up job
   │
6. Sandbox runner creates container
   │
7. Tool executes in container
   │
8. Outputs stored in artifact store
   │
9. Audit logger records execution
   │
10. Job marked complete
```

### Pipeline Execution

```
1. Client submits pipeline
   │
2. Pipeline executor parses definition
   │
3. DAG built from stage dependencies
   │
4. Stages executed in topological order
   │
   ├── Stage A (no deps) ────┐
   │                         │
   ├── Stage B (no deps) ────┼── Parallel execution
   │                         │
   │   ┌─────────────────────┘
   │   │
   │   ▼
   ├── Stage C (deps: A, B) ── Waits for A, B
   │   │
   │   ▼
   └── Stage D (deps: C) ── Final stage
   
5. Results aggregated
   │
6. Pipeline marked complete
```

## Storage

### SQLite Database

Tables:
- `tools` - Tool metadata and manifests
- `artifacts` - Artifact metadata
- `jobs` - Job records and status
- `pipelines` - Pipeline definitions

### File System

```
/var/lib/ravenforge/
├── artifacts/          # Artifact content storage
│   ├── ab/
│   │   └── cdef1234... # SHA256-based paths
│   └── ...
├── ravenforge.db       # SQLite database
└── tools/              # Tool directories
    ├── ingest/
    ├── detect/
    └── ...

/var/log/ravenforge/
└── audit.jsonl         # Audit log
```

## Security Model

### Container Security

Every tool container runs with:

| Setting | Value | Purpose |
|---------|-------|---------|
| `ReadonlyRootfs` | `true` | Prevent filesystem modifications |
| `NetworkMode` | `none` | No network by default |
| `CapDrop` | `ALL` | Remove all capabilities |
| `SecurityOpt` | `no-new-privileges` | Prevent privilege escalation |
| `User` | `1000` | Non-root user |

### Gate System

Tools declare required capabilities:

| Gate | Description |
|------|-------------|
| `network` | External network access |
| `ai` | AI/ML model invocation |
| `response_action` | Active response capabilities |
| `secrets` | Access to secrets store |

Policy engine verifies gates before execution.

### Audit Trail

All actions logged with:
- Timestamp
- Sequential number
- Action type
- Actor identity
- Resource affected
- Details
- Previous hash
- Current hash

Hash chain provides tamper evidence.

## API Design

### REST API

- **Versioned**: `/v1/` prefix
- **JSON**: Request/response bodies
- **RESTful**: Standard HTTP methods
- **Pagination**: For list endpoints
- **Filtering**: Query parameters

### Error Handling

```json
{
  "error": {
    "code": "TOOL_NOT_FOUND",
    "message": "Tool 'invalid-tool' not found",
    "details": { ... }
  }
}
```

## Extensibility

### Adding New Tools

1. Create tool directory
2. Write `tool.yaml` manifest
3. Implement tool logic
4. Create `Dockerfile`
5. Register with daemon

### SDK Support

Official SDKs:
- **Go** - `sdk/libs/go/rfsdk.go`
- **Python** - `sdk/libs/python/rfsdk.py`

SDKs provide:
- Input/output handling
- Logging integration
- Parameter access
- Error handling

## Performance Considerations

### Scalability

- Worker pool for parallel execution
- Connection pooling for Docker
- Caching in registry lookups
- Efficient artifact deduplication

### Resource Management

- Configurable container limits
- Job queue size limits
- Artifact retention policies
- Audit log rotation

## Future Considerations

### Planned Features

- Distributed execution across nodes
- Real-time streaming pipelines
- Enhanced AI/ML integration
- Kubernetes deployment option
- Web-based dashboard

### Integration Points

- SIEM connectors
- Ticketing system webhooks
- Threat intelligence feeds
- Cloud provider integrations
