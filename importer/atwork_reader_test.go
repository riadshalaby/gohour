package importer

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// writeUTF16LEFile creates a temporary UTF-16LE file with BOM from the given
// UTF-8 content string. Returns the path to the file.
func writeUTF16LEFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)

	runes := []rune(content)
	buf := make([]byte, 0, 2+len(runes)*2)
	// BOM (little-endian)
	buf = append(buf, 0xFF, 0xFE)
	for _, r := range runes {
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], uint16(r))
		buf = append(buf, b[:]...)
	}

	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestATWorkReader_HappyPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	content := "Einträge\t\t\t\t\t\t\t\n" +
		"#\tBeginn\tEnde\tDauer\tKunde\tProjekt\tAufgabe\tNotiz\n" +
		"1\t03.03.2026 08:30\t03.03.2026 10:00\t1,5\tVirtual7\tIntern\tTravel\tFake commute\n" +
		"2\t03.03.2026 10:15\t03.03.2026 12:00\t1,75\tVirtual7\tRole Activity\tAllgemein\tPlanning\n" +
		"Gesamt\t\t\t3,25\t\t\t\t\n"

	path := writeUTF16LEFile(t, dir, "atwork.csv", content)

	reader := &ATWorkReader{}
	records, err := reader.Read(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	// Verify first record values
	r1 := records[0]
	if got := r1.Get("Beginn"); got != "03.03.2026 08:30" {
		t.Errorf("record 1 Beginn = %q, want %q", got, "03.03.2026 08:30")
	}
	if got := r1.Get("Projekt"); got != "Intern" {
		t.Errorf("record 1 Projekt = %q, want %q", got, "Intern")
	}
	if got := r1.Get("Notiz"); got != "Fake commute" {
		t.Errorf("record 1 Notiz = %q, want %q", got, "Fake commute")
	}
	if got := r1.Get("Dauer"); got != "1,5" {
		t.Errorf("record 1 Dauer = %q, want %q", got, "1,5")
	}

	// Verify second record
	r2 := records[1]
	if got := r2.Get("Aufgabe"); got != "Allgemein" {
		t.Errorf("record 2 Aufgabe = %q, want %q", got, "Allgemein")
	}
}

func TestATWorkReader_StopsAtEmptyFirstColumn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	content := "Einträge\n" +
		"#\tBeginn\tEnde\tDauer\n" +
		"1\t03.03.2026 08:30\t03.03.2026 10:00\t1,5\n" +
		"\t\t\t\n" +
		"Kunden\n"

	path := writeUTF16LEFile(t, dir, "atwork.csv", content)

	reader := &ATWorkReader{}
	records, err := reader.Read(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record (stop at empty line), got %d", len(records))
	}
}

func TestATWorkReader_EmptyEntriesSection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	content := "Einträge\n" +
		"#\tBeginn\tEnde\tDauer\n" +
		"Gesamt\t\t\t0\n"

	path := writeUTF16LEFile(t, dir, "atwork.csv", content)

	reader := &ATWorkReader{}
	records, err := reader.Read(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 0 {
		t.Fatalf("expected 0 records, got %d", len(records))
	}
}

func TestATWorkReader_RowNumbers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	content := "Einträge\n" +
		"#\tBeginn\n" +
		"1\t03.03.2026 08:30\n" +
		"2\t03.03.2026 10:15\n" +
		"Gesamt\n"

	path := writeUTF16LEFile(t, dir, "atwork.csv", content)

	reader := &ATWorkReader{}
	records, err := reader.Read(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	// Row 1 = section title, Row 2 = headers, Row 3 = first data row
	if records[0].RowNumber != 3 {
		t.Errorf("first record row number = %d, want 3", records[0].RowNumber)
	}
	if records[1].RowNumber != 4 {
		t.Errorf("second record row number = %d, want 4", records[1].RowNumber)
	}
}
