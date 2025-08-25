package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			MarginLeft(2).
			Foreground(lipgloss.Color("62"))

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(4)

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("170"))

	paginationStyle = list.DefaultStyles().PaginationStyle.
			PaddingLeft(4)

	helpStyle = list.DefaultStyles().HelpStyle.
			PaddingLeft(4).
			PaddingBottom(1)

	quitTextStyle = lipgloss.NewStyle().
			Margin(1, 0, 2, 4)
)

type contextItem struct {
	name        string
	description string
	current     bool
}

func (i contextItem) FilterValue() string { return i.name }

type contextItemDelegate struct{}

func (d contextItemDelegate) Height() int                             { return 1 }
func (d contextItemDelegate) Spacing() int                            { return 0 }
func (d contextItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d contextItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(contextItem)
	if !ok {
		return
	}

	var prefix string
	if i.current {
		prefix = "● "
	} else {
		prefix = "  "
	}

	str := fmt.Sprintf("%s%s", prefix, i.name)
	
	if i.description != "" {
		str += fmt.Sprintf(" - %s", i.description)
	}

	isSelected := index == m.Index()
	if isSelected {
		fmt.Fprint(w, selectedItemStyle.Render("> "+str))
	} else {
		fmt.Fprint(w, itemStyle.Render(str))
	}
}

type ContextSelectorModel struct {
	list        list.Model
	choice      string
	quitting    bool
	currentContext string
}

func NewContextSelector(contexts []string, currentContext string) *ContextSelectorModel {
	items := make([]list.Item, len(contexts))
	for i, ctx := range contexts {
		description := ""
		if ctx == "main" {
			description = "main/master branches for all repos"
		}
		items[i] = contextItem{
			name:        ctx,
			description: description,
			current:     ctx == currentContext,
		}
	}

	const defaultWidth = 20
	const listHeight = 14

	l := list.New(items, contextItemDelegate{}, defaultWidth, listHeight)
	l.Title = "Select Context"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "select"),
			),
		}
	}

	return &ContextSelectorModel{
		list:           l,
		currentContext: currentContext,
	}
}

func (m ContextSelectorModel) Init() tea.Cmd {
	return nil
}

func (m ContextSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			i, ok := m.list.SelectedItem().(contextItem)
			if ok {
				m.choice = i.name
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m ContextSelectorModel) View() string {
	if m.choice != "" {
		return quitTextStyle.Render(fmt.Sprintf("Switching to context: %s", m.choice))
	}
	if m.quitting {
		return quitTextStyle.Render("Operation cancelled.")
	}
	return "\n" + m.list.View()
}

func (m ContextSelectorModel) GetChoice() string {
	return m.choice
}

func RunContextSelector(contexts []string, currentContext string) (string, error) {
	if len(contexts) == 0 {
		return "", fmt.Errorf("no contexts available")
	}

	m := NewContextSelector(contexts, currentContext)
	p := tea.NewProgram(m)
	
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running context selector: %w", err)
	}

	// Try both pointer and value types
	if model, ok := finalModel.(*ContextSelectorModel); ok {
		return model.GetChoice(), nil
	}
	
	if model, ok := finalModel.(ContextSelectorModel); ok {
		return model.GetChoice(), nil
	}

	return "", fmt.Errorf("unexpected model type: %T", finalModel)
}