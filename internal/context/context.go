package context

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/viniciusamelio/alfred/internal/config"
	"github.com/viniciusamelio/alfred/internal/git"
	"github.com/viniciusamelio/alfred/internal/pubspec"
	"github.com/viniciusamelio/alfred/internal/worktree"
)

type Manager struct {
	config          *config.Config
	logger          *log.Logger
	worktreeManager *worktree.Manager
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		config:          cfg,
		logger:          log.Default(),
		worktreeManager: worktree.NewManager(cfg),
	}
}

func (m *Manager) getCurrentContextFile() string {
	return filepath.Join(".", ".alfred", "current-context")
}

func (m *Manager) GetCurrentContext() (string, error) {
	contextFile := m.getCurrentContextFile()
	if _, err := os.Stat(contextFile); os.IsNotExist(err) {
		return "", nil
	}
	
	data, err := os.ReadFile(contextFile)
	if err != nil {
		return "", fmt.Errorf("failed to read context file: %w", err)
	}
	
	return strings.TrimSpace(string(data)), nil
}

func (m *Manager) SetCurrentContext(contextName string) error {
	// Ensure .alfred directory exists
	alfredDir := filepath.Join(".", ".alfred")
	if err := os.MkdirAll(alfredDir, 0755); err != nil {
		return fmt.Errorf("failed to create .alfred directory: %w", err)
	}
	
	contextFile := m.getCurrentContextFile()
	return os.WriteFile(contextFile, []byte(contextName), 0644)
}

func (m *Manager) SwitchContext(contextName string) error {
	m.logger.Infof("Switching to context: %s (mode: %s)", contextName, m.config.Mode)

	currentContext, err := m.GetCurrentContext()
	if err != nil {
		return fmt.Errorf("failed to get current context: %w", err)
	}

	if currentContext == contextName {
		m.logger.Infof("Already on context '%s'", contextName)
		return nil
	}

	if m.config.IsBranchMode() {
		return m.switchContextBranchMode(contextName, currentContext)
	} else {
		return m.switchContextWorktreeMode(contextName, currentContext)
	}
}

func (m *Manager) switchContextBranchMode(contextName string, currentContext string) error {
	// Get all repos for the target context
	repos, err := m.config.GetContextRepos(contextName)
	if err != nil {
		return err
	}

	// Step 1: Stash changes in all repos (branch mode uses git stash)
	if currentContext != "" {
		if err := m.stashAllRepos(repos, currentContext); err != nil {
			m.logger.Warnf("Failed to stash changes in repos: %v", err)
		}
	}

	// Step 2: Switch all repos to context branch
	var repoInfos []*worktree.WorktreeInfo
	for _, repo := range repos {
		if err := m.switchRepoToContext(repo, contextName); err != nil {
			return fmt.Errorf("failed to switch repo %s to context: %w", repo.Alias, err)
		}

		// Create repo info for dependency updates (all repos work in their original paths)
		repoInfo := &worktree.WorktreeInfo{
			Repo:         repo,
			WorktreePath: repo.Path,
			BranchName:   contextName,
		}
		repoInfos = append(repoInfos, repoInfo)
	}

	// Step 3: Restore stash in all repos
	for _, repoInfo := range repoInfos {
		if err := m.restoreStashInRepo(repoInfo, contextName); err != nil {
			m.logger.Warnf("Failed to restore stash in %s: %v", repoInfo.Repo.Alias, err)
		}
	}

	// Step 4: Update pubspec files to use relative paths between repos
	if err := m.updatePubspecFilesForBranchMode(repoInfos, contextName); err != nil {
		return fmt.Errorf("failed to update pubspec files: %w", err)
	}

	// Step 5: Set current context
	if err := m.SetCurrentContext(contextName); err != nil {
		return fmt.Errorf("failed to set current context: %w", err)
	}

	// Step 6: Run flutter pub get in each repo
	if err := m.runFlutterPubGet(repoInfos); err != nil {
		m.logger.Warnf("Failed to run flutter pub get: %v", err)
	}

	m.logger.Infof("Successfully switched to context '%s' in branch mode", contextName)
	return nil
}

func (m *Manager) switchContextWorktreeMode(contextName string, currentContext string) error {
	// Handle special "main" context - clean up worktrees and switch to main branches
	if contextName == "main" || contextName == "master" {
		return m.switchToMainContext(currentContext)
	}
	// Step 1: Handle master repository (if configured and in context)
	var masterWorktreeInfo *worktree.WorktreeInfo
	if m.config.IsContextContainsMaster(contextName) {
		masterRepo, err := m.config.GetMasterRepo()
		if err != nil {
			return fmt.Errorf("failed to get master repo: %w", err)
		}

		// Switch master repo to context branch (no worktree creation)
		if err := m.switchMasterRepoToContext(masterRepo, contextName); err != nil {
			return fmt.Errorf("failed to switch master repo to context: %w", err)
		}

		// Create a "fake" worktree info for master repo to include in dependency updates
		masterWorktreeInfo = &worktree.WorktreeInfo{
			Repo:         masterRepo,
			WorktreePath: masterRepo.Path,
			BranchName:   contextName,
		}
	}

	// Step 2: Stash changes in current context worktrees (excluding master)
	if currentContext != "" {
		if err := m.stashCurrentContextWorktrees(currentContext); err != nil {
			m.logger.Warnf("Failed to stash current context: %v", err)
		}
	}

	// Step 3: Create/setup worktrees for non-master repos in target context
	nonMasterRepos, err := m.config.GetNonMasterReposForContext(contextName)
	if err != nil {
		return fmt.Errorf("failed to get non-master repos: %w", err)
	}

	var contextWorktrees []*worktree.WorktreeInfo
	if masterWorktreeInfo != nil {
		contextWorktrees = append(contextWorktrees, masterWorktreeInfo)
	}

	for _, repo := range nonMasterRepos {
		worktreeInfo, err := m.worktreeManager.CreateWorktreeForContext(repo, contextName)
		if err != nil {
			return fmt.Errorf("failed to create worktree for repo %s: %w", repo.Alias, err)
		}
		contextWorktrees = append(contextWorktrees, worktreeInfo)
	}

	// Step 4: Restore stash in target context worktrees (excluding master)
	for _, worktreeInfo := range contextWorktrees {
		if worktreeInfo.Repo.Alias != m.config.Master {
			if err := m.worktreeManager.HandleStashForWorktree(worktreeInfo, contextName, "pop"); err != nil {
				m.logger.Warnf("Failed to restore stash for %s: %v", worktreeInfo.Repo.Alias, err)
			}
		}
	}

	// Step 5: Update pubspec files to use correct paths
	if err := m.updatePubspecFilesForWorktrees(contextWorktrees, contextName); err != nil {
		return fmt.Errorf("failed to update pubspec files: %w", err)
	}

	// Step 6: Set current context
	if err := m.SetCurrentContext(contextName); err != nil {
		return fmt.Errorf("failed to set current context: %w", err)
	}

	// Step 7: Run flutter pub get in each repo/worktree
	if err := m.runFlutterPubGet(contextWorktrees); err != nil {
		m.logger.Warnf("Failed to run flutter pub get: %v", err)
	}

	m.logger.Infof("Successfully switched to context '%s' in worktree mode", contextName)
	return nil
}

func (m *Manager) switchMasterRepoToContext(masterRepo *config.Repository, contextName string) error {
	gitRepo := git.NewGitRepo(masterRepo.Path)
	
	if !gitRepo.IsGitRepo() {
		return fmt.Errorf("master repository %s is not a git repository", masterRepo.Alias)
	}

	// Check if branch exists
	branchExists, err := gitRepo.BranchExists(contextName)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}

	if !branchExists {
		// Create new branch from current branch
		m.logger.Infof("Creating new branch %s in master repo %s", contextName, masterRepo.Alias)
		if err := gitRepo.CreateBranch(contextName, "HEAD"); err != nil {
			return fmt.Errorf("failed to create branch: %w", err)
		}
	} else {
		// Switch to existing branch
		m.logger.Infof("Switching to existing branch %s in master repo %s", contextName, masterRepo.Alias)
		if err := gitRepo.CheckoutBranch(contextName); err != nil {
			return fmt.Errorf("failed to checkout branch: %w", err)
		}
	}

	return nil
}

func (m *Manager) switchRepoToContext(repo *config.Repository, contextName string) error {
	gitRepo := git.NewGitRepo(repo.Path)
	
	if !gitRepo.IsGitRepo() {
		return fmt.Errorf("repository %s is not a git repository", repo.Alias)
	}

	// Handle special "main" context - switch to main/master branch
	if contextName == "main" || contextName == "master" {
		return m.switchRepoToMainBranch(gitRepo, repo)
	}

	// Check if branch exists
	branchExists, err := gitRepo.BranchExists(contextName)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}

	if !branchExists {
		// Create new branch from current branch
		m.logger.Infof("Creating new branch %s in repo %s", contextName, repo.Alias)
		if err := gitRepo.CreateBranch(contextName, "HEAD"); err != nil {
			return fmt.Errorf("failed to create branch: %w", err)
		}
	} else {
		// Switch to existing branch
		m.logger.Infof("Switching to existing branch %s in repo %s", contextName, repo.Alias)
		if err := gitRepo.CheckoutBranch(contextName); err != nil {
			return fmt.Errorf("failed to checkout branch: %w", err)
		}
	}

	return nil
}

func (m *Manager) switchRepoToMainBranch(gitRepo *git.GitRepo, repo *config.Repository) error {
	// Try to determine the main branch name (main, master, develop, etc.)
	mainBranchCandidates := []string{"main", "master", "develop"}
	
	for _, branchName := range mainBranchCandidates {
		branchExists, err := gitRepo.BranchExists(branchName)
		if err != nil {
			continue
		}
		
		if branchExists {
			m.logger.Infof("Switching repo %s to main branch: %s", repo.Alias, branchName)
			if err := gitRepo.CheckoutBranch(branchName); err != nil {
				return fmt.Errorf("failed to checkout main branch %s: %w", branchName, err)
			}
			return nil
		}
	}
	
	// If no standard main branch found, try to get the default branch
	currentBranch, err := gitRepo.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch for repo %s: %w", repo.Alias, err)
	}
	
	m.logger.Infof("No standard main branch found in repo %s, staying on current branch: %s", repo.Alias, currentBranch)
	return nil
}

func (m *Manager) switchToMainContext(currentContext string) error {
	m.logger.Info("Switching to main context - cleaning up worktrees and switching to main branches")
	
	// Step 1: Clean up all existing worktrees if there's a current context
	if currentContext != "" && currentContext != "main" && currentContext != "master" {
		if err := m.cleanupAllWorktrees(currentContext); err != nil {
			m.logger.Warnf("Failed to cleanup worktrees: %v", err)
		}
	}
	
	// Step 2: Switch all repositories to their main branches
	allRepos := m.config.Repos
	for _, repo := range allRepos {
		gitRepo := git.NewGitRepo(repo.Path)
		if err := m.switchRepoToMainBranch(gitRepo, &repo); err != nil {
			return fmt.Errorf("failed to switch repo %s to main branch: %w", repo.Alias, err)
		}
	}
	
	// Step 3: Update pubspec dependencies to use git URLs instead of local paths
	var repoInfos []*worktree.WorktreeInfo
	for _, repo := range allRepos {
		repoInfo := &worktree.WorktreeInfo{
			Repo:         &repo,
			WorktreePath: repo.Path,
			BranchName:   "main", // Use "main" as the logical context name
		}
		repoInfos = append(repoInfos, repoInfo)
	}
	
	if err := m.updateDependencies(repoInfos); err != nil {
		m.logger.Warnf("Failed to update dependencies: %v", err)
	}
	
	// Step 4: Run flutter pub get in all repositories
	if err := m.runFlutterPubGetForMain(allRepos); err != nil {
		m.logger.Warnf("Failed to run flutter pub get: %v", err)
	}
	
	// Step 5: Update current context
	if err := m.SetCurrentContext("main"); err != nil {
		return fmt.Errorf("failed to set current context: %w", err)
	}
	
	m.logger.Info("Successfully switched to main context")
	return nil
}

func (m *Manager) cleanupAllWorktrees(contextName string) error {
	// Get all repositories that might have worktrees for the current context
	allRepos := m.config.Repos
	
	for _, repo := range allRepos {
		// Skip master repository as it doesn't have worktrees
		if repo.Alias == m.config.Master {
			continue
		}
		
		worktreePath := m.worktreeManager.GetWorktreePath(&repo, contextName)
		
		gitRepo := git.NewGitRepo(repo.Path)
		if err := gitRepo.RemoveWorktree(worktreePath); err != nil {
			m.logger.Warnf("Failed to remove worktree for %s: %v", repo.Alias, err)
		}
	}
	
	return nil
}

func (m *Manager) runFlutterPubGetForMain(repos []config.Repository) error {
	for _, repo := range repos {
		// Check if this is a Flutter/Dart project (has pubspec.yaml)
		pubspecPath := filepath.Join(repo.Path, "pubspec.yaml")
		if _, err := os.Stat(pubspecPath); os.IsNotExist(err) {
			m.logger.Debugf("No pubspec.yaml in %s, skipping flutter pub get", repo.Alias)
			continue
		}

		m.logger.Infof("Running flutter pub get in %s (path: %s)", repo.Alias, repo.Path)
		cmd := exec.Command("flutter", "pub", "get")
		cmd.Dir = repo.Path
		
		// Capture output for logging
		output, err := cmd.CombinedOutput()
		if err != nil {
			m.logger.Warnf("flutter pub get failed in %s: %v\nOutput: %s", 
				repo.Alias, err, string(output))
			continue
		}

		m.logger.Infof("flutter pub get completed successfully in %s", repo.Alias)
	}
	return nil
}

func (m *Manager) stashAllRepos(repos []*config.Repository, contextName string) error {
	for _, repo := range repos {
		if err := m.stashRepoChanges(repo, contextName); err != nil {
			m.logger.Warnf("Failed to stash changes in %s: %v", repo.Alias, err)
		}
	}
	return nil
}

func (m *Manager) stashRepoChanges(repo *config.Repository, contextName string) error {
	gitRepo := git.NewGitRepo(repo.Path)
	
	if !gitRepo.IsGitRepo() {
		return nil
	}

	hasChanges, err := gitRepo.HasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("failed to check changes: %w", err)
	}

	if hasChanges {
		stashMessage := fmt.Sprintf("alfred-context-%s", contextName)
		if err := gitRepo.StashChanges(stashMessage); err != nil {
			return fmt.Errorf("failed to stash changes: %w", err)
		}
		m.logger.Infof("Stashed changes in %s", repo.Alias)
	}

	return nil
}

func (m *Manager) restoreStashInRepo(repoInfo *worktree.WorktreeInfo, contextName string) error {
	gitRepo := git.NewGitRepo(repoInfo.Repo.Path)
	stashMessage := fmt.Sprintf("alfred-context-%s", contextName)

	if err := gitRepo.PopStash(stashMessage); err != nil {
		m.logger.Debugf("No stash to restore in %s: %v", repoInfo.Repo.Alias, err)
	} else {
		m.logger.Infof("Restored stash in %s", repoInfo.Repo.Alias)
	}

	return nil
}

func (m *Manager) updatePubspecFilesForBranchMode(repoInfos []*worktree.WorktreeInfo, contextName string) error {
	// In branch mode, all repos work in their original paths, so dependencies should use relative paths
	for _, repoInfo := range repoInfos {
		pubspecPath := filepath.Join(repoInfo.WorktreePath, "pubspec.yaml")
		if _, err := os.Stat(pubspecPath); os.IsNotExist(err) {
			m.logger.Debugf("No pubspec.yaml found in %s, skipping", repoInfo.Repo.Alias)
			continue
		}

		pubspecFile, err := pubspec.LoadPubspec(repoInfo.WorktreePath)
		if err != nil {
			m.logger.Warnf("Failed to load pubspec.yaml in %s: %v", repoInfo.Repo.Alias, err)
			continue
		}

		if err := pubspecFile.BackupOriginal(); err != nil {
			m.logger.Warnf("Failed to backup pubspec.yaml in %s: %v", repoInfo.Repo.Alias, err)
		}

		// Update dependencies to point to other repos in the same context
		for _, otherRepo := range repoInfos {
			if otherRepo.Repo.Alias == repoInfo.Repo.Alias {
				continue
			}

			relativePath, err := filepath.Rel(repoInfo.WorktreePath, otherRepo.WorktreePath)
			if err != nil {
				m.logger.Warnf("Failed to get relative path from %s to %s: %v", 
					repoInfo.WorktreePath, otherRepo.WorktreePath, err)
				continue
			}

			if err := pubspecFile.ConvertGitToPath(otherRepo.Repo.Alias, relativePath); err != nil {
				m.logger.Debugf("Dependency %s not found or not a git dependency in %s: %v", 
					otherRepo.Repo.Alias, repoInfo.Repo.Alias, err)
			} else {
				m.logger.Infof("Updated %s dependency in %s to use local path", 
					otherRepo.Repo.Alias, repoInfo.Repo.Alias)
			}
		}

		if err := pubspecFile.Save(); err != nil {
			m.logger.Warnf("Failed to save pubspec.yaml in %s: %v", repoInfo.Repo.Alias, err)
		}
	}

	return nil
}

func (m *Manager) stashCurrentContextWorktrees(contextName string) error {
	// Only stash non-master repos (master repo doesn't use worktrees)
	nonMasterRepos, err := m.config.GetNonMasterReposForContext(contextName)
	if err != nil {
		return err
	}

	// Get existing worktrees for the current context
	worktrees, err := m.worktreeManager.ListWorktreesForContext(nonMasterRepos, contextName)
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Stash changes in each worktree
	for _, worktreeInfo := range worktrees {
		if err := m.worktreeManager.HandleStashForWorktree(worktreeInfo, contextName, "push"); err != nil {
			m.logger.Warnf("Failed to stash changes in %s worktree: %v", worktreeInfo.Repo.Alias, err)
		}
	}

	return nil
}

func (m *Manager) updatePubspecFilesForWorktrees(worktrees []*worktree.WorktreeInfo, contextName string) error {
	m.logger.Debugf("Updating pubspec files for %d worktrees in context '%s'", len(worktrees), contextName)
	for i, worktree := range worktrees {
		m.logger.Debugf("  [%d] %s: %s (master: %v)", i, worktree.Repo.Alias, worktree.WorktreePath, 
			worktree.Repo.Alias == m.config.Master)
	}
	
	for _, worktreeInfo := range worktrees {
		pubspecPath := filepath.Join(worktreeInfo.WorktreePath, "pubspec.yaml")
		if _, err := os.Stat(pubspecPath); os.IsNotExist(err) {
			m.logger.Debugf("No pubspec.yaml found in %s worktree, skipping", worktreeInfo.Repo.Alias)
			continue
		}

		pubspecFile, err := pubspec.LoadPubspec(worktreeInfo.WorktreePath)
		if err != nil {
			m.logger.Warnf("Failed to load pubspec.yaml in %s worktree: %v", worktreeInfo.Repo.Alias, err)
			continue
		}

		if err := pubspecFile.BackupOriginal(); err != nil {
			m.logger.Warnf("Failed to backup pubspec.yaml in %s worktree: %v", worktreeInfo.Repo.Alias, err)
		}

		// Update dependencies to point to other worktrees/repos in the same context
		for _, otherWorktree := range worktrees {
			if otherWorktree.Repo.Alias == worktreeInfo.Repo.Alias {
				continue
			}

			// Calculate the correct target path:
			// - If target is master repo: always use its original path
			// - If target is non-master repo: always use its worktree path for this context
			var targetPath string
			if otherWorktree.Repo.Alias == m.config.Master {
				// Target is master repo - use original path
				targetPath = otherWorktree.Repo.Path
				m.logger.Debugf("Target %s is master repo, using original path: %s", otherWorktree.Repo.Alias, targetPath)
			} else {
				// Target is non-master repo - use worktree path
				targetPath = m.worktreeManager.GetWorktreePath(otherWorktree.Repo, contextName)
				m.logger.Debugf("Target %s is non-master repo, using worktree path: %s", otherWorktree.Repo.Alias, targetPath)
			}

			relativePath, err := filepath.Rel(worktreeInfo.WorktreePath, targetPath)
			if err != nil {
				m.logger.Warnf("Failed to get relative path from %s to %s: %v", 
					worktreeInfo.WorktreePath, targetPath, err)
				continue
			}

			m.logger.Debugf("Updating %s in %s: %s -> %s (relative: %s)", 
				otherWorktree.Repo.Alias, worktreeInfo.Repo.Alias, 
				worktreeInfo.WorktreePath, targetPath, relativePath)

			// Try to convert git to path first, if that fails, try to update existing path
			if err := pubspecFile.ConvertGitToPath(otherWorktree.Repo.Alias, relativePath); err != nil {
				// If it's not a git dependency, try to update existing path dependency
				if err2 := pubspecFile.UpdatePathDependency(otherWorktree.Repo.Alias, relativePath); err2 != nil {
					m.logger.Debugf("Dependency %s not found as git or path dependency in %s: git_error=%v, path_error=%v", 
						otherWorktree.Repo.Alias, worktreeInfo.Repo.Alias, err, err2)
				} else {
					m.logger.Infof("Updated %s path dependency in %s to: %s", 
						otherWorktree.Repo.Alias, worktreeInfo.Repo.Alias, relativePath)
				}
			} else {
				m.logger.Infof("Converted %s dependency in %s from git to local path: %s", 
					otherWorktree.Repo.Alias, worktreeInfo.Repo.Alias, relativePath)
			}
		}

		if err := pubspecFile.Save(); err != nil {
			m.logger.Warnf("Failed to save pubspec.yaml in %s: %v", worktreeInfo.Repo.Alias, err)
		}
	}

	return nil
}

func (m *Manager) updateDependencies(repoInfos []*worktree.WorktreeInfo) error {
	// For main context or branch mode, update to use git dependencies
	// This is typically used when switching to main context or branch mode
	for _, repoInfo := range repoInfos {
		pubspecPath := filepath.Join(repoInfo.WorktreePath, "pubspec.yaml")
		if _, err := os.Stat(pubspecPath); os.IsNotExist(err) {
			m.logger.Debugf("No pubspec.yaml found in %s, skipping", repoInfo.Repo.Alias)
			continue
		}

		pubspecFile, err := pubspec.LoadPubspec(repoInfo.WorktreePath)
		if err != nil {
			m.logger.Warnf("Failed to load pubspec.yaml in %s: %v", repoInfo.Repo.Alias, err)
			continue
		}

		if err := pubspecFile.BackupOriginal(); err != nil {
			m.logger.Warnf("Failed to backup pubspec.yaml in %s: %v", repoInfo.Repo.Alias, err)
		}

		// Convert path dependencies back to git dependencies for main/master branches
		for _, otherRepo := range repoInfos {
			if otherRepo.Repo.Alias == repoInfo.Repo.Alias {
				continue
			}

			// For main context, we want to use git dependencies
			if err := pubspecFile.ConvertPathToGitFromBackup(otherRepo.Repo.Alias); err != nil {
				m.logger.Debugf("Dependency %s not found or could not convert back to git in %s: %v", 
					otherRepo.Repo.Alias, repoInfo.Repo.Alias, err)
			} else {
				m.logger.Infof("Converted %s dependency in %s back to git reference", 
					otherRepo.Repo.Alias, repoInfo.Repo.Alias)
			}
		}

		if err := pubspecFile.Save(); err != nil {
			m.logger.Warnf("Failed to save pubspec.yaml in %s: %v", repoInfo.Repo.Alias, err)
		}
	}

	return nil
}

func (m *Manager) ListContexts() []string {
	return m.config.GetContextNames()
}


func (m *Manager) GetContextStatus() (string, map[string]string, error) {
	currentContext, err := m.GetCurrentContext()
	if err != nil {
		return "", nil, err
	}

	if currentContext == "" {
		return "", nil, nil
	}

	repos, err := m.config.GetContextRepos(currentContext)
	if err != nil {
		return currentContext, nil, err
	}

	// Get worktrees for the current context
	worktrees, err := m.worktreeManager.ListWorktreesForContext(repos, currentContext)
	if err != nil {
		return currentContext, nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	status := make(map[string]string)
	for _, worktreeInfo := range worktrees {
		worktreeStatus, err := m.worktreeManager.GetWorktreeStatus(worktreeInfo)
		if err != nil {
			status[worktreeInfo.Repo.Alias] = fmt.Sprintf("Error: %v", err)
		} else {
			status[worktreeInfo.Repo.Alias] = worktreeStatus
		}
	}

	// Check for repos that don't have worktrees yet
	for _, repo := range repos {
		if _, exists := status[repo.Alias]; !exists {
			status[repo.Alias] = "No worktree (not switched to this context yet)"
		}
	}

	return currentContext, status, nil
}

func (m *Manager) runFlutterPubGet(worktrees []*worktree.WorktreeInfo) error {
	m.logger.Infof("Running flutter pub get in %d worktrees", len(worktrees))
	
	for _, worktreeInfo := range worktrees {
		// Check if this is a Flutter/Dart project (has pubspec.yaml)
		pubspecPath := filepath.Join(worktreeInfo.WorktreePath, "pubspec.yaml")
		if _, err := os.Stat(pubspecPath); os.IsNotExist(err) {
			m.logger.Debugf("No pubspec.yaml in %s, skipping flutter pub get", worktreeInfo.Repo.Alias)
			continue
		}

		m.logger.Infof("Running flutter pub get in %s (path: %s)", worktreeInfo.Repo.Alias, worktreeInfo.WorktreePath)
		cmd := exec.Command("flutter", "pub", "get")
		cmd.Dir = worktreeInfo.WorktreePath
		
		// Capture output for logging
		output, err := cmd.CombinedOutput()
		if err != nil {
			m.logger.Warnf("flutter pub get failed in %s: %v\nOutput: %s", 
				worktreeInfo.Repo.Alias, err, string(output))
			continue
		}

		m.logger.Infof("flutter pub get completed successfully in %s", worktreeInfo.Repo.Alias)
	}

	return nil
}

func (m *Manager) DeleteContexts(contextNames []string) error {
	m.logger.Infof("Deleting contexts: %s", strings.Join(contextNames, ", "))

	for _, contextName := range contextNames {
		if err := m.deleteContext(contextName); err != nil {
			return fmt.Errorf("failed to delete context %s: %w", contextName, err)
		}
	}

	// Remove contexts from config and save
	for _, contextName := range contextNames {
		if err := m.config.RemoveContext(contextName); err != nil {
			m.logger.Warnf("Failed to remove context from config: %v", err)
		}
	}

	if err := m.config.Save(); err != nil {
		return fmt.Errorf("failed to save config after deletion: %w", err)
	}

	m.logger.Infof("Successfully deleted contexts: %s", strings.Join(contextNames, ", "))
	return nil
}

func (m *Manager) deleteContext(contextName string) error {
	// Remove worktrees for non-master repos only
	nonMasterRepos, err := m.config.GetNonMasterReposForContext(contextName)
	if err != nil {
		return err
	}

	for _, repo := range nonMasterRepos {
		if err := m.worktreeManager.RemoveWorktreeForContext(repo, contextName); err != nil {
			m.logger.Warnf("Failed to remove worktree for %s: %v", repo.Alias, err)
		}
	}

	// Delete branches for all repos in context (including master)
	allRepos, err := m.config.GetContextRepos(contextName)
	if err != nil {
		return err
	}

	for _, repo := range allRepos {
		if err := m.deleteBranchIfExists(repo, contextName); err != nil {
			m.logger.Warnf("Failed to delete branch %s in %s: %v", contextName, repo.Alias, err)
		}
	}

	return nil
}

func (m *Manager) deleteBranchIfExists(repo *config.Repository, branchName string) error {
	gitRepo := git.NewGitRepo(repo.Path)
	
	if !gitRepo.IsGitRepo() {
		return nil
	}

	// Check if branch exists
	branchExists, err := gitRepo.BranchExists(branchName)
	if err != nil || !branchExists {
		return err
	}

	// Delete the branch
	cmd := exec.Command("git", "-C", repo.Path, "branch", "-D", branchName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete branch: %w", err)
	}

	m.logger.Infof("Deleted branch %s in %s", branchName, repo.Alias)
	return nil
}