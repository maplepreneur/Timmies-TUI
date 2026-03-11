# Chrono Usage Tutorial

This tutorial walks through a complete time-tracking flow.

## 1) Create a client and tracking type

```bash
chrono client add acme
chrono type add dev
```

List what you created:

```bash
chrono client list
chrono type list
```

## 2) Start tracking work

```bash
chrono start --client acme --type dev --note "Implement auth flow"
```

Check active status:

```bash
chrono status
```

## 3) Stop and resume

Stop the current session:

```bash
chrono stop
```

Resume the most recent stopped session:

```bash
chrono resume
```

Note: resume creates a new session segment.

## 4) Run reports

Show report totals for a client/date range:

```bash
chrono report --client @acme --from 2026-01-01 --to 2026-01-31
```

Export the same report to CSV:

```bash
chrono export csv --client @acme --from 2026-01-01 --to 2026-01-31 --out acme-jan.csv
```

## 5) Use the TUI

Launch:

```bash
chrono tui
```

Key bindings:

- `a`: add client
- `k`: add tracking type
- `s`: start session (`@client type note...`)
- `x`: stop active session
- `r`: resume latest stopped session
- `p`: report (`@client YYYY-MM-DD YYYY-MM-DD`)
- `q`: quit
