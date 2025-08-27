package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	scannerTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("62")).
				MarginBottom(1).
				Padding(0, 1)

	scannerSubtitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				MarginBottom(2)

	packageItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("252"))

	selectedPackageStyle = lipgloss.NewStyle().
				PaddingLeft(0).
				Foreground(lipgloss.Color("170")).
				Bold(true)

	masterLabelStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("230")).
				Padding(0, 1).
				Bold(true)

	scannerHelpStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				MarginTop(2)

	scannerSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).
				Bold(true)

	// packageCountStyle = lipgloss.NewStyle().
	// 			Foreground(lipgloss.Color("39")).
	// 			Bold(true)
)

type PackageInfo struct {
	Name string
	Path string
}

type ScannerModel struct {
	packages    []PackageInfo
	cursor      int
	finished    bool
	cancelled   bool
	selectedIdx int
	title       string
	subtitle    string
}

func NewScanner(packages []PackageInfo) *ScannerModel {
	return &ScannerModel{
		packages:    packages,
		cursor:      0,
		selectedIdx: -1,
		title:       "🔍 Repository Scanner",
		subtitle:    fmt.Sprintf("Found %d Dart/Flutter packages", len(packages)),
	}
}

func (m ScannerModel) Init() tea.Cmd {
	return nil
}

func (m ScannerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "enter":
			if len(m.packages) > 0 {
				m.selectedIdx = m.cursor
				m.finished = true
				return m, tea.Quit
			}

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.packages)-1 {
				m.cursor++
			}
		}
	}

	return m, nil
}

func (m ScannerModel) View() string {
	if m.cancelled {
		return scannerSuccessStyle.Render("Operation cancelled.\n")
	}

	if m.finished {
		selected := m.packages[m.selectedIdx]
		return fmt.Sprintf("%s\n\n%s %s\n",
			scannerSuccessStyle.Render("✅ Master repository selected!"),
			masterLabelStyle.Render("MASTER"),
			scannerSuccessStyle.Render(fmt.Sprintf("%s (%s)", selected.Name, selected.Path)))
	}

	var b strings.Builder

	// Title
	b.WriteString(scannerTitleStyle.Render(m.title))
	b.WriteString("\n")
	b.WriteString(scannerSubtitleStyle.Render(m.subtitle))
	b.WriteString("\n")

	if len(m.packages) == 0 {
		b.WriteString(packageItemStyle.Render("No Dart/Flutter packages found in current directory."))
		b.WriteString("\n")
		b.WriteString(scannerHelpStyle.Render("Press Esc to cancel"))
		return b.String()
	}

	b.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true).
		Render("Select the master repository (main app/entry point):"))
	b.WriteString("\n\n")

	// Package list
	for i, pkg := range m.packages {
		cursor := " "
		if m.cursor == i {
			cursor = "❯"
		}

		// Package icon
		icon := "📦"
		if strings.Contains(strings.ToLower(pkg.Name), "app") {
			icon = "📱"
		} else if strings.Contains(strings.ToLower(pkg.Name), "ui") {
			icon = "🎨"
		} else if strings.Contains(strings.ToLower(pkg.Name), "core") {
			icon = "⚙️"
		}

		line := fmt.Sprintf("%s %s %s", cursor, icon, pkg.Name)

		// Path info
		pathInfo := lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Render(fmt.Sprintf("(%s)", pkg.Path))

		line += " " + pathInfo

		if m.cursor == i {
			line = selectedPackageStyle.Render(line)
		} else {
			line = packageItemStyle.Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	// Help text
	b.WriteString("\n")
	b.WriteString(scannerHelpStyle.Render("↑/↓ navigate • Enter select • Esc cancel"))

	return b.String()
}

func (m ScannerModel) GetResult() (string, bool) {
	if m.cancelled || !m.finished || m.selectedIdx < 0 {
		return "", false
	}
	return m.packages[m.selectedIdx].Name, true
}

func RunPackageSelector(packages []PackageInfo) (string, error) {
	if len(packages) == 0 {
		return "", fmt.Errorf("no packages found")
	}

	// If only one package, auto-select it with a nice message
	if len(packages) == 1 {
		pkg := packages[0]
		icon := "📦"
		if strings.Contains(strings.ToLower(pkg.Name), "app") {
			icon = "📱"
		} else if strings.Contains(strings.ToLower(pkg.Name), "ui") {
			icon = "🎨"
		} else if strings.Contains(strings.ToLower(pkg.Name), "core") {
			icon = "⚙️"
		}

		fmt.Printf("%s %s %s\n",
			scannerTitleStyle.Render("🔍 Repository Scanner"),
			scannerSubtitleStyle.Render("Found 1 Dart/Flutter package"),
			"")
		fmt.Printf("\n%s %s %s %s\n\n",
			masterLabelStyle.Render("MASTER"),
			icon,
			scannerSuccessStyle.Render(pkg.Name),
			lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(fmt.Sprintf("(%s)", pkg.Path)))

		return pkg.Name, nil
	}

	m := NewScanner(packages)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running package selector: %w", err)
	}

	// Try both pointer and value types
	if model, ok := finalModel.(*ScannerModel); ok {
		result, success := model.GetResult()
		if !success {
			return "", fmt.Errorf("package selection cancelled")
		}
		return result, nil
	}

	if model, ok := finalModel.(ScannerModel); ok {
		result, success := model.GetResult()
		if !success {
			return "", fmt.Errorf("package selection cancelled")
		}
		return result, nil
	}

	return "", fmt.Errorf("unexpected model type: %T", finalModel)
}
