package store

import "encoding/json"

// ClientApp is a managed VPN client with a UA regex and custom response headers.
type ClientApp struct {
	ID        int64
	Name      string
	UAPattern string
	Headers   map[string]string
	Enabled   bool
	Sort      int
	Format    string // "" = use UA detection rules
}

func scanClientApp(row interface{ Scan(...any) error }) (*ClientApp, error) {
	var c ClientApp
	var headers string
	var enabled int
	if err := row.Scan(&c.ID, &c.Name, &c.UAPattern, &headers, &enabled, &c.Sort, &c.Format); err != nil {
		return nil, err
	}
	c.Enabled = enabled != 0
	c.Headers = map[string]string{}
	if headers != "" {
		_ = json.Unmarshal([]byte(headers), &c.Headers)
	}
	return &c, nil
}

const clientAppCols = `id, name, ua_pattern, headers, enabled, sort, format`

// ListClientApps returns all managed client apps in display order.
func (s *Store) ListClientApps() ([]*ClientApp, error) {
	rows, err := s.db.Query(`SELECT ` + clientAppCols + ` FROM client_apps ORDER BY sort, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*ClientApp
	for rows.Next() {
		c, err := scanClientApp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CreateClientApp(c *ClientApp) error {
	hb, _ := json.Marshal(orEmptyMap(c.Headers))
	res, err := s.db.Exec(`INSERT INTO client_apps (name, ua_pattern, headers, enabled, sort, format) VALUES (?, ?, ?, ?, ?, ?)`,
		c.Name, c.UAPattern, string(hb), boolInt(c.Enabled), c.Sort, c.Format)
	if err != nil {
		return err
	}
	c.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) UpdateClientApp(c *ClientApp) error {
	hb, _ := json.Marshal(orEmptyMap(c.Headers))
	_, err := s.db.Exec(`UPDATE client_apps SET name=?, ua_pattern=?, headers=?, enabled=?, sort=?, format=? WHERE id=?`,
		c.Name, c.UAPattern, string(hb), boolInt(c.Enabled), c.Sort, c.Format, c.ID)
	return err
}

func (s *Store) DeleteClientApp(id int64) error {
	_, err := s.db.Exec(`DELETE FROM client_apps WHERE id=?`, id)
	return err
}

func orEmptyMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
