package core

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/BX-Team/Nexon/internal/store"
)

// SettingLegacySecret holds the PasarGuard jwt secret used to verify legacy subscription tokens.
const SettingLegacySecret = "migrate.pasarguard.secret"

// SettingHappProviderID holds the happ-proxy.com provider id required for the new-url header.
const SettingHappProviderID = "sub.happ.providerid"

type legacyToken struct {
	UserID   int64  // "v2,<id>,<ts>" / "v3,<id>,<ts>" tokens
	Username string // oldest "<username>,<ts>" tokens
}

// parseLegacyToken verifies a PasarGuard subscription token and extracts its payload.
// Token layout: base64url(payload) + first 10 chars of sha256(body+secret) as hex or base64url.
func parseLegacyToken(token, secret string) (legacyToken, bool) {
	if len(token) < 15 {
		return legacyToken{}, false
	}
	body, sig := token[:len(token)-10], token[len(token)-10:]
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return legacyToken{}, false
	}
	sum := sha256.Sum256([]byte(body + secret))
	if sig != hex.EncodeToString(sum[:])[:10] && sig != base64.RawURLEncoding.EncodeToString(sum[:])[:10] {
		return legacyToken{}, false
	}
	parts := strings.Split(string(raw), ",")
	switch {
	case len(parts) == 3 && (parts[0] == "v2" || parts[0] == "v3"):
		id, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return legacyToken{}, false
		}
		return legacyToken{UserID: id}, true
	case len(parts) == 2:
		return legacyToken{Username: parts[0]}, true
	}
	return legacyToken{}, false
}

// resolveLegacyToken maps a PasarGuard token to a user when a legacy secret is configured.
func (s *Service) resolveLegacyToken(token string) (*store.User, error) {
	secret, err := s.st.GetSetting(SettingLegacySecret)
	if err != nil || secret == "" {
		return nil, store.ErrNotFound
	}
	lt, ok := parseLegacyToken(token, secret)
	if !ok {
		return nil, store.ErrNotFound
	}
	if lt.Username != "" {
		return s.st.GetUserByName(lt.Username)
	}
	return s.st.GetUserByLegacyID(lt.UserID)
}
