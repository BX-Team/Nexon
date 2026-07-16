package core

import (
	"errors"
	"strings"
	"time"

	"github.com/BX-Team/Nexon/internal/store"
	"github.com/BX-Team/Nexon/internal/subgen"
)

// ErrSubDenied is returned when a subscription fetch is rejected (disabled,
// expired, or device-limit exceeded).
var ErrSubDenied = errors.New("subscription denied")

// SubResult bundles everything the sub server needs to render a response.
type SubResult struct {
	User      *store.User
	Endpoints []subgen.Endpoint
	NewDevice bool
	// Legacy marks a fetch through an imported PasarGuard token (native sub_token did not match).
	Legacy bool
}

// Subscription resolves a token, enforces status and HWID limit, and builds connectable endpoints.
func (s *Service) Subscription(token, ua, hwid, ip string) (*SubResult, error) {
	legacy := false
	u, err := s.st.GetUserByToken(token)
	if errors.Is(err, store.ErrNotFound) {
		u, err = s.resolveLegacyToken(token)
		legacy = err == nil
	}
	if err != nil {
		return nil, err
	}
	if u.Status == store.StatusDisabled {
		return nil, ErrSubDenied
	}
	if u.ExpireAt != nil && u.ExpireAt.Before(time.Now()) {
		return nil, ErrSubDenied
	}

	// Device registration + HWID limit enforcement. Browsers are skipped: they
	// get the HTML dashboard, must never consume a device slot, and must not be
	// locked out of their own dashboard once the limit is reached.
	newDevice := false
	browser := strings.HasPrefix(ua, "Mozilla/")
	if hwid != "" || (ua != "" && !browser) {
		dev, created, derr := s.st.RegisterDevice(u.ID, hwid, ua, ip)
		if derr == nil {
			newDevice = created
			// A fetch needs a free slot when the device is brand-new, or when a
			// previously kicked (revoked) device comes back. Already-active
			// devices are always allowed. Without this, a revoked device would be
			// matched by UA on retry (created=false) and silently bypass the limit.
			if created || dev.Revoked {
				active, _ := s.st.CountActiveDevices(u.ID)
				projected := active
				if !created {
					// A revoked device is not counted in active yet; admitting it adds one.
					projected = active + 1
				}
				if u.HWIDLimit > 0 && projected > u.HWIDLimit {
					if created {
						// Roll back the just-added row so it doesn't occupy a slot.
						_ = s.st.RevokeDevice(u.ID, dev.ID)
					}
					return nil, ErrSubDenied
				}
				if !created && dev.Revoked {
					// Room is available: re-admit the returning device.
					_ = s.st.UnrevokeDevice(u.ID, dev.ID)
					newDevice = true
				}
			}
		}
	}
	_ = s.st.TouchSub(u.ID, ua)

	all, err := s.st.ListAllInbounds()
	if err != nil {
		return nil, err
	}
	// Hidden inbounds are provisioned but never handed out in a subscription.
	inbounds := make([]*store.Inbound, 0, len(all))
	for _, in := range all {
		if !in.Hidden {
			inbounds = append(inbounds, in)
		}
	}
	nodes, err := s.st.ListNodes()
	if err != nil {
		return nil, err
	}
	addrByID := map[int64]*store.Node{}
	for _, n := range nodes {
		addrByID[n.ID] = n
	}
	def := s.st.DefaultNodeGroupID()
	userGroup := groupOf(u.GroupID, def)
	eps := subgen.BuildEndpoints(u, inbounds, func(nodeID int64) (string, string) {
		n, ok := addrByID[nodeID]
		if !ok {
			return "", ""
		}
		// Only hand out nodes in the user's group.
		if groupOf(n.GroupID, def) != userGroup {
			return "", ""
		}
		// Expose the address even if not yet connected so a fresh install still hands out configs.
		return n.Name, n.Address
	})
	return &SubResult{User: u, Endpoints: eps, NewDevice: newDevice, Legacy: legacy}, nil
}

// Devices returns a user's registered devices.
func (s *Service) Devices(name string) ([]*store.Device, error) {
	u, err := s.st.GetUserByName(name)
	if err != nil {
		return nil, err
	}
	return s.st.ListDevices(u.ID)
}

// RevokeDevice frees a device slot for a user.
func (s *Service) RevokeDevice(name string, deviceID int64) error {
	u, err := s.st.GetUserByName(name)
	if err != nil {
		return err
	}
	return s.st.RevokeDevice(u.ID, deviceID)
}
