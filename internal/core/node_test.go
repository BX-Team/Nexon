package core

import (
	"testing"
	"time"

	"github.com/BX-Team/Nexon/internal/store"
)

func TestAccountsFor(t *testing.T) {
	u := &store.User{
		Username: "alice",
		Proxies: store.Proxies{
			VLESS:       &store.VLESSProxy{ID: "uuid-1", Flow: "xtls-rprx-vision"},
			Trojan:      &store.TrojanProxy{Password: "tpw"},
			Shadowsocks: &store.ShadowsocksProxy{Password: "spw", Method: "chacha20-ietf-poly1305"},
			Hysteria:    &store.HysteriaProxy{Auth: "hauth"},
			// No VMess proxy: a vmess inbound must be skipped.
		},
	}
	inbounds := []*store.Inbound{
		{Tag: "vless-in", Protocol: "vless"},
		{Tag: "trojan-in", Protocol: "trojan"},
		{Tag: "ss-in", Protocol: "shadowsocks"},
		{Tag: "hy-in", Protocol: "hysteria2"},
		{Tag: "vmess-in", Protocol: "vmess"},     // skipped (no secret)
		{Tag: "unknown-in", Protocol: "unknown"}, // unsupported -> skipped
	}

	accs := accountsFor(u, inbounds)
	if len(accs) != 4 {
		t.Fatalf("got %d accounts, want 4: %+v", len(accs), accs)
	}
	byTag := map[string]string{}
	for _, a := range accs {
		if a.Email != "alice" {
			t.Fatalf("email = %q, want alice", a.Email)
		}
		byTag[a.Tag] = a.Protocol
	}
	if byTag["vless-in"] != "vless" || byTag["trojan-in"] != "trojan" || byTag["ss-in"] != "shadowsocks" || byTag["hy-in"] != "hysteria2" {
		t.Fatalf("unexpected tag mapping: %+v", byTag)
	}
	// Verify secrets carried through.
	for _, a := range accs {
		if a.Tag == "vless-in" {
			if a.Secret["id"] != "uuid-1" || a.Secret["flow"] != "xtls-rprx-vision" {
				t.Fatalf("vless secret = %+v", a.Secret)
			}
		}
		if a.Tag == "hy-in" && a.Secret["auth"] != "hauth" {
			t.Fatalf("hysteria secret = %+v", a.Secret)
		}
	}
}

func TestParseDuration(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		spec string
		want time.Time
	}{
		{"30d", base.AddDate(0, 0, 30)},
		{"+15d", base.AddDate(0, 0, 15)},
		{"12h", base.Add(12 * time.Hour)},
	}
	for _, c := range cases {
		got, err := ParseDuration(c.spec, base)
		if err != nil {
			t.Fatalf("%s: %v", c.spec, err)
		}
		if !got.Equal(c.want) {
			t.Fatalf("%s: got %v want %v", c.spec, got, c.want)
		}
	}
	if d, err := ParseDuration("", base); err != nil || d != nil {
		t.Fatalf("empty should be nil,nil; got %v,%v", d, err)
	}
	if _, err := ParseDuration("bogus", base); err == nil {
		t.Fatal("expected error for bogus duration")
	}
}
