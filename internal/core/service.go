// Package core holds the service layer: all business logic shared by CLI and subscription server.
package core

import (
	"sync"

	"github.com/BX-Team/Nexon/internal/node"
	"github.com/BX-Team/Nexon/internal/store"
)

// Service is the central façade over the store and node connectors.
type Service struct {
	st      *store.Store
	connect node.Factory
	clients clientMatcher // cached, compiled managed client-app patterns

	uptimeMu   sync.Mutex
	lastUptime map[int64]int64 // node id → last seen xray uptime (restart detection)
}

// New builds a Service. nodeFactory may be nil to use the default connector.
func New(st *store.Store, nodeFactory node.Factory) *Service {
	if nodeFactory == nil {
		nodeFactory = node.DefaultFactory
	}
	return &Service{st: st, connect: nodeFactory, lastUptime: map[int64]int64{}}
}

// nodeRestarted records a node's uptime and reports whether it went backwards
// (xray restarted) or is seen for the first time this process (control-plane
// restarted) — both mean the node's in-memory users must be re-pushed.
func (s *Service) nodeRestarted(nodeID, uptime int64) bool {
	s.uptimeMu.Lock()
	defer s.uptimeMu.Unlock()
	last, seen := s.lastUptime[nodeID]
	s.lastUptime[nodeID] = uptime
	return !seen || uptime < last
}

// Store exposes the underlying store for read-only consumers (sub server).
func (s *Service) Store() *store.Store { return s.st }
