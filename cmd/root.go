package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/log"
	"github.com/viniciusamelio/alfred/internal/config"
	"github.com/viniciusamelio/alfred/internal/context"
	"github.com/viniciusamelio/alfred/internal/pubspec"
	"github.com/viniciusamelio/alfred/internal/tui"
)

var CLI struct {
	Debug      bool           `help:"Enable debug mode" default:"false"`
	Context    ContextCmd     `cmd:"" help:"Manage project contexts"`
	Init       InitCmd        `cmd:"" help:"Initialize alfred in current directory"`
	Scan       ScanCmd        `cmd:"" help:"Scan directory and auto-configure repositories"`
	Status     StatusCmd      `cmd:"" help:"Show current context and repository status"`
	List       ListCmd        `cmd:"" help:"List available contexts"`
	Switch     SwitchCmd      `cmd:"" help:"Switch to a different context"`
	Create     CreateCmd      `cmd:"" help:"Create a new context"`
	Delete     DeleteCmd      `cmd:"" help:"Delete contexts"`
	Prepare    PrepareCmd     `cmd:"" help:"Prepare repository for production by reverting to git dependencies"`
	MainBranch MainBranchCmd  `cmd:"" help:"Set the main branch used when switching to main context"`
	Version    VersionCmd     `cmd:"" help:"Show version information"`
}

type ContextCmd struct {
	List   ListCmd   `cmd:"" help:"List available contexts"`
	Switch SwitchCmd `cmd:"" help:"Switch to a context"`
	Create CreateCmd `cmd:"" help:"Create a new context"`
	Delete DeleteCmd `cmd:"" help:"Delete contexts"`
	Scan   ScanCmd   `cmd:"" help:"Scan directory and auto-configure repositories"`
}

type ScanCmd struct{}

func (c *ScanCmd) Run(ctx *kong.Context) error {
	// Check if alfred is already initialized
	if _, err := os.Stat(filepath.Join(".", ".alfred", "alfred.yaml")); err == nil {
		fmt.Println("⚠️  Alfred is already initialized in this directory.")
		fmt.Print("Do you want to overwrite the existing configuration? (y/N): ")
		
		var response string
		fmt.Scanln(&response)
		
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Operation cancelled.")
			return nil
		}
		fmt.Println()
	}
	
	// Scan for Dart/Flutter packages
	packages, err := c.scanForDartPackages()
	if err != nil {
		return fmt.Errorf("failed to scan for packages: %w", err)
	}
	
	if len(packages) == 0 {
		return fmt.Errorf("no Dart/Flutter packages found in current directory")
	}
	
	// Convert to TUI format
	tuiPackages := make([]tui.PackageInfo, len(packages))
	for i, pkg := range packages {
		tuiPackages[i] = tui.PackageInfo{
			Name: pkg.Name,
			Path: pkg.Path,
		}
	}
	
	// Use TUI to select master repository
	masterAlias, err := tui.RunPackageSelector(tuiPackages)
	if err != nil {
		return fmt.Errorf("failed to select master repository: %w", err)
	}
	
	// Find the selected package to get the correct identifier
	var masterRepo *DartPackage
	for _, pkg := range packages {
		if pkg.Name == masterAlias {
			masterRepo = &pkg
			break
		}
	}
	
	if masterRepo == nil {
		return fmt.Errorf("master repository not found in packages")
	}
	
	// Use alias if set, otherwise use name
	masterIdentifier := masterRepo.Alias
	if masterIdentifier == "" {
		masterIdentifier = masterRepo.Name
	}
	
	// Create alfred configuration
	mainBranch, err := c.createAlfredConfig(packages, masterIdentifier)
	if err != nil {
		return fmt.Errorf("failed to create alfred configuration: %w", err)
	}
	
	fmt.Printf("\n✅ Alfred configured successfully with %d repositories\n", len(packages))
	fmt.Printf("✅ Master repository: %s\n", masterIdentifier)
	fmt.Printf("✅ Main branch: %s\n", mainBranch)
	fmt.Println("✅ You can now use 'alfred switch <context-name>' to create and switch contexts")
	
	return nil
}

type DartPackage struct {
	Name  string
	Alias string
	Path  string
}

func (c *ScanCmd) scanForDartPackages() ([]DartPackage, error) {
	var packages []DartPackage
	
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("failed to read current directory: %w", err)
	}
	
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		
		pubspecPath := filepath.Join(entry.Name(), "pubspec.yaml")
		if _, err := os.Stat(pubspecPath); os.IsNotExist(err) {
			continue
		}
		
		// Read package name from pubspec.yaml
		packageName, err := pubspec.ExtractPackageNameFromFile(pubspecPath)
		if err != nil {
			fmt.Printf("Warning: Could not read package name from %s: %v\n", pubspecPath, err)
			continue
		}
		
		packages = append(packages, DartPackage{
			Name:  packageName,
			Alias: "", // Will be set by user if they want a nickname
			Path:  "./" + entry.Name(),
		})
	}
	
	return packages, nil
}

// promptForMainBranch prompts the user for the main branch name
func promptForMainBranch() (string, error) {
	fmt.Println("\nSet the main branch name:")
	fmt.Println("This branch will be used when running 'alfred switch main'")
	fmt.Print("Enter main branch name (default: main): ")
	
	var branchName string
	fmt.Scanln(&branchName)
	
	if branchName == "" {
		branchName = "main"
	}
	
	return branchName, nil
}

func (c *ScanCmd) createAlfredConfig(packages []DartPackage, masterAlias string) (string, error) {
	// Create .alfred directory
	alfredDir := filepath.Join(".", ".alfred")
	if err := os.MkdirAll(alfredDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .alfred directory: %w", err)
	}
	
	// Get main branch from user
	mainBranch, err := promptForMainBranch()
	if err != nil {
		return "", fmt.Errorf("failed to get main branch: %w", err)
	}
	
	// Create config
	var configContent strings.Builder
	configContent.WriteString("repos:\n")
	for _, pkg := range packages {
		configContent.WriteString(fmt.Sprintf("  - name: %s\n", pkg.Name))
		if pkg.Alias != "" {
			configContent.WriteString(fmt.Sprintf("    alias: %s\n", pkg.Alias))
		}
		configContent.WriteString(fmt.Sprintf("    path: %s\n", pkg.Path))
	}
	
	configContent.WriteString(fmt.Sprintf("\nmaster: %s\n", masterAlias))
	configContent.WriteString("mode: worktree\n")
	configContent.WriteString(fmt.Sprintf("main_branch: %s\n", mainBranch))
	configContent.WriteString("\ncontexts: {}\n")
	
	configPath := filepath.Join(alfredDir, "alfred.yaml")
	if err := os.WriteFile(configPath, []byte(configContent.String()), 0644); err != nil {
		return "", fmt.Errorf("failed to write alfred.yaml: %w", err)
	}
	
	// Update .gitignore
	if err := c.updateGitignore(); err != nil {
		fmt.Printf("⚠️  Warning: failed to update .gitignore: %v\n", err)
		fmt.Println("Please manually add '.alfred/' to your .gitignore file")
	} else {
		fmt.Println("✅ Updated .gitignore to ignore .alfred directory")
	}
	
	return mainBranch, nil
}

func (c *ScanCmd) updateGitignore() error {
	gitignorePath := ".gitignore"
	alfredIgnoreEntry := ".alfred/"

	// Read existing .gitignore if it exists
	var existingContent string
	if data, err := os.ReadFile(gitignorePath); err == nil {
		existingContent = string(data)
		// Check if .alfred is already in .gitignore
		if strings.Contains(existingContent, alfredIgnoreEntry) {
			return nil // Already exists, nothing to do
		}
	}

	// Append .alfred/ to .gitignore
	file, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open .gitignore: %w", err)
	}
	defer file.Close()

	// Add newline before if file exists and doesn't end with newline
	if existingContent != "" && !strings.HasSuffix(existingContent, "\n") {
		if _, err := file.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write to .gitignore: %w", err)
		}
	}

	// Add alfred ignore entry with comment
	ignoreEntry := "# Alfred CLI state and configuration\n.alfred/\n"
	if _, err := file.WriteString(ignoreEntry); err != nil {
		return fmt.Errorf("failed to write to .gitignore: %w", err)
	}

	return nil
}

type InitCmd struct{}

func (c *InitCmd) Run(ctx *kong.Context) error {
	fmt.Println("Initializing alfred...")
	
	// Check if .alfred directory already exists
	alfredDir := filepath.Join(".", ".alfred")
	configPath := filepath.Join(alfredDir, "alfred.yaml")
	
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		return fmt.Errorf("alfred is already initialized (.alfred/alfred.yaml exists)")
	}

	// Ask user if they want to scan for existing packages
	fmt.Println("\nChoose initialization method:")
	fmt.Println("  1. Scan directory for existing Dart/Flutter packages (recommended)")
	fmt.Println("  2. Create with sample configuration")
	fmt.Print("Enter your choice (1 or 2): ")
	
	var choice string
	fmt.Scanln(&choice)
	
	if choice == "1" {
		// Use scan functionality
		scanCmd := &ScanCmd{}
		return scanCmd.Run(ctx)
	}

	// Create .alfred directory
	if err := os.MkdirAll(alfredDir, 0755); err != nil {
		return fmt.Errorf("failed to create .alfred directory: %w", err)
	}
	fmt.Println("✅ Created .alfred directory")

	// Get main branch from user
	mainBranch, err := promptForMainBranch()
	if err != nil {
		return fmt.Errorf("failed to get main branch: %w", err)
	}

	// Create sample config with user's main branch
	sampleConfig := fmt.Sprintf(`repos:
  - name: core
    path: ./core
  - name: ui
    path: ./ui
  - name: app
    path: ./app

master: app
mode: worktree
main_branch: %s

contexts:
  feature-1:
    - ui
    - app
  feature-2:
    - ui
    - app
    - core
`, mainBranch)

	if err := os.WriteFile(configPath, []byte(sampleConfig), 0644); err != nil {
		return fmt.Errorf("failed to create alfred.yaml: %w", err)
	}
	fmt.Println("✅ Created .alfred/alfred.yaml")

	// Update .gitignore
	if err := c.updateGitignore(); err != nil {
		fmt.Printf("⚠️  Warning: failed to update .gitignore: %v\n", err)
		fmt.Println("Please manually add '.alfred/' to your .gitignore file")
	} else {
		fmt.Println("✅ Updated .gitignore to ignore .alfred directory")
	}

	fmt.Println()
	fmt.Println("✅ Alfred initialized with sample configuration")
	fmt.Printf("✅ Main branch: %s\n", mainBranch)
	fmt.Println("Edit .alfred/alfred.yaml to configure your repositories and contexts.")
	return nil
}

func (c *InitCmd) updateGitignore() error {
	gitignorePath := ".gitignore"
	alfredIgnoreEntry := ".alfred/"

	// Read existing .gitignore if it exists
	var existingContent string
	if data, err := os.ReadFile(gitignorePath); err == nil {
		existingContent = string(data)
		// Check if .alfred is already in .gitignore
		if strings.Contains(existingContent, alfredIgnoreEntry) {
			return nil // Already exists, nothing to do
		}
	}

	// Append .alfred/ to .gitignore
	file, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open .gitignore: %w", err)
	}
	defer file.Close()

	// Add newline before if file exists and doesn't end with newline
	if existingContent != "" && !strings.HasSuffix(existingContent, "\n") {
		if _, err := file.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write to .gitignore: %w", err)
		}
	}

	// Add alfred ignore entry with comment
	ignoreEntry := "# Alfred CLI state and configuration\n.alfred/\n"
	if _, err := file.WriteString(ignoreEntry); err != nil {
		return fmt.Errorf("failed to write to .gitignore: %w", err)
	}

	return nil
}

type StatusCmd struct{}

func (c *StatusCmd) Run(ctx *kong.Context) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	manager := context.NewManager(cfg)
	currentContext, repoStatus, err := manager.GetContextStatus()
	if err != nil {
		return fmt.Errorf("failed to get context status: %w", err)
	}

	fmt.Println("Alfred Project Status")
	fmt.Println("====================")
	fmt.Println()

	if currentContext == "" {
		fmt.Println("No context is currently active.")
		fmt.Println("Use 'alfred switch' to activate a context.")
		return nil
	}

	fmt.Printf("Current Context: %s\n", currentContext)
	fmt.Println()

	if len(repoStatus) == 0 {
		fmt.Println("No repositories in current context.")
		return nil
	}

	fmt.Println("Repository Status:")
	for repo, status := range repoStatus {
		fmt.Printf("  %s: %s\n", repo, status)
	}

	return nil
}

type ListCmd struct{}

func (c *ListCmd) Run(ctx *kong.Context) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	manager := context.NewManager(cfg)
	contexts := manager.ListContexts()

	if len(contexts) == 0 {
		fmt.Println("No contexts defined in alfred.yaml")
		return nil
	}

	fmt.Println("Available contexts:")
	currentContext, _ := manager.GetCurrentContext()
	
	for _, contextName := range contexts {
		if contextName == "main" {
			if contextName == currentContext {
				fmt.Printf("● %s (current) - main/master branches for all repos\n", contextName)
			} else {
				fmt.Printf("  %s - main/master branches for all repos\n", contextName)
			}
		} else if contextName == currentContext {
			fmt.Printf("● %s (current)\n", contextName)
		} else {
			fmt.Printf("  %s\n", contextName)
		}
	}

	return nil
}

type SwitchCmd struct {
	Context string `arg:"" help:"Context name to switch to" optional:"true"`
}

func (c *SwitchCmd) Run(ctx *kong.Context) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	manager := context.NewManager(cfg)
	contexts := manager.ListContexts()

	var targetContext string
	
	if c.Context != "" {
		found := false
		for _, ctx := range contexts {
			if ctx == c.Context {
				found = true
				break
			}
		}
		
		if !found {
			// Context doesn't exist - offer to create it (unless it's a reserved name)
			if c.Context == "main" || c.Context == "master" {
				return fmt.Errorf("'%s' is a built-in context that should already be available", c.Context)
			}
			
			fmt.Printf("Context '%s' not found.\n", c.Context)
			fmt.Printf("Would you like to create it? (y/N): ")
			
			var response string
			fmt.Scanln(&response)
			
			if strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
				if err := c.createNewContext(cfg, c.Context); err != nil {
					return fmt.Errorf("failed to create context: %w", err)
				}
				fmt.Printf("✅ Created context '%s'\n", c.Context)
			} else {
				if len(contexts) > 0 {
					fmt.Println("\nAvailable contexts:")
					for _, ctx := range contexts {
						fmt.Printf("  %s\n", ctx)
					}
				}
				return nil
			}
		}
		targetContext = c.Context
	} else {
		if len(contexts) == 0 {
			return fmt.Errorf("no contexts defined in alfred.yaml. Use 'alfred create' to create a context")
		}

		// Try to use TUI, but fallback to showing available contexts if no TTY
		currentContext, _ := manager.GetCurrentContext()
		selectedContext, err := tui.RunContextSelector(contexts, currentContext)
		if err != nil {
			// If TTY error, show available contexts and prompt user to specify one
			if strings.Contains(err.Error(), "TTY") || strings.Contains(err.Error(), "tty") {
				fmt.Println("Available contexts:")
				for _, ctx := range contexts {
					if ctx == currentContext {
						fmt.Printf("● %s (current)\n", ctx)
					} else {
						fmt.Printf("  %s\n", ctx)
					}
				}
				fmt.Println("\nUsage: alfred switch <context-name>")
				return nil
			}
			return err
		}
		
		if selectedContext == "" {
			fmt.Println("No context selected.")
			return nil
		}
		
		targetContext = selectedContext
	}

	if err := manager.SwitchContext(targetContext); err != nil {
		return fmt.Errorf("failed to switch context: %w", err)
	}

	if targetContext == "main" || targetContext == "master" {
		fmt.Printf("✅ Switched to main context - all repositories on main/master branches\n")
	} else {
		fmt.Printf("✅ Switched to context '%s'\n", targetContext)
	}
	return nil
}

func (c *SwitchCmd) createNewContext(cfg *config.Config, contextName string) error {
	if len(cfg.Repos) == 0 {
		return fmt.Errorf("no repositories configured in alfred.yaml")
	}

	repoAliases := cfg.GetRepoAliases()
	repoPaths := cfg.GetRepoPaths()

	fmt.Printf("\nSelect repositories for context '%s':\n", contextName)
	selectedRepos, err := tui.RunRepoSelector(repoAliases, repoPaths)
	if err != nil {
		// If TTY error, fallback to interactive selection
		if strings.Contains(err.Error(), "TTY") || strings.Contains(err.Error(), "tty") {
			selectedRepos, err = c.interactiveRepoSelection(repoAliases)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	if len(selectedRepos) == 0 {
		return fmt.Errorf("no repositories selected")
	}

	// Add context to config
	if err := cfg.AddContext(contextName, selectedRepos); err != nil {
		return fmt.Errorf("failed to add context: %w", err)
	}

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

func (c *SwitchCmd) interactiveRepoSelection(repoAliases []string) ([]string, error) {
	fmt.Println("Available repositories:")
	for i, alias := range repoAliases {
		fmt.Printf("  %d. %s\n", i+1, alias)
	}
	
	fmt.Printf("Enter repository numbers (comma-separated, e.g., 1,2,3): ")
	var input string
	fmt.Scanln(&input)
	
	if input == "" {
		return nil, fmt.Errorf("no repositories selected")
	}
	
	// Parse the input
	parts := strings.Split(strings.ReplaceAll(input, " ", ""), ",")
	var selectedRepos []string
	
	for _, part := range parts {
		if part == "" {
			continue
		}
		
		var index int
		if _, err := fmt.Sscanf(part, "%d", &index); err != nil {
			fmt.Printf("Invalid input: %s\n", part)
			continue
		}
		
		if index < 1 || index > len(repoAliases) {
			fmt.Printf("Invalid repository number: %d\n", index)
			continue
		}
		
		selectedRepos = append(selectedRepos, repoAliases[index-1])
	}
	
	if len(selectedRepos) == 0 {
		return nil, fmt.Errorf("no valid repositories selected")
	}
	
	return selectedRepos, nil
}

type CreateCmd struct{}

func (c *CreateCmd) Run(ctx *kong.Context) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	if len(cfg.Repos) == 0 {
		return fmt.Errorf("no repositories configured in alfred.yaml")
	}

	repoAliases := cfg.GetRepoAliases()
	repoPaths := cfg.GetRepoPaths()

	contextName, selectedRepos, err := tui.RunContextCreator(repoAliases, repoPaths)
	if err != nil {
		return err
	}

	// Check if trying to create reserved context names
	if contextName == "main" || contextName == "master" {
		return fmt.Errorf("cannot create context with reserved name '%s' - this is a built-in context", contextName)
	}

	// Check if context already exists
	if cfg.ContextExists(contextName) {
		return fmt.Errorf("context '%s' already exists", contextName)
	}

	// Add context to config
	if err := cfg.AddContext(contextName, selectedRepos); err != nil {
		return fmt.Errorf("failed to add context: %w", err)
	}

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✅ Created context '%s' with repositories: %s\n", 
		contextName, strings.Join(selectedRepos, ", "))
	
	return nil
}

type DeleteCmd struct {
	Contexts []string `arg:"" help:"Context names to delete" optional:"true"`
}

func (c *DeleteCmd) Run(ctx *kong.Context) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	manager := context.NewManager(cfg)
	allContexts := manager.ListContexts()

	if len(allContexts) == 0 {
		fmt.Println("No contexts available to delete.")
		return nil
	}

	var targetContexts []string

	if len(c.Contexts) > 0 {
		// Validate specified contexts exist and prevent deletion of main context
		for _, contextName := range c.Contexts {
			if contextName == "main" || contextName == "master" {
				return fmt.Errorf("cannot delete built-in main context")
			}
			
			found := false
			for _, ctx := range allContexts {
				if ctx == contextName {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("context '%s' not found", contextName)
			}
		}
		targetContexts = c.Contexts
	} else {
		// Use TUI to select contexts
		currentContext, _ := manager.GetCurrentContext()
		selectedContexts, err := tui.RunContextDeleter(allContexts, currentContext)
		if err != nil {
			// If TTY error, show available contexts and prompt user to specify them
			if strings.Contains(err.Error(), "TTY") || strings.Contains(err.Error(), "tty") {
				fmt.Println("Available contexts:")
				for _, ctx := range allContexts {
					if ctx == currentContext {
						fmt.Printf("● %s (current - cannot delete)\n", ctx)
					} else {
						fmt.Printf("  %s\n", ctx)
					}
				}
				fmt.Println("\nUsage: alfred delete <context-name> [<context-name>...]")
				return nil
			}
			return err
		}

		if len(selectedContexts) == 0 {
			fmt.Println("No contexts selected for deletion.")
			return nil
		}

		targetContexts = selectedContexts
	}

	// Perform deletion
	if err := manager.DeleteContexts(targetContexts); err != nil {
		return fmt.Errorf("failed to delete contexts: %w", err)
	}

	fmt.Printf("✅ Successfully deleted contexts: %s\n", strings.Join(targetContexts, ", "))
	return nil
}

type PrepareCmd struct {
	Repository string `arg:"" help:"Repository to prepare (alias or name). If not specified, prepares current master repository" optional:"true"`
}

func (c *PrepareCmd) Run(ctx *kong.Context) error {
	logger := log.Default()
	
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	var targetRepo *config.Repository

	if c.Repository != "" {
		// User specified a repository
		targetRepo, err = cfg.GetRepoByAlias(c.Repository)
		if err != nil {
			return fmt.Errorf("repository '%s' not found", c.Repository)
		}
	} else {
		// Use master repository if configured
		targetRepo, err = cfg.GetMasterRepo()
		if err != nil {
			return fmt.Errorf("no master repository configured and no repository specified")
		}
	}

	// Load pubspec.yaml from the target repository
	pubspecFile, err := pubspec.LoadPubspec(targetRepo.Path)
	if err != nil {
		return fmt.Errorf("failed to load pubspec.yaml from %s: %w", targetRepo.Path, err)
	}

	repoIdentifier := targetRepo.Alias
	if repoIdentifier == "" {
		repoIdentifier = targetRepo.Name
	}

	fmt.Printf("Preparing %s for production by reverting to git dependencies...\n", repoIdentifier)

	// Get all dependencies that might need to be reverted
	// Check for dependencies with commented git configuration
	dependenciesReverted := 0
	
	// Get all repositories from config to check for dependencies
	allRepos := cfg.Repos
	for _, repo := range allRepos {
		dependencyName := repo.Name
		
		// Try to uncomment git dependency and remove path
		if err := pubspecFile.UncommentGitDependencyAndRemovePath(dependencyName); err != nil {
			logger.Debugf("No commented git dependency found for %s in %s: %v", 
				dependencyName, repoIdentifier, err)
		} else {
			dependenciesReverted++
			fmt.Printf("  ✅ Reverted %s dependency to git reference\n", dependencyName)
		}
	}
	
	if dependenciesReverted == 0 {
		fmt.Printf("⚠️  No dependencies to revert in %s. Repository may already be prepared.\n", repoIdentifier)
		return nil
	}

	// Save the changes
	if err := pubspecFile.Save(); err != nil {
		return fmt.Errorf("failed to save pubspec.yaml: %w", err)
	}

	fmt.Printf("✅ Successfully prepared %s - all dependencies reverted to git references\n", repoIdentifier)
	fmt.Printf("✅ Repository is now ready for production deployment\n")

	// Optionally run flutter pub get
	fmt.Print("Run 'flutter pub get' to update dependencies? (y/N): ")
	var response string
	fmt.Scanln(&response)

	if strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
		cmd := exec.Command("flutter", "pub", "get")
		cmd.Dir = targetRepo.Path
		
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("⚠️  flutter pub get failed: %v\nOutput: %s\n", err, string(output))
		} else {
			fmt.Println("✅ Dependencies updated successfully")
		}
	}

	return nil
}

type MainBranchCmd struct {
	BranchName string `arg:"" help:"Branch name to set as main branch" optional:"true"`
}

func (c *MainBranchCmd) Run(ctx *kong.Context) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	var branchName string

	if c.BranchName != "" {
		// Branch name provided as argument
		branchName = c.BranchName
	} else {
		// No branch name provided, use TUI to get input
		inputBranch, err := tui.RunMainBranchInput()
		if err != nil {
			// If TUI fails (no TTY), ask for input via prompt
			if strings.Contains(err.Error(), "TTY") || strings.Contains(err.Error(), "tty") {
				fmt.Printf("Enter the main branch name (default: main): ")
				fmt.Scanln(&branchName)
				if branchName == "" {
					branchName = "main"
				}
			} else {
				return fmt.Errorf("failed to get main branch input: %w", err)
			}
		} else {
			branchName = inputBranch
		}
	}

	// Validate branch name
	if branchName == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	// Set the main branch in config
	if err := cfg.SetMainBranch(branchName); err != nil {
		return fmt.Errorf("failed to set main branch: %w", err)
	}

	fmt.Printf("✅ Main branch set to: %s\n", branchName)
	fmt.Printf("Now 'alfred switch main' will switch all repositories to the '%s' branch\n", branchName)

	return nil
}

type VersionCmd struct{}

func (c *VersionCmd) Run(ctx *kong.Context) error {
	fmt.Println("alfred v1.0.0")
	return nil
}

func Execute() {
	ctx := kong.Parse(&CLI,
		kong.Name("alfred"),
		kong.Description("A CLI tool for managing multi-repo Flutter/Dart projects"),
		kong.UsageOnError(),
	)

	if CLI.Debug {
		log.SetLevel(log.DebugLevel)
	}

	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}