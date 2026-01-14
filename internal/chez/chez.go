package chez

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/dnery/dotstate/dot/internal/runner"
)

type Chezmoi struct {
	Bin string
	R   *runner.Runner
}

func New(bin string, r *runner.Runner) *Chezmoi {
	if bin == "" {
		bin = "chezmoi"
	}
	if r == nil {
		r = runner.New()
	}
	return &Chezmoi{Bin: bin, R: r}
}

func (c *Chezmoi) ReAdd(ctx context.Context, repoPath string) error {
	// `chezmoi re-add` with no args re-adds all managed files that differ in destination.
	// This is the core of the "edit real files normally" workflow.
	_, err := c.R.Run(ctx, repoPath, c.Bin, "re-add")
	return err
}

func (c *Chezmoi) Apply(ctx context.Context, repoPath, sourceDir string) error {
	// If your repo uses a dedicated source directory (e.g. ./home),
	// pass it explicitly so the tool can live in the same repo.
	args := []string{}
	if sourceDir != "" {
		args = append(args, "--source", filepath.Join(repoPath, sourceDir))
	}
	args = append(args, "apply")
	_, err := c.R.Run(ctx, repoPath, c.Bin, args...)
	if err != nil {
		return fmt.Errorf("chezmoi apply failed: %w", err)
	}
	return nil
}
