#!/usr/bin/env python3
"""
JSONL Log Ingester Tool

Reads JSONL log files, validates their structure, and outputs normalized events.
"""

import json
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional

# Add SDK to path (in container, SDK is pre-installed)
sys.path.insert(0, "/app/sdk")

try:
    from rfsdk import Tool, main_wrapper, get_param
except ImportError:
    # Fallback for local testing
    import os
    sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "libs", "python"))
    from rfsdk import Tool, main_wrapper, get_param


def normalize_timestamp(value: Any) -> Optional[str]:
    """Normalize various timestamp formats to ISO8601."""
    if value is None:
        return None
    
    if isinstance(value, (int, float)):
        # Unix timestamp
        try:
            dt = datetime.fromtimestamp(value, tz=timezone.utc)
            return dt.isoformat()
        except (ValueError, OSError):
            return None
    
    if isinstance(value, str):
        # Try parsing common formats
        formats = [
            "%Y-%m-%dT%H:%M:%S.%fZ",
            "%Y-%m-%dT%H:%M:%SZ",
            "%Y-%m-%d %H:%M:%S",
            "%Y-%m-%d %H:%M:%S.%f",
            "%b %d %H:%M:%S",  # syslog format
        ]
        for fmt in formats:
            try:
                dt = datetime.strptime(value, fmt)
                if dt.tzinfo is None:
                    dt = dt.replace(tzinfo=timezone.utc)
                return dt.isoformat()
            except ValueError:
                continue
        return value  # Return as-is if no format matches
    
    return None


def normalize_event(raw: Dict[str, Any], source_file: str) -> Dict[str, Any]:
    """Normalize a raw log event to ECS-like format."""
    event = {
        "@timestamp": None,
        "event": {
            "kind": "event",
            "category": [],
            "type": [],
            "original": json.dumps(raw),
        },
        "source": {
            "file": source_file,
        },
        "labels": {},
    }
    
    # Extract timestamp
    for ts_field in ["@timestamp", "timestamp", "time", "ts", "datetime", "date"]:
        if ts_field in raw:
            event["@timestamp"] = normalize_timestamp(raw[ts_field])
            break
    
    if event["@timestamp"] is None:
        event["@timestamp"] = datetime.now(timezone.utc).isoformat()
    
    # Extract message
    for msg_field in ["message", "msg", "log", "text", "content"]:
        if msg_field in raw:
            event["message"] = str(raw[msg_field])
            break
    
    # Extract log level
    for level_field in ["level", "severity", "log_level", "loglevel"]:
        if level_field in raw:
            level = str(raw[level_field]).lower()
            event["log"] = {"level": level}
            break
    
    # Extract source IP
    for ip_field in ["source_ip", "src_ip", "client_ip", "remote_addr", "ip"]:
        if ip_field in raw:
            event.setdefault("source", {})["ip"] = raw[ip_field]
            break
    
    # Extract destination IP
    for ip_field in ["dest_ip", "dst_ip", "destination_ip", "server_ip"]:
        if ip_field in raw:
            event["destination"] = {"ip": raw[ip_field]}
            break
    
    # Extract user
    for user_field in ["user", "username", "user_name", "uid"]:
        if user_field in raw:
            event["user"] = {"name": str(raw[user_field])}
            break
    
    # Extract host
    for host_field in ["host", "hostname", "server", "node"]:
        if host_field in raw:
            event["host"] = {"name": str(raw[host_field])}
            break
    
    # Extract process info
    for proc_field in ["process", "program", "service", "application"]:
        if proc_field in raw:
            event["process"] = {"name": str(raw[proc_field])}
            break
    
    # Extract event type hints
    message = event.get("message", "").lower()
    if "login" in message or "auth" in message:
        event["event"]["category"].append("authentication")
    if "error" in message or "fail" in message:
        event["event"]["type"].append("error")
    if "success" in message or "accepted" in message:
        event["event"]["type"].append("allowed")
    
    # Copy remaining fields as labels
    known_fields = {
        "@timestamp", "timestamp", "time", "ts", "datetime", "date",
        "message", "msg", "log", "text", "content",
        "level", "severity", "log_level", "loglevel",
        "source_ip", "src_ip", "client_ip", "remote_addr", "ip",
        "dest_ip", "dst_ip", "destination_ip", "server_ip",
        "user", "username", "user_name", "uid",
        "host", "hostname", "server", "node",
        "process", "program", "service", "application",
    }
    for key, value in raw.items():
        if key not in known_fields and isinstance(value, (str, int, float, bool)):
            event["labels"][key] = str(value)
    
    return event


@main_wrapper
def main(tool: Tool):
    """Main entry point."""
    tool.logger.info("Starting JSONL ingestion")
    
    stats = {
        "total_lines": 0,
        "valid_events": 0,
        "invalid_lines": 0,
        "sources": [],
    }
    
    normalized_events = []
    
    # Process all input files
    for input_name in tool.list_inputs():
        tool.logger.info(f"Processing {input_name}")
        stats["sources"].append(input_name)
        
        try:
            text = tool.read_input_text(input_name)
            lines = text.strip().split("\n")
            
            for line_num, line in enumerate(lines, 1):
                stats["total_lines"] += 1
                
                if not line.strip():
                    continue
                
                try:
                    raw = json.loads(line)
                    if not isinstance(raw, dict):
                        stats["invalid_lines"] += 1
                        tool.logger.warn(f"Line {line_num} is not an object", 
                                        file=input_name)
                        continue
                    
                    event = normalize_event(raw, input_name)
                    normalized_events.append(event)
                    stats["valid_events"] += 1
                    
                except json.JSONDecodeError as e:
                    stats["invalid_lines"] += 1
                    tool.logger.warn(f"Invalid JSON at line {line_num}: {e}",
                                    file=input_name)
                    
        except Exception as e:
            tool.logger.error(f"Error processing {input_name}: {e}")
            raise
    
    # Write outputs
    tool.write_output_jsonl("events", normalized_events)
    tool.write_output_json("stats", stats)
    
    tool.logger.info(f"Ingestion complete", 
                    total=stats["total_lines"],
                    valid=stats["valid_events"],
                    invalid=stats["invalid_lines"])
    
    return 0


if __name__ == "__main__":
    sys.exit(main())
