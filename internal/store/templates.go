package store

import (
	"database/sql"
	"time"
)

// SubTemplate is a custom Go text/template for one subscription format.
type SubTemplate struct {
	Format    string
	Body      string
	UpdatedAt time.Time
}

// GetTemplate returns the stored template body for a format, or ErrNotFound.
func (s *Store) GetTemplate(format string) (string, error) {
	var body string
	err := s.db.QueryRow(`SELECT body FROM sub_templates WHERE format = ?`, format).Scan(&body)
	if err != nil {
		return "", ErrNotFound
	}
	return body, nil
}

// SetTemplate upserts a format's template body.
func (s *Store) SetTemplate(format, body string) error {
	_, err := s.db.Exec(`
		INSERT INTO sub_templates (format, body, updated_at) VALUES (?, ?, datetime('now'))
		ON CONFLICT(format) DO UPDATE SET body=excluded.body, updated_at=excluded.updated_at`,
		format, body)
	return err
}

// DeleteTemplate removes a format's custom template (reverting to built-in).
func (s *Store) DeleteTemplate(format string) error {
	_, err := s.db.Exec(`DELETE FROM sub_templates WHERE format = ?`, format)
	return err
}

// ListTemplates returns the formats that currently have a custom template.
func (s *Store) ListTemplates() (map[string]time.Time, error) {
	rows, err := s.db.Query(`SELECT format, updated_at FROM sub_templates`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]time.Time{}
	for rows.Next() {
		var f string
		var ts sql.NullString
		if err := rows.Scan(&f, &ts); err != nil {
			return nil, err
		}
		if t := parseTime(ts); t != nil {
			out[f] = *t
		}
	}
	return out, rows.Err()
}
