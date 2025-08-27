package tui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

var (
	mainBranchInputStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(1, 2).
				Width(50)

	mainBranchTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Bold(true).
				Margin(1, 0)

	mainBranchLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				Margin(1, 0, 0, 0)

	mainBranchSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("46")).
				Bold(true)
)

type mainBranchInputModel struct {
	textInput textinput.Model
	err       error
	quitting  bool
	result    string
}

func initialMainBranchInputModel() mainBranchInputModel {
	ti := textinput.New()
	ti.Placeholder = "main"
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 30

	return mainBranchInputModel{
		textInput: ti,
		err:       nil,
	}
}

func (m mainBranchInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m mainBranchInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			value := m.textInput.Value()
			if value == "" {
				value = "main" // Default value
			}
			m.result = value
			m.quitting = true
			return m, tea.Quit
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		}

	case error:
		m.err = msg
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m mainBranchInputModel) View() string {
	if m.quitting {
		if m.result != "" {
			return mainBranchSuccessStyle.Render(fmt.Sprintf("✓ Main branch set to: %s", m.result)) + "\n"
		}
		return "Cancelled.\n"
	}

	title := mainBranchTitleStyle.Render("Set Main Branch")
	label := mainBranchLabelStyle.Render("Enter the main branch name to be used when running 'alfred switch main':")
	input := m.textInput.View()

	content := fmt.Sprintf("%s\n%s\n\n%s\n\n%s",
		title,
		label,
		input,
		"Press Enter to confirm, Esc to cancel")

	return mainBranchInputStyle.Render(content) + "\n"
}

// RunMainBranchInput shows TUI for main branch input and returns the branch name
func RunMainBranchInput() (string, error) {
	// Check if we have a TTY available
	if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		return "", fmt.Errorf("TTY not available for interactive main branch input")
	}

	p := tea.NewProgram(initialMainBranchInputModel())
	m, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("failed to run main branch input TUI: %w", err)
	}

	finalModel := m.(mainBranchInputModel)
	if finalModel.result == "" {
		return "", fmt.Errorf("main branch input cancelled")
	}

	return finalModel.result, nil
}
