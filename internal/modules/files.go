package modules

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dnery/dotstate/dot/internal/chez"
	"github.com/dnery/dotstate/dot/internal/config"
	"github.com/dnery/dotstate/dot/internal/redact"
)

const filesSurface = "files"

type FilesModule struct {
	Chez       *chez.Chezmoi
	RepoPath   string
	SourceDir  string
	Home       string
	BackupRoot string
	now        func() time.Time
}

func NewFilesModule(cfg *config.Config, ch *chez.Chezmoi, home string) *FilesModule {
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return &FilesModule{
		Chez:       ch,
		RepoPath:   cfg.Repo.Path,
		SourceDir:  cfg.Chex.SourceDir,
		Home:       home,
		BackupRoot: filepath.Join(cfg.Repo.Path, "state", "backups"),
		now:        time.Now,
	}
}

func (m *FilesModule) Surface() string { return filesSurface }

func (m *FilesModule) Discover(ctx context.Context) ([]Fact, []Diagnostic, error) {
	return []Fact{m.sourceFact(map[string]any{"source_dir": m.SourceDir})}, nil, nil
}

func (m *FilesModule) Audit(ctx context.Context) ([]Fact, []Diagnostic, error) {
	managed, err := m.Chez.Managed(ctx, m.RepoPath, m.SourceDir)
	if err != nil {
		return nil, nil, err
	}
	facts := make([]Fact, 0, len(managed))
	for _, managedPath := range managed {
		dest, err := m.destinationPath(managedPath)
		if err != nil {
			fact := m.sourceFact(map[string]any{"path": managedPath, "path_invalid": true})
			fact.ID = "files:path/" + managedPath
			fact.Confidence = ConfidenceLow
			facts = append(facts, fact)
			continue
		}
		current := map[string]any{"path": m.displayPath(dest)}
		if info, err := os.Lstat(dest); err == nil {
			current["exists"] = true
			current["mode"] = fmt.Sprintf("%04o", info.Mode().Perm())
			current["type"] = fileType(info)
		} else if os.IsNotExist(err) {
			current["exists"] = false
		} else {
			current["exists"] = nil
			current["stat_error"] = true
		}
		fact := m.sourceFact(current)
		fact.ID = "files:path/" + m.displayPath(dest)
		fact.Source = Source{Kind: "path", Value: m.displayPath(dest)}
		facts = append(facts, fact)
	}
	return facts, nil, nil
}

func (m *FilesModule) Plan(ctx context.Context, operation Operation) ([]Change, []Diagnostic, error) {
	switch operation {
	case OperationApply:
		return m.planApply(ctx)
	case OperationCapture:
		return m.planCapture(), nil, nil
	default:
		change := m.baseChange(operation)
		change.Action = ActionBlocked
		change.Capability = []Capability{CapabilityUnsupported}
		change.Diagnostics = []Diagnostic{NewDiagnostic(SeverityError, "files.operation_unsupported", "Files module does not support this operation.", filesSurface, change.ID)}
		return []Change{change}, nil, nil
	}
}

func (m *FilesModule) Backup(ctx context.Context, changes []Change, plan *Plan) ([]Backup, []Diagnostic, error) {
	if !requiresBackup(changes) {
		return nil, nil, nil
	}

	managed, err := m.Chez.Managed(ctx, m.RepoPath, m.SourceDir)
	if err != nil {
		return nil, nil, err
	}

	createdAt := m.now().UTC()
	backupID := newID(createdAt, "files")
	var backups []Backup
	var diagnostics []Diagnostic
	for _, managedPath := range managed {
		dest, err := m.destinationPath(managedPath)
		if err != nil {
			diagnostics = append(diagnostics, backupDiagnostic(SeverityWarning, "files.backup.path_invalid", err.Error(), managedPath))
			continue
		}

		backup := Backup{
			SchemaVersion: SchemaBackupV1,
			BackupID:      backupID,
			CreatedAt:     Timestamp(createdAt),
			Surface:       filesSurface,
			ID:            "files:path/" + m.displayPath(dest),
			Source:        Source{Kind: "path", Value: m.displayPath(dest)},
			Current:       map[string]any{"exists": false},
			Desired:       nil,
			ManagedBy:     []string{"dotstate", "chezmoi"},
			Sensitivity:   SensitivityLocalPath,
			Confidence:    ConfidenceConfirmed,
			Capability:    []Capability{CapabilityAutoApply},
			Risk:          LowRisk(true),
			Restore:       RestoreInfo{Supported: true, RequiresConfirmation: true},
		}

		info, err := os.Lstat(dest)
		if os.IsNotExist(err) {
			backups = append(backups, backup)
			continue
		}
		if err != nil {
			diagnostics = append(diagnostics, backupDiagnostic(SeverityWarning, "files.backup.stat_failed", fmt.Sprintf("Could not stat %s: %v", m.displayPath(dest), err), dest))
			backup.Restore.Supported = false
			backups = append(backups, backup)
			continue
		}

		backup.Current["exists"] = true
		backup.Current["mode"] = fmt.Sprintf("%04o", info.Mode().Perm())
		backup.Current["type"] = fileType(info)
		if !info.Mode().IsRegular() {
			backup.Restore.Supported = false
			diagnostics = append(diagnostics, backupDiagnostic(SeverityWarning, "files.backup.non_regular", fmt.Sprintf("Skipping non-regular managed path %s during backup.", m.displayPath(dest)), dest))
			backups = append(backups, backup)
			continue
		}

		payloadPath := m.payloadPath(backupID, dest)
		sha, err := copyFileWithSHA(dest, payloadPath, info.Mode().Perm())
		if err != nil {
			return backups, diagnostics, err
		}
		backup.Current["sha256"] = sha
		contentReport := inspectFileSensitivity(dest)
		backup.Sensitivity = promoteSensitivity(backup.Sensitivity, contentReport)
		if contentReport.Sensitivity >= redact.SensitivitySecret {
			backup.Current["content_redacted"] = true
			backup.Current["content_sensitivity"] = string(backup.Sensitivity)
		}
		backup.PayloadRef = PayloadRef{Kind: "local_file", Path: payloadPath, SHA256: sha}
		backups = append(backups, backup)
	}

	return backups, diagnostics, nil
}

func (m *FilesModule) Apply(ctx context.Context, changes []Change, plan *Plan) ([]Result, []Diagnostic, error) {
	if !hasActionableMutation(changes) {
		return []Result{m.result(plan, PhaseApply, StatusNoop, firstChange(changes), nil)}, nil, nil
	}
	if err := m.Chez.Apply(ctx, m.RepoPath, m.SourceDir); err != nil {
		return []Result{m.result(plan, PhaseApply, StatusFailed, firstChange(changes), nil)}, nil, err
	}
	return []Result{m.result(plan, PhaseApply, StatusApplied, firstChange(changes), nil)}, nil, nil
}

func (m *FilesModule) Capture(ctx context.Context, changes []Change, plan *Plan) ([]Result, []Diagnostic, error) {
	if err := m.Chez.ReAdd(ctx, m.RepoPath, m.SourceDir); err != nil {
		return []Result{m.result(plan, PhaseCapture, StatusFailed, firstChange(changes), nil)}, nil, err
	}
	return []Result{m.result(plan, PhaseCapture, StatusCaptured, firstChange(changes), nil)}, nil, nil
}

func (m *FilesModule) Verify(ctx context.Context, operation Operation, changes []Change, plan *Plan) ([]Result, []Diagnostic, error) {
	switch operation {
	case OperationApply:
		diff, err := m.Chez.Diff(ctx, m.RepoPath, m.SourceDir)
		if err != nil {
			return []Result{m.result(plan, PhaseVerify, StatusFailed, firstChange(changes), nil)}, nil, err
		}
		current := map[string]any{"diff_empty": strings.TrimSpace(diff) == ""}
		if strings.TrimSpace(diff) != "" {
			return []Result{m.result(plan, PhaseVerify, StatusFailed, firstChange(changes), current)}, nil, fmt.Errorf("chezmoi diff still reports changes after apply")
		}
		return []Result{m.result(plan, PhaseVerify, StatusVerified, firstChange(changes), current)}, nil, nil
	case OperationCapture:
		return []Result{m.result(plan, PhaseVerify, StatusVerified, firstChange(changes), map[string]any{"capture_command": "chezmoi re-add"})}, nil, nil
	default:
		return nil, nil, nil
	}
}

func (m *FilesModule) Restore(ctx context.Context, backups []Backup) ([]Result, []Diagnostic, error) {
	var results []Result
	var diagnostics []Diagnostic
	for _, backup := range backups {
		started := m.now().UTC()
		status := StatusRestored
		if !backup.Restore.Supported {
			status = StatusSkipped
			diagnostics = append(diagnostics, backupDiagnostic(SeverityWarning, "files.restore.unsupported", "Backup is not restorable automatically.", backup.Source.Value))
		} else if dest, err := m.destinationPath(backup.Source.Value); err != nil {
			status = StatusFailed
		} else if exists, _ := backup.Current["exists"].(bool); !exists {
			if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
				status = StatusFailed
			}
		} else if backup.PayloadRef.Path == "" {
			status = StatusFailed
		} else if err := restoreFile(backup.PayloadRef.Path, dest); err != nil {
			status = StatusFailed
		}
		ended := m.now().UTC()
		results = append(results, Result{
			SchemaVersion: SchemaResultV1,
			RunID:         newID(started, "files-restore"),
			Phase:         PhaseRestore,
			Surface:       filesSurface,
			ID:            backup.ID,
			Source:        backup.Source,
			Current:       backup.Current,
			Desired:       backup.Desired,
			ManagedBy:     backup.ManagedBy,
			Sensitivity:   backup.Sensitivity,
			Confidence:    backup.Confidence,
			Capability:    backup.Capability,
			Risk:          backup.Risk,
			Status:        status,
			StartedAt:     Timestamp(started),
			EndedAt:       Timestamp(ended),
		})
	}
	return results, diagnostics, nil
}

func (m *FilesModule) planApply(ctx context.Context) ([]Change, []Diagnostic, error) {
	diff, err := m.Chez.Diff(ctx, m.RepoPath, m.SourceDir)
	if err != nil {
		return nil, nil, err
	}
	change := m.baseChange(OperationApply)
	if strings.TrimSpace(diff) == "" {
		change.Action = ActionNoop
		change.Current = map[string]any{"diff_empty": true}
		change.Desired = map[string]any{"source_dir": m.SourceDir}
		change.BackupRequired = false
		return []Change{change}, nil, nil
	}
	change.Action = ActionUpdate
	change.Current = map[string]any{"diff_empty": false, "diff_redacted": true}
	change.Desired = map[string]any{"source_dir": m.SourceDir}
	change.BackupRequired = true
	change.Risk = Risk{Level: RiskMedium, Reasons: []string{"managed files will be updated"}, RequiresConfirmation: false, Reversible: true}
	return []Change{change}, nil, nil
}

func (m *FilesModule) planCapture() []Change {
	change := m.baseChange(OperationCapture)
	change.Action = ActionUpdate
	change.Current = map[string]any{"managed_by": "chezmoi"}
	change.Desired = map[string]any{"command": "chezmoi re-add", "source_dir": m.SourceDir}
	change.BackupRequired = false
	return []Change{change}
}

func (m *FilesModule) baseChange(operation Operation) Change {
	id := "files:source/" + m.SourceDir
	return Change{
		ChangeID:       fmt.Sprintf("%s:%s", id, operation),
		Surface:        filesSurface,
		ID:             id,
		Action:         ActionNoop,
		Source:         Source{Kind: "chezmoi", Value: m.SourceDir},
		Current:        nil,
		Desired:        nil,
		ManagedBy:      []string{"dotstate", "chezmoi"},
		Sensitivity:    SensitivityLocalPath,
		Confidence:     ConfidenceConfirmed,
		Capability:     []Capability{CapabilityAutoApply},
		Risk:           LowRisk(true),
		BackupRequired: false,
		DependsOn:      []string{},
		Diagnostics:    []Diagnostic{},
	}
}

func (m *FilesModule) sourceFact(current map[string]any) Fact {
	return Fact{
		SchemaVersion: SchemaFactV1,
		Surface:       filesSurface,
		ID:            "files:source/" + m.SourceDir,
		Source:        Source{Kind: "chezmoi", Value: m.SourceDir},
		Current:       current,
		Desired:       nil,
		ManagedBy:     []string{"dotstate", "chezmoi"},
		Sensitivity:   SensitivityLocalPath,
		Confidence:    ConfidenceConfirmed,
		Capability:    []Capability{CapabilityReadOnly, CapabilityAutoApply},
		Risk:          LowRisk(true),
		Diagnostics:   []Diagnostic{},
	}
}

func (m *FilesModule) result(plan *Plan, phase Phase, status ResultStatus, change Change, current map[string]any) Result {
	started := m.now().UTC()
	ended := m.now().UTC()
	if current == nil {
		current = change.Current
	}
	return Result{
		SchemaVersion: SchemaResultV1,
		RunID:         newID(started, string(phase)+"-files"),
		PlanID:        plan.PlanID,
		Phase:         phase,
		Surface:       change.Surface,
		ID:            change.ID,
		ChangeID:      change.ChangeID,
		Source:        change.Source,
		Current:       current,
		Desired:       change.Desired,
		ManagedBy:     change.ManagedBy,
		Sensitivity:   change.Sensitivity,
		Confidence:    change.Confidence,
		Capability:    change.Capability,
		Risk:          change.Risk,
		Status:        status,
		StartedAt:     Timestamp(started),
		EndedAt:       Timestamp(ended),
		Diagnostics:   change.Diagnostics,
	}
}

func firstChange(changes []Change) Change {
	if len(changes) > 0 {
		return changes[0]
	}
	return Change{Surface: filesSurface, ID: "files:source/unknown", ManagedBy: []string{"dotstate", "chezmoi"}, Capability: []Capability{CapabilityAutoApply}, Risk: LowRisk(true), Confidence: ConfidenceUnknown, Sensitivity: SensitivityLocalPath}
}

func hasActionableMutation(changes []Change) bool {
	for _, change := range changes {
		if isMutation(change.Action) {
			return true
		}
	}
	return false
}

func (m *FilesModule) destinationPath(managedPath string) (string, error) {
	p := strings.TrimSpace(managedPath)
	if p == "" {
		return "", fmt.Errorf("empty managed path")
	}
	if p == "~" {
		return m.Home, nil
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		return filepath.Join(m.Home, p[2:]), nil
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	return filepath.Join(m.Home, p), nil
}

func (m *FilesModule) displayPath(path string) string {
	cleanHome := filepath.Clean(m.Home)
	cleanPath := filepath.Clean(path)
	if cleanPath == cleanHome {
		return "~"
	}
	if rel, err := filepath.Rel(cleanHome, cleanPath); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return "~/" + filepath.ToSlash(rel)
	}
	return cleanPath
}

func (m *FilesModule) payloadPath(backupID, dest string) string {
	rel := ""
	if candidate, err := filepath.Rel(filepath.Clean(m.Home), filepath.Clean(dest)); err == nil && candidate != "." && !strings.HasPrefix(candidate, "..") {
		rel = candidate
	} else {
		rel = strings.TrimPrefix(filepath.Clean(dest), string(filepath.Separator))
		rel = strings.ReplaceAll(rel, ":", "_")
	}
	return filepath.Join(m.BackupRoot, backupID, filesSurface, rel)
}

func backupDiagnostic(severity DiagnosticSeverity, code, message, path string) Diagnostic {
	d := NewDiagnostic(severity, code, message, filesSurface, "files:path/"+path)
	d.Sensitivity = SensitivityLocalPath
	d.Capability = []Capability{CapabilityAutoApply}
	return d
}

func fileType(info os.FileInfo) string {
	mode := info.Mode()
	switch {
	case mode.IsRegular():
		return "file"
	case mode.IsDir():
		return "directory"
	case mode&os.ModeSymlink != 0:
		return "symlink"
	default:
		return "other"
	}
}

func copyFileWithSHA(src, dst string, mode os.FileMode) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return "", err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return "", err
	}
	defer out.Close()

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(out, h), in); err != nil {
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func inspectFileSensitivity(path string) redact.Report {
	file, err := os.Open(path)
	if err != nil {
		return redact.Report{Sensitivity: redact.SensitivityPublic}
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, 10*1024*1024))
	if err != nil {
		return redact.Report{Sensitivity: redact.SensitivityPublic}
	}
	_, report := redact.String(string(content))
	return report
}

func restoreFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	_, err = copyFileWithSHA(src, dst, info.Mode().Perm())
	return err
}
