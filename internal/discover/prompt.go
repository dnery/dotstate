package discover

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Prompter handles user interaction for file selection.
type Prompter struct {
	in     io.Reader
	out    io.Writer
	autoYes bool
}

// NewPrompter creates a new prompter.
func NewPrompter(autoYes bool) *Prompter {
	return &Prompter{
		in:      os.Stdin,
		out:     os.Stdout,
		autoYes: autoYes,
	}
}

// NewPrompterWithIO creates a prompter with custom I/O (for testing).
func NewPrompterWithIO(in io.Reader, out io.Writer, autoYes bool) *Prompter {
	return &Prompter{
		in:      in,
		out:     out,
		autoYes: autoYes,
	}
}

// SelectCandidates prompts the user to select candidates to add.
// Returns the list of selected candidates.
func (p *Prompter) SelectCandidates(ctx context.Context, result *Result) ([]*Candidate, error) {
	if len(result.Candidates) == 0 {
		fmt.Fprintln(p.out, "No candidates found.")
		return nil, nil
	}

	// Print summary
	summary := result.Summary()
	fmt.Fprintf(p.out, "\nDiscovered %d candidates:\n", len(result.Candidates))
	if count := summary[CategoryRecommended]; count > 0 {
		fmt.Fprintf(p.out, "  Recommended: %d\n", count)
	}
	if count := summary[CategoryMaybe]; count > 0 {
		fmt.Fprintf(p.out, "  Maybe: %d\n", count)
	}
	if count := summary[CategoryRisky]; count > 0 {
		fmt.Fprintf(p.out, "  Risky: %d\n", count)
	}
	if len(result.SubRepos) > 0 {
		fmt.Fprintf(p.out, "  Sub-repos: %d\n", len(result.SubRepos))
	}
	fmt.Fprintln(p.out)

	// Group candidates by category
	recommended := result.Candidates.ByCategory(CategoryRecommended)
	maybe := result.Candidates.ByCategory(CategoryMaybe)
	risky := result.Candidates.ByCategory(CategoryRisky)

	// Build selection
	selected := make(map[int]*Candidate)
	index := 1

	// Print and pre-select recommended
	if len(recommended) > 0 {
		fmt.Fprintln(p.out, "=== Recommended (pre-selected) ===")
		for _, c := range recommended {
			prefix := "[x]"
			selected[index] = c
			p.printCandidate(index, c, prefix)
			index++
		}
		fmt.Fprintln(p.out)
	}

	// Print maybe (not pre-selected)
	maybeStart := index
	if len(maybe) > 0 {
		fmt.Fprintln(p.out, "=== Maybe ===")
		for _, c := range maybe {
			p.printCandidate(index, c, "[ ]")
			index++
		}
		fmt.Fprintln(p.out)
	}

	// Print risky (not pre-selected, with warnings)
	riskyStart := index
	if len(risky) > 0 {
		fmt.Fprintln(p.out, "=== Risky (may contain secrets) ===")
		for _, c := range risky {
			p.printCandidate(index, c, "[ ]")
			if len(c.SecretWarnings) > 0 {
				for _, w := range c.SecretWarnings {
					fmt.Fprintf(p.out, "       WARNING: %s\n", w)
				}
			}
			index++
		}
		fmt.Fprintln(p.out)
	}

	// Auto-yes mode: return pre-selected (recommended) items
	if p.autoYes {
		var items []*Candidate
		for _, c := range selected {
			items = append(items, c)
		}
		fmt.Fprintf(p.out, "Auto-selecting %d recommended items.\n", len(items))
		return items, nil
	}

	// Interactive selection
	fmt.Fprintln(p.out, "Commands:")
	fmt.Fprintln(p.out, "  Enter  - Accept current selection")
	fmt.Fprintln(p.out, "  a      - Select all")
	fmt.Fprintln(p.out, "  n      - Select none")
	fmt.Fprintln(p.out, "  1,2,3  - Toggle specific items")
	fmt.Fprintln(p.out, "  +5     - Add item 5")
	fmt.Fprintln(p.out, "  -5     - Remove item 5")
	fmt.Fprintln(p.out, "  q      - Quit without adding")
	fmt.Fprintln(p.out)

	scanner := bufio.NewScanner(p.in)

	for {
		// Show current selection count
		fmt.Fprintf(p.out, "Selected: %d items. Command: ", len(selected))

		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())

		switch strings.ToLower(input) {
		case "", "y", "yes":
			// Accept current selection
			var items []*Candidate
			for _, c := range selected {
				items = append(items, c)
			}
			return items, nil

		case "q", "quit", "exit":
			fmt.Fprintln(p.out, "Cancelled.")
			return nil, nil

		case "a", "all":
			// Select all
			idx := 1
			for _, c := range recommended {
				selected[idx] = c
				idx++
			}
			for _, c := range maybe {
				selected[idx] = c
				idx++
			}
			for _, c := range risky {
				selected[idx] = c
				idx++
			}
			fmt.Fprintf(p.out, "Selected all %d items.\n", len(selected))

		case "n", "none":
			// Select none
			selected = make(map[int]*Candidate)
			fmt.Fprintln(p.out, "Cleared selection.")

		default:
			// Parse number-based commands
			p.parseSelection(input, selected, recommended, maybe, risky, maybeStart, riskyStart)
		}
	}

	return nil, scanner.Err()
}

// printCandidate prints a single candidate line.
func (p *Prompter) printCandidate(index int, c *Candidate, prefix string) {
	typeStr := ""
	if c.IsSubRepo {
		typeStr = " [repo]"
	}

	sizeStr := ""
	if c.Size > 0 {
		sizeStr = fmt.Sprintf(" (%s)", humanSize(c.Size))
	}

	reasons := ""
	if len(c.Reasons) > 0 {
		reasons = " - " + strings.Join(c.Reasons, ", ")
	}

	fmt.Fprintf(p.out, "  %s %3d. %s%s%s%s\n", prefix, index, c.RelPath, typeStr, sizeStr, reasons)
}

// parseSelection parses a selection input and updates the selected map.
func (p *Prompter) parseSelection(input string, selected map[int]*Candidate,
	recommended, maybe, risky CandidateList, maybeStart, riskyStart int) {

	// Handle comma-separated list
	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Handle +N (add) or -N (remove) syntax
		add := true
		if strings.HasPrefix(part, "+") {
			part = strings.TrimPrefix(part, "+")
		} else if strings.HasPrefix(part, "-") {
			add = false
			part = strings.TrimPrefix(part, "-")
		}

		num, err := strconv.Atoi(part)
		if err != nil {
			continue
		}

		// Find the candidate by index
		var candidate *Candidate
		if num >= 1 && num < maybeStart && num <= len(recommended) {
			candidate = recommended[num-1]
		} else if num >= maybeStart && num < riskyStart && num-maybeStart < len(maybe) {
			candidate = maybe[num-maybeStart]
		} else if num >= riskyStart && num-riskyStart < len(risky) {
			candidate = risky[num-riskyStart]
		}

		if candidate == nil {
			fmt.Fprintf(p.out, "Invalid item number: %d\n", num)
			continue
		}

		if add {
			selected[num] = candidate
			fmt.Fprintf(p.out, "Added: %s\n", candidate.RelPath)
		} else {
			delete(selected, num)
			fmt.Fprintf(p.out, "Removed: %s\n", candidate.RelPath)
		}
	}
}

// humanSize converts bytes to human-readable format.
func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ConfirmAdd asks for confirmation before adding files.
func (p *Prompter) ConfirmAdd(candidates []*Candidate) bool {
	if p.autoYes {
		return true
	}

	fmt.Fprintf(p.out, "\nAdd %d files to the repository? [Y/n] ", len(candidates))

	scanner := bufio.NewScanner(p.in)
	if !scanner.Scan() {
		return false
	}

	input := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return input == "" || input == "y" || input == "yes"
}

// ConfirmCommit asks for confirmation before committing.
func (p *Prompter) ConfirmCommit() bool {
	if p.autoYes {
		return true
	}

	fmt.Fprint(p.out, "Commit the changes? [Y/n] ")

	scanner := bufio.NewScanner(p.in)
	if !scanner.Scan() {
		return false
	}

	input := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return input == "" || input == "y" || input == "yes"
}

// PrintReport prints a non-interactive report of discovered candidates.
func (p *Prompter) PrintReport(result *Result) {
	fmt.Fprintf(p.out, "Scan completed in %v\n", result.ScanDuration)
	fmt.Fprintf(p.out, "Scanned: %d directories, %d files\n", result.ScannedDirs, result.ScannedFiles)
	fmt.Fprintln(p.out)

	if len(result.Candidates) == 0 {
		fmt.Fprintln(p.out, "No candidates found.")
		return
	}

	// Print by category
	for _, cat := range []Category{CategoryRecommended, CategoryMaybe, CategoryRisky} {
		candidates := result.Candidates.ByCategory(cat)
		if len(candidates) == 0 {
			continue
		}

		fmt.Fprintf(p.out, "=== %s (%d) ===\n", cat.String(), len(candidates))
		for _, c := range candidates {
			typeStr := "file"
			if c.IsSubRepo {
				typeStr = "repo"
			} else if c.IsDir {
				typeStr = "dir"
			}

			fmt.Fprintf(p.out, "[%s] %s (score=%d, %s)\n",
				typeStr, c.RelPath, c.Score, humanSize(c.Size))

			if len(c.Reasons) > 0 {
				fmt.Fprintf(p.out, "       reasons: %s\n", strings.Join(c.Reasons, ", "))
			}
			if len(c.SecretWarnings) > 0 {
				for _, w := range c.SecretWarnings {
					fmt.Fprintf(p.out, "       WARNING: %s\n", w)
				}
			}
			if c.IsSubRepo && c.SubRepoURL != "" {
				fmt.Fprintf(p.out, "       remote: %s\n", c.SubRepoURL)
			}
		}
		fmt.Fprintln(p.out)
	}

	// Print sub-repos summary
	if len(result.SubRepos) > 0 {
		fmt.Fprintf(p.out, "=== Sub-repositories (%d) ===\n", len(result.SubRepos))
		for _, r := range result.SubRepos {
			url := r.SubRepoURL
			if url == "" {
				url = "(local only)"
			}
			fmt.Fprintf(p.out, "  %s -> %s\n", r.RelPath, url)
		}
		fmt.Fprintln(p.out)
	}
}
