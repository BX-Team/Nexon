package core

import (
	"log/slog"
	"strings"
	"time"

	"github.com/BX-Team/Nexon/internal/store"
	"github.com/BX-Team/Nexon/internal/subgen"
)

// templatesSeededKey marks that the default templates were seeded once, so a
// later delete (revert to built-in) is not undone on the next startup.
const templatesSeededKey = "templates.seeded"

// TemplateFormats lists formats that support custom templates.
func (s *Service) TemplateFormats() []string { return subgen.TemplateFormats() }

// OutputFormats lists every format a client app can be pinned to.
func (s *Service) OutputFormats() []string { return subgen.OutputFormats() }

// GetTemplate returns the stored template body for a format, or "" if none.
func (s *Service) GetTemplate(format string) (string, bool) {
	body, err := s.st.GetTemplate(format)
	if err != nil {
		return "", false
	}
	return body, true
}

// StarterTemplate returns a working starting point for editing a format.
func (s *Service) StarterTemplate(format string) string { return subgen.StarterTemplate(format) }

// sampleSub returns a representative user + endpoints for validating and
// previewing templates without a live subscription.
func sampleSub() (*store.User, []subgen.Endpoint) {
	limit := int64(100) << 30
	return &store.User{Username: "sample", DataLimit: limit, Status: store.StatusActive}, subgen.SampleEndpoints()
}

// ValidateTemplate renders body against a sample subscription and checks the
// output is well-formed for the format. Returns a descriptive error if not.
func (s *Service) ValidateTemplate(format, body string) error {
	if !subgen.SupportsTemplate(format) {
		return store.ErrNotFound
	}
	u, eps := sampleSub()
	out, _, err := subgen.RenderWithTemplate(format, body, u, eps)
	if err != nil {
		return err
	}
	return subgen.ValidateOutput(format, out)
}

// RenderPreview renders body against the sample subscription and returns the
// output plus any validity warning (so the UI can show the result even when it
// does not parse).
func (s *Service) RenderPreview(format, body string) (string, error) {
	if !subgen.SupportsTemplate(format) {
		return "", store.ErrNotFound
	}
	u, eps := sampleSub()
	out, _, err := subgen.RenderWithTemplate(format, body, u, eps)
	if err != nil {
		return "", err
	}
	return string(out), subgen.ValidateOutput(format, out)
}

// SetTemplate validates (Go-template syntax + rendered YAML/JSON) and stores a
// custom template for a format.
func (s *Service) SetTemplate(format, body string) error {
	if err := s.ValidateTemplate(format, body); err != nil {
		return err
	}
	if err := s.st.SetTemplate(format, body); err != nil {
		return err
	}
	_ = s.st.AddLog("info", "template", "обновлён шаблон "+format)
	return nil
}

// DeleteTemplate drops a format's custom template (reverting to built-in).
func (s *Service) DeleteTemplate(format string) error { return s.st.DeleteTemplate(format) }

// ListTemplates reports which formats currently have a custom template.
func (s *Service) ListTemplates() (map[string]time.Time, error) { return s.st.ListTemplates() }

// SeedTemplates installs the rich default templates once (guarded by a settings
// flag), so fresh installs serve full clash/singbox/xray configs out of the box
// while later reverts to built-in stick.
func (s *Service) SeedTemplates() error {
	if v, _ := s.st.GetSetting(templatesSeededKey); v == "1" {
		return nil
	}
	for _, f := range subgen.TemplateFormats() {
		if _, err := s.st.GetTemplate(f); err == nil {
			continue // operator already set one
		}
		if err := s.SetTemplate(f, subgen.StarterTemplate(f)); err != nil {
			return err
		}
	}
	return s.st.SetSetting(templatesSeededKey, "1")
}

// RenderSubscription renders a user's endpoints in format, preferring a custom
// template when one is set and produces valid output, else the built-in
// generator.
func (s *Service) RenderSubscription(format string, u *store.User, eps []subgen.Endpoint) ([]byte, string) {
	if subgen.SupportsTemplate(format) {
		if body, err := s.st.GetTemplate(format); err == nil && strings.TrimSpace(body) != "" {
			out, ctype, terr := subgen.RenderWithTemplate(format, body, u, eps)
			switch {
			case terr != nil:
				slog.Warn("custom template failed; using built-in", "format", format, "err", terr)
			default:
				if verr := subgen.ValidateOutput(format, out); verr != nil {
					slog.Warn("custom template produced invalid output; using built-in", "format", format, "err", verr)
				} else {
					return out, ctype
				}
			}
		}
	}
	return subgen.Get(format).Render(u, eps)
}
