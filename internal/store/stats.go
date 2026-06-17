package store

// Stats are aggregate counters for the admin dashboard.
type Stats struct {
	UsersTotal    int
	UsersActive   int
	UsersLimited  int
	UsersExpired  int
	UsersDisabled int
	NodesTotal    int
	NodesOnline   int
}

// ComputeStats returns dashboard aggregates in a few cheap queries.
func (s *Store) ComputeStats() (Stats, error) {
	var st Stats
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM users GROUP BY status`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return st, err
		}
		st.UsersTotal += n
		switch UserStatus(status) {
		case StatusActive:
			st.UsersActive = n
		case StatusLimited:
			st.UsersLimited = n
		case StatusExpired:
			st.UsersExpired = n
		case StatusDisabled:
			st.UsersDisabled = n
		}
	}
	if err := rows.Err(); err != nil {
		return st, err
	}
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM nodes`).Scan(&st.NodesTotal)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE status='connected'`).Scan(&st.NodesOnline)
	return st, nil
}
