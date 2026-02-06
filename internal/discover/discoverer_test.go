package discover

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dnery/dotstate/dot/internal/config"
	"github.com/dnery/dotstate/dot/internal/testutil"
	toml "github.com/pelletier/go-toml/v2"
)

func TestHandleSubReposWritesManifest(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	configPath := testutil.TempDotToml(t, tmpDir, testutil.MinimalDotToml())

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	d := &Discoverer{cfg: cfg}
	candidates := []*Candidate{
		{
			IsSubRepo:     true,
			RelPath:       ".config/nvim",
			SubRepoURL:    "https://github.com/user/nvim-config",
			SubRepoBranch: "main",
		},
		{
			IsSubRepo:  true,
			RelPath:    ".config/local-only",
			SubRepoURL: "",
		},
	}

	if err := d.handleSubRepos(context.Background(), candidates); err != nil {
		t.Fatalf("handleSubRepos: %v", err)
	}

	manifestPath := filepath.Join(cfg.StatePath(), "subrepos.toml")
	testutil.AssertFileExists(t, manifestPath)

	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	expected := SubReposManifest{
		SubRepos: []SubRepoManifest{
			{
				Path:   ".config/nvim",
				URL:    "https://github.com/user/nvim-config",
				Branch: "main",
			},
		},
	}

	data, err := toml.Marshal(expected)
	if err != nil {
		t.Fatalf("marshal expected manifest: %v", err)
	}

	if string(content) != string(data) {
		t.Fatalf("manifest mismatch:\nexpected:\n%s\nactual:\n%s", string(data), string(content))
	}
}
