// Package gitx provides git operations for dotstate.
package gitx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dnery/dotstate/dot/internal/runner"
)

// Git provides git operations using an external runner.
type Git struct {
	Bin string
	R   runner.Runner
}

// New creates a new Git with the given binary path and runner.
// If bin is empty, "git" is used. If r is nil, a default runner is created.
func New(bin string, r runner.Runner) *Git {
	if bin == "" {
		bin = "git"
	}
	if r == nil {
		r = runner.New()
	}
	return &Git{Bin: bin, R: r}
}

// EnsureCloned clones a repository if it doesn't exist.
func (g *Git) EnsureCloned(ctx context.Context, repoURL, repoPath, branch string) error {
	if repoURL == "" {
		return fmt.Errorf("repo URL is empty")
	}
	if repoPath == "" {
		return fmt.Errorf("repo path is empty")
	}

	// If .git exists, assume it's already cloned.
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
		return nil
	}

	if info, err := os.Stat(repoPath); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("repo path exists and is not a directory: %s", repoPath)
		}
		empty, err := isDirEmpty(repoPath)
		if err != nil {
			return err
		}
		if !empty {
			return fmt.Errorf("repo path exists but is not a git repo or empty directory: %s", repoPath)
		}
	} else if os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(repoPath), 0o755); err != nil {
			return err
		}
	} else {
		return err
	}

	args := []string{"clone", repoURL, repoPath}
	if _, err := g.R.Run(ctx, "", g.Bin, args...); err != nil {
		return err
	}

	if branch != "" && branch != "main" {
		if _, err := g.R.Run(ctx, repoPath, g.Bin, "checkout", branch); err != nil {
			return err
		}
	}
	return nil
}

func isDirEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

// PorcelainStatus returns the git status in porcelain format.
func (g *Git) PorcelainStatus(ctx context.Context, repoPath string) (string, error) {
	res, err := g.R.Run(ctx, repoPath, g.Bin, "status", "--porcelain")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Stdout), nil
}

// HasChanges returns true if there are uncommitted changes.
func (g *Git) HasChanges(ctx context.Context, repoPath string) (bool, error) {
	status, err := g.PorcelainStatus(ctx, repoPath)
	if err != nil {
		return false, err
	}
	return status != "", nil
}

// AddAll stages all changes.
func (g *Git) AddAll(ctx context.Context, repoPath string) error {
	_, err := g.R.Run(ctx, repoPath, g.Bin, "add", "-A")
	return err
}

// Add stages specific files.
func (g *Git) Add(ctx context.Context, repoPath string, files ...string) error {
	if len(files) == 0 {
		return nil
	}
	args := append([]string{"add"}, files...)
	_, err := g.R.Run(ctx, repoPath, g.Bin, args...)
	return err
}

// Commit commits staged changes with the given message.
// Returns true if a commit was made, false if there was nothing to commit.
func (g *Git) Commit(ctx context.Context, repoPath, message string) (bool, error) {
	// If no changes, do nothing.
	hasChanges, err := g.HasChanges(ctx, repoPath)
	if err != nil {
		return false, err
	}
	if !hasChanges {
		return false, nil
	}

	if err := g.AddAll(ctx, repoPath); err != nil {
		return false, err
	}

	_, err = g.R.Run(ctx, repoPath, g.Bin, "commit", "-m", message)
	if err != nil {
		return false, err
	}
	return true, nil
}

// PullRebase pulls and rebases with autostash.
func (g *Git) PullRebase(ctx context.Context, repoPath string) error {
	_, err := g.R.Run(ctx, repoPath, g.Bin, "pull", "--rebase", "--autostash")
	return err
}

// Push pushes to the remote.
func (g *Git) Push(ctx context.Context, repoPath string) error {
	_, err := g.R.Run(ctx, repoPath, g.Bin, "push")
	return err
}

// CurrentBranch returns the current branch name.
func (g *Git) CurrentBranch(ctx context.Context, repoPath string) (string, error) {
	res, err := g.R.Run(ctx, repoPath, g.Bin, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Stdout), nil
}

// RemoteURL returns the remote URL for origin.
func (g *Git) RemoteURL(ctx context.Context, repoPath string) (string, error) {
	res, err := g.R.Run(ctx, repoPath, g.Bin, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Stdout), nil
}

// DefaultCommitMessage generates a commit message with hostname and timestamp.
func DefaultCommitMessage(hostname string) string {
	ts := time.Now().Format(time.RFC3339)
	if hostname == "" {
		hostname = "unknown-host"
	}
	return fmt.Sprintf("dot sync from %s at %s", hostname, ts)
}
