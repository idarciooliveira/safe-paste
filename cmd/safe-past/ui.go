package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/idarciooliveira/safe-past/internal/preview"
)

var (
	uiTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	uiStat  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	uiRed   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	uiDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	uiHelp  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
)

type uiModel struct {
	vp       viewport.Model
	lines    []preview.Line
	stats    string
	accepted bool
	ready    bool
}

func newUIModel(orig, clean, stats string, width, height int) uiModel {
	lines := preview.DiffLines(orig, clean)
	var sb strings.Builder
	red := 0
	for _, l := range lines {
		if l.Redacted {
			red++
			sb.WriteString(uiRed.Render("~ " + l.Clean))
		} else {
			sb.WriteString(uiDim.Render("  " + l.Clean))
		}
		sb.WriteString("\n")
	}
	vp := viewport.New(width, max(height-4, 5))
	vp.SetContent(strings.TrimRight(sb.String(), "\n"))
	return uiModel{vp: vp, lines: lines, stats: stats}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m uiModel) Init() tea.Cmd { return nil }

func (m uiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.vp.Width = msg.Width
		m.vp.Height = max(msg.Height-4, 5)
		m.ready = true
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "y", "enter":
			m.accepted = true
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			m.accepted = false
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m uiModel) View() string {
	var sb strings.Builder
	sb.WriteString(uiTitle.Render("safe-past preview"))
	sb.WriteString("  ")
	sb.WriteString(uiStat.Render(m.stats))
	sb.WriteString("\n\n")
	sb.WriteString(m.vp.View())
	sb.WriteString("\n")
	sb.WriteString(uiHelp.Render("y/enter copy+quit · ↑↓/j/k scroll · q quit without copying"))
	return sb.String()
}

// runUI shows the preview. Returns accepted=false when the user aborts.
// Callers must check for a TTY before invoking; test/dump path uses DumpUI.
func runUI(orig, clean, stats string) (bool, error) {
	m := newUIModel(orig, clean, stats, 80, 24)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return false, err
	}
	if um, ok := final.(uiModel); ok {
		return um.accepted, nil
	}
	return false, fmt.Errorf("unexpected model type")
}

// DumpUI renders one static frame (no TTY needed) for tests / headless proof.
func DumpUI(orig, clean, stats string) string {
	m := newUIModel(orig, clean, stats, 80, 20)
	return m.View()
}
