package tui

import (
	"strconv"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/BX-Team/Nexon/internal/core"
	"github.com/BX-Team/Nexon/internal/store"
)

type groupsMsg struct {
	groups []*store.NodeGroup
	err    error
}

type groupMode int

const (
	gList groupMode = iota
	gForm
	gConfirmDelete
)

type groupsPanel struct {
	svc    *core.Service
	tbl    table.Model
	groups []*store.NodeGroup
	mode   groupMode
	name   textinput.Model
	status string
	err    error
}

func newGroupsPanel(svc *core.Service) panel {
	cols := []table.Column{
		{Title: "ID", Width: 5},
		{Title: "NAME", Width: 24},
		{Title: "DEFAULT", Width: 8},
	}
	return &groupsPanel{svc: svc, tbl: newStyledTable(cols)}
}

func (p *groupsPanel) title() string   { return "Groups" }
func (p *groupsPanel) capturing() bool { return p.mode != gList }
func (p *groupsPanel) resize(w, h int) { p.tbl.SetWidth(w); setTableHeight(&p.tbl, h) }

func (p *groupsPanel) load() tea.Cmd {
	svc := p.svc
	return func() tea.Msg {
		g, err := svc.ListNodeGroups()
		return groupsMsg{groups: g, err: err}
	}
}

func (p *groupsPanel) update(msg tea.Msg) tea.Cmd {
	if m, ok := msg.(groupsMsg); ok {
		p.err = m.err
		if m.err == nil {
			p.groups = m.groups
			rows := make([]table.Row, 0, len(m.groups))
			for _, g := range m.groups {
				def := ""
				if g.IsDefault {
					def = "✓"
				}
				rows = append(rows, table.Row{strconv.FormatInt(g.ID, 10), g.Name, def})
			}
			p.tbl.SetRows(rows)
		}
		return nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch p.mode {
	case gForm:
		return p.updateForm(key)
	case gConfirmDelete:
		return p.updateConfirm(key)
	default:
		return p.updateList(key)
	}
}

func (p *groupsPanel) updateList(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "n":
		p.name = newInput("group name", "")
		p.name.Focus()
		p.mode = gForm
		return textinput.Blink
	case "d":
		if g := p.selected(); g != nil {
			p.mode = gConfirmDelete
		}
	case "s":
		if g := p.selected(); g != nil && !g.IsDefault {
			if err := p.svc.SetDefaultNodeGroup(g.ID); err != nil {
				p.status = "error: " + err.Error()
				return nil
			}
			p.status = g.Name + " is now the default group"
			return p.load()
		}
	default:
		var cmd tea.Cmd
		p.tbl, cmd = p.tbl.Update(key)
		return cmd
	}
	return nil
}

func (p *groupsPanel) updateForm(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "esc":
		p.mode = gList
		p.status = ""
	case "enter":
		if _, err := p.svc.CreateNodeGroup(p.name.Value()); err != nil {
			p.status = "error: " + err.Error()
			return nil
		}
		p.status = "created " + p.name.Value()
		p.mode = gList
		return p.load()
	default:
		var cmd tea.Cmd
		p.name, cmd = p.name.Update(key)
		return cmd
	}
	return nil
}

func (p *groupsPanel) updateConfirm(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "y", "Y", "enter":
		g := p.selected()
		p.mode = gList
		if g == nil {
			return nil
		}
		if err := p.svc.DeleteNodeGroup(g.ID); err != nil {
			p.status = "error: " + err.Error()
			return nil
		}
		p.status = "deleted " + g.Name
		return p.load()
	default:
		p.mode = gList
	}
	return nil
}

// defaultGroupID finds the flagged default in a loaded group list.
func defaultGroupID(groups []*store.NodeGroup) int64 {
	for _, g := range groups {
		if g.IsDefault {
			return g.ID
		}
	}
	return store.DefaultGroupID
}

// groupName resolves a nullable group id to its display name.
func groupName(id *int64, groups []*store.NodeGroup) string {
	gid := defaultGroupID(groups)
	if id != nil {
		gid = *id
	}
	for _, g := range groups {
		if g.ID == gid {
			return g.Name
		}
	}
	return "Default"
}

// nextGroupID returns the id of the group after the current one (wrapping), for
// cycling a user/node through groups with a single key.
func nextGroupID(cur *int64, groups []*store.NodeGroup) *int64 {
	if len(groups) == 0 {
		return cur
	}
	curID := defaultGroupID(groups)
	if cur != nil {
		curID = *cur
	}
	idx := 0
	for i, g := range groups {
		if g.ID == curID {
			idx = i
			break
		}
	}
	id := groups[(idx+1)%len(groups)].ID
	return &id
}

func (p *groupsPanel) selected() *store.NodeGroup {
	i := p.tbl.Cursor()
	if i < 0 || i >= len(p.groups) {
		return nil
	}
	return p.groups[i]
}

func (p *groupsPanel) view() string {
	switch p.mode {
	case gForm:
		return formBox.Render(styleFormTitle.Render("New group") + "\n\n" +
			labeledField("Name", p.name, true) + "\n\n" + styleHint.Render("enter create · esc cancel"))
	case gConfirmDelete:
		name := "?"
		if g := p.selected(); g != nil {
			name = g.Name
		}
		return formBox.Render(styleErr.Render("Delete group "+name+"?  ") + styleHint.Render("y / n") +
			"\n" + styleHint.Render("(its nodes/users revert to Default)"))
	default:
		help := styleHint.Render("n new · s set default · d delete · ↑↓ move · assign in Users/Nodes with g")
		status := p.status
		if p.err != nil {
			status = styleErr.Render("error: " + p.err.Error())
		}
		return p.tbl.View() + "\n" + statusLine(status, help)
	}
}
