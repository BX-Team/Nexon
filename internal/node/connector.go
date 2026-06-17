// Package node abstracts talking to a remote Xray host via the NodeConnector interface.
package node

import (
	"context"

	"github.com/BX-Team/Nexon/internal/store"
)

// AccountUser is a user's credentials projected onto a single inbound.
type AccountUser struct {
	Email    string // identity used by Xray Stats (username)
	Protocol string // vmess|vless|trojan|shadowsocks|...
	Tag      string // inbound tag on the node
	// Secret carries the protocol-specific credential (uuid/password/...).
	Secret map[string]string
}

// Stat is one traffic counter read from Xray StatsService.
type Stat struct {
	Email    string
	Uplink   int64
	Downlink int64
}

// Connector projects Nexon state onto a node and reads traffic back.
// Implementations: GRPCConnector (direct Xray API), later AgentConnector.
type Connector interface {
	// Connect establishes the session and returns the reported Xray version.
	Connect(ctx context.Context) (xrayVersion string, err error)
	// AddUser adds a user account to a specific inbound (HandlerService.AddUser).
	AddUser(ctx context.Context, u AccountUser) error
	// RemoveUser removes a user account from an inbound (HandlerService.RemoveUser).
	RemoveUser(ctx context.Context, tag, email string) error
	// QueryStats reads and (optionally) resets per-user traffic (StatsService).
	QueryStats(ctx context.Context, reset bool) ([]Stat, error)
	// Inbounds returns the inbounds the node currently exposes.
	Inbounds(ctx context.Context) ([]*store.Inbound, error)
	// Close tears down the session.
	Close() error
}

// Factory builds a Connector for a node record. Swappable for tests / agent mode.
type Factory func(n *store.Node) Connector

// DefaultFactory builds the real Xray gRPC connector; pass StubFactory in tests.
var DefaultFactory Factory = GRPCFactory

// StubFactory builds logging-only connectors (no live node required).
var StubFactory Factory = func(n *store.Node) Connector { return NewStubConnector(n) }
