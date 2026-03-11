package service

import (
	"path/filepath"
	"testing"

	sqlstore "github.com/maplepreneur/chrono/internal/store/sqlite"
)

func newTestService(t *testing.T) *TimerService {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "chrono-test.db")
	store, err := sqlstore.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.AddClient("acme"); err != nil {
		t.Fatalf("add client: %v", err)
	}
	if err := store.AddTrackingType("dev"); err != nil {
		t.Fatalf("add type: %v", err)
	}
	return NewTimerService(store)
}

func TestSingleActiveSession(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.Start("acme", "dev", "task one"); err != nil {
		t.Fatalf("start session: %v", err)
	}
	if _, err := svc.Start("acme", "dev", "task two"); err == nil {
		t.Fatal("expected error when starting second active session")
	}
}

func TestResumeCreatesNewSegment(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.Start("acme", "dev", "task"); err != nil {
		t.Fatalf("start session: %v", err)
	}
	if _, err := svc.Stop(); err != nil {
		t.Fatalf("stop session: %v", err)
	}
	if _, err := svc.Resume(); err != nil {
		t.Fatalf("resume session: %v", err)
	}
	active, err := svc.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if active == nil {
		t.Fatal("expected active session after resume")
	}
	if active.Status != "active" {
		t.Fatalf("expected active status, got %q", active.Status)
	}
}

func TestBrandingSettingsRoundTrip(t *testing.T) {
	svc := newTestService(t)

	if err := svc.SetBrandingDisplayName("Maple Entrepreneur"); err != nil {
		t.Fatalf("set display name: %v", err)
	}
	if err := svc.SetBrandingLogoPath("/tmp/logo.png"); err != nil {
		t.Fatalf("set logo path: %v", err)
	}

	branding, err := svc.BrandingSettings()
	if err != nil {
		t.Fatalf("get branding settings: %v", err)
	}
	if branding.DisplayName != "Maple Entrepreneur" {
		t.Fatalf("expected display name, got %q", branding.DisplayName)
	}
	if branding.LogoPath != "/tmp/logo.png" {
		t.Fatalf("expected logo path, got %q", branding.LogoPath)
	}
}
