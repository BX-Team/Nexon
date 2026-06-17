package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/BX-Team/Nexon/internal/core"
	"github.com/BX-Team/Nexon/internal/store"
)

type usersMsg struct {
	users  []*store.User
	groups []*store.NodeGroup
	err    error
}

// userDevicesMsg carries a user's devices into the detail view.
type userDevicesMsg struct {
	name    string
	devices []*store.Device
	err     error
}

type userMode int

const (
	uList userMode = iota
	uForm
	uConfirmDelete
	uDetail
)

type usersPanel struct {
	svc        *core.Service
	subBaseURL string
	tbl        table.Model
	users      []*store.User
	groups     []*store.NodeGroup
	mode       userMode
	status     string
	err        error

	// form state
	editName string // "" = create
	labels   []string
	fields   []textinput.Model
	focus    int

	// detail state
	detail    *store.User
	devices   []*store.Device
	qr        string
	devCursor int
	detailErr error
}

func newUsersPanel(svc *core.Service, subBaseURL string) panel {
	cols := []table.Column{
		{Title: "USER", Width: 20},
		{Title: "STATUS", Width: 9},
		{Title: "USED", Width: 10},
		{Title: "LIMIT", Width: 10},
		{Title: "EXPIRES", Width: 11},
		{Title: "GROUP", Width: 14},
	}
	return &usersPanel{svc: svc, subBaseURL: subBaseURL, tbl: newStyledTable(cols)}
}

func (p *usersPanel) title() string   { return "Users" }
func (p *usersPanel) capturing() bool { return p.mode != uList }
func (p *usersPanel) resize(w, h int) { p.tbl.SetWidth(w); setTableHeight(&p.tbl, h) }

func (p *usersPanel) load() tea.Cmd {
	svc := p.svc
	return func() tea.Msg {
		users, err := svc.ListUsers("")
		if err != nil {
			return usersMsg{err: err}
		}
		groups, err := svc.ListNodeGroups()
		return usersMsg{users: users, groups: groups, err: err}
	}
}

func (p *usersPanel) update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case usersMsg:
		p.err = m.err
		if m.err == nil {
			p.users = m.users
			p.groups = m.groups
			p.tbl.SetRows(p.userRows())
		}
		return nil
	case userDevicesMsg:
		if p.detail != nil && m.name == p.detail.Username {
			p.devices, p.detailErr = m.devices, m.err
			if p.devCursor >= len(p.devices) {
				p.devCursor = 0
			}
		}
		return nil
	}
	key, isKey := msg.(tea.KeyMsg)
	if !isKey {
		return nil
	}
	switch p.mode {
	case uForm:
		return p.updateForm(key)
	case uConfirmDelete:
		return p.updateConfirm(key)
	case uDetail:
		return p.updateDetail(key)
	default:
		return p.updateList(key)
	}
}

func (p *usersPanel) updateList(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "enter":
		if u := p.selected(); u != nil {
			return p.openDetail(u)
		}
	case "n":
		p.openCreate()
		return textinput.Blink
	case "e":
		if u := p.selected(); u != nil {
			p.openEdit(u)
			return textinput.Blink
		}
	case "d":
		if u := p.selected(); u != nil {
			p.mode = uConfirmDelete
		}
	case "s":
		if u := p.selected(); u != nil {
			next := store.StatusActive
			if u.Status == store.StatusActive {
				next = store.StatusDisabled
			}
			if err := p.svc.SetStatus(u.Username, next); err != nil {
				p.status = "error: " + err.Error()
			} else {
				p.status = fmt.Sprintf("%s → %s", u.Username, next)
				return p.load()
			}
		}
	case "t":
		if u := p.selected(); u != nil {
			if err := p.svc.ResetTraffic(u.Username); err != nil {
				p.status = "error: " + err.Error()
			} else {
				p.status = u.Username + ": traffic reset"
				return p.load()
			}
		}
	case "g":
		if u := p.selected(); u != nil {
			next := nextGroupID(u.GroupID, p.groups)
			if err := p.svc.SetUserGroup(u.ID, next); err != nil {
				p.status = "error: " + err.Error()
			} else {
				p.status = u.Username + " → group " + groupName(next, p.groups)
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

func (p *usersPanel) updateConfirm(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "y", "Y", "enter":
		u := p.selected()
		p.mode = uList
		if u == nil {
			return nil
		}
		if err := p.svc.DeleteUser(u.Username); err != nil {
			p.status = "error: " + err.Error()
			return nil
		}
		p.status = "deleted " + u.Username
		return p.load()
	default: // n, esc, anything else
		p.mode = uList
	}
	return nil
}

func (p *usersPanel) updateForm(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "esc":
		p.mode = uList
		p.status = ""
		return nil
	case "enter":
		return p.submitForm()
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

func (p *usersPanel) submitForm() tea.Cmd {
	if p.editName == "" {
		data, err := parseSize(p.fields[1].Value())
		if err != nil {
			p.status = "error: " + err.Error()
			return nil
		}
		days := atoiOr(p.fields[2].Value(), 0)
		var expire *time.Time
		if days > 0 {
			t := time.Now().AddDate(0, 0, days)
			expire = &t
		}
		_, err = p.svc.AddUser(core.CreateUserParams{
			Username:  strings.TrimSpace(p.fields[0].Value()),
			DataLimit: data,
			ExpireAt:  expire,
			HWIDLimit: atoiOr(p.fields[3].Value(), 0),
		})
		if err != nil {
			p.status = "error: " + err.Error()
			return nil
		}
		p.status = "created " + p.fields[0].Value()
	} else {
		data, err := parseSize(p.fields[0].Value())
		if err != nil {
			p.status = "error: " + err.Error()
			return nil
		}
		hwid := atoiOr(p.fields[2].Value(), 0)
		params := core.SetUserParams{DataLimit: &data, HWIDLimit: &hwid}
		if daysStr := strings.TrimSpace(p.fields[1].Value()); daysStr != "" {
			t := time.Now().AddDate(0, 0, atoiOr(daysStr, 0))
			params.ExpireAt = &t
		}
		if _, err := p.svc.SetUser(p.editName, params); err != nil {
			p.status = "error: " + err.Error()
			return nil
		}
		p.status = "updated " + p.editName
	}
	p.mode = uList
	return p.load()
}

func (p *usersPanel) openCreate() {
	p.editName = ""
	p.labels = []string{"Username", "Data limit (e.g. 100G, 0=∞)", "Expire in days (0=never)", "Device limit (0=∞)"}
	p.fields = []textinput.Model{
		newInput("alice", ""),
		newInput("0", ""),
		newInput("0", ""),
		newInput("0", ""),
	}
	p.focus = 0
	p.focusInput()
	p.mode = uForm
}

func (p *usersPanel) openEdit(u *store.User) {
	p.editName = u.Username
	p.labels = []string{"Data limit (0=∞)", "Expire in days from now (blank=keep)", "Device limit (0=∞)"}
	dataVal := ""
	if u.DataLimit > 0 {
		dataVal = sizeFlag(u.DataLimit) // parseSize-friendly, e.g. "200.0G"
	}
	p.fields = []textinput.Model{
		newInput("0", dataVal),
		newInput("blank=keep", ""),
		newInput("0", fmt.Sprintf("%d", u.HWIDLimit)),
	}
	p.focus = 0
	p.focusInput()
	p.mode = uForm
}

func (p *usersPanel) focusInput() {
	for i := range p.fields {
		if i == p.focus {
			p.fields[i].Focus()
		} else {
			p.fields[i].Blur()
		}
	}
}

func (p *usersPanel) selected() *store.User {
	i := p.tbl.Cursor()
	if i < 0 || i >= len(p.users) {
		return nil
	}
	return p.users[i]
}

func (p *usersPanel) subURL(u *store.User) string {
	return fmt.Sprintf("%s/sub/%s", strings.TrimRight(p.subBaseURL, "/"), u.SubToken)
}

func (p *usersPanel) openDetail(u *store.User) tea.Cmd {
	p.detail = u
	p.devCursor = 0
	p.devices = nil
	p.detailErr = nil
	p.qr = ""
	if qr, err := qrcode.New(p.subURL(u), qrcode.Low); err == nil {
		p.qr = qr.ToSmallString(false)
	}
	p.mode = uDetail
	return p.loadDevices(u.Username)
}

func (p *usersPanel) loadDevices(name string) tea.Cmd {
	svc := p.svc
	return func() tea.Msg {
		d, err := svc.Devices(name)
		return userDevicesMsg{name: name, devices: d, err: err}
	}
}

func (p *usersPanel) updateDetail(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "esc":
		p.mode = uList
	case "up", "k":
		if p.devCursor > 0 {
			p.devCursor--
		}
	case "down", "j":
		if p.devCursor < len(p.devices)-1 {
			p.devCursor++
		}
	case "x":
		if p.detail != nil && p.devCursor < len(p.devices) {
			d := p.devices[p.devCursor]
			if err := p.svc.RevokeDevice(p.detail.Username, d.ID); err != nil {
				p.detailErr = err
			} else {
				return p.loadDevices(p.detail.Username)
			}
		}
	}
	return nil
}

func (p *usersPanel) view() string {
	switch p.mode {
	case uForm:
		return p.formView()
	case uDetail:
		return p.detailView()
	case uConfirmDelete:
		name := "?"
		if u := p.selected(); u != nil {
			name = u.Username
		}
		return formBox.Render(styleErr.Render("Delete user "+name+"?  ") + styleHint.Render("y / n"))
	default:
		help := styleHint.Render("enter details · n new · e edit · d delete · s on/off · t reset-traffic · g group · ↑↓ move")
		status := p.status
		if p.err != nil {
			status = styleErr.Render("error: " + p.err.Error())
		}
		return p.tbl.View() + "\n" + statusLine(status, help)
	}
}

func (p *usersPanel) formView() string {
	title := "New user"
	if p.editName != "" {
		title = "Edit " + p.editName
	}
	var b strings.Builder
	b.WriteString(styleFormTitle.Render(title) + "\n\n")
	for i, in := range p.fields {
		b.WriteString(labeledField(p.labels[i], in, i == p.focus) + "\n")
	}
	b.WriteString("\n" + styleHint.Render("tab move · enter save · esc cancel"))
	if p.status != "" {
		b.WriteString("\n" + styleErr.Render(p.status))
	}
	return formBox.Render(b.String())
}

func (p *usersPanel) detailView() string {
	u := p.detail
	if u == nil {
		return ""
	}

	var info strings.Builder
	info.WriteString(styleFormTitle.Render(u.Username) + "  " + styleValue.Render("["+string(u.Status)+"]") + "\n")
	kv := func(k, v string) string { return styleKey.Render(fmt.Sprintf("%-9s", k)) + styleValue.Render(v) }
	info.WriteString(kv("Used", humanBytes(u.UsedTraffic)+" / "+limitStr(u.DataLimit)) + "\n")
	info.WriteString(kv("Expires", expireStr(u.ExpireAt)) + "\n")
	info.WriteString(kv("Group", groupName(u.GroupID, p.groups)) + "\n")
	info.WriteString(kv("Proxies", proxyProtocols(u.Proxies)) + "\n\n")
	info.WriteString(styleKey.Render("Sub link") + "\n" + styleValue.Render(p.subURL(u)) + "\n\n")

	info.WriteString(styleSection.Render(fmt.Sprintf("Devices (%d)", len(p.devices))) + "\n")
	if p.detailErr != nil {
		info.WriteString(styleErr.Render("error: "+p.detailErr.Error()) + "\n")
	} else if len(p.devices) == 0 {
		info.WriteString(styleHint.Render("  (no devices seen yet)") + "\n")
	}
	for i, d := range p.devices {
		marker := "  "
		if i == p.devCursor {
			marker = "▸ "
		}
		rev := ""
		if d.Revoked {
			rev = styleErr.Render(" revoked")
		}
		line := fmt.Sprintf("%s%-3d %-14s %-22s %s%s", marker, d.ID, orDash(d.HWID),
			truncate(d.UserAgent, 22), d.LastSeen.Format("01-02 15:04"), rev)
		if i == p.devCursor {
			line = styleFieldLabelActive.Render(line)
		}
		info.WriteString(line + "\n")
	}
	info.WriteString("\n" + styleHint.Render("↑↓ select device · x revoke · esc back"))

	left := lipgloss.NewStyle().Padding(0, 2).Render(info.String())
	if p.qr == "" {
		return left
	}
	qr := lipgloss.NewStyle().Padding(0, 1).Render(p.qr)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, qr)
}

func proxyProtocols(px store.Proxies) string {
	var ps []string
	if px.VMess != nil {
		ps = append(ps, "vmess")
	}
	if px.VLESS != nil {
		ps = append(ps, "vless")
	}
	if px.Trojan != nil {
		ps = append(ps, "trojan")
	}
	if px.Shadowsocks != nil {
		ps = append(ps, "shadowsocks")
	}
	if px.Hysteria != nil {
		ps = append(ps, "hysteria")
	}
	if len(ps) == 0 {
		return "-"
	}
	return strings.Join(ps, ", ")
}

func (p *usersPanel) userRows() []table.Row {
	rows := make([]table.Row, 0, len(p.users))
	for _, u := range p.users {
		rows = append(rows, table.Row{
			u.Username, string(u.Status), humanBytes(u.UsedTraffic),
			limitStr(u.DataLimit), expireStr(u.ExpireAt), groupName(u.GroupID, p.groups),
		})
	}
	return rows
}

// statusLine renders a left status (or blank) and right-aligned help text on
// one line, falling back gracefully when there is no status.
func statusLine(status, help string) string {
	if status == "" {
		return help
	}
	return status + "  " + help
}
