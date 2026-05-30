# Kako koristiti RavenForge – praktični vodič

## Što RavenForge radi?

RavenForge je automatizirana SOC (Security Operations Center) platforma. Zamjenjuje:

- ručno čitanje logova → automatskom analizom
- kopiranje i lijepljenje između alata → automatiziranim cjevovodom (pipeline)
- ručno pisanje izvješća → automatski generiranim izvješćima
- propuštanje sigurnosnih incidenata → detekcijom u stvarnom vremenu

## Kada ti treba?

### 1) Kada imaš puno logova, a malo vremena

```
Problem: 10 000 linija logova dnevno – ne možeš sve pročitati
Rješenje: RavenForge automatski pronalazi samo važne događaje
```

### 2) Kada moraš brzo reagirati na incident

```
Problem: poslužitelj je napadnut – treba ti brza analiza
Rješenje: cjevovod ti za 2 minute daje kompletno izvješće
```

### 3) Usklađenost (compliance) i revizija (audit)

```
Problem: treba ti potpuno dokumentiran trag svih sigurnosnih događaja
Rješenje: RavenForge sve bilježi uz kriptografski hash-lanac
```

### 4) Kada gradiš vlastiti SOC

```
Problem: komercijalni SIEM alati su skupi (50 000+ USD godišnje)
Rješenje: RavenForge je open-source, besplatan i možeš ga prilagoditi
```

---

## Praktičan primjer – sigurnosno nadziranje

### Scenarij: sigurnost web poslužitelja

Imaš web aplikaciju i dnevno dobivaš:

- 5000+ linija access logova
- 200+ error logova
- 50+ pokušaja neuspjele prijave

Ručno: 2–3 sata dnevno za analizu i svejedno propustiš većinu napada.

Uz RavenForge: 5 minuta da pokreneš postupak i dobiješ detekcije.

---

## Primjer korak po korak

### Korak 1: pokreni daemon (jednom)

```powershell
cd c:\Users\Gregor\Desktop\RavenForge\core
.\bin\ravenforged.exe --config config\ravenforge.yaml --log-format console
```

### Korak 2: analiziraj logove (svaki dan)

```powershell
# U drugom terminalu
cd c:\Users\Gregor\Desktop\RavenForge\core

# Pokreni ingest alat – normalizira sve formate logova
.\bin\ravenforge.exe run ingest-jsonlines -f ..\examples\real-world-logs.jsonl
```

Što se događa:

1. alat učitava JSON logove
2. normalizira ih u standardni format (ECS)
3. validira sva polja
4. izlaz: čisti, standardizirani događaji

### Korak 3: detektiraj napade

```powershell
# Pokreni detekciju s prilagođenim pravilima
.\bin\ravenforge.exe run detect-simple-rules \
  -f normalized-events.jsonl \
  --params '{"rules_file": "..\\examples\\detection-rules.yaml"}'
```

Što detektira:

- brute force napade (npr. 5 pokušaja s 203.0.113.45)
- pokušaje SQL injectiona
- directory traversal
- pokušaje XSS-a
- command injection

### Korak 4: obogati podatke

```powershell
# Dodaj geo informacije za IP adrese napadača
.\bin\ravenforge.exe run enrich-geoip -f detections.jsonl
```

Izlaz:

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

### Korak 5: korelacija i incident

```powershell
# Grupiraj povezane napade u incidente
.\bin\ravenforge.exe run correlate-events -f enriched-alerts.jsonl
```

Rezultat:

```
Incident #1: kampanja napada na web aplikaciju
- 4 napada s 198.51.100.23
- SQL injection + XSS + Directory traversal + Command injection
- Trajanje: 90 sekundi
- Ozbiljnost: KRITIČNO
```

### Korak 6: odredi prioritete

```powershell
# Dodijeli prioritete (P1 = hitno, P4 = nisko)
.\bin\ravenforge.exe run triage-prioritize -f incidents.jsonl
```

### Korak 7: generiraj izvješće

```powershell
# Kreiraj izvješće za menadžment
.\bin\ravenforge.exe run report-generate -f prioritized-incidents.jsonl
```

Izlaz: report.md

```markdown
# Izvješće o sigurnosnom incidentu
Datum: 2026-01-15

## Sažetak za rukovodstvo
Otkrivena su 2 sigurnosna incidenta:
- 1 kritični: koordinirani napad na web (P1)
- 1 visoki: brute force napad (P2)

## Incident #1 – napad na web aplikaciju (KRITIČNO)
- Napadač: 198.51.100.23
- Vrste napada: SQL injection, XSS, directory traversal, command injection
- Trajanje: 90 sekundi
- Preporuka: odmah blokirati IP i zakrpati aplikaciju

## Incident #2 – brute force napad (VISOKO)
- Napadač: 203.0.113.45
- 5+ neuspjelih pokušaja prijave
- Preporuka: uvesti rate limiting
```

---

## Automatiziraj s cjevovodom (pipeline)

Umjesto da pokrećeš svaki alat ručno, napravi cjevovod:

security-pipeline.yaml:

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

Pokretanje:

```powershell
.\bin\ravenforge.exe pipeline run security-pipeline.yaml \
  --input logs=todays-logs.jsonl
```

Jedna naredba → kompletan sigurnosni pregled.

---

## Kada koristiti koji alat?

| Alat | Kada koristiti | Primjer |
|------|----------------|--------|
| ingest-jsonlines | Imaš JSON/JSONL logove | Nginx, Apache, aplikacijski logovi |
| detect-simple-rules | Trebaju ti prilagođena pravila detekcije | tvoji specifični napadi |
| enrich-geoip | Želiš znati odakle dolaze napadi | IP → država/grad |
| correlate-events | Želiš vidjeti cijeli incident, ne pojedinačne alarme | 10 alarma → 1 incident |
| triage-prioritize | Imaš 50 incidenata i ne znaš što je najvažnije | automatsko prioritiziranje |
| report-generate | Moraš izvijestiti šefa/klijenta | sažetak za rukovodstvo |

---

## Primjeri iz stvarnog svijeta

### 1) Startup s jednim DevOpsom

```
Problem: nemaš dedicated sigurnosni tim
Rješenje: RavenForge automatizira 90% SOC poslova
Cijena: 0 USD (open-source)
```

### 2) Tvrtka sa zahtjevima usklađenosti

```
Problem: PCI-DSS/GDPR/ISO27001 traže audit trail
Rješenje: RavenForge sve bilježi uz kriptografski lanac
```

### 3) Sigurnosni konzultant

```
Problem: analiziraš infrastrukturu različitih klijenata
Rješenje: isti alati, prilagođena pravila po klijentu
```

### 4) Pentester

```
Problem: nakon testa moraš napisati detaljno izvješće
Rješenje: ubaci napadačke logove → automatski generirano izvješće
```

### 5) SOC analitičar

```
Problem: 500 alarma dnevno, 90% lažno pozitivnih
Rješenje: korelacija smanjuje na 20 stvarnih incidenata
```

---

## Kako prilagoditi za sebe?

### Napravi vlastiti alat

```bash
.\bin\ravenforge.exe tool scaffold my-custom-detector --category detect
```

### Dodaj vlastita pravila

Uredi `detection-rules.yaml` sa svojim pravilima:

```yaml
- id: my-attack-pattern
  name: Moj specifičan obrazac napada
  severity: high
  conditions:
    - field: url
      regex: "/admin/backdoor"
```

### Integriraj s postojećim sustavima

- Ulaz: SIEM izvozi, syslog, CloudWatch logovi
- Izlaz: Slack obavijesti, Jira zadaci, e-mail upozorenja

---

## Prednosti

- Vrijeme: 3 sata ručnog rada → 5 minuta automatizirano
- Detekcija: 60% ručno → 99% automatski
- Trošak: 50 000 USD SIEM licenca → 0 USD open-source
- Prilagodba: zatvoreni vendor alati → puna kontrola
- Skaliranje: 1000 događaja/dan ručno → 1 000 000 događaja/dan automatizirano

---

## Zaključak

RavenForge je za tebe ako:

- analiziraš server/aplikacijske logove
- radiš u sigurnosti / DevSecOps-u
- trebaš audit trag za usklađenost
- želiš automatizirati SOC operacije
- gradiš prilagođeno sigurnosno nadziranje

Nije ti potreban ako:

- nemaš logove za analizu
- već imaš skupi SIEM koji ti odgovara
- ne radiš ništa sa sigurnosnim nadziranjem

---

## Sljedeći koraci

1. Testiraj sa svojim logovima: zamijeni `real-world-logs.jsonl` svojim logovima
2. Prilagodi pravila: uredi `detection-rules.yaml`
3. Napravi cjevovod: za svoj dnevni workflow
4. Automatiziraj: pokreni kao cron job / scheduled task
5. Dodaj obavijesti: integriraj sa Slackom / e-mailom

Pitanja? Pogledaj [ARCHITECTURE.md](../docs/ARCHITECTURE.md) i [TOOL_DEVELOPMENT.md](../docs/TOOL_DEVELOPMENT.md)
