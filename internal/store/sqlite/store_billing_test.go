package sqlite

import (
	"database/sql"
	"math"
	"path/filepath"
	"testing"
	"time"
)

func trackingTypeColumns(t *testing.T, db *sql.DB) map[string]bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(tracking_types)`)
	if err != nil {
		t.Fatalf("pragma table_info: %v", err)
	}
	defer rows.Close()

	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan table info: %v", err)
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table info: %v", err)
	}
	return cols
}

func sessionResourceColumns(t *testing.T, db *sql.DB) map[string]bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(session_resources)`)
	if err != nil {
		t.Fatalf("pragma table_info: %v", err)
	}
	defer rows.Close()

	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan table info: %v", err)
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table info: %v", err)
	}
	return cols
}

func settingsColumns(t *testing.T, db *sql.DB) map[string]bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(settings)`)
	if err != nil {
		t.Fatalf("pragma table_info settings: %v", err)
	}
	defer rows.Close()

	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan settings table info: %v", err)
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate settings table info: %v", err)
	}
	return cols
}

func TestOpenNewDBIncludesBillingColumns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "new.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cols := trackingTypeColumns(t, store.db)
	if !cols["is_billable"] {
		t.Fatal("expected is_billable column")
	}
	if !cols["hourly_rate"] {
		t.Fatal("expected hourly_rate column")
	}
	resourceCols := sessionResourceColumns(t, store.db)
	for _, col := range []string{"session_id", "resource_name", "cost_amount", "created_at"} {
		if !resourceCols[col] {
			t.Fatalf("expected session_resources.%s column", col)
		}
	}
	settingsCols := settingsColumns(t, store.db)
	for _, col := range []string{"key", "value", "created_at", "updated_at"} {
		if !settingsCols[col] {
			t.Fatalf("expected settings.%s column", col)
		}
	}
}

func TestOpenMigratesExistingDBWithMissingBillingColumns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "existing.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw sqlite db: %v", err)
	}
	_, err = db.Exec(`
CREATE TABLE tracking_types (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	created_at TEXT NOT NULL
);
`)
	if err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close raw sqlite db: %v", err)
	}

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open migrated store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cols := trackingTypeColumns(t, store.db)
	if !cols["is_billable"] {
		t.Fatal("expected migrated is_billable column")
	}
	if !cols["hourly_rate"] {
		t.Fatal("expected migrated hourly_rate column")
	}
	resourceCols := sessionResourceColumns(t, store.db)
	if !resourceCols["cost_amount"] {
		t.Fatal("expected migrated session_resources table with cost_amount column")
	}
}

func TestAddTrackingTypeWithBillingForcesZeroWhenNonBillable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "billing.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.AddTrackingTypeWithBilling("internal", false, 125.5); err != nil {
		t.Fatalf("add non-billable type: %v", err)
	}
	if err := store.AddTrackingTypeWithBilling("client", true, 200); err != nil {
		t.Fatalf("add billable type: %v", err)
	}

	details, err := store.ListTrackingTypeDetails()
	if err != nil {
		t.Fatalf("list tracking type details: %v", err)
	}

	got := map[string]TrackingTypeView{}
	for _, d := range details {
		got[d.Name] = d
	}
	if got["internal"].IsBillable {
		t.Fatal("internal should not be billable")
	}
	if got["internal"].HourlyRate != 0 {
		t.Fatalf("internal hourly rate should be 0, got %v", got["internal"].HourlyRate)
	}
	if !got["client"].IsBillable {
		t.Fatal("client should be billable")
	}
	if got["client"].HourlyRate != 200 {
		t.Fatalf("client hourly rate should be 200, got %v", got["client"].HourlyRate)
	}
}

func TestReportByClientIncludesBillingAndTotals(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "report-billing.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.AddClient("acme"); err != nil {
		t.Fatalf("add client: %v", err)
	}
	if err := store.AddTrackingTypeWithBilling("client-work", true, 120); err != nil {
		t.Fatalf("add billable type: %v", err)
	}
	if err := store.AddTrackingTypeWithBilling("internal", false, 500); err != nil {
		t.Fatalf("add non-billable type: %v", err)
	}

	startBillable := time.Date(2025, 1, 2, 10, 0, 0, 0, time.UTC)
	billableSessionID, err := store.StartSession("acme", "client-work", "feature work", startBillable)
	if err != nil {
		t.Fatalf("start billable session: %v", err)
	}
	if _, err := store.StopActiveSession(startBillable.Add(90 * time.Minute)); err != nil {
		t.Fatalf("stop billable session: %v", err)
	}

	startNonBillable := time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC)
	nonBillableSessionID, err := store.StartSession("acme", "internal", "ops", startNonBillable)
	if err != nil {
		t.Fatalf("start non-billable session: %v", err)
	}
	if _, err := store.StopActiveSession(startNonBillable.Add(30 * time.Minute)); err != nil {
		t.Fatalf("stop non-billable session: %v", err)
	}

	from := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 2, 23, 59, 59, 0, time.UTC)
	if err := store.AddSessionResource(billableSessionID, "ai_tokens", 12.5); err != nil {
		t.Fatalf("add session resource: %v", err)
	}
	if err := store.AddSessionResource(billableSessionID, "gpu_minutes", 7.5); err != nil {
		t.Fatalf("add session resource: %v", err)
	}
	if err := store.AddSessionResource(nonBillableSessionID, "tool_license", 5); err != nil {
		t.Fatalf("add session resource: %v", err)
	}

	rows, summary, err := store.ReportByClient("acme", from, to)
	if err != nil {
		t.Fatalf("report by client: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	if summary.DurationSec != 7200 {
		t.Fatalf("expected total duration 7200, got %d", summary.DurationSec)
	}
	if math.Abs(summary.TimeBillableTotal-180) > 0.0001 {
		t.Fatalf("expected total time amount 180, got %.4f", summary.TimeBillableTotal)
	}
	if math.Abs(summary.ResourceCostTotal-25) > 0.0001 {
		t.Fatalf("expected total resource amount 25, got %.4f", summary.ResourceCostTotal)
	}
	if math.Abs(summary.MonetaryTotal-205) > 0.0001 {
		t.Fatalf("expected combined total 205, got %.4f", summary.MonetaryTotal)
	}

	rowByType := map[string]ReportRow{}
	for _, row := range rows {
		rowByType[row.TrackingTypeName] = row
	}

	billable := rowByType["client-work"]
	if !billable.IsBillable {
		t.Fatal("client-work row should be billable")
	}
	if billable.HourlyRate != 120 {
		t.Fatalf("expected billable hourly rate 120, got %v", billable.HourlyRate)
	}
	if billable.ComputedDurationS != 5400 {
		t.Fatalf("expected billable duration 5400, got %d", billable.ComputedDurationS)
	}
	if math.Abs(billable.BillableAmount-180) > 0.0001 {
		t.Fatalf("expected billable amount 180, got %.4f", billable.BillableAmount)
	}
	if math.Abs(billable.ResourceCostTotal-20) > 0.0001 {
		t.Fatalf("expected billable resource total 20, got %.4f", billable.ResourceCostTotal)
	}
	if math.Abs(billable.MonetaryTotal-200) > 0.0001 {
		t.Fatalf("expected billable monetary total 200, got %.4f", billable.MonetaryTotal)
	}

	nonBillable := rowByType["internal"]
	if nonBillable.IsBillable {
		t.Fatal("internal row should not be billable")
	}
	if nonBillable.HourlyRate != 0 {
		t.Fatalf("expected non-billable hourly rate 0, got %v", nonBillable.HourlyRate)
	}
	if nonBillable.ComputedDurationS != 1800 {
		t.Fatalf("expected non-billable duration 1800, got %d", nonBillable.ComputedDurationS)
	}
	if nonBillable.BillableAmount != 0 {
		t.Fatalf("expected non-billable amount 0, got %.4f", nonBillable.BillableAmount)
	}
	if math.Abs(nonBillable.ResourceCostTotal-5) > 0.0001 {
		t.Fatalf("expected non-billable resource total 5, got %.4f", nonBillable.ResourceCostTotal)
	}
	if math.Abs(nonBillable.MonetaryTotal-5) > 0.0001 {
		t.Fatalf("expected non-billable monetary total 5, got %.4f", nonBillable.MonetaryTotal)
	}
}

func TestSessionResourcesAddAndListValidation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "session-resources.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.AddClient("acme"); err != nil {
		t.Fatalf("add client: %v", err)
	}
	if err := store.AddTrackingType("dev"); err != nil {
		t.Fatalf("add tracking type: %v", err)
	}
	start := time.Date(2025, 2, 1, 9, 0, 0, 0, time.UTC)
	sessionID, err := store.StartSession("acme", "dev", "work", start)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	if err := store.AddSessionResource(sessionID, "", 1); err == nil {
		t.Fatal("expected resource name validation error")
	}
	if err := store.AddSessionResource(sessionID, "tokens", -1); err == nil {
		t.Fatal("expected resource cost validation error")
	}
	if err := store.AddSessionResource(sessionID, "tokens", 2.25); err != nil {
		t.Fatalf("add first resource: %v", err)
	}
	if err := store.AddSessionResource(sessionID, "tool", 3.75); err != nil {
		t.Fatalf("add second resource: %v", err)
	}

	got, err := store.ListSessionResources(sessionID)
	if err != nil {
		t.Fatalf("list resources: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(got))
	}
	if got[0].SessionID != sessionID || got[0].ResourceName != "tokens" || math.Abs(got[0].CostAmount-2.25) > 0.0001 {
		t.Fatalf("unexpected first resource: %#v", got[0])
	}
	if got[1].SessionID != sessionID || got[1].ResourceName != "tool" || math.Abs(got[1].CostAmount-3.75) > 0.0001 {
		t.Fatalf("unexpected second resource: %#v", got[1])
	}
}

func TestBrandingSettingsPersistAcrossReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "branding-settings.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.SetBrandingDisplayName("Maple Entrepreneur"); err != nil {
		t.Fatalf("set display name: %v", err)
	}
	if err := store.SetBrandingLogoPath("/tmp/logo.png"); err != nil {
		t.Fatalf("set logo path: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	reopened, err := Open(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	branding, err := reopened.GetBrandingSettings()
	if err != nil {
		t.Fatalf("get branding: %v", err)
	}
	if branding.DisplayName != "Maple Entrepreneur" {
		t.Fatalf("expected display name persisted, got %q", branding.DisplayName)
	}
	if branding.LogoPath != "/tmp/logo.png" {
		t.Fatalf("expected logo path persisted, got %q", branding.LogoPath)
	}
}
