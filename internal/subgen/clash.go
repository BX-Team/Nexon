package subgen

import (
	"strconv"
	"strings"

	"github.com/BX-Team/Nexon/internal/store"
)

// clashGen renders Clash/Clash.Meta YAML; meta enables vless, reality, and hysteria2.
type clashGen struct{ meta bool }

func (g clashGen) Render(_ *store.User, eps []Endpoint) ([]byte, string) {
	var b strings.Builder
	b.WriteString("proxies:\n")
	names := g.writeProxies(&b, eps)
	b.WriteString("proxy-groups:\n")
	b.WriteString("  - name: NEXON\n    type: select\n    proxies:\n")
	for _, n := range names {
		b.WriteString("      - " + yamlStr(n) + "\n")
	}
	b.WriteString("rules:\n  - MATCH,NEXON\n")
	return []byte(b.String()), "text/yaml; charset=utf-8"
}

// writeProxies emits each endpoint as a block-style list item and returns proxy names.
func (g clashGen) writeProxies(b *strings.Builder, eps []Endpoint) []string {
	var names []string
	for _, e := range eps {
		kvs, name := g.proxyKVs(e)
		if kvs == nil {
			continue
		}
		b.WriteString(proxyBlock(kvs) + "\n")
		names = append(names, name)
	}
	return names
}

type kv struct{ k, v string }

// proxyBlock renders ordered key/values as a YAML list item (first key gets "  - " prefix).
func proxyBlock(kvs []kv) string {
	var b strings.Builder
	for i, p := range kvs {
		if i == 0 {
			b.WriteString("  - " + p.k + ": " + p.v + "\n")
		} else {
			b.WriteString("    " + p.k + ": " + p.v + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// proxyKVs returns the ordered Clash fields for an endpoint, or (nil, "") if unsupported.
func (g clashGen) proxyKVs(e Endpoint) ([]kv, string) {
	switch e.Protocol {
	case "shadowsocks":
		return []kv{
			{"name", yamlStr(e.Name)}, {"type", "ss"},
			{"server", e.Address}, {"port", itoa(e.Port)},
			{"cipher", e.Method}, {"password", yamlStr(e.Password)},
		}, e.Name
	case "trojan":
		kvs := []kv{
			{"name", yamlStr(e.Name)}, {"type", "trojan"},
			{"server", e.Address}, {"port", itoa(e.Port)},
			{"password", yamlStr(e.Password)}, {"udp", "true"},
		}
		if sni := e.setting("sni"); sni != "" {
			kvs = append(kvs, kv{"sni", yamlStr(sni)})
		}
		if fp := e.setting("fp"); fp != "" {
			kvs = append(kvs, kv{"client-fingerprint", fp})
		}
		return append(kvs, g.netKVs(e)...), e.Name
	case "vmess":
		kvs := []kv{
			{"name", yamlStr(e.Name)}, {"type", "vmess"},
			{"server", e.Address}, {"port", itoa(e.Port)},
			{"uuid", e.UUID}, {"alterId", "0"}, {"cipher", "auto"}, {"udp", "true"},
		}
		if e.TLS == "tls" {
			kvs = append(kvs, kv{"tls", "true"})
		}
		return append(kvs, g.netKVs(e)...), e.Name
	case "vless":
		if !g.meta {
			return nil, "" // vless needs Clash.Meta
		}
		kvs := []kv{
			{"name", yamlStr(e.Name)}, {"type", "vless"},
			{"server", e.Address}, {"port", itoa(e.Port)},
			{"uuid", e.UUID}, {"udp", "true"},
		}
		if e.TLS != "" {
			kvs = append(kvs, kv{"tls", "true"})
		}
		if e.Flow != "" {
			kvs = append(kvs, kv{"flow", e.Flow})
		}
		if sni := e.setting("sni"); sni != "" {
			kvs = append(kvs, kv{"servername", yamlStr(sni)})
		}
		kvs = append(kvs, kv{"client-fingerprint", fpOr(e)})
		if e.TLS == "reality" {
			if pbk := e.setting("pbk"); pbk != "" {
				ro := "{public-key: " + yamlStr(pbk)
				if sid := e.setting("sid"); sid != "" {
					ro += ", short-id: " + yamlStr(sid)
				}
				kvs = append(kvs, kv{"reality-opts", ro + "}"})
			}
		}
		return append(kvs, g.netKVs(e)...), e.Name
	case "hysteria2", "hysteria":
		if !g.meta {
			return nil, "" // hysteria2 needs Clash.Meta
		}
		kvs := []kv{
			{"name", yamlStr(e.Name)}, {"type", "hysteria2"},
			{"server", e.Address}, {"port", itoa(e.Port)},
			{"password", yamlStr(e.Password)},
		}
		if sni := e.setting("sni"); sni != "" {
			kvs = append(kvs, kv{"sni", yamlStr(sni)})
		}
		return kvs, e.Name
	}
	return nil, ""
}

// netKVs renders ws/grpc transport options as block fields with inline-flow nested maps.
func (g clashGen) netKVs(e Endpoint) []kv {
	switch e.Network {
	case "ws":
		opts := "{path: " + yamlStr(e.setting("path"))
		if host := e.setting("host"); host != "" {
			opts += ", headers: {Host: " + yamlStr(host) + "}"
		}
		return []kv{{"network", "ws"}, {"ws-opts", opts + "}"}}
	case "grpc":
		return []kv{{"network", "grpc"}, {"grpc-opts", "{grpc-service-name: " + yamlStr(e.setting("path")) + "}"}}
	}
	return nil
}

func itoa(n int) string { return strconv.Itoa(n) }

// yamlStr quotes a scalar when needed.
func yamlStr(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, ":#{}[],&*?|<>=!%@` \"'") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}
