package subgen

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/BX-Team/Nexon/internal/store"
)

func testUser() *store.User {
	return &store.User{
		Username: "alice",
		Proxies: store.Proxies{
			VLESS:       &store.VLESSProxy{ID: "uuid-1", Flow: "xtls-rprx-vision"},
			Shadowsocks: &store.ShadowsocksProxy{Password: "spw", Method: "chacha20-ietf-poly1305"},
		},
	}
}

func testInbounds() []*store.Inbound {
	return []*store.Inbound{
		{NodeID: 1, Tag: "vless-in", Protocol: "vless", Network: "tcp", TLS: "reality", Port: 443, SettingsJSON: `{"sni":"example.com","pbk":"PBK","sid":"ab"}`},
		{NodeID: 1, Tag: "ss-in", Protocol: "shadowsocks", Network: "tcp", Port: 8388, SettingsJSON: `{}`},
	}
}

func buildEps() []Endpoint {
	return BuildEndpoints(testUser(), testInbounds(), func(id int64) (string, string) {
		return "tokyo", "1.2.3.4"
	})
}

func TestBuildEndpoints(t *testing.T) {
	eps := buildEps()
	if len(eps) != 2 {
		t.Fatalf("got %d endpoints, want 2", len(eps))
	}
	if eps[0].Protocol != "vless" || eps[0].Address != "1.2.3.4" || eps[0].UUID != "uuid-1" {
		t.Fatalf("vless endpoint wrong: %+v", eps[0])
	}
	if eps[0].setting("sni") != "example.com" {
		t.Fatalf("sni not parsed: %q", eps[0].setting("sni"))
	}
}

func TestVlessURI(t *testing.T) {
	eps := buildEps()
	uri := eps[0].URI()
	if !strings.HasPrefix(uri, "vless://uuid-1@1.2.3.4:443") {
		t.Fatalf("bad vless uri: %s", uri)
	}
	for _, want := range []string{"security=reality", "pbk=PBK", "sid=ab", "flow=xtls-rprx-vision", "sni=example.com"} {
		if !strings.Contains(uri, want) {
			t.Fatalf("vless uri missing %q: %s", want, uri)
		}
	}
}

func TestBase64Render(t *testing.T) {
	body, ctype := Get("base64").Render(testUser(), buildEps())
	if !strings.HasPrefix(ctype, "text/plain") {
		t.Fatalf("ctype = %q", ctype)
	}
	dec, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		t.Fatalf("body not base64: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(dec)), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d uris, want 2: %q", len(lines), string(dec))
	}
	if !strings.HasPrefix(lines[0], "vless://") || !strings.HasPrefix(lines[1], "ss://") {
		t.Fatalf("unexpected uris: %v", lines)
	}
}

func TestClashMetaRendersVless(t *testing.T) {
	// Plain clash skips vless; clash-meta includes it.
	plain, _ := Get("clash").Render(testUser(), buildEps())
	if strings.Contains(string(plain), "type: vless") {
		t.Fatal("plain clash should not contain vless")
	}
	meta, _ := Get("clash-meta").Render(testUser(), buildEps())
	if !strings.Contains(string(meta), "type: vless") {
		t.Fatal("clash-meta should contain vless")
	}
}
