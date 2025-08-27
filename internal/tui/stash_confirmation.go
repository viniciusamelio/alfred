package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	confirmationStyle = lipgloss.NewStyle().
				Margin(1, 2).
				Padding(1, 2).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62"))

	stashWarningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("208")).
				Bold(true)

	promptStyle = lipgloss.NewStyle().
			Margin(1, 0)

	optionStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	selectedOptionStyle = lipgloss.NewStyle().
				PaddingLeft(0).
				Foreground(lipgloss.Color("170")).
				Bold(true)
)

type StashConfirmationModel struct {
	contextName    string
	repoName       string
	selectedOption int
	confirmed      bool
	cancelled      bool
}

func NewStashConfirmation(contextName, repoName string) *StashConfirmationModel {
	return &StashConfirmationModel{
		contextName:    contextName,
		repoName:       repoName,
		selectedOption: 0, // Default to "Yes"
	}
}

func (m StashConfirmationModel) Init() tea.Cmd {
	return nil
}

func (m StashConfirmationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "q":
			m.cancelled = true
			return m, tea.Quit

		case "up", "k":
			if m.selectedOption > 0 {
				m.selectedOption--
			}

		case "down", "j":
			if m.selectedOption < 1 {
				m.selectedOption++
			}

		case "enter":
			m.confirmed = m.selectedOption == 0
			return m, tea.Quit

		case "y", "Y":
			m.confirmed = true
			return m, tea.Quit

		case "n", "N":
			m.confirmed = false
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m StashConfirmationModel) View() string {
	if m.confirmed || m.cancelled {
		return ""
	}

	var content string

	content += stashWarningStyle.Render("⚠️  Uncommitted changes detected!") + "\n\n"

	content += fmt.Sprintf("Repository: %s\n", m.repoName)
	content += fmt.Sprintf("Switching from context '%s' to 'main'\n\n", m.contextName)

	content += "Your uncommitted changes will be stashed and can be restored\n"
	content += "when you return to this context.\n\n"

	content += promptStyle.Render("Do you want to proceed?")

	// Options
	options := []string{"Yes, stash changes", "No, cancel switch"}
	for i, option := range options {
		if i == m.selectedOption {
			content += selectedOptionStyle.Render(fmt.Sprintf("> %s", option))
		} else {
			content += optionStyle.Render(fmt.Sprintf("  %s", option))
		}
		content += "\n"
	}

	content += "\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("Use arrow keys to navigate • Enter to confirm • Y/N for quick selection • Ctrl+C to cancel")

	return confirmationStyle.Render(content)
}

func (m StashConfirmationModel) IsConfirmed() bool {
	return m.confirmed
}

func (m StashConfirmationModel) IsCancelled() bool {
	return m.cancelled
}

func RunStashConfirmation(contextName, repoName string) (bool, error) {
	m := NewStashConfirmation(contextName, repoName)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return false, fmt.Errorf("error running stash confirmation: %w", err)
	}

	// Try both pointer and value types
	if model, ok := finalModel.(*StashConfirmationModel); ok {
		if model.IsCancelled() {
			return false, fmt.Errorf("operation cancelled by user")
		}
		return model.IsConfirmed(), nil
	}

	if model, ok := finalModel.(StashConfirmationModel); ok {
		if model.IsCancelled() {
			return false, fmt.Errorf("operation cancelled by user")
		}
		return model.IsConfirmed(), nil
	}

	return false, fmt.Errorf("unexpected model type: %T", finalModel)
}
