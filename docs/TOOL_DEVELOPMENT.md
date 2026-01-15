# Tool Development Guide

This guide explains how to develop custom tools for the RavenForge platform.

## Overview

Tools are containerized security capabilities that:
- Accept defined inputs (streams or files)
- Produce defined outputs
- Run in isolated Docker containers
- Follow a declarative manifest format

## Tool Structure

Every tool consists of:

```
tools/
└── <category>/
    └── <tool-name>/
        ├── tool.yaml      # Tool manifest (required)
        ├── main.py        # Tool implementation
        ├── Dockerfile     # Container build file
        └── requirements.txt  # Python dependencies (optional)
```

## Tool Manifest (tool.yaml)

The manifest defines the tool's interface:

```yaml
name: my-detection-tool
version: "1.0.0"
description: |
  Detailed description of what this tool does.
  Can span multiple lines.

author: Your Name <your.email@example.com>
license: MIT
category: detect

# Input definitions
inputs:
  events:
    type: stream           # stream | file
    format: jsonl          # jsonl | json | csv | binary | text
    description: Input security events
    required: true
  
  config:
    type: file
    format: json
    description: Optional configuration
    required: false

# Output definitions
outputs:
  detections:
    type: stream
    format: jsonl
    description: Generated detections
  
  stats:
    type: file
    format: json
    description: Processing statistics

# Configuration parameters
parameters:
  threshold:
    type: integer
    description: Detection threshold
    default: 10
    min: 1
    max: 100
  
  severity_filter:
    type: string
    description: Minimum severity to report
    default: "medium"
    enum: ["low", "medium", "high", "critical"]
  
  enabled_rules:
    type: array
    description: List of rules to enable
    default: []

# Required capabilities
gates:
  network: false           # External network access
  ai: false                # AI/ML model usage
  response_action: false   # Active response capability
  secrets: false           # Access to secrets store

# Resource limits
resources:
  cpu: "1.0"              # CPU cores
  memory: "512M"          # Memory limit

# Execution timeout (seconds)
timeout: 300
```

### Input/Output Types

| Type | Description |
|------|-------------|
| `stream` | Line-by-line data (JSONL, CSV) |
| `file` | Complete file (JSON, binary) |

### Input/Output Formats

| Format | Description |
|--------|-------------|
| `jsonl` | JSON Lines (one JSON object per line) |
| `json` | Standard JSON |
| `csv` | Comma-separated values |
| `text` | Plain text |
| `binary` | Binary data |

### Parameter Types

| Type | Description |
|------|-------------|
| `string` | Text value |
| `integer` | Whole number |
| `number` | Decimal number |
| `boolean` | True/false |
| `array` | List of values |
| `object` | Key-value map |

### Categories

| Category | Description |
|----------|-------------|
| `ingest` | Data normalization and parsing |
| `detect` | Detection and alerting |
| `enrich` | Data enrichment |
| `correlate` | Event correlation |
| `triage` | Prioritization and routing |
| `respond` | Automated response |
| `report` | Report generation |

## Python SDK

### Installation

The SDK is available at `/app/sdk` inside containers.

### Basic Usage

```python
#!/usr/bin/env python3
from rfsdk import Tool, main_wrapper, get_param

@main_wrapper
def main(tool: Tool):
    """Main entry point for the tool."""
    
    # Log with structured data
    tool.logger.info("Starting processing", version="1.0.0")
    
    # Get parameters
    threshold = get_param("threshold", 10)
    severity = get_param("severity_filter", "medium")
    
    # Read input events
    events = tool.read_input_jsonl("events")
    
    results = []
    for event in events:
        # Process each event
        result = process_event(event, threshold)
        if result:
            results.append(result)
    
    # Write outputs
    tool.write_output_jsonl("detections", results)
    tool.write_output_json("stats", {
        "processed": len(list(events)),
        "detected": len(results),
    })
    
    tool.logger.info("Processing complete", 
                    detected=len(results))
    
    return 0  # Exit code

def process_event(event, threshold):
    # Your detection logic here
    pass

if __name__ == "__main__":
    import sys
    sys.exit(main())
```

### SDK Reference

#### Tool Class

```python
class Tool:
    # Logger instance
    logger: Logger
    
    # List available inputs
    def list_inputs() -> List[str]
    
    # Get input file path
    def get_input_path(name: str) -> str
    
    # Read inputs
    def read_input_text(name: str) -> str
    def read_input_json(name: str) -> Any
    def read_input_jsonl(name: str) -> Iterator[Dict]
    
    # Get output file path
    def get_output_path(name: str) -> str
    
    # Write outputs
    def write_output(name: str, content: str)
    def write_output_json(name: str, data: Any)
    def write_output_jsonl(name: str, items: Iterable[Dict])
```

#### Logger Class

```python
class Logger:
    def debug(msg: str, **kwargs)
    def info(msg: str, **kwargs)
    def warn(msg: str, **kwargs)
    def error(msg: str, **kwargs)
```

#### Helper Functions

```python
# Get parameter with default
def get_param(name: str, default: Any = None) -> Any

# Main wrapper decorator
@main_wrapper
def main(tool: Tool) -> int:
    pass
```

### Error Handling

```python
@main_wrapper
def main(tool: Tool):
    try:
        # Your code here
        process_data(tool)
        return 0
    except ValueError as e:
        tool.logger.error("Validation error", error=str(e))
        return 1
    except Exception as e:
        tool.logger.error("Unexpected error", error=str(e))
        return 2
```

## Go SDK

### Basic Usage

```go
package main

import (
    "os"
    
    rfsdk "github.com/ravenforge/ravenforge/sdk/libs/go"
)

func main() {
    tool, err := rfsdk.NewTool()
    if err != nil {
        os.Exit(1)
    }
    
    tool.Logger.Info("Starting processing")
    
    // Get parameter
    threshold := rfsdk.GetParam("threshold", 10).(int)
    
    // Read events
    events, err := tool.ReadInputJSONL("events")
    if err != nil {
        tool.Logger.Error("Failed to read input", "error", err)
        os.Exit(1)
    }
    
    var results []map[string]interface{}
    for _, event := range events {
        // Process event
        if result := processEvent(event, threshold); result != nil {
            results = append(results, result)
        }
    }
    
    // Write outputs
    tool.WriteOutputJSONL("detections", results)
    tool.WriteOutputJSON("stats", map[string]interface{}{
        "processed": len(events),
        "detected":  len(results),
    })
    
    tool.Logger.Info("Processing complete", "detected", len(results))
}
```

## Dockerfile

Standard Dockerfile template:

```dockerfile
FROM python:3.11-slim

LABEL org.opencontainers.image.title="my-detection-tool"
LABEL org.opencontainers.image.description="Custom detection tool"
LABEL org.opencontainers.image.vendor="Your Organization"

WORKDIR /app

# Install dependencies (if any)
# RUN pip install --no-cache-dir pyyaml

# Copy SDK
COPY sdk/libs/python /app/sdk

# Copy tool
COPY tools/detect/my-detection-tool/main.py /app/

# Create non-root user (REQUIRED)
RUN useradd -r -u 1000 ravenforge && \
    chown -R ravenforge:ravenforge /app
USER ravenforge

ENTRYPOINT ["python", "/app/main.py"]
```

### Important Notes

1. **Non-root user**: Always run as non-root for security
2. **Minimal image**: Use slim/alpine base images
3. **No unnecessary tools**: Don't install shells, editors, etc.
4. **Pin versions**: Pin dependency versions for reproducibility

## Data Formats

### ECS-Aligned Events

Follow Elastic Common Schema for interoperability:

```json
{
  "@timestamp": "2024-01-15T10:30:00.000Z",
  "event": {
    "kind": "event",
    "category": ["authentication"],
    "type": ["start"],
    "outcome": "failure"
  },
  "source": {
    "ip": "192.168.1.100",
    "port": 54321
  },
  "destination": {
    "ip": "10.0.0.1",
    "port": 22
  },
  "user": {
    "name": "admin"
  },
  "host": {
    "name": "server-01"
  },
  "message": "Authentication failure for user admin"
}
```

### Detection Output

```json
{
  "@timestamp": "2024-01-15T10:30:05.000Z",
  "detection": {
    "rule_id": "auth-failure",
    "rule_name": "Authentication Failure",
    "description": "Detected authentication failure",
    "severity": "medium"
  },
  "event": {
    "kind": "alert",
    "category": ["intrusion_detection"],
    "type": ["indicator"],
    "risk_score": 50
  },
  "source_event": { ... }
}
```

## Testing Your Tool

### Local Testing

```bash
# Set up test environment
mkdir -p /tmp/ravenforge/{input,output}

# Create test input
echo '{"event":"test1"}' > /tmp/ravenforge/input/events.jsonl
echo '{"event":"test2"}' >> /tmp/ravenforge/input/events.jsonl

# Export environment variables
export RF_INPUT_DIR=/tmp/ravenforge/input
export RF_OUTPUT_DIR=/tmp/ravenforge/output
export RF_PARAMS='{"threshold": 5}'

# Run tool locally
python main.py

# Check outputs
cat /tmp/ravenforge/output/detections.jsonl
cat /tmp/ravenforge/output/stats.json
```

### Container Testing

```bash
# Build container
docker build -t my-detection-tool -f Dockerfile .

# Run with test inputs
docker run --rm \
  -v /tmp/ravenforge/input:/input:ro \
  -v /tmp/ravenforge/output:/output \
  -e RF_INPUT_DIR=/input \
  -e RF_OUTPUT_DIR=/output \
  -e RF_PARAMS='{"threshold": 5}' \
  my-detection-tool
```

### Integration Testing

```bash
# Register tool
ravenforge tool register ./tools/detect/my-detection-tool

# Run via platform
ravenforge run my-detection-tool \
  --input events=./test-data/events.jsonl \
  --output ./test-output \
  --param threshold=5

# Check results
cat ./test-output/detections.jsonl
```

## Best Practices

### 1. Input Validation

```python
def validate_event(event):
    """Validate event has required fields."""
    required = ["@timestamp", "message"]
    for field in required:
        if field not in event:
            raise ValueError(f"Missing required field: {field}")
```

### 2. Graceful Error Handling

```python
@main_wrapper
def main(tool: Tool):
    try:
        events = tool.read_input_jsonl("events")
        for event in events:
            try:
                process_event(event)
            except Exception as e:
                tool.logger.warn("Failed to process event", 
                               error=str(e),
                               event_id=event.get("id"))
        return 0
    except Exception as e:
        tool.logger.error("Fatal error", error=str(e))
        return 1
```

### 3. Memory Efficiency

```python
# Good: Process events one at a time
for event in tool.read_input_jsonl("events"):
    result = process(event)
    if result:
        yield result

# Bad: Load all events into memory
events = list(tool.read_input_jsonl("events"))
```

### 4. Structured Logging

```python
# Good: Structured logging
tool.logger.info("Processing complete", 
                events_processed=1000,
                detections=50,
                duration_ms=1234)

# Bad: Unstructured logging
print(f"Processed 1000 events, found 50 detections in 1234ms")
```

### 5. Idempotency

Tools should produce the same output for the same input:

```python
# Good: Deterministic processing
detections.sort(key=lambda x: x["@timestamp"])

# Bad: Non-deterministic output
import random
random.shuffle(detections)
```

## Troubleshooting

### Common Issues

**1. Import errors**
```python
# Ensure SDK path is set
import sys
sys.path.insert(0, "/app/sdk")
```

**2. Permission denied**
```dockerfile
# Run as correct user
USER ravenforge
```

**3. Missing inputs**
```python
# Check input availability
if "optional_input" in tool.list_inputs():
    data = tool.read_input_json("optional_input")
```

**4. Timeout issues**
```yaml
# Increase timeout in manifest
timeout: 600
```

## Examples

See the reference tools for complete examples:

- [ingest-jsonlines](../tools/ingest/ingest-jsonlines/) - JSON normalization
- [detect-simple-rules](../tools/detect/detect-simple-rules/) - Rule-based detection
- [enrich-geoip](../tools/enrich/enrich-geoip/) - GeoIP enrichment
- [correlate-events](../tools/correlate/correlate-events/) - Event correlation
- [report-generate](../tools/report/report-generate/) - Report generation
- [triage-prioritize](../tools/triage/triage-prioritize/) - Incident triage
