package core

import (
	"errors"
	"testing"
)

// TestSubscriptionHWIDLimit verifies the device limit is enforced and, crucially,
// that a rejected device stays rejected on retry (it must not be re-matched by
// User-Agent and silently admitted).
func TestSubscriptionHWIDLimit(t *testing.T) {
	svc := testService(t)
	u, err := svc.AddUser(CreateUserParams{Username: "alice", HWIDLimit: 2})
	if err != nil {
		t.Fatalf("add user: %v", err)
	}
	tok := u.SubToken

	// Two distinct devices fit within the limit.
	if _, err := svc.Subscription(tok, "clientA", "", "1.1.1.1"); err != nil {
		t.Fatalf("device A should be admitted: %v", err)
	}
	if _, err := svc.Subscription(tok, "clientB", "", "1.1.1.1"); err != nil {
		t.Fatalf("device B should be admitted: %v", err)
	}

	// The third device is over the limit and must be denied...
	if _, err := svc.Subscription(tok, "clientC", "", "1.1.1.1"); !errors.Is(err, ErrSubDenied) {
		t.Fatalf("device C should be denied, got %v", err)
	}
	// ...and must STAY denied on retry (regression: revoked device was matched by
	// UA on retry, returned created=false, and bypassed the limit check).
	if _, err := svc.Subscription(tok, "clientC", "", "1.1.1.1"); !errors.Is(err, ErrSubDenied) {
		t.Fatalf("device C retry should still be denied, got %v", err)
	}

	// An already-registered device keeps working.
	if _, err := svc.Subscription(tok, "clientA", "", "1.1.1.1"); err != nil {
		t.Fatalf("existing device A should still work: %v", err)
	}

	if n, _ := svc.st.CountActiveDevices(u.ID); n != 2 {
		t.Fatalf("active devices = %d, want 2", n)
	}

	// Freeing a slot re-admits a previously kicked device on its next fetch.
	devs, _ := svc.st.ListDevices(u.ID)
	for _, d := range devs {
		if d.UserAgent == "clientA" {
			if err := svc.st.RevokeDevice(u.ID, d.ID); err != nil {
				t.Fatalf("revoke: %v", err)
			}
		}
	}
	if _, err := svc.Subscription(tok, "clientC", "", "1.1.1.1"); err != nil {
		t.Fatalf("device C should be re-admitted after a slot freed: %v", err)
	}
	if n, _ := svc.st.CountActiveDevices(u.ID); n != 2 {
		t.Fatalf("active devices after re-admit = %d, want 2", n)
	}
}

// TestAddUserAnchorsTrafficReset ensures a new user's traffic-reset timestamp is
// set at creation, so the first poll cycle does not treat them as overdue and
// wipe their freshly-recorded traffic.
func TestAddUserAnchorsTrafficReset(t *testing.T) {
	svc := testService(t)
	u, err := svc.AddUser(CreateUserParams{Username: "bob"})
	if err != nil {
		t.Fatalf("add user: %v", err)
	}
	if u.TrafficResetAt == nil {
		t.Fatal("TrafficResetAt should be set on creation")
	}
	// And it must be persisted, not just set in memory.
	reloaded, err := svc.st.GetUserByName("bob")
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.TrafficResetAt == nil {
		t.Fatal("TrafficResetAt should be persisted by CreateUser")
	}
}
