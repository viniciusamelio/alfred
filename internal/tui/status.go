package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	statusTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("62")).
				MarginBottom(1)

	contextStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	repoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86"))

	branchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	modifiedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	noContextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Italic(true)
)

type StatusModel struct {
	currentContext string
	repoStatus     map[string]string
	width          int
	height         int
}

func NewStatusModel(currentContext string, repoStatus map[string]string) *StatusModel {
	return &StatusModel{
		currentContext: currentContext,
		repoStatus:     repoStatus,
	}
}

func (m StatusModel) Init() tea.Cmd {
	return tea.Quit
}

func (m StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

func (m StatusModel) View() string {
	var b strings.Builder

	b.WriteString(statusTitleStyle.Render("Alfred Project Status"))
	b.WriteString("\n\n")

	if m.currentContext == "" {
		b.WriteString(noContextStyle.Render("No context is currently active."))
		b.WriteString("\n")
		b.WriteString(noContextStyle.Render("Use 'alfred context switch' to activate a context."))
		return b.String()
	}

	b.WriteString("Current Context: ")
	b.WriteString(contextStyle.Render(m.currentContext))
	b.WriteString("\n\n")

	if len(m.repoStatus) == 0 {
		b.WriteString(noContextStyle.Render("No repositories in current context."))
		return b.String()
	}

	b.WriteString(statusTitleStyle.Render("Repository Status:"))
	b.WriteString("\n")

	for repo, status := range m.repoStatus {
		b.WriteString("  ")
		b.WriteString(repoStyle.Render(repo))
		b.WriteString(": ")

		if strings.Contains(status, "error") || strings.Contains(status, "Error") {
			b.WriteString(errorStyle.Render(status))
		} else if strings.Contains(status, "modified") {
			parts := strings.Split(status, " ")
			if len(parts) > 0 {
				b.WriteString(branchStyle.Render(parts[0]))
				if len(parts) > 1 {
					b.WriteString(" ")
					b.WriteString(modifiedStyle.Render(strings.Join(parts[1:], " ")))
				}
			}
		} else {
			b.WriteString(branchStyle.Render(status))
		}

		b.WriteString("\n")
	}

	return b.String()
}

func RunStatusView(currentContext string, repoStatus map[string]string) error {
	m := NewStatusModel(currentContext, repoStatus)

	opts := []tea.ProgramOption{}

	p := tea.NewProgram(m, opts...)

	_, err := p.Run()
	return err
}
