#!/usr/bin/env python3
"""
Simple Rule-Based Detection Tool

Matches normalized events against YAML-defined rules.
"""

import json
import re
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional
import hashlib

sys.path.insert(0, "/app/sdk")

try:
    from rfsdk import Tool, main_wrapper, get_param
except ImportError:
    import os
    sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "libs", "python"))
    from rfsdk import Tool, main_wrapper, get_param

# Built-in detection rules
DEFAULT_RULES = [
    {
        "id": "auth-failure",
        "name": "Authentication Failure",
        "description": "Detects authentication failures",
        "severity": "medium",
        "conditions": [
            {"field": "event.category", "contains": "authentication"},
            {"field": "message", "regex": "(?i)(fail|denied|invalid|unauthorized)"}
        ],
        "match_all": True,
    },
    {
        "id": "brute-force",
        "name": "Potential Brute Force",
        "description": "Multiple auth failures from same source",
        "severity": "high",
        "conditions": [
            {"field": "event.category", "contains": "authentication"},
            {"field": "message", "regex": "(?i)fail"}
        ],
        "threshold": {
            "count": 5,
            "window": 300,
            "group_by": "source.ip"
        },
        "match_all": True,
    },
    {
        "id": "error-spike",
        "name": "Error Log Spike",
        "description": "High volume of error logs",
        "severity": "medium",
        "conditions": [
            {"field": "log.level", "equals": "error"}
        ],
        "threshold": {
            "count": 10,
            "window": 60,
        },
        "match_all": True,
    },
    {
        "id": "suspicious-command",
        "name": "Suspicious Command Execution",
        "description": "Potentially malicious command detected",
        "severity": "high",
        "conditions": [
            {"field": "message", "regex": "(?i)(curl.*\\|.*sh|wget.*\\|.*bash|nc\\s+-e|reverse.?shell)"}
        ],
        "match_all": False,
    },
    {
        "id": "privilege-escalation",
        "name": "Privilege Escalation Attempt",
        "description": "Potential privilege escalation detected",
        "severity": "critical",
        "conditions": [
            {"field": "message", "regex": "(?i)(sudo|su\\s+-|setuid|chmod.*\\+s|/etc/shadow)"}
        ],
        "match_all": False,
    },
]


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


def check_condition(event: Dict, condition: Dict) -> bool:
    """Check if an event matches a single condition."""
    field = condition.get("field", "")
    value = get_nested_value(event, field)
    
    if value is None:
        return False
    
    # Handle list values
    if isinstance(value, list):
        values = value
    else:
        values = [value]
    
    for v in values:
        v_str = str(v)
        
        if "equals" in condition:
            if v_str.lower() == str(condition["equals"]).lower():
                return True
        
        if "contains" in condition:
            if condition["contains"].lower() in v_str.lower():
                return True
        
        if "regex" in condition:
            if re.search(condition["regex"], v_str):
                return True
        
        if "gt" in condition:
            try:
                if float(v) > float(condition["gt"]):
                    return True
            except (ValueError, TypeError):
                pass
        
        if "lt" in condition:
            try:
                if float(v) < float(condition["lt"]):
                    return True
            except (ValueError, TypeError):
                pass
    
    return False


def check_rule(event: Dict, rule: Dict) -> bool:
    """Check if an event matches a rule (excluding thresholds)."""
    conditions = rule.get("conditions", [])
    match_all = rule.get("match_all", True)
    
    if not conditions:
        return False
    
    results = [check_condition(event, c) for c in conditions]
    
    if match_all:
        return all(results)
    else:
        return any(results)


def create_detection(event: Dict, rule: Dict, correlation_id: Optional[str] = None) -> Dict:
    """Create a detection alert from a matched event and rule."""
    detection = {
        "@timestamp": datetime.now(timezone.utc).isoformat(),
        "detection": {
            "rule_id": rule["id"],
            "rule_name": rule["name"],
            "description": rule.get("description", ""),
            "severity": rule.get("severity", "medium"),
        },
        "event": {
            "kind": "alert",
            "category": ["intrusion_detection"],
            "type": ["indicator"],
            "risk_score": severity_to_score(rule.get("severity", "medium")),
        },
        "source_event": event,
    }
    
    if correlation_id:
        detection["detection"]["correlation_id"] = correlation_id
    
    # Copy relevant fields from source event
    for field in ["source", "destination", "user", "host", "process"]:
        if field in event:
            detection[field] = event[field]
    
    return detection


def severity_to_score(severity: str) -> int:
    """Convert severity to numeric score."""
    scores = {
        "low": 25,
        "medium": 50,
        "high": 75,
        "critical": 100,
    }
    return scores.get(severity.lower(), 50)


@main_wrapper
def main(tool: Tool):
    """Main entry point."""
    tool.logger.info("Starting rule-based detection")
    
    # Load rules
    rules = DEFAULT_RULES.copy()
    if "rules" in tool.list_inputs():
        try:
            import yaml
            custom_rules = yaml.safe_load(tool.read_input_text("rules"))
            if isinstance(custom_rules, list):
                rules.extend(custom_rules)
                tool.logger.info(f"Loaded {len(custom_rules)} custom rules")
        except Exception as e:
            tool.logger.warn(f"Failed to load custom rules: {e}")
    
    stats = {
        "events_processed": 0,
        "detections": 0,
        "rules_matched": {},
        "severity_breakdown": {"low": 0, "medium": 0, "high": 0, "critical": 0},
    }
    
    detections = []
    
    # Track events for threshold rules
    threshold_windows: Dict[str, List[Dict]] = {}
    
    # Process events
    events = tool.read_input_jsonl("events")
    
    for event in events:
        stats["events_processed"] += 1
        
        for rule in rules:
            if check_rule(event, rule):
                # Check threshold if defined
                threshold = rule.get("threshold")
                if threshold:
                    rule_id = rule["id"]
                    group_by = threshold.get("group_by")
                    
                    # Generate group key
                    if group_by:
                        group_value = get_nested_value(event, group_by) or "unknown"
                        window_key = f"{rule_id}:{group_value}"
                    else:
                        window_key = rule_id
                    
                    # Get event timestamp
                    event_ts = event.get("@timestamp", "")
                    try:
                        event_time = datetime.fromisoformat(event_ts.replace("Z", "+00:00"))
                    except:
                        event_time = datetime.now(timezone.utc)
                    
                    # Add to window
                    if window_key not in threshold_windows:
                        threshold_windows[window_key] = []
                    threshold_windows[window_key].append({
                        "time": event_time,
                        "event": event,
                    })
                    
                    # Clean old events from window
                    window_seconds = threshold["window"]
                    cutoff = event_time.timestamp() - window_seconds
                    threshold_windows[window_key] = [
                        e for e in threshold_windows[window_key]
                        if e["time"].timestamp() > cutoff
                    ]
                    
                    # Check if threshold exceeded
                    if len(threshold_windows[window_key]) >= threshold["count"]:
                        # Generate correlation ID for this group
                        corr_id = hashlib.sha256(window_key.encode()).hexdigest()[:16]
                        
                        # Create detection for the triggering event
                        detection = create_detection(event, rule, corr_id)
                        detection["detection"]["threshold_count"] = len(threshold_windows[window_key])
                        detections.append(detection)
                        
                        stats["detections"] += 1
                        stats["rules_matched"][rule["id"]] = stats["rules_matched"].get(rule["id"], 0) + 1
                        stats["severity_breakdown"][rule.get("severity", "medium")] += 1
                        
                        # Clear window to avoid duplicate alerts
                        threshold_windows[window_key] = []
                else:
                    # No threshold, create detection immediately
                    detection = create_detection(event, rule)
                    detections.append(detection)
                    
                    stats["detections"] += 1
                    stats["rules_matched"][rule["id"]] = stats["rules_matched"].get(rule["id"], 0) + 1
                    stats["severity_breakdown"][rule.get("severity", "medium")] += 1
    
    # Write outputs
    tool.write_output_jsonl("detections", detections)
    tool.write_output_json("stats", stats)
    
    tool.logger.info(f"Detection complete",
                    events=stats["events_processed"],
                    detections=stats["detections"])
    
    return 0


if __name__ == "__main__":
    sys.exit(main())
