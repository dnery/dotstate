package discover

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockFileInfo implements os.FileInfo for testing.
type mockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return m.modTime }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() interface{}   { return nil }

func TestClassifier_Classify(t *testing.T) {
	c := NewClassifier()
	home := "/home/user"

	tests := []struct {
		name         string
		path         string
		info         os.FileInfo
		wantCategory Category
		wantMinScore int
	}{
		{
			name: "gitconfig is recommended",
			path: filepath.Join(home, ".gitconfig"),
			info: mockFileInfo{name: ".gitconfig", size: 1024},
			wantCategory: CategoryRecommended,
			wantMinScore: 100,
		},
		{
			name: "zshrc is recommended",
			path: filepath.Join(home, ".zshrc"),
			info: mockFileInfo{name: ".zshrc", size: 512},
			wantCategory: CategoryRecommended,
			wantMinScore: 100,
		},
		{
			name: "vscode settings is recommended",
			path: filepath.Join(home, ".config/Code/User/settings.json"),
			info: mockFileInfo{name: "settings.json", size: 2048},
			wantCategory: CategoryRecommended,
			wantMinScore: 70,
		},
		{
			name: "random json in config is maybe",
			path: filepath.Join(home, ".config/app/data.json"),
			info: mockFileInfo{name: "data.json", size: 100},
			wantCategory: CategoryMaybe,
			wantMinScore: 30,
		},
		{
			name: "ssh private key is risky",
			path: filepath.Join(home, ".ssh/id_rsa"),
			info: mockFileInfo{name: "id_rsa", size: 2048},
			wantCategory: CategoryRisky,
		},
		{
			name: "ssh config is recommended",
			path: filepath.Join(home, ".ssh/config"),
			info: mockFileInfo{name: "config", size: 256},
			wantCategory: CategoryRecommended,
			wantMinScore: 50,
		},
		{
			name: "file with token in name is risky",
			path: filepath.Join(home, ".config/app/api_token"),
			info: mockFileInfo{name: "api_token", size: 100},
			wantCategory: CategoryRisky,
		},
		{
			name: "file with secret in name is risky",
			path: filepath.Join(home, ".config/app/secret.json"),
			info: mockFileInfo{name: "secret.json", size: 100},
			wantCategory: CategoryRisky,
		},
		{
			name: "env file is risky",
			path: filepath.Join(home, ".env"),
			info: mockFileInfo{name: ".env", size: 100},
			wantCategory: CategoryRisky,
		},
		{
			name: "kubeconfig is risky",
			path: filepath.Join(home, ".kube/config"),
			info: mockFileInfo{name: "config", size: 1024},
			wantCategory: CategoryRisky,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := c.Classify(tt.path, tt.info, home)

			if candidate.Category != tt.wantCategory {
				t.Errorf("Classify() category = %v, want %v", candidate.Category, tt.wantCategory)
			}

			if tt.wantMinScore > 0 && candidate.Score < tt.wantMinScore {
				t.Errorf("Classify() score = %v, want >= %v", candidate.Score, tt.wantMinScore)
			}
		})
	}
}

func TestClassifier_IsConfigExtension(t *testing.T) {
	c := NewClassifier()

	tests := []struct {
		path string
		want bool
	}{
		{"config.json", true},
		{"config.toml", true},
		{"config.yaml", true},
		{"config.yml", true},
		{"settings.ini", true},
		{"app.conf", true},
		{"init.lua", true},
		{"config.kdl", true},
		{"file.txt", false},
		{"binary.exe", false},
		{"image.png", false},
		{"no_extension", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := c.IsConfigExtension(tt.path)
			if got != tt.want {
				t.Errorf("IsConfigExtension(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestClassifier_IsSafeSSHFile(t *testing.T) {
	c := NewClassifier()

	tests := []struct {
		name string
		want bool
	}{
		{"config", true},
		{"known_hosts", true},
		{"known_hosts.old", true},
		{"authorized_keys", true},
		{"id_rsa", false},
		{"id_ed25519", false},
		{"id_ecdsa", false},
		{"random_file", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.IsSafeSSHFile(tt.name)
			if got != tt.want {
				t.Errorf("IsSafeSSHFile(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestCategory_String(t *testing.T) {
	tests := []struct {
		cat  Category
		want string
	}{
		{CategoryIgnored, "Ignored"},
		{CategoryRisky, "Risky"},
		{CategoryMaybe, "Maybe"},
		{CategoryRecommended, "Recommended"},
		{Category(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.cat.String()
			if got != tt.want {
				t.Errorf("Category.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCandidateList_ByCategory(t *testing.T) {
	list := CandidateList{
		{Path: "/a", Category: CategoryRecommended},
		{Path: "/b", Category: CategoryMaybe},
		{Path: "/c", Category: CategoryRecommended},
		{Path: "/d", Category: CategoryRisky},
		{Path: "/e", Category: CategoryMaybe},
	}

	recommended := list.ByCategory(CategoryRecommended)
	if len(recommended) != 2 {
		t.Errorf("ByCategory(Recommended) returned %d items, want 2", len(recommended))
	}

	maybe := list.ByCategory(CategoryMaybe)
	if len(maybe) != 2 {
		t.Errorf("ByCategory(Maybe) returned %d items, want 2", len(maybe))
	}

	risky := list.ByCategory(CategoryRisky)
	if len(risky) != 1 {
		t.Errorf("ByCategory(Risky) returned %d items, want 1", len(risky))
	}

	ignored := list.ByCategory(CategoryIgnored)
	if len(ignored) != 0 {
		t.Errorf("ByCategory(Ignored) returned %d items, want 0", len(ignored))
	}
}
