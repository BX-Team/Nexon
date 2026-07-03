package tui

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/BX-Team/Nexon/internal/core"
	"github.com/BX-Team/Nexon/internal/store"
)

type nodesMsg struct {
	nodes  []*store.Node
	groups []*store.NodeGroup
	err    error
}

// nodesPanel lists registered nodes. Creation needs cert files (CLI: `nexon
// node add`), but group assignment is available here with `g`.
type nodesPanel struct {
	svc    *core.Service
	tbl    table.Model
	nodes  []*store.Node
	groups []*store.NodeGroup
	status string
	err    error
}

func newNodesPanel(svc *core.Service) panel {
	cols := []table.Column{
		{Title: "NAME", Width: 16},
		{Title: "ADDRESS", Width: 18},
		{Title: "PORT", Width: 6},
		{Title: "STATUS", Width: 12},
		{Title: "GROUP", Width: 12},
		{Title: "LAST SEEN", Width: 16},
	}
	return &nodesPanel{svc: svc, tbl: newStyledTable(cols)}
}

func (p *nodesPanel) title() string   { return "Nodes" }
func (p *nodesPanel) capturing() bool { return false }
func (p *nodesPanel) resize(w, h int) { p.tbl.SetWidth(w); setTableHeight(&p.tbl, h) }

func (p *nodesPanel) load() tea.Cmd {
	svc := p.svc
	return func() tea.Msg {
		nodes, err := svc.ListNodes()
		if err != nil {
			return nodesMsg{err: err}
		}
		groups, err := svc.ListNodeGroups()
		return nodesMsg{nodes: nodes, groups: groups, err: err}
	}
}

func (p *nodesPanel) update(msg tea.Msg) tea.Cmd {
	if m, ok := msg.(nodesMsg); ok {
		p.err = m.err
		if m.err == nil {
			p.nodes = m.nodes
			p.groups = m.groups
			p.tbl.SetRows(p.nodeRows())
		}
		return nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	if key.String() == "g" {
		if n := p.selected(); n != nil {
			next := nextGroupID(n.GroupID, p.groups)
			if err := p.svc.SetNodeGroup(n.ID, next); err != nil {
				p.status = "error: " + err.Error()
			} else {
				p.status = n.Name + " → group " + groupName(next, p.groups)
				return p.load()
			}
		}
		return nil
	}
	var cmd tea.Cmd
	p.tbl, cmd = p.tbl.Update(key)
	return cmd
}

func (p *nodesPanel) nodeRows() []table.Row {
	rows := make([]table.Row, 0, len(p.nodes))
	for _, n := range p.nodes {
		ls := "-"
		if n.LastSeen != nil {
			ls = n.LastSeen.Format("2006-01-02 15:04")
		}
		rows = append(rows, table.Row{
			n.Name, n.Address, strconv.Itoa(n.APIPort), n.Status, groupName(n.GroupID, p.groups), ls,
		})
	}
	return rows
}

func (p *nodesPanel) selected() *store.Node {
	i := p.tbl.Cursor()
	if i < 0 || i >= len(p.nodes) {
		return nil
	}
	return p.nodes[i]
}

func (p *nodesPanel) view() string {
	if p.err != nil {
		return styleErr.Render("error: " + p.err.Error())
	}
	help := styleHint.Render(fmt.Sprintf("%d nodes · g group · add/remove via `nexon node`", len(p.nodes)))
	return p.tbl.View() + "\n" + statusLine(p.status, help)
}

func newStyledTable(cols []table.Column) table.Model {
	t := table.New(table.WithColumns(cols), table.WithFocused(true), table.WithHeight(10))
	st := table.DefaultStyles()
	st.Header = st.Header.Bold(true).Foreground(lipgloss.Color("63")).
		BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(lipgloss.Color("238"))
	st.Selected = st.Selected.Bold(true).Foreground(lipgloss.Color("231")).Background(lipgloss.Color("63"))
	t.SetStyles(st)
	return t
}

func setTableHeight(t *table.Model, h int) {
	if h > 2 {
		t.SetHeight(h - 2) // leave room for a status/help line
	}
}
