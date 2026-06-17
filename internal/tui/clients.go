package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/BX-Team/Nexon/internal/core"
	"github.com/BX-Team/Nexon/internal/store"
)

type clientsMsg struct {
	apps []*store.ClientApp
	err  error
}

type clientMode int

const (
	cList clientMode = iota
	cForm
	cConfirmDelete
)

// fixed focus indices before the dynamic header rows begin.
const (
	fcName   = 0
	fcUA     = 1
	fcSort   = 2
	fcOn     = 3 // the enabled toggle (not a text input)
	fcFormat = 4 // the output-format selector (not a text input)
	fcHdr0   = 5 // first header field; row i → key=5+2i, value=6+2i
)

type headerRow struct {
	key textinput.Model
	val textinput.Model
}

type clientsPanel struct {
	svc    *core.Service
	tbl    table.Model
	apps   []*store.ClientApp
	mode   clientMode
	status string
	err    error

	// form state
	editID  int64
	name    textinput.Model
	ua      textinput.Model
	sortIn  textinput.Model
	enabled bool
	format  string // "" = auto
	headers []headerRow
	focus   int
}

func newClientsPanel(svc *core.Service) panel {
	cols := []table.Column{
		{Title: "ID", Width: 4},
		{Title: "SORT", Width: 5},
		{Title: "ON", Width: 3},
		{Title: "NAME", Width: 14},
		{Title: "UA-PATTERN", Width: 24},
		{Title: "FORMAT", Width: 11},
		{Title: "HDRS", Width: 4},
	}
	return &clientsPanel{svc: svc, tbl: newStyledTable(cols)}
}

func (p *clientsPanel) title() string   { return "Clients" }
func (p *clientsPanel) capturing() bool { return p.mode != cList }
func (p *clientsPanel) resize(w, h int) { p.tbl.SetWidth(w); setTableHeight(&p.tbl, h) }

func (p *clientsPanel) load() tea.Cmd {
	svc := p.svc
	return func() tea.Msg {
		apps, err := svc.ListClientApps()
		return clientsMsg{apps: apps, err: err}
	}
}

func (p *clientsPanel) update(msg tea.Msg) tea.Cmd {
	if m, ok := msg.(clientsMsg); ok {
		p.err = m.err
		if m.err == nil {
			p.apps = m.apps
			p.tbl.SetRows(clientRows(m.apps))
		}
		return nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch p.mode {
	case cForm:
		return p.updateForm(key)
	case cConfirmDelete:
		return p.updateConfirm(key)
	default:
		return p.updateList(key)
	}
}

func (p *clientsPanel) updateList(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "n":
		p.openCreate()
		return textinput.Blink
	case "e":
		if a := p.selected(); a != nil {
			p.openEdit(a)
			return textinput.Blink
		}
	case "d":
		if a := p.selected(); a != nil {
			p.mode = cConfirmDelete
		}
	default:
		var cmd tea.Cmd
		p.tbl, cmd = p.tbl.Update(key)
		return cmd
	}
	return nil
}

func (p *clientsPanel) updateConfirm(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "y", "Y", "enter":
		a := p.selected()
		p.mode = cList
		if a == nil {
			return nil
		}
		if err := p.svc.DeleteClientApp(a.ID); err != nil {
			p.status = "error: " + err.Error()
			return nil
		}
		p.status = "deleted " + a.Name
		return p.load()
	default:
		p.mode = cList
	}
	return nil
}

func (p *clientsPanel) updateForm(key tea.KeyMsg) tea.Cmd {
	n := p.focusCount()
	switch key.String() {
	case "esc":
		p.mode = cList
		p.status = ""
		return nil
	case "enter":
		return p.submitForm()
	case "tab", "down":
		p.focus = (p.focus + 1) % n
		p.focusInput()
		return textinput.Blink
	case "shift+tab", "up":
		p.focus = (p.focus - 1 + n) % n
		p.focusInput()
		return textinput.Blink
	case "ctrl+n":
		p.headers = append(p.headers, headerRow{key: newInput("X-Header", ""), val: newInput("value", "")})
		p.focus = fcHdr0 + 2*(len(p.headers)-1) // jump to the new key field
		p.focusInput()
		return textinput.Blink
	case "ctrl+d":
		if p.focus >= fcHdr0 {
			row := (p.focus - fcHdr0) / 2
			p.headers = append(p.headers[:row], p.headers[row+1:]...)
			if p.focus >= p.focusCount() {
				p.focus = p.focusCount() - 1
			}
			p.focusInput()
		}
		return nil
	default:
		// Toggle field: space / ←→ flip it.
		if p.focus == fcOn {
			switch key.String() {
			case " ", "space", "left", "right", "x":
				p.enabled = !p.enabled
			}
			return nil
		}
		// Format selector: space / →  next, ←  previous.
		if p.focus == fcFormat {
			switch key.String() {
			case " ", "space", "right":
				p.format = cycleFormat(p.format, p.svc.OutputFormats(), +1)
			case "left":
				p.format = cycleFormat(p.format, p.svc.OutputFormats(), -1)
			}
			return nil
		}
		cur := p.focusList()[p.focus]
		if cur == nil {
			return nil
		}
		var cmd tea.Cmd
		*cur, cmd = cur.Update(key)
		return cmd
	}
}

func (p *clientsPanel) submitForm() tea.Cmd {
	headers := map[string]string{}
	for _, h := range p.headers {
		k := strings.TrimSpace(h.key.Value())
		if k != "" {
			headers[k] = h.val.Value()
		}
	}
	app := &store.ClientApp{
		ID:        p.editID,
		Name:      p.name.Value(),
		UAPattern: p.ua.Value(),
		Headers:   headers,
		Enabled:   p.enabled,
		Sort:      atoiOr(p.sortIn.Value(), 100),
		Format:    p.format,
	}
	var err error
	if p.editID == 0 {
		err = p.svc.CreateClientApp(app)
	} else {
		err = p.svc.UpdateClientApp(app)
	}
	if err != nil {
		p.status = "error: " + err.Error()
		return nil
	}
	p.status = "saved " + app.Name
	p.mode = cList
	return p.load()
}

func (p *clientsPanel) openCreate() {
	p.editID = 0
	p.name = newInput("Happ", "")
	p.ua = newInput("^[Hh]app", "")
	p.sortIn = newInput("100", "100")
	p.enabled = true
	p.format = ""
	p.headers = nil
	p.focus = fcName
	p.focusInput()
	p.mode = cForm
}

func (p *clientsPanel) openEdit(a *store.ClientApp) {
	p.editID = a.ID
	p.name = newInput("name", a.Name)
	p.ua = newInput("regex", a.UAPattern)
	p.sortIn = newInput("100", strconv.Itoa(a.Sort))
	p.enabled = a.Enabled
	p.format = a.Format
	p.headers = p.headers[:0]
	for _, k := range sortedKeys(a.Headers) {
		p.headers = append(p.headers, headerRow{key: newInput("X-Header", k), val: newInput("value", a.Headers[k])})
	}
	p.focus = fcName
	p.focusInput()
	p.mode = cForm
}

// focusList maps each focus index to its text input, or nil for the toggle /
// format selector (indices fcOn, fcFormat).
func (p *clientsPanel) focusList() []*textinput.Model {
	fs := []*textinput.Model{&p.name, &p.ua, &p.sortIn, nil, nil}
	for i := range p.headers {
		fs = append(fs, &p.headers[i].key, &p.headers[i].val)
	}
	return fs
}

func (p *clientsPanel) focusCount() int { return fcHdr0 + 2*len(p.headers) }

func (p *clientsPanel) focusInput() {
	fs := p.focusList()
	for i, in := range fs {
		if in == nil {
			continue
		}
		if i == p.focus {
			in.Focus()
		} else {
			in.Blur()
		}
	}
}

func (p *clientsPanel) selected() *store.ClientApp {
	i := p.tbl.Cursor()
	if i < 0 || i >= len(p.apps) {
		return nil
	}
	return p.apps[i]
}

func (p *clientsPanel) view() string {
	switch p.mode {
	case cForm:
		return p.formView()
	case cConfirmDelete:
		name := "?"
		if a := p.selected(); a != nil {
			name = a.Name
		}
		return formBox.Render(styleErr.Render("Delete client "+name+"?  ") + styleHint.Render("y / n"))
	default:
		help := styleHint.Render("n new · e edit · d delete · ↑↓ move")
		status := p.status
		if p.err != nil {
			status = styleErr.Render("error: " + p.err.Error())
		}
		return p.tbl.View() + "\n" + statusLine(status, help)
	}
}

func (p *clientsPanel) formView() string {
	title := "New client"
	if p.editID != 0 {
		title = "Edit client"
	}
	var b strings.Builder
	b.WriteString(styleFormTitle.Render(title) + "\n\n")
	b.WriteString(labeledField("Name", p.name, p.focus == fcName) + "\n")
	b.WriteString(labeledField("UA pattern (regex)", p.ua, p.focus == fcUA) + "\n")
	b.WriteString(labeledField("Sort", p.sortIn, p.focus == fcSort) + "\n")
	b.WriteString(labeledToggle("Enabled", p.enabled, p.focus == fcOn) + "\n")
	b.WriteString(labeledChoice("Output format", formatLabel(p.format), p.focus == fcFormat) + "\n")

	b.WriteString(styleSection.Render("Custom response headers") + "\n")
	if len(p.headers) == 0 {
		b.WriteString(styleHint.Render("  (none — ctrl+n to add)") + "\n")
	}
	for i := range p.headers {
		keyIdx := fcHdr0 + 2*i
		b.WriteString(labeledField(fmt.Sprintf("hdr %d key", i+1), p.headers[i].key, p.focus == keyIdx) + "\n")
		b.WriteString(labeledField(fmt.Sprintf("hdr %d val", i+1), p.headers[i].val, p.focus == keyIdx+1) + "\n")
	}

	b.WriteString("\n" + styleHint.Render("tab move · space toggle · ctrl+n add header · ctrl+d del header · enter save · esc cancel"))
	if p.status != "" {
		b.WriteString("\n" + styleErr.Render(p.status))
	}
	return formBox.Render(b.String())
}

func clientRows(apps []*store.ClientApp) []table.Row {
	rows := make([]table.Row, 0, len(apps))
	for _, a := range apps {
		on := "✓"
		if !a.Enabled {
			on = "✗"
		}
		format := a.Format
		if format == "" {
			format = "auto"
		}
		rows = append(rows, table.Row{
			strconv.FormatInt(a.ID, 10), strconv.Itoa(a.Sort), on, a.Name, a.UAPattern, format, strconv.Itoa(len(a.Headers)),
		})
	}
	return rows
}

// formatLabel renders the pinned format, showing "auto" for the empty value.
func formatLabel(f string) string {
	if f == "" {
		return "auto (detect by UA)"
	}
	return f
}

// cycleFormat advances the pinned format through ["", formats...] by dir (±1).
func cycleFormat(cur string, formats []string, dir int) string {
	all := append([]string{""}, formats...)
	idx := 0
	for i, f := range all {
		if f == cur {
			idx = i
			break
		}
	}
	idx = (idx + dir + len(all)) % len(all)
	return all[idx]
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
