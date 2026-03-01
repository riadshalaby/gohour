package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"gohour/worklog"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

var ErrWorklogNotFound = errors.New("worklog not found")

func OpenSQLite(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite db: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.ensureSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) ensureSchema() error {
	// NOTE: billable changed from CHECK(billable > 0) to CHECK(billable >= 0).
	// Existing databases are not auto-migrated; delete gohour.db and re-import
	// source files to apply the new constraint and preserve correct dedup behavior.
	const schema = `
CREATE TABLE IF NOT EXISTS worklogs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	start_datetime TEXT NOT NULL,
	end_datetime TEXT NOT NULL,
	billable INTEGER NOT NULL CHECK(billable >= 0),
	description TEXT NOT NULL,
	project TEXT NOT NULL,
	activity TEXT NOT NULL,
	skill TEXT NOT NULL,
	source_format TEXT NOT NULL,
	source_mapper TEXT NOT NULL DEFAULT '',
	source_file TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(start_datetime, end_datetime, billable, description, project, activity, skill, source_file)
);
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	if err := s.ensureSourceMapperColumn(); err != nil {
		return err
	}

	return nil
}

func (s *SQLiteStore) ensureSourceMapperColumn() error {
	rows, err := s.db.Query(`PRAGMA table_info(worklogs);`)
	if err != nil {
		return fmt.Errorf("query table info: %w", err)
	}
	defer rows.Close()

	hasSourceMapper := false
	for rows.Next() {
		var (
			cid       int
			name      string
			colType   string
			notNull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("scan table info: %w", err)
		}
		if strings.EqualFold(name, "source_mapper") {
			hasSourceMapper = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table info: %w", err)
	}

	if hasSourceMapper {
		return nil
	}

	if _, err := s.db.Exec(`ALTER TABLE worklogs ADD COLUMN source_mapper TEXT NOT NULL DEFAULT '';`); err != nil {
		return fmt.Errorf("add source_mapper column: %w", err)
	}

	return nil
}

func (s *SQLiteStore) InsertWorklogs(entries []worklog.Entry) (int, error) {
	if len(entries) == 0 {
		return 0, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}

	const insertStmt = `
INSERT OR IGNORE INTO worklogs (
	start_datetime,
	end_datetime,
	billable,
	description,
	project,
	activity,
	skill,
	source_format,
	source_mapper,
	source_file
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

	stmt, err := tx.Prepare(insertStmt)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("prepare insert statement: %w", err)
	}
	defer stmt.Close()

	inserted := 0
	for _, entry := range entries {
		res, err := stmt.Exec(
			entry.StartDateTime.Format(time.RFC3339),
			entry.EndDateTime.Format(time.RFC3339),
			entry.Billable,
			entry.Description,
			entry.Project,
			entry.Activity,
			entry.Skill,
			entry.SourceFormat,
			entry.SourceMapper,
			entry.SourceFile,
		)
		if err != nil {
			_ = tx.Rollback()
			return inserted, fmt.Errorf("insert worklog: %w", err)
		}

		rows, err := res.RowsAffected()
		if err == nil && rows > 0 {
			inserted++
		}
	}

	if err := tx.Commit(); err != nil {
		return inserted, fmt.Errorf("commit transaction: %w", err)
	}

	return inserted, nil
}

// InsertWorklog inserts one worklog entry and returns the new row ID when inserted.
// The second return value is false when the row is ignored by the UNIQUE constraint.
func (s *SQLiteStore) InsertWorklog(entry worklog.Entry) (int64, bool, error) {
	const insertStmt = `
INSERT OR IGNORE INTO worklogs (
	start_datetime,
	end_datetime,
	billable,
	description,
	project,
	activity,
	skill,
	source_format,
	source_mapper,
	source_file
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

	res, err := s.db.Exec(
		insertStmt,
		entry.StartDateTime.Format(time.RFC3339),
		entry.EndDateTime.Format(time.RFC3339),
		entry.Billable,
		entry.Description,
		entry.Project,
		entry.Activity,
		entry.Skill,
		entry.SourceFormat,
		entry.SourceMapper,
		entry.SourceFile,
	)
	if err != nil {
		return 0, false, fmt.Errorf("insert worklog: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return 0, false, fmt.Errorf("read inserted row count: %w", err)
	}
	if rows == 0 {
		return 0, false, nil
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, false, fmt.Errorf("read inserted row id: %w", err)
	}
	if id <= 0 {
		return 0, false, fmt.Errorf("invalid inserted row id %d", id)
	}
	return id, true, nil
}

func (s *SQLiteStore) ListWorklogs() ([]worklog.Entry, error) {
	const query = `
SELECT
	id,
	start_datetime,
	end_datetime,
	billable,
	description,
	project,
	activity,
	skill,
	source_format,
	source_mapper,
	source_file
FROM worklogs
ORDER BY start_datetime, id;
`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query worklogs: %w", err)
	}
	defer rows.Close()

	entries := make([]worklog.Entry, 0, 256)
	for rows.Next() {
		var (
			id       int64
			startRaw string
			endRaw   string
			entry    worklog.Entry
		)

		if err := rows.Scan(
			&id,
			&startRaw,
			&endRaw,
			&entry.Billable,
			&entry.Description,
			&entry.Project,
			&entry.Activity,
			&entry.Skill,
			&entry.SourceFormat,
			&entry.SourceMapper,
			&entry.SourceFile,
		); err != nil {
			return nil, fmt.Errorf("scan worklog: %w", err)
		}
		entry.ID = id

		entry.StartDateTime, err = time.Parse(time.RFC3339, startRaw)
		if err != nil {
			return nil, fmt.Errorf("parse start datetime %q: %w", startRaw, err)
		}
		entry.EndDateTime, err = time.Parse(time.RFC3339, endRaw)
		if err != nil {
			return nil, fmt.Errorf("parse end datetime %q: %w", endRaw, err)
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate worklogs: %w", err)
	}

	return entries, nil
}

// GetWorklogByID returns one worklog by ID.
func (s *SQLiteStore) GetWorklogByID(id int64) (worklog.Entry, bool, error) {
	if id <= 0 {
		return worklog.Entry{}, false, fmt.Errorf("worklog id must be > 0")
	}

	const query = `
SELECT
	id,
	start_datetime,
	end_datetime,
	billable,
	description,
	project,
	activity,
	skill,
	source_format,
	source_mapper,
	source_file
FROM worklogs
WHERE id = ?;
`

	var (
		entry    worklog.Entry
		startRaw string
		endRaw   string
	)

	err := s.db.QueryRow(query, id).Scan(
		&entry.ID,
		&startRaw,
		&endRaw,
		&entry.Billable,
		&entry.Description,
		&entry.Project,
		&entry.Activity,
		&entry.Skill,
		&entry.SourceFormat,
		&entry.SourceMapper,
		&entry.SourceFile,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return worklog.Entry{}, false, nil
		}
		return worklog.Entry{}, false, fmt.Errorf("query worklog %d: %w", id, err)
	}

	entry.StartDateTime, err = time.Parse(time.RFC3339, startRaw)
	if err != nil {
		return worklog.Entry{}, false, fmt.Errorf("parse start datetime %q: %w", startRaw, err)
	}
	entry.EndDateTime, err = time.Parse(time.RFC3339, endRaw)
	if err != nil {
		return worklog.Entry{}, false, fmt.Errorf("parse end datetime %q: %w", endRaw, err)
	}

	return entry, true, nil
}

// UpdateWorklog replaces all user-editable fields for the row with the given ID.
func (s *SQLiteStore) UpdateWorklog(entry worklog.Entry) error {
	if entry.ID <= 0 {
		return fmt.Errorf("worklog id must be > 0")
	}

	const updateStmt = `
UPDATE worklogs
SET start_datetime = ?,
	end_datetime = ?,
	billable = ?,
	description = ?,
	project = ?,
	activity = ?,
	skill = ?
WHERE id = ?;`

	res, err := s.db.Exec(
		updateStmt,
		entry.StartDateTime.Format(time.RFC3339),
		entry.EndDateTime.Format(time.RFC3339),
		entry.Billable,
		entry.Description,
		entry.Project,
		entry.Activity,
		entry.Skill,
		entry.ID,
	)
	if err != nil {
		return fmt.Errorf("update worklog %d: %w", entry.ID, err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated row count: %w", err)
	}
	if rowsAffected == 0 {
		return ErrWorklogNotFound
	}

	return nil
}

// DeleteWorklog removes the row with the given ID.
func (s *SQLiteStore) DeleteWorklog(id int64) (bool, error) {
	if id <= 0 {
		return false, fmt.Errorf("worklog id must be > 0")
	}

	res, err := s.db.Exec(`DELETE FROM worklogs WHERE id = ?;`, id)
	if err != nil {
		return false, fmt.Errorf("delete worklog %d: %w", id, err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read deleted row count: %w", err)
	}
	return rowsAffected > 0, nil
}

func (s *SQLiteStore) UpdateWorklogTimes(entries []worklog.Entry) (int, error) {
	if len(entries) == 0 {
		return 0, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}

	const updateStmt = `
UPDATE worklogs
SET start_datetime = ?, end_datetime = ?
WHERE id = ?;
`

	stmt, err := tx.Prepare(updateStmt)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("prepare update statement: %w", err)
	}
	defer stmt.Close()

	updated := 0
	for _, entry := range entries {
		if entry.ID <= 0 {
			continue
		}
		res, err := stmt.Exec(
			entry.StartDateTime.Format(time.RFC3339),
			entry.EndDateTime.Format(time.RFC3339),
			entry.ID,
		)
		if err != nil {
			_ = tx.Rollback()
			return updated, fmt.Errorf("update worklog %d: %w", entry.ID, err)
		}

		rowsAffected, err := res.RowsAffected()
		if err == nil && rowsAffected > 0 {
			updated++
		}
	}

	if err := tx.Commit(); err != nil {
		return updated, fmt.Errorf("commit update transaction: %w", err)
	}

	return updated, nil
}

func (s *SQLiteStore) DeleteAllWorklogs() (int64, error) {
	res, err := s.db.Exec(`DELETE FROM worklogs;`)
	if err != nil {
		return 0, fmt.Errorf("delete worklogs: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read deleted row count: %w", err)
	}
	return rows, nil
}
