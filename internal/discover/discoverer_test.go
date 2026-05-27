package discover

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestHandleSubReposMergesExistingManifestAndRedactsCredentials(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	configPath := testutil.TempDotToml(t, tmpDir, testutil.MinimalDotToml())

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	existing := SubReposManifest{SubRepos: []SubRepoManifest{{Path: ".config/old", URL: "https://github.com/user/old", Branch: "main"}}}
	data, err := toml.Marshal(existing)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(cfg.StatePath(), "subrepos.toml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	d := &Discoverer{cfg: cfg}
	candidates := []*Candidate{{
		IsSubRepo:     true,
		RelPath:       ".config/new",
		SubRepoURL:    "https://ghp_SECRET@github.com/user/new.git",
		SubRepoBranch: "develop",
	}}

	if err := d.handleSubRepos(context.Background(), candidates); err != nil {
		t.Fatalf("handleSubRepos: %v", err)
	}

	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "ghp_SECRET") {
		t.Fatalf("manifest leaked credentialed remote: %s", string(content))
	}

	var got SubReposManifest
	if err := toml.Unmarshal(content, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.SubRepos) != 2 {
		t.Fatalf("merged manifest has %d entries, want 2: %#v", len(got.SubRepos), got.SubRepos)
	}
	foundNew := false
	for _, entry := range got.SubRepos {
		if entry.Path == ".config/new" {
			foundNew = true
			if entry.URL != "https://github.com/user/new.git" {
				t.Fatalf("new subrepo URL = %q, want sanitized URL", entry.URL)
			}
		}
	}
	if !foundNew {
		t.Fatalf("new subrepo missing from merged manifest: %#v", got.SubRepos)
	}
}

func TestNormalizeManagedPathsExpandsRelativeDestinationPaths(t *testing.T) {
	home := t.TempDir()
	paths := normalizeManagedPaths([]string{".zshrc", "~/.config/app/settings.json", filepath.Join(home, ".gitconfig")}, home)

	for _, path := range []string{
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".config", "app", "settings.json"),
		filepath.Join(home, ".gitconfig"),
	} {
		if !paths[path] {
			t.Fatalf("expected managed path %q in %#v", path, paths)
		}
	}
}

func TestSanitizeGitRemoteURLRedactsMalformedCredentialedHTTPSURL(t *testing.T) {
	cases := map[string]string{
		"https://ghp_%ZZ@github.com/user/repo.git":                 "https://github.com/user/repo.git",
		"https://ghp_SECRET@":                                      "https://",
		"https://github.com/user/repo.git?access_token=ghp_SECRET": "https://github.com/user/repo.git?access_token=REDACTED",
		"ssh://ghp_SECRET@github.com/user/repo.git":                "ssh://github.com/user/repo.git",
		"ssh://ghp_SECRET:pass@github.com/user/repo.git":           "ssh://github.com/user/repo.git",
		"ghp_SECRET@github.com:user/repo.git":                      "github.com:user/repo.git",
		"user:pass@github.com:user/repo.git":                       "github.com:user/repo.git",
	}
	for raw, want := range cases {
		got, redacted := sanitizeGitRemoteURL(raw)
		if !redacted {
			t.Fatalf("expected malformed credentialed URL %q to be redacted", raw)
		}
		if strings.Contains(got, "ghp_") || strings.Contains(got, "pass") || got != want {
			t.Fatalf("sanitizeGitRemoteURL(%q) = %q, want %q", raw, got, want)
		}
	}

	got, redacted := sanitizeGitRemoteURL("git@github.com:user/repo.git")
	if redacted || got != "git@github.com:user/repo.git" {
		t.Fatalf("normal SSH remote should be preserved, got %q redacted=%v", got, redacted)
	}
}
