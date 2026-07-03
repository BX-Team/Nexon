package node

import (
	"context"
	"log/slog"

	"github.com/BX-Team/Nexon/internal/store"
)

// StubConnector is a logging-only connector for use without a live node.
type StubConnector struct {
	node *store.Node
}

// NewStubConnector returns a logging-only connector for the given node.
func NewStubConnector(n *store.Node) *StubConnector {
	return &StubConnector{node: n}
}

func (c *StubConnector) Connect(ctx context.Context) (string, error) {
	slog.Info("node connect (stub)", "node", c.node.Name, "addr", c.node.Address, "port", c.node.APIPort)
	return "stub-0.0.0", nil
}

func (c *StubConnector) AddUser(ctx context.Context, u AccountUser) error {
	slog.Info("node add user (stub)", "node", c.node.Name, "tag", u.Tag, "email", u.Email, "proto", u.Protocol)
	return nil
}

func (c *StubConnector) RemoveUser(ctx context.Context, tag, email string) error {
	slog.Info("node remove user (stub)", "node", c.node.Name, "tag", tag, "email", email)
	return nil
}

func (c *StubConnector) QueryStats(ctx context.Context, reset bool) ([]Stat, error) {
	// No live node: nothing to report.
	return nil, nil
}

func (c *StubConnector) Uptime(ctx context.Context) (int64, error) {
	// Constant: after the first poll cycle the stub never looks restarted.
	return 1, nil
}

func (c *StubConnector) Inbounds(ctx context.Context) ([]*store.Inbound, error) {
	return nil, nil
}

func (c *StubConnector) Close() error { return nil }
