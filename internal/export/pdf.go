package export

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/maplepreneur/chrono/internal/report"
	"github.com/maplepreneur/chrono/internal/store/sqlite"
)

const maxPDFLines = 52

type ReportBranding struct {
	DisplayName string
	LogoPath    string
}

func WriteReportPDF(path, client string, from, to time.Time, rows []sqlite.ReportRow, summary sqlite.ReportSummary, branding ReportBranding) error {
	if err := validateBrandingForPDF(branding); err != nil {
		return err
	}

	lines := []string{
		"Timmies TUI Report",
	}
	if branding.DisplayName != "" {
		lines = append(lines, fmt.Sprintf("Prepared by: %s", branding.DisplayName))
	}
	if branding.LogoPath != "" {
		lines = append(lines, fmt.Sprintf("Logo file: %s", branding.LogoPath))
	}
	lines = append(lines,
		fmt.Sprintf("Client: @%s", client),
		fmt.Sprintf("Period: %s -> %s", from.Format("2006-01-02"), to.Format("2006-01-02")),
		"",
		"Sessions",
	)

	if len(rows) == 0 {
		lines = append(lines, "No sessions in range.")
	}
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf(
			"%s | %s | %s | time:$%.2f resources:$%.2f total:$%.2f",
			r.StartedAt.Local().Format("2006-01-02 15:04"),
			r.TrackingTypeName,
			report.HumanDuration(r.ComputedDurationS),
			r.BillableAmount,
			r.ResourceCostTotal,
			r.MonetaryTotal,
		))
		if r.Note != "" {
			lines = append(lines, fmt.Sprintf("note: %s", r.Note))
		}
	}

	lines = append(lines,
		"",
		"Summary Totals",
		fmt.Sprintf("duration total: %s", report.HumanDuration(summary.DurationSec)),
		fmt.Sprintf("time-billable total: $%.2f", summary.TimeBillableTotal),
		fmt.Sprintf("resource total: $%.2f", summary.ResourceCostTotal),
		fmt.Sprintf("combined monetary total: $%.2f", summary.MonetaryTotal),
	)

	return writeSimplePDF(path, lines)
}

func validateBrandingForPDF(branding ReportBranding) error {
	if branding.LogoPath == "" {
		return nil
	}
	f, err := os.Open(branding.LogoPath)
	if err != nil {
		return fmt.Errorf("branding logo file %q is not readable: %w", branding.LogoPath, err)
	}
	defer f.Close()
	buf := make([]byte, 1)
	if _, err := f.Read(buf); err != nil && err != io.EOF {
		return fmt.Errorf("branding logo file %q cannot be read: %w", branding.LogoPath, err)
	}
	return nil
}

func writeSimplePDF(path string, lines []string) error {
	renderLines := lines
	if len(lines) > maxPDFLines {
		renderLines = append([]string{}, lines[:maxPDFLines-1]...)
		renderLines = append(renderLines, fmt.Sprintf("... %d more line(s) omitted", len(lines)-len(renderLines)))
	}

	var content strings.Builder
	content.WriteString("BT\n/F1 11 Tf\n14 TL\n50 800 Td\n")
	for i, line := range renderLines {
		if i > 0 {
			content.WriteString("T*\n")
		}
		fmt.Fprintf(&content, "(%s) Tj\n", sanitizePDFText(line))
	}
	content.WriteString("ET")
	contentStr := content.String()

	var out strings.Builder
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, 6)
	writeObj := func(id int, body string) {
		offsets[id] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", id, body)
	}

	writeObj(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObj(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	writeObj(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>")
	writeObj(4, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(contentStr), contentStr))
	writeObj(5, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")

	xrefOffset := out.Len()
	out.WriteString("xref\n0 6\n")
	out.WriteString("0000000000 65535 f \n")
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(&out, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&out, "trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xrefOffset)

	return os.WriteFile(path, []byte(out.String()), 0o644)
}

func sanitizePDFText(in string) string {
	in = strings.ReplaceAll(in, "\\", "\\\\")
	in = strings.ReplaceAll(in, "(", "\\(")
	in = strings.ReplaceAll(in, ")", "\\)")

	var b strings.Builder
	for _, r := range in {
		if r == '\t' || (r >= 32 && r <= 126) {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('?')
	}
	return b.String()
}
