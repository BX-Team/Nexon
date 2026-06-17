package store

import (
	"encoding/json"
	"time"
)

// UserStatus enumerates the lifecycle states of a user.
type UserStatus string

const (
	StatusActive   UserStatus = "active"
	StatusDisabled UserStatus = "disabled"
	StatusLimited  UserStatus = "limited"
	StatusExpired  UserStatus = "expired"
)

// Proxies is the per-user secret bundle, generated upfront to support any inbound.
type Proxies struct {
	VMess       *VMessProxy       `json:"vmess,omitempty"`
	VLESS       *VLESSProxy       `json:"vless,omitempty"`
	Trojan      *TrojanProxy      `json:"trojan,omitempty"`
	Shadowsocks *ShadowsocksProxy `json:"shadowsocks,omitempty"`
	Hysteria    *HysteriaProxy    `json:"hysteria,omitempty"`
}

type VMessProxy struct {
	ID string `json:"id"`
}

type VLESSProxy struct {
	ID   string `json:"id"`
	Flow string `json:"flow,omitempty"`
}

type TrojanProxy struct {
	Password string `json:"password"`
}

type ShadowsocksProxy struct {
	Password string `json:"password"`
	Method   string `json:"method"`
}

type HysteriaProxy struct {
	Auth string `json:"auth"`
}

// User is the source-of-truth record for a subscriber.
type User struct {
	ID                   int64
	Username             string
	CreatedAt            time.Time
	Status               UserStatus
	DataLimit            int64
	UsedTraffic          int64
	TrafficResetStrategy string
	ExpireAt             *time.Time
	HWIDLimit            int
	Proxies              Proxies
	SubToken             string
	SubLastUserAgent     string
	SubUpdatedAt         *time.Time
	TrafficResetAt       *time.Time
	// ExpiryNotifiedFor is the expire_at the user was already warned about (3-day reminder).
	ExpiryNotifiedFor *time.Time
	// GroupID picks which node group serves this user (nil = default group).
	GroupID *int64
}

// MarshalProxies serializes the proxy bundle for storage.
func (p Proxies) Marshal() (string, error) {
	b, err := json.Marshal(p)
	return string(b), err
}

// Node is a remote Xray host Nexon projects state onto.
type Node struct {
	ID          int64
	Name        string
	Address     string
	APIPort     int
	Status      string
	XrayVersion string
	LastSeen    *time.Time
	CreatedAt   time.Time
	// GroupID places this node in a node group (nil = default group).
	GroupID *int64
}

// Inbound describes a protocol entry available on a node.
type Inbound struct {
	ID           int64
	NodeID       int64
	Tag          string
	Protocol     string
	Network      string
	TLS          string
	Port         int
	SettingsJSON string
}

// Device is a registered client (HWID/UA) for device-limit enforcement.
type Device struct {
	ID        int64
	UserID    int64
	HWID      string
	UserAgent string
	FirstSeen time.Time
	LastSeen  time.Time
	IPLast    string
	Revoked   bool
}
