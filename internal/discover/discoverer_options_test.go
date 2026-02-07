package discover

import "testing"

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
