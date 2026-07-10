# AGENTS.md

This file provides guidance to AI coding agents (e.g. Claude Code, Codex, Cursor, Gemini CLI, and similar tools) when working with code in this repository.

Rclone welcomes AI-assisted contributions, but the expectation is that you, the human submitter, understand every line you propose and have compiled and tested it against real rclone code - not just generated it. See the "AI-assisted contributions" section of [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.

## Project Overview

Rclone is a command-line program to sync files and directories to and from cloud storage providers. It's written in Go and supports 70+ backends (cloud storage systems). Think "rsync for cloud storage".

## Build and Test Commands

```bash
# Build rclone (simple)
go build

# Build with version info (preferred)
make

# Run all unit tests (no cloud credentials needed)
make quicktest
# or equivalently:
RCLONE_CONFIG="/notfound" go test ./...

# Run tests for a specific package
cd backend/memory && go test -v
# or from root:
go test -v ./backend/memory/

# Run a single test
go test -v -run TestIntegration/FsCheckWrap ./backend/memory/

# Run tests with race detector
make racequicktest

# Lint (requires golangci-lint)
golangci-lint run ./...

# Run backend integration tests (requires configured TestRemote remote)
cd backend/drive && go test -v
# Run sync/operations integration tests against a remote
cd fs/sync && go test -v -remote TestDrive:
cd fs/operations && go test -v -remote TestDrive:

# Run integration tests via test framework
go run ./fstest/test_all -backends drive
```

## Architecture

### Entry Point and Plugin Registration

`rclone.go` is the main entry point. It imports `backend/all` and `cmd/all` which use Go's `init()` pattern to register all backends and commands. Each backend calls `fs.Register()` with a `fs.RegInfo` struct during init.

### Core Interfaces (`fs/`)

The `fs` package defines the core abstractions:
- **`fs.Fs`** (`fs/types.go`): The filesystem interface every backend must implement (List, NewObject, Put, Mkdir, Rmdir).
- **`fs.Object`** (`fs/types.go`): Interface for a file/object (Open, Update, Remove, SetModTime).
- **`fs.Features`** (`fs/features.go`): Optional capabilities a backend can declare (Purge, Copy, Move, DirMove, etc.). Backends set function pointers for operations they support; nil means not supported.
- **`fs.RegInfo`** (`fs/registry.go`): Registration metadata for a backend including its name, config options, and NewFs constructor.

### Backend Structure (`backend/`)

Each backend is a single Go package (e.g., `backend/s3/`, `backend/drive/`). Key conventions:
- Main implementation in a single file (e.g., `s3.go`) - **do not** split into `fs.go`/`object.go`.
- API types go in a separate `api/types.go` file.
- Test file (e.g., `s3_test.go`) uses `fstests.Run()` from `fstest/fstests` for standardized integration tests.
- Register in `backend/all/all.go` via blank import.
- HTTP-based backends should use `lib/rest` for HTTP calls and `fs/fshttp` for the HTTP client.
- Use `lib/dircache` for directory-ID-based remotes, `lib/oauthutil` for OAuth, `lib/pacer` for rate limiting.

### Command Structure (`cmd/`)

Each command is a package under `cmd/` registered in `cmd/all/all.go` via blank import. Commands use cobra via `cmd.Main()`.

### Key Subsystems

- **`fs/operations/`**: Core file operations (Copy, Move, Delete, etc.)
- **`fs/sync/`**: Directory sync logic
- **`fs/march/`**: Parallel directory tree walker used by sync
- **`fs/filter/`**: Include/exclude filtering
- **`fs/accounting/`**: Transfer statistics and bandwidth limiting
- **`fs/config/`**: Configuration file management
- **`vfs/`**: Virtual filesystem layer (used by mount, serve)
- **`librclone/`**: C-compatible library interface for embedding rclone
- **`fstest/`**: Integration test framework; `fstest/fstests/` has the generic backend test suite

## Commit Message Convention

Prefix with the directory of the change, then a colon: `drive: add team drive support - fixes #885`. For cross-cutting changes use a broader prefix like `fs` or `operations`.

Make the first line of your commit message a summary of the change that a user (not a developer) of rclone would like to read. So write `drive: fix server side copy of big files` instead of `drive: no longer set the MimeType in Move or Copy`. This is important because these lines go into the change log which is read by users.

## Code Commenting Style

Comments describe the code as it is now, for a future reader who has no knowledge of the change that introduced it.

Every exported type, function, field, and constant has a godoc comment that starts with its name and is phrased as a present-tense statement of what it is or does (`// Mkdir makes the directory (container, bucket)`)

Document the contract callers need - preconditions, what's returned, which sentinel errors are returned and when, and any "shouldn't return an error if it already exists" style caveats - rather than the implementation.

Keep comments terse: a single line for most things, with extra paragraphs (separated by blank `//` lines) reserved for genuine subtlety.

Inline comments inside function bodies should explain *why* - a non-obvious API quirk, a workaround, a gotcha, or an ordering constraint - and may cite an external reference (forum thread, vendor docs, RFC) when that's what makes the behaviour non-obvious; skip comments that merely restate what the code plainly does.

Use `FIXME` and `TODO` for known shortcomings.

Do **not** write comments that narrate the change itself or compare against the previous behaviour (no "now we also handle...", "changed to...", "previously this returned...", or references to bug/PR numbers in the code) - that context belongs in the commit message, not in source that will outlive the change.

For example, do **not** write comments like this:

```go
// Create a new HTTP client
client := fshttp.NewClient(ctx)
// Loop over the entries returned by the API
for _, entry := range entries {
	// Now we also skip directories to fix the pagination bug
	if entry.IsDir {
		continue
	}
}
```

Every comment there either restates the code or narrates the change. Written in house style it is just the code, with a comment only where one earns its place:

```go
client := fshttp.NewClient(ctx)
for _, entry := range entries {
	// Directories are returned again on the next page, so skip them here.
	if entry.IsDir {
		continue
	}
}
```

A real example of the same fix. This comment explains at length why the code is written the way it is and how it mirrors another feature - all of which belongs in the commit message:

```go
// Ephemeral "config_"-prefixed parameters (e.g. config_template_file) must
// not be written to the config file, but backends read them from the
// mapper, so expose them as a getter overlay instead of dropping them. This
// mirrors how `rclone authorize --template` makes the template readable
// without persisting it.
```

One line is enough - say what the code does, not the history behind it:

```go
// Add a getter to make sure ephemeral config is still visible
```

## Linting Configuration

Uses golangci-lint v2 with config in `.golangci.yml`. Enabled linters: errcheck, govet, ineffassign, staticcheck, unused, gocritic, misspell, revive, unconvert. The `goimports` formatter is also enabled.

## Documentation

- Backend option docs come from `Help:` fields in the Go source Options structs, not from markdown files.
- Command docs are in the command source code (e.g., `cmd/ls/ls.go`).
- Don't commit autogenerated doc changes from `make backenddocs` or `make commanddocs`.
- Website docs are in `docs/content/` as markdown, built with Hugo (`make serve` to preview).
