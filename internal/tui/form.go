package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// newInput builds a text input pre-filled with value and a placeholder.
func newInput(placeholder, value string) textinput.Model {
	in := textinput.New()
	in.Placeholder = placeholder
	in.SetValue(value)
	in.CharLimit = 0
	in.Width = 40
	in.Prompt = "" // labels are rendered separately
	return in
}

// labeledField renders one "label: <input>" row, highlighting it when focused.
func labeledField(label string, in textinput.Model, focused bool) string {
	l := styleFieldLabel.Render(label)
	if focused {
		l = styleFieldLabelActive.Render("▸ " + label)
	} else {
		l = styleFieldLabel.Render("  " + label)
	}
	return l + "  " + in.View()
}

// labeledToggle renders a boolean field (no text input).
func labeledToggle(label string, on, focused bool) string {
	box := "[ ]"
	if on {
		box = "[✓]"
	}
	prefix := "  "
	style := styleFieldLabel
	if focused {
		prefix = "▸ "
		style = styleFieldLabelActive
	}
	return style.Render(prefix+label) + "  " + box
}

// labeledChoice renders a selector field showing its current value.
func labeledChoice(label, value string, focused bool) string {
	prefix := "  "
	style := styleFieldLabel
	if focused {
		prefix = "▸ "
		style = styleFieldLabelActive
	}
	return style.Render(prefix+label) + "  ‹ " + styleValue.Render(value) + " ›"
}

var (
	styleFieldLabel       = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleFieldLabelActive = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	styleFormTitle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231"))
	styleOK               = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	styleHint             = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleSection          = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).
				Bold(true).MarginTop(1)
)

// formBox frames a form body.
var formBox = lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63"))
