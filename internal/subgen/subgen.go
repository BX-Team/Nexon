// Package subgen renders a user's proxies into subscription formats (base64, clash, singbox, xray...).
package subgen

import (
	"encoding/json"
	"fmt"

	"github.com/BX-Team/Nexon/internal/store"
)

// Endpoint is one connectable proxy — a user's secret resolved against a single node inbound.
type Endpoint struct {
	Name     string // display name, e.g. "node1-vless"
	Protocol string
	Address  string
	Port     int
	Network  string
	TLS      string
	// Resolved per-protocol credential + transport fields.
	UUID     string
	Password string
	Method   string
	Flow     string
	Settings map[string]any // sni, host, path, reality pbk/sid...
}

// Generator renders endpoints into a concrete subscription body.
type Generator interface {
	// Render returns the response body and the content type.
	Render(user *store.User, eps []Endpoint) (body []byte, contentType string)
}

// Registry maps format names to generators.
var Registry = map[string]Generator{
	"base64":     base64Gen{},
	"links":      linksGen{},
	"xray":       xrayGen{},
	"clash":      clashGen{meta: false},
	"clash-meta": clashGen{meta: true},
	"singbox":    singboxGen{},
}

// OutputFormats lists every format a client app can be pinned to ("" = auto-detect).
func OutputFormats() []string {
	return []string{"links", "base64", "clash", "clash-meta", "singbox", "xray"}
}

// fpOr returns the endpoint's uTLS fingerprint, defaulting to "chrome".
func fpOr(e Endpoint) string {
	if f := e.setting("fp"); f != "" {
		return f
	}
	return "chrome"
}

// Get returns the generator for a format, falling back to base64.
func Get(format string) Generator {
	if g, ok := Registry[format]; ok {
		return g
	}
	return base64Gen{}
}

// BuildEndpoints resolves a user's proxies against inbounds, emitting only matching protocols.
func BuildEndpoints(u *store.User, inbounds []*store.Inbound, addrFor func(nodeID int64) (name, address string)) []Endpoint {
	var eps []Endpoint
	for _, in := range inbounds {
		nodeName, addr := addrFor(in.NodeID)
		if addr == "" {
			continue
		}
		var settings map[string]any
		if in.SettingsJSON != "" {
			_ = json.Unmarshal([]byte(in.SettingsJSON), &settings)
		}
		name := in.Remark
		if name == "" {
			name = fmt.Sprintf("%s-%s", nodeName, in.Tag)
		}
		ep := Endpoint{
			Name:     name,
			Protocol: in.Protocol,
			Address:  addr,
			Port:     in.Port,
			Network:  in.Network,
			TLS:      in.TLS,
			Settings: settings,
		}
		switch in.Protocol {
		case "vmess":
			if u.Proxies.VMess == nil {
				continue
			}
			ep.UUID = u.Proxies.VMess.ID
		case "vless":
			if u.Proxies.VLESS == nil {
				continue
			}
			ep.UUID = u.Proxies.VLESS.ID
			ep.Flow = u.Proxies.VLESS.Flow
		case "trojan":
			if u.Proxies.Trojan == nil {
				continue
			}
			ep.Password = u.Proxies.Trojan.Password
		case "shadowsocks":
			if u.Proxies.Shadowsocks == nil {
				continue
			}
			ep.Password = u.Proxies.Shadowsocks.Password
			ep.Method = u.Proxies.Shadowsocks.Method
		case "hysteria", "hysteria2":
			if u.Proxies.Hysteria == nil {
				continue
			}
			ep.Password = u.Proxies.Hysteria.Auth
		default:
			continue
		}
		eps = append(eps, ep)
	}
	return eps
}
