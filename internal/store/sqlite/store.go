package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
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
created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
id INTEGER PRIMARY KEY AUTOINCREMENT,
client_id INTEGER NOT NULL,
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

type ReportRow struct {
	SessionID         int64
	ClientName        string
	TrackingTypeName  string
	Note              string
	StartedAt         time.Time
	StoppedAt         *time.Time
	DurationSec       int64
	ComputedDurationS int64
}

func Open(path string) (*Store, error) {
	if path == "" {
		path = "chrono.db"
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
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) AddClient(name string) error {
	_, err := s.db.Exec(`INSERT INTO clients(name, created_at) VALUES(?, ?)`, name, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *Store) AddTrackingType(name string) error {
	_, err := s.db.Exec(`INSERT INTO tracking_types(name, created_at) VALUES(?, ?)`, name, time.Now().UTC().Format(time.RFC3339))
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

	var clientID int64
	if err = tx.QueryRow(`SELECT id FROM clients WHERE name = ?`, clientName).Scan(&clientID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("client not found: %s", clientName)
		}
		return 0, err
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
SELECT s.id, c.name, t.name, s.note, s.started_at, s.stopped_at,
       COALESCE(s.duration_seconds, 0), s.status
FROM sessions s
JOIN clients c ON c.id = s.client_id
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

	var clientID, trackingTypeID int64
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

func (s *Store) ReportByClient(clientName string, from, to time.Time) ([]ReportRow, int64, error) {
	rows, err := s.db.Query(`
SELECT s.id, c.name, t.name, s.note, s.started_at, s.stopped_at,
       COALESCE(s.duration_seconds, 0)
FROM sessions s
JOIN clients c ON c.id = s.client_id
JOIN tracking_types t ON t.id = s.tracking_type_id
WHERE c.name = ?
  AND s.started_at >= ?
  AND s.started_at <= ?
ORDER BY s.started_at ASC
`, clientName, from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []ReportRow
	var total int64
	now := time.Now().UTC()
	for rows.Next() {
		var r ReportRow
		var startedRaw string
		var stoppedRaw sql.NullString
		if err := rows.Scan(&r.SessionID, &r.ClientName, &r.TrackingTypeName, &r.Note, &startedRaw, &stoppedRaw, &r.DurationSec); err != nil {
			return nil, 0, err
		}
		start, err := time.Parse(time.RFC3339, startedRaw)
		if err != nil {
			return nil, 0, err
		}
		r.StartedAt = start
		if stoppedRaw.Valid {
			t, err := time.Parse(time.RFC3339, stoppedRaw.String)
			if err != nil {
				return nil, 0, err
			}
			r.StoppedAt = &t
			r.ComputedDurationS = int64(t.Sub(start).Seconds())
		} else {
			r.ComputedDurationS = int64(now.Sub(start).Seconds())
		}
		if r.ComputedDurationS < 0 {
			r.ComputedDurationS = 0
		}
		total += r.ComputedDurationS
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return result, total, nil
}
