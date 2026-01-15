#!/usr/bin/env python3
"""
Incident Triage and Prioritization Tool

Assigns priority to incidents based on severity, context, and asset criticality.
"""

import json
import re
import sys
from datetime import datetime, timezone, timedelta
from typing import Any, Dict, List, Optional

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


def matches_pattern(value: str, patterns: List[str]) -> bool:
    """Check if value matches any pattern (case-insensitive, supports wildcards)."""
    if not value or not patterns:
        return False
    
    value_lower = value.lower()
    for pattern in patterns:
        pattern_lower = pattern.lower()
        # Convert wildcard to regex
        if "*" in pattern_lower:
            regex = pattern_lower.replace("*", ".*")
            if re.match(regex, value_lower):
                return True
        elif pattern_lower in value_lower:
            return True
    
    return False


def calculate_priority_score(incident: Dict, 
                            critical_assets: List[str],
                            critical_users: List[str],
                            asset_inventory: Dict) -> Dict:
    """
    Calculate priority score and level for an incident.
    
    Scoring factors:
    - Base severity (critical=40, high=30, medium=20, low=10)
    - Critical asset (+20)
    - Critical user (+15)
    - High event count (+10)
    - Recent activity (+5)
    - Multiple rules triggered (+5)
    """
    score = 0
    factors = []
    
    inc_data = incident.get("incident", {})
    
    # Base severity score
    severity = inc_data.get("severity", "medium")
    severity_scores = {
        "critical": 40,
        "high": 30,
        "medium": 20,
        "low": 10,
    }
    base_score = severity_scores.get(severity, 20)
    score += base_score
    factors.append(f"base_severity:{base_score}")
    
    # Critical asset check
    host = get_nested_value(incident, "host.name")
    source_ip = get_nested_value(incident, "source.ip")
    
    is_critical_asset = False
    if host and matches_pattern(host, critical_assets):
        is_critical_asset = True
    if source_ip and matches_pattern(source_ip, critical_assets):
        is_critical_asset = True
    
    # Check asset inventory
    if asset_inventory and host:
        asset_info = asset_inventory.get(host, {})
        if asset_info.get("criticality") in ["critical", "high"]:
            is_critical_asset = True
    
    if is_critical_asset:
        score += 20
        factors.append("critical_asset:+20")
    
    # Critical user check
    user = get_nested_value(incident, "user.name")
    if user and matches_pattern(user, critical_users):
        score += 15
        factors.append("critical_user:+15")
    
    # Event count factor
    event_count = inc_data.get("event_count", 0)
    if event_count >= 10:
        score += 10
        factors.append("high_event_count:+10")
    elif event_count >= 5:
        score += 5
        factors.append("moderate_event_count:+5")
    
    # Recency factor
    last_seen = inc_data.get("last_seen", "")
    if last_seen:
        try:
            last_ts = datetime.fromisoformat(last_seen.replace("Z", "+00:00"))
            age = datetime.now(timezone.utc) - last_ts
            if age < timedelta(hours=1):
                score += 5
                factors.append("recent_activity:+5")
        except:
            pass
    
    # Multiple rules factor
    rules = inc_data.get("rules_triggered", [])
    if len(rules) >= 3:
        score += 5
        factors.append("multiple_rules:+5")
    
    # Determine priority level
    if score >= 70:
        priority = "P1"
        priority_name = "Critical"
    elif score >= 50:
        priority = "P2"
        priority_name = "High"
    elif score >= 30:
        priority = "P3"
        priority_name = "Medium"
    else:
        priority = "P4"
        priority_name = "Low"
    
    return {
        "score": score,
        "priority": priority,
        "priority_name": priority_name,
        "factors": factors,
    }


@main_wrapper
def main(tool: Tool):
    """Main entry point."""
    tool.logger.info("Starting incident triage")
    
    # Get parameters
    critical_assets = get_param("critical_assets", [])
    critical_users = get_param("critical_users", ["admin", "root", "administrator", "service"])
    response_sla = get_param("response_sla", {
        "p1": 15,
        "p2": 60,
        "p3": 240,
        "p4": 1440,
    })
    
    # Load asset inventory if provided
    asset_inventory = {}
    if "asset_inventory" in tool.list_inputs():
        try:
            asset_inventory = tool.read_input_json("asset_inventory")
            tool.logger.info(f"Loaded {len(asset_inventory)} assets from inventory")
        except Exception as e:
            tool.logger.warn(f"Failed to load asset inventory: {e}")
    
    stats = {
        "incidents_processed": 0,
        "priority_breakdown": {"P1": 0, "P2": 0, "P3": 0, "P4": 0},
        "critical_assets_involved": 0,
        "critical_users_involved": 0,
    }
    
    prioritized_incidents = []
    
    # Process incidents
    incidents = tool.read_input_jsonl("incidents")
    
    for incident in incidents:
        stats["incidents_processed"] += 1
        
        # Calculate priority
        priority_info = calculate_priority_score(
            incident,
            critical_assets,
            critical_users,
            asset_inventory,
        )
        
        # Add triage information
        incident["triage"] = {
            "priority": priority_info["priority"],
            "priority_name": priority_info["priority_name"],
            "priority_score": priority_info["score"],
            "scoring_factors": priority_info["factors"],
            "response_sla_minutes": response_sla.get(priority_info["priority"].lower(), 1440),
            "triaged_at": datetime.now(timezone.utc).isoformat(),
        }
        
        prioritized_incidents.append(incident)
        stats["priority_breakdown"][priority_info["priority"]] += 1
        
        # Track critical involvement
        if "critical_asset" in str(priority_info["factors"]):
            stats["critical_assets_involved"] += 1
        if "critical_user" in str(priority_info["factors"]):
            stats["critical_users_involved"] += 1
    
    # Sort by priority score (highest first)
    prioritized_incidents.sort(
        key=lambda x: x["triage"]["priority_score"],
        reverse=True
    )
    
    # Build analyst queue
    queue = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "total_incidents": len(prioritized_incidents),
        "priority_summary": stats["priority_breakdown"],
        "queue": [],
    }
    
    for inc in prioritized_incidents:
        inc_data = inc.get("incident", {})
        triage = inc.get("triage", {})
        
        queue_item = {
            "incident_id": inc_data.get("id", "unknown"),
            "priority": triage.get("priority"),
            "priority_score": triage.get("priority_score"),
            "severity": inc_data.get("severity"),
            "response_sla_minutes": triage.get("response_sla_minutes"),
            "event_count": inc_data.get("event_count", 0),
            "first_seen": inc_data.get("first_seen"),
            "rules_triggered": inc_data.get("rules_triggered", []),
        }
        
        # Add key context
        if "source" in inc:
            queue_item["source_ip"] = inc["source"].get("ip")
        if "user" in inc:
            queue_item["user"] = inc["user"].get("name")
        if "host" in inc:
            queue_item["host"] = inc["host"].get("name")
        
        queue["queue"].append(queue_item)
    
    # Write outputs
    tool.write_output_jsonl("prioritized", prioritized_incidents)
    tool.write_output_json("queue", queue)
    
    tool.logger.info("Triage complete",
                    processed=stats["incidents_processed"],
                    p1=stats["priority_breakdown"]["P1"],
                    p2=stats["priority_breakdown"]["P2"])
    
    return 0


if __name__ == "__main__":
    sys.exit(main())
