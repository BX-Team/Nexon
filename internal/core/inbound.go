package core

import (
	"fmt"

	"github.com/BX-Team/Nexon/internal/store"
)

// AddInboundParams describes an inbound to register on a node; protocol/port/transport are admin-defined.
type AddInboundParams struct {
	NodeName     string
	Tag          string
	Protocol     string
	Port         int
	Network      string
	TLS          string
	SettingsJSON string
	Remark       string
}

// AddInbound registers an inbound on a node and resyncs active users to it.
func (s *Service) AddInbound(p AddInboundParams) (*store.Inbound, error) {
	n, err := s.st.GetNodeByName(p.NodeName)
	if err != nil {
		return nil, err
	}
	if p.Tag == "" || p.Protocol == "" {
		return nil, fmt.Errorf("tag and protocol required")
	}
	if p.SettingsJSON == "" {
		p.SettingsJSON = "{}"
	}
	in := &store.Inbound{
		NodeID:       n.ID,
		Tag:          p.Tag,
		Protocol:     p.Protocol,
		Port:         p.Port,
		Network:      p.Network,
		TLS:          p.TLS,
		SettingsJSON: p.SettingsJSON,
		Remark:       p.Remark,
	}
	if err := s.st.UpsertInbound(in); err != nil {
		return nil, err
	}
	// Push every active user onto the new inbound (best-effort).
	if err := s.SyncNode(n.Name); err != nil {
		return in, nil // inbound saved; sync failure is non-fatal
	}
	return in, nil
}

// RemoveInbound deletes an inbound from a node.
func (s *Service) RemoveInbound(nodeName, tag string) error {
	n, err := s.st.GetNodeByName(nodeName)
	if err != nil {
		return err
	}
	return s.st.DeleteInbound(n.ID, tag)
}
