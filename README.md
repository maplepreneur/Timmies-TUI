# chrono

`chrono` is a Go CLI/TUI time tracker using Charm libraries.

## Features

- Manage tracking clients and tracking types.
- Start, stop, resume, and inspect one active timer at a time.
- Store data in SQLite.
- View reports by client/date range in both CLI and TUI.
- Export report rows to CSV.

## Quickstart

```bash
go run ./cmd/chrono --help
```

### Create metadata

```bash
go run ./cmd/chrono client add acme
go run ./cmd/chrono type add dev
```

### Track time

```bash
go run ./cmd/chrono start --client acme --type dev --note "feature work"
go run ./cmd/chrono status
go run ./cmd/chrono stop
go run ./cmd/chrono resume
```

### Reporting

```bash
go run ./cmd/chrono report --client @acme --from 2026-01-01 --to 2026-01-31
go run ./cmd/chrono export csv --client @acme --from 2026-01-01 --to 2026-01-31 --out acme-jan.csv
```

### TUI

```bash
go run ./cmd/chrono tui
```

In the TUI:
- `a`: add client
- `k`: add tracking type
- `s`: start session (`@client type note...`)
- `x`: stop active session
- `r`: resume latest stopped session as a new segment
- `p`: report (`@client YYYY-MM-DD YYYY-MM-DD`)
- `q`: quit
