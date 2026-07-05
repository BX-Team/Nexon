package core

import (
	"fmt"
	"strings"

	"github.com/BX-Team/Nexon/internal/store"
)

// groupOf resolves a nullable group id against the current default group.
func groupOf(g *int64, def int64) int64 {
	if g == nil {
		return def
	}
	return *g
}

// ListNodeGroups returns all node groups (admin).
func (s *Service) ListNodeGroups() ([]*store.NodeGroup, error) { return s.st.ListNodeGroups() }

// CreateNodeGroup adds a node group.
func (s *Service) CreateNodeGroup(name string) (*store.NodeGroup, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("название группы обязательно")
	}
	g, err := s.st.CreateNodeGroup(name)
	if err != nil {
		return nil, err
	}
	_ = s.st.AddLog("info", "node-group", fmt.Sprintf("создана группа нод %q", name))
	return g, nil
}

// SetDefaultNodeGroup makes a group the default (NULL group_id members follow it)
// and re-syncs every node, since effective membership of ungrouped users/nodes changes.
func (s *Service) SetDefaultNodeGroup(id int64) error {
	if err := s.st.SetDefaultNodeGroup(id); err != nil {
		return err
	}
	_ = s.st.AddLog("info", "node-group", fmt.Sprintf("группа нод #%d назначена группой по умолчанию", id))
	if nodes, err := s.st.ListNodes(); err == nil {
		for _, n := range nodes {
			_ = s.SyncNode(n.Name)
		}
	}
	return nil
}

// DeleteNodeGroup removes a group; its nodes/users revert to the default group.
func (s *Service) DeleteNodeGroup(id int64) error {
	if id == s.st.DefaultNodeGroupID() {
		return fmt.Errorf("нельзя удалить группу по умолчанию")
	}
	if err := s.st.DeleteNodeGroup(id); err != nil {
		return err
	}
	_ = s.st.AddLog("info", "node-group", fmt.Sprintf("удалена группа нод #%d", id))
	return nil
}

// SetNodeGroup moves a node into a group (groupID nil = default) and re-pushes
// users so the node serves the right audience.
func (s *Service) SetNodeGroup(nodeID int64, groupID *int64) error {
	if err := s.st.SetNodeGroup(nodeID, groupID); err != nil {
		return err
	}
	if n, err := s.st.GetNodeByID(nodeID); err == nil {
		_ = s.SyncNode(n.Name)
	}
	return nil
}

// SetUserGroup moves a user into a node group (groupID nil = default) and
// re-projects them: removed from every node, then re-added to the new group's.
func (s *Service) SetUserGroup(userID int64, groupID *int64) error {
	u, err := s.st.GetUserByID(userID)
	if err != nil {
		return err
	}
	s.removeUserFromNodes(u)
	if err := s.st.SetUserGroup(userID, groupID); err != nil {
		return err
	}
	u.GroupID = groupID
	s.syncUserToNodes(u)
	return nil
}
