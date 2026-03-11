package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateLogoPath(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing-logo.png")
	if err := validateLogoPath(missingPath); err == nil {
		t.Fatal("expected missing file error")
	}

	dir := t.TempDir()
	if err := validateLogoPath(dir); err == nil {
		t.Fatal("expected directory validation error")
	}

	filePath := filepath.Join(t.TempDir(), "logo.png")
	if err := os.WriteFile(filePath, []byte("logo"), 0o644); err != nil {
		t.Fatalf("write logo: %v", err)
	}
	if err := validateLogoPath(filePath); err != nil {
		t.Fatalf("expected readable file, got error: %v", err)
	}
}
