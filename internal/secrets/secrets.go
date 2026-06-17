// Package secrets generates the full per-user proxy secret bundle at user creation.
package secrets

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/BX-Team/Nexon/internal/store"
)

// GenerateProxies builds a complete Proxies bundle with fresh secrets.
func GenerateProxies() (store.Proxies, error) {
	uid, err := uuidV4()
	if err != nil {
		return store.Proxies{}, err
	}
	return store.Proxies{
		VMess:       &store.VMessProxy{ID: uid},
		VLESS:       &store.VLESSProxy{ID: uid, Flow: "xtls-rprx-vision"},
		Trojan:      &store.TrojanProxy{Password: randHex(16)},
		Shadowsocks: &store.ShadowsocksProxy{Password: randBase64(16), Method: "chacha20-ietf-poly1305"},
		Hysteria:    &store.HysteriaProxy{Auth: randHex(16)},
	}, nil
}

// SubToken returns a random, hard-to-guess subscription token.
func SubToken() string {
	return randURLSafe(24)
}

func uuidV4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failure is unrecoverable
	}
	return b
}

func randHex(n int) string     { return hex.EncodeToString(randBytes(n)) }
func randBase64(n int) string  { return base64.StdEncoding.EncodeToString(randBytes(n)) }
func randURLSafe(n int) string { return base64.RawURLEncoding.EncodeToString(randBytes(n)) }
