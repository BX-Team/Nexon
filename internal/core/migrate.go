package core

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/BX-Team/Nexon/internal/secrets"
	"github.com/BX-Team/Nexon/internal/store"
)

// PasarguardImportOptions filters and shapes a PasarGuard → Nexon user import.
type PasarguardImportOptions struct {
	Only         map[string]bool // import only these usernames (empty = all)
	Skip         map[string]bool
	ResetTraffic bool            // zero used_traffic instead of carrying it over
	KeepTraffic  map[string]bool // per-user exceptions to ResetTraffic
	GroupID      *int64          // node group for imported users (nil = default)
	DryRun       bool
}

// PasarguardImportResult reports what happened to one PasarGuard user.
type PasarguardImportResult struct {
	Username    string
	LegacyID    int64
	Action      string // imported | mapped | skipped
	Detail      string
	DataLimit   int64
	UsedTraffic int64
}

// ImportPasarguard copies users, the legacy id mapping, and the token secret
// from a PasarGuard SQLite database so old subscription links keep working.
func (s *Service) ImportPasarguard(dbPath string, opt PasarguardImportOptions) ([]PasarguardImportResult, error) {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open pasarguard db: %w", err)
	}
	defer db.Close()

	var secret string
	if err := db.QueryRow(`SELECT secret_key FROM jwt LIMIT 1`).Scan(&secret); err != nil {
		return nil, fmt.Errorf("read jwt secret (не база PasarGuard?): %w", err)
	}
	rows, err := db.Query(`SELECT id, username, status, used_traffic, COALESCE(data_limit, 0),
		COALESCE(data_limit_reset_strategy, 'no_reset'), COALESCE(expire, ''), COALESCE(hwid_limit, 0)
		FROM users ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("read users: %w", err)
	}
	defer rows.Close()

	type pgUser struct {
		id                  int64
		name, status, reset string
		used, limit         int64
		expire              string
		hwid                int64
	}
	var src []pgUser
	for rows.Next() {
		var u pgUser
		if err := rows.Scan(&u.id, &u.name, &u.status, &u.used, &u.limit, &u.reset, &u.expire, &u.hwid); err != nil {
			return nil, err
		}
		src = append(src, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []PasarguardImportResult
	for _, p := range src {
		res := PasarguardImportResult{Username: p.name, LegacyID: p.id}
		switch {
		case len(opt.Only) > 0 && !opt.Only[p.name]:
			res.Action, res.Detail = "skipped", "не в --only"
		case opt.Skip[p.name]:
			res.Action, res.Detail = "skipped", "--skip"
		default:
			if existing, err := s.st.GetUserByName(p.name); err == nil {
				res.Action, res.Detail = "mapped", "юзер уже существует, привязан только legacy id"
				if !opt.DryRun {
					if err := s.st.SetLegacyMapping(p.id, existing.ID); err != nil {
						return out, fmt.Errorf("map %s: %w", p.name, err)
					}
				}
			} else {
				used := p.used
				if opt.ResetTraffic && !opt.KeepTraffic[p.name] {
					used = 0
				}
				res.Action = "imported"
				res.DataLimit, res.UsedTraffic = p.limit, used
				if !opt.DryRun {
					u, err := s.importPasarguardUser(p.name, p.status, p.reset, p.expire, p.limit, used, p.hwid, opt.GroupID)
					if err != nil {
						return out, fmt.Errorf("import %s: %w", p.name, err)
					}
					if err := s.st.SetLegacyMapping(p.id, u.ID); err != nil {
						return out, fmt.Errorf("map %s: %w", p.name, err)
					}
				}
			}
		}
		out = append(out, res)
	}
	if !opt.DryRun {
		if err := s.st.SetSetting(SettingLegacySecret, secret); err != nil {
			return out, fmt.Errorf("store legacy secret: %w", err)
		}
	}
	return out, nil
}

func (s *Service) importPasarguardUser(name, status, reset, expire string, limit, used, hwid int64, groupID *int64) (*store.User, error) {
	proxies, err := secrets.GenerateProxies()
	if err != nil {
		return nil, fmt.Errorf("generate proxies: %w", err)
	}
	now := time.Now()
	u := &store.User{
		Username:             name,
		Status:               mapPasarguardStatus(status),
		DataLimit:            limit,
		TrafficResetStrategy: mapPasarguardReset(reset),
		ExpireAt:             parsePasarguardTime(expire),
		HWIDLimit:            int(hwid),
		Proxies:              proxies,
		SubToken:             secrets.SubToken(),
		TrafficResetAt:       &now,
		GroupID:              groupID,
	}
	if err := s.st.CreateUser(u); err != nil {
		return nil, err
	}
	if used > 0 {
		if err := s.st.AddUserTraffic(u.ID, used); err != nil {
			return nil, err
		}
		u.UsedTraffic = used
	}
	s.syncUserToNodes(u)
	return u, nil
}

func mapPasarguardStatus(st string) store.UserStatus {
	switch st {
	case "disabled":
		return store.StatusDisabled
	case "limited":
		return store.StatusLimited
	case "expired":
		return store.StatusExpired
	default: // active, on_hold
		return store.StatusActive
	}
}

func mapPasarguardReset(r string) string {
	switch r {
	case "day", "week", "month":
		return r
	default: // no_reset, year and anything unknown
		return "no_reset"
	}
}

// parsePasarguardTime reads naive-UTC datetimes like "2027-03-24 09:00:29.000000".
func parsePasarguardTime(v string) *time.Time {
	if v == "" {
		return nil
	}
	if i := strings.IndexByte(v, '.'); i > 0 {
		v = v[:i]
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", v, time.UTC)
	if err != nil {
		return nil
	}
	return &t
}
