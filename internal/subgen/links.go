package subgen

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// setting reads a string from an endpoint's Settings map.
func (e Endpoint) setting(key string) string {
	if e.Settings == nil {
		return ""
	}
	if v, ok := e.Settings[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// URI renders a single endpoint as a share link (vless://, vmess://, ...).
func (e Endpoint) URI() string {
	switch e.Protocol {
	case "vless":
		return e.vlessURI()
	case "vmess":
		return e.vmessURI()
	case "trojan":
		return e.trojanURI()
	case "shadowsocks":
		return e.ssURI()
	case "hysteria", "hysteria2":
		return e.hysteria2URI()
	}
	return ""
}

func (e Endpoint) transportQuery() url.Values {
	q := url.Values{}
	net := e.Network
	if net == "" {
		net = "tcp"
	}
	q.Set("type", net)
	if e.TLS != "" {
		q.Set("security", e.TLS)
	}
	if sni := e.setting("sni"); sni != "" {
		q.Set("sni", sni)
	}
	if host := e.setting("host"); host != "" {
		q.Set("host", host)
	}
	if path := e.setting("path"); path != "" {
		q.Set("path", path)
	}
	if pbk := e.setting("pbk"); pbk != "" { // reality public key
		q.Set("pbk", pbk)
	}
	if sid := e.setting("sid"); sid != "" { // reality short id
		q.Set("sid", sid)
	}
	if fp := e.setting("fp"); fp != "" { // utls fingerprint
		q.Set("fp", fp)
	}
	return q
}

func (e Endpoint) vlessURI() string {
	q := e.transportQuery()
	if e.Flow != "" && e.TLS == "reality" {
		q.Set("flow", e.Flow)
	}
	u := url.URL{
		Scheme:   "vless",
		User:     url.User(e.UUID),
		Host:     fmt.Sprintf("%s:%d", e.Address, e.Port),
		RawQuery: q.Encode(),
		Fragment: e.Name,
	}
	return u.String()
}

func (e Endpoint) vmessURI() string {
	// vmess uses a base64-encoded JSON blob.
	obj := map[string]any{
		"v":    "2",
		"ps":   e.Name,
		"add":  e.Address,
		"port": strconv.Itoa(e.Port),
		"id":   e.UUID,
		"aid":  "0",
		"net":  orDefault(e.Network, "tcp"),
		"type": "none",
		"host": e.setting("host"),
		"path": e.setting("path"),
		"tls":  e.TLS,
		"sni":  e.setting("sni"),
	}
	b, _ := json.Marshal(obj)
	return "vmess://" + base64.StdEncoding.EncodeToString(b)
}

func (e Endpoint) trojanURI() string {
	q := e.transportQuery()
	u := url.URL{
		Scheme:   "trojan",
		User:     url.User(e.Password),
		Host:     fmt.Sprintf("%s:%d", e.Address, e.Port),
		RawQuery: q.Encode(),
		Fragment: e.Name,
	}
	return u.String()
}

// hysteria2URI renders a hysteria2://auth@host:port/?sni=...#name share link.
func (e Endpoint) hysteria2URI() string {
	q := url.Values{}
	sni := e.setting("sni")
	if sni == "" {
		sni = e.Address
	}
	q.Set("sni", sni)
	if obfs := e.setting("obfs"); obfs != "" {
		q.Set("obfs", obfs)
		if op := e.setting("obfs-password"); op != "" {
			q.Set("obfs-password", op)
		}
	}
	u := url.URL{
		Scheme:   "hysteria2",
		User:     url.User(e.Password),
		Host:     fmt.Sprintf("%s:%d", e.Address, e.Port),
		Path:     "/",
		RawQuery: q.Encode(),
		Fragment: e.Name,
	}
	return u.String()
}

func (e Endpoint) ssURI() string {
	userinfo := base64.RawURLEncoding.EncodeToString([]byte(e.Method + ":" + e.Password))
	return fmt.Sprintf("ss://%s@%s:%d#%s", userinfo, e.Address, e.Port, url.QueryEscape(e.Name))
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
