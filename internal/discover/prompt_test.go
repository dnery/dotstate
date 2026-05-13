package discover

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dnery/dotstate/dot/internal/modules"
)

func TestParseSelectionCategoryBoundaries(t *testing.T) {
	recommended := CandidateList{
		{RelPath: "~/.gitconfig"},
		{RelPath: "~/.zshrc"},
	}
	maybe := CandidateList{
		{RelPath: "~/.config/app/settings.json"},
		{RelPath: "~/.config/app/theme.toml"},
	}
	risky := CandidateList{
		{RelPath: "~/.ssh/id_rsa"},
	}

	selected := make(map[int]*Candidate)
	out := &bytes.Buffer{}
	p := NewPrompterWithIO(strings.NewReader(""), out, false)

	maybeStart := len(recommended) + 1    // 3
	riskyStart := maybeStart + len(maybe) // 5

	p.parseSelection("1,3,4,5", selected, recommended, maybe, risky, maybeStart, riskyStart)

	if got := selected[1].RelPath; got != "~/.gitconfig" {
		t.Fatalf("selection 1 mapped to %q, want ~/.gitconfig", got)
	}
	if got := selected[3].RelPath; got != "~/.config/app/settings.json" {
		t.Fatalf("selection 3 mapped to %q, want first maybe candidate", got)
	}
	if got := selected[4].RelPath; got != "~/.config/app/theme.toml" {
		t.Fatalf("selection 4 mapped to %q, want second maybe candidate", got)
	}
	if got := selected[5].RelPath; got != "~/.ssh/id_rsa" {
		t.Fatalf("selection 5 mapped to %q, want first risky candidate", got)
	}
}

func TestParseSelectionBareNumberTogglesExistingSelection(t *testing.T) {
	recommended := CandidateList{{RelPath: "~/.gitconfig"}}
	selected := map[int]*Candidate{1: recommended[0]}
	out := &bytes.Buffer{}
	p := NewPrompterWithIO(strings.NewReader(""), out, false)

	p.parseSelection("1", selected, recommended, nil, nil, 2, 2)

	if len(selected) != 0 {
		t.Fatalf("expected bare number to toggle selection off, got %d selected", len(selected))
	}
	if !strings.Contains(out.String(), "Removed: ~/.gitconfig") {
		t.Fatalf("expected removal message, got %q", out.String())
	}
}

func TestParseSelectionRejectsOutOfRange(t *testing.T) {
	selected := make(map[int]*Candidate)
	out := &bytes.Buffer{}
	p := NewPrompterWithIO(strings.NewReader(""), out, false)

	recommended := CandidateList{{RelPath: "~/.gitconfig"}}
	maybe := CandidateList{{RelPath: "~/.config/app/settings.json"}}
	risky := CandidateList{{RelPath: "~/.ssh/id_rsa"}}

	maybeStart := len(recommended) + 1    // 2
	riskyStart := maybeStart + len(maybe) // 3

	p.parseSelection("4", selected, recommended, maybe, risky, maybeStart, riskyStart)

	if len(selected) != 0 {
		t.Fatalf("expected no selections for out-of-range input, got %d", len(selected))
	}
	if !strings.Contains(out.String(), "Invalid item number: 4") {
		t.Fatalf("expected invalid-number warning, got %q", out.String())
	}
}

func TestPrintReportRedactsSentinelValues(t *testing.T) {
	const sentinel = "DOTSTATE_TEST_SECRET_DO_NOT_PRINT"
	result := &Result{
		Candidates: CandidateList{
			{
				RelPath:        "~/.config/" + sentinel + ".env",
				Category:       CategoryRisky,
				Score:          10,
				Reasons:        []string{"contains " + sentinel},
				SecretWarnings: []string{"token-assignment: " + sentinel + " (line 1)"},
			},
			{
				RelPath:    "~/src/repo",
				Category:   CategoryMaybe,
				IsSubRepo:  true,
				SubRepoURL: "https://user:" + sentinel + "@github.com/dnery/dotstate.git",
			},
		},
		SubRepos: []*Candidate{
			{RelPath: "~/src/repo", SubRepoURL: "https://user:" + sentinel + "@github.com/dnery/dotstate.git"},
		},
		Diagnostics: []modules.Diagnostic{
			modules.NewDiagnostic(modules.SeverityWarning, "secrets.gitleaks.unavailable", "missing "+sentinel, "secrets", "secrets:gitleaks"),
		},
	}
	out := &bytes.Buffer{}
	p := NewPrompterWithIO(strings.NewReader(""), out, false)

	p.PrintReport(result)

	if strings.Contains(out.String(), sentinel) {
		t.Fatalf("report leaked sentinel:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "<redacted:secret>") || !strings.Contains(out.String(), "secrets.gitleaks.unavailable") {
		t.Fatalf("report missing redaction marker or diagnostic:\n%s", out.String())
	}
}
