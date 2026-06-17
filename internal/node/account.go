package node

import (
	"fmt"

	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	hysteria "github.com/xtls/xray-core/proxy/hysteria/account"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	"github.com/xtls/xray-core/proxy/trojan"
	"github.com/xtls/xray-core/proxy/vless"
	"github.com/xtls/xray-core/proxy/vmess"
	"google.golang.org/protobuf/proto"
)

// ssCiphers maps Shadowsocks method names to xray-core CipherType.
var ssCiphers = map[string]shadowsocks.CipherType{
	"aes-128-gcm":            shadowsocks.CipherType_AES_128_GCM,
	"aes-256-gcm":            shadowsocks.CipherType_AES_256_GCM,
	"chacha20-poly1305":      shadowsocks.CipherType_CHACHA20_POLY1305,
	"chacha20-ietf-poly1305": shadowsocks.CipherType_CHACHA20_POLY1305,
	"xchacha20-poly1305":     shadowsocks.CipherType_XCHACHA20_POLY1305,
	"none":                   shadowsocks.CipherType_NONE,
}

// xrayAccount builds the protocol-specific xray-core account message from a
// Nexon AccountUser's resolved secrets.
func xrayAccount(u AccountUser) (proto.Message, error) {
	switch u.Protocol {
	case "vmess":
		return &vmess.Account{Id: u.Secret["id"]}, nil
	case "vless":
		return &vless.Account{Id: u.Secret["id"], Flow: u.Secret["flow"], Encryption: "none"}, nil
	case "trojan":
		return &trojan.Account{Password: u.Secret["password"]}, nil
	case "shadowsocks":
		cipher, ok := ssCiphers[u.Secret["method"]]
		if !ok {
			cipher = shadowsocks.CipherType_CHACHA20_POLY1305
		}
		return &shadowsocks.Account{Password: u.Secret["password"], CipherType: cipher}, nil
	case "hysteria", "hysteria2":
		return &hysteria.Account{Auth: u.Secret["auth"]}, nil
	default:
		return nil, fmt.Errorf("unsupported protocol %q", u.Protocol)
	}
}

// xrayUser wraps an AccountUser into an xray-core protocol.User with its typed
// account, ready for an AddUserOperation.
func xrayUser(u AccountUser) (*protocol.User, error) {
	acc, err := xrayAccount(u)
	if err != nil {
		return nil, err
	}
	return &protocol.User{
		Level:   0,
		Email:   u.Email,
		Account: serial.ToTypedMessage(acc),
	}, nil
}
