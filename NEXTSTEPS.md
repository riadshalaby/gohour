# NEXTSTEPS

All 9 validation bugs from the initial code review have been resolved.

## Recent Additions

- **`atwork` mapper**: New mapper for UTF-16 tab-separated CSV exports from the atwork time-tracking app. Includes a dedicated reader (`ATWorkReader`) that handles encoding conversion, section parsing, and column mapping. Project/Activity/Skill are resolved from rule config (like EPM).
- **`billable` rule flag**: Rules now support an optional `billable` field (default: `true`). When set to `false`, all entries imported via that rule get `Billable=0`. Works with all mappers.

---

## Open Items

### S1. `billable: false` wird beim Submit überschrieben (Bug)

**File:** `cmd/submit.go:418-424`

**Problem:**
Wenn eine Rule `billable: false` gesetzt hat, speichert `importer/service.go` die Einträge mit `Billable=0` in SQLite. In `buildSubmitDayBatches` wird `Billable=0` aber fälschlicherweise als "nicht gesetzt" interpretiert und durch `duration` überschrieben:

```go
billable := entry.Billable
if billable <= 0 {
    billable = duration  // ← überschreibt absichtlich gesetzte 0
}
```

Das Feature `billable: false` hat damit keinen Effekt beim Submit.

**Fix:**
Den Fallback entfernen. Alle Mapper setzen `entry.Billable > 0` für fakturierbare Einträge;
`importer/service.go` setzt bewusst `Billable=0` wenn `billable: false`. `Billable=0` ist gültig
und bedeutet "nicht fakturierbar" → so an OnePoint senden. Nur negative Werte sind ein Fehler:

```go
billable := entry.Billable
if billable < 0 {
    return nil, fmt.Errorf("worklog id=%d has negative billable value (%d)", entry.ID, billable)
}
```

Die zweite Prüfung `if billable <= 0 { return nil, fmt.Errorf(...) }` ebenfalls entfernen.

---

### S2. Gesamter Tag wird übersprungen wenn irgendein Eintrag gesperrt ist

**Files:** `onepoint/client.go:302-327`, `cmd/submit.go:136-152`

**Problem:**
Aktuell werden gesperrte Einträge (`Locked != 0`) einzeln aus dem Payload gefiltert, der Tag
wird aber weiterhin bearbeitet und neue Einträge werden hinzugefügt. Das gewünschte Verhalten:
Hat ein Tag **irgendeinen** gesperrten Eintrag, wird der gesamte Tag übersprungen.

**Fix in `onepoint/client.go`:**
Einen neuen exportierten Fehlertyp einführen:

```go
// ErrDayLocked is returned when a day has locked entries and must not be modified.
type ErrDayLocked struct {
    Day         time.Time
    LockedCount int
}

func (e *ErrDayLocked) Error() string {
    return fmt.Sprintf("day %s has %d locked entry/entries — skipping",
        FormatDay(e.Day), e.LockedCount)
}
```

In `MergeAndPersistWorklogs`: Nach dem Abrufen der bestehenden Einträge prüfen ob irgendeiner
`Locked != 0` hat. Wenn ja, `*ErrDayLocked` zurückgeben ohne einen Persist-Call zu machen.

**Fix in `cmd/submit.go`:**
Im Submit-Loop den Fehlertyp abfangen und eine informative Meldung ausgeben, dann weitermachen:

```go
results, submitErr := client.MergeAndPersistWorklogs(dayCtx, batch.Day, batch.Worklogs)
var lockedErr *onepoint.ErrDayLocked
if errors.As(submitErr, &lockedErr) {
    fmt.Printf("Skipping day %s: %s\n", onepoint.FormatDay(batch.Day), lockedErr.Error())
    continue
}
if submitErr != nil {
    return fmt.Errorf("submit day %s failed: %w", onepoint.FormatDay(batch.Day), submitErr)
}
```

**Tests:** Test hinzufügen bei dem ein Tag gemischte Einträge hat (1 locked + 1 unlocked) →
kein Persist-Call darf stattfinden, `ErrDayLocked` wird returned.

---

### S3. Duplikaterkennung nur nach Zeit + Projekt + Aktivität + Skill (kein Kommentar)

**File:** `onepoint/client.go:554-566` (`persistWorklogsEquivalent`)

**Problem:**
Die aktuelle Äquivalenzprüfung vergleicht auch `Duration`, `Billable` und `Comment`. Zwei
Einträge sollen als Duplikat gelten wenn **StartTime, FinishTime, ProjectID, ActivityID und
SkillID** übereinstimmen — unabhängig vom Kommentar oder fakturierbaren Minuten.

**Fix:**
`persistWorklogsEquivalent` vereinfachen:

```go
func persistWorklogsEquivalent(a, b PersistWorklog) bool {
    return equalIntPointer(a.StartTime, b.StartTime) &&
        equalIntPointer(a.FinishTime, b.FinishTime) &&
        a.ProjectID.Valid == b.ProjectID.Valid &&
        a.ProjectID.Value == b.ProjectID.Value &&
        a.ActivityID.Valid == b.ActivityID.Valid &&
        a.ActivityID.Value == b.ActivityID.Value &&
        a.SkillID.Valid == b.SkillID.Valid &&
        a.SkillID.Value == b.SkillID.Value
}
```

`Duration` ist redundant (ergibt sich aus FinishTime − StartTime). `Billable` und `Comment`
werden absichtlich ausgeschlossen.

**Tests:**
- Existing test `TestHTTPClient_MergeAndPersistWorklogs_DeduplicatesEquivalentLocalEntries`
  anpassen: Kommentar des lokalen Eintrags absichtlich abweichen lassen → muss trotzdem als
  Duplikat erkannt werden.
- Neuen Test hinzufügen: gleiche Zeit + Projekt + Aktivität + Skill, aber unterschiedlicher
  Billable-Wert → Duplikat.

---

### S4. Interaktive Überlappungs-Erkennung beim Submit

**Files:** `onepoint/client.go`, `cmd/submit.go`

**Problem:**
Wenn ein neuer lokaler Eintrag zeitlich mit einem bestehenden OnePoint-Eintrag überlappt
(aber kein Duplikat nach S3-Kriterien ist), wird der Nutzer derzeit nicht gewarnt.

**Gewünschtes Verhalten:**
- Überlappende Einträge werden erkannt und dem Nutzer interaktiv gemeldet.
- Im `--dry-run`-Modus: OnePoint wird ausgelesen, Überlappungen werden als Warnungen ausgegeben,
  der Dry-Run wird aber nicht gestoppt und fragt nicht interaktiv.
- Gesperrte Tage werden im Dry-Run ebenfalls als Warnung gemeldet.

---

#### 4a. Hilfsfunktionen in `onepoint/client.go`

Neue exportierte Funktion `WorklogTimeOverlaps` hinzufügen:

```go
// WorklogTimeOverlaps reports whether a and b have overlapping time ranges
// but are not duplicates (per persistWorklogsEquivalent).
func WorklogTimeOverlaps(a, b PersistWorklog) bool {
    if persistWorklogsEquivalent(a, b) {
        return false
    }
    if a.StartTime == nil || a.FinishTime == nil || b.StartTime == nil || b.FinishTime == nil {
        return false
    }
    return *a.StartTime < *b.FinishTime && *b.StartTime < *a.FinishTime
}
```

Neuen exportierten Typ `OverlapInfo` hinzufügen:

```go
type OverlapInfo struct {
    Local    PersistWorklog
    Existing PersistWorklog
}
```

---

#### 4b. Submit-Loop in `cmd/submit.go` umstrukturieren

Die `MergeAndPersistWorklogs`-Methode enthält aktuell zu viel Logik (Fetch + Merge + Persist).
Den Submit-Loop so umschreiben, dass die Schritte getrennt ablaufen. Neuer Ablauf pro Tag:

```
existing, err := client.GetDayWorklogs(ctx, day)

// 1. Gesperrter Tag?
lockedCount := count entries where Locked != 0
if lockedCount > 0:
    print "Skipping day %s: %d locked entry/entries found — no changes made"
    continue

// 2. Unlocked bestehende Einträge als Basis-Payload
existingPayload := [item.ToPersistWorklog() for item in existing]

// 3. Lokale Einträge klassifizieren
toAdd := []
overlaps := []OverlapInfo{}
for each localEntry in batch.Worklogs:
    if containsEquivalentPersistWorklog(existingPayload, localEntry):
        continue  // Duplikat → still skip
    hasOverlap := false
    for each existingEntry in existingPayload:
        if WorklogTimeOverlaps(localEntry, existingEntry):
            overlaps = append(overlaps, OverlapInfo{Local: localEntry, Existing: existingEntry})
            hasOverlap = true
    if !hasOverlap:
        toAdd = append(toAdd, localEntry)

// 4. Overlaps behandeln
approvedOverlaps := handleOverlaps(overlaps, dryRun, &globalSkipAll, &globalWriteAll)
toAdd = append(toAdd, approvedOverlaps...)

if dryRun:
    continue  // kein Persist

payload := append(existingPayload, toAdd...)
client.PersistWorklogs(ctx, day, payload)
```

---

#### 4c. Interaktiver Prompt `handleOverlaps` in `cmd/submit.go`

Neue interne Funktion:

```go
func handleOverlaps(
    overlaps []onepoint.OverlapInfo,
    dryRun bool,
    globalSkipAll *bool,
    globalWriteAll *bool,
) []onepoint.PersistWorklog
```

**Dry-run Verhalten:** Für jeden Overlap eine Warnung ausgeben, alle überspringen, kein Prompt:
```
Warning: local entry 09:00-10:00 (ProjectID=123) overlaps with existing 09:30-10:30
```

**Interaktives Verhalten (nicht dry-run):**

Wenn `*globalSkipAll == true` → alle verwerfen, keine Frage.
Wenn `*globalWriteAll == true` → alle akzeptieren, keine Frage.

Andernfalls Überlappungen für diesen Tag auflisten und fragen:

```
Warning: 2 local entries overlap with existing OnePoint entries for 05-03-2026:
  [1] 09:00-10:00 "Task A"  overlaps with existing  09:30-10:30 "Existing task"
  [2] 11:00-12:00 "Task B"  overlaps with existing  11:15-12:15 "Another task"

How to handle overlapping entries?
  (w) Write overlapping entries anyway
  (s) Skip overlapping entries
  (W) Write ALL overlapping entries for all remaining days
  (S) Skip ALL overlapping entries for all remaining days
  (a) Abort submit
Enter choice:
```

Gültige Eingaben: `w` / `s` / `W` / `S` / `a`. Bei ungültiger Eingabe erneut fragen.
Bei `a` → `fmt.Errorf("submit aborted by user")` aus dem Caller zurückgeben.
Bei `W` → `*globalWriteAll = true` setzen.
Bei `S` → `*globalSkipAll = true` setzen.

---

#### 4d. `--dry-run` Anpassung

Aktuell bricht `--dry-run` vor dem OnePoint-Kontakt ab. Neues Verhalten:

1. ID-Auflösung wie bisher (OnePoint-API für Projektnamen → IDs).
2. **NEU**: Für jeden Tag `GetDayWorklogs` aufrufen.
3. Gesperrte Tage und Overlaps als Warnungen ausgeben (kein Prompt).
4. Kein Persist-Call.
5. Zusammenfassung am Ende:

```
Dry-run summary:
  Days to submit:               5
  Days skipped (locked):        1  [05-03-2026]
  Local entries prepared:      12
  Duplicates (skipped):         2
  Overlapping entries (warned): 1
```

---

#### 4e. `MergeAndPersistWorklogs` entfernen

Diese Methode wird durch die neue Logik in `cmd/submit.go` ersetzt. Sie soll aus dem
`Client`-Interface und der `HTTPClient`-Implementierung entfernt werden.
`GetDayWorklogs` und `PersistWorklogs` bleiben erhalten und werden direkt von `cmd/submit.go`
verwendet.

---

### Prioritätsreihenfolge

1. **S1** — `billable: false` Bug (einfach, 1 Zeile)
2. **S3** — Duplikaterkennung vereinfachen (einfach, klare Spec)
3. **S2** — Locked-Day-Handling (medium, neuer Error-Typ + Tests)
4. **S4** — Overlap-Erkennung + interaktiver Prompt (komplex, Refactoring + Tests)
