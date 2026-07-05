package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/BX-Team/Nexon/internal/node"
	"github.com/BX-Team/Nexon/internal/store"
)

// AddNodeParams holds inputs for AddNode.
type AddNodeParams struct {
	Name    string
	Address string
	APIPort int
}

// AddNode registers a node, attempts a connection and records its version.
func (s *Service) AddNode(p AddNodeParams) (*store.Node, error) {
	if p.Name == "" || p.Address == "" {
		return nil, fmt.Errorf("name and address required")
	}
	n := &store.Node{
		Name:    p.Name,
		Address: p.Address,
		APIPort: p.APIPort,
	}
	if err := s.st.CreateNode(n); err != nil {
		return nil, err
	}
	// Best-effort initial connect; failure leaves node in disconnected state.
	if err := s.SyncNode(n.Name); err != nil {
		slog.Warn("initial node sync failed", "node", n.Name, "err", err)
	}
	// Re-fetch so the caller sees the post-sync status/version.
	if refreshed, err := s.st.GetNodeByName(n.Name); err == nil {
		return refreshed, nil
	}
	return n, nil
}

// ListNodes returns all nodes.
func (s *Service) ListNodes() ([]*store.Node, error) { return s.st.ListNodes() }

// GetNode returns a node by name.
func (s *Service) GetNode(name string) (*store.Node, error) { return s.st.GetNodeByName(name) }

// DeleteNode removes a node and its inbounds.
func (s *Service) DeleteNode(name string) error {
	n, err := s.st.GetNodeByName(name)
	if err != nil {
		return err
	}
	return s.st.DeleteNode(n.ID)
}

// SyncNode connects to a node, refreshes its inbounds and re-pushes every active user.
func (s *Service) SyncNode(name string) error {
	n, err := s.st.GetNodeByName(name)
	if err != nil {
		return err
	}
	conn := s.connect(n)
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ver, err := conn.Connect(ctx)
	if err != nil {
		_ = s.st.UpdateNodeStatus(n.ID, "error", "")
		return err
	}
	_ = s.st.UpdateNodeStatus(n.ID, "connected", ver)

	// Refresh inbounds the node reports (if the connector supports it).
	if ins, err := conn.Inbounds(ctx); err == nil {
		for _, in := range ins {
			in.NodeID = n.ID
			_ = s.st.UpsertInbound(in)
		}
	}

	inbounds, err := s.st.ListInbounds(n.ID)
	if err != nil {
		return err
	}
	users, err := s.st.ListUsers(string(store.StatusActive))
	if err != nil {
		return err
	}
	def := s.st.DefaultNodeGroupID()
	nodeGroup := groupOf(n.GroupID, def)
	for _, u := range users {
		if groupOf(u.GroupID, def) != nodeGroup {
			continue // node only serves its own group
		}
		for _, acc := range accountsFor(u, inbounds) {
			if err := conn.AddUser(ctx, acc); err != nil {
				slog.Warn("resync add user failed", "node", n.Name, "user", u.Username, "tag", acc.Tag, "err", err)
			}
		}
	}
	return nil
}

// syncUserToNodes pushes one active user to every node's matching inbounds (best-effort).
func (s *Service) syncUserToNodes(u *store.User) {
	if u.Status != store.StatusActive {
		return
	}
	nodes, err := s.st.ListNodes()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	def := s.st.DefaultNodeGroupID()
	userGroup := groupOf(u.GroupID, def)
	for _, n := range nodes {
		if groupOf(n.GroupID, def) != userGroup {
			continue // push only to nodes in the user's group
		}
		inbounds, err := s.st.ListInbounds(n.ID)
		if err != nil || len(inbounds) == 0 {
			continue
		}
		conn := s.connect(n)
		if _, err := conn.Connect(ctx); err != nil {
			conn.Close()
			continue
		}
		for _, acc := range accountsFor(u, inbounds) {
			_ = conn.AddUser(ctx, acc)
		}
		conn.Close()
	}
}

// removeUserFromNodes removes a user from all nodes (auto-kick / disable).
func (s *Service) removeUserFromNodes(u *store.User) {
	nodes, err := s.st.ListNodes()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for _, n := range nodes {
		inbounds, err := s.st.ListInbounds(n.ID)
		if err != nil {
			continue
		}
		conn := s.connect(n)
		if _, err := conn.Connect(ctx); err != nil {
			conn.Close()
			continue
		}
		for _, in := range inbounds {
			_ = conn.RemoveUser(ctx, in.Tag, u.Username)
		}
		conn.Close()
	}
}

// accountsFor maps a user's proxy bundle onto the inbounds available on a node.
func accountsFor(u *store.User, inbounds []*store.Inbound) []node.AccountUser {
	var out []node.AccountUser
	for _, in := range inbounds {
		acc := node.AccountUser{Email: u.Username, Protocol: in.Protocol, Tag: in.Tag, Secret: map[string]string{}}
		switch in.Protocol {
		case "vmess":
			if u.Proxies.VMess == nil {
				continue
			}
			acc.Secret["id"] = u.Proxies.VMess.ID
		case "vless":
			if u.Proxies.VLESS == nil {
				continue
			}
			acc.Secret["id"] = u.Proxies.VLESS.ID
			acc.Secret["flow"] = u.Proxies.VLESS.Flow
		case "trojan":
			if u.Proxies.Trojan == nil {
				continue
			}
			acc.Secret["password"] = u.Proxies.Trojan.Password
		case "shadowsocks":
			if u.Proxies.Shadowsocks == nil {
				continue
			}
			acc.Secret["password"] = u.Proxies.Shadowsocks.Password
			acc.Secret["method"] = u.Proxies.Shadowsocks.Method
		case "hysteria", "hysteria2":
			if u.Proxies.Hysteria == nil {
				continue
			}
			acc.Secret["auth"] = u.Proxies.Hysteria.Auth
		default:
			continue // unsupported protocol for now
		}
		out = append(out, acc)
	}
	return out
}
