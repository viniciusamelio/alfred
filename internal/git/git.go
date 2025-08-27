package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type GitRepo struct {
	Path string
}

func NewGitRepo(path string) *GitRepo {
	return &GitRepo{Path: path}
}

func (g *GitRepo) IsGitRepo() bool {
	gitPath := filepath.Join(g.Path, ".git")

	// Check if .git exists (could be directory or file)
	if _, err := os.Stat(gitPath); err != nil {
		return false
	}

	// Try running a simple git command to verify it's a valid repo
	cmd := exec.Command("git", "-C", g.Path, "rev-parse", "--git-dir")
	err := cmd.Run()
	return err == nil
}

func (g *GitRepo) GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "-C", g.Path, "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (g *GitRepo) HasUncommittedChanges() (bool, error) {
	cmd := exec.Command("git", "-C", g.Path, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

func (g *GitRepo) StashChanges(message string) error {
	cmd := exec.Command("git", "-C", g.Path, "stash", "push", "-m", message)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stash changes: %w", err)
	}
	return nil
}

func (g *GitRepo) PopStash(stashName string) error {
	stashes, err := g.ListStashes()
	if err != nil {
		return err
	}

	for i, stash := range stashes {
		if strings.Contains(stash, stashName) {
			cmd := exec.Command("git", "-C", g.Path, "stash", "pop", fmt.Sprintf("stash@{%d}", i))
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to pop stash: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("stash with name '%s' not found", stashName)
}

func (g *GitRepo) ListStashes() ([]string, error) {
	cmd := exec.Command("git", "-C", g.Path, "stash", "list")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list stashes: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return []string{}, nil
	}

	return lines, nil
}

func (g *GitRepo) BranchExists(branchName string) (bool, error) {
	cmd := exec.Command("git", "-C", g.Path, "show-ref", "--verify", "--quiet", fmt.Sprintf("refs/heads/%s", branchName))
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return false, nil
			}
		}
		return false, fmt.Errorf("failed to check if branch exists: %w", err)
	}
	return true, nil
}

func (g *GitRepo) CreateBranch(branchName string, fromBranch string) error {
	if fromBranch == "" {
		fromBranch = "HEAD"
	}

	cmd := exec.Command("git", "-C", g.Path, "checkout", "-b", branchName, fromBranch)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}
	return nil
}

func (g *GitRepo) CheckoutBranch(branchName string) error {
	cmd := exec.Command("git", "-C", g.Path, "checkout", branchName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout branch: %w", err)
	}
	return nil
}

func (g *GitRepo) CreateWorktree(path, branchName string) error {
	// First check if branch exists
	branchExists, err := g.BranchExists(branchName)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}

	// Convert to absolute path to avoid issues with git -C
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	var cmd *exec.Cmd
	if branchExists {
		// Branch exists, create worktree and checkout existing branch
		cmd = exec.Command("git", "-C", g.Path, "worktree", "add", absPath, branchName)
	} else {
		// Branch doesn't exist, create worktree with new branch
		cmd = exec.Command("git", "-C", g.Path, "worktree", "add", "-b", branchName, absPath)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}
	return nil
}

func (g *GitRepo) RemoveWorktree(path string) error {
	// Convert to absolute path to match git worktree expectations
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if worktree directory exists
	if _, err := os.Stat(absPath); err == nil {
		cmd := exec.Command("git", "-C", g.Path, "worktree", "remove", "--force", absPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to remove worktree: %w, output: %s", err, string(output))
		}
	}
	return nil
}

func (g *GitRepo) ListWorktrees() ([]string, error) {
	cmd := exec.Command("git", "-C", g.Path, "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var worktrees []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			worktreePath := strings.TrimPrefix(line, "worktree ")
			worktrees = append(worktrees, worktreePath)
		}
	}

	return worktrees, nil
}

func (g *GitRepo) WorktreeExists(path string) (bool, error) {
	worktrees, err := g.ListWorktrees()
	if err != nil {
		return false, err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, fmt.Errorf("failed to get absolute path: %w", err)
	}

	for _, worktree := range worktrees {
		worktreeAbs, err := filepath.Abs(worktree)
		if err != nil {
			continue
		}
		if worktreeAbs == absPath {
			return true, nil
		}
	}
	return false, nil
}

func (g *GitRepo) GetWorktreeForContext(contextName string) string {
	// Generate worktree path: <repo-dir>-<context>
	repoDir := filepath.Base(g.Path)
	return filepath.Join(filepath.Dir(g.Path), fmt.Sprintf("%s-%s", repoDir, contextName))
}

// HasStashForContext checks if there's a stash with the given context name
func (g *GitRepo) HasStashForContext(contextName string) (bool, error) {
	stashes, err := g.ListStashes()
	if err != nil {
		return false, err
	}

	stashMessage := fmt.Sprintf("alfred-context-%s", contextName)
	for _, stash := range stashes {
		if strings.Contains(stash, stashMessage) {
			return true, nil
		}
	}

	return false, nil
}

// StashForContext creates a stash with context-specific message
func (g *GitRepo) StashForContext(contextName string) error {
	message := fmt.Sprintf("alfred-context-%s", contextName)
	return g.StashChanges(message)
}

// PopStashForContext pops the stash for a specific context
func (g *GitRepo) PopStashForContext(contextName string) error {
	stashMessage := fmt.Sprintf("alfred-context-%s", contextName)
	return g.PopStash(stashMessage)
}

// HasUpstream checks if the current branch has an upstream configured
func (g *GitRepo) HasUpstream() (bool, error) {
	cmd := exec.Command("git", "-C", g.Path, "rev-parse", "--abbrev-ref", "@{upstream}")
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 128 typically means no upstream is set
			if exitErr.ExitCode() == 128 {
				return false, nil
			}
		}
		return false, fmt.Errorf("failed to check upstream: %w", err)
	}
	return true, nil
}

// SetUpstream sets the upstream for the current branch
func (g *GitRepo) SetUpstream(remote, branch string) error {
	if remote == "" {
		remote = "origin"
	}

	if branch == "" {
		// Get current branch name
		currentBranch, err := g.GetCurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		branch = currentBranch
	}

	cmd := exec.Command("git", "-C", g.Path, "branch", "--set-upstream-to", fmt.Sprintf("%s/%s", remote, branch))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set upstream: %w", err)
	}
	return nil
}

// PushWithUpstream pushes and sets upstream if not configured
func (g *GitRepo) PushWithUpstream(remote string) error {
	if remote == "" {
		remote = "origin"
	}

	// Get current branch name
	currentBranch, err := g.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Check if upstream is configured
	hasUpstream, err := g.HasUpstream()
	if err != nil {
		return fmt.Errorf("failed to check upstream: %w", err)
	}

	var cmd *exec.Cmd
	if !hasUpstream {
		// Push and set upstream
		cmd = exec.Command("git", "-C", g.Path, "push", "--set-upstream", remote, currentBranch)
	} else {
		// Just push
		cmd = exec.Command("git", "-C", g.Path, "push")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Provide more detailed error information
		outputStr := strings.TrimSpace(string(output))
		if outputStr != "" {
			return fmt.Errorf("failed to push: %s", outputStr)
		}
		return fmt.Errorf("failed to push: %w", err)
	}
	return nil
}

// Pull pulls from upstream, setting it up if needed
func (g *GitRepo) Pull(rebase bool) error {
	// Check if upstream is configured
	hasUpstream, err := g.HasUpstream()
	if err != nil {
		return fmt.Errorf("failed to check upstream: %w", err)
	}

	if !hasUpstream {
		// Try to set upstream automatically
		currentBranch, err := g.GetCurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}

		// Check if remote branch exists before setting upstream
		checkCmd := exec.Command("git", "-C", g.Path, "ls-remote", "--heads", "origin", currentBranch)
		checkOutput, checkErr := checkCmd.Output()
		if checkErr != nil || len(strings.TrimSpace(string(checkOutput))) == 0 {
			return fmt.Errorf("remote branch 'origin/%s' does not exist. Push the branch first with 'alfred push'", currentBranch)
		}

		// Try to set upstream to origin/<current-branch>
		if err := g.SetUpstream("origin", currentBranch); err != nil {
			return fmt.Errorf("no upstream configured and failed to set upstream: %w", err)
		}
	}

	var cmd *exec.Cmd
	if rebase {
		cmd = exec.Command("git", "-C", g.Path, "pull", "--rebase")
	} else {
		cmd = exec.Command("git", "-C", g.Path, "pull")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Provide more detailed error information
		outputStr := strings.TrimSpace(string(output))
		if outputStr != "" {
			return fmt.Errorf("failed to pull: %s", outputStr)
		}
		return fmt.Errorf("failed to pull: %w", err)
	}
	return nil
}
