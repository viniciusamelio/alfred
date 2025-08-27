package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/viniciusamelio/alfred/internal/git"
)

var (
	commitTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("62")).
				MarginBottom(1)

	repoHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			MarginTop(0).
			MarginBottom(0)

	fileItemStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			Foreground(lipgloss.Color("240")) // Cinza mais escuro

	selectedFileStyle = lipgloss.NewStyle().
				PaddingLeft(0).
				Foreground(lipgloss.Color("255")). // Branco para hover
				Bold(true)

	stagedFileStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			Foreground(lipgloss.Color("46"))

	commitMessageStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(1).
				MarginTop(1)

	helpCommitStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			MarginTop(1)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")).
			Bold(true)

	errorCommitStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true)
)

type CommitItem struct {
	FileChange git.FileChange
	Selected   bool
}

type CommitModel struct {
	repos          map[string][]*git.GitRepo // repo alias -> git repos
	items          []CommitItem
	cursor         int
	mode           int // 0: file selection, 1: commit message, 2: diff navigation
	messageInput   textarea.Model
	diffViewport   viewport.Model
	currentDiff    string
	width          int
	height         int
	finished       bool
	cancelled      bool
	error          string
	success        string
	commitMessage  string
	selectedFiles  map[string][]string // repo alias -> selected file paths
	showDiffPanel  bool                // whether to show diff panel alongside file list
	diffPanelWidth int                 // width of the diff panel
}

func NewCommitModel(repos map[string]*git.GitRepo) (*CommitModel, error) {
	// Get all file changes from all repositories
	var allItems []CommitItem
	repoMap := make(map[string][]*git.GitRepo)

	for alias, repo := range repos {
		changes, err := repo.GetFileChanges()
		if err != nil {
			continue // Skip repos with errors
		}

		// Set repo alias for each change
		for i := range changes {
			changes[i].RepoAlias = alias
		}

		// Convert to CommitItems
		for _, change := range changes {
			allItems = append(allItems, CommitItem{
				FileChange: change,
				Selected:   false,
			})
		}

		repoMap[alias] = []*git.GitRepo{repo}
	}

	// Sort items by repo alias, then by file path
	sort.Slice(allItems, func(i, j int) bool {
		if allItems[i].FileChange.RepoAlias != allItems[j].FileChange.RepoAlias {
			return allItems[i].FileChange.RepoAlias < allItems[j].FileChange.RepoAlias
		}
		return allItems[i].FileChange.Path < allItems[j].FileChange.Path
	})

	// Initialize textarea for commit message
	ta := textarea.New()
	ta.Placeholder = "Enter commit message..."
	ta.Focus()
	ta.CharLimit = 500
	ta.SetWidth(60)
	ta.SetHeight(5)

	// Initialize viewport for diff view
	vp := viewport.New(80, 20)

	model := &CommitModel{
		repos:          repoMap,
		items:          allItems,
		messageInput:   ta,
		diffViewport:   vp,
		selectedFiles:  make(map[string][]string),
		showDiffPanel:  true, // Show diff panel by default
		diffPanelWidth: 50,   // Default width percentage
	}

	// Load initial diff if there are items
	if len(allItems) > 0 {
		model.loadCurrentDiff()
	}

	return model, nil
}

func (m CommitModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m CommitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update component sizes based on layout
		if m.showDiffPanel && m.mode == 0 {
			// In file selection mode with diff panel
			totalWidth := max(80, m.width) // Ensure minimum width
			diffWidth := (totalWidth*m.diffPanelWidth)/100 - 4

			m.diffViewport.Width = max(25, diffWidth)
			m.diffViewport.Height = max(10, m.height-8)
		} else {
			// Normal layout
			m.messageInput.SetWidth(max(40, min(80, m.width-4)))
			m.diffViewport.Width = max(60, min(120, m.width-4))
			m.diffViewport.Height = max(15, min(30, m.height-10))
		}

		return m, nil

	case commitResultMsg:
		// Handle commit result
		m.finished = true

		if len(msg.successes) > 0 {
			m.success = strings.Join(msg.successes, "\n")
		}

		if len(msg.errors) > 0 {
			m.error = strings.Join(msg.errors, "\n")
		}

		return m, tea.Quit

	case tea.KeyMsg:
		switch m.mode {
		case 0: // File selection mode
			return m.updateFileSelection(msg)
		case 1: // Commit message mode
			return m.updateCommitMessage(msg)
		case 2: // Diff navigation mode
			return m.updateDiffNavigation(msg)
		}
	}

	return m, cmd
}

func (m CommitModel) updateFileSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.cancelled = true
		return m, tea.Quit

	case "up", "k":
		if len(m.items) > 0 && m.cursor > 0 {
			m.cursor--
			m.normalizeCursor()
			// Auto-load diff for new selection and reset viewport position
			if m.showDiffPanel {
				m.loadCurrentDiff()
				m.diffViewport.GotoTop() // Reset scroll position
			}
		}

	case "down", "j":
		if len(m.items) > 0 && m.cursor < len(m.items)-1 {
			m.cursor++
			m.normalizeCursor()
			// Auto-load diff for new selection and reset viewport position
			if m.showDiffPanel {
				m.loadCurrentDiff()
				m.diffViewport.GotoTop() // Reset scroll position
			}
		}

	case " ":
		// Toggle file selection
		if m.cursor < len(m.items) {
			m.items[m.cursor].Selected = !m.items[m.cursor].Selected
		}

	case "a":
		// Select all files
		for i := range m.items {
			m.items[i].Selected = true
		}

	case "n":
		// Deselect all files
		for i := range m.items {
			m.items[i].Selected = false
		}

	case "d":
		// Toggle diff panel
		m.showDiffPanel = !m.showDiffPanel
		if m.showDiffPanel {
			m.loadCurrentDiff()
		}

	case "v":
		// Enter diff navigation mode (view diff)
		if m.showDiffPanel && m.currentDiff != "" {
			m.mode = 2
		}

	case "enter", "c":
		// Proceed to commit message
		selectedCount := 0
		for _, item := range m.items {
			if item.Selected {
				selectedCount++
			}
		}

		if selectedCount == 0 {
			m.error = "Please select at least one file to commit"
			return m, nil
		}

		m.mode = 1
		m.error = ""
		return m, textarea.Blink
	}

	return m, nil
}

func (m CommitModel) updateCommitMessage(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "ctrl+c":
		m.cancelled = true
		return m, tea.Quit

	case "esc":
		// Go back to file selection
		m.mode = 0
		return m, nil

	case "ctrl+s", "ctrl+enter":
		// Commit changes
		message := strings.TrimSpace(m.messageInput.Value())
		if message == "" {
			m.error = "Commit message cannot be empty"
			return m, nil
		}

		return m, m.performCommit(message)
	}

	m.messageInput, cmd = m.messageInput.Update(msg)
	return m, cmd
}

func (m CommitModel) updateDiffNavigation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "ctrl+c", "q", "esc":
		// Go back to file selection
		m.mode = 0
		return m, nil

	case "up", "k":
		// Scroll up in diff
		m.diffViewport.ScrollUp(1)

	case "down", "j":
		// Scroll down in diff
		m.diffViewport.ScrollDown(1)

	case "pgup", "b":
		// Page up in diff
		m.diffViewport.PageUp()

	case "pgdown", "f":
		// Page down in diff
		m.diffViewport.PageDown()

	case "home", "g":
		// Go to top of diff
		m.diffViewport.GotoTop()

	case "end", "G":
		// Go to bottom of diff
		m.diffViewport.GotoBottom()

	case "left", "h":
		// Navigate to previous file
		if len(m.items) > 0 && m.cursor > 0 {
			m.cursor--
			m.normalizeCursor()
			m.loadCurrentDiff()
		}

	case "right", "l":
		// Navigate to next file
		if len(m.items) > 0 && m.cursor < len(m.items)-1 {
			m.cursor++
			m.normalizeCursor()
			m.loadCurrentDiff()
		}
	}

	return m, cmd
}

func (m CommitModel) performCommit(message string) tea.Cmd {
	return func() tea.Msg {
		// Group selected files by repository
		repoFiles := make(map[string][]string)

		for _, item := range m.items {
			if item.Selected {
				repoAlias := item.FileChange.RepoAlias
				repoFiles[repoAlias] = append(repoFiles[repoAlias], item.FileChange.Path)
			}
		}

		// Commit to each repository
		var errors []string
		var successes []string

		for repoAlias, files := range repoFiles {
			repo := m.repos[repoAlias][0]

			// Stage selected files
			for _, filePath := range files {
				if err := repo.StageFile(filePath); err != nil {
					errors = append(errors, fmt.Sprintf("%s: failed to stage %s: %v", repoAlias, filePath, err))
					continue
				}
			}

			// Check if there are staged changes
			hasStagedChanges, err := repo.HasStagedChanges()
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: failed to check staged changes: %v", repoAlias, err))
				continue
			}

			if !hasStagedChanges {
				continue // Skip if no staged changes
			}

			// Commit changes
			if err := repo.CommitChanges(message); err != nil {
				errors = append(errors, fmt.Sprintf("%s: failed to commit: %v", repoAlias, err))
			} else {
				successes = append(successes, fmt.Sprintf("%s: committed %d files", repoAlias, len(files)))
			}
		}

		return commitResultMsg{
			successes: successes,
			errors:    errors,
		}
	}
}

type commitResultMsg struct {
	successes []string
	errors    []string
}

func (m CommitModel) View() string {
	if m.cancelled {
		return "Commit cancelled.\n"
	}

	if m.finished {
		var b strings.Builder

		if m.success != "" {
			b.WriteString(successStyle.Render("✅ Commit successful!"))
			b.WriteString("\n\n")
			b.WriteString(m.success)
		}

		if m.error != "" {
			if m.success != "" {
				b.WriteString("\n\n")
			}
			b.WriteString(errorCommitStyle.Render("❌ Some errors occurred:"))
			b.WriteString("\n\n")
			b.WriteString(m.error)
		}

		return b.String()
	}

	switch m.mode {
	case 0:
		return m.viewFileSelection()
	case 1:
		return m.viewCommitMessage()
	case 2:
		return m.viewDiffNavigation()
	}

	return ""
}

func (m CommitModel) viewFileSelection() string {
	// Ensure cursor is normalized before rendering
	m.normalizeCursor()

	if !m.showDiffPanel {
		return m.viewFileSelectionOnly()
	}

	// Calculate layout dimensions with better spacing
	totalWidth := max(80, m.width) // Ensure minimum width
	fileListWidth := (totalWidth * (100 - m.diffPanelWidth)) / 100
	diffWidth := (totalWidth * m.diffPanelWidth) / 100

	// Ensure minimum widths
	fileListWidth = max(30, fileListWidth)
	diffWidth = max(30, diffWidth)

	// Build file list
	fileListContent := m.buildFileList(fileListWidth - 2)

	// Build diff panel
	diffContent := m.buildDiffPanel(diffWidth - 2)

	// Combine side by side
	fileLines := strings.Split(fileListContent, "\n")
	diffLines := strings.Split(diffContent, "\n")

	maxLines := max(len(fileLines), len(diffLines))

	var result strings.Builder
	for i := range maxLines {
		var fileLine, diffLine string

		if i < len(fileLines) {
			fileLine = fileLines[i]
		}
		if i < len(diffLines) {
			diffLine = diffLines[i]
		}

		// Calculate actual display width (accounting for ANSI codes)
		fileDisplayWidth := lipgloss.Width(fileLine)
		targetWidth := fileListWidth - 2

		// Pad or truncate file line to exact width
		if fileDisplayWidth < targetWidth {
			// Add padding to reach target width
			padding := targetWidth - fileDisplayWidth
			fileLine += strings.Repeat(" ", padding)
		} else if fileDisplayWidth > targetWidth {
			// Truncate if too long, but preserve styling
			// This is a simple truncation - for better results we'd need to handle ANSI codes
			if len(fileLine) > targetWidth {
				fileLine = fileLine[:max(0, targetWidth-3)] + "..."
			}
		}

		result.WriteString(fileLine)
		result.WriteString(" │ ")
		result.WriteString(diffLine)
		result.WriteString("\n")
	}

	return result.String()
}

func (m CommitModel) viewFileSelectionOnly() string {
	var b strings.Builder

	b.WriteString(commitTitleStyle.Render("Git Commit - Select Files"))
	b.WriteString("\n\n")

	if len(m.items) == 0 {
		b.WriteString("No changes found in any repository.\n")
		return b.String()
	}

	b.WriteString(m.buildFileList(m.width - 4))

	if m.error != "" {
		b.WriteString(errorCommitStyle.Render(m.error))
		b.WriteString("\n\n")
	}

	b.WriteString(helpCommitStyle.Render("↑/↓ navigate • Space select • A select all • N deselect all • D toggle diff • V view diff • Enter/C commit • Q quit"))

	return b.String()
}

func (m CommitModel) buildFileList(maxWidth int) string {
	var b strings.Builder

	if !m.showDiffPanel {
		b.WriteString(commitTitleStyle.Render("Git Commit - Select Files"))
		b.WriteString("\n\n")
	}

	if len(m.items) == 0 {
		b.WriteString("No changes found in any repository.\n")
		return b.String()
	}

	// Group items by repository
	repoGroups := make(map[string][]CommitItem)
	for _, item := range m.items {
		repoGroups[item.FileChange.RepoAlias] = append(repoGroups[item.FileChange.RepoAlias], item)
	}

	// Display files grouped by repository
	// We need to maintain the original order to match cursor position
	itemIndex := 0
	for _, item := range m.items {
		// Check if this is the first item of a new repo group
		if itemIndex == 0 || m.items[itemIndex-1].FileChange.RepoAlias != item.FileChange.RepoAlias {
			header := fmt.Sprintf("📁 %s", item.FileChange.RepoAlias)
			if maxWidth > 0 && len(header) > maxWidth {
				header = header[:maxWidth]
			}
			b.WriteString(repoHeaderStyle.Render(header))
			b.WriteString("\n")
		}

		cursor := "  "
		if itemIndex == m.cursor {
			cursor = ">"
		}

		checkbox := "☐"
		style := fileItemStyle
		if item.Selected {
			checkbox = "☑"
			style = stagedFileStyle
		}

		statusDesc := git.GetStatusDescription(item.FileChange.Status)

		line := fmt.Sprintf("%s %s [%s] %s",
			cursor, checkbox, statusDesc, item.FileChange.Path)

		// Truncate line if too long, but be smarter about it
		if maxWidth > 10 && len(line) > maxWidth {
			// Try to preserve the important parts: cursor, checkbox, status, and part of path
			prefixLen := len(fmt.Sprintf("%s %s [%s] ", cursor, checkbox, statusDesc))
			if prefixLen < maxWidth-3 {
				availableForPath := maxWidth - prefixLen - 3
				if availableForPath > 0 {
					truncatedPath := item.FileChange.Path
					if len(truncatedPath) > availableForPath {
						truncatedPath = "..." + truncatedPath[len(truncatedPath)-(availableForPath-3):]
					}
					line = fmt.Sprintf("%s %s [%s] %s",
						cursor, checkbox, statusDesc, truncatedPath)
				} else {
					line = line[:maxWidth-3] + "..."
				}
			} else {
				line = line[:maxWidth-3] + "..."
			}
		}

		if itemIndex == m.cursor {
			line = selectedFileStyle.Render(line)
		} else {
			line = style.Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
		itemIndex++

		// Add minimal spacing after each repo group (except the last one)
		if itemIndex < len(m.items) && m.items[itemIndex].FileChange.RepoAlias != item.FileChange.RepoAlias {
			// Removed extra newline to reduce spacing between groups
		}
	}

	if !m.showDiffPanel {
		if m.error != "" {
			b.WriteString(errorCommitStyle.Render(m.error))
			b.WriteString("\n\n")
		}

		b.WriteString(helpCommitStyle.Render("↑/↓ navigate • Space select • A select all • N deselect all • D toggle diff • V view diff • Enter/C commit • Q quit"))
	}

	return b.String()
}

func (m CommitModel) buildDiffPanel(maxWidth int) string {
	var b strings.Builder

	// Ensure minimum width for diff panel
	effectiveWidth := max(25, maxWidth)

	// Header for diff panel
	if len(m.items) > 0 && m.cursor >= 0 && m.cursor < len(m.items) {
		item := m.items[m.cursor]
		header := fmt.Sprintf("📄 %s/%s", item.FileChange.RepoAlias, item.FileChange.Path)
		if len(header) > effectiveWidth-3 {
			// Smart truncation: keep the filename visible
			parts := strings.Split(item.FileChange.Path, "/")
			filename := parts[len(parts)-1]
			if len(filename)+10 < effectiveWidth { // 10 for "📄 .../"
				header = fmt.Sprintf("📄 .../%s", filename)
			} else {
				header = header[:effectiveWidth-3] + "..."
			}
		}
		b.WriteString(repoHeaderStyle.Render(header))
		b.WriteString("\n")

		// Status info
		statusInfo := fmt.Sprintf("[%s] %s",
			git.GetStatusDescription(item.FileChange.Status),
			func() string {
				if item.FileChange.Staged {
					return "Staged"
				}
				return "Unstaged"
			}())
		b.WriteString(helpCommitStyle.Render(statusInfo))
		b.WriteString("\n\n")
	} else {
		b.WriteString(repoHeaderStyle.Render("📄 No file selected"))
		b.WriteString("\n\n")
	}

	// Diff content
	if m.currentDiff != "" {
		// Create a viewport-like display for the diff
		diffLines := strings.Split(m.currentDiff, "\n")
		maxLines := max(10, m.height-8) // Reserve space for headers and help

		for i, line := range diffLines {
			if i >= maxLines {
				b.WriteString(helpCommitStyle.Render("... (truncated)"))
				break
			}

			// Smart truncation for diff lines
			if len(line) > effectiveWidth {
				if effectiveWidth > 10 {
					line = line[:effectiveWidth-3] + "..."
				} else {
					line = line[:effectiveWidth]
				}
			}

			// Color diff lines
			if strings.HasPrefix(line, "+") {
				b.WriteString(stagedFileStyle.Render(line))
			} else if strings.HasPrefix(line, "-") {
				b.WriteString(errorCommitStyle.Render(line))
			} else if strings.HasPrefix(line, "@@") {
				b.WriteString(repoHeaderStyle.Render(line))
			} else {
				b.WriteString(line)
			}
			b.WriteString("\n")
		}
	} else {
		b.WriteString(helpCommitStyle.Render("No diff available"))
		b.WriteString("\n")
	}

	return b.String()
}

func (m CommitModel) viewCommitMessage() string {
	var b strings.Builder

	b.WriteString(commitTitleStyle.Render("Git Commit - Enter Message"))
	b.WriteString("\n\n")

	// Show selected files summary
	selectedCount := 0
	repoCount := make(map[string]int)
	for _, item := range m.items {
		if item.Selected {
			selectedCount++
			repoCount[item.FileChange.RepoAlias]++
		}
	}

	b.WriteString(fmt.Sprintf("Selected %d files across %d repositories:\n", selectedCount, len(repoCount)))
	for repo, count := range repoCount {
		b.WriteString(fmt.Sprintf("  • %s: %d files\n", repo, count))
	}
	b.WriteString("\n")

	b.WriteString(commitMessageStyle.Render(m.messageInput.View()))
	b.WriteString("\n")

	if m.error != "" {
		b.WriteString(errorCommitStyle.Render(m.error))
		b.WriteString("\n")
	}

	b.WriteString(helpCommitStyle.Render("Ctrl+S or Ctrl+Enter to commit • Esc to go back • Ctrl+C to cancel"))

	return b.String()
}

func (m CommitModel) viewDiffNavigation() string {
	var b strings.Builder

	// Title
	if len(m.items) > 0 && m.cursor >= 0 && m.cursor < len(m.items) {
		item := m.items[m.cursor]
		title := fmt.Sprintf("📄 Viewing: %s/%s", item.FileChange.RepoAlias, item.FileChange.Path)
		b.WriteString(commitTitleStyle.Render(title))
		b.WriteString("\n")

		// Status info
		statusInfo := fmt.Sprintf("[%s] %s",
			git.GetStatusDescription(item.FileChange.Status),
			func() string {
				if item.FileChange.Staged {
					return "Staged"
				}
				return "Unstaged"
			}())
		b.WriteString(helpCommitStyle.Render(statusInfo))
		b.WriteString("\n\n")
	} else {
		b.WriteString(commitTitleStyle.Render("📄 Diff Viewer"))
		b.WriteString("\n\n")
	}

	// Diff content using viewport
	if m.currentDiff != "" {
		// Update viewport content and size
		m.diffViewport.SetContent(m.currentDiff)
		m.diffViewport.Width = m.width - 4
		m.diffViewport.Height = m.height - 8

		// Render the viewport
		diffContent := m.diffViewport.View()

		// Apply styling to diff lines
		lines := strings.Split(diffContent, "\n")
		var styledLines []string

		for _, line := range lines {
			if strings.HasPrefix(line, "+") {
				styledLines = append(styledLines, stagedFileStyle.Render(line))
			} else if strings.HasPrefix(line, "-") {
				styledLines = append(styledLines, errorCommitStyle.Render(line))
			} else if strings.HasPrefix(line, "@@") {
				styledLines = append(styledLines, repoHeaderStyle.Render(line))
			} else {
				styledLines = append(styledLines, line)
			}
		}

		b.WriteString(strings.Join(styledLines, "\n"))
	} else {
		b.WriteString(helpCommitStyle.Render("No diff available"))
	}

	b.WriteString("\n\n")

	// Help text
	help := "↑/↓ or j/k scroll • PgUp/PgDn page • Home/End or g/G top/bottom • ←/→ or h/l prev/next file • Esc/Q back"
	b.WriteString(helpCommitStyle.Render(help))

	return b.String()
}

// normalizeCursor ensures the cursor is within valid bounds
func (m *CommitModel) normalizeCursor() {
	if len(m.items) == 0 {
		m.cursor = 0
		return
	}

	if m.cursor < 0 {
		m.cursor = 0
	} else if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
}

// loadCurrentDiff loads the diff for the currently selected file
func (m *CommitModel) loadCurrentDiff() {
	// Normalize cursor first
	m.normalizeCursor()
	if m.cursor >= len(m.items) || m.cursor < 0 {
		m.currentDiff = ""
		return
	}

	item := m.items[m.cursor]

	// Safety check for repo existence
	repos, exists := m.repos[item.FileChange.RepoAlias]
	if !exists || len(repos) == 0 {
		m.currentDiff = fmt.Sprintf("Error: repository %s not found", item.FileChange.RepoAlias)
		return
	}

	repo := repos[0]

	// For new files (untracked), show the complete content directly
	if item.FileChange.Status == "??" {
		content, err := repo.GetFileContent(item.FileChange.Path)
		if err != nil {
			m.currentDiff = fmt.Sprintf("Error loading file content: %v", err)
		} else {
			// Format as if it's all new content (with + prefix)
			lines := strings.Split(content, "\n")
			var formattedLines []string
			formattedLines = append(formattedLines, fmt.Sprintf("+++ %s", item.FileChange.Path))
			formattedLines = append(formattedLines, "@@ -0,0 +1,"+fmt.Sprintf("%d", len(lines))+" @@")
			for _, line := range lines {
				formattedLines = append(formattedLines, "+"+line)
			}
			m.currentDiff = strings.Join(formattedLines, "\n")
		}
	} else {
		// For tracked files, get the diff
		diff, err := repo.GetFileDiff(item.FileChange.Path, item.FileChange.Staged)
		if err != nil {
			m.currentDiff = fmt.Sprintf("Error loading diff: %v", err)
		} else {
			if diff == "" {
				// For added files that are staged but have no diff
				if item.FileChange.Status == "A" {
					content, err := repo.GetFileContent(item.FileChange.Path)
					if err != nil {
						m.currentDiff = fmt.Sprintf("Error loading file content: %v", err)
					} else {
						// Format as if it's all new content (with + prefix)
						lines := strings.Split(content, "\n")
						var formattedLines []string
						formattedLines = append(formattedLines, fmt.Sprintf("+++ %s", item.FileChange.Path))
						formattedLines = append(formattedLines, "@@ -0,0 +1,"+fmt.Sprintf("%d", len(lines))+" @@")
						for _, line := range lines {
							formattedLines = append(formattedLines, "+"+line)
						}
						m.currentDiff = strings.Join(formattedLines, "\n")
					}
				} else {
					m.currentDiff = "No changes to display"
				}
			} else {
				m.currentDiff = diff
			}
		}
	}
	m.diffViewport.SetContent(m.currentDiff)
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Helper function for max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func RunCommitInterface(repos map[string]*git.GitRepo) error {
	m, err := NewCommitModel(repos)
	if err != nil {
		return fmt.Errorf("failed to create commit model: %w", err)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running commit interface: %w", err)
	}

	// Check if commit was successful
	if model, ok := finalModel.(*CommitModel); ok {
		if model.cancelled {
			return fmt.Errorf("commit cancelled")
		}
		return nil
	}

	if model, ok := finalModel.(CommitModel); ok {
		if model.cancelled {
			return fmt.Errorf("commit cancelled")
		}
		return nil
	}

	return nil
}
