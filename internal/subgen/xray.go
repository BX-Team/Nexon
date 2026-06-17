package subgen

import (
	"encoding/json"

	"github.com/BX-Team/Nexon/internal/store"
)

// xrayGen renders an Xray-core JSON config with one outbound per endpoint.
type xrayGen struct{}

func (xrayGen) Render(_ *store.User, eps []Endpoint) ([]byte, string) {
	outbounds := make([]map[string]any, 0, len(eps))
	for _, e := range eps {
		outbounds = append(outbounds, xrayOutbound(e))
	}
	cfg := map[string]any{
		"outbounds": outbounds,
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return b, "application/json; charset=utf-8"
}

// xrayOutbound builds one xray-core outbound object for an endpoint.
func xrayOutbound(e Endpoint) map[string]any {
	if e.Protocol == "hysteria" || e.Protocol == "hysteria2" {
		ss := map[string]any{
			"hysteriaSettings": map[string]any{"auth": e.Password, "version": 2},
			"network":          "hysteria",
		}
		if e.TLS != "" {
			ss["security"] = e.TLS
			tls := map[string]any{"alpn": []string{"h3"}}
			if sni := e.setting("sni"); sni != "" {
				tls["serverName"] = sni
			}
			ss["tlsSettings"] = tls
		}
		return map[string]any{
			"tag":            e.Name,
			"protocol":       "hysteria",
			"settings":       map[string]any{"address": e.Address, "port": e.Port, "version": 2},
			"streamSettings": ss,
		}
	}
	ob := map[string]any{
		"tag":      e.Name,
		"protocol": e.Protocol,
		"settings": xraySettings(e),
	}
	if sa := streamSettings(e); sa != nil {
		ob["streamSettings"] = sa
	}
	return ob
}

func xraySettings(e Endpoint) map[string]any {
	switch e.Protocol {
	case "vless":
		user := map[string]any{"id": e.UUID, "encryption": "none"}
		if e.Flow != "" {
			user["flow"] = e.Flow
		}
		return map[string]any{"vnext": []map[string]any{{
			"address": e.Address, "port": e.Port,
			"users": []map[string]any{user},
		}}}
	case "vmess":
		return map[string]any{"vnext": []map[string]any{{
			"address": e.Address, "port": e.Port,
			"users": []map[string]any{{"id": e.UUID, "alterId": 0}},
		}}}
	case "trojan":
		return map[string]any{"servers": []map[string]any{{
			"address": e.Address, "port": e.Port, "password": e.Password,
		}}}
	case "shadowsocks":
		return map[string]any{"servers": []map[string]any{{
			"address": e.Address, "port": e.Port, "password": e.Password, "method": e.Method,
		}}}
	}
	return map[string]any{}
}

func streamSettings(e Endpoint) map[string]any {
	net := orDefault(e.Network, "tcp")
	ss := map[string]any{"network": net}
	if e.TLS != "" {
		ss["security"] = e.TLS
	}
	switch e.TLS {
	case "tls":
		tls := map[string]any{"fingerprint": fpOr(e)}
		if sni := e.setting("sni"); sni != "" {
			tls["serverName"] = sni
		}
		ss["tlsSettings"] = tls
	case "reality":
		r := map[string]any{"fingerprint": fpOr(e)}
		if sni := e.setting("sni"); sni != "" {
			r["serverName"] = sni
		}
		if pbk := e.setting("pbk"); pbk != "" {
			r["publicKey"] = pbk
		}
		if sid := e.setting("sid"); sid != "" {
			r["shortId"] = sid
		}
		ss["realitySettings"] = r
	}
	switch net {
	case "ws":
		hdr := map[string]any{}
		if host := e.setting("host"); host != "" {
			hdr["Host"] = host
		}
		ss["wsSettings"] = map[string]any{"path": e.setting("path"), "headers": hdr}
	case "grpc":
		ss["grpcSettings"] = map[string]any{"serviceName": e.setting("path")}
	}
	return ss
}
