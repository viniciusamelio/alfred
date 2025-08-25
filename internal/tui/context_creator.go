package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	creatorTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("62")).
				MarginBottom(1)

	checkboxStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	checkedStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			Foreground(lipgloss.Color("86"))

	selectedCheckboxStyle = lipgloss.NewStyle().
				PaddingLeft(0).
				Foreground(lipgloss.Color("170"))

	inputLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			MarginTop(1).
			MarginBottom(1)

	helpTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			MarginTop(2)

	creatorErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				MarginTop(1)
)

type repoItem struct {
	alias   string
	path    string
	checked bool
}

type ContextCreatorModel struct {
	repos       []repoItem
	cursor      int
	nameInput   textinput.Model
	step        int // 0: name input, 1: repo selection
	finished    bool
	cancelled   bool
	contextName string
	selectedRepos []string
	error       string
}

func NewContextCreator(repoAliases []string, repoPaths []string) *ContextCreatorModel {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 30
	ti.Placeholder = "Enter context name..."

	repos := make([]repoItem, len(repoAliases))
	for i, alias := range repoAliases {
		repos[i] = repoItem{
			alias: alias,
			path:  repoPaths[i],
			checked: false,
		}
	}

	return &ContextCreatorModel{
		repos:     repos,
		nameInput: ti,
		step:      0,
	}
}

func (m ContextCreatorModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m ContextCreatorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelled = true
			return m, tea.Quit

		case "enter":
			if m.step == 0 {
				// Name input step
				name := strings.TrimSpace(m.nameInput.Value())
				if name == "" {
					m.error = "Context name cannot be empty"
					return m, nil
				}
				m.contextName = name
				m.step = 1
				m.error = ""
				return m, nil
			} else {
				// Repo selection step - confirm creation
				selectedCount := 0
				for _, repo := range m.repos {
					if repo.checked {
						selectedCount++
						m.selectedRepos = append(m.selectedRepos, repo.alias)
					}
				}
				
				if selectedCount == 0 {
					m.error = "Please select at least one repository"
					return m, nil
				}
				
				m.finished = true
				return m, tea.Quit
			}

		case "up", "k":
			if m.step == 1 && m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.step == 1 && m.cursor < len(m.repos)-1 {
				m.cursor++
			}

		case " ":
			if m.step == 1 {
				m.repos[m.cursor].checked = !m.repos[m.cursor].checked
				m.error = ""
			}

		case "esc":
			if m.step == 1 {
				m.step = 0
				m.nameInput.Focus()
				return m, textinput.Blink
			} else {
				m.cancelled = true
				return m, tea.Quit
			}
		}

	}

	if m.step == 0 {
		m.nameInput, cmd = m.nameInput.Update(msg)
	}

	return m, cmd
}

func (m ContextCreatorModel) View() string {
	if m.cancelled {
		return "Context creation cancelled.\n"
	}

	if m.finished {
		return fmt.Sprintf("✅ Context '%s' will be created with repositories: %s\n", 
			m.contextName, strings.Join(m.selectedRepos, ", "))
	}

	var b strings.Builder

	if m.step == 0 {
		// Name input step
		b.WriteString(creatorTitleStyle.Render("Create New Context"))
		b.WriteString("\n\n")
		b.WriteString(inputLabelStyle.Render("Context Name:"))
		b.WriteString("\n")
		b.WriteString(m.nameInput.View())
		b.WriteString("\n")
		
		if m.error != "" {
			b.WriteString(creatorErrorStyle.Render(m.error))
			b.WriteString("\n")
		}
		
		b.WriteString(helpTextStyle.Render("Press Enter to continue, Esc to cancel"))
	} else {
		// Repository selection step
		b.WriteString(creatorTitleStyle.Render(fmt.Sprintf("Select repositories for '%s'", m.contextName)))
		b.WriteString("\n\n")

		for i, repo := range m.repos {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
			}

			checked := "☐"
			style := checkboxStyle
			if repo.checked {
				checked = "☑"
				style = checkedStyle
			}

			line := fmt.Sprintf("%s %s %s (%s)", cursor, checked, repo.alias, repo.path)
			
			if m.cursor == i {
				line = selectedCheckboxStyle.Render(line)
			} else {
				line = style.Render(line)
			}
			
			b.WriteString(line)
			b.WriteString("\n")
		}

		if m.error != "" {
			b.WriteString(creatorErrorStyle.Render(m.error))
			b.WriteString("\n")
		}

		b.WriteString(helpTextStyle.Render("↑/↓ navigate • Space select • Enter confirm • Esc back"))
	}

	return b.String()
}

func (m ContextCreatorModel) GetResult() (string, []string, bool) {
	if m.cancelled || !m.finished {
		return "", nil, false
	}
	return m.contextName, m.selectedRepos, true
}

func RunRepoSelector(repoAliases []string, repoPaths []string) ([]string, error) {
	m := NewContextCreator(repoAliases, repoPaths)
	m.step = 1 // Skip context name input, go directly to repo selection
	
	p := tea.NewProgram(m)
	
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running repo selector: %w", err)
	}
	
	// Try both pointer and value types
	if model, ok := finalModel.(*ContextCreatorModel); ok {
		_, repos, success := model.GetResult()
		if !success {
			return nil, fmt.Errorf("repo selection cancelled")
		}
		return repos, nil
	}
	
	if model, ok := finalModel.(ContextCreatorModel); ok {
		_, repos, success := model.GetResult()
		if !success {
			return nil, fmt.Errorf("repo selection cancelled")
		}
		return repos, nil
	}
	
	return nil, fmt.Errorf("unexpected model type: %T", finalModel)
}

func RunContextCreator(repoAliases []string, repoPaths []string) (string, []string, error) {
	if len(repoAliases) == 0 {
		return "", nil, fmt.Errorf("no repositories available")
	}

	m := NewContextCreator(repoAliases, repoPaths)
	p := tea.NewProgram(m)
	
	finalModel, err := p.Run()
	if err != nil {
		return "", nil, fmt.Errorf("error running context creator: %w", err)
	}

	// Try both pointer and value types
	if model, ok := finalModel.(*ContextCreatorModel); ok {
		name, repos, success := model.GetResult()
		if !success {
			return "", nil, fmt.Errorf("context creation cancelled")
		}
		return name, repos, nil
	}
	
	if model, ok := finalModel.(ContextCreatorModel); ok {
		name, repos, success := model.GetResult()
		if !success {
			return "", nil, fmt.Errorf("context creation cancelled")
		}
		return name, repos, nil
	}

	return "", nil, fmt.Errorf("unexpected model type: %T", finalModel)
}