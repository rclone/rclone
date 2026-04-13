---
title: "RS"
description: "Reed–Solomon virtual backend that stripes objects across multiple remotes with parity for degraded reads and repair via backend heal"
---

# {{< icon "fa fa-th" >}} RS (Reed–Solomon)

The `rs` backend is a **virtual** remote: it does not store data by itself. It
presents a single filesystem while splitting each logical object across several
backends using **erasure coding**.

## Erasure coding and Reed–Solomon

**Erasure coding** splits a logical object into multiple **fragments** and adds
**redundancy** so the object can still be recovered when some backends fail or
are offline. **Reed–Solomon** is the usual “k data + m parity” family of
codes: you need only **k** of the **k+m** fragments to reconstruct the original
data, which is what makes multi-remote storage **robust** compared to a single
copy.

rclone’s `rs` backend uses the widely deployed Go library
**[github.com/klauspost/reedsolomon](https://github.com/klauspost/reedsolomon)**
([pkg.go.dev](https://pkg.go.dev/github.com/klauspost/reedsolomon)) for encoding
and reconstruction.

Like [union](/union/) you combine several remotes.

## Status: Alpha / experimental

The backend is intended for testing and feedback. Core paths (list, get, put,
delete, reconstruction when enough shards survive, explicit `heal`, and
same-layout server-side `Copy` / `Move` / `DirMove` where shard remotes support them)
are implemented, but behavior and options may still change. See
`backend/rs/docs/OPEN_QUESTIONS.md` in the source tree.

## How it works

- **Parameters**: `data_shards` = **k**, `parity_shards` = **m**. Configure
  exactly **k+m** entries in `remotes`, in **shard index order** (shard `0` …
  shard `k+m-1`).
- **Encoding**: Content is read in logical chunks of up to **k × S** bytes (see
  **`stripe_fragment_size`** = **S**). Each chunk is zero-padded to **k × S**,
  Reed–Solomon split/encoded, and **S** bytes per stripe are appended to each
  shard’s payload. **`NumStripes`** in the footer is the stripe count; empty
  objects use **0** stripes.
- **Footer**: Each particle ends with the same **102-byte EC footer** (version
  **3**, magic **`RCLONE/EC`**). Fields such as logical **size**, **mtime**,
  **MD5/SHA256**, **k**, **m**, **algorithm**, **`StripeSize` (S)**,
  **`NumStripes`**, and **`NumBlocks`** (reserved, 0 today) are **the same across
  shards**. **Per-shard fields differ**, notably **`CurrentShard`** and
  **`PayloadCRC32C`** (CRC of that shard’s full payload), so footers are **not
  byte-identical** copies.
- **Upload**: Encoded shards are written to local disk when `use_spooling=true`
  (see `staging_dir` / system temp), or streamed directly when `use_spooling=false`
  and the source size is known. In both cases the backend runs **one concurrent
  `Put` per shard** (see `backend/rs/operations.go`).
- **Download / `Open`**: Stripe-wise reads use **parallel range reads** across
  shards for each stripe. Reconstruct mode probes **all shards in parallel once**
  to see which particles are present before streaming.
- **Rollback**: If `rollback=true` (default) and the operation fails before the
  write quorum is satisfied, shards already uploaded for that attempt are
  removed where possible.

### Quorum

**Quorum** means the minimum participation needed for an operation to succeed or
for data to be reconstructable.

- **Read / reconstruct:** Standard Reed–Solomon needs **any k** healthy shards
  out of **k+m** to reconstruct the logical object.
- **Write / metadata / namespace / listing (rclone `rs`):** Quorum-based
  operations—including **`Put`**, **`List`** (per-name merge), metadata updates,
  **`mkdir`/`rmdir`**, and same-layout **`Copy`/`Move`/`DirMove`** when
  used—succeed when enough shard remotes agree per the configured threshold.
  This is configurable via **`write_quorum`** (range **k..k+m**, default **k+1**).
  Operations run in two phases: a full parallel pass, then one bounded retry
  pass for remaining failing shards. Partial failures are logged.

## Listing (quorum merge)

Directory listing is merged from shard remotes using quorum voting.
Shard `List` calls run **in parallel**; each name is included only if enough
shards report it as a **file** or as a **directory** under **`write_quorum`**
(and enough shards participate in the directory listing overall). Resulting
names are **sorted alphabetically**. Entries that do not meet quorum are omitted
from normal listings and can be inspected with backend diagnostics. Type
conflicts across shards (file vs directory for the same name) are logged and
treated as degraded state (the name is omitted unless one side reaches quorum).

**`NewObject`** probes shards **in parallel** and picks the **lowest shard index** with a valid footer (same as the previous sequential winner). **`backend heal`** uses parallel discovery and parallel stripe fragment reads (and parallel legacy reads / healed `Put`s where applicable).

## Topology constraints

For this backend, configuration requires **`data_shards > parity_shards`**
(`k > m`).

## Configuration

Configure **k+m** backing remotes first, then add `rs` with `rclone config`.

Example (illustrative names):

```console
rclone config
```

```text
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> myrs
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Reed-Solomon virtual backend
   \ "rs"
[snip]
Storage> rs
Comma-separated shard remotes in shard index order.
Enter a string value.
remotes> remote1:rs,remote2:rs,remote3:rs,remote4:rs
Number of data shards (k).
Enter a signed integer.
data_shards> 3
Number of parity shards (m).
Enter a signed integer.
parity_shards> 1
[snip]
```

Here `remote1` … `remote4` are remotes you defined earlier; the path segment
`rs` is the same on each (you may use another directory name).

### Advanced options

- **`use_spooling`**: Spool encoded shards to disk before upload (default
  `false` streams encoded particles when the source size is known; unknown-size
  uploads use spooling for that transfer only).
- **`staging_dir`**: Directory for spooled data (empty = system temp).
- **`rollback`**: Remove successfully uploaded shards if the operation fails
  before write quorum (default `true`).

## Basic usage

```console
rclone lsd myrs:
rclone copy /path/to/files myrs:backup
```

### `rclone backend` (status, heal, degraded)

```console
rclone backend COMMAND myrs:
```

See [rclone backend](/commands/rclone_backend/) for options and arguments.

- **`status`** — Health / quorum-oriented view of shard remotes.
- **`heal`** — Heal logical objects by reconstructing and uploading missing
  shard particles. With no path it scans the namespace; with a path it repairs
  only that single logical object. `-o dry-run=true` reports only.
- **`degraded`** — Inspect degraded state (`summary`, `ls`, `lsd`) without
  mutating data.

The section below is generated from `rclone help backend rs` and `rclone backend help rs` (run `make backenddocs` after changing `backend/rs/rs.go`).

<!-- autogenerated options start - DO NOT EDIT - instead edit fs.RegInfo in backend/rs/rs.go and run make backenddocs to verify --> <!-- markdownlint-disable-line line-length -->
### Standard options

Here are the Standard options specific to rs (Reed-Solomon virtual backend).

#### --rs-remotes

Comma-separated shard remotes in shard index order.

Properties:

- Config:      remotes
- Env Var:     RCLONE_RS_REMOTES
- Type:        string
- Required:    true

#### --rs-data-shards

Number of data shards (k).

Properties:

- Config:      data_shards
- Env Var:     RCLONE_RS_DATA_SHARDS
- Type:        int
- Default:     4

#### --rs-parity-shards

Number of parity shards (m).

Properties:

- Config:      parity_shards
- Env Var:     RCLONE_RS_PARITY_SHARDS
- Type:        int
- Default:     2

### Advanced options

Here are the Advanced options specific to rs (Reed-Solomon virtual backend).

#### --rs-use-spooling

Spool shards to local disk before upload. Default false streams encoded particles directly when the source size is known; unknown-size uploads (e.g. rcat) automatically use spooling for that transfer only.

Properties:

- Config:      use_spooling
- Env Var:     RCLONE_RS_USE_SPOOLING
- Type:        bool
- Default:     false

#### --rs-staging-dir

Directory for spooled shards. Empty means system temp.

Properties:

- Config:      staging_dir
- Env Var:     RCLONE_RS_STAGING_DIR
- Type:        string
- Required:    false

#### --rs-rollback

Delete uploaded shards when write quorum is not met.

Properties:

- Config:      rollback
- Env Var:     RCLONE_RS_ROLLBACK
- Type:        bool
- Default:     true

#### --rs-write-quorum

Minimum shard successes required for quorum operations (Put, List merge, mkdir/rmdir, Copy/Move/DirMove when used, etc.). Must be between data_shards (k) and data_shards+parity_shards (k+m). Default is k+1.

Properties:

- Config:      write_quorum
- Env Var:     RCLONE_RS_WRITE_QUORUM
- Type:        int
- Default:     0

#### --rs-stripe-fragment-size

RS stripe fragment size S in bytes (bytes appended per shard per stripe). If <= 0, defaults to 256KiB.

Properties:

- Config:      stripe_fragment_size
- Env Var:     RCLONE_RS_STRIPE_FRAGMENT_SIZE
- Type:        int
- Default:     262144

#### --rs-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_RS_DESCRIPTION
- Type:        string
- Required:    false

## Backend commands

Here are the commands specific to the rs backend.

Run them with:

```console
rclone backend COMMAND remote:
```

The help below will explain what arguments each command takes.

See the [backend](/commands/rclone_backend/) command for more
info on how to pass options and arguments.

These can be run on a running backend using the rc command
[backend/command](/rc/#backend-command).

### status

Show RS backend health and quorum status

```console
rclone backend status remote: [options] [<arguments>+]
```

### heal

Scan objects and restore missing shards where at least k shards are readable

```console
rclone backend heal remote: [options] [<arguments>+]
```

Scans logical objects (union of paths seen on shard remotes), and for each object
that is missing one or more shards but still has enough shards to reconstruct (>= data_shards),
reconstructs missing shard payloads and uploads them.

Without a path argument, all known objects are considered. With a path, only that logical
object is repaired (single-object repair).

This is an explicit, admin-driven repair. It does not run automatically on read.

Usage:

    rclone backend heal rs:
    rclone backend heal rs: path/to/file.bin

Options:

    -o dry-run=true    Report what would be healed without uploading shards

Examples:

    rclone backend heal rs:
    rclone backend heal rs: important.dat -o dry-run=true

Output includes counts (scanned / healed or would heal / skipped / failed) and per-object lines.
Objects with fewer than k good shards cannot be reconstructed and are reported as failed.


Options:

- "dry-run": If "true", only analyze and print "WOULD_HEAL" lines; no shard uploads.

### degraded

Inspect degraded object/directory state across shard remotes

```console
rclone backend degraded remote: [options] [<arguments>+]
```

Reports objects and directories that are not aligned across shard remotes.

Subcommands:
    summary   Show aggregate counts of degraded objects
    ls        List degraded logical objects
    lsd       List degraded directories (placeholder in current version)

Examples:

    rclone backend degraded rs:
    rclone backend degraded rs: summary
    rclone backend degraded rs: ls


<!-- autogenerated options stop -->

## Metadata (alpha)

For logical objects, `rs` surfaces metadata from the **EC footer** stored in every shard particle:
- logical **content length**
- logical **modification time** (`Mtime`, used for `ModTime` and footer updates)
- **MD5 / SHA256** hashes of the logical content where supported

This alpha does **not** attempt to preserve or synchronize arbitrary per-remote metadata from the underlying shard remotes.

### Range reads

Range reads are supported on logical objects: `Object.Open` honors `fs.RangeOption` (and `fs.SeekOption`) so partial reads work. Depending on the access pattern, rs may reconstruct more than strictly the requested window before slicing (full reconstruction + slice vs sequential split reads).

## Semantic guarantees (short)

- `rs` uses quorum semantics for writes, namespace operations, **listing**, and
  same-layout server-side **`Copy`/`Move`/`DirMove`**; success means quorum
  reached, not necessarily all shards converged immediately.
- For `Move`/`Copy`/`DirMove` on quorum, temporary skew can exist (for example source remnants on minority shards) until repaired.
- Use `rclone backend degraded` to inspect skew and `rclone backend heal` to converge shard state.

## Known limitations (short)

- **`Copy` / `Move` / `DirMove`** on `rs` only use fast server-side paths when the source is an **`rs` object** on an **`rs` remote with the same k/m and shard count** as the destination; otherwise rclone uses ordinary transfers (or returns “can’t copy/move”). Partial failures do not currently run **`Put`-style rollback** on shards that already succeeded—use **`degraded`** / **`heal`** if needed.
- Large namespaces: full **`heal`** without a path may be heavy.
- Some server-side operations can expose eventual-consistency effects under degraded shards (documented above).

See **`backend/rs/docs/OPEN_QUESTIONS.md`** for a fuller list.

