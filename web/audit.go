package web

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type auditRecord struct {
	Timestamp     string   `json:"timestamp"`
	Operation     string   `json:"operation"`
	Scope         string   `json:"scope"`
	Target        string   `json:"target"`
	DryRun        bool     `json:"dryRun,omitempty"`
	Submitted     int      `json:"submitted,omitempty"`
	Duplicates    int      `json:"duplicates,omitempty"`
	Overlaps      int      `json:"overlaps,omitempty"`
	Deleted       int      `json:"deleted,omitempty"`
	SkippedLocked int      `json:"skippedLocked,omitempty"`
	LockedDays    []string `json:"lockedDays,omitempty"`
	Outcome       string   `json:"outcome"`
	Error         string   `json:"error,omitempty"`
}

type auditLogger interface {
	Log(record auditRecord) error
}

type fileAuditLogger struct {
	path string
	mu   sync.Mutex
}

func newFileAuditLogger(path string) auditLogger {
	return &fileAuditLogger{path: path}
}

func defaultAuditLogPath() string {
	return "gohour-audit.log"
}

func (l *fileAuditLogger) Log(record auditRecord) error {
	record.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	file, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

func (s *Server) logAudit(record auditRecord) {
	if s == nil || s.audit == nil {
		return
	}
	_ = s.audit.Log(record)
}
