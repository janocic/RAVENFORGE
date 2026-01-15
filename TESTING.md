# RavenForge Testing Guide

## Setup Complete! ✅

Daemon i CLI su uspešno build-ovani i konfigurisani.

## Testiranje - Korak po Korak

### 1. Restart Daemon (u prvom terminalu)

Prvo **zaustavi trenutni daemon** (Ctrl+C), pa ponovo pokreni:

```powershell
cd c:\Users\Gregor\Desktop\RavenForge\core
.\bin\ravenforged.exe --config config\ravenforge.yaml --log-format console
```

Sada bi trebao da vidiš: `discovered tools {"count": 6}` umesto 0.

### 2. Build Docker Images za Tools

U **drugom terminalu**:

```powershell
cd c:\Users\Gregor\Desktop\RavenForge
.\build-tools.ps1
```

Ovo će build-ovati Docker images za svih 6 tools (~5-10 min).

### 3. Proveri Registrovane Tools

```powershell
cd c:\Users\Gregor\Desktop\RavenForge\core
.\bin\ravenforge.exe tool list
```

Trebao bi da vidiš listu sa 6 tools:
- ingest-jsonlines
- detect-simple-rules  
- enrich-geoip
- correlate-events
- report-generate
- triage-prioritize

### 4. Testiraj Pojedinačan Tool

#### Test 1: Ingest Tool (JSON logs → Normalized events)

```powershell
# Vidi detalje tool-a
.\bin\ravenforge.exe tool info ingest-jsonlines

# Pokreni tool sa test podacima
.\bin\ravenforge.exe run ingest-jsonlines `
  --input logs=c:\Users\Gregor\Desktop\RavenForge\test-data\sample-logs.jsonl `
  --output-dir c:\Users\Gregor\Desktop\RavenForge\test-output

# Proveri rezultate
Get-Content c:\Users\Gregor\Desktop\RavenForge\test-output\events
Get-Content c:\Users\Gregor\Desktop\RavenForge\test-output\stats
```

#### Test 2: Validate Tool Manifest

```powershell
.\bin\ravenforge.exe tool validate ..\tools\detect\detect-simple-rules\tool.yaml
```

### 5. Proveri Job History

```powershell
# Lista svih job-ova
.\bin\ravenforge.exe job list

# Detalji o specifičnom job-u
.\bin\ravenforge.exe job info <JOB_ID>
```

### 6. Testiraj Pipeline (Kompletan Workflow)

Kreiraj pipeline fajl `test-pipeline.yaml`:

```yaml
name: security-analysis
version: 1.0.0
description: Complete security event analysis pipeline

stages:
  - name: ingest
    tool: ingest-jsonlines
    inputs:
      logs: ${input.raw_logs}
    outputs:
      events: normalized_events
      
  - name: detect
    tool: detect-simple-rules
    inputs:
      events: ${stage.ingest.events}
    outputs:
      alerts: detected_alerts
      
  - name: correlate
    tool: correlate-events
    inputs:
      alerts: ${stage.detect.alerts}
    outputs:
      incidents: correlated_incidents
      
  - name: triage
    tool: triage-prioritize
    inputs:
      incidents: ${stage.correlate.incidents}
    outputs:
      prioritized: triage_results
      
  - name: report
    tool: report-generate
    inputs:
      incidents: ${stage.triage.prioritized}
    outputs:
      report: final_report
```

Pokreni pipeline:

```powershell
.\bin\ravenforge.exe pipeline run test-pipeline.yaml `
  --input raw_logs=c:\Users\Gregor\Desktop\RavenForge\test-data\sample-logs.jsonl `
  --output-dir c:\Users\Gregor\Desktop\RavenForge\pipeline-output
```

### 7. Proveri Audit Logs

```powershell
# Vidi sve audit events
.\bin\ravenforge.exe audit list

# Filtriraj po tipu
.\bin\ravenforge.exe audit list --type tool.run
```

### 8. Artifact Management

```powershell
# Lista svih artifacts
.\bin\ravenforge.exe artifact list

# Download artifact
.\bin\ravenforge.exe artifact get <ARTIFACT_ID> -o output.jsonl
```

## Troubleshooting

### Problem: "No tools discovered"
**Rešenje:** Proveri da li je `tool_dirs` u config fajlu i restartuj daemon.

### Problem: "Image not found"
**Rešenje:** Pokreni `.\build-tools.ps1` da build-uješ Docker images.

### Problem: Docker error
**Rešenje:** 
1. Proveri da li Docker Desktop radi: `docker ps`
2. Proveri da li images postoje: `docker images ravenforge/*`

### Problem: "Connection refused"
**Rešenje:** Daemon nije pokrenut. Pokreni ga u prvom terminalu.

## Napredne Opcije

### Debug Mode
```powershell
.\bin\ravenforged.exe --config config\ravenforge.yaml --log-level debug
```

### Custom Server URL
```powershell
.\bin\ravenforge.exe --server http://localhost:7433 tool list
```

### Scaffold New Tool
```powershell
.\bin\ravenforge.exe tool scaffold my-custom-tool --category enrich
```

## Dalje Testiranje

1. **Kreiraj custom detection rules** u detect-simple-rules
2. **Test sa većim log fajlovima**
3. **Modifikuj tool.yaml** i testiraj različite konfiguracije
4. **Napravi custom pipeline** za specifične use case-ove
5. **Testiraj policy engine** sa restricted network permissions

---

Srećno testiranje! 🚀
