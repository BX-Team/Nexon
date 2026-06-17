package core

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/BX-Team/Nexon/internal/store"
	"github.com/BX-Team/Nexon/internal/subgen"
)

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// clientMatcher caches the compiled managed client-app UA patterns. It is
// reloaded whenever an admin edits the client list.
type clientMatcher struct {
	mu       sync.RWMutex
	compiled []compiledClient
	loaded   bool
}

type compiledClient struct {
	app *store.ClientApp
	re  *regexp.Regexp
}

func (s *Service) ensureClients() {
	s.clients.mu.RLock()
	loaded := s.clients.loaded
	s.clients.mu.RUnlock()
	if !loaded {
		_ = s.ReloadClients()
	}
}

// ReloadClients recompiles the managed client-app patterns from the store.
func (s *Service) ReloadClients() error {
	apps, err := s.st.ListClientApps()
	if err != nil {
		return err
	}
	compiled := make([]compiledClient, 0, len(apps))
	for _, a := range apps {
		if !a.Enabled {
			continue
		}
		re, err := regexp.Compile(a.UAPattern)
		if err != nil {
			continue // skip invalid user-defined patterns rather than failing
		}
		compiled = append(compiled, compiledClient{app: a, re: re})
	}
	s.clients.mu.Lock()
	s.clients.compiled = compiled
	s.clients.loaded = true
	s.clients.mu.Unlock()
	return nil
}

// SubFormat returns the output format pinned to the client app matching this
// UA, or "" if none matches or the app has no pinned format (→ caller falls
// back to UA detection rules).
func (s *Service) SubFormat(ua string) string {
	if app, ok := s.MatchClientApp(ua); ok {
		return app.Format
	}
	return ""
}

// MatchClientApp returns the managed client app whose UA pattern matches, if any.
func (s *Service) MatchClientApp(ua string) (*store.ClientApp, bool) {
	if ua == "" {
		return nil, false
	}
	s.ensureClients()
	s.clients.mu.RLock()
	defer s.clients.mu.RUnlock()
	for _, c := range s.clients.compiled {
		if c.re.MatchString(ua) {
			return c.app, true
		}
	}
	return nil, false
}

// IsKnownClient reports whether a UA is a recognised VPN client (not a browser or empty UA).
func (s *Service) IsKnownClient(ua string) bool {
	if ua == "" || strings.HasPrefix(ua, "Mozilla/") {
		return false
	}
	_, ok := s.MatchClientApp(ua)
	return ok
}

// VisibleDevices returns a user's non-revoked devices that belong to a recognised
// VPN client (browsers, curl and other noise are filtered out).
func (s *Service) VisibleDevices(userID int64) ([]*store.Device, error) {
	all, err := s.st.ListDevices(userID)
	if err != nil {
		return nil, err
	}
	out := make([]*store.Device, 0, len(all))
	for _, d := range all {
		if d.Revoked {
			continue
		}
		if !s.IsKnownClient(d.UserAgent) {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

// SubResponseHeaders builds global and per-client-app headers for a subscription response.
func (s *Service) SubResponseHeaders(ua string) map[string]string {
	h := map[string]string{}
	if v, _ := s.st.GetSetting("sub.profile_title"); v != "" {
		h["profile-title"] = "base64:" + b64(v)
	}
	if v, _ := s.st.GetSetting("sub.support_url"); v != "" {
		h["support-url"] = v
	}
	if v, _ := s.st.GetSetting("sub.announce"); v != "" {
		h["announce"] = "base64:" + b64(v)
	}
	if app, ok := s.MatchClientApp(ua); ok {
		for k, v := range app.Headers {
			h[k] = v
		}
	}
	return h
}

// ---- Client-app admin CRUD (reload cache after every mutation) ----

func (s *Service) ListClientApps() ([]*store.ClientApp, error) { return s.st.ListClientApps() }

func (s *Service) CreateClientApp(c *store.ClientApp) error {
	if err := validateClientApp(c); err != nil {
		return err
	}
	if err := s.st.CreateClientApp(c); err != nil {
		return err
	}
	return s.ReloadClients()
}

func (s *Service) UpdateClientApp(c *store.ClientApp) error {
	if err := validateClientApp(c); err != nil {
		return err
	}
	if err := s.st.UpdateClientApp(c); err != nil {
		return err
	}
	return s.ReloadClients()
}

func (s *Service) DeleteClientApp(id int64) error {
	if err := s.st.DeleteClientApp(id); err != nil {
		return err
	}
	return s.ReloadClients()
}

func validateClientApp(c *store.ClientApp) error {
	c.Name = strings.TrimSpace(c.Name)
	c.UAPattern = strings.TrimSpace(c.UAPattern)
	if c.Name == "" {
		return fmt.Errorf("название обязательно")
	}
	if c.UAPattern == "" {
		return fmt.Errorf("UA-паттерн обязателен")
	}
	if _, err := regexp.Compile(c.UAPattern); err != nil {
		return fmt.Errorf("некорректный regex: %w", err)
	}
	c.Format = strings.TrimSpace(c.Format)
	if c.Format != "" && !validFormat(c.Format) {
		return fmt.Errorf("неизвестный формат %q", c.Format)
	}
	return nil
}

func validFormat(f string) bool {
	for _, v := range subgen.OutputFormats() {
		if v == f {
			return true
		}
	}
	return false
}
