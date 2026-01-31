// Package chez provides chezmoi operations for dotstate.
package chez

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dnery/dotstate/dot/internal/runner"
)

// Chezmoi provides chezmoi operations using an external runner.
type Chezmoi struct {
	Bin string
	R   runner.Runner
}

// New creates a new Chezmoi with the given binary path and runner.
// If bin is empty, "chezmoi" is used. If r is nil, a default runner is created.
func New(bin string, r runner.Runner) *Chezmoi {
	if bin == "" {
		bin = "chezmoi"
	}
	if r == nil {
		r = runner.New()
	}
	return &Chezmoi{Bin: bin, R: r}
}

// ReAdd re-adds all managed files that differ in destination.
// This is the core of the "edit real files normally" workflow.
func (c *Chezmoi) ReAdd(ctx context.Context, repoPath string) error {
	_, err := c.R.Run(ctx, repoPath, c.Bin, "re-add")
	return err
}

// Apply applies the source state to the destination.
func (c *Chezmoi) Apply(ctx context.Context, repoPath, sourceDir string) error {
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

// Add adds files to the source state.
// The secretsError option causes an error if secrets are detected.
func (c *Chezmoi) Add(ctx context.Context, repoPath string, files []string, secretsError bool) error {
	if len(files) == 0 {
		return nil
	}

	args := []string{"add"}
	if secretsError {
		args = append(args, "--secrets=error")
	}
	args = append(args, files...)

	_, err := c.R.Run(ctx, repoPath, c.Bin, args...)
	return err
}

// Managed returns the list of files managed by chezmoi.
func (c *Chezmoi) Managed(ctx context.Context, repoPath string) ([]string, error) {
	res, err := c.R.Run(ctx, repoPath, c.Bin, "managed")
	if err != nil {
		return nil, err
	}

	var files []string
	for _, line := range strings.Split(res.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// Version returns the chezmoi version.
func (c *Chezmoi) Version(ctx context.Context) (string, error) {
	res, err := c.R.Run(ctx, "", c.Bin, "--version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Stdout), nil
}

// Diff shows the diff between source and destination.
func (c *Chezmoi) Diff(ctx context.Context, repoPath, sourceDir string) (string, error) {
	args := []string{}
	if sourceDir != "" {
		args = append(args, "--source", filepath.Join(repoPath, sourceDir))
	}
	args = append(args, "diff")

	res, err := c.R.Run(ctx, repoPath, c.Bin, args...)
	if err != nil {
		return "", err
	}
	return res.Stdout, nil
}
