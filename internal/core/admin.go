package core

import (
	"fmt"
	"strconv"
	"time"

	"github.com/BX-Team/Nexon/internal/store"
)

// AdminExtendUser adds days to a user's expiry (from the later of now / current).
func (s *Service) AdminExtendUser(id int64, days int) error {
	u, err := s.st.GetUserByID(id)
	if err != nil {
		return err
	}
	base := time.Now()
	if u.ExpireAt != nil && u.ExpireAt.After(base) {
		base = *u.ExpireAt
	}
	t := base.AddDate(0, 0, days)
	u.ExpireAt = &t
	if u.Status == store.StatusExpired {
		u.Status = store.StatusActive
	}
	if err := s.st.UpdateUser(u); err != nil {
		return err
	}
	s.syncUserToNodes(u)
	return nil
}

// AdminSetLifetime clears a user's expiry (subscription never ends) and
// reactivates them if they were expired.
func (s *Service) AdminSetLifetime(id int64) error {
	u, err := s.st.GetUserByID(id)
	if err != nil {
		return err
	}
	u.ExpireAt = nil
	u.ExpiryNotifiedFor = nil
	reactivate := u.Status == store.StatusExpired
	if reactivate {
		u.Status = store.StatusActive
	}
	if err := s.st.UpdateUser(u); err != nil {
		return err
	}
	if reactivate {
		s.syncUserToNodes(u)
	}
	_ = s.st.AddLog("info", "subscription", fmt.Sprintf("%s — подписка сделана бессрочной", u.Username))
	return nil
}

// AdminSetData sets a user's data limit (bytes, 0 = unlimited).
func (s *Service) AdminSetData(id int64, dataLimit int64) error {
	u, err := s.st.GetUserByID(id)
	if err != nil {
		return err
	}
	u.DataLimit = dataLimit
	if u.Status == store.StatusLimited && (dataLimit == 0 || u.UsedTraffic < dataLimit) {
		u.Status = store.StatusActive
		defer s.syncUserToNodes(u)
	}
	return s.st.UpdateUser(u)
}

// AdminSetStatus enables/disables a user by id.
func (s *Service) AdminSetStatus(id int64, status store.UserStatus) error {
	u, err := s.st.GetUserByID(id)
	if err != nil {
		return err
	}
	return s.SetStatus(u.Username, status)
}

// AdminResetTraffic zeroes a user's used traffic by id.
func (s *Service) AdminResetTraffic(id int64) error {
	u, err := s.st.GetUserByID(id)
	if err != nil {
		return err
	}
	return s.ResetTraffic(u.Username)
}

// AdminDeleteUser removes a user by id (and from all nodes).
func (s *Service) AdminDeleteUser(id int64) error {
	u, err := s.st.GetUserByID(id)
	if err != nil {
		return err
	}
	return s.DeleteUser(u.Username)
}

// ---- Settings ----

// GetResetDay returns the configured monthly traffic-reset day (default 20).
func (s *Service) GetResetDay() int { return s.resetDay() }

// SetResetDay updates the monthly traffic-reset day (1–28).
func (s *Service) SetResetDay(day int) error {
	if day < 1 || day > 28 {
		return fmt.Errorf("reset day must be 1–28")
	}
	return s.st.SetSetting("traffic.reset_day", strconv.Itoa(day))
}
