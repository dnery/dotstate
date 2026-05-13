package discover

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dnery/dotstate/dot/internal/config"
)

func TestNormalizeOptionsAppliesDefaults(t *testing.T) {
	opts, err := normalizeOptions(Options{})
	if err != nil {
		t.Fatalf("normalizeOptions returned error: %v", err)
	}

	if opts.SecretsMode != SecretsModeError {
		t.Fatalf("SecretsMode = %q, want %q", opts.SecretsMode, SecretsModeError)
	}
	if opts.MaxFileSize != DefaultMaxFileSize {
		t.Fatalf("MaxFileSize = %d, want %d", opts.MaxFileSize, DefaultMaxFileSize)
	}
}

func TestNormalizeOptionsValidatesSecretsMode(t *testing.T) {
	_, err := normalizeOptions(Options{SecretsMode: "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid secrets mode")
	}
}

func TestNormalizeOptionsLowercasesMode(t *testing.T) {
	opts, err := normalizeOptions(Options{SecretsMode: "WARNING", MaxFileSize: DefaultMaxFileSize})
	if err != nil {
		t.Fatalf("normalizeOptions returned error: %v", err)
	}

	if opts.SecretsMode != SecretsModeWarning {
		t.Fatalf("SecretsMode = %q, want %q", opts.SecretsMode, SecretsModeWarning)
	}
}

func TestDiscoverRegistriesLoadCuratedRootsAndIgnores(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	registryDir := filepath.Join(repo, "state", "discover")
	if err := os.MkdirAll(registryDir, 0o755); err != nil {
		t.Fatalf("create registry dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(registryDir, "curated-roots.txt"), []byte("# custom roots\n~/Library/Application Support/MyApp\n.config/tool\n"), 0o644); err != nil {
		t.Fatalf("write curated roots: %v", err)
	}
	if err := os.WriteFile(filepath.Join(registryDir, "ignore.txt"), []byte("# ignore patterns\n*.sqlite\nSecretApp\n"), 0o644); err != nil {
		t.Fatalf("write ignore patterns: %v", err)
	}
	cfgPath := filepath.Join(repo, config.ConfigFileName)
	if err := os.WriteFile(cfgPath, []byte("[repo]\npath = \""+repo+"\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	curated, ignored := readDiscoverRegistries(loaded, home)
	if len(curated) != 2 || curated[0] != filepath.Join(home, "Library", "Application Support", "MyApp") || curated[1] != filepath.Join(home, ".config", "tool") {
		t.Fatalf("curated roots = %#v", curated)
	}
	if len(ignored) != 2 || ignored[0] != "*.sqlite" || ignored[1] != "SecretApp" {
		t.Fatalf("ignored patterns = %#v", ignored)
	}
}
