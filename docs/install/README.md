# Install Timmies TUI

This is the quickest way to get `timmies` running locally.

## Prerequisites

- Go 1.22+
- A shell terminal on Linux/macOS (Windows works with PowerShell too)

## 1) Clone the repo

```bash
git clone https://github.com/maplepreneur/Timmies-TUI/
cd Timmies-TUI
```

## 2) Build and install

The quickest way is to run the install script:

```bash
./install.sh
```

This builds a local `timmies` binary and installs it to your Go bin path.

### Manual steps

Build only (local binary):

```bash
go build -o timmies ./cmd/timmies
```

Install globally:

```bash
go install ./cmd/timmies
```

After this, make sure your Go bin path is in `PATH` (usually `~/go/bin`).

## 3) Verify install

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
