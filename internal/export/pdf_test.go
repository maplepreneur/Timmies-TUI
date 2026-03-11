package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maplepreneur/chrono/internal/store/sqlite"
)

func TestWriteReportPDF(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "report.pdf")
	started := time.Date(2026, time.January, 8, 14, 0, 0, 0, time.UTC)

	rows := []sqlite.ReportRow{
		{
			SessionID:         1,
			ClientName:        "acme",
			TrackingTypeName:  "consulting",
			Note:              "Kickoff",
			StartedAt:         started,
			ComputedDurationS: 5400,
			BillableAmount:    225,
			ResourceCostTotal: 14.5,
			MonetaryTotal:     239.5,
		},
	}
	summary := sqlite.ReportSummary{
		DurationSec:       5400,
		TimeBillableTotal: 225,
		ResourceCostTotal: 14.5,
		MonetaryTotal:     239.5,
	}

	if err := WriteReportPDF(outPath, "acme", started, started.Add(24*time.Hour), rows, summary, ReportBranding{}); err != nil {
		t.Fatalf("write pdf: %v", err)
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read pdf: %v", err)
	}
	if !strings.HasPrefix(string(content), "%PDF-1.4") {
		t.Fatalf("expected PDF header, got %q", string(content[:8]))
	}
	text := string(content)
	for _, expected := range []string{
		"duration total:",
		"time-billable total:",
		"resource total:",
		"combined monetary total:",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected PDF content to include %q", expected)
		}
	}
}

func TestWriteReportPDFWithBranding(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "report-branded.pdf")
	logoPath := filepath.Join(t.TempDir(), "logo.png")
	if err := os.WriteFile(logoPath, []byte("fake"), 0o644); err != nil {
		t.Fatalf("write logo: %v", err)
	}

	started := time.Date(2026, time.January, 8, 14, 0, 0, 0, time.UTC)
	rows := []sqlite.ReportRow{}
	summary := sqlite.ReportSummary{}

	if err := WriteReportPDF(outPath, "acme", started, started.Add(24*time.Hour), rows, summary, ReportBranding{
		DisplayName: "Maple Entrepreneur",
		LogoPath:    logoPath,
	}); err != nil {
		t.Fatalf("write branded pdf: %v", err)
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read pdf: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "Prepared by: Maple Entrepreneur") {
		t.Fatalf("expected branded display name in PDF")
	}
	if !strings.Contains(text, "Logo file: "+logoPath) {
		t.Fatalf("expected logo path in PDF")
	}
}

func TestWriteReportPDFWithUnreadableLogoReturnsError(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "report-bad-logo.pdf")
	started := time.Date(2026, time.January, 8, 14, 0, 0, 0, time.UTC)

	err := WriteReportPDF(outPath, "acme", started, started.Add(24*time.Hour), nil, sqlite.ReportSummary{}, ReportBranding{
		DisplayName: "Maple Entrepreneur",
		LogoPath:    filepath.Join(t.TempDir(), "missing-logo.png"),
	})
	if err == nil {
		t.Fatal("expected error for missing logo")
	}
	if !strings.Contains(err.Error(), "branding logo file") {
		t.Fatalf("expected branding logo error, got: %v", err)
	}
}
