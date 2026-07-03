package store

// DefaultSubRules are the built-in ordered UA→format detection rules, seeded on first run.
var DefaultSubRules = []SubRule{
	{Priority: 10, Regex: `^([Cc]lash[\-\.]?[Vv]erge|[Cc]lash[\-\.]?[Mm]eta|[Ff][Ll][Cc]lash|[Mm]ihomo|[Ss]tash)`, Format: "clash-meta"},
	{Priority: 20, Regex: `^([Cc]lash|[Ss]tash)`, Format: "clash"},
	{Priority: 30, Regex: `^[Hh]app`, Format: "happ"},
	{Priority: 40, Regex: `INCY/[\d.]+`, Format: "xray"},
	{Priority: 50, Regex: `^([Vv]2rayNG|[Vv]2rayN|[Ss]treisand|[Kk]tor\-client)`, Format: "xray"},
	{Priority: 60, Regex: `^(SFA|SFI|SFM|SFT|[Kk]aring|[Hh]iddify[Nn]ext)|.*[Ss]ing[-_]?box.*`, Format: "singbox"},
	{Priority: 99, Regex: `.*`, Format: "base64"},
}

// SeedDefaults populates built-in rules and settings if the DB is empty.
// Idempotent: only seeds when the respective tables have no rows.
func (s *Store) SeedDefaults() error {
	n, err := s.CountSubRules()
	if err != nil {
		return err
	}
	if n == 0 {
		for _, r := range DefaultSubRules {
			if err := s.AddSubRule(r.Priority, r.Regex, r.Format); err != nil {
				return err
			}
		}
	}
	// Default Happ headers profile placeholder.
	if _, err := s.GetSetting("sub.happ.headers"); err == ErrNotFound {
		_ = s.SetSetting("sub.happ.headers", "{}")
	}
	// Day of month the monthly traffic reset runs on (configurable).
	if _, err := s.GetSetting("traffic.reset_day"); err == ErrNotFound {
		_ = s.SetSetting("traffic.reset_day", "20")
	}
	return nil
}
