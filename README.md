# Timmies TUI

`timmies` is an Open-Source Go CLI/TUI time tracker using Charm libraries.

Created with ❤️ by **Voxel North Technologies Inc.**

## Start here

- Install guide: [`docs/install/README.md`](docs/install/README.md)
- Usage tutorial: [`docs/tutorial/README.md`](docs/tutorial/README.md)

## Quick install

```bash
git clone https://github.com/maplepreneur/Timmies-TUI/ && cd Timmies-TUI
./install.sh
```

See the full [install guide](docs/install/README.md) for details.

## Features

- Manage tracking clients and tracking types.
- Start, stop, resume, and inspect one active timer at a time.
- Store data in SQLite.
- View reports by client/date range in both CLI and TUI.
- Attach per-session resource costs and include them in totals.
- Export report rows to CSV.
- Configure report branding (display name + logo file path) for PDF exports.
- Self-update from the repository main branch with `timmies update`.

## Testing

Run tests from the repository root:

- Run all tests:
	- `go test ./...`
- Run tests for a single package:
	- `go test ./internal/report`
- Run a single test by name:
	- `go test ./internal/report -run TestResolveDateRangeThisYear -v`
	- `go test ./internal/store/sqlite -run TestReportByClientIncludesBillingAndTotals -v`

## License

This project is licensed under the [O'Saasy License](LICENSE.md).

© 2026 Voxel North Technologies Inc.
