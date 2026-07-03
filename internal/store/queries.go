package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("not found")

const tsLayout = "2006-01-02 15:04:05"

func parseTime(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	if t, err := time.Parse(tsLayout, s.String); err == nil {
		return &t
	}
	if t, err := time.Parse(time.RFC3339, s.String); err == nil {
		return &t
	}
	return nil
}

func fmtTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(tsLayout)
}

// ---- Users ----

// CreateUser inserts a new user. Proxies and sub_token must be set by caller.
func (s *Store) CreateUser(u *User) error {
	px, err := u.Proxies.Marshal()
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`
		INSERT INTO users (username, status, data_limit, traffic_reset_strategy, expire_at, hwid_limit, proxies, sub_token)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		u.Username, string(u.Status), u.DataLimit, u.TrafficResetStrategy, fmtTime(u.ExpireAt), u.HWIDLimit, px, u.SubToken)
	if err != nil {
		return err
	}
	u.ID, _ = res.LastInsertId()
	return nil
}

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var u User
	var status, resetStrategy, proxies, subToken string
	var expireAt, subUpdatedAt, lastUA, createdAt, resetAt, notifiedFor sql.NullString
	var groupID sql.NullInt64
	err := row.Scan(&u.ID, &u.Username, &createdAt, &status, &u.DataLimit, &u.UsedTraffic,
		&resetStrategy, &expireAt, &u.HWIDLimit, &proxies, &subToken, &lastUA, &subUpdatedAt, &resetAt, &notifiedFor, &groupID)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.Status = UserStatus(status)
	u.TrafficResetStrategy = resetStrategy
	u.SubToken = subToken
	u.SubLastUserAgent = lastUA.String
	u.ExpireAt = parseTime(expireAt)
	u.SubUpdatedAt = parseTime(subUpdatedAt)
	u.TrafficResetAt = parseTime(resetAt)
	u.ExpiryNotifiedFor = parseTime(notifiedFor)
	if c := parseTime(createdAt); c != nil {
		u.CreatedAt = *c
	}
	if groupID.Valid {
		u.GroupID = &groupID.Int64
	}
	if proxies != "" {
		_ = json.Unmarshal([]byte(proxies), &u.Proxies)
	}
	return &u, nil
}

const userCols = `id, username, created_at, status, data_limit, used_traffic, traffic_reset_strategy, expire_at, hwid_limit, proxies, sub_token, sub_last_user_agent, sub_updated_at, traffic_reset_at, expiry_notified_for, group_id`

// GetUserByName fetches a user by username.
func (s *Store) GetUserByName(name string) (*User, error) {
	return scanUser(s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE username = ?`, name))
}

// GetUserByID fetches a user by primary key.
func (s *Store) GetUserByID(id int64) (*User, error) {
	return scanUser(s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

// GetUserByToken fetches a user by subscription token.
func (s *Store) GetUserByToken(token string) (*User, error) {
	return scanUser(s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE sub_token = ?`, token))
}

// ListUsers returns all users, optionally filtered by status ("" = all).
func (s *Store) ListUsers(status string) ([]*User, error) {
	q := `SELECT ` + userCols + ` FROM users`
	args := []any{}
	if status != "" {
		q += ` WHERE status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY id`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpdateUser persists mutable fields of a user.
func (s *Store) UpdateUser(u *User) error {
	px, err := u.Proxies.Marshal()
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		UPDATE users SET status=?, data_limit=?, used_traffic=?, traffic_reset_strategy=?,
			expire_at=?, hwid_limit=?, proxies=?, sub_token=?, traffic_reset_at=?, expiry_notified_for=?, group_id=? WHERE id=?`,
		string(u.Status), u.DataLimit, u.UsedTraffic, u.TrafficResetStrategy,
		fmtTime(u.ExpireAt), u.HWIDLimit, px, u.SubToken, fmtTime(u.TrafficResetAt), fmtTime(u.ExpiryNotifiedFor), u.GroupID, u.ID)
	return err
}

// TouchSub records the last User-Agent/time a user fetched their subscription.
func (s *Store) TouchSub(userID int64, ua string) error {
	_, err := s.db.Exec(`UPDATE users SET sub_last_user_agent=?, sub_updated_at=? WHERE id=?`,
		ua, time.Now().UTC().Format(tsLayout), userID)
	return err
}

// DeleteUser removes a user (cascades to devices/traffic).
func (s *Store) DeleteUser(id int64) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id=?`, id)
	return err
}

// ---- Nodes ----

func (s *Store) CreateNode(n *Node) error {
	res, err := s.db.Exec(`
		INSERT INTO nodes (name, address, api_port, status)
		VALUES (?, ?, ?, ?)`,
		n.Name, n.Address, n.APIPort, statusOr(n.Status))
	if err != nil {
		return err
	}
	n.ID, _ = res.LastInsertId()
	return nil
}

func statusOr(s string) string {
	if s == "" {
		return "disconnected"
	}
	return s
}

const nodeCols = `id, name, address, api_port, status, xray_version, last_seen, created_at, group_id`

func scanNode(row interface{ Scan(...any) error }) (*Node, error) {
	var n Node
	var status, ver sql.NullString
	var lastSeen, createdAt sql.NullString
	var groupID sql.NullInt64
	err := row.Scan(&n.ID, &n.Name, &n.Address, &n.APIPort, &status, &ver, &lastSeen, &createdAt, &groupID)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	n.Status, n.XrayVersion = status.String, ver.String
	n.LastSeen = parseTime(lastSeen)
	if c := parseTime(createdAt); c != nil {
		n.CreatedAt = *c
	}
	if groupID.Valid {
		n.GroupID = &groupID.Int64
	}
	return &n, nil
}

func (s *Store) GetNodeByName(name string) (*Node, error) {
	return scanNode(s.db.QueryRow(`SELECT `+nodeCols+` FROM nodes WHERE name = ?`, name))
}

func (s *Store) GetNodeByID(id int64) (*Node, error) {
	return scanNode(s.db.QueryRow(`SELECT `+nodeCols+` FROM nodes WHERE id = ?`, id))
}

func (s *Store) ListNodes() ([]*Node, error) {
	rows, err := s.db.Query(`SELECT ` + nodeCols + ` FROM nodes ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) UpdateNodeStatus(id int64, status, xrayVersion string) error {
	_, err := s.db.Exec(`UPDATE nodes SET status=?, xray_version=?, last_seen=? WHERE id=?`,
		status, xrayVersion, time.Now().UTC().Format(tsLayout), id)
	return err
}

func (s *Store) DeleteNode(id int64) error {
	_, err := s.db.Exec(`DELETE FROM nodes WHERE id=?`, id)
	return err
}

// ---- Inbounds ----

func (s *Store) ListInbounds(nodeID int64) ([]*Inbound, error) {
	rows, err := s.db.Query(`SELECT id, node_id, tag, protocol, network, tls, port, settings_json FROM inbounds WHERE node_id=? ORDER BY id`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Inbound
	for rows.Next() {
		var in Inbound
		var network, tls sql.NullString
		if err := rows.Scan(&in.ID, &in.NodeID, &in.Tag, &in.Protocol, &network, &tls, &in.Port, &in.SettingsJSON); err != nil {
			return nil, err
		}
		in.Network, in.TLS = network.String, tls.String
		out = append(out, &in)
	}
	return out, rows.Err()
}

// ListAllInbounds returns inbounds across all connected nodes, used by subgen.
func (s *Store) ListAllInbounds() ([]*Inbound, error) {
	rows, err := s.db.Query(`SELECT id, node_id, tag, protocol, network, tls, port, settings_json FROM inbounds ORDER BY node_id, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Inbound
	for rows.Next() {
		var in Inbound
		var network, tls sql.NullString
		if err := rows.Scan(&in.ID, &in.NodeID, &in.Tag, &in.Protocol, &network, &tls, &in.Port, &in.SettingsJSON); err != nil {
			return nil, err
		}
		in.Network, in.TLS = network.String, tls.String
		out = append(out, &in)
	}
	return out, rows.Err()
}

// DeleteInbound removes an inbound by node and tag.
func (s *Store) DeleteInbound(nodeID int64, tag string) error {
	_, err := s.db.Exec(`DELETE FROM inbounds WHERE node_id=? AND tag=?`, nodeID, tag)
	return err
}

func (s *Store) UpsertInbound(in *Inbound) error {
	_, err := s.db.Exec(`
		INSERT INTO inbounds (node_id, tag, protocol, network, tls, port, settings_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(node_id, tag) DO UPDATE SET protocol=excluded.protocol, network=excluded.network,
			tls=excluded.tls, port=excluded.port, settings_json=excluded.settings_json`,
		in.NodeID, in.Tag, in.Protocol, in.Network, in.TLS, in.Port, in.SettingsJSON)
	return err
}

// ---- Devices ----

// RegisterDevice records or refreshes a device for a user. Returns the device
// and whether it is newly created.
func (s *Store) RegisterDevice(userID int64, hwid, ua, ip string) (*Device, bool, error) {
	now := time.Now().UTC().Format(tsLayout)
	// Look up existing device by (user, hwid); fall back to user_agent when hwid is empty.
	var d Device
	var query string
	var arg any
	if hwid != "" {
		query = `SELECT id, user_id, hwid, user_agent, first_seen, last_seen, ip_last, revoked FROM devices WHERE user_id=? AND hwid=?`
		arg = hwid
	} else {
		query = `SELECT id, user_id, hwid, user_agent, first_seen, last_seen, ip_last, revoked FROM devices WHERE user_id=? AND hwid IS NULL AND user_agent=?`
		arg = ua
	}
	row := s.db.QueryRow(query, userID, arg)
	var dh, dua, dip sql.NullString
	var fs, ls sql.NullString
	var revoked int
	err := row.Scan(&d.ID, &d.UserID, &dh, &dua, &fs, &ls, &dip, &revoked)
	if err == nil {
		d.HWID, d.UserAgent, d.IPLast = dh.String, dua.String, dip.String
		d.Revoked = revoked != 0
		if t := parseTime(fs); t != nil {
			d.FirstSeen = *t
		}
		_, _ = s.db.Exec(`UPDATE devices SET last_seen=?, user_agent=?, ip_last=? WHERE id=?`, now, ua, ip, d.ID)
		return &d, false, nil
	}
	if err != sql.ErrNoRows {
		return nil, false, err
	}
	// Insert new device.
	var hwidArg any
	if hwid != "" {
		hwidArg = hwid
	}
	res, err := s.db.Exec(`INSERT INTO devices (user_id, hwid, user_agent, first_seen, last_seen, ip_last) VALUES (?, ?, ?, ?, ?, ?)`,
		userID, hwidArg, ua, now, now, ip)
	if err != nil {
		return nil, false, err
	}
	d.ID, _ = res.LastInsertId()
	d.UserID, d.HWID, d.UserAgent, d.IPLast = userID, hwid, ua, ip
	return &d, true, nil
}

func (s *Store) ListDevices(userID int64) ([]*Device, error) {
	rows, err := s.db.Query(`SELECT id, user_id, hwid, user_agent, first_seen, last_seen, ip_last, revoked FROM devices WHERE user_id=? ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Device
	for rows.Next() {
		var d Device
		var dh, dua, dip, fs, ls sql.NullString
		var revoked int
		if err := rows.Scan(&d.ID, &d.UserID, &dh, &dua, &fs, &ls, &dip, &revoked); err != nil {
			return nil, err
		}
		d.HWID, d.UserAgent, d.IPLast = dh.String, dua.String, dip.String
		d.Revoked = revoked != 0
		if t := parseTime(fs); t != nil {
			d.FirstSeen = *t
		}
		if t := parseTime(ls); t != nil {
			d.LastSeen = *t
		}
		out = append(out, &d)
	}
	return out, rows.Err()
}

// CountActiveDevices counts non-revoked devices for a user.
func (s *Store) CountActiveDevices(userID int64) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM devices WHERE user_id=? AND revoked=0`, userID).Scan(&n)
	return n, err
}

func (s *Store) RevokeDevice(userID, deviceID int64) error {
	_, err := s.db.Exec(`UPDATE devices SET revoked=1 WHERE user_id=? AND id=?`, userID, deviceID)
	return err
}

// UnrevokeDevice re-activates a previously revoked device slot (e.g. when a
// kicked device reconnects and there is room under the HWID limit again).
func (s *Store) UnrevokeDevice(userID, deviceID int64) error {
	_, err := s.db.Exec(`UPDATE devices SET revoked=0 WHERE user_id=? AND id=?`, userID, deviceID)
	return err
}

// ---- Settings ----

func (s *Store) GetSetting(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return v, err
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

// ---- Sub rules ----

// SubRule is an ordered UA→format detection rule.
type SubRule struct {
	ID       int64
	Priority int
	Regex    string
	Format   string
}

func (s *Store) ListSubRules() ([]SubRule, error) {
	rows, err := s.db.Query(`SELECT id, priority, regex, format FROM sub_rules ORDER BY priority, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SubRule
	for rows.Next() {
		var r SubRule
		if err := rows.Scan(&r.ID, &r.Priority, &r.Regex, &r.Format); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) AddSubRule(priority int, regex, format string) error {
	_, err := s.db.Exec(`INSERT INTO sub_rules (priority, regex, format) VALUES (?, ?, ?)`, priority, regex, format)
	return err
}

func (s *Store) CountSubRules() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM sub_rules`).Scan(&n)
	return n, err
}
