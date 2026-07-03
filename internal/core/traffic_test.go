package core

import (
	"testing"
	"time"

	"github.com/BX-Team/Nexon/internal/store"
)

func TestResetBoundary(t *testing.T) {
	// Wednesday 2026-06-24 15:00 local.
	now := time.Date(2026, 6, 24, 15, 0, 0, 0, time.UTC)
	cases := []struct {
		strategy string
		want     time.Time
		ok       bool
	}{
		{"day", time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC), true},
		{"week", time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC), true}, // Monday
		{"month", time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC), true},
		{"no_reset", time.Time{}, false},
		{"bogus", time.Time{}, false},
	}
	for _, c := range cases {
		got, ok := resetBoundary(c.strategy, now, 20)
		if ok != c.ok {
			t.Fatalf("%s: ok = %v, want %v", c.strategy, ok, c.ok)
		}
		if ok && !got.Equal(c.want) {
			t.Fatalf("%s: boundary = %v, want %v", c.strategy, got, c.want)
		}
	}
}

// TestResetTrafficHonorsStrategy ensures no_reset users are never zeroed and
// day-strategy users overdue for a reset are.
func TestResetTrafficHonorsStrategy(t *testing.T) {
	svc := testService(t)
	keep, err := svc.AddUser(CreateUserParams{Username: "keep", ResetStrategy: "no_reset"})
	if err != nil {
		t.Fatalf("add keep: %v", err)
	}
	zero, err := svc.AddUser(CreateUserParams{Username: "zero", ResetStrategy: "day"})
	if err != nil {
		t.Fatalf("add zero: %v", err)
	}

	past := time.Now().AddDate(0, -2, 0)
	for _, u := range []*store.User{keep, zero} {
		u.UsedTraffic = 1 << 30
		u.TrafficResetAt = &past
		if err := svc.st.UpdateUser(u); err != nil {
			t.Fatalf("seed traffic: %v", err)
		}
	}

	svc.resetTrafficPeriod(time.Now())

	if u, _ := svc.st.GetUserByName("keep"); u.UsedTraffic != 1<<30 {
		t.Fatalf("no_reset user was zeroed: used = %d", u.UsedTraffic)
	}
	if u, _ := svc.st.GetUserByName("zero"); u.UsedTraffic != 0 {
		t.Fatalf("day user not zeroed: used = %d", u.UsedTraffic)
	}
}

func TestAddUserRejectsBadResetStrategy(t *testing.T) {
	svc := testService(t)
	if _, err := svc.AddUser(CreateUserParams{Username: "x", ResetStrategy: "banana"}); err == nil {
		t.Fatal("expected error for invalid reset strategy")
	}
}
