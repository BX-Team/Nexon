// Package subserver serves GET /sub/{token}: detects the client by User-Agent and renders the subscription.
package subserver

import (
	"regexp"
	"strings"
	"sync"

	"github.com/BX-Team/Nexon/internal/store"
)

// Detector matches a User-Agent against ordered rules to choose a format.
type Detector struct {
	mu    sync.RWMutex
	rules []compiledRule
}

type compiledRule struct {
	re     *regexp.Regexp
	format string
}

// NewDetector loads and compiles the ordered rules from the store.
func NewDetector(st *store.Store) (*Detector, error) {
	d := &Detector{}
	if err := d.Reload(st); err != nil {
		return nil, err
	}
	return d, nil
}

// Reload recompiles rules from the store (call after edits via CLI/bot).
func (d *Detector) Reload(st *store.Store) error {
	rules, err := st.ListSubRules()
	if err != nil {
		return err
	}
	compiled := make([]compiledRule, 0, len(rules))
	for _, r := range rules {
		re, err := regexp.Compile(r.Regex)
		if err != nil {
			continue // skip invalid user-defined rules rather than failing
		}
		compiled = append(compiled, compiledRule{re: re, format: r.Format})
	}
	d.mu.Lock()
	d.rules = compiled
	d.mu.Unlock()
	return nil
}

// Detect returns the format for a User-Agent (first match wins, base64 default).
func (d *Detector) Detect(ua string) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, r := range d.rules {
		if r.re.MatchString(ua) {
			return r.format
		}
	}
	return "base64"
}

// IsBrowser reports whether the UA looks like a web browser (→ HTML dashboard).
func IsBrowser(ua string) bool {
	return strings.HasPrefix(ua, "Mozilla/")
}
