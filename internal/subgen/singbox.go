package subgen

import (
	"encoding/json"

	"github.com/BX-Team/Nexon/internal/store"
)

// singboxGen renders a sing-box JSON config with one outbound per endpoint plus a selector.
type singboxGen struct{}

func (singboxGen) Render(_ *store.User, eps []Endpoint) ([]byte, string) {
	outbounds := make([]map[string]any, 0, len(eps)+2)
	var tags []string
	for _, e := range eps {
		ob := singboxOutbound(e)
		if ob == nil {
			continue
		}
		outbounds = append(outbounds, ob)
		tags = append(tags, e.Name)
	}
	// Selector + direct.
	outbounds = append(outbounds,
		map[string]any{"type": "selector", "tag": "NEXON", "outbounds": tags},
		map[string]any{"type": "direct", "tag": "direct"},
	)
	cfg := map[string]any{
		"outbounds": outbounds,
		"route":     map[string]any{"final": "NEXON"},
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return b, "application/json; charset=utf-8"
}

func singboxOutbound(e Endpoint) map[string]any {
	base := map[string]any{"tag": e.Name, "server": e.Address, "server_port": e.Port}
	withTLS := func(m map[string]any) {
		if e.TLS != "" {
			tls := map[string]any{
				"enabled": true,
				"utls":    map[string]any{"enabled": true, "fingerprint": fpOr(e)},
			}
			if sni := e.setting("sni"); sni != "" {
				tls["server_name"] = sni
			}
			if e.TLS == "reality" {
				reality := map[string]any{"enabled": true}
				if pbk := e.setting("pbk"); pbk != "" {
					reality["public_key"] = pbk
				}
				if sid := e.setting("sid"); sid != "" {
					reality["short_id"] = e.setting("sid")
				}
				tls["reality"] = reality
			}
			m["tls"] = tls
		}
	}
	switch e.Protocol {
	case "vless":
		base["type"] = "vless"
		base["uuid"] = e.UUID
		base["packet_encoding"] = "xudp"
		if e.Flow != "" {
			base["flow"] = e.Flow
		}
		withTLS(base)
	case "vmess":
		base["type"] = "vmess"
		base["uuid"] = e.UUID
		base["alter_id"] = 0
		base["security"] = "auto"
		withTLS(base)
	case "trojan":
		base["type"] = "trojan"
		base["password"] = e.Password
		withTLS(base)
	case "shadowsocks":
		base["type"] = "shadowsocks"
		base["method"] = e.Method
		base["password"] = e.Password
	case "hysteria", "hysteria2":
		base["type"] = "hysteria2"
		base["password"] = e.Password
		withTLS(base)
	default:
		return nil
	}
	return base
}
