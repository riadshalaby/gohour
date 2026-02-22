package worklog

import "time"

// Entry is the normalized worklog record used across importers and outputs.
type Entry struct {
	ID            int64
	StartDateTime time.Time
	EndDateTime   time.Time
	Billable      int
	Description   string
	Project       string
	Activity      string
	Skill         string
	SourceFormat  string
	SourceMapper  string
	SourceFile    string
}
