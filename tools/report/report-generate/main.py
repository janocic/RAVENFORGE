#!/usr/bin/env python3
"""
Security Report Generation Tool

Generates formatted reports from incidents and detections.
"""

import json
import sys
from datetime import datetime, timezone
from collections import Counter, defaultdict
from typing import Any, Dict, List, Optional

sys.path.insert(0, "/app/sdk")

try:
    from rfsdk import Tool, main_wrapper, get_param
except ImportError:
    import os
    sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "libs", "python"))
    from rfsdk import Tool, main_wrapper, get_param


# Remediation recommendations by rule type
RECOMMENDATIONS = {
    "auth-failure": [
        "Review authentication logs for the affected accounts",
        "Consider implementing account lockout policies",
        "Enable multi-factor authentication",
    ],
    "brute-force": [
        "Block source IP at firewall",
        "Implement rate limiting on authentication endpoints",
        "Force password reset for targeted accounts",
        "Review for successful compromise following attacks",
    ],
    "error-spike": [
        "Review application logs for root cause",
        "Check for resource exhaustion or misconfiguration",
        "Monitor for ongoing service degradation",
    ],
    "suspicious-command": [
        "Isolate affected system for investigation",
        "Capture memory and disk forensics",
        "Review command history and network connections",
        "Check for persistence mechanisms",
    ],
    "privilege-escalation": [
        "Immediately isolate affected system",
        "Disable compromised user accounts",
        "Audit all privileged access",
        "Review sudo and setuid configurations",
        "Engage incident response team",
    ],
    "default": [
        "Review affected systems and logs",
        "Assess potential impact",
        "Document findings for future reference",
    ],
}


def get_recommendations(rule_id: str) -> List[str]:
    """Get recommendations for a rule ID."""
    return RECOMMENDATIONS.get(rule_id, RECOMMENDATIONS["default"])


def severity_to_emoji(severity: str) -> str:
    """Convert severity to emoji for markdown."""
    emojis = {
        "critical": "🔴",
        "high": "🟠",
        "medium": "🟡",
        "low": "🟢",
    }
    return emojis.get(severity.lower(), "⚪")


def format_timestamp(ts_str: str) -> str:
    """Format timestamp for display."""
    try:
        ts = datetime.fromisoformat(ts_str.replace("Z", "+00:00"))
        return ts.strftime("%Y-%m-%d %H:%M:%S UTC")
    except:
        return ts_str


class ReportGenerator:
    """Generates security reports in various formats."""
    
    def __init__(self, title: str, include_recommendations: bool, include_timeline: bool):
        self.title = title
        self.include_recommendations = include_recommendations
        self.include_timeline = include_timeline
        self.incidents: List[Dict] = []
        self.detections: List[Dict] = []
        self.generated_at = datetime.now(timezone.utc)
    
    def add_incidents(self, incidents: List[Dict]):
        """Add incidents to report."""
        self.incidents.extend(incidents)
    
    def add_detections(self, detections: List[Dict]):
        """Add detections to report."""
        self.detections.extend(detections)
    
    def compute_stats(self) -> Dict:
        """Compute statistics for the report."""
        stats = {
            "total_incidents": len(self.incidents),
            "total_detections": len(self.detections),
            "severity_breakdown": Counter(),
            "rules_triggered": Counter(),
            "sources_involved": set(),
            "users_involved": set(),
            "hosts_involved": set(),
        }
        
        for incident in self.incidents:
            inc_data = incident.get("incident", {})
            stats["severity_breakdown"][inc_data.get("severity", "unknown")] += 1
            
            for rule in inc_data.get("rules_triggered", []):
                stats["rules_triggered"][rule] += 1
            
            if "source" in incident and "ip" in incident["source"]:
                stats["sources_involved"].add(incident["source"]["ip"])
            if "user" in incident and "name" in incident["user"]:
                stats["users_involved"].add(incident["user"]["name"])
            if "host" in incident and "name" in incident["host"]:
                stats["hosts_involved"].add(incident["host"]["name"])
        
        # Convert sets to lists for JSON serialization
        stats["sources_involved"] = list(stats["sources_involved"])
        stats["users_involved"] = list(stats["users_involved"])
        stats["hosts_involved"] = list(stats["hosts_involved"])
        stats["severity_breakdown"] = dict(stats["severity_breakdown"])
        stats["rules_triggered"] = dict(stats["rules_triggered"])
        
        return stats
    
    def generate_markdown(self, max_incidents: int) -> str:
        """Generate markdown report."""
        stats = self.compute_stats()
        lines = []
        
        # Header
        lines.append(f"# {self.title}")
        lines.append("")
        lines.append(f"**Generated:** {self.generated_at.strftime('%Y-%m-%d %H:%M:%S UTC')}")
        lines.append("")
        
        # Executive Summary
        lines.append("## Executive Summary")
        lines.append("")
        lines.append(f"This report covers **{stats['total_incidents']} security incidents** ")
        lines.append(f"derived from **{stats['total_detections']} detection events**.")
        lines.append("")
        
        # Severity breakdown
        lines.append("### Severity Distribution")
        lines.append("")
        lines.append("| Severity | Count |")
        lines.append("|----------|-------|")
        for sev in ["critical", "high", "medium", "low"]:
            count = stats["severity_breakdown"].get(sev, 0)
            if count > 0:
                lines.append(f"| {severity_to_emoji(sev)} {sev.capitalize()} | {count} |")
        lines.append("")
        
        # Key findings
        lines.append("### Key Findings")
        lines.append("")
        lines.append(f"- **{len(stats['sources_involved'])}** unique source IPs involved")
        lines.append(f"- **{len(stats['users_involved'])}** user accounts affected")
        lines.append(f"- **{len(stats['hosts_involved'])}** hosts impacted")
        lines.append("")
        
        # Top rules
        if stats["rules_triggered"]:
            lines.append("### Top Detection Rules")
            lines.append("")
            lines.append("| Rule | Triggers |")
            lines.append("|------|----------|")
            for rule, count in sorted(stats["rules_triggered"].items(), 
                                     key=lambda x: x[1], reverse=True)[:10]:
                lines.append(f"| {rule} | {count} |")
            lines.append("")
        
        # Timeline
        if self.include_timeline and self.incidents:
            lines.append("## Incident Timeline")
            lines.append("")
            
            sorted_incidents = sorted(
                self.incidents,
                key=lambda x: x.get("incident", {}).get("first_seen", "")
            )
            
            for inc in sorted_incidents[:max_incidents]:
                inc_data = inc.get("incident", {})
                sev = inc_data.get("severity", "unknown")
                lines.append(f"- {severity_to_emoji(sev)} **{format_timestamp(inc_data.get('first_seen', ''))}** - ")
                lines.append(f"  {inc_data.get('event_count', 0)} events, {sev} severity")
            lines.append("")
        
        # Detailed incidents
        lines.append("## Incident Details")
        lines.append("")
        
        sorted_by_severity = sorted(
            self.incidents,
            key=lambda x: {"critical": 0, "high": 1, "medium": 2, "low": 3}.get(
                x.get("incident", {}).get("severity", "low"), 4
            )
        )
        
        for i, incident in enumerate(sorted_by_severity[:max_incidents]):
            inc_data = incident.get("incident", {})
            sev = inc_data.get("severity", "unknown")
            
            lines.append(f"### {severity_to_emoji(sev)} Incident: {inc_data.get('id', 'unknown')[:8]}")
            lines.append("")
            lines.append(f"- **Severity:** {sev.capitalize()}")
            lines.append(f"- **Risk Score:** {inc_data.get('risk_score', 'N/A')}")
            lines.append(f"- **Event Count:** {inc_data.get('event_count', 0)}")
            lines.append(f"- **First Seen:** {format_timestamp(inc_data.get('first_seen', ''))}")
            lines.append(f"- **Last Seen:** {format_timestamp(inc_data.get('last_seen', ''))}")
            lines.append(f"- **Duration:** {inc_data.get('duration_seconds', 0):.0f} seconds")
            lines.append("")
            
            # Correlation details
            if "source" in incident:
                lines.append(f"**Source IP:** `{incident['source'].get('ip', 'N/A')}`")
            if "user" in incident:
                lines.append(f"**User:** `{incident['user'].get('name', 'N/A')}`")
            if "host" in incident:
                lines.append(f"**Host:** `{incident['host'].get('name', 'N/A')}`")
            lines.append("")
            
            # Rules triggered
            rules = inc_data.get("rules_triggered", [])
            if rules:
                lines.append("**Rules Triggered:**")
                for rule in rules:
                    lines.append(f"- `{rule}`")
                lines.append("")
            
            # Recommendations
            if self.include_recommendations and rules:
                lines.append("**Recommended Actions:**")
                seen_recs = set()
                for rule in rules:
                    for rec in get_recommendations(rule):
                        if rec not in seen_recs:
                            lines.append(f"- {rec}")
                            seen_recs.add(rec)
                lines.append("")
            
            lines.append("---")
            lines.append("")
        
        return "\n".join(lines)
    
    def generate_json(self) -> Dict:
        """Generate JSON report data."""
        stats = self.compute_stats()
        return {
            "title": self.title,
            "generated_at": self.generated_at.isoformat(),
            "statistics": stats,
            "incidents": self.incidents,
        }
    
    def generate_executive_summary(self) -> str:
        """Generate executive summary."""
        stats = self.compute_stats()
        lines = []
        
        lines.append(f"SECURITY EXECUTIVE SUMMARY")
        lines.append(f"Generated: {self.generated_at.strftime('%Y-%m-%d %H:%M:%S UTC')}")
        lines.append("")
        lines.append("=" * 50)
        lines.append("")
        
        critical = stats["severity_breakdown"].get("critical", 0)
        high = stats["severity_breakdown"].get("high", 0)
        
        if critical > 0:
            lines.append(f"⚠️  CRITICAL: {critical} critical severity incidents detected")
            lines.append("")
        
        if high > 0:
            lines.append(f"⚠️  HIGH: {high} high severity incidents detected")
            lines.append("")
        
        lines.append(f"SUMMARY:")
        lines.append(f"  - Total Incidents: {stats['total_incidents']}")
        lines.append(f"  - Affected Users: {len(stats['users_involved'])}")
        lines.append(f"  - Affected Hosts: {len(stats['hosts_involved'])}")
        lines.append(f"  - Unique Sources: {len(stats['sources_involved'])}")
        lines.append("")
        
        if critical > 0 or high > 0:
            lines.append("IMMEDIATE ACTIONS REQUIRED:")
            lines.append("  1. Review critical and high severity incidents")
            lines.append("  2. Initiate incident response procedures")
            lines.append("  3. Consider network containment measures")
        else:
            lines.append("STATUS: No critical or high severity incidents")
        
        return "\n".join(lines)


@main_wrapper
def main(tool: Tool):
    """Main entry point."""
    tool.logger.info("Starting report generation")
    
    # Get parameters
    output_format = get_param("format", "markdown")
    title = get_param("title", "Security Incident Report")
    include_recommendations = get_param("include_recommendations", True)
    include_timeline = get_param("include_timeline", True)
    max_incidents = get_param("max_incidents", 50)
    
    # Initialize generator
    generator = ReportGenerator(
        title=title,
        include_recommendations=include_recommendations,
        include_timeline=include_timeline,
    )
    
    # Load incidents
    incidents = list(tool.read_input_jsonl("incidents"))
    generator.add_incidents(incidents)
    tool.logger.info(f"Loaded {len(incidents)} incidents")
    
    # Load detections if provided
    if "detections" in tool.list_inputs():
        detections = list(tool.read_input_jsonl("detections"))
        generator.add_detections(detections)
        tool.logger.info(f"Loaded {len(detections)} detections")
    
    # Generate reports
    if output_format == "markdown":
        report_content = generator.generate_markdown(max_incidents)
    elif output_format == "html":
        # Simple HTML wrapper around markdown
        md_content = generator.generate_markdown(max_incidents)
        report_content = f"""<!DOCTYPE html>
<html>
<head>
    <title>{title}</title>
    <style>
        body {{ font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; 
               max-width: 1200px; margin: 0 auto; padding: 20px; }}
        table {{ border-collapse: collapse; width: 100%; }}
        th, td {{ border: 1px solid #ddd; padding: 8px; text-align: left; }}
        th {{ background-color: #f2f2f2; }}
        code {{ background-color: #f4f4f4; padding: 2px 6px; border-radius: 3px; }}
        hr {{ border: none; border-top: 1px solid #ddd; margin: 20px 0; }}
    </style>
</head>
<body>
<pre style="white-space: pre-wrap;">{md_content}</pre>
</body>
</html>"""
    else:
        report_content = json.dumps(generator.generate_json(), indent=2)
    
    # Write outputs
    tool.write_output("report", report_content)
    tool.write_output_json("report_json", generator.generate_json())
    tool.write_output("executive_summary", generator.generate_executive_summary())
    
    tool.logger.info("Report generation complete",
                    incidents=len(incidents),
                    format=output_format)
    
    return 0


if __name__ == "__main__":
    sys.exit(main())
