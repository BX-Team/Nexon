package core

import (
	"testing"

	"github.com/BX-Team/Nexon/internal/store"
)

func TestSetDefaultNodeGroup(t *testing.T) {
	svc := testService(t)
	grp, err := svc.CreateNodeGroup("premium")
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	// alice is ungrouped; the node lives in "premium" → no endpoints for her.
	u, err := svc.AddUser(CreateUserParams{Username: "alice"})
	if err != nil {
		t.Fatalf("add user: %v", err)
	}
	n := &store.Node{Name: "n1", Address: "n1.example.com", APIPort: 62789}
	if err := svc.Store().CreateNode(n); err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := svc.Store().SetNodeGroup(n.ID, &grp.ID); err != nil {
		t.Fatalf("assign node: %v", err)
	}
	if err := svc.Store().UpsertInbound(&store.Inbound{NodeID: n.ID, Tag: "vless-in", Protocol: "vless", Port: 443, SettingsJSON: "{}"}); err != nil {
		t.Fatalf("inbound: %v", err)
	}

	res, err := svc.Subscription(u.SubToken, "clientA", "", "1.1.1.1")
	if err != nil {
		t.Fatalf("sub: %v", err)
	}
	if len(res.Endpoints) != 0 {
		t.Fatalf("ungrouped user must not see premium node, got %d endpoints", len(res.Endpoints))
	}

	// Making "premium" the default pulls ungrouped users/nodes into it.
	if err := svc.SetDefaultNodeGroup(grp.ID); err != nil {
		t.Fatalf("set default: %v", err)
	}
	res, err = svc.Subscription(u.SubToken, "clientA", "", "1.1.1.1")
	if err != nil {
		t.Fatalf("sub after set-default: %v", err)
	}
	if len(res.Endpoints) == 0 {
		t.Fatal("ungrouped user must see the node once its group is default")
	}

	// The current default cannot be deleted; the seeded group now can be.
	if err := svc.DeleteNodeGroup(grp.ID); err == nil {
		t.Fatal("deleting the current default group must fail")
	}
	if err := svc.DeleteNodeGroup(store.DefaultGroupID); err != nil {
		t.Fatalf("deleting the demoted seeded group must work: %v", err)
	}

	if err := svc.SetDefaultNodeGroup(9999); err == nil {
		t.Fatal("set-default on a missing group must fail")
	}
}
