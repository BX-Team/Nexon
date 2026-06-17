package node

import (
	"context"
	"fmt"
	"strings"

	command "github.com/xtls/xray-core/app/proxyman/command"
	statscmd "github.com/xtls/xray-core/app/stats/command"
	"github.com/xtls/xray-core/common/serial"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/BX-Team/Nexon/internal/store"
)

// GRPCConnector talks to a node's xray-core gRPC API via HandlerService and StatsService.
type GRPCConnector struct {
	node    *store.Node
	conn    *grpc.ClientConn
	handler command.HandlerServiceClient
	stats   statscmd.StatsServiceClient
}

// NewGRPCConnector builds a connector for a node (not yet connected).
func NewGRPCConnector(n *store.Node) *GRPCConnector {
	return &GRPCConnector{node: n}
}

// GRPCFactory is a Factory producing real gRPC connectors.
var GRPCFactory Factory = func(n *store.Node) Connector { return NewGRPCConnector(n) }

func (c *GRPCConnector) target() string {
	return fmt.Sprintf("%s:%d", c.node.Address, c.node.APIPort)
}

// Connect dials the node and verifies the API is reachable (plaintext; restrict by firewall).
func (c *GRPCConnector) Connect(ctx context.Context) (string, error) {
	conn, err := grpc.NewClient(c.target(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", err
	}
	c.conn = conn
	c.handler = command.NewHandlerServiceClient(conn)
	c.stats = statscmd.NewStatsServiceClient(conn)

	// Liveness probe: list inbound tags (cheap, confirms API is up).
	if _, err := c.handler.ListInbounds(ctx, &command.ListInboundsRequest{IsOnlyTags: true}); err != nil {
		conn.Close()
		c.conn = nil
		return "", fmt.Errorf("xray API unreachable: %w", err)
	}
	return "xray", nil
}

// AddUser adds an account to an inbound via AlterInbound + AddUserOperation.
func (c *GRPCConnector) AddUser(ctx context.Context, u AccountUser) error {
	user, err := xrayUser(u)
	if err != nil {
		return err
	}
	_, err = c.handler.AlterInbound(ctx, &command.AlterInboundRequest{
		Tag:       u.Tag,
		Operation: serial.ToTypedMessage(&command.AddUserOperation{User: user}),
	})
	// Xray errors on duplicate user; treat as success to keep resync idempotent.
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return nil
	}
	return err
}

// RemoveUser removes an account from an inbound by email.
func (c *GRPCConnector) RemoveUser(ctx context.Context, tag, email string) error {
	_, err := c.handler.AlterInbound(ctx, &command.AlterInboundRequest{
		Tag:       tag,
		Operation: serial.ToTypedMessage(&command.RemoveUserOperation{Email: email}),
	})
	if err != nil && strings.Contains(err.Error(), "not found") {
		return nil // already gone
	}
	return err
}

// QueryStats reads per-user traffic counters (stat name format: user>>>{email}>>>traffic>>>direction).
func (c *GRPCConnector) QueryStats(ctx context.Context, reset bool) ([]Stat, error) {
	resp, err := c.stats.QueryStats(ctx, &statscmd.QueryStatsRequest{Pattern: "user>>>", Reset_: reset})
	if err != nil {
		return nil, err
	}
	byEmail := map[string]*Stat{}
	for _, s := range resp.GetStat() {
		parts := strings.Split(s.GetName(), ">>>")
		if len(parts) < 4 || parts[0] != "user" {
			continue
		}
		email, direction := parts[1], parts[3]
		st := byEmail[email]
		if st == nil {
			st = &Stat{Email: email}
			byEmail[email] = st
		}
		switch direction {
		case "uplink":
			st.Uplink += s.GetValue()
		case "downlink":
			st.Downlink += s.GetValue()
		}
	}
	out := make([]Stat, 0, len(byEmail))
	for _, st := range byEmail {
		out = append(out, *st)
	}
	return out, nil
}

// Inbounds returns nil; inbounds are admin-defined in the DB to preserve port/TLS details.
func (c *GRPCConnector) Inbounds(ctx context.Context) ([]*store.Inbound, error) {
	return nil, nil
}

// ListTags returns the inbound tags the node currently exposes, for validating
// that admin-defined tags actually exist on the node.
func (c *GRPCConnector) ListTags(ctx context.Context) ([]string, error) {
	resp, err := c.handler.ListInbounds(ctx, &command.ListInboundsRequest{IsOnlyTags: true})
	if err != nil {
		return nil, err
	}
	var tags []string
	for _, in := range resp.GetInbounds() {
		if tag := in.GetTag(); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

// Close tears down the gRPC connection.
func (c *GRPCConnector) Close() error {
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}
