# Copilot Instructions for Timmies TUI

## Build, test, and formatting commands

- Build CLI binary:
  - `go build -o timmies ./cmd/timmies`
- Run all tests:
  - `go test ./...`
- Run a single package:
  - `go test ./internal/report`
- Run a single test:
  - `go test ./internal/report -run TestResolveDateRangeThisYear -v`
  - `go test ./internal/store/sqlite -run TestReportByClientIncludesBillingAndTotals -v`
- Format Go code:
  - `gofmt -w ./cmd ./internal`

## High-level architecture

- `cmd/timmies/main.go` is the command composition layer (Cobra). It wires all CLI subcommands, configures the DB path flag, and uses a shared dependency wrapper (`withDeps`) to open the SQLite store and construct `TimerService`.
- `internal/store/sqlite/store.go` is the system of record for persistence and reporting logic:
  - schema creation
  - additive migration checks (for older DBs)
  - CRUD-like operations for clients/types/sessions/resources/settings
  - report and dashboard aggregations (duration + monetary totals)
- `internal/service/timer.go` is intentionally thin and delegates to the store while applying current UTC timestamps for start/stop/resume flows.
- `internal/report/report.go` centralizes date range logic used by both CLI and TUI:
  - explicit `--from/--to`
  - relative periods (`last N days`, `last N weeks`, `this year`)
  - duration formatting helpers
- `internal/tui/model.go` is a Bubble Tea app with Bubbles/Lipgloss/Glamour:
  - dashboard for current-month totals
  - paused-session resume flow
  - report rendering and key-driven input flows
- `internal/export/` contains report exporters:
  - `csv.go` for row exports
  - `pdf.go` for client-facing branded reports
- `internal/update/selfupdate.go` parses GitHub `origin` remotes and builds the self-update install target (`.../cmd/timmies@main`) used by `timmies update`.

## Key repository conventions

- Persist timestamps in UTC/RFC3339; parse back to `time.Time` at boundaries.
- Keep migrations additive and backward compatible. New schema fields should be introduced in `Open()` startup flow (with checks), not destructive rewrites.
- Only one active session is allowed at a time (DB-level unique index + command/store validation).
- Billing model is split:
  - tracking type hourly billing (`is_billable`, `hourly_rate`)
  - per-session resource costs (`session_resources`)
  - report summaries must expose time billable total, resource total, and combined monetary total.
- CLI and TUI should share report-period behavior via `report.ResolveDateRange` rather than duplicating date logic.
- For user input that accepts client handles, existing command flows normalize with `strings.TrimPrefix(value, "@")`.
- Branding settings (display name/logo path) are persisted in `settings` and consumed by PDF export; logo path validation is explicit and should return actionable errors.
