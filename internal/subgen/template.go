package subgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/BX-Team/Nexon/internal/store"
)

// TemplateData is passed to custom subscription templates: .Proxies (format-native entries),
// .Names (proxy names/tags), and .User (subscriber info).
type TemplateData struct {
	User    *store.User
	Proxies string
	Names   []string
}

// fragmenter renders just the proxy/outbound entries for a format, plus their
// names, so a template can inject them and own everything else.
type fragmenter interface {
	fragment(eps []Endpoint) (proxies string, names []string)
	contentType() string
}

var fragmenters = map[string]fragmenter{
	"clash":      clashFrag{meta: false},
	"clash-meta": clashFrag{meta: true},
	"singbox":    singboxFrag{},
	"xray":       xrayFrag{},
}

// TemplateFormats lists formats that support custom templates.
func TemplateFormats() []string { return []string{"clash", "clash-meta", "singbox", "xray"} }

// SupportsTemplate reports whether a format can be rendered through a template.
func SupportsTemplate(format string) bool { _, ok := fragmenters[format]; return ok }

// RenderWithTemplate executes a custom template for format, returning body, content type, and any error.
func RenderWithTemplate(format, body string, u *store.User, eps []Endpoint) ([]byte, string, error) {
	f, ok := fragmenters[format]
	if !ok {
		return nil, "", fmt.Errorf("format %q does not support templates", format)
	}
	proxies, names := f.fragment(eps)
	tmpl, err := template.New(format).Option("missingkey=zero").Parse(body)
	if err != nil {
		return nil, "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, TemplateData{User: u, Proxies: proxies, Names: names}); err != nil {
		return nil, "", fmt.Errorf("execute template: %w", err)
	}
	return buf.Bytes(), f.contentType(), nil
}

// StarterTemplate returns a working template for a format, used when no custom template exists.
func StarterTemplate(format string) string {
	switch format {
	case "clash", "clash-meta":
		return clashStarter
	case "singbox":
		return singboxStarter
	case "xray":
		return xrayStarter
	}
	return ""
}

type clashFrag struct{ meta bool }

func (c clashFrag) contentType() string { return "text/yaml; charset=utf-8" }

func (c clashFrag) fragment(eps []Endpoint) (string, []string) {
	g := clashGen{meta: c.meta}
	var b strings.Builder
	names := g.writeProxies(&b, eps)
	return strings.TrimRight(b.String(), "\n"), names
}

type singboxFrag struct{}

func (singboxFrag) contentType() string { return "application/json; charset=utf-8" }

func (singboxFrag) fragment(eps []Endpoint) (string, []string) {
	var objs []string
	var tags []string
	for _, e := range eps {
		ob := singboxOutbound(e)
		if ob == nil {
			continue
		}
		objs = append(objs, marshalIndented(ob))
		tags = append(tags, e.Name)
	}
	return strings.Join(objs, ",\n"), tags
}

type xrayFrag struct{}

func (xrayFrag) contentType() string { return "application/json; charset=utf-8" }

func (xrayFrag) fragment(eps []Endpoint) (string, []string) {
	var objs []string
	var tags []string
	for _, e := range eps {
		objs = append(objs, marshalIndented(xrayOutbound(e)))
		tags = append(tags, e.Name)
	}
	return strings.Join(objs, ",\n"), tags
}

func marshalIndented(v any) string {
	b, _ := json.MarshalIndent(v, "    ", "  ")
	return "    " + string(b)
}

// {{ .Proxies }} expands to the generated proxy entries; {{ .Names }} are their
// names. Everything else (tun/dns/sniffer/rules) is yours to edit.
const clashStarter = `mode: rule
mixed-port: 7890
ipv6: true

tun:
  enable: true
  stack: mixed
  dns-hijack:
    - "any:53"
  auto-route: true
  auto-detect-interface: true
  strict-route: true

dns:
  enable: true
  listen: :1053
  ipv6: true
  nameserver:
    - 'https://1.1.1.1/dns-query'
    - 'https://8.8.8.8/dns-query'

sniffer:
  enable: true
  override-destination: true
  sniff:
    HTTP:
      ports: [80, 8080-8880]
    TLS:
      ports: [443, 8443]
    QUIC:
      ports: [443, 8443]

proxies:
{{ .Proxies }}

proxy-groups:
  - name: NEXON
    type: select
    proxies:
{{- range .Names }}
      - {{ . }}
{{- end }}

rules:
  - MATCH,NEXON
`

const xrayStarter = `{
  "log": { "access": "", "error": "", "loglevel": "warning" },
  "inbounds": [
    {
      "tag": "socks", "port": 10808, "listen": "0.0.0.0", "protocol": "socks",
      "sniffing": { "enabled": true, "destOverride": ["http", "tls"], "routeOnly": false },
      "settings": { "auth": "noauth", "udp": true, "allowTransparent": false }
    },
    {
      "tag": "http", "port": 10809, "listen": "0.0.0.0", "protocol": "http",
      "sniffing": { "enabled": true, "destOverride": ["http", "tls"], "routeOnly": false },
      "settings": { "auth": "noauth", "udp": true, "allowTransparent": false }
    }
  ],
  "outbounds": [
{{ .Proxies }},
    { "protocol": "freedom", "tag": "DIRECT" },
    { "protocol": "blackhole", "tag": "BLOCK" }
  ],
  "dns": { "servers": ["1.1.1.1", "8.8.8.8"] },
  "routing": { "domainStrategy": "AsIs", "rules": [] }
}
`

const singboxStarter = `{
  "log": { "level": "warn", "timestamp": false },
  "dns": {
    "servers": [
      { "type": "udp", "tag": "dns-remote", "server": "1.1.1.2", "detour": "proxy" },
      { "type": "local", "tag": "dns-local" }
    ],
    "final": "dns-remote"
  },
  "inbounds": [
    {
      "type": "tun", "tag": "tun-in", "interface_name": "sing-tun",
      "address": ["172.19.0.1/30", "fdfe:dcba:9876::1/126"],
      "auto_route": true,
      "route_exclude_address": ["192.168.0.0/16", "10.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "fe80::/10", "fc00::/7"]
    }
  ],
  "outbounds": [
{{ .Proxies }},
    { "type": "selector", "tag": "proxy", "outbounds": [{{ range $i, $n := .Names }}{{ if $i }}, {{ end }}"{{ $n }}"{{ end }}{{ if .Names }}, {{ end }}"Best Latency"], "interrupt_exist_connections": true },
    { "type": "urltest", "tag": "Best Latency", "outbounds": [{{ range $i, $n := .Names }}{{ if $i }}, {{ end }}"{{ $n }}"{{ end }}] },
    { "type": "direct", "tag": "direct" }
  ],
  "route": {
    "rules": [
      { "inbound": "tun-in", "action": "sniff" },
      { "protocol": "dns", "action": "hijack-dns" }
    ],
    "final": "proxy",
    "auto_detect_interface": true,
    "override_android_vpn": true
  },
  "experimental": { "cache_file": { "enabled": true, "store_dns": true } }
}
`
