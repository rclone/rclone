---
type: tasks
story: S1-03
---

# S1-03 Tasks — Registration, docs, and test stubs (both backends)

Wires the two Wave-1 backends into the rclone binary, docs, and test harness. Every
task here assumes S1-01 and S1-02 already produce `go build`-passing packages.

## T01 — Blank imports in `backend/all/all.go`

**Scope.** Add two blank-import lines to `backend/all/all.go` in alphabetical position
relative to existing entries (between `gofile`/`googlecloudstorage` region as
appropriate — `gcalfs` and `gmailfs` sort before `gofile`):
```go
_ "github.com/rclone/rclone/backend/gcalfs"
_ "github.com/rclone/rclone/backend/gmailfs"
```
Do not reorder or remove any existing entry.

**Rules.** build gate; sprint must-not (no import before package exists; no reorder).
**Done when.** `grep -c 'gmailfs\|gcalfs' backend/all/all.go` returns 2; `go build ./...`
exits 0.

## T02 — `docs/content/gmailfs.md`

**Scope.** Write the Gmail doc page following `docs/content/googlephotos.md` — Hugo
front matter (title, description, versionIntroduced, weight/url as conventions
require), an overview, a configuration walkthrough (OAuth app setup, supplying
`client_id` and `client_secret`, `start_year`, the interactive `rclone config` flow),
a filesystem-layout section showing the `Year/Month/Day/thread/.eml + attachments/`
tree, and a Limitations section stating: read-only (no upload/delete/modify), no hash
support, and that `client_id`/`client_secret` are REQUIRED with no rclone-provided
default credential.

**Rules.** sprint must (doc states required credentials; no write capability described).
**Done when.** `docs/content/gmailfs.md` exists with overview, configuration,
filesystem-layout, and limitations sections; no text implies writes work.

## T03 — `docs/content/gcalfs.md`

**Scope.** Same structure as T02 for Calendar: overview, configuration (OAuth,
`client_id`, `client_secret`, `start_year`), filesystem layout
(`Calendar/Year/Month/Day/.ics`), and a Limitations section (read-only, no hash,
required credentials, all-day-event representation note, calendar-name disambiguation
via `{calendarId[:8]}` suffix).

**Rules.** sprint must.
**Done when.** `docs/content/gcalfs.md` exists with the four sections; no write
capability described.

## T04 — Integration test stubs

**Scope.** Write `backend/gmailfs/gmailfs_test.go` and `backend/gcalfs/gcalfs_test.go`,
each with a `TestIntegration` that:
- calls `fstest.Initialise()`,
- defaults `*fstest.RemoteName` to `"TestGmailFs:"` / `"TestGcalFs:"` when empty,
- calls `fs.NewFs(ctx, *fstest.RemoteName)`,
- `t.Skipf`s when `errors.Is(err, fs.ErrorNotFoundInConfigFile)`,
- when configured: asserts root List returns ≥1 entry, a known day path lists without
  error, and each write op (`Put`/`Mkdir`/`Rmdir`/`Remove`/`SetModTime`) returns an
  error.

Note: the per-task unit tests authored during S1-01/S1-02 RED already live in these
same `_test.go` files; this task adds (or merges in) the integration `TestIntegration`
function specifically, without removing the unit tests.

**Rules.** T-1, T-2; sprint must (skip, never fail, without credentials).
**Done when.** `go test ./backend/gmailfs/ ./backend/gcalfs/ -run TestIntegration -v`
outputs SKIP (not FAIL) on a machine with no credentials.

## T05 — `fstest/test_all/config.yaml` entries

**Scope.** Add two entries (placed alphabetically; `gcalfs`/`gmailfs` sort before
`googlephotos`), no duplicates:
```yaml
 - backend:  "gcalfs"
   remote:   "TestGcalFs:"
   fastlist: false
 - backend:  "gmailfs"
   remote:   "TestGmailFs:"
   fastlist: false
```

**Rules.** sprint must-not (no duplicate keys); the `backend` value must match each
package's `fs.RegInfo.Prefix`.
**Done when.** `grep 'TestGmailFs\|TestGcalFs' fstest/test_all/config.yaml` returns both
entries; the file still parses as YAML.

## T06 — Backend data files, site navigation, and index entries

**Scope.** Add both backends to all rclone site/navigation registration points as
required by CONTRIBUTING.md:

- `docs/data/backends/gmailfs.yaml` — create backend data YAML (use
  `bin/manage_backends.py create` + `bin/manage_backends.py features` if available;
  otherwise model on an existing backend YAML). Set: name="Gmail", description="Read-only access to Gmail via IMAP-like virtual filesystem", home="https://mail.google.com/", prefix="gmailfs", type=storage, tier=3 (read-only/limited), and relevant feature flags (no hash, no chunking, no purge, no copy, no move, no DirMove, no CleanUp, no PublicLink, no About, ReadMimeType=true).
- `docs/data/backends/gcalfs.yaml` — same for Calendar: name="Google Calendar", description="Read-only access to Google Calendar events as .ics files", home="https://calendar.google.com/", prefix="gcalfs".
- `docs/layouts/chrome/navbar.html` — add `<a class="dropdown-item" href="/gmailfs/">Gmail</a>` and `<a class="dropdown-item" href="/gcalfs/">Google Calendar</a>` in alphabetical position relative to existing Google entries.
- `bin/make_manual.py` — add `"gmailfs"` and `"gcalfs"` to the `docs` list constant in alphabetical position (near the other Google entries: googlephotos, googlecloudstorage, drive).
- `README.md` — add `- Gmail [:page_facing_up:](https://rclone.org/gmailfs/)` and `- Google Calendar [:page_facing_up:](https://rclone.org/gcalfs/)` to the backends list in alphabetical position (after Google Drive, before Gofile/HDFS as appropriate).
- `docs/content/docs.md` — add `- [Gmail](/gmailfs/)` and `- [Google Calendar](/gcalfs/)` in alphabetical position.
- `docs/content/_index.md` — add `{{< provider name="Gmail" home="https://mail.google.com/" config="/gmailfs/" >}}` and `{{< provider name="Google Calendar" home="https://calendar.google.com/" config="/gcalfs/" >}}` in alphabetical position.

**Rules.** Alphabetical order; no duplicate entries; all 7 file touches in one task.
**Done when.** All 7 files contain the new entries and `grep` confirms both backends appear in each file.
