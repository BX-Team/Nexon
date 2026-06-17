package subgen

import (
	"encoding/base64"
	"strings"

	"github.com/BX-Team/Nexon/internal/store"
)

// base64Gen renders the universal base64-encoded newline-joined URI list.
type base64Gen struct{}

func (base64Gen) Render(_ *store.User, eps []Endpoint) ([]byte, string) {
	enc := base64.StdEncoding.EncodeToString([]byte(linkList(eps)))
	return []byte(enc), "text/plain; charset=utf-8"
}

// linksGen renders the raw newline-joined share links (vless://, trojan://, ...)
// without base64-encoding, for clients that consume plain links.
type linksGen struct{}

func (linksGen) Render(_ *store.User, eps []Endpoint) ([]byte, string) {
	return []byte(linkList(eps)), "text/plain; charset=utf-8"
}

func linkList(eps []Endpoint) string {
	var lines []string
	for _, e := range eps {
		if uri := e.URI(); uri != "" {
			lines = append(lines, uri)
		}
	}
	return strings.Join(lines, "\n")
}
