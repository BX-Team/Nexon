package core

import (
	"errors"
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
}

// Subscription resolves a token, enforces status and HWID limit, and builds connectable endpoints.
func (s *Service) Subscription(token, ua, hwid, ip string) (*SubResult, error) {
	u, err := s.st.GetUserByToken(token)
	if err != nil {
		return nil, err
	}
	if u.Status == store.StatusDisabled {
		return nil, ErrSubDenied
	}
	if u.ExpireAt != nil && u.ExpireAt.Before(time.Now()) {
		return nil, ErrSubDenied
	}

	// Device registration + HWID limit enforcement.
	newDevice := false
	if hwid != "" || ua != "" {
		dev, created, derr := s.st.RegisterDevice(u.ID, hwid, ua, ip)
		if derr == nil {
			newDevice = created
			if created && u.HWIDLimit > 0 {
				if n, _ := s.st.CountActiveDevices(u.ID); n > u.HWIDLimit {
					// Over the limit: revoke the just-added device and deny.
					_ = s.st.RevokeDevice(u.ID, dev.ID)
					return nil, ErrSubDenied
				}
			}
		}
	}
	_ = s.st.TouchSub(u.ID, ua)

	inbounds, err := s.st.ListAllInbounds()
	if err != nil {
		return nil, err
	}
	nodes, err := s.st.ListNodes()
	if err != nil {
		return nil, err
	}
	addrByID := map[int64]*store.Node{}
	for _, n := range nodes {
		addrByID[n.ID] = n
	}
	userGroup := groupOf(u.GroupID)
	eps := subgen.BuildEndpoints(u, inbounds, func(nodeID int64) (string, string) {
		n, ok := addrByID[nodeID]
		if !ok {
			return "", ""
		}
		// Only hand out nodes in the user's group.
		if groupOf(n.GroupID) != userGroup {
			return "", ""
		}
		// Expose the address even if not yet connected so a fresh install still hands out configs.
		return n.Name, n.Address
	})
	return &SubResult{User: u, Endpoints: eps, NewDevice: newDevice}, nil
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
