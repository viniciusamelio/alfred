package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	deleterTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("196")).
				MarginBottom(1)

	deleteCheckboxStyle = lipgloss.NewStyle().
				PaddingLeft(2)

	deleteCheckedStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("196"))

	deleteSelectedCheckboxStyle = lipgloss.NewStyle().
					PaddingLeft(0).
					Foreground(lipgloss.Color("170"))

	deleteInputLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")).
				MarginTop(1).
				MarginBottom(1)

	deleteHelpTextStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("243")).
				MarginTop(2)

	deleteErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				MarginTop(1)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			MarginTop(1)
)

type deleteContextItem struct {
	name    string
	current bool
	checked bool
}

type ContextDeleterModel struct {
	contexts       []deleteContextItem
	cursor         int
	confirmInput   textinput.Model
	step           int // 0: context selection, 1: confirmation
	finished       bool
	cancelled      bool
	selectedNames  []string
	error          string
	currentContext string
}

func NewContextDeleter(contextNames []string, currentContext string) *ContextDeleterModel {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 20
	ti.Width = 20
	ti.Placeholder = "Type 'DELETE' to confirm..."

	contexts := make([]deleteContextItem, len(contextNames))
	for i, name := range contextNames {
		contexts[i] = deleteContextItem{
			name:    name,
			current: name == currentContext,
			checked: false,
		}
	}

	return &ContextDeleterModel{
		contexts:       contexts,
		confirmInput:   ti,
		step:           0,
		currentContext: currentContext,
	}
}

func (m ContextDeleterModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m ContextDeleterModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelled = true
			return m, tea.Quit

		case "enter":
			if m.step == 0 {
				// Context selection step
				selectedCount := 0
				for _, ctx := range m.contexts {
					if ctx.checked {
						selectedCount++
						m.selectedNames = append(m.selectedNames, ctx.name)
					}
				}

				if selectedCount == 0 {
					m.error = "Please select at least one context to delete"
					return m, nil
				}

				// Check if trying to delete current context
				for _, ctx := range m.contexts {
					if ctx.checked && ctx.current {
						m.error = "Cannot delete the current active context. Switch to another context first."
						return m, nil
					}
				}

				m.step = 1
				m.error = ""
				return m, nil
			} else {
				// Confirmation step
				confirmation := strings.TrimSpace(m.confirmInput.Value())
				if confirmation != "DELETE" {
					m.error = "You must type 'DELETE' to confirm deletion"
					return m, nil
				}

				m.finished = true
				return m, tea.Quit
			}

		case "up", "k":
			if m.step == 0 && m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.step == 0 && m.cursor < len(m.contexts)-1 {
				m.cursor++
			}

		case " ":
			if m.step == 0 {
				m.contexts[m.cursor].checked = !m.contexts[m.cursor].checked
				m.error = ""
			}

		case "esc":
			if m.step == 1 {
				m.step = 0
				m.selectedNames = nil
				m.confirmInput.SetValue("")
				m.error = ""
				return m, nil
			} else {
				m.cancelled = true
				return m, tea.Quit
			}
		}
	}

	if m.step == 1 {
		m.confirmInput, cmd = m.confirmInput.Update(msg)
	}

	return m, cmd
}

func (m ContextDeleterModel) View() string {
	if m.cancelled {
		return "Context deletion cancelled.\n"
	}

	if m.finished {
		return fmt.Sprintf("✅ Contexts %s will be deleted\n",
			strings.Join(m.selectedNames, ", "))
	}

	var b strings.Builder

	if m.step == 0 {
		// Context selection step
		b.WriteString(deleterTitleStyle.Render("⚠️  Delete Contexts"))
		b.WriteString("\n\n")
		b.WriteString("Select contexts to delete:\n\n")

		for i, ctx := range m.contexts {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
			}

			checked := "☐"
			style := deleteCheckboxStyle
			if ctx.checked {
				checked = "☑"
				style = deleteCheckedStyle
			}

			status := ""
			if ctx.current {
				status = " (current - cannot delete)"
			}

			line := fmt.Sprintf("%s %s %s%s", cursor, checked, ctx.name, status)

			if m.cursor == i {
				line = deleteSelectedCheckboxStyle.Render(line)
			} else {
				line = style.Render(line)
			}

			b.WriteString(line)
			b.WriteString("\n")
		}

		if m.error != "" {
			b.WriteString(deleteErrorStyle.Render(m.error))
			b.WriteString("\n")
		}

		b.WriteString(deleteHelpTextStyle.Render("↑/↓ navigate • Space select • Enter continue • Esc cancel"))
	} else {
		// Confirmation step
		b.WriteString(deleterTitleStyle.Render("⚠️  DANGER ZONE"))
		b.WriteString("\n\n")
		b.WriteString(warningStyle.Render("You are about to delete the following contexts:"))
		b.WriteString("\n")
		for _, name := range m.selectedNames {
			b.WriteString(deleteErrorStyle.Render(fmt.Sprintf("• %s", name)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(warningStyle.Render("This action will:"))
		b.WriteString("\n")
		b.WriteString("• Remove all worktrees for these contexts\n")
		b.WriteString("• Delete branches for these contexts\n")
		b.WriteString("• Remove contexts from configuration\n")
		b.WriteString("• THIS CANNOT BE UNDONE\n")
		b.WriteString("\n")

		b.WriteString(deleteInputLabelStyle.Render("Type 'DELETE' to confirm:"))
		b.WriteString("\n")
		b.WriteString(m.confirmInput.View())
		b.WriteString("\n")

		if m.error != "" {
			b.WriteString(deleteErrorStyle.Render(m.error))
			b.WriteString("\n")
		}

		b.WriteString(deleteHelpTextStyle.Render("Enter to confirm • Esc to go back"))
	}

	return b.String()
}

func (m ContextDeleterModel) GetResult() ([]string, bool) {
	if m.cancelled || !m.finished {
		return nil, false
	}
	return m.selectedNames, true
}

func RunContextDeleter(contextNames []string, currentContext string) ([]string, error) {
	if len(contextNames) == 0 {
		return nil, fmt.Errorf("no contexts available")
	}

	// Filter out main context - it cannot be deleted
	var deletableContexts []string
	for _, ctx := range contextNames {
		if ctx != "main" && ctx != "master" {
			deletableContexts = append(deletableContexts, ctx)
		}
	}

	if len(deletableContexts) == 0 {
		return nil, fmt.Errorf("no deletable contexts available (main context cannot be deleted)")
	}

	if len(deletableContexts) == 1 && deletableContexts[0] == currentContext {
		return nil, fmt.Errorf("cannot delete the only context, and it's currently active")
	}

	m := NewContextDeleter(deletableContexts, currentContext)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running context deleter: %w", err)
	}

	// Try both pointer and value types
	if model, ok := finalModel.(*ContextDeleterModel); ok {
		contexts, success := model.GetResult()
		if !success {
			return nil, fmt.Errorf("context deletion cancelled")
		}
		return contexts, nil
	}

	if model, ok := finalModel.(ContextDeleterModel); ok {
		contexts, success := model.GetResult()
		if !success {
			return nil, fmt.Errorf("context deletion cancelled")
		}
		return contexts, nil
	}

	return nil, fmt.Errorf("unexpected model type: %T", finalModel)
}
