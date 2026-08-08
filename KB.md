# KB — rclone/rclone

Onboarded slot-4 (daily tick 2026-07-29). Pure Go, builds+tests green on macOS,
no daemon. Grok rubric 3/3/3/1/3 total=13.

## What it is
`rsync for cloud storage` — Go CLI to sync/manage files across 70+ storage
backends (S3, GCS, Drive, etc.). Large, mature, very active. Entry: `rclone.go`
imports `backend/all` + `cmd/all` (init() registration via `fs.Register()`).

## AI-policy screen: PASS (explicit)
CONTRIBUTING.md §"AI-assisted contributions" WELCOMES AI coding assistants
(Claude Code, Codex, Cursor, Gemini named). No ban, NO mandatory AI-disclosure.
Conditions we must meet: understand every line, actually build+test it, no
plausible-guess PRs, and TRIM comments (AGENTS.md style: no narration/change-
describing comments, godoc starts with the name). AGENTS.md is the conventions
file — point tooling at it.

## Build / test / lint (verified macOS arm64, go1.26.5)
- Build: `go build ./fs/...` ✅ (full `go build ./...` also fine; deps auto-download)
- Canonical gate: `make quicktest` (CONTRIBUTING requires `go build` + `make quicktest` pass)
- Scoped test verified: `go test ./fs/hash/... ./lib/encoder/... ./lib/rest/... ./fs/filter/...` → 10562 passed ✅
- Backend changes need integration tests vs a REAL remote — AVOID backend-specific
  changes in the auto-lane (can't verify locally). Stick to fs/lib/cmd/docs fixes.

## Conventions (from AGENTS.md — gate on these)
- Backend = single impl file (e.g. `s3.go`), do NOT split into fs.go/object.go.
- Godoc on every exported symbol, present-tense, starts with the name.
- Inline comments explain WHY only; NO "now we also…/changed to…/previously…"
  narration, NO bug/PR numbers in source (belongs in commit msg). Trim verbose
  AI comments — reviewers explicitly watch for this.
- Docs are Markdown → Hugo; command docs auto-generated from .go files.

## Work lane
- Prefer: fs/lib core bug fixes, doc fixes, small self-contained non-backend bugs.
- AVOID: backend behavior changes (need real-remote integration tests), anything
  touching cross-backend compatibility (AGENTS.md stresses compat is key).
- Find grabbable: issues labeled `good first issue` / `help wanted`, unassigned,
  no competing PR — re-verify live each cycle.

## Reward angle
Popular infra tool; credibility signal. reward=1 (minor).
