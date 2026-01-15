# RavenForge Build Guide

## Preduslovi

### 1. Instalacija Go (1.22+)
```powershell
# Preuzmi sa: https://go.dev/dl/
# Instaliraj go1.22.windows-amd64.msi ili noviji
# Proveri instalaciju:
go version
```

### 2. Instalacija Docker Desktop
```powershell
# Preuzmi sa: https://www.docker.com/products/docker-desktop/
# Instaliraj Docker Desktop for Windows
# Proveri instalaciju:
docker --version
```

### 3. Python (već instalirano ✓)
```powershell
python --version
```

---

## Build Proces

### Korak 1: Instalacija Go Zavisnosti
```powershell
cd c:\Users\Gregor\Desktop\RavenForge\core
go mod download
go mod verify
```

### Korak 2: Build Binaries

#### Build Daemon (ravenforged)
```powershell
cd c:\Users\Gregor\Desktop\RavenForge\core
go build -o bin/ravenforged.exe ./cmd/ravenforged
```

#### Build CLI (ravenforge)
```powershell
cd c:\Users\Gregor\Desktop\RavenForge\core
go build -o bin/ravenforge.exe ./cmd/ravenforge
```

#### Build Sve Odjednom
```powershell
cd c:\Users\Gregor\Desktop\RavenForge\core
mkdir -p bin
go build -o bin/ravenforged.exe ./cmd/ravenforged
go build -o bin/ravenforge.exe ./cmd/ravenforge
```

### Korak 3: Build Docker Images za Tools

#### Build sve tool images
```powershell
cd c:\Users\Gregor\Desktop\RavenForge

# Ingest tool
docker build -t ravenforge/ingest-jsonlines:1.0.0 ./tools/ingest/ingest-jsonlines

# Detect tool
docker build -t ravenforge/detect-simple-rules:1.0.0 ./tools/detect/detect-simple-rules

# Enrich tool
docker build -t ravenforge/enrich-geoip:1.0.0 ./tools/enrich/enrich-geoip

# Correlate tool
docker build -t ravenforge/correlate-events:1.0.0 ./tools/correlate/correlate-events

# Report tool
docker build -t ravenforge/report-generate:1.0.0 ./tools/report/report-generate

# Triage tool
docker build -t ravenforge/triage-prioritize:1.0.0 ./tools/triage/triage-prioritize
```

---

## Testiranje

### Go Testovi
```powershell
cd c:\Users\Gregor\Desktop\RavenForge\core

# Pokreni sve unit testove
go test ./...

# Pokreni testove sa verbose outputom
go test -v ./...

# Pokreni testove sa coverage
go test -cover ./...

# Pokreni integration testove
go test -v ./test/integration/...
```

### Python Tools Testovi (Lokalno)
```powershell
cd c:\Users\Gregor\Desktop\RavenForge\tools\ingest\ingest-jsonlines

# Testiraj Python syntax
python main.py --help
```

---

## Pokretanje

### 1. Kreiraj Config Fajl
```powershell
cd c:\Users\Gregor\Desktop\RavenForge\core
copy config\ravenforge.example.yaml config\ravenforge.yaml
```

### 2. Pokreni Daemon
```powershell
cd c:\Users\Gregor\Desktop\RavenForge\core
.\bin\ravenforged.exe --config config\ravenforge.yaml
```

### 3. Koristi CLI (u drugom terminalu)
```powershell
cd c:\Users\Gregor\Desktop\RavenForge\core

# Proveri status
.\bin\ravenforge.exe status

# Discover tools
.\bin\ravenforge.exe tool discover ..\tools

# Lista tools
.\bin\ravenforge.exe tool list
```

---

## Brzi Build Script

Možeš kreirati `build.ps1` fajl za brz build:

```powershell
# build.ps1
$ErrorActionPreference = "Stop"

Write-Host "Building RavenForge..." -ForegroundColor Green

# Build Go binaries
Write-Host "`n[1/2] Building Go binaries..." -ForegroundColor Yellow
cd core
go mod download
New-Item -ItemType Directory -Force -Path bin | Out-Null
go build -o bin/ravenforged.exe ./cmd/ravenforged
go build -o bin/ravenforge.exe ./cmd/ravenforge

# Run tests
Write-Host "`n[2/2] Running tests..." -ForegroundColor Yellow
go test ./...

Write-Host "`nBuild completed successfully!" -ForegroundColor Green
Write-Host "Binaries location: core/bin/" -ForegroundColor Cyan
```

Pokretanje: `.\build.ps1`

---

## Troubleshooting

### Problem: "go: command not found"
**Rešenje:** Go nije u PATH-u. Restartuj terminal nakon instalacije ili dodaj Go manually:
```powershell
$env:Path += ";C:\Program Files\Go\bin"
```

### Problem: "docker: command not found"
**Rešenje:** Docker Desktop nije pokrenut. Startuj Docker Desktop aplikaciju.

### Problem: Build errors sa CGO
**Rešenje:** Instaliraj gcc (npr. preko MinGW ili TDM-GCC)

---

## Sledeći Koraci

1. ✅ Instaliraj Go i Docker
2. ✅ Pokreni build process
3. ✅ Testiraj binaries
4. 📖 Pročitaj [ARCHITECTURE.md](docs/ARCHITECTURE.md)
5. 🔧 Pogledaj [TOOL_DEVELOPMENT.md](docs/TOOL_DEVELOPMENT.md)
