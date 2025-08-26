package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Repos      []Repository        `yaml:"repos"`
	Master     string              `yaml:"master"`
	Mode       string              `yaml:"mode"`
	MainBranch string              `yaml:"main_branch,omitempty"`
	Contexts   map[string][]string `yaml:"contexts"`
}

type Repository struct {
	Name  string `yaml:"name"`
	Alias string `yaml:"alias,omitempty"`
	Path  string `yaml:"path"`
}

const (
	ConfigFileName = "alfred.yaml"
	AlfredDir      = ".alfred"

	ModeBranch   = "branch"
	ModeWorktree = "worktree"
	DefaultMode  = ModeWorktree
)

func getAlfredDir() string {
	return filepath.Join(".", AlfredDir)
}

func getConfigPath() string {
	return filepath.Join(getAlfredDir(), ConfigFileName)
}

func ensureAlfredDir() error {
	alfredDir := getAlfredDir()
	if _, err := os.Stat(alfredDir); os.IsNotExist(err) {
		if err := os.MkdirAll(alfredDir, 0755); err != nil {
			return fmt.Errorf("failed to create .alfred directory: %w", err)
		}
	}
	return nil
}

func LoadConfig() (*Config, error) {
	configPath := getConfigPath()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("alfred.yaml not found in .alfred directory. Run 'alfred init' to initialize")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set default mode if not specified
	if config.Mode == "" {
		config.Mode = DefaultMode
	}

	// Validate mode
	if config.Mode != ModeBranch && config.Mode != ModeWorktree {
		return nil, fmt.Errorf("invalid mode '%s'. Must be 'branch' or 'worktree'", config.Mode)
	}

	// Set default main branch if not specified
	if config.MainBranch == "" {
		config.MainBranch = "main"
	}

	return &config, nil
}

func (c *Config) Save() error {
	if err := ensureAlfredDir(); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := getConfigPath()
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (c *Config) GetRepoByAlias(alias string) (*Repository, error) {
	for _, repo := range c.Repos {
		// Check alias first (if set), then name
		if (repo.Alias != "" && repo.Alias == alias) || (repo.Alias == "" && repo.Name == alias) {
			return &repo, nil
		}
	}
	return nil, fmt.Errorf("repository with alias '%s' not found", alias)
}

func (c *Config) GetContextRepos(contextName string) ([]*Repository, error) {
	// Handle special "main" or "master" context - includes all repositories
	if contextName == "main" || contextName == "master" {
		var repos []*Repository
		for i := range c.Repos {
			repos = append(repos, &c.Repos[i])
		}
		return repos, nil
	}

	aliases, exists := c.Contexts[contextName]
	if !exists {
		return nil, fmt.Errorf("context '%s' not found", contextName)
	}

	var repos []*Repository
	for _, alias := range aliases {
		repo, err := c.GetRepoByAlias(alias)
		if err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}

	return repos, nil
}

func (c *Config) GetContextNames() []string {
	var names []string

	// Always include the built-in "main" context first
	names = append(names, "main")

	for name := range c.Contexts {
		names = append(names, name)
	}
	return names
}

func (c *Config) AddContext(name string, repoAliases []string) error {
	if c.Contexts == nil {
		c.Contexts = make(map[string][]string)
	}

	// Validate that all repo aliases exist
	for _, alias := range repoAliases {
		found := false
		for _, repo := range c.Repos {
			// Check both alias and name
			if (repo.Alias != "" && repo.Alias == alias) || (repo.Alias == "" && repo.Name == alias) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("repository alias '%s' not found", alias)
		}
	}

	c.Contexts[name] = repoAliases
	return nil
}

func (c *Config) ContextExists(name string) bool {
	_, exists := c.Contexts[name]
	return exists
}

func (c *Config) RemoveContext(name string) error {
	// Check if context exists
	if !c.ContextExists(name) {
		return fmt.Errorf("context '%s' does not exist", name)
	}

	// Prevent removal of built-in contexts
	if name == "main" || name == "master" {
		return fmt.Errorf("cannot remove built-in context '%s'", name)
	}

	// Remove the context
	delete(c.Contexts, name)
	return nil
}

func (c *Config) IsContextContainsMaster(contextName string) bool {
	if c.Master == "" {
		return false
	}

	// Handle special "main" context
	if contextName == "main" || contextName == "master" {
		return true
	}

	// Check if master alias is in the context's repository list
	contextRepos, exists := c.Contexts[contextName]
	if !exists {
		return false
	}

	for _, alias := range contextRepos {
		if alias == c.Master {
			return true
		}
	}

	return false
}

func (c *Config) GetRepoAliases() []string {
	aliases := make([]string, len(c.Repos))
	for i, repo := range c.Repos {
		// Return alias if set, otherwise return name
		if repo.Alias != "" {
			aliases[i] = repo.Alias
		} else {
			aliases[i] = repo.Name
		}
	}
	return aliases
}

func (c *Config) GetRepoPaths() []string {
	paths := make([]string, len(c.Repos))
	for i, repo := range c.Repos {
		paths[i] = repo.Path
	}
	return paths
}

func (c *Config) GetMasterRepo() (*Repository, error) {
	if c.Master == "" {
		return nil, fmt.Errorf("no master repository configured")
	}
	return c.GetRepoByAlias(c.Master)
}

func (c *Config) GetNonMasterReposForContext(contextName string) ([]*Repository, error) {
	contextRepos, err := c.GetContextRepos(contextName)
	if err != nil {
		return nil, err
	}

	var nonMasterRepos []*Repository
	for _, repo := range contextRepos {
		repoIdentifier := repo.Alias
		if repoIdentifier == "" {
			repoIdentifier = repo.Name
		}
		if repoIdentifier != c.Master {
			nonMasterRepos = append(nonMasterRepos, repo)
		}
	}

	return nonMasterRepos, nil
}

func (c *Config) IsBranchMode() bool {
	return c.Mode == ModeBranch
}

func (c *Config) IsWorktreeMode() bool {
	return c.Mode == ModeWorktree
}

// GetMainBranch returns the configured main branch name
func (c *Config) GetMainBranch() string {
	if c.MainBranch == "" {
		return "main"
	}
	return c.MainBranch
}

// SetMainBranch sets the main branch name and saves the config
func (c *Config) SetMainBranch(branchName string) error {
	c.MainBranch = branchName
	return c.Save()
}
