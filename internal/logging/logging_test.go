package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoggerRedactsSecretsInFileLogs(t *testing.T) {
	const sentinel = "DOTSTATE_TEST_SECRET_DO_NOT_PRINT"
	dir := t.TempDir()
	logger, err := New(Config{LogDir: dir, LogLevel: LevelDebug})
	if err != nil {
		t.Fatalf("New logger: %v", err)
	}
	logger.With("api_token", sentinel).Info("message "+sentinel, "nested", map[string]any{"password": sentinel})
	if err := logger.Close(); err != nil {
		t.Fatalf("Close logger: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "dot.log"))
	if err != nil {
		t.Fatalf("Read log: %v", err)
	}
	if strings.Contains(string(content), sentinel) {
		t.Fatalf("log leaked sentinel: %s", content)
	}
	if !strings.Contains(string(content), "<redacted:secret>") {
		t.Fatalf("log did not include redaction marker: %s", content)
	}
}
