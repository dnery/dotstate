// Package discover provides file discovery and classification for dotstate.
//
// The discover package scans the filesystem for configuration files that are
// not yet tracked in the dotstate repository, classifies them by likelihood
// of being useful, detects potential secrets, and optionally adds them to
// the repository's managed state.
package discover

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dnery/dotstate/dot/internal/platform"
)

// Category represents the classification of a discovered file.
type Category int

const (
	// CategoryIgnored files are filtered out and not shown.
	CategoryIgnored Category = iota
	// CategoryRisky files may contain secrets and are not preselected.
	CategoryRisky
	// CategoryMaybe files might be useful but aren't preselected.
	CategoryMaybe
	// CategoryRecommended files are high-confidence configs, preselected.
	CategoryRecommended
)

func (c Category) String() string {
	switch c {
	case CategoryIgnored:
		return "Ignored"
	case CategoryRisky:
		return "Risky"
	case CategoryMaybe:
		return "Maybe"
	case CategoryRecommended:
		return "Recommended"
	default:
		return "Unknown"
	}
}

// Candidate represents a discovered file or sub-repository.
type Candidate struct {
	// Path is the absolute path to the file or directory.
	Path string

	// RelPath is the path relative to home directory.
	RelPath string

	// Category is the classification of this candidate.
	Category Category

	// Score is the numeric score used for ranking (higher = more likely useful).
	Score int

	// Size is the file size in bytes (0 for directories/subrepos).
	Size int64

	// IsDir indicates if this is a directory.
	IsDir bool

	// IsSubRepo indicates if this is a git sub-repository.
	IsSubRepo bool

	// SubRepoURL is the remote URL for sub-repositories.
	SubRepoURL string

	// SubRepoBranch is the current branch for sub-repositories.
	SubRepoBranch string

	// Reasons explains why this candidate received its category/score.
	Reasons []string

	// SecretWarnings contains any secret detection warnings.
	SecretWarnings []string

	// ModTime is the last modification time.
	ModTime time.Time
}

// CandidateList is a sortable list of candidates.
type CandidateList []*Candidate

func (l CandidateList) Len() int      { return len(l) }
func (l CandidateList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }

// Less sorts by category (Recommended first), then by score (higher first).
func (l CandidateList) Less(i, j int) bool {
	if l[i].Category != l[j].Category {
		return l[i].Category > l[j].Category // Higher category = better
	}
	return l[i].Score > l[j].Score // Higher score = better
}

// ByCategory returns candidates filtered by category.
func (l CandidateList) ByCategory(cat Category) CandidateList {
	var result CandidateList
	for _, c := range l {
		if c.Category == cat {
			result = append(result, c)
		}
	}
	return result
}

// ScanOptions configures the discovery scan.
type ScanOptions struct {
	// Roots are the directories to scan. If empty, default roots are used.
	Roots []string

	// Deep enables scanning of broader directories (AppData, Library).
	Deep bool

	// MaxFileSize is the maximum file size to consider (default 2MB).
	MaxFileSize int64

	// IncludeHidden includes hidden files/directories in scan.
	IncludeHidden bool

	// Home is the user's home directory (for relative path calculation).
	Home string

	// Platform overrides the detected platform when set.
	Platform *platform.Platform

	// ManagedPaths are paths already managed by chezmoi (to exclude).
	ManagedPaths map[string]bool
}

// DefaultMaxFileSize is 2 MiB.
const DefaultMaxFileSize = 2 * 1024 * 1024

// DefaultScanOptions returns default scan options for the given home directory.
func DefaultScanOptions(home string) ScanOptions {
	return ScanOptions{
		MaxFileSize:   DefaultMaxFileSize,
		IncludeHidden: true,
		Home:          home,
		ManagedPaths:  make(map[string]bool),
	}
}

// Result holds the complete discovery result.
type Result struct {
	// Candidates is the list of all discovered candidates.
	Candidates CandidateList

	// SubRepos is the list of detected sub-repositories.
	SubRepos []*Candidate

	// ScanDuration is how long the scan took.
	ScanDuration time.Duration

	// ScannedDirs is the number of directories scanned.
	ScannedDirs int

	// ScannedFiles is the number of files examined.
	ScannedFiles int

	// Errors contains any non-fatal errors encountered.
	Errors []error
}

// Summary returns counts by category.
func (r *Result) Summary() map[Category]int {
	counts := make(map[Category]int)
	for _, c := range r.Candidates {
		counts[c.Category]++
	}
	return counts
}

// relPath returns the path relative to home, or the original path if not under home.
func relPath(path, home string) string {
	if home == "" {
		return path
	}
	rel, err := filepath.Rel(home, path)
	if err != nil {
		return path
	}
	// If the relative path starts with "..", it's not under home
	if strings.HasPrefix(rel, "..") {
		return path
	}
	return "~/" + rel
}

// isHidden returns true if the file/directory is hidden.
func isHidden(name string) bool {
	return strings.HasPrefix(name, ".")
}

// fileExt returns the lowercase file extension.
func fileExt(path string) string {
	return strings.ToLower(filepath.Ext(path))
}

// baseName returns the lowercase base name.
func baseName(path string) string {
	return strings.ToLower(filepath.Base(path))
}

// containsAny returns true if s contains any of the substrings.
func containsAny(s string, substrs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// matchesAny returns true if s matches any of the patterns exactly.
func matchesAny(s string, patterns ...string) bool {
	lower := strings.ToLower(s)
	for _, p := range patterns {
		if lower == strings.ToLower(p) {
			return true
		}
	}
	return false
}

// fileInfo wraps os.FileInfo with the full path.
type fileInfo struct {
	os.FileInfo
	Path string
}
