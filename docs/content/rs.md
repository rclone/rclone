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

## File formats

RS stores one **particle object per shard** at the same logical path on each
configured shard remote. The particle format is implementation-oriented and
versioned through a fixed trailing EC footer.

### Particle format overview

For non-empty logical objects, each shard particle is:

- **payload**: `NumStripes × StripeSize` bytes
- **footer**: fixed 102-byte EC footer (version 3)

For empty logical objects, payload is length `0` and only the 102-byte footer
is stored.

High-level structure:

```text
+------------------------------+--------------------+
| shard payload (N * S bytes)  | EC footer (102 B)  |
+------------------------------+--------------------+
```

Where:

- `N = NumStripes`
- `S = StripeSize` (fragment size per shard per stripe)

### Terminology

- **Logical object**: The file seen on the `rs` remote.
- **Particle**: One shard object stored on one backing remote.
- **Stripe**: One RS encoding cycle over up to `k × S` logical bytes.
- **k / m**: Data shard count / parity shard count.
- **S (`StripeSize`)**: Bytes appended per shard for each stripe.
- **N (`NumStripes`)**: Stripe count for the logical object.

### Shard naming and object mapping

- A logical path maps to the **same relative object path** on every shard
  remote.
- Shard identity is not encoded in the object name; it is encoded in footer
  field `CurrentShard`.
- Shard remotes are ordered by configuration (`remotes` list). Index domain is
  `0..(k+m-1)`.

### Binary layout

The footer is the final 102 bytes of every particle.

```text
Offset  Size  Field
0       9     Magic "RCLONE/EC"
9       2     Version (little-endian uint16) = 3
11      8     ContentLength (little-endian uint64)
19      16    MD5 (logical object)
35      32    SHA256 (logical object)
67      8     Mtime (little-endian uint64, unix seconds)
75      4     Compression tag ([4]byte, currently zero)
79      4     NumBlocks (little-endian uint32, reserved; currently 0)
83      4     Algorithm tag ([4]byte, currently "RS\\x00\\x00")
87      1     DataShards (k)
88      1     ParityShards (m)
89      1     CurrentShard (this particle's shard index)
90      4     StripeSize (little-endian uint32)
94      4     PayloadCRC32C (little-endian uint32, over full payload)
98      4     NumStripes (little-endian uint32)
```

### Field definitions and invariants

- `Magic` must be exactly `RCLONE/EC`.
- `Version` must be `3`; other versions are currently rejected.
- `ContentLength`, `MD5`, `SHA256`, `Mtime`, `Algorithm`, `DataShards`,
  `ParityShards`, `StripeSize`, and `NumStripes` are expected to be consistent
  across shard particles for the same logical object.
- `CurrentShard` must match the shard index of the backing remote where that
  particle is stored.
- `PayloadCRC32C` is per-shard and computed over the full particle payload
  (everything before the footer).
- Empty logical objects use `StripeSize=0` and `NumStripes=0`.

### Padding and stripe rules

- RS encoding reads logical content in chunks of at most `k × S` bytes.
- Each stripe is zero-padded to `k × S` for RS split/encode.
- `NumStripes` is `ceil(ContentLength / (k × S))` for non-empty objects and `0`
  for empty objects.
- Each shard receives exactly `S` bytes per stripe, so particle payload length
  is always `N × S`.
- During reconstruction/join, decoded output is trimmed back to logical stripe
  length, and total decoded length must equal `ContentLength`.

### Reconstruction rules

- Reconstruction requires at least `k` readable shard fragments per stripe.
- Stripe-wise reconstruction is used for footer v3 particles (`StripeSize>0` and
  `NumStripes>0`).
- If all `k` data shards are present and footer-compatible, read path can join
  data shards directly stripe-by-stripe.
- If data-shard-only path is unavailable, full RS reconstruction is used.
- If any stripe has fewer than `k` available fragments, reconstruction fails.

### Validation rules

- Footer parse fails if:
  - particle length is `< 102` bytes
  - magic is invalid
  - version is not supported
- Particle payload extraction fails if:
  - `CurrentShard` mismatches expected shard index
  - payload CRC32C does not match `PayloadCRC32C`
- Stripe reconstruction fails if:
  - `StripeSize <= 0` or `NumStripes <= 0` for non-empty decode
  - present shard payload length differs from `NumStripes × StripeSize`
  - reconstructed logical byte count differs from `ContentLength`

### Compatibility and versioning

- Current format version is fixed at footer version `3`.
- Current parser behavior is strict: unknown footer versions are rejected.
- No extension block is defined yet in v3; future compatibility changes should
  use a new footer version and explicit decode rules.

### Future compression layout (design placeholder)

This subsection is a design placeholder and is **not implemented** in the
current `rs` backend.

If RS later adds block compression (Snappy or Zstandard), particle payloads can
follow a block-indexed structure compatible with efficient range reads.

Planned compression algorithms:

- `snappy`
- `zstd`

Proposed footer semantics:

- `Compression` (`[4]byte`): compression algorithm tag (`none`, `snappy`,
  `zstd`)
- `NumBlocks` (`uint32`): number of compressed blocks used in the payload

Proposed compressed particle shape:

```text
+-------------------------------+---------------------------+-----------------+
| shard payload from compressed | block inv. (NumBlocks*4)  | EC footer (vN)  |
| stream (stripe/shard encoded) | LE uint32 compressed len  | incl. tag/count |
+-------------------------------+---------------------------+-----------------+
```

Where:

- Logical input is split into fixed uncompressed blocks (for example 128 KiB).
- Each block is compressed independently with the selected algorithm.
- Inventory stores compressed byte length of each block (`uint32`
  little-endian), in block order, and is appended to each shard payload.
- `NumBlocks` equals the number of inventory entries.
- `PayloadCRC32C` covers the full shard payload before the footer (compressed
  shard bytes plus inventory bytes).

Uncompressed (`Compression=none`) behavior can remain the current layout
(payload + footer, `NumBlocks=0`), with no inventory trailer.

### Worked examples

Example A: tiny non-empty file (`k=2`, `m=2`, `S=32`, `ContentLength=1`)

- `k × S = 64`
- `NumStripes = ceil(1/64) = 1`
- per-shard payload = `1 × 32 = 32` bytes
- particle size = `32 + 102 = 134` bytes

Example B: exact numbers used in tests (`k=2`, `m=2`, `S=32`, `ContentLength=100`)

- `k × S = 64`
- `NumStripes = ceil(100/64) = 2`
- per-shard payload = `2 × 32 = 64` bytes
- particle size = `64 + 102 = 166` bytes
- first stripe contributes 64 logical bytes; second stripe contributes 36
  logical bytes after trimming padding.

Example C: empty logical object (`ContentLength=0`)

- `NumStripes = 0`
- payload length = `0`
- particle size = `102` bytes (footer only)

Example D: ASCII payload view (`"Hello Black Forest"`, `k=2`, `m=1`, `S=16`)

- `ContentLength = 18`
- `k × S = 32`
- `NumStripes = ceil(18/32) = 1`
- per-shard payload = `1 × 16 = 16` bytes
- particle size = `16 + 102 = 118` bytes

Payload bytes by shard (ASCII; `\0` means NUL byte):

- shard 0 (data): `"Hello Black Fore"`
- shard 1 (data): `"st\0\0\0\0\0\0\0\0\0\0\0\0\0\0"`
- shard 2 (parity): `";\x11llo Black Fore"`

Notes on Example D:

- The logical content is zero-padded to 32 bytes before RS encode.
- Because `m=1`, the parity payload is computed bytewise from the two data
  shards and may include non-printable bytes (for this input byte 1 is `0x11`).

### Implementation references

- Format constants and footer encoding: `backend/rs/footer.go`
- Stripe math and particle size formulas: `backend/rs/encode.go`
- Payload extraction, CRC checks, and reconstruction: `backend/rs/object.go`
- Heal reconstruction path: `backend/rs/commands.go`
- Format-oriented tests: `backend/rs/rs_test.go`

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

