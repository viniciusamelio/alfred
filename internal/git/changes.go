package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FileChange represents a changed file in a git repository
type FileChange struct {
	Path      string // Relative path from repo root
	Status    string // Git status (M, A, D, ??, etc.)
	Staged    bool   // Whether the file is staged
	RepoPath  string // Path to the repository
	RepoAlias string // Repository alias/name
}

// GetFileChanges returns all changed files in the repository
func (g *GitRepo) GetFileChanges() ([]FileChange, error) {
	// Get both staged and unstaged changes
	cmd := exec.Command("git", "-C", g.Path, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git status: %w", err)
	}

	var changes []FileChange
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		if len(line) < 3 {
			continue
		}

		// Git status format: XY filename
		// X = staged status, Y = unstaged status
		stagedStatus := string(line[0])
		unstagedStatus := string(line[1])
		filePath := strings.TrimSpace(line[2:])

		// Handle renamed files (format: "R  old -> new")
		if strings.Contains(filePath, " -> ") {
			parts := strings.Split(filePath, " -> ")
			if len(parts) == 2 {
				filePath = parts[1]
			}
		}

		// Determine overall status and if it's staged
		var status string
		var staged bool

		// Handle untracked files (status ??)
		if stagedStatus == "?" && unstagedStatus == "?" {
			status = "??"
			staged = false
		} else if stagedStatus != " " && stagedStatus != "?" {
			// File has staged changes
			status = stagedStatus
			staged = true
		} else if unstagedStatus != " " {
			// File has unstaged changes
			status = unstagedStatus
			staged = false
		} else {
			continue
		}

		changes = append(changes, FileChange{
			Path:     filePath,
			Status:   status,
			Staged:   staged,
			RepoPath: g.Path,
		})
	}

	return changes, nil
}

// GetFileDiff returns the diff for a specific file
func (g *GitRepo) GetFileDiff(filePath string, staged bool) (string, error) {
	var cmd *exec.Cmd

	if staged {
		// Show diff for staged changes
		cmd = exec.Command("git", "-C", g.Path, "diff", "--cached", "--", filePath)
	} else {
		// Show diff for unstaged changes
		cmd = exec.Command("git", "-C", g.Path, "diff", "--", filePath)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %w", err)
	}

	return string(output), nil
}

// StageFile stages a specific file
func (g *GitRepo) StageFile(filePath string) error {
	cmd := exec.Command("git", "-C", g.Path, "add", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stage file: %w", err)
	}
	return nil
}

// UnstageFile unstages a specific file
func (g *GitRepo) UnstageFile(filePath string) error {
	cmd := exec.Command("git", "-C", g.Path, "reset", "HEAD", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to unstage file: %w", err)
	}
	return nil
}

// CommitChanges commits staged changes with the given message
func (g *GitRepo) CommitChanges(message string) error {
	if message == "" {
		return fmt.Errorf("commit message cannot be empty")
	}

	cmd := exec.Command("git", "-C", g.Path, "commit", "-m", message)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}
	return nil
}

// HasStagedChanges checks if there are any staged changes
func (g *GitRepo) HasStagedChanges() (bool, error) {
	cmd := exec.Command("git", "-C", g.Path, "diff", "--cached", "--quiet")
	err := cmd.Run()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 means there are differences (staged changes)
			if exitErr.ExitCode() == 1 {
				return true, nil
			}
		}
		return false, fmt.Errorf("failed to check staged changes: %w", err)
	}

	// Exit code 0 means no differences (no staged changes)
	return false, nil
}

// GetFileContent returns the complete content of a file
func (g *GitRepo) GetFileContent(filePath string) (string, error) {
	fullPath := filepath.Join(g.Path, filePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return string(content), nil
}

// GetStatusDescription returns a human-readable description of the file status
func GetStatusDescription(status string) string {
	switch status {
	case "M":
		return "Modified"
	case "A":
		return "Added"
	case "D":
		return "Deleted"
	case "R":
		return "Renamed"
	case "C":
		return "Copied"
	case "U":
		return "Unmerged"
	case "??":
		return "New"
	default:
		return "New"
	}
}

// GetStatusColor returns a color code for the file status
func GetStatusColor(status string) string {
	switch status {
	case "M":
		return "214" // Orange for modified
	case "A":
		return "46" // Green for added
	case "D":
		return "196" // Red for deleted
	case "R":
		return "39" // Blue for renamed
	case "C":
		return "39" // Blue for copied
	case "U":
		return "208" // Orange-red for unmerged
	case "??":
		return "243" // Gray for untracked
	default:
		return "255" // White for unknown
	}
}
