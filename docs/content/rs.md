---
title: "RS"
description: "Reed–Solomon virtual backend that stripes objects across multiple remotes with parity for degraded reads and repair via backend heal"
---

# RS (Reed–Solomon)

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
- **Footer**: Each particle ends with the same **104-byte RS footer** (version
  **1**, magic **`RCLONERS`**). Fields such as logical **size**, **mtime**
  (nanoseconds), **MD5/SHA256**, **k**, **m**, **`Algorithm`** (`SYMM` =
  stripe-wise systematic RS), **`StripeSize` (S)**, **`NumStripes`**, and
  **`WriteID`** (a random per-`Put` nonce shared by all shards of one write) are
  **the same across shards**. **Per-shard fields differ**, notably
  **`CurrentShard`** and **`PayloadCRC32C`** (CRC of that shard’s full
  payload), so footers are **not byte-identical** copies.
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
- **Read / list merge:** Directory listing and per-name file votes use **k**
  (`data_shards`) as the floor—the same minimum needed to reconstruct.
- **Write / metadata / namespace:** **`Put`**, metadata updates, **`mkdir`/`rmdir`**,
  and same-layout **`Copy`/`Move`/`DirMove`** succeed when enough shard remotes
  agree per **`write_quorum`** (range **k..k+m**, default **k+1**).
- **Degraded / healthy:** `backend degraded` treats objects with **`present_shards >= k`**
  as healthy for inspection.
- Operations run in two phases: a full parallel pass, then one bounded retry
  pass for remaining failing shards. The backend attempts **every** shard in the
  op set (no early exit when `write_quorum` is reached); **commit** is when
  `successes >= write_quorum` after all attempts finish. Partial failures are
  logged; minority laggards are converged by **`backend heal`** (async). See
  **`backend/rs/docs/QUORUM_TRANSACTIONS.md`** for the full write/namespace policy.

## File formats

RS stores one **particle object per shard** at the same logical path on each
configured shard remote. The particle format is implementation-oriented and
versioned through a fixed trailing RS footer.

### Particle format overview

For non-empty logical objects, each shard particle is:

- **payload**: variable length per shard (virtual padding; parity shards use
  `NumStripes × StripeSize`, data shards store trimmed fragments only)
- **footer**: fixed 104-byte RS footer (version 1)

For empty logical objects, payload is length `0` and only the 104-byte footer
is stored.

High-level structure:

```text
+------------------------------+--------------------+
| shard payload (variable)     | RS footer (104 B)  |
+------------------------------+--------------------+
```

Where:

- Payload length depends on shard index and virtual padding (see below); parity
  shards use `NumStripes × StripeSize`, data shards store trimmed fragments only.
- `N = NumStripes`, `S = StripeSize` (fragment size per shard per stripe)

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

The footer is the final 104 bytes of every particle. Fields are little-endian and
aligned on 4- or 8-byte boundaries for efficient access.

```text
Offset  Size  Field
0       8     Magic "RCLONERS"
8       4     Version (uint32) = 1
12      4     Algorithm ([4]byte) = "SYMM" (stripe-wise systematic RS)
16      8     ContentLength (int64)
24      16    MD5 (logical object)
40      32    SHA256 (logical object)
72      8     Mtime (int64, nanoseconds since Unix epoch)
80      4     StripeSize (uint32)
84      4     NumStripes (uint32)
88      4     PayloadCRC32C (uint32, over full payload)
92      1     DataShards (k)
93      1     ParityShards (m)
94      1     CurrentShard (this particle's shard index)
95      1     Reserved (must be 0 on write)
96      8     WriteID (uint64, random per-Put nonce shared by all shards)
```

### Field definitions and invariants

- `Magic` must be exactly `RCLONERS` (Reed–Solomon particle footer).
- `Version` must be `1`; other versions are currently rejected.
- `Algorithm` must be `SYMM` for the stripe-wise systematic encoding used by
  this backend (`Split`/`Encode` per stripe via klauspost/reedsolomon). The
  magic identifies the RS footer family; `Algorithm` identifies the variant.
- `ContentLength`, `MD5`, `SHA256`, `Mtime`, `Algorithm`, `DataShards`,
  `ParityShards`, `StripeSize`, `NumStripes`, and `WriteID` are expected to be
  consistent across shard particles for the same logical object.
- `WriteID` is a random 64-bit nonce generated per `Put` and stamped identically
  on every shard of that write. Reads and `heal` select the unique `WriteID`
  group present on at least **k** shards, so a torn or mixed overwrite never
  joins fragments from different writes (uniqueness relies on `k > m`). See
  **`backend/rs/docs/QUORUM_TRANSACTIONS.md`**.
- `CurrentShard` must match the shard index of the backing remote where that
  particle is stored.
- `PayloadCRC32C` is per-shard and computed over the full particle payload
  (everything before the footer).
- Empty logical objects use `StripeSize=0` and `NumStripes=0`.

### Padding and stripe rules (virtual padding)

- RS encoding reads logical content in chunks of at most `k × S` bytes.
- Each stripe is zero-padded to `k × S` for RS split/encode.
- `NumStripes` is `ceil(ContentLength / (k × S))` for non-empty objects and `0`
  for empty objects.
- **On disk (virtual padding):** data shards store only the non-padding fragment
  bytes per stripe; **parity shards** store the full **`S`** bytes per stripe.
- Data shard `i` stripe `t` stores `max(0, min(S, L_t − i·S))` bytes where
  `L_t` is the logical byte length of stripe `t`.
- Logical size from list metadata (when all **k** data shards are listed):
  `ContentLength = Σ(i=0..k−1) (listParticleSize_i − 104)`.
- During reconstruction/join, decoded output is trimmed back to logical stripe
  length, and total decoded length must equal `ContentLength`.

### Reconstruction rules

- Reconstruction requires at least `k` readable shard fragments per stripe.
- Stripe-wise reconstruction is used for footer v1 particles (`StripeSize>0` and
  `NumStripes>0`).
- If all `k` data shards are present and footer-compatible, read path can join
  data shards directly stripe-by-stripe.
- If data-shard-only path is unavailable, full RS reconstruction is used.
- If any stripe has fewer than `k` available fragments, reconstruction fails.

### Validation rules

- Footer parse fails if:
  - particle length is `< 104` bytes
  - magic is invalid
  - version is not supported
- Particle payload extraction fails if:
  - `CurrentShard` mismatches expected shard index
  - payload CRC32C does not match `PayloadCRC32C`
- Stripe reconstruction fails if:
  - `StripeSize <= 0` or `NumStripes <= 0` for non-empty decode
  - present shard payload length differs from the virtual-padding expectation for
    that shard index
  - reconstructed logical byte count differs from `ContentLength`

### Compatibility and versioning

- Current format version is fixed at footer version `1`.
- Current parser behavior is strict: unknown footer versions are rejected.
- Future compatibility changes should use a new footer version (and usually a
  new `Algorithm` tag) with explicit decode rules.

Other rclone backends use different metadata schemes: **crypt** prefixes
encrypted files with `RCLONE\x00\x00`; **chunker** uses filename patterns and
optional JSON sidecar objects — not a trailing binary footer.

### Worked examples

Example A: tiny non-empty file (`k=2`, `m=2`, `S=32`, `ContentLength=1`)

- `k × S = 64`
- `NumStripes = ceil(1/64) = 1`
- data shard 0 payload = `1` byte; data shard 1 payload = `0` bytes
- parity shard payloads = `32` bytes each
- particle sizes: data0 `105`, data1 `104`, parity `136` bytes

Example B: exact numbers used in tests (`k=2`, `m=2`, `S=32`, `ContentLength=100`)

- `k × S = 64`
- `NumStripes = ceil(100/64) = 2`
- data shard 0 payload = `64` bytes; data shard 1 payload = `36` bytes
- parity shard payloads = `64` bytes each
- first stripe contributes 64 logical bytes; second stripe contributes 36
  logical bytes after trimming padding.

Example C: empty logical object (`ContentLength=0`)

- `NumStripes = 0`
- payload length = `0`
- particle size = `104` bytes (footer only)

Example D: ASCII payload view (`"Hello Black Forest"`, `k=2`, `m=1`, `S=16`)

- `ContentLength = 18`
- `k × S = 32`
- `NumStripes = ceil(18/32) = 1`
- data shard 0 payload = `16` bytes; data shard 1 payload = `2` bytes
- parity shard payload = `16` bytes
- particle sizes: data0 `120`, data1 `106`, parity `120` bytes

Payload bytes by shard after RS encode (in-memory stripe columns; `\0` is NUL):

- shard 0 (data, stored): `"Hello Black Fore"`
- shard 1 (data, stored): `"st"` (only 2 bytes on disk)
- shard 2 (parity, stored): full 16-byte RS fragment (may include non-printable bytes)

Notes on Example D:

- The logical content is zero-padded to 32 bytes before RS encode.
- Virtual padding omits trailing zero columns on data shards only.

### Implementation references

- Format constants and footer encoding: `backend/rs/footer.go`
- Virtual-padding layout helpers: `backend/rs/payloadlayout.go`
- Stripe math and encode: `backend/rs/encode.go`
- Listing internals: `backend/rs/docs/LIST_METADATA.md`
- List metadata fast path: `backend/rs/list_metadata.go`
- Payload extraction, CRC checks, and reconstruction: `backend/rs/object.go`
- Heal reconstruction path: `backend/rs/commands.go`
- Format-oriented tests: `backend/rs/rs_test.go`, `backend/rs/payloadlayout_test.go`

## Listing (quorum merge + metadata fast path)

Directory listing is merged from shard remotes using quorum voting.
Shard `List` calls run **in parallel**; each name is included only if enough
shards report it as a **file** or as a **directory** under **read quorum `k`**
(and enough shards participate in the directory listing overall). Resulting
names are **sorted alphabetically**. Objects with **`fileVotes < k`** are omitted
and logged as broken. Type conflicts across shards (file vs directory for the
same name) are logged and treated as degraded state (the name is omitted unless
one side reaches quorum).

**List metadata (no footer when possible):** for quorum-listed files, rclone
derives **size** from all **k** data-shard list sizes when available
(`Σ(particleSize − footerSize)`). **ModTime** uses the **lowest-index** shard
list time at **1s** precision when supported; skew beyond 1s is logged. A single
footer read on the lowest listing shard fills in size or ModTime only when list
metadata is insufficient. **`Open`** / **`Hash`** load the footer lazily
(`ensureFooter`).

**`NewObject`** (direct lookup, not list) probes shards **in parallel** for particle
size and ModTime — the same fast path as list when all **k** data sizes and a shard
ModTime are available (**no footer read**). Otherwise it reads **one** footer on the
lowest valid shard as fallback. **`backend heal`** uses parallel discovery with
per-shard virtual-padding size checks and parallel stripe fragment reads (and parallel
legacy reads / healed `Put`s where applicable). Heal applies **reference ModTime**
from surviving shard remotes to rebuilt particles.

Implementer detail (flow, edge cases, code map): **`backend/rs/docs/LIST_METADATA.md`**.

## Topology constraints

For this backend, configuration requires **`data_shards > parity_shards`**
(`k > m`).

## Capabilities (`Features`)

Reported capabilities are the **full intersection** across **all** configured
shard remotes: rs advertises a feature only if **every** shard backend supports
it (`Features().Mask` per shard). There is no “≥ k shards” relaxation.

**`CanHaveEmptyDirectories`** follows the same rule: it is `true` only when
every shard can persist empty directories (for example all-local shards), and
`false` when any shard lacks that support (for example a bucket backend).

rs then clears flags whose semantics differ on the logical namespace:
**`BucketBased`**, **`ReadMetadata`/`WriteMetadata`**, and **`SetTier`/`GetTier`**
stay off. **`SlowHash`** is set **`true`** after masking (not taken from shards):
logical **`Hash()`** loads digests from the EC footer on one shard — an extra read
per object — so rclone should not treat hashing as free during sync/checksum.
**`DuplicateFiles`** is always `false`; **`IsLocal`** is `false`.

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

Minimum shard successes required for write operations (Put, mkdir/rmdir, Copy/Move/DirMove when used, Remove, SetModTime, etc.). List merge uses read quorum **k** separately. Must be between data_shards (k) and data_shards+parity_shards (k+m). Default is k+1.

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

Logical **`Size()`** and **`ModTime()`** prefer **shard remote** metadata when available:

- **Size:** derived from all **k** data-shard particle sizes (virtual-padding sum) when
  every data shard has a valid particle; otherwise one footer `ContentLength` is read
  lazily.
- **ModTime:** lowest-index shard remote ModTime at **1s** precision when backends support
  it; otherwise footer `Mtime` (nanoseconds) is read lazily. `Fs.Precision()` is **1s**;
  **`Put`** stores the source mtime at 1s resolution on shard remotes and in the footer.

**`Open`** / **`Hash`** load a footer for stripe layout and digests; loaded footer does
**not** override successful k-data-shard size or shard-remote ModTime.

**`SetModTime`** updates shard remotes via `Object.SetModTime` when supported (no payload
read); `ModTimeNotSupported` backends use a footer rewrite fallback. **`Copy`** / **`Move`**
return provisional destination metadata from the source object (no destination footer read).

**MD5 / SHA256** hashes of logical content come from the footer when `Hash` is called.

**`About`** (`rclone about`) reports **derived logical** quota, not the physical sum across shards:
**free** and **total** are **k × the most-limited (fullest) shard**, because each shard stores
≈ `1/k` of every logical object, so the fullest shard gates how much more you can store. It requires
**every** shard remote to support `about` (reported only when all shards do). Small objects carry a
fixed ~`(k+m) × footer` overhead, so for many tiny files the figure is an approximation.

Implementer detail: **`backend/rs/docs/LIST_METADATA.md`**.

### Range reads

Range reads are supported on logical objects: `Object.Open` honors `fs.RangeOption` (and `fs.SeekOption`) so partial reads work. Depending on the access pattern, rs may reconstruct more than strictly the requested window before slicing (full reconstruction + slice vs sequential split reads).

## Semantic guarantees (short)

- `rs` uses quorum semantics for writes, namespace operations, **listing**, and
  same-layout server-side **`Copy`/`Move`/`DirMove`**; success means quorum
  reached, not necessarily all shards converged immediately.
- For `Move`/`Copy`/`DirMove` on quorum, temporary skew can exist (for example source remnants on minority shards) until repaired.
- **Copy/Move overwrite** uses per-shard staging (`.rs-tmp-*` / `.rs-bak-*`) so an existing destination is not deleted before new particles exist; in-process rollback restores from backup on failure. A **process crash mid-swap** is converged by **`backend heal`** (no central transaction log).
- Use `rclone backend degraded` to inspect skew and `rclone backend heal` to converge shard state.

## Known limitations (short)

- **`Copy` / `Move` / `DirMove`** on `rs` only use fast server-side paths when the source is an **`rs` object** on an **`rs` remote with the same k/m and shard count** as the destination; otherwise rclone uses ordinary transfers (or returns “can’t copy/move”). Failed server-side copy/move runs compensating rollback when **`rollback`** is enabled (like `Put`). **`Remove`** cannot restore deleted shard bytes after a partial delete—use **`degraded`** / **`heal`** if needed.
- Large namespaces: full **`heal`** without a path may be heavy.
- Some server-side operations can expose eventual-consistency effects under degraded shards (documented above).

See **`backend/rs/docs/OPEN_QUESTIONS.md`** for a fuller list.

