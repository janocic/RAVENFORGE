#!/usr/bin/env python3
"""
Event Correlation Tool

Correlates related events based on common fields and time windows.
"""

import json
import hashlib
import sys
from datetime import datetime, timezone, timedelta
from collections import defaultdict
from typing import Any, Dict, List, Optional, Tuple

sys.path.insert(0, "/app/sdk")

try:
    from rfsdk import Tool, main_wrapper, get_param
except ImportError:
    import os
    sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "libs", "python"))
    from rfsdk import Tool, main_wrapper, get_param


def get_nested_value(obj: Dict, path: str) -> Any:
    """Get a nested value from a dict using dot notation."""
    parts = path.split(".")
    value = obj
    for part in parts:
        if isinstance(value, dict) and part in value:
            value = value[part]
        else:
            return None
    return value


def parse_timestamp(ts_str: str) -> Optional[datetime]:
    """Parse ISO timestamp string."""
    if not ts_str:
        return None
    try:
        # Handle various ISO formats
        ts_str = ts_str.replace("Z", "+00:00")
        return datetime.fromisoformat(ts_str)
    except:
        return None


def generate_correlation_key(event: Dict, fields: List[str]) -> Tuple[str, Dict]:
    """Generate a correlation key from specified fields."""
    key_parts = []
    key_values = {}
    
    for field in fields:
        value = get_nested_value(event, field)
        if value is not None:
            key_parts.append(f"{field}={value}")
            key_values[field] = value
    
    if not key_parts:
        return "", {}
    
    key_str = "|".join(sorted(key_parts))
    return key_str, key_values


def calculate_aggregate_risk(events: List[Dict], method: str) -> int:
    """Calculate aggregate risk score from events."""
    scores = []
    for event in events:
        score = get_nested_value(event, "event.risk_score")
        if score is not None:
            try:
                scores.append(int(score))
            except (ValueError, TypeError):
                pass
    
    if not scores:
        return 50  # Default medium risk
    
    if method == "max":
        return max(scores)
    elif method == "sum":
        return min(100, sum(scores))  # Cap at 100
    elif method == "avg":
        return int(sum(scores) / len(scores))
    else:
        return max(scores)


def get_severity_from_risk(risk_score: int) -> str:
    """Convert risk score to severity label."""
    if risk_score >= 90:
        return "critical"
    elif risk_score >= 70:
        return "high"
    elif risk_score >= 40:
        return "medium"
    else:
        return "low"


def extract_event_summary(event: Dict) -> Dict:
    """Extract key fields from an event for summary."""
    summary = {}
    
    # Copy timestamp
    if "@timestamp" in event:
        summary["timestamp"] = event["@timestamp"]
    
    # Copy detection info
    if "detection" in event:
        summary["rule_id"] = event["detection"].get("rule_id")
        summary["rule_name"] = event["detection"].get("rule_name")
        summary["severity"] = event["detection"].get("severity")
    
    # Copy event kind
    if "event" in event:
        summary["event_kind"] = event["event"].get("kind")
        summary["event_category"] = event["event"].get("category")
    
    # Copy key identifiers
    for field in ["message", "source.ip", "destination.ip", "user.name", "host.name", "process.name"]:
        value = get_nested_value(event, field)
        if value:
            summary[field.replace(".", "_")] = value
    
    return summary


@main_wrapper
def main(tool: Tool):
    """Main entry point."""
    tool.logger.info("Starting event correlation")
    
    # Get parameters
    correlation_fields = get_param("correlation_fields", ["source.ip", "user.name", "host.name"])
    time_window = get_param("time_window", 3600)
    min_events = get_param("min_events", 2)
    risk_aggregation = get_param("risk_aggregation", "max")
    
    # Group events by correlation key
    event_groups: Dict[str, Dict] = {}
    
    stats = {
        "events_processed": 0,
        "events_correlated": 0,
        "incidents_created": 0,
        "uncorrelated_events": 0,
        "correlation_field_hits": defaultdict(int),
    }
    
    # Process events
    events = tool.read_input_jsonl("events")
    
    for event in events:
        stats["events_processed"] += 1
        
        # Generate correlation key
        corr_key, key_values = generate_correlation_key(event, correlation_fields)
        
        if not corr_key:
            stats["uncorrelated_events"] += 1
            continue
        
        # Track which fields contributed
        for field in key_values:
            stats["correlation_field_hits"][field] += 1
        
        # Get event timestamp
        event_ts = parse_timestamp(event.get("@timestamp", ""))
        if not event_ts:
            event_ts = datetime.now(timezone.utc)
        
        # Find or create group
        if corr_key not in event_groups:
            event_groups[corr_key] = {
                "key_values": key_values,
                "events": [],
                "first_seen": event_ts,
                "last_seen": event_ts,
            }
        
        group = event_groups[corr_key]
        
        # Check if within time window of existing events
        if (event_ts - group["last_seen"]).total_seconds() > time_window:
            # Start new group (reset)
            event_groups[corr_key] = {
                "key_values": key_values,
                "events": [event],
                "first_seen": event_ts,
                "last_seen": event_ts,
            }
        else:
            group["events"].append(event)
            if event_ts < group["first_seen"]:
                group["first_seen"] = event_ts
            if event_ts > group["last_seen"]:
                group["last_seen"] = event_ts
    
    # Create incidents from groups meeting threshold
    incidents = []
    timeline = []
    
    for corr_key, group in event_groups.items():
        events_in_group = group["events"]
        
        if len(events_in_group) < min_events:
            stats["uncorrelated_events"] += len(events_in_group)
            continue
        
        stats["events_correlated"] += len(events_in_group)
        stats["incidents_created"] += 1
        
        # Generate incident ID
        incident_id = hashlib.sha256(
            f"{corr_key}:{group['first_seen'].isoformat()}".encode()
        ).hexdigest()[:16]
        
        # Calculate aggregate risk
        risk_score = calculate_aggregate_risk(events_in_group, risk_aggregation)
        severity = get_severity_from_risk(risk_score)
        
        # Collect unique rules triggered
        rules_triggered = {}
        for event in events_in_group:
            if "detection" in event:
                rule_id = event["detection"].get("rule_id")
                if rule_id:
                    rules_triggered[rule_id] = event["detection"].get("rule_name", rule_id)
        
        # Create incident
        incident = {
            "@timestamp": datetime.now(timezone.utc).isoformat(),
            "incident": {
                "id": incident_id,
                "severity": severity,
                "risk_score": risk_score,
                "event_count": len(events_in_group),
                "first_seen": group["first_seen"].isoformat(),
                "last_seen": group["last_seen"].isoformat(),
                "duration_seconds": (group["last_seen"] - group["first_seen"]).total_seconds(),
                "correlation_key": corr_key,
                "rules_triggered": list(rules_triggered.keys()),
            },
            "event": {
                "kind": "signal",
                "category": ["intrusion_detection"],
                "type": ["indicator"],
                "risk_score": risk_score,
            },
        }
        
        # Add correlation field values to incident
        for field, value in group["key_values"].items():
            parts = field.split(".")
            current = incident
            for part in parts[:-1]:
                if part not in current:
                    current[part] = {}
                current = current[part]
            current[parts[-1]] = value
        
        # Add event summaries
        incident["related_events"] = [
            extract_event_summary(e) for e in events_in_group
        ]
        
        incidents.append(incident)
        
        # Add to timeline
        timeline.append({
            "incident_id": incident_id,
            "first_seen": group["first_seen"].isoformat(),
            "last_seen": group["last_seen"].isoformat(),
            "event_count": len(events_in_group),
            "severity": severity,
            "correlation_key": corr_key,
        })
    
    # Sort incidents by risk score
    incidents.sort(key=lambda x: x["incident"]["risk_score"], reverse=True)
    
    # Sort timeline by first_seen
    timeline.sort(key=lambda x: x["first_seen"])
    
    # Write outputs
    tool.write_output_jsonl("incidents", incidents)
    tool.write_output_json("timeline", {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "incident_count": len(incidents),
        "timeline": timeline,
    })
    
    # Convert defaultdict to regular dict for JSON
    stats["correlation_field_hits"] = dict(stats["correlation_field_hits"])
    tool.write_output_json("stats", stats)
    
    tool.logger.info("Correlation complete",
                    events=stats["events_processed"],
                    incidents=stats["incidents_created"])
    
    return 0


if __name__ == "__main__":
    sys.exit(main())
