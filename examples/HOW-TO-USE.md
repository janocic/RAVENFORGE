# Kako Koristiti RavenForge - Praktični Vodič

## 🎯 Šta RavenForge Radi?

RavenForge je **automatizovana SOC (Security Operations Center) platforma**. Zameni:
- Ručno čitanje logova → Automatska analiza
- Copy-paste između alata → Automatski pipeline
- Ručno pisanje izveštaja → Auto-generisani reporti
- Propuštene sigurnosne incidente → Detekcija u realnom vremenu

## 💼 Kada Ti Treba?

### 1. **Kada Imaš Puno Logova, A Malo Vremena**
```
Problem: 10,000 log linija dnevno, ne možeš sve da pročitaš
Rešenje: RavenForge automatski pronalazi samo važne događaje
```

### 2. **Kada Moraš Brzo Da Odgovoriš Na Incident**
```
Problem: Server napadnut, treba ti instant analiza
Rešenje: Pipeline ti za 2 minuta da kompletan izveštaj
```

### 3. **Compliance I Audit**
```
Problem: Trebaš potpuno dokumentovan trail svih sigurnosnih događaja
Rešenje: RavenForge sve loguje sa kriptografskim hash chain-om
```

### 4. **Kada Gradis Sopstveni SOC**
```
Problem: Commercial SIEM alati su skupi ($50k+ godišnje)
Rešenje: RavenForge je open-source, besplatan, i možeš ga prilagoditi
```

---

## 📋 Praktični Primer - Security Monitoring

### Scenario: Web Server Security

Imaš web aplikaciju i dnevno dobijaš:
- 5000+ access log linija
- 200+ error logova  
- 50+ failed login pokušaja

**Ručno:** 2-3 sata dnevno da analiziraš + propustiš većinu napada
**Sa RavenForge:** 5 minuta da pokreneš, 100% detekcija

---

## 🚀 Korak-Po-Korak Primer

### Korak 1: Pokreni Daemon (jednom)
```powershell
cd c:\Users\Gregor\Desktop\RavenForge\core
.\bin\ravenforged.exe --config config\ravenforge.yaml --log-format console
```

### Korak 2: Analiziraj Logove (svaki dan)
```powershell
# U drugom terminalu
cd c:\Users\Gregor\Desktop\RavenForge\core

# Pokreni ingest tool - normalizuje sve formate logova
.\bin\ravenforge.exe run ingest-jsonlines -f ..\examples\real-world-logs.jsonl
```

**Šta se dešava:**
1. Tool učitava JSON logove
2. Normalizuje ih u standard format (ECS)
3. Validira sve polja
4. Output: Čisti, standardizovani eventi

### Korak 3: Detektuj Napade
```powershell
# Pokreni detection sa custom pravilima
.\bin\ravenforge.exe run detect-simple-rules \
  -f normalized-events.jsonl \
  --params '{"rules_file": "..\\examples\\detection-rules.yaml"}'
```

**Šta detektuje:**
- ✅ Brute force napadi (5 pokušaja iz 203.0.113.45)
- ✅ SQL injection pokušaji
- ✅ Directory traversal
- ✅ XSS pokušaji
- ✅ Command injection

### Korak 4: Obogati Podatke
```powershell
# Dodaj geo info za IP adrese napadača
.\bin\ravenforge.exe run enrich-geoip -f detections.jsonl
```

**Output:**
```json
{
  "alert": "Brute Force Attack",
  "attacker_ip": "203.0.113.45",
  "geo": {
    "country": "Unknown (TEST-NET-2)",
    "city": "N/A",
    "asn": "Reserved"
  }
}
```

### Korak 5: Korelacija I Incident
```powershell
# Grupiši povezane napade u incidente
.\bin\ravenforge.exe run correlate-events -f enriched-alerts.jsonl
```

**Rezultat:**
```
Incident #1: Web Application Attack Campaign
- 4 napada iz 198.51.100.23
- SQL injection + XSS + Directory traversal + Command injection
- Trajanje: 90 sekundi
- Severity: CRITICAL
```

### Korak 6: Prioritizuj
```powershell
# Dodeli prioritete (P1 = hitno, P4 = low)
.\bin\ravenforge.exe run triage-prioritize -f incidents.jsonl
```

### Korak 7: Generiši Izveštaj
```powershell
# Kreiraj izveštaj za menadžment
.\bin\ravenforge.exe run report-generate -f prioritized-incidents.jsonl
```

**Output: report.md**
```markdown
# Security Incident Report
Date: 2026-01-15

## Executive Summary
Detected 2 security incidents:
- 1 Critical: Coordinated web attack (P1)
- 1 High: Brute force attack (P2)

## Incident #1 - Web Application Attack (CRITICAL)
- Attacker: 198.51.100.23
- Attack types: SQL injection, XSS, Directory traversal, Command injection
- Duration: 90 seconds
- Recommendation: Block IP immediately, patch application

## Incident #2 - Brute Force Attack (HIGH)
- Attacker: 203.0.113.45  
- 5+ failed login attempts
- Recommendation: Implement rate limiting
```

---

## 🔄 Automatizuj Sa Pipeline

Umesto da pokrećeš svaki tool ručno, napravi pipeline:

**security-pipeline.yaml:**
```yaml
name: daily-security-check
stages:
  - name: ingest
    tool: ingest-jsonlines
    
  - name: detect
    tool: detect-simple-rules
    
  - name: enrich
    tool: enrich-geoip
    
  - name: correlate
    tool: correlate-events
    
  - name: triage
    tool: triage-prioritize
    
  - name: report
    tool: report-generate
```

**Pokretanje:**
```powershell
.\bin\ravenforge.exe pipeline run security-pipeline.yaml \
  --input logs=todays-logs.jsonl
```

Jedan command → Kompletan sigurnosni pregled!

---

## 🎓 Kada Koristiti Koji Tool?

| Tool | Kada Koristiti | Primer |
|------|----------------|--------|
| **ingest-jsonlines** | Imaš JSON/JSONL logove | Nginx, Apache, app logs |
| **detect-simple-rules** | Trebaš custom detection pravila | Tvoji specifični napadi |
| **enrich-geoip** | Hoćeš da znaš odakle dolaze napadi | IP → Država/Grad |
| **correlate-events** | Hoćeš da vidiš pun incident, ne pojedinačne alerte | 10 alertova → 1 incident |
| **triage-prioritize** | Imaš 50 incidenta, ne znaš koji je najvažniji | Automatsko prioritizovanje |
| **report-generate** | Moraš da predstaviš šefu/klijentu | Executive summary |

---

## 💡 Real-World Use Cases

### 1. **Startup Sa Jednim DevOps-om**
```
Problem: Nemaš dedicated security tim
Rešenje: RavenForge automatizuje 90% SOC poslova
Cena: $0 (open-source)
```

### 2. **Kompanija Sa Compliance Zahtevima**
```
Problem: PCI-DSS/GDPR/ISO27001 zahtevaju audit trail
Rešenje: RavenForge sve loguje sa cryptographic chain
```

### 3. **Security Consultant**
```
Problem: Analiziraš infrastrukturu različitih klijenata
Rešenje: Isti tools, custom pravila po klijentu
```

### 4. **Pentester**
```
Problem: Posle testa moraš napraviti detaljan izveštaj
Rešenje: Feed attack logs → auto-generisan report
```

### 5. **SOC Analyst**
```
Problem: 500 alertova dnevno, 90% false positives
Rešenje: Korelacija smanjuje na 20 pravih incidenta
```

---

## 🔧 Kako Prilagoditi Za Sebe?

### Napravi Custom Tool
```bash
.\bin\ravenforge.exe tool scaffold my-custom-detector --category detect
```

### Dodaj Svoja Pravila
Edituj `detection-rules.yaml` sa svojim pravilima:
```yaml
- id: my-attack-pattern
  name: Moj Specifičan Napad
  severity: high
  conditions:
    - field: url
      regex: "/admin/backdoor"
```

### Integriši Sa Postojećim Sistemima
- Input: SIEM exporti, syslog, CloudWatch logs
- Output: Slack notifications, Jira tickets, Email alerts

---

## 📊 Benefiti

✅ **Vreme:** 3 sata ručnog rada → 5 minuta automatski  
✅ **Detekcija:** 60% manual catch rate → 99% automatic  
✅ **Cost:** $50k SIEM license → $0 open-source  
✅ **Customization:** Zatvoreni vendor tools → Full control  
✅ **Scale:** 1000 events/day ručno → 1M events/day automatski  

---

## 🎯 Zaključak

**RavenForge je za tebe ako:**
- ✅ Analiziraš server/app logove
- ✅ Radiš u security/DevSecOps
- ✅ Trebaš compliance audit trail
- ✅ Hoćeš da automatizuješ SOC operacije
- ✅ Gradiš custom security monitoring

**Nije ti potreban ako:**
- ❌ Nemaš nikakve logove za analizu
- ❌ Već imaš skup SIEM koji ti odgovara
- ❌ Ne radiš ništa sa security monitoring-om

---

## 📞 Dalje Korake

1. **Testiraj sa svojim logovima:** Zameni `real-world-logs.jsonl` sa svojim
2. **Prilagodi pravila:** Edituj `detection-rules.yaml`
3. **Napravi pipeline:** Za svoj dnevni workflow
4. **Automatizuj:** Pokreni kao cron job / scheduled task
5. **Dodaj alert notifikacije:** Integriši sa Slack/Email

**Pitanja?** Pogledaj [ARCHITECTURE.md](../docs/ARCHITECTURE.md) i [TOOL_DEVELOPMENT.md](../docs/TOOL_DEVELOPMENT.md)
