package git

import (
	"os"
	"path/filepath"
	"testing"
)

// findGitRoot walks up the directory tree to find the git repository root
func findGitRoot(startDir string) (string, bool) {
	currentDir := startDir
	for {
		if _, err := os.Stat(filepath.Join(currentDir, ".git")); err == nil {
			return currentDir, true
		}
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			return "", false
		}
		currentDir = parent
	}
}

func TestGitRepo_HasUpstream(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	gitRoot, found := findGitRoot(wd)
	if !found {
		t.Skip("Not in a git repository, skipping test")
	}

	repo := NewGitRepo(gitRoot)

	// Test HasUpstream - this will vary depending on the actual repo state
	hasUpstream, err := repo.HasUpstream()
	if err != nil {
		t.Logf("HasUpstream returned error (this may be expected): %v", err)
	} else {
		t.Logf("HasUpstream result: %v", hasUpstream)
	}
}

func TestGitRepo_GetCurrentBranch(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	gitRoot, found := findGitRoot(wd)
	if !found {
		t.Skip("Not in a git repository, skipping test")
	}

	repo := NewGitRepo(gitRoot)

	branch, err := repo.GetCurrentBranch()
	if err != nil {
		t.Fatalf("Failed to get current branch: %v", err)
	}

	if branch == "" {
		t.Error("Current branch should not be empty")
	}

	t.Logf("Current branch: %s", branch)
}

func TestGitRepo_IsGitRepo(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	gitRoot, found := findGitRoot(wd)
	if !found {
		t.Skip("Not in a git repository, skipping test")
	}

	repo := NewGitRepo(gitRoot)

	isRepo := repo.IsGitRepo()
	if !isRepo {
		t.Error("Expected project root to be a git repository")
	}

	// Test with a non-git directory
	tempDir := t.TempDir()
	nonGitRepo := NewGitRepo(tempDir)

	isNonRepo := nonGitRepo.IsGitRepo()
	if isNonRepo {
		t.Error("Expected temp directory to not be a git repository")
	}
}
