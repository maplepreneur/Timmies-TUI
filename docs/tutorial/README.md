# Timmies TUI Usage Tutorial

This tutorial walks through a complete time-tracking flow.

## 1) Create a client and tracking type

```bash
timmies client add acme
timmies type add dev
timmies type add consulting --billable --rate 150
```

List what you created:

```bash
timmies client list
timmies type list
```

`timmies type list` now shows whether each type is billable and, when billable, its hourly rate.

## 2) Start tracking work

```bash
timmies start --client acme --type dev --note "Implement auth flow"
```

Check active status:

```bash
timmies status
```

## 3) Stop and resume

Stop the current session:

```bash
timmies stop
```

Resume the most recent stopped session:

```bash
timmies resume
```

Note: resume creates a new session segment.

## 4) Run reports

Attach resource costs to a session before reporting:

```bash
timmies session resource add --session 1 --name ai_tokens --cost 12.50
```

Show report totals for a client/date range:

```bash
timmies report --client @acme --from 2026-01-01 --to 2026-01-31

# or use relative periods
timmies report --client @acme --last-days 7
timmies report --client @acme --last-weeks 4
timmies report --client @acme --this-year
```

Export the same report to CSV:

```bash
timmies export csv --client @acme --from 2026-01-01 --to 2026-01-31 --out acme-jan.csv
```

## 5) Configure report branding (optional)

Set a display name and logo path for branded PDF exports:

```bash
timmies config set-name "Maple Entrepreneur"
timmies config set-logo /path/to/logo.png
timmies config show
```

`timmies config set-logo` validates that the file exists and is readable. If the file later becomes unavailable, PDF export returns an explicit branding logo error.

Export a branded PDF:

```bash
timmies export pdf --client @acme --from 2026-01-01 --to 2026-01-31 --out acme-jan.pdf
```

## 6) Use the TUI

Launch:

```bash
timmies tui
```

Key bindings:

- `a`: add client
- `k`: add tracking type
- `s`: start session (`@client type note...`)
- `x`: stop active session
- `r`: resume latest stopped session
- `c`: add resource cost to the active session (or selected paused session)
- `p`: report (`@client YYYY-MM-DD YYYY-MM-DD` or `@client last N days` or `@client last N weeks` or `@client this year`)
- `q`: quit

## 7) Update to latest main

If you cloned this repo from GitHub, you can reinstall `timmies` directly from its `main` branch:

```bash
timmies update
```

`timmies update` reads your current repository's `origin` URL and runs `go install github.com/<owner>/<repo>/cmd/timmies@main`.
