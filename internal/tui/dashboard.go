package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/BX-Team/Nexon/internal/core"
	"github.com/BX-Team/Nexon/internal/store"
)

// dashRefresh is how often the dashboard re-reads stats on its own.
const dashRefresh = 5 * time.Second

type dashMsg struct {
	stats   store.Stats
	clients int
	err     error
}

// dashTickMsg fires on the dashboard's self-refresh timer.
type dashTickMsg time.Time

type dashboardPanel struct {
	svc     *core.Service
	stats   store.Stats
	clients int
	err     error
	loaded  bool
	ticking bool // a single tick chain is live
	updated time.Time
}

func newDashboardPanel(svc *core.Service) panel { return &dashboardPanel{svc: svc} }

func (p *dashboardPanel) title() string   { return "Dashboard" }
func (p *dashboardPanel) capturing() bool { return false }
func (p *dashboardPanel) resize(w, h int) {}

// fetch reads fresh stats off the service (cheap local SQLite queries).
func (p *dashboardPanel) fetch() tea.Cmd {
	svc := p.svc
	return func() tea.Msg {
		stats, err := svc.Store().ComputeStats()
		if err != nil {
			return dashMsg{err: err}
		}
		clients, _ := svc.ListClientApps()
		return dashMsg{stats: stats, clients: len(clients)}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(dashRefresh, func(t time.Time) tea.Msg { return dashTickMsg(t) })
}

func (p *dashboardPanel) load() tea.Cmd {
	// Start exactly one tick chain; manual reloads (`r`) just refetch.
	if !p.ticking {
		p.ticking = true
		return tea.Batch(p.fetch(), tickCmd())
	}
	return p.fetch()
}

func (p *dashboardPanel) update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case dashMsg:
		p.err, p.stats, p.clients, p.loaded = m.err, m.stats, m.clients, true
		p.updated = time.Now()
	case dashTickMsg:
		// Refetch and reschedule the next tick (one continuous chain).
		return tea.Batch(p.fetch(), tickCmd())
	}
	return nil
}

func (p *dashboardPanel) view() string {
	if p.err != nil {
		return styleErr.Render("error: " + p.err.Error())
	}
	if !p.loaded {
		return styleHint.Render("loading…")
	}
	s := p.stats
	kv := func(k, v string) string {
		return styleKey.Render(fmt.Sprintf("%-16s", k)) + styleValue.Render(v)
	}
	users := strings.Join([]string{
		kv("Users total", strconv.Itoa(s.UsersTotal)),
		kv("  active", strconv.Itoa(s.UsersActive)),
		kv("  limited", strconv.Itoa(s.UsersLimited)),
		kv("  expired", strconv.Itoa(s.UsersExpired)),
		kv("  disabled", strconv.Itoa(s.UsersDisabled)),
	}, "\n")
	infra := strings.Join([]string{
		kv("Nodes", fmt.Sprintf("%d (%d online)", s.NodesTotal, s.NodesOnline)),
		kv("Client apps", strconv.Itoa(p.clients)),
	}, "\n")
	box := lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238"))
	cards := lipgloss.JoinHorizontal(lipgloss.Top, box.Render(users), "  ", box.Render(infra))
	live := styleHint.Render(fmt.Sprintf("● live · refreshes every %ds · updated %s",
		int(dashRefresh.Seconds()), p.updated.Format("15:04:05")))
	return cards + "\n" + live
}
