package worktree

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/viniciusamelio/alfred/internal/config"
	"github.com/viniciusamelio/alfred/internal/git"
)

type Manager struct {
	config *config.Config
	logger *log.Logger
}

type WorktreeInfo struct {
	Repo         *config.Repository
	WorktreePath string
	BranchName   string
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		config: cfg,
		logger: log.Default(),
	}
}

func (w *Manager) GetWorktreePath(repo *config.Repository, contextName string) string {
	// Generate path: <repo-path>-<context>
	return fmt.Sprintf("%s-%s", repo.Path, contextName)
}

func (w *Manager) CreateWorktreeForContext(repo *config.Repository, contextName string) (*WorktreeInfo, error) {
	gitRepo := git.NewGitRepo(repo.Path)
	
	if !gitRepo.IsGitRepo() {
		return nil, fmt.Errorf("repository %s is not a git repository", repo.Alias)
	}

	worktreePath := w.GetWorktreePath(repo, contextName)
	
	// Check if worktree already exists
	worktreeExists, err := gitRepo.WorktreeExists(worktreePath)
	if err != nil {
		return nil, fmt.Errorf("failed to check worktree existence: %w", err)
	}

	if worktreeExists {
		w.logger.Infof("Worktree %s already exists for %s", worktreePath, repo.Alias)
	} else {
		w.logger.Infof("Creating worktree %s for %s with branch %s", worktreePath, repo.Alias, contextName)
		if err := gitRepo.CreateWorktree(worktreePath, contextName); err != nil {
			return nil, fmt.Errorf("failed to create worktree: %w", err)
		}
	}

	return &WorktreeInfo{
		Repo:         repo,
		WorktreePath: worktreePath,
		BranchName:   contextName,
	}, nil
}

func (w *Manager) RemoveWorktreeForContext(repo *config.Repository, contextName string) error {
	gitRepo := git.NewGitRepo(repo.Path)
	worktreePath := w.GetWorktreePath(repo, contextName)
	
	worktreeExists, err := gitRepo.WorktreeExists(worktreePath)
	if err != nil {
		return fmt.Errorf("failed to check worktree existence: %w", err)
	}

	if worktreeExists {
		w.logger.Infof("Removing worktree %s for %s", worktreePath, repo.Alias)
		if err := gitRepo.RemoveWorktree(worktreePath); err != nil {
			return fmt.Errorf("failed to remove worktree: %w", err)
		}
	}

	return nil
}

func (w *Manager) ListWorktreesForContext(repos []*config.Repository, contextName string) ([]*WorktreeInfo, error) {
	var worktrees []*WorktreeInfo
	
	for _, repo := range repos {
		worktreePath := w.GetWorktreePath(repo, contextName)
		
		// Check if worktree exists
		if _, err := os.Stat(worktreePath); err == nil {
			worktrees = append(worktrees, &WorktreeInfo{
				Repo:         repo,
				WorktreePath: worktreePath,
				BranchName:   contextName,
			})
		}
	}
	
	return worktrees, nil
}

func (w *Manager) HandleStashForWorktree(worktree *WorktreeInfo, contextName string, operation string) error {
	// Create a git repo instance for the worktree
	worktreeGitRepo := git.NewGitRepo(worktree.WorktreePath)
	stashMessage := fmt.Sprintf("alfred-context-%s", contextName)

	switch operation {
	case "push":
		hasChanges, err := worktreeGitRepo.HasUncommittedChanges()
		if err != nil {
			return fmt.Errorf("failed to check changes: %w", err)
		}

		if hasChanges {
			if err := worktreeGitRepo.StashChanges(stashMessage); err != nil {
				return fmt.Errorf("failed to stash changes: %w", err)
			}
			w.logger.Infof("Stashed changes in %s worktree", worktree.Repo.Alias)
		}

	case "pop":
		if err := worktreeGitRepo.PopStash(stashMessage); err != nil {
			w.logger.Debugf("No stash to restore in %s worktree: %v", worktree.Repo.Alias, err)
		} else {
			w.logger.Infof("Restored stash in %s worktree", worktree.Repo.Alias)
		}
	}

	return nil
}

func (w *Manager) ValidateWorktreeState(worktree *WorktreeInfo) error {
	// Check if worktree directory exists
	if _, err := os.Stat(worktree.WorktreePath); os.IsNotExist(err) {
		return fmt.Errorf("worktree directory %s does not exist", worktree.WorktreePath)
	}

	// Check if it's actually a git worktree
	worktreeGitRepo := git.NewGitRepo(worktree.WorktreePath)
	if !worktreeGitRepo.IsGitRepo() {
		return fmt.Errorf("worktree %s is not a valid git repository", worktree.WorktreePath)
	}

	// Check if we're on the correct branch
	currentBranch, err := worktreeGitRepo.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch in worktree: %w", err)
	}

	if currentBranch != worktree.BranchName {
		return fmt.Errorf("worktree %s is on branch %s, expected %s", 
			worktree.WorktreePath, currentBranch, worktree.BranchName)
	}

	return nil
}

func (w *Manager) GetWorktreeStatus(worktree *WorktreeInfo) (string, error) {
	if err := w.ValidateWorktreeState(worktree); err != nil {
		return fmt.Sprintf("Invalid: %v", err), nil
	}

	worktreeGitRepo := git.NewGitRepo(worktree.WorktreePath)
	
	hasChanges, err := worktreeGitRepo.HasUncommittedChanges()
	if err != nil {
		return fmt.Sprintf("%s (error checking changes)", worktree.BranchName), nil
	}

	if hasChanges {
		return fmt.Sprintf("%s (modified)", worktree.BranchName), nil
	}

	return worktree.BranchName, nil
}