package store

// DefaultGroupID is the seeded "Default" node group; NULL group_id resolves to this.
const DefaultGroupID int64 = 1

// NodeGroup routes a set of nodes to a set of users.
type NodeGroup struct {
	ID        int64
	Name      string
	IsDefault bool
}

func (s *Store) ListNodeGroups() ([]*NodeGroup, error) {
	rows, err := s.db.Query(`SELECT id, name, is_default FROM node_groups ORDER BY is_default DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*NodeGroup
	for rows.Next() {
		var g NodeGroup
		var def int
		if err := rows.Scan(&g.ID, &g.Name, &def); err != nil {
			return nil, err
		}
		g.IsDefault = def != 0
		out = append(out, &g)
	}
	return out, rows.Err()
}

func (s *Store) CreateNodeGroup(name string) (*NodeGroup, error) {
	res, err := s.db.Exec(`INSERT INTO node_groups (name) VALUES (?)`, name)
	if err != nil {
		return nil, err
	}
	g := &NodeGroup{Name: name}
	g.ID, _ = res.LastInsertId()
	return g, nil
}

// DeleteNodeGroup removes a group; nodes and users referencing it revert to the default group.
func (s *Store) DeleteNodeGroup(id int64) error {
	if id == DefaultGroupID {
		return ErrNotFound
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE nodes SET group_id=NULL WHERE group_id=?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE users SET group_id=NULL WHERE group_id=?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM node_groups WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// SetNodeGroup assigns a node to a group (nil = default).
func (s *Store) SetNodeGroup(nodeID int64, groupID *int64) error {
	_, err := s.db.Exec(`UPDATE nodes SET group_id=? WHERE id=?`, groupID, nodeID)
	return err
}

// SetUserGroup assigns a user to a group (nil = default).
func (s *Store) SetUserGroup(userID int64, groupID *int64) error {
	_, err := s.db.Exec(`UPDATE users SET group_id=? WHERE id=?`, groupID, userID)
	return err
}
