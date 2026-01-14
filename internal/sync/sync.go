package sync

import (
	"context"
	"fmt"
	"os"

	"github.com/dnery/dotstate/dot/internal/chez"
	"github.com/dnery/dotstate/dot/internal/config"
	"github.com/dnery/dotstate/dot/internal/gitx"
)

type Syncer struct {
	Cfg  *config.Config
	Git  *gitx.Git
	Chez *chez.Chezmoi
}

type Options struct {
	NoApply bool
	NoPush  bool
}

func New(cfg *config.Config, g *gitx.Git, ch *chez.Chezmoi) *Syncer {
	return &Syncer{Cfg: cfg, Git: g, Chez: ch}
}

func (s *Syncer) Capture(ctx context.Context) error {
	// Phase 1: capture dotfiles (managed artifacts) back into repo source state.
	if err := s.Chez.ReAdd(ctx, s.Cfg.Repo.Path); err != nil {
		return err
	}
	// TODO: capture OS exports (packages lists, registry, defaults) into state/
	return nil
}

func (s *Syncer) Apply(ctx context.Context) error {
	// Phase 2: apply desired state to machine.
	return s.Chez.Apply(ctx, s.Cfg.Repo.Path, s.Cfg.Chex.SourceDir)
}

func (s *Syncer) Sync(ctx context.Context, opts Options) error {
	if err := s.Capture(ctx); err != nil {
		return fmt.Errorf("capture: %w", err)
	}

	host, _ := os.Hostname()
	msg := gitx.DefaultCommitMessage(host)

	committed, err := s.Git.Commit(ctx, s.Cfg.Repo.Path, msg)
	if err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	_ = committed // used later for UX

	// Pull/rebase before apply so we converge on the canonical remote state.
	if err := s.Git.PullRebase(ctx, s.Cfg.Repo.Path); err != nil {
		return fmt.Errorf("pull: %w", err)
	}

	if !opts.NoApply {
		if err := s.Apply(ctx); err != nil {
			return fmt.Errorf("apply: %w", err)
		}
	}

	if !opts.NoPush {
		if err := s.Git.Push(ctx, s.Cfg.Repo.Path); err != nil {
			return fmt.Errorf("push: %w", err)
		}
	}

	return nil
}
