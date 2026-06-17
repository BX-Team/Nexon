package subgen

import (
	"strings"
	"testing"

	"github.com/BX-Team/Nexon/internal/store"
)

func sampleEndpoints() []Endpoint {
	return []Endpoint{{
		Name: "tokyo-vless", Protocol: "vless", Address: "1.2.3.4", Port: 443,
		Network: "tcp", TLS: "tls", UUID: "uuid-1", Settings: map[string]any{"sni": "example.com"},
	}}
}

func TestRenderWithTemplateClash(t *testing.T) {
	body, ctype, err := RenderWithTemplate("clash-meta", clashStarter, &store.User{Username: "alice"}, sampleEndpoints())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	out := string(body)
	if !strings.Contains(ctype, "yaml") {
		t.Fatalf("content type = %q", ctype)
	}
	for _, want := range []string{"dns:", "proxies:", "tokyo-vless", "proxy-groups:", "rules:", "MATCH,NEXON"} {
		if !strings.Contains(out, want) {
			t.Fatalf("clash output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderWithTemplateSingbox(t *testing.T) {
	body, _, err := RenderWithTemplate("singbox", singboxStarter, &store.User{}, sampleEndpoints())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	out := string(body)
	if !strings.Contains(out, "tokyo-vless") || !strings.Contains(out, "\"outbounds\"") {
		t.Fatalf("singbox output unexpected:\n%s", out)
	}
}

func TestTemplateSyntaxErrorReported(t *testing.T) {
	_, _, err := RenderWithTemplate("clash", "{{ .Nope", &store.User{}, nil)
	if err == nil {
		t.Fatal("expected a parse error for malformed template")
	}
}

func TestClashBlockStyleAndHysteria2(t *testing.T) {
	eps := []Endpoint{{
		Name: "🇸🇪 Hysteria", Protocol: "hysteria2", Address: "stockholm.bxteam.org", Port: 2026,
		TLS: "tls", Password: "jtnFoThE39", Settings: map[string]any{"sni": "stockholm.bxteam.org"},
	}}
	out, _ := clashGen{meta: true}.Render(&store.User{}, eps)
	s := string(out)
	// Block style: a list item with indented fields, not inline {…}.
	if !strings.Contains(s, "  - name:") {
		t.Fatalf("expected block-style list item, got:\n%s", s)
	}
	if strings.Contains(s, "- {name:") {
		t.Fatalf("should not emit inline flow style:\n%s", s)
	}
	for _, want := range []string{"type: hysteria2", "password: jtnFoThE39", "port: 2026", "sni: stockholm.bxteam.org"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in:\n%s", want, s)
		}
	}
	if err := ValidateOutput("clash-meta", out); err != nil {
		t.Fatalf("block output is not valid YAML: %v", err)
	}
}

func TestHysteria2XrayAndSingbox(t *testing.T) {
	eps := []Endpoint{{
		Name: "se-hy2", Protocol: "hysteria2", Address: "stockholm.bxteam.org", Port: 2026,
		TLS: "tls", Password: "secret-auth", Settings: map[string]any{"sni": "stockholm.bxteam.org"},
	}}

	xray, _ := xrayGen{}.Render(&store.User{}, eps)
	xs := string(xray)
	for _, want := range []string{`"protocol": "hysteria"`, `"hysteriaSettings"`, `"auth": "secret-auth"`, `"version": 2`, `"network": "hysteria"`} {
		if !strings.Contains(xs, want) {
			t.Fatalf("xray hysteria missing %q:\n%s", want, xs)
		}
	}
	if err := ValidateOutput("xray", xray); err != nil {
		t.Fatalf("xray output invalid: %v", err)
	}

	sb, _ := singboxGen{}.Render(&store.User{}, eps)
	ss := string(sb)
	if !strings.Contains(ss, `"type": "hysteria2"`) || !strings.Contains(ss, `"password": "secret-auth"`) {
		t.Fatalf("singbox hysteria2 missing fields:\n%s", ss)
	}
	if err := ValidateOutput("singbox", sb); err != nil {
		t.Fatalf("singbox output invalid: %v", err)
	}
}

func TestLinksGenerator(t *testing.T) {
	out, ctype := linksGen{}.Render(&store.User{}, sampleEndpoints())
	if !strings.Contains(ctype, "text/plain") {
		t.Fatalf("content type = %q", ctype)
	}
	if !strings.Contains(string(out), "vless://") {
		t.Fatalf("expected a plain vless:// link, got:\n%s", out)
	}
}

func TestValidateOutput(t *testing.T) {
	if err := ValidateOutput("clash", []byte("a: 1\nb: 2\n")); err != nil {
		t.Fatalf("valid YAML rejected: %v", err)
	}
	if err := ValidateOutput("clash", []byte("foo: [1, 2\nbar")); err == nil {
		t.Fatal("expected invalid YAML to be rejected")
	}
	if err := ValidateOutput("xray", []byte(`{"a":1}`)); err != nil {
		t.Fatalf("valid JSON rejected: %v", err)
	}
	if err := ValidateOutput("singbox", []byte("{not json")); err == nil {
		t.Fatal("expected invalid JSON to be rejected")
	}
	if err := ValidateOutput("links", []byte("anything")); err != nil {
		t.Fatalf("links should always validate: %v", err)
	}
}

func TestUnsupportedFormat(t *testing.T) {
	if SupportsTemplate("base64") {
		t.Fatal("base64 should not support templates")
	}
	if _, _, err := RenderWithTemplate("base64", "x", &store.User{}, nil); err == nil {
		t.Fatal("expected error for unsupported format")
	}
}
