// Package core holds the service layer: all business logic shared by CLI and subscription server.
package core

import (
	"github.com/BX-Team/Nexon/internal/node"
	"github.com/BX-Team/Nexon/internal/store"
)

// Service is the central façade over the store and node connectors.
type Service struct {
	st      *store.Store
	connect node.Factory
	clients clientMatcher // cached, compiled managed client-app patterns
}

// New builds a Service. nodeFactory may be nil to use the default connector.
func New(st *store.Store, nodeFactory node.Factory) *Service {
	if nodeFactory == nil {
		nodeFactory = node.DefaultFactory
	}
	return &Service{st: st, connect: nodeFactory}
}

// Store exposes the underlying store for read-only consumers (sub server).
func (s *Service) Store() *store.Store { return s.st }
