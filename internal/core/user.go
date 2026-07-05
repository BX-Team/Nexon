package core

import (
	"fmt"
	"strconv"
	"time"

	"github.com/BX-Team/Nexon/internal/secrets"
	"github.com/BX-Team/Nexon/internal/store"
)

// CreateUserParams holds inputs for AddUser.
type CreateUserParams struct {
	Username      string
	DataLimit     int64      // bytes, 0 = unlimited
	ExpireAt      *time.Time // nil = never
	HWIDLimit     int        // 0 = no limit
	ResetStrategy string     // no_reset|day|week|month
}

// AddUser creates a user with a fresh proxy bundle and subscription token.
func (s *Service) AddUser(p CreateUserParams) (*store.User, error) {
	if p.Username == "" {
		return nil, fmt.Errorf("username required")
	}
	if _, err := s.st.GetUserByName(p.Username); err == nil {
		return nil, fmt.Errorf("user %q already exists", p.Username)
	}
	proxies, err := secrets.GenerateProxies()
	if err != nil {
		return nil, fmt.Errorf("generate proxies: %w", err)
	}
	reset := p.ResetStrategy
	if reset == "" {
		reset = "no_reset"
	}
	switch reset {
	case "no_reset", "day", "week", "month":
	default:
		return nil, fmt.Errorf("invalid reset strategy %q (no_reset|day|week|month)", reset)
	}
	// Anchor the traffic-reset period at creation time. Without this, a new user
	// has a nil TrafficResetAt, which resetTrafficPeriod treats as "overdue" and
	// zeroes their traffic on the very first poll cycle.
	now := time.Now()
	u := &store.User{
		Username:             p.Username,
		Status:               store.StatusActive,
		DataLimit:            p.DataLimit,
		TrafficResetStrategy: reset,
		ExpireAt:             p.ExpireAt,
		HWIDLimit:            p.HWIDLimit,
		Proxies:              proxies,
		SubToken:             secrets.SubToken(),
		TrafficResetAt:       &now,
	}
	if err := s.st.CreateUser(u); err != nil {
		return nil, err
	}
	// Project onto nodes (stub today, real gRPC later).
	s.syncUserToNodes(u)
	return u, nil
}

// GetUser fetches a user by name.
func (s *Service) GetUser(name string) (*store.User, error) {
	return s.st.GetUserByName(name)
}

// ListUsers returns users filtered by status ("" = all).
func (s *Service) ListUsers(status string) ([]*store.User, error) {
	return s.st.ListUsers(status)
}

// SetUserParams holds optional mutations; nil pointers are left unchanged.
type SetUserParams struct {
	DataLimit *int64
	ExpireAt  *time.Time
	// ClearExpire removes the expiry (nil ExpireAt alone means "unchanged").
	ClearExpire bool
	HWIDLimit   *int
}

// SetUser applies mutations to an existing user and re-syncs.
func (s *Service) SetUser(name string, p SetUserParams) (*store.User, error) {
	u, err := s.st.GetUserByName(name)
	if err != nil {
		return nil, err
	}
	if p.DataLimit != nil {
		u.DataLimit = *p.DataLimit
	}
	if p.ClearExpire {
		u.ExpireAt = nil
		u.ExpiryNotifiedFor = nil
	} else if p.ExpireAt != nil {
		u.ExpireAt = p.ExpireAt
	}
	if p.HWIDLimit != nil {
		u.HWIDLimit = *p.HWIDLimit
	}
	if err := s.st.UpdateUser(u); err != nil {
		return nil, err
	}
	s.syncUserToNodes(u)
	return u, nil
}

// SetStatus enables/disables a user and pushes the change to nodes.
func (s *Service) SetStatus(name string, status store.UserStatus) error {
	u, err := s.st.GetUserByName(name)
	if err != nil {
		return err
	}
	u.Status = status
	if err := s.st.UpdateUser(u); err != nil {
		return err
	}
	if status == store.StatusActive {
		s.syncUserToNodes(u)
	} else {
		s.removeUserFromNodes(u)
	}
	return nil
}

// ResetTraffic zeroes a user's used traffic.
func (s *Service) ResetTraffic(name string) error {
	u, err := s.st.GetUserByName(name)
	if err != nil {
		return err
	}
	u.UsedTraffic = 0
	if u.Status == store.StatusLimited {
		u.Status = store.StatusActive
		s.syncUserToNodes(u)
	}
	return s.st.UpdateUser(u)
}

// RotateToken issues a fresh subscription token for compromise recovery.
func (s *Service) RotateToken(name string) (*store.User, error) {
	u, err := s.st.GetUserByName(name)
	if err != nil {
		return nil, err
	}
	u.SubToken = secrets.SubToken()
	if err := s.st.UpdateUser(u); err != nil {
		return nil, err
	}
	// A rotation must also cut off imported PasarGuard tokens, or the leak survives it.
	_ = s.st.DeleteLegacyMapping(u.ID)
	return u, nil
}

// DeleteUser removes a user from the store and all nodes.
func (s *Service) DeleteUser(name string) error {
	u, err := s.st.GetUserByName(name)
	if err != nil {
		return err
	}
	s.removeUserFromNodes(u)
	return s.st.DeleteUser(u.ID)
}

// ParseDuration parses durations like "30d", "+15d", "12h" into a future time
// relative to base. A leading '+' is allowed and ignored.
func ParseDuration(spec string, base time.Time) (*time.Time, error) {
	if spec == "" {
		return nil, nil
	}
	if spec[0] == '+' {
		spec = spec[1:]
	}
	// Support the "d" suffix that time.ParseDuration lacks.
	if n := len(spec); n > 1 && spec[n-1] == 'd' {
		days, err := strconv.Atoi(spec[:n-1])
		if err != nil {
			return nil, fmt.Errorf("invalid duration %q", spec)
		}
		t := base.AddDate(0, 0, days)
		return &t, nil
	}
	d, err := time.ParseDuration(spec)
	if err != nil {
		return nil, fmt.Errorf("invalid duration %q", spec)
	}
	t := base.Add(d)
	return &t, nil
}
