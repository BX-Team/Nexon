package node

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	command "github.com/xtls/xray-core/app/proxyman/command"
	statscmd "github.com/xtls/xray-core/app/stats/command"
	hysteria "github.com/xtls/xray-core/proxy/hysteria/account"
	"github.com/xtls/xray-core/proxy/vless"
	"google.golang.org/grpc"

	"github.com/BX-Team/Nexon/internal/store"
)

// fakeXray implements the HandlerService + StatsService subset Nexon uses, so
// the GRPCConnector can be exercised end-to-end without a live xray-core node.
type fakeXray struct {
	command.UnimplementedHandlerServiceServer
	statscmd.UnimplementedStatsServiceServer

	mu     sync.Mutex
	users  map[string]string // "tag/email" -> account type URL
	stats  map[string]int64  // stat name -> value
	uptime uint32
}

func (f *fakeXray) key(tag, email string) string { return tag + "/" + email }

func (f *fakeXray) AlterInbound(_ context.Context, req *command.AlterInboundRequest) (*command.AlterInboundResponse, error) {
	op, err := req.Operation.GetInstance()
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch o := op.(type) {
	case *command.AddUserOperation:
		acc, _ := o.User.Account.GetInstance()
		typeURL := req.Operation.Type
		if acc != nil {
			typeURL = o.User.Account.Type
		}
		f.users[f.key(req.Tag, o.User.Email)] = typeURL
	case *command.RemoveUserOperation:
		delete(f.users, f.key(req.Tag, o.Email))
	}
	return &command.AlterInboundResponse{}, nil
}

func (f *fakeXray) ListInbounds(_ context.Context, _ *command.ListInboundsRequest) (*command.ListInboundsResponse, error) {
	return &command.ListInboundsResponse{}, nil
}

func (f *fakeXray) QueryStats(_ context.Context, _ *statscmd.QueryStatsRequest) (*statscmd.QueryStatsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var stats []*statscmd.Stat
	for name, val := range f.stats {
		stats = append(stats, &statscmd.Stat{Name: name, Value: val})
	}
	return &statscmd.QueryStatsResponse{Stat: stats}, nil
}

func (f *fakeXray) GetSysStats(_ context.Context, _ *statscmd.SysStatsRequest) (*statscmd.SysStatsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return &statscmd.SysStatsResponse{Uptime: f.uptime}, nil
}

// startFakeXray spins the fake server on a random localhost port.
func startFakeXray(t *testing.T) (*fakeXray, int) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	fake := &fakeXray{users: map[string]string{}, stats: map[string]int64{}}
	srv := grpc.NewServer()
	command.RegisterHandlerServiceServer(srv, fake)
	statscmd.RegisterStatsServiceServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return fake, lis.Addr().(*net.TCPAddr).Port
}

func TestGRPCConnector(t *testing.T) {
	fake, port := startFakeXray(t)
	conn := NewGRPCConnector(&store.Node{Name: "test", Address: "127.0.0.1", APIPort: port})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ver, err := conn.Connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if ver != "" {
		t.Fatalf("version = %q, want empty (API has no version RPC)", ver)
	}
	defer conn.Close()

	// AddUser projects a vless account with the right id/flow.
	acc := AccountUser{Email: "alice", Protocol: "vless", Tag: "vless-in", Secret: map[string]string{"id": "uuid-123", "flow": "xtls-rprx-vision"}}
	if err := conn.AddUser(ctx, acc); err != nil {
		t.Fatalf("add user: %v", err)
	}
	fake.mu.Lock()
	typeURL, ok := fake.users["vless-in/alice"]
	fake.mu.Unlock()
	if !ok {
		t.Fatal("user not registered on node")
	}
	if typeURL == "" {
		t.Fatal("account type URL missing")
	}

	// RemoveUser deletes it.
	if err := conn.RemoveUser(ctx, "vless-in", "alice"); err != nil {
		t.Fatalf("remove user: %v", err)
	}
	fake.mu.Lock()
	_, stillThere := fake.users["vless-in/alice"]
	fake.mu.Unlock()
	if stillThere {
		t.Fatal("user not removed")
	}

	// QueryStats aggregates uplink+downlink per email.
	fake.mu.Lock()
	fake.stats["user>>>alice>>>traffic>>>uplink"] = 100
	fake.stats["user>>>alice>>>traffic>>>downlink"] = 250
	fake.stats["user>>>bob>>>traffic>>>uplink"] = 7
	fake.stats["inbound>>>api>>>traffic>>>downlink"] = 9999 // must be ignored
	fake.mu.Unlock()

	stats, err := conn.QueryStats(ctx, true)
	if err != nil {
		t.Fatalf("query stats: %v", err)
	}
	got := map[string][2]int64{}
	for _, s := range stats {
		got[s.Email] = [2]int64{s.Uplink, s.Downlink}
	}
	if got["alice"] != [2]int64{100, 250} {
		t.Fatalf("alice stats = %v, want [100 250]", got["alice"])
	}
	if got["bob"] != [2]int64{7, 0} {
		t.Fatalf("bob stats = %v, want [7 0]", got["bob"])
	}
	if _, ok := got["api"]; ok {
		t.Fatal("non-user stat leaked into results")
	}

	// Uptime comes from GetSysStats.
	fake.mu.Lock()
	fake.uptime = 42
	fake.mu.Unlock()
	up, err := conn.Uptime(ctx)
	if err != nil {
		t.Fatalf("uptime: %v", err)
	}
	if up != 42 {
		t.Fatalf("uptime = %d, want 42", up)
	}
}

// verify the vless account really decodes back to the right fields.
func TestXrayAccountVless(t *testing.T) {
	msg, err := xrayAccount(AccountUser{Protocol: "vless", Secret: map[string]string{"id": "abc", "flow": "vision"}})
	if err != nil {
		t.Fatal(err)
	}
	a, ok := msg.(*vless.Account)
	if !ok {
		t.Fatalf("got %T, want *vless.Account", msg)
	}
	if a.Id != "abc" || a.Flow != "vision" {
		t.Fatalf("account = %+v", a)
	}
}

func TestXrayUserHysteria(t *testing.T) {
	// Full provisioning path: hysteria auth -> typed account -> protocol.User.
	msg, err := xrayAccount(AccountUser{Protocol: "hysteria2", Secret: map[string]string{"auth": "secret-auth"}})
	if err != nil {
		t.Fatal(err)
	}
	a, ok := msg.(*hysteria.Account)
	if !ok {
		t.Fatalf("got %T, want *hysteria.Account", msg)
	}
	if a.Auth != "secret-auth" {
		t.Fatalf("account = %+v", a)
	}
	u, err := xrayUser(AccountUser{Protocol: "hysteria2", Email: "bob", Secret: map[string]string{"auth": "secret-auth"}})
	if err != nil {
		t.Fatalf("xrayUser: %v", err)
	}
	if u.Email != "bob" || u.Account == nil {
		t.Fatalf("user = %+v", u)
	}
}
