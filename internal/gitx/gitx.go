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

type Git struct {
	Bin string
	R   *runner.Runner
}

func New(bin string, r *runner.Runner) *Git {
	if bin == "" {
		bin = "git"
	}
	if r == nil {
		r = runner.New()
	}
	return &Git{Bin: bin, R: r}
}

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

	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		return err
	}

	// Clone into repoPath parent, because `git clone <url> <path>` requires parent exists.
	// But we already created repoPath; remove it to avoid clone error.
	_ = os.RemoveAll(repoPath)

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

func (g *Git) PorcelainStatus(ctx context.Context, repoPath string) (string, error) {
	res, err := g.R.Run(ctx, repoPath, g.Bin, "status", "--porcelain")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Stdout), nil
}

func (g *Git) AddAll(ctx context.Context, repoPath string) error {
	_, err := g.R.Run(ctx, repoPath, g.Bin, "add", "-A")
	return err
}

func (g *Git) Commit(ctx context.Context, repoPath, message string) (bool, error) {
	// If no changes, do nothing.
	st, err := g.PorcelainStatus(ctx, repoPath)
	if err != nil {
		return false, err
	}
	if st == "" {
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

func (g *Git) PullRebase(ctx context.Context, repoPath string) error {
	// A reasonable default: rebase + autostash to reduce conflicts.
	_, err := g.R.Run(ctx, repoPath, g.Bin, "pull", "--rebase", "--autostash")
	return err
}

func (g *Git) Push(ctx context.Context, repoPath string) error {
	_, err := g.R.Run(ctx, repoPath, g.Bin, "push")
	return err
}

func DefaultCommitMessage(hostname string) string {
	ts := time.Now().Format(time.RFC3339)
	if hostname == "" {
		hostname = "unknown-host"
	}
	return fmt.Sprintf("dot sync from %s at %s", hostname, ts)
}
