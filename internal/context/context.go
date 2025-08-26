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
	"github.com/viniciusamelio/alfred/internal/tui"
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
	
	repoIdentifier := masterRepo.Alias
	if repoIdentifier == "" {
		repoIdentifier = masterRepo.Name
	}
	
	if !gitRepo.IsGitRepo() {
		return fmt.Errorf("master repository %s is not a git repository", repoIdentifier)
	}

	// Check if branch exists
	branchExists, err := gitRepo.BranchExists(contextName)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}

	if !branchExists {
		// Create new branch from current branch
		m.logger.Infof("Creating new branch %s in master repo %s", contextName, repoIdentifier)
		if err := gitRepo.CreateBranch(contextName, "HEAD"); err != nil {
			return fmt.Errorf("failed to create branch: %w", err)
		}
	} else {
		// Switch to existing branch
		m.logger.Infof("Switching to existing branch %s in master repo %s", contextName, repoIdentifier)
		if err := gitRepo.CheckoutBranch(contextName); err != nil {
			return fmt.Errorf("failed to checkout branch: %w", err)
		}
	}

	// Restore stash if switching from main to another context
	if err := m.handleMasterRepoStashRestore(contextName); err != nil {
		m.logger.Warnf("Failed to restore stash in master repo: %v", err)
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
	// Get the configured main branch name
	configuredMainBranch := m.config.GetMainBranch()
	
	// First, try the configured main branch
	branchExists, err := gitRepo.BranchExists(configuredMainBranch)
	if err == nil && branchExists {
		m.logger.Infof("Switching repo %s to configured main branch: %s", repo.Alias, configuredMainBranch)
		if err := gitRepo.CheckoutBranch(configuredMainBranch); err != nil {
			return fmt.Errorf("failed to checkout main branch %s: %w", configuredMainBranch, err)
		}
		return nil
	}
	
	// If configured main branch doesn't exist, try common alternatives
	mainBranchCandidates := []string{"main", "master", "develop"}
	
	// Remove the configured branch from candidates to avoid duplicates
	var filteredCandidates []string
	for _, candidate := range mainBranchCandidates {
		if candidate != configuredMainBranch {
			filteredCandidates = append(filteredCandidates, candidate)
		}
	}
	
	for _, branchName := range filteredCandidates {
		branchExists, err := gitRepo.BranchExists(branchName)
		if err != nil {
			continue
		}
		
		if branchExists {
			m.logger.Infof("Configured main branch '%s' not found in repo %s, switching to: %s", configuredMainBranch, repo.Alias, branchName)
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
	
	m.logger.Infof("No main branch candidates found in repo %s (including configured '%s'), staying on current branch: %s", repo.Alias, configuredMainBranch, currentBranch)
	return nil
}

func (m *Manager) switchToMainContext(currentContext string) error {
	m.logger.Info("Switching to main context - keeping worktrees and reverting dependencies to git")
	
	// Step 0: Check for uncommitted changes in master repo and handle stash with confirmation
	if currentContext != "" && currentContext != "main" && currentContext != "master" {
		if err := m.handleMasterRepoStashForMainSwitch(currentContext); err != nil {
			return err
		}
	}
	
	// Step 1: Switch master repository to main branch (keep worktrees intact)
	masterRepo, err := m.config.GetMasterRepo()
	if err != nil {
		m.logger.Warnf("No master repository configured: %v", err)
	} else {
		gitRepo := git.NewGitRepo(masterRepo.Path)
		if err := m.switchRepoToMainBranch(gitRepo, masterRepo); err != nil {
			return fmt.Errorf("failed to switch master repo to main branch: %w", err)
		}
	}
	
	// Step 2: Revert master repository dependencies to git references only
	if masterRepo != nil {
		if err := m.revertMasterDependenciesToGit(masterRepo); err != nil {
			m.logger.Warnf("Failed to revert master dependencies to git: %v", err)
		}
		
		// Run flutter pub get in master repository
		if err := m.runFlutterPubGetForRepo(masterRepo); err != nil {
			m.logger.Warnf("Failed to run flutter pub get in master repo: %v", err)
		}
	}
	
	// Step 3: Update current context
	if err := m.SetCurrentContext("main"); err != nil {
		return fmt.Errorf("failed to set current context: %w", err)
	}
	
	m.logger.Info("Successfully switched to main context (worktrees preserved)")
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
			// Get the correct identifier for current repo
			currentRepoIdentifier := repoInfo.Repo.Alias
			if currentRepoIdentifier == "" {
				currentRepoIdentifier = repoInfo.Repo.Name
			}
			
			// Get the correct identifier for other repo
			otherRepoIdentifier := otherRepo.Repo.Alias
			if otherRepoIdentifier == "" {
				otherRepoIdentifier = otherRepo.Repo.Name
			}
			
			if otherRepoIdentifier == currentRepoIdentifier {
				continue
			}

			relativePath, err := filepath.Rel(repoInfo.WorktreePath, otherRepo.WorktreePath)
			if err != nil {
				m.logger.Warnf("Failed to get relative path from %s to %s: %v", 
					repoInfo.WorktreePath, otherRepo.WorktreePath, err)
				continue
			}

			// Use the package name (from pubspec.yaml) for dependency identification
			dependencyName := otherRepo.Repo.Name
			
			if err := pubspecFile.CommentGitDependencyAndAddPath(dependencyName, relativePath); err != nil {
				m.logger.Debugf("Dependency %s not found or not a git dependency in %s: %v", 
					dependencyName, currentRepoIdentifier, err)
			} else {
				m.logger.Infof("Commented git and added path dependency for %s in %s", 
					dependencyName, currentRepoIdentifier)
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
		// Only convert dependencies to repos that are also in this context
		for _, otherWorktree := range worktrees {
			// Get the correct identifier for current repo
			currentRepoIdentifier := worktreeInfo.Repo.Alias
			if currentRepoIdentifier == "" {
				currentRepoIdentifier = worktreeInfo.Repo.Name
			}
			
			// Get the correct identifier for other repo
			otherRepoIdentifier := otherWorktree.Repo.Alias
			if otherRepoIdentifier == "" {
				otherRepoIdentifier = otherWorktree.Repo.Name
			}
			
			if otherRepoIdentifier == currentRepoIdentifier {
				continue
			}

			// Calculate the correct target path:
			// - If target is master repo: always use its original path
			// - If target is non-master repo: always use its worktree path for this context
			var targetPath string
			if otherRepoIdentifier == m.config.Master {
				// Target is master repo - use original path
				targetPath = otherWorktree.Repo.Path
				m.logger.Debugf("Target %s is master repo, using original path: %s", otherRepoIdentifier, targetPath)
			} else {
				// Target is non-master repo - use worktree path
				targetPath = m.worktreeManager.GetWorktreePath(otherWorktree.Repo, contextName)
				m.logger.Debugf("Target %s is non-master repo, using worktree path: %s", otherRepoIdentifier, targetPath)
			}

			relativePath, err := filepath.Rel(worktreeInfo.WorktreePath, targetPath)
			if err != nil {
				m.logger.Warnf("Failed to get relative path from %s to %s: %v", 
					worktreeInfo.WorktreePath, targetPath, err)
				continue
			}

			m.logger.Debugf("Updating %s in %s: %s -> %s (relative: %s)", 
				otherRepoIdentifier, currentRepoIdentifier, 
				worktreeInfo.WorktreePath, targetPath, relativePath)

			// Use the package name (from pubspec.yaml) for dependency identification
			dependencyName := otherWorktree.Repo.Name
			
			// Try to comment git and add path first, if that fails, try to update existing path
			if err := pubspecFile.CommentGitDependencyAndAddPath(dependencyName, relativePath); err != nil {
				// If it's not a git dependency, try to update existing path dependency
				if err2 := pubspecFile.UpdatePathDependency(dependencyName, relativePath); err2 != nil {
					m.logger.Debugf("Dependency %s not found as git or path dependency in %s: git_error=%v, path_error=%v", 
						dependencyName, currentRepoIdentifier, err, err2)
				} else {
					m.logger.Infof("Updated %s path dependency in %s to: %s", 
						dependencyName, currentRepoIdentifier, relativePath)
				}
			} else {
				m.logger.Infof("Commented git and added path dependency for %s in %s: %s", 
					dependencyName, currentRepoIdentifier, relativePath)
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
			// Get the correct identifier for current repo
			currentRepoIdentifier := repoInfo.Repo.Alias
			if currentRepoIdentifier == "" {
				currentRepoIdentifier = repoInfo.Repo.Name
			}
			
			// Get the correct identifier for other repo
			otherRepoIdentifier := otherRepo.Repo.Alias
			if otherRepoIdentifier == "" {
				otherRepoIdentifier = otherRepo.Repo.Name
			}
			
			if otherRepoIdentifier == currentRepoIdentifier {
				continue
			}

			// Use the package name (from pubspec.yaml) for dependency identification
			dependencyName := otherRepo.Repo.Name

			// For main context, we want to use git dependencies
			if err := pubspecFile.ConvertPathToGitFromBackup(dependencyName); err != nil {
				m.logger.Debugf("Dependency %s not found or could not convert back to git in %s: %v", 
					dependencyName, currentRepoIdentifier, err)
			} else {
				m.logger.Infof("Converted %s dependency in %s back to git reference", 
					dependencyName, currentRepoIdentifier)
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

// handleMasterRepoStashForMainSwitch checks for uncommitted changes in master repo
// and shows confirmation dialog for stashing when switching to main context
func (m *Manager) handleMasterRepoStashForMainSwitch(currentContext string) error {
	// Get master repository
	masterRepo, err := m.config.GetMasterRepo()
	if err != nil {
		// No master repo configured, nothing to do
		m.logger.Debug("No master repository configured, skipping stash check")
		return nil
	}
	
	gitRepo := git.NewGitRepo(masterRepo.Path)
	if !gitRepo.IsGitRepo() {
		return nil
	}
	
	// Check for uncommitted changes
	hasChanges, err := gitRepo.HasUncommittedChanges()
	if err != nil {
		m.logger.Warnf("Failed to check for uncommitted changes in master repo: %v", err)
		return nil
	}
	
	if !hasChanges {
		// No changes to stash, proceed normally
		return nil
	}
	
	// Show confirmation dialog via TUI
	repoIdentifier := masterRepo.Alias
	if repoIdentifier == "" {
		repoIdentifier = masterRepo.Name
	}
	
	// Try TUI confirmation, if it fails (no TTY), auto-confirm
	confirmed, err := tui.RunStashConfirmation(currentContext, repoIdentifier)
	if err != nil {
		if strings.Contains(err.Error(), "TTY") || strings.Contains(err.Error(), "tty") {
			// No TTY available, auto-confirm stash
			m.logger.Infof("No TTY available for stash confirmation, auto-stashing changes in %s", repoIdentifier)
			confirmed = true
		} else {
			return fmt.Errorf("stash confirmation failed: %w", err)
		}
	}
	
	if !confirmed {
		return fmt.Errorf("switch cancelled by user")
	}
	
	// User confirmed, stash the changes
	if err := gitRepo.StashForContext(currentContext); err != nil {
		return fmt.Errorf("failed to stash changes in master repo: %w", err)
	}
	
	m.logger.Infof("Stashed uncommitted changes in master repo %s for context %s", repoIdentifier, currentContext)
	return nil
}

// handleMasterRepoStashRestore restores stash when switching back from main context
func (m *Manager) handleMasterRepoStashRestore(targetContext string) error {
	// Get master repository
	masterRepo, err := m.config.GetMasterRepo()
	if err != nil {
		// No master repo configured, nothing to do
		return nil
	}
	
	gitRepo := git.NewGitRepo(masterRepo.Path)
	if !gitRepo.IsGitRepo() {
		return nil
	}
	
	// Check if there's a stash for this context
	hasStash, err := gitRepo.HasStashForContext(targetContext)
	if err != nil {
		m.logger.Warnf("Failed to check for stash in master repo: %v", err)
		return nil
	}
	
	if hasStash {
		// Restore the stash
		if err := gitRepo.PopStashForContext(targetContext); err != nil {
			m.logger.Warnf("Failed to restore stash in master repo: %v", err)
			return nil
		}
		
		repoIdentifier := masterRepo.Alias
		if repoIdentifier == "" {
			repoIdentifier = masterRepo.Name
		}
		
		m.logger.Infof("Restored stashed changes in master repo %s from context %s", repoIdentifier, targetContext)
	}
	
	return nil
}

// revertMasterDependenciesToGit reverts all commented git dependencies back to git in master repository
func (m *Manager) revertMasterDependenciesToGit(masterRepo *config.Repository) error {
	pubspecPath := filepath.Join(masterRepo.Path, "pubspec.yaml")
	if _, err := os.Stat(pubspecPath); os.IsNotExist(err) {
		m.logger.Debugf("No pubspec.yaml found in master repo, skipping dependency revert")
		return nil
	}

	pubspecFile, err := pubspec.LoadPubspec(masterRepo.Path)
	if err != nil {
		return fmt.Errorf("failed to load pubspec.yaml in master repo: %w", err)
	}

	// Get all repositories to find dependencies to revert
	allRepos := m.config.Repos
	for _, repo := range allRepos {
		// Skip the master repository itself
		if repo.Alias == masterRepo.Alias || repo.Name == masterRepo.Name {
			continue
		}

		// Use package name for dependency identification
		dependencyName := repo.Name
		
		// Try to uncomment git dependency and remove path dependency
		if err := pubspecFile.UncommentGitDependencyAndRemovePath(dependencyName); err != nil {
			m.logger.Debugf("Dependency %s not found or not in expected format in master repo: %v", dependencyName, err)
		} else {
			m.logger.Infof("Reverted %s dependency in master repo back to git reference", dependencyName)
		}
	}

	if err := pubspecFile.Save(); err != nil {
		return fmt.Errorf("failed to save pubspec.yaml in master repo: %w", err)
	}

	return nil
}

// runFlutterPubGetForRepo runs flutter pub get in a specific repository
func (m *Manager) runFlutterPubGetForRepo(repo *config.Repository) error {
	pubspecPath := filepath.Join(repo.Path, "pubspec.yaml")
	if _, err := os.Stat(pubspecPath); os.IsNotExist(err) {
		m.logger.Debugf("No pubspec.yaml in %s, skipping flutter pub get", repo.Alias)
		return nil
	}

	repoIdentifier := repo.Alias
	if repoIdentifier == "" {
		repoIdentifier = repo.Name
	}

	m.logger.Infof("Running flutter pub get in %s (path: %s)", repoIdentifier, repo.Path)
	cmd := exec.Command("flutter", "pub", "get")
	cmd.Dir = repo.Path
	
	// Capture output for logging
	output, err := cmd.CombinedOutput()
	if err != nil {
		m.logger.Warnf("flutter pub get failed in %s: %v\nOutput: %s", 
			repoIdentifier, err, string(output))
		return fmt.Errorf("flutter pub get failed in %s: %w", repoIdentifier, err)
	}

	m.logger.Infof("flutter pub get completed successfully in %s", repoIdentifier)
	return nil
}