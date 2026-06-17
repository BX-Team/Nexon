// Package tui is an interactive terminal cockpit for Nexon, built on Bubble
// Tea (the Elm architecture). It is a thin view over core.Service — the exact
// same business logic the CLI and subscription server use, never a second
// implementation. Tabs: Dashboard, Users, Nodes, Clients.
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/BX-Team/Nexon/internal/core"
)

// Run launches the TUI over svc and blocks until the user quits. subBaseURL is
// the public base used to render subscription links (same as `nexon user sub`).
func Run(svc *core.Service, subBaseURL string) error {
	p := tea.NewProgram(newModel(svc, subBaseURL), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// panel is one switchable tab. Implementations mutate through pointer receivers
// so the root model can hold them as a stable slice.
type panel interface {
	title() string
	load() tea.Cmd            // (re)fetch this panel's data from the service
	update(tea.Msg) tea.Cmd   // handle a message (data result or, when focused, a key)
	view() string             // render the panel body
	resize(width, height int) // body area available to the panel
	capturing() bool          // true while a form/dialog needs every keystroke
}

type model struct {
	panels []panel
	active int
	width  int
	height int
}

func newModel(svc *core.Service, subBaseURL string) model {
	return model{
		panels: []panel{
			newDashboardPanel(svc),
			newUsersPanel(svc, subBaseURL),
			newNodesPanel(svc),
			newGroupsPanel(svc),
			newClientsPanel(svc),
			newTemplatesPanel(svc),
			newSettingsPanel(svc),
		},
	}
}

func (m model) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.panels))
	for _, p := range m.panels {
		cmds = append(cmds, p.load())
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		bodyH := msg.Height - chromeHeight
		if bodyH < 1 {
			bodyH = 1
		}
		for _, p := range m.panels {
			p.resize(msg.Width, bodyH)
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		// While a panel is in a form/dialog, it owns every keystroke so the user
		// can type (tab navigates fields, q types a 'q', etc.).
		if m.panels[m.active].capturing() {
			return m, m.panels[m.active].update(msg)
		}
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "tab", "right", "l":
			m.active = (m.active + 1) % len(m.panels)
			return m, nil
		case "shift+tab", "left", "h":
			m.active = (m.active - 1 + len(m.panels)) % len(m.panels)
			return m, nil
		case "r":
			return m, m.panels[m.active].load()
		default:
			// Forward navigation keys to the focused panel only.
			return m, m.panels[m.active].update(msg)
		}

	default:
		// Data-result messages: let every panel pick up the ones it owns.
		var cmds []tea.Cmd
		for _, p := range m.panels {
			if cmd := p.update(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)
	}
}

func (m model) View() string {
	if m.width == 0 {
		return "loading…"
	}
	return strings.Join([]string{
		m.tabBar(),
		m.panels[m.active].view(),
		footer(),
	}, "\n")
}

func (m model) tabBar() string {
	tabs := make([]string, len(m.panels))
	for i, p := range m.panels {
		if i == m.active {
			tabs[i] = styleTabActive.Render(p.title())
		} else {
			tabs[i] = styleTabInactive.Render(p.title())
		}
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)
	return styleBrand.Render("Nexon") + "  " + bar
}

func footer() string {
	return styleFooter.Render("tab/←→ switch · r refresh · q quit")
}

// chromeHeight is the number of lines the tab bar + footer + spacing consume,
// so panels know how much height is left for their body.
const chromeHeight = 4

var (
	brand = lipgloss.Color("63") // indigo

	styleBrand = lipgloss.NewStyle().Bold(true).Foreground(brand)

	styleTabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("231")).
			Background(brand).
			Padding(0, 2)

	styleTabInactive = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				Padding(0, 2)

	styleFooter = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	styleErr = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))

	styleKey   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleValue = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231"))
)
