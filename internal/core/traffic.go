package core

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/BX-Team/Nexon/internal/node"
	"github.com/BX-Team/Nexon/internal/store"
)

// PollTraffic polls StatsService from every node, updates traffic and node
// status, re-syncs restarted nodes, enforces limits, and returns records applied.
func (s *Service) PollTraffic(ctx context.Context) (int, error) {
	nodes, err := s.st.ListNodes()
	if err != nil {
		return 0, err
	}
	applied := 0
	deltas := map[string]int64{}
	var resync []string

	for _, n := range nodes {
		stats, restarted, err := s.pollNode(ctx, n)
		if err != nil {
			slog.Warn("poll node failed", "node", n.Name, "err", err)
			_ = s.st.UpdateNodeStatus(n.ID, "error", n.XrayVersion)
			continue
		}
		_ = s.st.UpdateNodeStatus(n.ID, "connected", n.XrayVersion)
		if restarted {
			resync = append(resync, n.Name)
		}
		for _, st := range stats {
			deltas[st.Email] += st.Uplink + st.Downlink
			applied++
		}
	}

	for email, delta := range deltas {
		if delta == 0 {
			continue
		}
		u, err := s.st.GetUserByName(email)
		if err != nil {
			continue // stats for a user Nexon no longer knows
		}
		if err := s.st.AddUserTraffic(u.ID, delta); err != nil {
			slog.Warn("persist traffic failed", "user", email, "err", err)
		}
	}

	// Restarted xray lost its in-memory users: push them back.
	for _, name := range resync {
		slog.Info("node restart detected, resyncing users", "node", name)
		if err := s.SyncNode(name); err != nil {
			slog.Warn("resync after restart failed", "node", name, "err", err)
		}
	}

	now := time.Now()
	s.resetTrafficPeriod(now)
	s.enforceLimits(now)
	return applied, nil
}

// pollNode reads one node's stats and restart flag under a per-node timeout,
// so one hung node cannot stall the whole poll cycle.
func (s *Service) pollNode(ctx context.Context, n *store.Node) (stats []node.Stat, restarted bool, err error) {
	nctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	conn := s.connect(n)
	defer conn.Close()
	if _, err := conn.Connect(nctx); err != nil {
		return nil, false, err
	}
	stats, err = conn.QueryStats(nctx, true) // reset counters after read
	if err != nil {
		return nil, false, err
	}
	if uptime, uerr := conn.Uptime(nctx); uerr == nil {
		restarted = s.nodeRestarted(n.ID, uptime)
	}
	return stats, restarted, nil
}

// resetDay returns the configured day-of-month for the monthly traffic reset
// (default 20), clamped to a sane range.
func (s *Service) resetDay() int {
	v, err := s.st.GetSetting("traffic.reset_day")
	if err != nil {
		return 20
	}
	d, err := strconv.Atoi(v)
	if err != nil || d < 1 || d > 28 {
		return 20
	}
	return d
}

// periodStart returns the most recent reset boundary at or before now for the
// given day-of-month (this month's reset time, or last month's if not reached).
func periodStart(now time.Time, day int) time.Time {
	thisMonth := time.Date(now.Year(), now.Month(), day, 0, 0, 0, 0, now.Location())
	if now.Before(thisMonth) {
		return thisMonth.AddDate(0, -1, 0)
	}
	return thisMonth
}

// resetBoundary returns the most recent reset boundary for a strategy, or
// ok=false when the strategy never resets (no_reset or unknown).
func resetBoundary(strategy string, now time.Time, monthDay int) (time.Time, bool) {
	switch strategy {
	case "day":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), true
	case "week":
		monday := now.AddDate(0, 0, -((int(now.Weekday()) + 6) % 7))
		return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, now.Location()), true
	case "month":
		return periodStart(now, monthDay), true
	}
	return time.Time{}, false
}

// resetTrafficPeriod zeroes used_traffic for users overdue per their own reset
// strategy and re-activates limited users.
func (s *Service) resetTrafficPeriod(now time.Time) {
	users, err := s.st.ListUsers("")
	if err != nil {
		return
	}
	day := s.resetDay()
	for _, u := range users {
		boundary, ok := resetBoundary(u.TrafficResetStrategy, now, day)
		if !ok {
			continue
		}
		if u.TrafficResetAt != nil && !u.TrafficResetAt.Before(boundary) {
			continue // already reset this period
		}
		wasLimited := u.Status == store.StatusLimited
		u.UsedTraffic = 0
		u.TrafficResetAt = &now
		// Quota-limited users come back; expired/disabled stay as they are.
		if wasLimited {
			u.Status = store.StatusActive
		}
		if err := s.st.UpdateUser(u); err != nil {
			continue
		}
		if wasLimited {
			s.syncUserToNodes(u)
		}
		slog.Debug("traffic reset", "user", u.Username)
	}
}

// enforceLimits scans all active users and limits/expires those over quota or
// past their expiry, kicking them off every node.
func (s *Service) enforceLimits(now time.Time) {
	users, err := s.st.ListUsers(string(store.StatusActive))
	if err != nil {
		return
	}
	for _, u := range users {
		var newStatus store.UserStatus
		if u.ExpireAt != nil && u.ExpireAt.Before(now) {
			newStatus = store.StatusExpired
		} else if u.DataLimit > 0 && u.UsedTraffic >= u.DataLimit {
			newStatus = store.StatusLimited
		}
		if newStatus != "" {
			u.Status = newStatus
			_ = s.st.UpdateUser(u)
			s.removeUserFromNodes(u)
			slog.Info("user auto-kicked", "user", u.Username, "status", newStatus)
			_ = s.st.AddLog("warn", "enforcement", fmt.Sprintf("%s отключён: %s", u.Username, newStatus))
		}
	}
}

// RunPoller loops PollTraffic every interval until ctx is cancelled.
func (s *Service) RunPoller(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := s.PollTraffic(ctx); err != nil {
				slog.Warn("poll cycle error", "err", err)
			} else if n > 0 {
				slog.Debug("poll cycle applied stats", "records", n)
			}
		}
	}
}
