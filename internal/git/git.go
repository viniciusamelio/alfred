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