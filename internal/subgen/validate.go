package subgen

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ValidateOutput checks that a rendered subscription body is well-formed for its
// format: valid YAML for clash, valid JSON for singbox/xray. links/base64 are
// always accepted.
func ValidateOutput(format string, out []byte) error {
	switch format {
	case "clash", "clash-meta":
		var v any
		if err := yaml.Unmarshal(out, &v); err != nil {
			return fmt.Errorf("invalid YAML: %w", err)
		}
	case "singbox", "xray":
		if !json.Valid(out) {
			return fmt.Errorf("invalid JSON")
		}
	}
	return nil
}

// SampleEndpoints returns a small, representative set of endpoints used to
// validate and preview templates without a live subscription.
func SampleEndpoints() []Endpoint {
	return []Endpoint{
		{
			Name: "node1-vless", Protocol: "vless", Address: "203.0.113.10", Port: 443,
			Network: "tcp", TLS: "reality", UUID: "11111111-1111-1111-1111-111111111111",
			Flow:     "xtls-rprx-vision",
			Settings: map[string]any{"sni": "example.com", "pbk": "PUBLICKEY", "sid": "abcd"},
		},
		{
			Name: "node2-trojan", Protocol: "trojan", Address: "203.0.113.20", Port: 443,
			Network: "ws", TLS: "tls", Password: "sample-pass",
			Settings: map[string]any{"sni": "example.com", "host": "example.com", "path": "/ws"},
		},
		{
			Name: "node3-hysteria2", Protocol: "hysteria2", Address: "203.0.113.30", Port: 2026,
			TLS: "tls", Password: "sample-auth",
			Settings: map[string]any{"sni": "example.com"},
		},
	}
}
