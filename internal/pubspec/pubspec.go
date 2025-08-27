package pubspec

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type PubspecYaml struct {
	content string
	path    string
}

type GitDependency struct {
	URL string `yaml:"url"`
	Ref string `yaml:"ref"`
}

func LoadPubspec(repoPath string) (*PubspecYaml, error) {
	pubspecPath := filepath.Join(repoPath, "pubspec.yaml")

	data, err := os.ReadFile(pubspecPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pubspec.yaml: %w", err)
	}

	return &PubspecYaml{
		content: string(data),
		path:    pubspecPath,
	}, nil
}

func (p *PubspecYaml) Save() error {
	if err := os.WriteFile(p.path, []byte(p.content), 0644); err != nil {
		return fmt.Errorf("failed to write pubspec.yaml: %w", err)
	}
	return nil
}

func (p *PubspecYaml) ConvertGitToPath(depName, localPath string) error {
	// Pattern to find git dependency block for the specified dependency
	gitPattern := regexp.MustCompile(`(?m)^(\s*)` + regexp.QuoteMeta(depName) + `:\s*\n(\s+)git:\s*\n(\s+url:.*\n)(\s+ref:.*\n)?`)

	if !gitPattern.MatchString(p.content) {
		return fmt.Errorf("dependency '%s' is not a git dependency", depName)
	}

	// Replace git dependency with path dependency
	replacement := fmt.Sprintf("${1}%s:\n${2}path: %s\n", depName, localPath)
	p.content = gitPattern.ReplaceAllString(p.content, replacement)

	return nil
}

func (p *PubspecYaml) ConvertPathToGit(depName, gitUrl, gitRef string) error {
	// Pattern to find path dependency for the specified dependency
	pathPattern := regexp.MustCompile(`(?m)^(\s*)` + regexp.QuoteMeta(depName) + `:\s*\n(\s+)path:.*\n`)

	if !pathPattern.MatchString(p.content) {
		return fmt.Errorf("dependency '%s' is not a path dependency", depName)
	}

	// Replace path dependency with git dependency
	replacement := fmt.Sprintf("${1}%s:\n${2}git:\n${2}  url: %s\n${2}  ref: %s\n", depName, gitUrl, gitRef)
	p.content = pathPattern.ReplaceAllString(p.content, replacement)

	return nil
}

func (p *PubspecYaml) BackupOriginal() error {
	backupPath := p.path + ".backup"

	data, err := os.ReadFile(p.path)
	if err != nil {
		return fmt.Errorf("failed to read original pubspec: %w", err)
	}

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	return nil
}

func (p *PubspecYaml) RestoreFromBackup() error {
	backupPath := p.path + ".backup"

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found")
	}

	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	if err := os.WriteFile(p.path, data, 0644); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	return nil
}

func (p *PubspecYaml) GetGitDependencies() map[string]*GitDependency {
	gitDeps := make(map[string]*GitDependency)

	// Pattern to find git dependencies
	gitPattern := regexp.MustCompile(`(?m)^(\s*)(\w+):\s*\n(\s+)git:\s*\n(\s+)url:\s*(.+)\n(?:(\s+)ref:\s*(.+)\n)?`)

	matches := gitPattern.FindAllStringSubmatch(p.content, -1)
	for _, match := range matches {
		if len(match) >= 6 {
			depName := match[2]
			url := strings.TrimSpace(match[5])
			ref := ""
			if len(match) >= 8 && match[7] != "" {
				ref = strings.TrimSpace(match[7])
			}

			gitDeps[depName] = &GitDependency{
				URL: url,
				Ref: ref,
			}
		}
	}

	return gitDeps
}

func ExtractRepoNameFromGitURL(gitURL string) string {
	re := regexp.MustCompile(`([^/]+?)(?:\.git)?$`)
	matches := re.FindStringSubmatch(gitURL)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (p *PubspecYaml) ConvertPathToGitFromBackup(depName string) error {
	// First, try to get the git dependency info from backup if it exists
	backupPath := p.path + ".backup"
	if _, err := os.Stat(backupPath); err == nil {
		// Load backup to get original git dependency info
		backupData, err := os.ReadFile(backupPath)
		if err == nil {
			backupPubspec := &PubspecYaml{content: string(backupData)}
			gitDeps := backupPubspec.GetGitDependencies()
			if gitDep, exists := gitDeps[depName]; exists {
				return p.ConvertPathToGit(depName, gitDep.URL, gitDep.Ref)
			}
		}
	}

	// If no backup or dependency not found in backup, return error
	return fmt.Errorf("could not find git dependency info for '%s' in backup", depName)
}

func (p *PubspecYaml) UpdatePathDependency(depName, newPath string) error {
	// Pattern to find path dependency block for the specified dependency
	pathPattern := regexp.MustCompile(`(?m)^(\s*)` + regexp.QuoteMeta(depName) + `:\s*\n(\s+)path:\s*(.+)\n`)

	if !pathPattern.MatchString(p.content) {
		return fmt.Errorf("dependency '%s' is not a path dependency", depName)
	}

	// Replace the path with the new path
	replacement := fmt.Sprintf("${1}%s:\n${2}path: %s\n", depName, newPath)
	p.content = pathPattern.ReplaceAllString(p.content, replacement)

	return nil
}

// GetPackageName extracts the package name from pubspec.yaml content
func (p *PubspecYaml) GetPackageName() (string, error) {
	// Pattern to find the name field
	namePattern := regexp.MustCompile(`(?m)^name:\s*(.+)$`)
	matches := namePattern.FindStringSubmatch(p.content)

	if len(matches) < 2 {
		return "", fmt.Errorf("package name not found in pubspec.yaml")
	}

	name := strings.TrimSpace(matches[1])
	// Remove quotes if present
	name = strings.Trim(name, "\"'")

	return name, nil
}

// ExtractPackageNameFromFile extracts package name directly from a pubspec.yaml file path
func ExtractPackageNameFromFile(pubspecPath string) (string, error) {
	pubspec, err := LoadPubspec(filepath.Dir(pubspecPath))
	if err != nil {
		return "", err
	}
	return pubspec.GetPackageName()
}

// CommentGitDependencyAndAddPath comments out git dependency and adds path dependency
func (p *PubspecYaml) CommentGitDependencyAndAddPath(depName, localPath string) error {
	// Pattern to find git dependency block for the specified dependency
	gitPattern := regexp.MustCompile(`(?m)^(\s*)` + regexp.QuoteMeta(depName) + `:\s*\n(\s+)git:\s*\n(\s+url:.*\n)(\s+ref:.*\n)?`)

	if !gitPattern.MatchString(p.content) {
		return fmt.Errorf("dependency '%s' is not a git dependency", depName)
	}

	// Replace git dependency with commented git + new path
	replacement := func(match string) string {
		lines := strings.Split(strings.TrimSuffix(match, "\n"), "\n")
		var result strings.Builder

		// Add path dependency first
		result.WriteString(fmt.Sprintf("%s:\n", depName))
		result.WriteString(fmt.Sprintf("    path: %s\n", localPath))

		// Comment out the original git dependency
		for _, line := range lines {
			result.WriteString("  # ")
			result.WriteString(line)
			result.WriteString("\n")
		}

		return result.String()
	}

	p.content = gitPattern.ReplaceAllStringFunc(p.content, replacement)
	return nil
}

// UncommentGitDependencyAndRemovePath uncomments git dependency and removes path dependency
func (p *PubspecYaml) UncommentGitDependencyAndRemovePath(depName string) error {
	// Pattern to find the path dependency followed by commented git dependency
	pathAndCommentedGitPattern := regexp.MustCompile(`(?ms)^(\s*)` + regexp.QuoteMeta(depName) + `:\s*\n\s*path:.*\n(\s*#\s*` + regexp.QuoteMeta(depName) + `:\s*\n\s*#\s*git:\s*\n(\s*#\s*url:.*\n)(\s*#\s*ref:.*\n)?)`)

	match := pathAndCommentedGitPattern.FindStringSubmatch(p.content)
	if len(match) == 0 {
		return fmt.Errorf("dependency '%s' pattern not found", depName)
	}

	// Build the restored git dependency
	var gitDep strings.Builder
	gitDep.WriteString(fmt.Sprintf("%s:\n", depName))
	gitDep.WriteString("  git:\n")

	// Extract and restore URL line
	urlLine := match[3] // url line with # prefix
	// Remove leading # and whitespace, but preserve the actual content
	cleanUrl := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(urlLine), "#"))
	gitDep.WriteString(fmt.Sprintf("    %s\n", cleanUrl))

	// Extract and restore ref line if exists
	if len(match) > 4 && match[4] != "" {
		refLine := match[4] // ref line with # prefix
		cleanRef := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(refLine), "#"))
		gitDep.WriteString(fmt.Sprintf("    %s\n", cleanRef))
	}

	// Replace the entire block with just the git dependency
	p.content = pathAndCommentedGitPattern.ReplaceAllString(p.content, gitDep.String())
	return nil
}
