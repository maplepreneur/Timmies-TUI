# Install Chrono

This is the quickest way to get `chrono` running locally.

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
go build -o chrono ./cmd/chrono
```

This creates a local executable named `chrono`.

## 3) (Optional) Install globally

```bash
go install ./cmd/chrono
```

After this, make sure your Go bin path is in `PATH` (usually `~/go/bin`).

## 4) Verify install

```bash
./chrono --help
# or, if installed globally:
chrono --help
```

## Database location

By default, Chrono uses `chrono.db` in your current directory.

You can point to a custom DB file with:

```bash
chrono --db /path/to/chrono.db status
```
