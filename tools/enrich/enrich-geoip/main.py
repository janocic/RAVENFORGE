#!/usr/bin/env python3
"""
GeoIP Enrichment Tool

Enriches IP addresses in events with geographic and ASN information.
"""

import json
import ipaddress
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional

sys.path.insert(0, "/app/sdk")

try:
    from rfsdk import Tool, main_wrapper, get_param
except ImportError:
    import os
    sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "libs", "python"))
    from rfsdk import Tool, main_wrapper, get_param

# Try to import maxminddb for local GeoIP database
try:
    import maxminddb
    HAS_MAXMIND = True
except ImportError:
    HAS_MAXMIND = False


class GeoIPEnricher:
    """GeoIP enrichment using MaxMind database or fallback API."""
    
    def __init__(self, db_path: Optional[str] = None, use_api_fallback: bool = False):
        self.db = None
        self.use_api = use_api_fallback
        self.cache: Dict[str, Dict] = {}
        
        if db_path and HAS_MAXMIND:
            try:
                self.db = maxminddb.open_database(db_path)
            except Exception as e:
                print(f"Failed to load GeoIP database: {e}", file=sys.stderr)
    
    def is_private_ip(self, ip: str) -> bool:
        """Check if IP is private/reserved."""
        try:
            ip_obj = ipaddress.ip_address(ip)
            return ip_obj.is_private or ip_obj.is_loopback or ip_obj.is_reserved
        except ValueError:
            return True
    
    def lookup(self, ip: str) -> Optional[Dict]:
        """Look up geo information for an IP address."""
        if not ip or self.is_private_ip(ip):
            return None
        
        # Check cache
        if ip in self.cache:
            return self.cache[ip]
        
        result = None
        
        # Try local database first
        if self.db:
            try:
                record = self.db.get(ip)
                if record:
                    result = self._parse_maxmind_record(record)
            except Exception:
                pass
        
        # Fallback to API if enabled (not implemented for security)
        # In production, this would call ip-api.com or similar
        
        if result:
            self.cache[ip] = result
        
        return result
    
    def _parse_maxmind_record(self, record: Dict) -> Dict:
        """Parse MaxMind database record into standard format."""
        geo = {}
        
        # Location
        if "location" in record:
            loc = record["location"]
            geo["location"] = {}
            if "latitude" in loc:
                geo["location"]["lat"] = loc["latitude"]
            if "longitude" in loc:
                geo["location"]["lon"] = loc["longitude"]
            if "time_zone" in loc:
                geo["timezone"] = loc["time_zone"]
        
        # Country
        if "country" in record:
            country = record["country"]
            if "iso_code" in country:
                geo["country_iso_code"] = country["iso_code"]
            if "names" in country and "en" in country["names"]:
                geo["country_name"] = country["names"]["en"]
        
        # City
        if "city" in record:
            city = record["city"]
            if "names" in city and "en" in city["names"]:
                geo["city_name"] = city["names"]["en"]
        
        # Region
        if "subdivisions" in record and record["subdivisions"]:
            sub = record["subdivisions"][0]
            if "iso_code" in sub:
                geo["region_iso_code"] = sub["iso_code"]
            if "names" in sub and "en" in sub["names"]:
                geo["region_name"] = sub["names"]["en"]
        
        # Continent
        if "continent" in record:
            cont = record["continent"]
            if "code" in cont:
                geo["continent_code"] = cont["code"]
            if "names" in cont and "en" in cont["names"]:
                geo["continent_name"] = cont["names"]["en"]
        
        # Postal
        if "postal" in record and "code" in record["postal"]:
            geo["postal_code"] = record["postal"]["code"]
        
        return geo
    
    def close(self):
        """Close the database."""
        if self.db:
            self.db.close()


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


def set_nested_value(obj: Dict, path: str, value: Any):
    """Set a nested value in a dict using dot notation."""
    parts = path.split(".")
    current = obj
    for part in parts[:-1]:
        if part not in current:
            current[part] = {}
        current = current[part]
    current[parts[-1]] = value


@main_wrapper
def main(tool: Tool):
    """Main entry point."""
    tool.logger.info("Starting GeoIP enrichment")
    
    # Get parameters
    ip_fields = get_param("ip_fields", ["source.ip", "destination.ip", "client.ip", "server.ip"])
    include_asn = get_param("include_asn", True)
    include_city = get_param("include_city", True)
    use_fallback = get_param("fallback_api", False)
    
    # Initialize enricher
    db_path = None
    if "geoip_db" in tool.list_inputs():
        db_path = tool.get_input_path("geoip_db")
    
    enricher = GeoIPEnricher(db_path=db_path, use_api_fallback=use_fallback)
    
    stats = {
        "events_processed": 0,
        "events_enriched": 0,
        "ips_looked_up": 0,
        "ips_resolved": 0,
        "ips_private": 0,
        "field_enrichments": {},
    }
    
    enriched_events = []
    
    try:
        # Process events
        events = tool.read_input_jsonl("events")
        
        for event in events:
            stats["events_processed"] += 1
            enriched = False
            
            for ip_field in ip_fields:
                ip = get_nested_value(event, ip_field)
                if not ip:
                    continue
                
                stats["ips_looked_up"] += 1
                
                # Check if private
                if enricher.is_private_ip(ip):
                    stats["ips_private"] += 1
                    continue
                
                # Lookup geo info
                geo = enricher.lookup(ip)
                if geo:
                    stats["ips_resolved"] += 1
                    enriched = True
                    
                    # Determine where to put geo data
                    # e.g., source.ip -> source.geo
                    parts = ip_field.split(".")
                    if len(parts) >= 2 and parts[-1] == "ip":
                        geo_field = ".".join(parts[:-1] + ["geo"])
                    else:
                        geo_field = ip_field + "_geo"
                    
                    # Apply field filtering
                    if not include_city:
                        geo.pop("city_name", None)
                        geo.pop("postal_code", None)
                    
                    set_nested_value(event, geo_field, geo)
                    
                    # Track field enrichments
                    stats["field_enrichments"][ip_field] = stats["field_enrichments"].get(ip_field, 0) + 1
            
            if enriched:
                stats["events_enriched"] += 1
            
            enriched_events.append(event)
        
    finally:
        enricher.close()
    
    # Write outputs
    tool.write_output_jsonl("enriched_events", enriched_events)
    tool.write_output_json("stats", stats)
    
    tool.logger.info(f"Enrichment complete",
                    events=stats["events_processed"],
                    enriched=stats["events_enriched"],
                    resolved=stats["ips_resolved"])
    
    return 0


if __name__ == "__main__":
    sys.exit(main())
