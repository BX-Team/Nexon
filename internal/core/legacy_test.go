package core

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"

	"github.com/BX-Team/Nexon/internal/store"
)

// makeLegacyToken reproduces PasarGuard's create_subscription_token:
// base64url(payload).rstrip("=") + sha256(body+secret).hexdigest()[:10].
func makeLegacyToken(payload, secret string) string {
	body := base64.RawURLEncoding.EncodeToString([]byte(payload))
	sum := sha256.Sum256([]byte(body + secret))
	return body + hex.EncodeToString(sum[:])[:10]
}

func TestParseLegacyToken(t *testing.T) {
	const secret = "test-secret"

	tok := makeLegacyToken("v3,42,1700000000", secret)
	lt, ok := parseLegacyToken(tok, secret)
	if !ok || lt.UserID != 42 {
		t.Fatalf("v3 token: got %+v ok=%v, want UserID=42", lt, ok)
	}

	if _, ok := parseLegacyToken(tok, "wrong-secret"); ok {
		t.Fatal("token must not verify under a different secret")
	}

	// Base64 signature variant (pasarguard accepts both).
	body := base64.RawURLEncoding.EncodeToString([]byte("v2,7,1700000000"))
	sum := sha256.Sum256([]byte(body + secret))
	b64tok := body + base64.RawURLEncoding.EncodeToString(sum[:])[:10]
	if lt, ok := parseLegacyToken(b64tok, secret); !ok || lt.UserID != 7 {
		t.Fatalf("v2/base64-sig token: got %+v ok=%v, want UserID=7", lt, ok)
	}

	// Oldest username format.
	if lt, ok := parseLegacyToken(makeLegacyToken("alice,1700000000", secret), secret); !ok || lt.Username != "alice" {
		t.Fatalf("username token: got %+v ok=%v, want Username=alice", lt, ok)
	}

	for _, bad := range []string{"", "short", "!!!not-base64!!!" + "0123456789", makeLegacyToken("v3,notanumber,1", secret)} {
		if _, ok := parseLegacyToken(bad, secret); ok {
			t.Fatalf("token %q must not parse", bad)
		}
	}
}

func newPasarguardFixture(t *testing.T, secret string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pasarguard.sqlite")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer db.Close()
	stmts := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT, status TEXT, used_traffic INTEGER,
			data_limit INTEGER, data_limit_reset_strategy TEXT, expire TEXT, hwid_limit INTEGER)`,
		`CREATE TABLE jwt (id INTEGER PRIMARY KEY, secret_key TEXT)`,
		`INSERT INTO jwt (secret_key) VALUES ('` + secret + `')`,
		`INSERT INTO users VALUES (5, 'bob',   'active', 111, 161061273600, 'month', '2027-03-24 09:00:29.000000', 0)`,
		`INSERT INTO users VALUES (9, 'carol', 'active', 222, NULL, 'no_reset', NULL, NULL)`,
		`INSERT INTO users VALUES (12, 'dave', 'active', 333, 0, 'no_reset', NULL, 0)`,
	}
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("fixture %q: %v", q, err)
		}
	}
	return path
}

func TestImportPasarguardAndLegacySubscription(t *testing.T) {
	const secret = "pg-secret"
	svc := testService(t)
	path := newPasarguardFixture(t, secret)

	// carol pre-exists in Nexon: must be mapped, not duplicated or overwritten.
	pre, err := svc.AddUser(CreateUserParams{Username: "carol"})
	if err != nil {
		t.Fatalf("pre-create carol: %v", err)
	}

	grp, err := svc.CreateNodeGroup("migrated")
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	results, err := svc.ImportPasarguard(path, PasarguardImportOptions{
		ResetTraffic: true,
		KeepTraffic:  map[string]bool{"dave": true},
		GroupID:      &grp.ID,
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	actions := map[string]string{}
	for _, r := range results {
		actions[r.Username] = r.Action
	}
	if actions["bob"] != "imported" || actions["dave"] != "imported" || actions["carol"] != "mapped" {
		t.Fatalf("unexpected actions: %+v", actions)
	}

	bob, err := svc.GetUser("bob")
	if err != nil {
		t.Fatalf("bob missing: %v", err)
	}
	if bob.UsedTraffic != 0 || bob.DataLimit != 161061273600 || bob.TrafficResetStrategy != "month" {
		t.Fatalf("bob fields: used=%d limit=%d reset=%s", bob.UsedTraffic, bob.DataLimit, bob.TrafficResetStrategy)
	}
	if bob.ExpireAt == nil || bob.ExpireAt.Year() != 2027 {
		t.Fatalf("bob expire: %v", bob.ExpireAt)
	}
	if bob.GroupID == nil || *bob.GroupID != grp.ID {
		t.Fatalf("bob group: %v, want %d", bob.GroupID, grp.ID)
	}
	dave, _ := svc.GetUser("dave")
	if dave.UsedTraffic != 333 || dave.DataLimit != 0 {
		t.Fatalf("dave keep-traffic: used=%d limit=%d", dave.UsedTraffic, dave.DataLimit)
	}

	// Legacy tokens resolve to the right users, marked Legacy.
	res, err := svc.Subscription(makeLegacyToken("v3,5,1750000000", secret), "clientA", "", "1.1.1.1")
	if err != nil || res.User.Username != "bob" || !res.Legacy {
		t.Fatalf("legacy token for bob: res=%+v err=%v", res, err)
	}
	// Any generation timestamp is accepted — the signature is what counts.
	res, err = svc.Subscription(makeLegacyToken("v3,5,1600000000", secret), "clientA", "", "1.1.1.1")
	if err != nil || res.User.Username != "bob" {
		t.Fatalf("older-timestamp token for bob: %v", err)
	}
	res, err = svc.Subscription(makeLegacyToken("v3,9,1750000000", secret), "clientB", "", "1.1.1.1")
	if err != nil || res.User.ID != pre.ID {
		t.Fatalf("mapped legacy token for carol: res=%+v err=%v", res, err)
	}

	// Native tokens still work and are not flagged legacy.
	res, err = svc.Subscription(pre.SubToken, "clientB", "", "1.1.1.1")
	if err != nil || res.Legacy {
		t.Fatalf("native token: legacy=%v err=%v", res.Legacy, err)
	}

	// Forged and unknown-id tokens are rejected.
	if _, err := svc.Subscription(makeLegacyToken("v3,5,1750000000", "other-secret"), "x", "", "1.1.1.1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("forged token must be ErrNotFound, got %v", err)
	}
	if _, err := svc.Subscription(makeLegacyToken("v3,999,1750000000", secret), "x", "", "1.1.1.1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown legacy id must be ErrNotFound, got %v", err)
	}

	// Rotating the native token severs the legacy mapping too.
	if _, err := svc.RotateToken("bob"); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if _, err := svc.Subscription(makeLegacyToken("v3,5,1750000000", secret), "clientA", "", "1.1.1.1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("legacy token must die after rotation, got %v", err)
	}
}
