package discover

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
)

// SubRepoDetector detects and analyzes git repositories within config directories.
//
// These are repositories like ~/.config/nvim that are managed separately from
// the main dotstate repository. Instead of tracking their contents as files,
// dotstate tracks them as references (URL + branch) and can clone/update them
// during apply.
type SubRepoDetector struct{}

// NewSubRepoDetector creates a new sub-repository detector.
func NewSubRepoDetector() *SubRepoDetector {
	return &SubRepoDetector{}
}

// IsSubRepo returns true if the directory contains a .git directory or file.
func (d *SubRepoDetector) IsSubRepo(path string) bool {
	gitPath := filepath.Join(path, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	// .git can be a directory (normal repo) or a file (worktree/submodule)
	return info.IsDir() || info.Mode().IsRegular()
}

// Analyze extracts information about a sub-repository.
func (d *SubRepoDetector) Analyze(ctx context.Context, path, home string) (*Candidate, error) {
	if !d.IsSubRepo(path) {
		return nil, nil
	}

	candidate := &Candidate{
		Path:      path,
		RelPath:   relPath(path, home),
		IsDir:     true,
		IsSubRepo: true,
		Category:  CategoryRecommended,
		Score:     100,
		Reasons:   []string{"git sub-repository"},
	}

	// Try to get the remote URL
	remoteURL, err := d.getRemoteURL(path)
	if err == nil && remoteURL != "" {
		candidate.SubRepoURL = remoteURL
		candidate.Reasons = append(candidate.Reasons, "has remote: "+remoteURL)
	} else {
		// No remote - this is a local-only repo
		candidate.Category = CategoryMaybe
		candidate.Score = 50
		candidate.Reasons = append(candidate.Reasons, "local-only repository (no remote)")
	}

	// Try to get the current branch
	branch, err := d.getCurrentBranch(path)
	if err == nil && branch != "" {
		candidate.SubRepoBranch = branch
	}

	return candidate, nil
}

// getRemoteURL reads the origin remote URL from git config.
func (d *SubRepoDetector) getRemoteURL(repoPath string) (string, error) {
	configPath := filepath.Join(repoPath, ".git", "config")

	// Handle worktrees/submodules where .git is a file
	gitPath := filepath.Join(repoPath, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return "", err
	}
	if info.Mode().IsRegular() {
		// .git is a file, read the gitdir from it
		content, err := os.ReadFile(gitPath)
		if err != nil {
			return "", err
		}
		// Format: "gitdir: /path/to/git/dir"
		line := strings.TrimSpace(string(content))
		if strings.HasPrefix(line, "gitdir:") {
			gitDir := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
			if !filepath.IsAbs(gitDir) {
				gitDir = filepath.Join(repoPath, gitDir)
			}
			configPath = filepath.Join(gitDir, "config")
		}
	}

	// Parse the git config file
	file, err := os.Open(configPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Simple parsing - look for [remote "origin"] section
	scanner := bufio.NewScanner(file)
	inOriginSection := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Check for section headers
		if strings.HasPrefix(line, "[") {
			inOriginSection = strings.Contains(line, `[remote "origin"]`)
			continue
		}

		// Look for url in origin section
		if inOriginSection && strings.HasPrefix(line, "url") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	return "", scanner.Err()
}

// getCurrentBranch reads the current branch from HEAD.
func (d *SubRepoDetector) getCurrentBranch(repoPath string) (string, error) {
	headPath := filepath.Join(repoPath, ".git", "HEAD")

	// Handle worktrees
	gitPath := filepath.Join(repoPath, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return "", err
	}
	if info.Mode().IsRegular() {
		content, err := os.ReadFile(gitPath)
		if err != nil {
			return "", err
		}
		line := strings.TrimSpace(string(content))
		if strings.HasPrefix(line, "gitdir:") {
			gitDir := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
			if !filepath.IsAbs(gitDir) {
				gitDir = filepath.Join(repoPath, gitDir)
			}
			headPath = filepath.Join(gitDir, "HEAD")
		}
	}

	content, err := os.ReadFile(headPath)
	if err != nil {
		return "", err
	}

	head := strings.TrimSpace(string(content))

	// HEAD can be a ref (branch) or a commit hash
	if strings.HasPrefix(head, "ref: refs/heads/") {
		return strings.TrimPrefix(head, "ref: refs/heads/"), nil
	}

	// Detached HEAD - return empty
	return "", nil
}

// SubRepoManifest represents a sub-repository reference in the manifest.
type SubRepoManifest struct {
	// Path is the relative path from home where the repo should be cloned.
	Path string `toml:"path"`

	// URL is the git remote URL.
	URL string `toml:"url"`

	// Branch is the branch to checkout (empty means default).
	Branch string `toml:"branch,omitempty"`

	// Description is an optional description.
	Description string `toml:"description,omitempty"`
}

// ToManifest converts a Candidate to a SubRepoManifest.
func (c *Candidate) ToManifest() *SubRepoManifest {
	if !c.IsSubRepo {
		return nil
	}
	return &SubRepoManifest{
		Path:   c.RelPath,
		URL:    c.SubRepoURL,
		Branch: c.SubRepoBranch,
	}
}

// SubReposManifest holds all sub-repository references.
type SubReposManifest struct {
	// SubRepos is the list of sub-repositories to manage.
	SubRepos []SubRepoManifest `toml:"subrepo"`
}
