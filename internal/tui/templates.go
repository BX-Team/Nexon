package tui

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/BX-Team/Nexon/internal/core"
)

type templatesMsg struct {
	err error
}

// editorDoneMsg is delivered when the external $EDITOR exits.
type editorDoneMsg struct {
	format string
	path   string
	err    error
}

type templatesPanel struct {
	svc        *core.Service
	tbl        table.Model
	status     string
	err        error
	previewing bool
	vp         viewport.Model
	vpReady    bool
	bodyH      int
}

func newTemplatesPanel(svc *core.Service) panel {
	cols := []table.Column{
		{Title: "FORMAT", Width: 12},
		{Title: "TEMPLATE", Width: 10},
		{Title: "UPDATED", Width: 18},
	}
	return &templatesPanel{svc: svc, tbl: newStyledTable(cols)}
}

func (p *templatesPanel) title() string   { return "Templates" }
func (p *templatesPanel) capturing() bool { return p.previewing }

func (p *templatesPanel) resize(w, h int) {
	p.tbl.SetWidth(w)
	setTableHeight(&p.tbl, h)
	p.bodyH = h
	if p.vpReady {
		p.vp.Width = w
		p.vp.Height = h - 1
	}
}

func (p *templatesPanel) load() tea.Cmd {
	svc := p.svc
	return func() tea.Msg {
		_, err := svc.ListTemplates() // surface DB errors; rebuild() reads the data
		return templatesMsg{err: err}
	}
}

func (p *templatesPanel) rebuild() {
	custom, _ := p.svc.ListTemplates()
	rows := make([]table.Row, 0, len(p.svc.TemplateFormats()))
	for _, f := range p.svc.TemplateFormats() {
		mark, when := "built-in", "-"
		if ts, ok := custom[f]; ok {
			mark, when = "custom", ts.Format("2006-01-02 15:04")
		}
		rows = append(rows, table.Row{f, mark, when})
	}
	p.tbl.SetRows(rows)
}

func (p *templatesPanel) update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case templatesMsg:
		p.err = m.err
		if m.err == nil {
			p.rebuild()
		}
		return nil
	case editorDoneMsg:
		defer os.Remove(m.path)
		if m.err != nil {
			p.status = "editor: " + m.err.Error()
			return nil
		}
		body, err := os.ReadFile(m.path)
		if err != nil {
			p.status = "read: " + err.Error()
			return nil
		}
		if err := p.svc.SetTemplate(m.format, string(body)); err != nil {
			p.status = "invalid template: " + err.Error()
			return nil
		}
		p.status = "saved " + m.format
		return p.load()
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	if p.previewing {
		if s := key.String(); s == "esc" || s == "q" || s == "p" {
			p.previewing = false
			return nil
		}
		var cmd tea.Cmd
		p.vp, cmd = p.vp.Update(key)
		return cmd
	}
	switch key.String() {
	case "e", "enter":
		return p.editSelected()
	case "p":
		return p.openPreview()
	case "d":
		if f := p.selectedFormat(); f != "" {
			if err := p.svc.DeleteTemplate(f); err != nil {
				p.status = "error: " + err.Error()
			} else {
				p.status = "reverted " + f + " to built-in"
				return p.load()
			}
		}
	default:
		var cmd tea.Cmd
		p.tbl, cmd = p.tbl.Update(key)
		return cmd
	}
	return nil
}

func (p *templatesPanel) selectedFormat() string {
	i := p.tbl.Cursor()
	formats := p.svc.TemplateFormats()
	if i < 0 || i >= len(formats) {
		return ""
	}
	return formats[i]
}

func (p *templatesPanel) editSelected() tea.Cmd {
	format := p.selectedFormat()
	if format == "" {
		return nil
	}
	body, ok := p.svc.GetTemplate(format)
	if !ok {
		body = p.svc.StarterTemplate(format)
	}
	ext := ".json"
	if format == "clash" || format == "clash-meta" {
		ext = ".yaml"
	}
	f, err := os.CreateTemp("", "nexon-"+format+"-*"+ext)
	if err != nil {
		p.status = "tempfile: " + err.Error()
		return nil
	}
	path := f.Name()
	_, _ = f.WriteString(body)
	f.Close()

	cmd := exec.Command(editorBinary(), path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorDoneMsg{format: format, path: path, err: err}
	})
}

func (p *templatesPanel) openPreview() tea.Cmd {
	format := p.selectedFormat()
	if format == "" {
		return nil
	}
	body, ok := p.svc.GetTemplate(format)
	if !ok {
		body = p.svc.StarterTemplate(format)
	}
	out, verr := p.svc.RenderPreview(format, body)
	content := out
	if verr != nil {
		content = "⚠ invalid " + format + ": " + verr.Error() + "\n\n" + out
	}
	h := p.bodyH - 1
	if h < 3 {
		h = 10
	}
	p.vp = viewport.New(p.tbl.Width(), h)
	p.vp.SetContent(content)
	p.vpReady = true
	p.previewing = true
	return nil
}

func editorBinary() string {
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	return "vi"
}

func (p *templatesPanel) view() string {
	if p.previewing {
		hdr := styleFormTitle.Render("Preview: "+p.selectedFormat()) + styleHint.Render("   (sample subscription · ↑↓ scroll · esc back)")
		return hdr + "\n" + p.vp.View()
	}
	help := styleHint.Render("e/enter edit in $EDITOR · p preview · d revert to built-in · ↑↓ move")
	status := p.status
	if p.err != nil {
		status = styleErr.Render("error: " + p.err.Error())
	}
	note := styleHint.Render("Custom templates inject {{ .Proxies }} / {{ .Names }}; you own dns/rules/proxy-groups.")
	return p.tbl.View() + "\n" + note + "\n" + statusLine(status, help)
}
