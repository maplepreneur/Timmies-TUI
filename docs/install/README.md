# Install Timmies TUI

This is the quickest way to get `timmies` running locally.

## Prerequisites

- Go 1.22+
- A shell terminal on Linux/macOS (Windows works with PowerShell too)

## 1) Clone the repo

```bash
git clone git@github.com:maplepreneur/chrono.git
cd chrono
```

## 2) Build the binary

```bash
go build -o timmies ./cmd/timmies
```

This creates a local executable named `timmies`.

## 3) (Optional) Install globally

```bash
go install ./cmd/timmies
```

After this, make sure your Go bin path is in `PATH` (usually `~/go/bin`).

## 4) Verify install

```bash
./timmies --help
# or, if installed globally:
timmies --help
```

## Database location

By default, Timmies TUI uses `tim.db` in your current directory.

You can point to a custom DB file with:

```bash
timmies --db /path/to/tim.db status
```
