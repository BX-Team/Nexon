package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/BX-Team/Nexon/internal/core"
)

// settingKeys are the runtime subscription settings the panel edits, in order.
var settingKeys = []struct{ key, label string }{
	{"sub.profile_title", "Profile title"},
	{"sub.support_url", "Support URL"},
	{"sub.announce", "Announce"},
}

type settingsMsg struct {
	values   []string // parallel to settingKeys
	resetDay int
	err      error
}

type settingsPanel struct {
	svc     *core.Service
	fields  []textinput.Model // settingKeys… then reset-day
	labels  []string
	editing bool
	focus   int
	status  string
	err     error
}

func newSettingsPanel(svc *core.Service) panel {
	labels := make([]string, 0, len(settingKeys)+1)
	fields := make([]textinput.Model, 0, len(settingKeys)+1)
	for _, s := range settingKeys {
		labels = append(labels, s.label)
		fields = append(fields, newInput("", ""))
	}
	labels = append(labels, "Traffic reset day (1–28)")
	fields = append(fields, newInput("20", ""))
	return &settingsPanel{svc: svc, fields: fields, labels: labels}
}

func (p *settingsPanel) title() string   { return "Settings" }
func (p *settingsPanel) capturing() bool { return p.editing }
func (p *settingsPanel) resize(w, h int) {}

func (p *settingsPanel) load() tea.Cmd {
	svc := p.svc
	return func() tea.Msg {
		vals := make([]string, len(settingKeys))
		for i, s := range settingKeys {
			vals[i], _ = svc.Store().GetSetting(s.key) // missing → ""
		}
		return settingsMsg{values: vals, resetDay: svc.GetResetDay()}
	}
}

func (p *settingsPanel) update(msg tea.Msg) tea.Cmd {
	if m, ok := msg.(settingsMsg); ok {
		if p.editing {
			return nil // don't clobber an in-progress edit
		}
		p.err = m.err
		for i := range settingKeys {
			p.fields[i].SetValue(m.values[i])
		}
		p.fields[len(settingKeys)].SetValue(fmt.Sprintf("%d", m.resetDay))
		return nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	if !p.editing {
		if key.String() == "e" {
			p.editing = true
			p.focus = 0
			p.focusInput()
			p.status = ""
			return textinput.Blink
		}
		return nil
	}
	switch key.String() {
	case "esc":
		p.editing = false
		p.status = ""
		return p.load() // discard edits, reload from store
	case "enter":
		return p.save()
	case "tab", "down":
		p.focus = (p.focus + 1) % len(p.fields)
		p.focusInput()
		return textinput.Blink
	case "shift+tab", "up":
		p.focus = (p.focus - 1 + len(p.fields)) % len(p.fields)
		p.focusInput()
		return textinput.Blink
	default:
		var cmd tea.Cmd
		p.fields[p.focus], cmd = p.fields[p.focus].Update(key)
		return cmd
	}
}

func (p *settingsPanel) save() tea.Cmd {
	for i, s := range settingKeys {
		if err := p.svc.Store().SetSetting(s.key, strings.TrimSpace(p.fields[i].Value())); err != nil {
			p.status = "error: " + err.Error()
			return nil
		}
	}
	day := atoiOr(p.fields[len(settingKeys)].Value(), 20)
	if err := p.svc.SetResetDay(day); err != nil {
		p.status = "error: " + err.Error()
		return nil
	}
	p.editing = false
	p.status = "saved ✓"
	return nil
}

func (p *settingsPanel) focusInput() {
	for i := range p.fields {
		if i == p.focus && p.editing {
			p.fields[i].Focus()
		} else {
			p.fields[i].Blur()
		}
	}
}

func (p *settingsPanel) view() string {
	var b strings.Builder
	b.WriteString(styleFormTitle.Render("Subscription settings") + "\n\n")
	for i, in := range p.fields {
		b.WriteString(labeledField(p.labels[i], in, p.editing && i == p.focus) + "\n")
	}
	b.WriteString("\n")
	if p.editing {
		b.WriteString(styleHint.Render("tab move · enter save · esc cancel"))
	} else {
		b.WriteString(styleHint.Render("e edit"))
	}
	if p.status == "saved ✓" {
		b.WriteString("\n" + styleOK.Render(p.status))
	} else if p.status != "" {
		b.WriteString("\n" + styleErr.Render(p.status))
	}
	if p.err != nil {
		b.WriteString("\n" + styleErr.Render("error: "+p.err.Error()))
	}
	return formBox.Render(b.String())
}
