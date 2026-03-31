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
delete, reconstruction when enough shards survive, and explicit `heal`) are
implemented, but behavior and options may still change. See
`backend/rs/docs/OPEN_QUESTIONS.md` in the source tree.

## How it works

- **Parameters**: `data_shards` = **k**, `parity_shards` = **m**. Configure
  exactly **k+m** entries in `remotes`, in **shard index order** (shard `0` …
  shard `k+m-1`).
- **Encoding**: Content is split into stripes and Reed–Solomon encoded; each
  shard receives its **particle** (that shard’s payload bytes).
- **Footer**: Each particle ends with the same **98-byte EC footer layout**
  (`RCLONE/EC`). Fields such as logical **size**, **mtime**, **MD5/SHA256**,
  **k**, **m**, **algorithm**, and **stripe size** are **the same across shards**.
  **Per-shard fields differ**, notably **`CurrentShard`** (fragment index) and
  **`PayloadCRC32C`** (CRC of that shard’s payload), so footers are **not
  byte-identical** copies.
- **Upload (`use_spooling=true`, default)**: Encoded shards are written to local
  disk (see `staging_dir` / system temp), then **uploaded in parallel** to each
  backend, with concurrency limited by **`max_parallel_uploads`** (they are not
  strictly sequential).
- **Rollback**: If `rollback=true` (default) and the operation fails before the
  write quorum is satisfied, shards already uploaded for that attempt are
  removed where possible.

### Quorum

**Quorum** means the minimum participation needed for an operation to succeed or
for data to be reconstructable.

- **Read / reconstruct:** Standard Reed–Solomon needs **any k** healthy shards
  out of **k+m** to reconstruct the logical object.
- **Write (rclone `rs`):** A `Put` succeeds only if **at least k+1** shard uploads
  succeed, where **k = `data_shards`** (i.e. **`data_shards + 1`** successful
  uploads). That is **stricter** than the mathematical minimum of **k**, so the
  commit path keeps **more than the bare minimum** of fragments before
  accepting the write. Preflight also checks that enough shard remotes look
  usable before starting the upload.

## Listing and shard 0 (alpha)

In this **first alpha**, **directory listing** follows **shard 0**’s view of the
tree; other shards are used when **opening** objects and resolving **footer**
metadata. If **shard 0** is unavailable or diverges, behavior may be wrong or
incomplete. **Choosing another primary shard or merging listings** when shard 0
fails is **future work**.

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
data_shards> 2
Number of parity shards (m).
Enter a signed integer.
parity_shards> 2
[snip]
```

Here `remote1` … `remote4` are remotes you defined earlier; the path segment
`rs` is the same on each (you may use another directory name).

### Advanced options

- **`use_spooling`**: Spool encoded shards to disk before upload (default
  `true`). **`use_spooling=false` is not implemented yet** and currently
  returns an error; streaming encode-and-upload may be added later (see
  `OPEN_QUESTIONS.md`).
- **`staging_dir`**: Directory for spooled data (empty = system temp).
- **`rollback`**: Remove successfully uploaded shards if the operation fails
  before write quorum (default `true`).
- **`max_parallel_uploads`**: Maximum concurrent shard uploads during `Put`.

## Basic usage

```console
rclone lsd myrs:
rclone copy /path/to/files myrs:backup
```

## Backend commands

```console
rclone backend COMMAND myrs:
```

See [rclone backend](/commands/rclone_backend/) for options and arguments.

- **`status`** — Health / quorum-oriented view of shard remotes.
- **`heal`** — Heal logical objects by reconstructing and uploading missing
  shard particles. With no path it scans the namespace; with a path it repairs
  only that single logical object. `-o dry-run=true` reports only.

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

Spool shards to local disk before upload.

Properties:

- Config:      use_spooling
- Env Var:     RCLONE_RS_USE_SPOOLING
- Type:        bool
- Default:     true

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

#### --rs-max-parallel-uploads

Maximum concurrent shard uploads during Put.

Properties:

- Config:      max_parallel_uploads
- Env Var:     RCLONE_RS_MAX_PARALLEL_UPLOADS
- Type:        int
- Default:     4

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

<!-- autogenerated options stop -->

## Metadata (alpha)

For logical objects, `rs` surfaces metadata from the **EC footer** stored in every shard particle:
- logical **content length**
- logical **modification time** (`Mtime`, used for `ModTime` and footer updates)
- **MD5 / SHA256** hashes of the logical content where supported

This alpha does **not** attempt to preserve or synchronize arbitrary per-remote metadata from the underlying shard remotes.

### Range reads

Range reads are supported on logical objects: `Object.Open` honors `fs.RangeOption` (and `fs.SeekOption`) so partial reads work. Depending on the access pattern, rs may reconstruct more than strictly the requested window before slicing (full reconstruction + slice vs sequential split reads).

## Known limitations (short)

- No coordinated **`Move` / `Copy` / `DirMove`** on the `rs` remote.
- **`use_spooling=false`** not implemented yet.
- Large namespaces: full **`heal`** without a path may be heavy.
- **`SetModTime`** may require every shard object to be present.

See **`backend/rs/docs/OPEN_QUESTIONS.md`** for a fuller list.

