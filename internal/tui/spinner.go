package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("62"))

	scanningTextStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("252")).
				MarginLeft(1)

	foundTextStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			MarginLeft(1)
)

type ScanningModel struct {
	spinner  spinner.Model
	scanning bool
	found    int
	done     bool
}

type scanCompleteMsg struct {
	count int
}

func NewScanningModel() *ScanningModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return &ScanningModel{
		spinner:  s,
		scanning: true,
		done:     false,
	}
}

func (m ScanningModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			// Simulate scanning time
			time.Sleep(500 * time.Millisecond)
			return scanCompleteMsg{count: m.found}
		},
	)
}

func (m ScanningModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case scanCompleteMsg:
		m.scanning = false
		m.found = msg.count
		m.done = true
		return m, tea.Quit

	case spinner.TickMsg:
		if m.scanning {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m ScanningModel) View() string {
	if m.done {
		return foundTextStyle.Render(fmt.Sprintf("✅ Found %d Dart/Flutter packages", m.found))
	}

	var b strings.Builder
	b.WriteString(m.spinner.View())
	b.WriteString(scanningTextStyle.Render("Scanning for Dart/Flutter packages..."))

	return b.String()
}

func ShowScanningSpinner(count int) error {
	model := NewScanningModel()
	model.found = count

	p := tea.NewProgram(model)
	_, err := p.Run()

	return err
}
