package store

import (
	"database/sql"
	"time"
)

// LogEntry is one row in the admin-facing event log.
type LogEntry struct {
	ID       int64
	Ts       time.Time
	Level    string
	Category string
	Message  string
}

// AddLog appends an event. Best-effort: callers ignore the error.
func (s *Store) AddLog(level, category, message string) error {
	if level == "" {
		level = "info"
	}
	if category == "" {
		category = "system"
	}
	_, err := s.db.Exec(`INSERT INTO event_log (level, category, message) VALUES (?, ?, ?)`, level, category, message)
	return err
}

// ListLogs returns the newest entries first, capped at limit.
func (s *Store) ListLogs(limit int) ([]*LogEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := s.db.Query(`SELECT id, ts, level, category, message FROM event_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*LogEntry
	for rows.Next() {
		var e LogEntry
		var ts sql.NullString
		if err := rows.Scan(&e.ID, &ts, &e.Level, &e.Category, &e.Message); err != nil {
			return nil, err
		}
		if t := parseTime(ts); t != nil {
			e.Ts = *t
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}
