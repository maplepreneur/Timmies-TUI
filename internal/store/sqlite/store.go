package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNoActiveSession = errors.New("no active session")

const schemaSQL = `
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS clients (
id INTEGER PRIMARY KEY AUTOINCREMENT,
name TEXT NOT NULL UNIQUE,
created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tracking_types (
id INTEGER PRIMARY KEY AUTOINCREMENT,
name TEXT NOT NULL UNIQUE,
is_billable INTEGER NOT NULL DEFAULT 0 CHECK(is_billable IN (0, 1)),
hourly_rate REAL NOT NULL DEFAULT 0 CHECK(hourly_rate >= 0),
created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
id INTEGER PRIMARY KEY AUTOINCREMENT,
client_id INTEGER,
tracking_type_id INTEGER NOT NULL,
note TEXT NOT NULL DEFAULT '',
started_at TEXT NOT NULL,
stopped_at TEXT,
duration_seconds INTEGER,
status TEXT NOT NULL CHECK(status IN ('active', 'stopped')),
created_at TEXT NOT NULL,
FOREIGN KEY(client_id) REFERENCES clients(id),
FOREIGN KEY(tracking_type_id) REFERENCES tracking_types(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_one_active
ON sessions(status) WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_sessions_client_started
ON sessions(client_id, started_at);

CREATE TABLE IF NOT EXISTS session_resources (
id INTEGER PRIMARY KEY AUTOINCREMENT,
session_id INTEGER NOT NULL,
resource_name TEXT NOT NULL,
cost_amount REAL NOT NULL CHECK(cost_amount >= 0),
created_at TEXT NOT NULL,
FOREIGN KEY(session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_session_resources_session
ON session_resources(session_id);

CREATE TABLE IF NOT EXISTS settings (
key TEXT PRIMARY KEY,
value TEXT NOT NULL,
created_at TEXT NOT NULL,
updated_at TEXT NOT NULL
);
`

type Store struct {
	db *sql.DB
}

type SessionView struct {
	ID               int64
	ClientName       string
	TrackingTypeName string
	Note             string
	StartedAt        time.Time
	StoppedAt        *time.Time
	DurationSec      int64
	Status           string
}

type TrackingTypeView struct {
	Name       string
	IsBillable bool
	HourlyRate float64
}

type ReportRow struct {
	SessionID         int64
	ClientName        string
	TrackingTypeName  string
	IsBillable        bool
	HourlyRate        float64
	BillableAmount    float64
	Note              string
	StartedAt         time.Time
	StoppedAt         *time.Time
	DurationSec       int64
	ComputedDurationS int64
	ResourceCostTotal float64
	MonetaryTotal     float64
}

type DurationAmountTotal struct {
	Name         string
	DurationSec  int64
	AmountTotal  float64
	BillableOnly bool
}

type ReportSummary struct {
	DurationSec       int64
	TimeBillableTotal float64
	ResourceCostTotal float64
	MonetaryTotal     float64
}

type SessionResourceView struct {
	ID           int64
	SessionID    int64
	ResourceName string
	CostAmount   float64
	CreatedAt    time.Time
}

type BrandingSettings struct {
	DisplayName string
	LogoPath    string
}

const (
	settingKeyDisplayName = "branding.display_name"
	settingKeyLogoPath    = "branding.logo_path"
)

func Open(path string) (*Store, error) {
	if path == "" {
		path = "tim.db"
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err = db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err = db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := ensureTrackingTypeBillingColumns(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := ensureClientIDNullable(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func ensureTrackingTypeBillingColumns(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(tracking_types)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasBillable := false
	hasRate := false
	for rows.Next() {
		var cid int
		var name, colType string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dflt, &pk); err != nil {
			return err
		}
		switch name {
		case "is_billable":
			hasBillable = true
		case "hourly_rate":
			hasRate = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if !hasBillable {
		if _, err := db.Exec(`ALTER TABLE tracking_types ADD COLUMN is_billable INTEGER NOT NULL DEFAULT 0 CHECK(is_billable IN (0, 1))`); err != nil {
			return err
		}
	}
	if !hasRate {
		if _, err := db.Exec(`ALTER TABLE tracking_types ADD COLUMN hourly_rate REAL NOT NULL DEFAULT 0 CHECK(hourly_rate >= 0)`); err != nil {
			return err
		}
	}
	return nil
}

func ensureClientIDNullable(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(sessions)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	needsMigration := false
	for rows.Next() {
		var cid int
		var name, colType string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == "client_id" && notnull == 1 {
			needsMigration = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !needsMigration {
		return nil
	}

	if _, err := db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		return err
	}
	defer db.Exec("PRAGMA foreign_keys = ON")

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmts := []string{
		`CREATE TABLE sessions_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			client_id INTEGER,
			tracking_type_id INTEGER NOT NULL,
			note TEXT NOT NULL DEFAULT '',
			started_at TEXT NOT NULL,
			stopped_at TEXT,
			duration_seconds INTEGER,
			status TEXT NOT NULL CHECK(status IN ('active', 'stopped')),
			created_at TEXT NOT NULL,
			FOREIGN KEY(client_id) REFERENCES clients(id),
			FOREIGN KEY(tracking_type_id) REFERENCES tracking_types(id)
		)`,
		`INSERT INTO sessions_new SELECT * FROM sessions`,
		`DROP TABLE sessions`,
		`ALTER TABLE sessions_new RENAME TO sessions`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_one_active ON sessions(status) WHERE status = 'active'`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_client_started ON sessions(client_id, started_at)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("client_id migration: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) SetBrandingDisplayName(name string) error {
	return s.setSetting(settingKeyDisplayName, name)
}

func (s *Store) SetBrandingLogoPath(path string) error {
	return s.setSetting(settingKeyLogoPath, path)
}

func (s *Store) GetBrandingSettings() (BrandingSettings, error) {
	displayName, err := s.getSetting(settingKeyDisplayName)
	if err != nil {
		return BrandingSettings{}, err
	}
	logoPath, err := s.getSetting(settingKeyLogoPath)
	if err != nil {
		return BrandingSettings{}, err
	}
	return BrandingSettings{
		DisplayName: displayName,
		LogoPath:    logoPath,
	}, nil
}

func (s *Store) setSetting(key, value string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
INSERT INTO settings(key, value, created_at, updated_at)
VALUES(?, ?, ?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at
`, key, value, now, now)
	return err
}

func (s *Store) getSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

func (s *Store) AddClient(name string) error {
	_, err := s.db.Exec(`INSERT INTO clients(name, created_at) VALUES(?, ?)`, name, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *Store) AddTrackingType(name string) error {
	return s.AddTrackingTypeWithBilling(name, false, 0)
}

func (s *Store) AddTrackingTypeWithBilling(name string, isBillable bool, hourlyRate float64) error {
	if hourlyRate < 0 {
		return fmt.Errorf("hourly rate must be >= 0")
	}
	if !isBillable {
		hourlyRate = 0
	}
	billableInt := 0
	if isBillable {
		billableInt = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO tracking_types(name, is_billable, hourly_rate, created_at) VALUES(?, ?, ?, ?)`,
		name,
		billableInt,
		hourlyRate,
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Store) ListClients() ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM clients ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func (s *Store) ListTrackingTypes() ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM tracking_types ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func (s *Store) ListTrackingTypeDetails() ([]TrackingTypeView, error) {
	rows, err := s.db.Query(`SELECT name, is_billable, hourly_rate FROM tracking_types ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TrackingTypeView
	for rows.Next() {
		var t TrackingTypeView
		var billableInt int
		if err := rows.Scan(&t.Name, &billableInt, &t.HourlyRate); err != nil {
			return nil, err
		}
		t.IsBillable = billableInt == 1
		if !t.IsBillable {
			t.HourlyRate = 0
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) StartSession(clientName, trackingTypeName, note string, startedAt time.Time) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var activeCount int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM sessions WHERE status='active'`).Scan(&activeCount); err != nil {
		return 0, err
	}
	if activeCount > 0 {
		return 0, fmt.Errorf("an active session already exists")
	}

	var clientID sql.NullInt64
	if clientName != "" {
		var cid int64
		if err = tx.QueryRow(`SELECT id FROM clients WHERE name = ?`, clientName).Scan(&cid); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, fmt.Errorf("client not found: %s", clientName)
			}
			return 0, err
		}
		clientID = sql.NullInt64{Int64: cid, Valid: true}
	}

	var trackingTypeID int64
	if err = tx.QueryRow(`SELECT id FROM tracking_types WHERE name = ?`, trackingTypeName).Scan(&trackingTypeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("tracking type not found: %s", trackingTypeName)
		}
		return 0, err
	}

	res, err := tx.Exec(`
INSERT INTO sessions(client_id, tracking_type_id, note, started_at, status, created_at)
VALUES(?, ?, ?, ?, 'active', ?)
`, clientID, trackingTypeID, note, startedAt.UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) StopActiveSession(stoppedAt time.Time) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var id int64
	var startedAtRaw string
	if err = tx.QueryRow(`SELECT id, started_at FROM sessions WHERE status='active' LIMIT 1`).Scan(&id, &startedAtRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNoActiveSession
		}
		return 0, err
	}

	startedAt, err := time.Parse(time.RFC3339, startedAtRaw)
	if err != nil {
		return 0, err
	}
	dur := int64(stoppedAt.UTC().Sub(startedAt).Seconds())
	if dur < 0 {
		return 0, fmt.Errorf("stopped time is before started time")
	}

	_, err = tx.Exec(`
UPDATE sessions
SET status='stopped', stopped_at=?, duration_seconds=?
WHERE id=?
`, stoppedAt.UTC().Format(time.RFC3339), dur, id)
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) GetActiveSession() (*SessionView, error) {
	row := s.db.QueryRow(`
SELECT s.id, COALESCE(c.name, ''), t.name, s.note, s.started_at, s.stopped_at,
       COALESCE(s.duration_seconds, 0), s.status
FROM sessions s
LEFT JOIN clients c ON c.id = s.client_id
JOIN tracking_types t ON t.id = s.tracking_type_id
WHERE s.status='active'
LIMIT 1
`)

	var out SessionView
	var startedRaw string
	var stoppedRaw sql.NullString
	if err := row.Scan(&out.ID, &out.ClientName, &out.TrackingTypeName, &out.Note, &startedRaw, &stoppedRaw, &out.DurationSec, &out.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	startedAt, err := time.Parse(time.RFC3339, startedRaw)
	if err != nil {
		return nil, err
	}
	out.StartedAt = startedAt
	if stoppedRaw.Valid {
		t, err := time.Parse(time.RFC3339, stoppedRaw.String)
		if err != nil {
			return nil, err
		}
		out.StoppedAt = &t
	}
	return &out, nil
}

func (s *Store) ResumeLatest(startedAt time.Time) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var activeCount int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM sessions WHERE status='active'`).Scan(&activeCount); err != nil {
		return 0, err
	}
	if activeCount > 0 {
		return 0, fmt.Errorf("an active session already exists")
	}

	var clientID sql.NullInt64
	var trackingTypeID int64
	var note string
	if err = tx.QueryRow(`
SELECT client_id, tracking_type_id, note
FROM sessions
WHERE status='stopped'
ORDER BY stopped_at DESC
LIMIT 1
`).Scan(&clientID, &trackingTypeID, &note); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("no stopped session to resume")
		}
		return 0, err
	}

	res, err := tx.Exec(`
INSERT INTO sessions(client_id, tracking_type_id, note, started_at, status, created_at)
VALUES(?, ?, ?, ?, 'active', ?)
`, clientID, trackingTypeID, note, startedAt.UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) ResumePausedSession(sessionID int64, startedAt time.Time) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var activeCount int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM sessions WHERE status='active'`).Scan(&activeCount); err != nil {
		return 0, err
	}
	if activeCount > 0 {
		return 0, fmt.Errorf("an active session already exists")
	}

	var clientID sql.NullInt64
	var trackingTypeID int64
	var note string
	if err = tx.QueryRow(`
SELECT client_id, tracking_type_id, note
FROM sessions
WHERE status='stopped' AND id=?
LIMIT 1
`, sessionID).Scan(&clientID, &trackingTypeID, &note); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("stopped session not found: %d", sessionID)
		}
		return 0, err
	}

	res, err := tx.Exec(`
INSERT INTO sessions(client_id, tracking_type_id, note, started_at, status, created_at)
VALUES(?, ?, ?, ?, 'active', ?)
`, clientID, trackingTypeID, note, startedAt.UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) AddSessionResource(sessionID int64, resourceName string, costAmount float64) error {
	if resourceName == "" {
		return fmt.Errorf("resource name is required")
	}
	if costAmount < 0 {
		return fmt.Errorf("resource cost must be >= 0")
	}
	_, err := s.db.Exec(
		`INSERT INTO session_resources(session_id, resource_name, cost_amount, created_at) VALUES(?, ?, ?, ?)`,
		sessionID,
		resourceName,
		costAmount,
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Store) ListSessionResources(sessionID int64) ([]SessionResourceView, error) {
	rows, err := s.db.Query(`
SELECT id, session_id, resource_name, cost_amount, created_at
FROM session_resources
WHERE session_id = ?
ORDER BY created_at ASC, id ASC
`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SessionResourceView
	for rows.Next() {
		var r SessionResourceView
		var createdAtRaw string
		if err := rows.Scan(&r.ID, &r.SessionID, &r.ResourceName, &r.CostAmount, &createdAtRaw); err != nil {
			return nil, err
		}
		createdAt, err := time.Parse(time.RFC3339, createdAtRaw)
		if err != nil {
			return nil, err
		}
		r.CreatedAt = createdAt
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ReportByClient(clientName string, from, to time.Time) ([]ReportRow, ReportSummary, error) {
	query := `
SELECT s.id, COALESCE(c.name, '(no client)'), t.name, t.is_billable, t.hourly_rate, s.note, s.started_at, s.stopped_at,
       COALESCE(s.duration_seconds, 0), COALESCE(sr.resource_total, 0)
FROM sessions s
LEFT JOIN clients c ON c.id = s.client_id
JOIN tracking_types t ON t.id = s.tracking_type_id
LEFT JOIN (
	SELECT session_id, SUM(cost_amount) AS resource_total
	FROM session_resources
	GROUP BY session_id
) sr ON sr.session_id = s.id
WHERE s.started_at >= ?
  AND s.started_at <= ?`
	args := []any{from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339)}
	if clientName != "" {
		query += `
  AND (c.name = ? OR (? = '(no client)' AND s.client_id IS NULL))`
		args = append(args, clientName, clientName)
	}
	query += `
ORDER BY s.started_at ASC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, ReportSummary{}, err
	}
	defer rows.Close()

	var result []ReportRow
	var summary ReportSummary
	now := time.Now().UTC()
	for rows.Next() {
		var r ReportRow
		var startedRaw string
		var stoppedRaw sql.NullString
		var isBillableInt int
		if err := rows.Scan(&r.SessionID, &r.ClientName, &r.TrackingTypeName, &isBillableInt, &r.HourlyRate, &r.Note, &startedRaw, &stoppedRaw, &r.DurationSec, &r.ResourceCostTotal); err != nil {
			return nil, ReportSummary{}, err
		}
		r.IsBillable = isBillableInt == 1
		if !r.IsBillable {
			r.HourlyRate = 0
		}
		start, err := time.Parse(time.RFC3339, startedRaw)
		if err != nil {
			return nil, ReportSummary{}, err
		}
		r.StartedAt = start
		if stoppedRaw.Valid {
			t, err := time.Parse(time.RFC3339, stoppedRaw.String)
			if err != nil {
				return nil, ReportSummary{}, err
			}
			r.StoppedAt = &t
			r.ComputedDurationS = int64(t.Sub(start).Seconds())
		} else {
			r.ComputedDurationS = int64(now.Sub(start).Seconds())
		}
		if r.ComputedDurationS < 0 {
			r.ComputedDurationS = 0
		}
		if r.IsBillable {
			r.BillableAmount = (float64(r.ComputedDurationS) / 3600.0) * r.HourlyRate
		}
		r.MonetaryTotal = r.BillableAmount + r.ResourceCostTotal
		summary.DurationSec += r.ComputedDurationS
		summary.TimeBillableTotal += r.BillableAmount
		summary.ResourceCostTotal += r.ResourceCostTotal
		summary.MonetaryTotal += r.MonetaryTotal
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, ReportSummary{}, err
	}
	return result, summary, nil
}

func (s *Store) DashboardTotalsByClient(from, to time.Time) ([]DurationAmountTotal, error) {
	rows, err := s.dashboardRows(from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return aggregateTotals(rows, true)
}

func (s *Store) DashboardTotalsByTrackingType(from, to time.Time) ([]DurationAmountTotal, error) {
	rows, err := s.dashboardRows(from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return aggregateTotals(rows, false)
}

func (s *Store) dashboardRows(from, to time.Time) (*sql.Rows, error) {
	return s.db.Query(`
SELECT COALESCE(c.name, '(no client)'), t.name, t.is_billable, t.hourly_rate, s.started_at, s.stopped_at
FROM sessions s
LEFT JOIN clients c ON c.id = s.client_id
JOIN tracking_types t ON t.id = s.tracking_type_id
WHERE s.started_at >= ?
  AND s.started_at <= ?
`, from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339))
}

func aggregateTotals(rows *sql.Rows, byClient bool) ([]DurationAmountTotal, error) {
	type agg struct {
		dur int64
		amt float64
	}
	acc := map[string]agg{}
	now := time.Now().UTC()
	for rows.Next() {
		var clientName, typeName string
		var isBillableInt int
		var hourlyRate float64
		var startedRaw string
		var stoppedRaw sql.NullString
		if err := rows.Scan(&clientName, &typeName, &isBillableInt, &hourlyRate, &startedRaw, &stoppedRaw); err != nil {
			return nil, err
		}
		start, err := time.Parse(time.RFC3339, startedRaw)
		if err != nil {
			return nil, err
		}
		stop := now
		if stoppedRaw.Valid {
			parsedStop, err := time.Parse(time.RFC3339, stoppedRaw.String)
			if err != nil {
				return nil, err
			}
			stop = parsedStop
		}
		dur := int64(stop.Sub(start).Seconds())
		if dur < 0 {
			dur = 0
		}
		key := typeName
		if byClient {
			key = clientName
		}
		a := acc[key]
		a.dur += dur
		if isBillableInt == 1 {
			a.amt += (float64(dur) / 3600.0) * hourlyRate
		}
		acc[key] = a
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]DurationAmountTotal, 0, len(acc))
	for name, v := range acc {
		out = append(out, DurationAmountTotal{Name: name, DurationSec: v.dur, AmountTotal: v.amt})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out, nil
}

type PausedSessionView struct {
	ID                int64
	ClientName        string
	TrackingTypeName  string
	Note              string
	StoppedAt         time.Time
	DurationSec       int64
	ResourceCostTotal float64
}

func (s *Store) ListPausedSessions(limit int) ([]PausedSessionView, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := s.db.Query(`
SELECT s.id, COALESCE(c.name, '(no client)'), t.name, s.note, s.stopped_at, COALESCE(s.duration_seconds, 0), COALESCE(sr.resource_total, 0)
FROM sessions s
LEFT JOIN clients c ON c.id = s.client_id
JOIN tracking_types t ON t.id = s.tracking_type_id
LEFT JOIN (
	SELECT session_id, SUM(cost_amount) AS resource_total
	FROM session_resources
	GROUP BY session_id
) sr ON sr.session_id = s.id
WHERE s.status='stopped' AND s.stopped_at IS NOT NULL
ORDER BY s.stopped_at DESC
LIMIT ?
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PausedSessionView
	for rows.Next() {
		var p PausedSessionView
		var stoppedRaw string
		if err := rows.Scan(&p.ID, &p.ClientName, &p.TrackingTypeName, &p.Note, &stoppedRaw, &p.DurationSec, &p.ResourceCostTotal); err != nil {
			return nil, err
		}
		stoppedAt, err := time.Parse(time.RFC3339, stoppedRaw)
		if err != nil {
			return nil, err
		}
		p.StoppedAt = stoppedAt
		out = append(out, p)
	}
	return out, rows.Err()
}

type DetailSessionView struct {
	ID                int64
	ClientName        string
	TrackingTypeName  string
	Note              string
	StartedAt         time.Time
	StoppedAt         *time.Time
	DurationSec       int64
	IsBillable        bool
	HourlyRate        float64
	BillableAmount    float64
	ResourceCostTotal float64
	MonetaryTotal     float64
}

func (s *Store) ListSessionsByClient(clientName string, from, to time.Time) ([]DetailSessionView, error) {
	var filter string
	var args []any
	if clientName == "(no client)" {
		filter = "s.client_id IS NULL"
	} else {
		filter = "c.name = ?"
		args = append(args, clientName)
	}
	args = append(args, from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339))
	return s.listDetailSessions(filter, args)
}

func (s *Store) ListSessionsByTrackingType(typeName string, from, to time.Time) ([]DetailSessionView, error) {
	return s.listDetailSessions("t.name = ?", []any{typeName, from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339)})
}

func (s *Store) listDetailSessions(filter string, args []any) ([]DetailSessionView, error) {
	query := fmt.Sprintf(`
SELECT s.id, COALESCE(c.name, '(no client)'), t.name, s.note, s.started_at, s.stopped_at,
       COALESCE(s.duration_seconds, 0), t.is_billable, t.hourly_rate, COALESCE(sr.resource_total, 0)
FROM sessions s
LEFT JOIN clients c ON c.id = s.client_id
JOIN tracking_types t ON t.id = s.tracking_type_id
LEFT JOIN (
	SELECT session_id, SUM(cost_amount) AS resource_total
	FROM session_resources
	GROUP BY session_id
) sr ON sr.session_id = s.id
WHERE %s AND s.started_at >= ? AND s.started_at <= ?
ORDER BY s.started_at DESC`, filter)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now().UTC()
	var out []DetailSessionView
	for rows.Next() {
		var d DetailSessionView
		var startedRaw string
		var stoppedRaw sql.NullString
		var isBillableInt int
		if err := rows.Scan(&d.ID, &d.ClientName, &d.TrackingTypeName, &d.Note, &startedRaw, &stoppedRaw, &d.DurationSec, &isBillableInt, &d.HourlyRate, &d.ResourceCostTotal); err != nil {
			return nil, err
		}
		d.IsBillable = isBillableInt == 1
		start, err := time.Parse(time.RFC3339, startedRaw)
		if err != nil {
			return nil, err
		}
		d.StartedAt = start
		if stoppedRaw.Valid {
			t, err := time.Parse(time.RFC3339, stoppedRaw.String)
			if err != nil {
				return nil, err
			}
			d.StoppedAt = &t
			d.DurationSec = int64(t.Sub(start).Seconds())
		} else {
			d.DurationSec = int64(now.Sub(start).Seconds())
		}
		if d.DurationSec < 0 {
			d.DurationSec = 0
		}
		if d.IsBillable {
			d.BillableAmount = (float64(d.DurationSec) / 3600.0) * d.HourlyRate
		}
		d.MonetaryTotal = d.BillableAmount + d.ResourceCostTotal
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) RenameClient(oldName, newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return fmt.Errorf("new client name is required")
	}
	res, err := s.db.Exec(`UPDATE clients SET name = ? WHERE name = ?`, newName, oldName)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("client not found: %s", oldName)
	}
	return nil
}

func (s *Store) CountSessionsByClient(name string) (int, error) {
	var count int
	err := s.db.QueryRow(`
SELECT COUNT(*) FROM sessions s
JOIN clients c ON c.id = s.client_id
WHERE c.name = ?`, name).Scan(&count)
	return count, err
}

func (s *Store) DeleteClient(name string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var clientID int64
	if err := tx.QueryRow(`SELECT id FROM clients WHERE name = ?`, name).Scan(&clientID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("client not found: %s", name)
		}
		return err
	}

	var activeCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM sessions WHERE client_id = ? AND status = 'active'`, clientID).Scan(&activeCount); err != nil {
		return err
	}
	if activeCount > 0 {
		return fmt.Errorf("cannot delete client with an active session — stop it first")
	}

	if _, err := tx.Exec(`DELETE FROM session_resources WHERE session_id IN (SELECT id FROM sessions WHERE client_id = ?)`, clientID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE client_id = ?`, clientID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM clients WHERE id = ?`, clientID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) UpdateTrackingType(oldName, newName string, isBillable bool, hourlyRate float64) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return fmt.Errorf("tracking type name is required")
	}
	if hourlyRate < 0 {
		return fmt.Errorf("hourly rate must be >= 0")
	}
	if !isBillable {
		hourlyRate = 0
	}
	billableInt := 0
	if isBillable {
		billableInt = 1
	}
	res, err := s.db.Exec(`UPDATE tracking_types SET name = ?, is_billable = ?, hourly_rate = ? WHERE name = ?`, newName, billableInt, hourlyRate, oldName)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("tracking type not found: %s", oldName)
	}
	return nil
}

func (s *Store) CountSessionsByTrackingType(name string) (int, error) {
	var count int
	err := s.db.QueryRow(`
SELECT COUNT(*) FROM sessions s
JOIN tracking_types t ON t.id = s.tracking_type_id
WHERE t.name = ?`, name).Scan(&count)
	return count, err
}

func (s *Store) DeleteTrackingType(name string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var typeID int64
	if err := tx.QueryRow(`SELECT id FROM tracking_types WHERE name = ?`, name).Scan(&typeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("tracking type not found: %s", name)
		}
		return err
	}

	var activeCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM sessions WHERE tracking_type_id = ? AND status = 'active'`, typeID).Scan(&activeCount); err != nil {
		return err
	}
	if activeCount > 0 {
		return fmt.Errorf("cannot delete tracking type with an active session — stop it first")
	}

	if _, err := tx.Exec(`DELETE FROM session_resources WHERE session_id IN (SELECT id FROM sessions WHERE tracking_type_id = ?)`, typeID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE tracking_type_id = ?`, typeID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM tracking_types WHERE id = ?`, typeID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteSession(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status string
	if err := tx.QueryRow(`SELECT status FROM sessions WHERE id = ?`, id).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("session not found: %d", id)
		}
		return err
	}
	if status == "active" {
		return fmt.Errorf("cannot delete an active session — stop it first")
	}

	if _, err := tx.Exec(`DELETE FROM session_resources WHERE session_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}
