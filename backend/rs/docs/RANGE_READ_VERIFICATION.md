# RS range-read integrity via parity verification (proposal)

**Audience:** implementers and reviewers.
**Status:** **PROPOSED — not implemented.** Design note for a future change; nothing here is
wired into the backend yet.
**Code (today):** [`object.go`](../object.go) (`fill`, `readStripeFragmentsParallel`, data-shard
fast path), [`payloadlayout.go`](../payloadlayout.go) (virtual-padding stripe math),
[`footer.go`](../footer.go) (`PayloadCRC32C`, hashes).
**Related:** [`OPEN_QUESTIONS.md`](OPEN_QUESTIONS.md) Q18, [`LIST_METADATA.md`](LIST_METADATA.md)
(size fast-path), [`QUORUM_TRANSACTIONS.md`](QUORUM_TRANSACTIONS.md).
**Last updated:** 2026-06-13

---

## Problem

The data-shard-only read path (used for `Open` with a `Range`/`SeekOption`, and for VFS) joins the
`k` data fragments for the touched stripes and serves them **without any checksum check**. The only
at-rest checksum is the whole-payload `PayloadCRC32C` in each shard footer, which cannot be
evaluated from a partial read. So bitrot in a fragment served via a range read is **undetected**,
even though whole-object reads are protected.

We want range/partial reads to have the same bitrot guarantee as whole-object reads, **without**
regressing the List size fast-path.

## Hard constraint: preserve the List size fast-path

`Fs.List`/`NewObject` derive logical size from shard particle sizes **without reading a footer**:

```text
contentLength = Σ (particle_sizeᵢ − FooterSize)   over the k data shards
```

(`ContentLengthFromDataShardPayloads` in [`payloadlayout.go`](../payloadlayout.go)). This identity
is exact and **independent of `StripeSize`/`NumStripes`** because data shards store only logical
bytes plus a **fixed** footer (no padding).

> Any scheme that adds a **variable**, stripe-count-dependent region to the **data-shard** particles
> (e.g. a `NumStripes × uint32` per-stripe CRC table prepended to the footer) breaks this identity:
> the per-particle overhead would depend on `NumStripes = ceil(contentLength /(k·S))`, which is not
> recoverable from sizes alone (and `S` is per-object, recorded only in the footer). List would then
> have to read a footer per object — losing the fast-path. **This constraint rules out per-stripe CRC
> tables stored inside data shards.**

## Proposal: verify range reads against parity (store nothing extra)

Reed–Solomon parity is already a checksum over the data. Instead of storing new checksums, verify a
range read by re-checking that data and parity are mutually consistent for the touched stripes.

For each stripe `t` overlapped by the requested range:

1. Read the `k` **data** fragments for stripe `t` (offsets/lengths via
   `DataShardStripeOffset` / `DataShardFragLen`).
2. Read the `m` **parity** fragments for stripe `t` (offset `t·S`, length `S`).
3. Rebuild the `k×S` stripe matrix with **virtual zero padding** (data fragments zero-extended to
   `S`, exactly as the encoder did) and verify parity (klauspost `Encoder.Verify(shards)`).
4. **Verify ok →** serve the data fragments (same guarantee as a whole-object read).
5. **Verify fails →** corruption is present in that stripe; route into the existing reconstruct path
   to recover, or return a clear integrity error if unrecoverable.

Nothing is added to any particle: data-shard sizes stay `payload + FooterSize`, the footer is
unchanged, and the List size fast-path is **fully preserved**. This also keeps plan (a)'s decision to
redefine footer **v1 in place** with no variable region.

### Detection vs. correction

- **Detection** is unconditional and works even with `m = 1`: any single corrupt fragment (data or
  parity) in the stripe fails `Verify`.
- **Correction / locating the bad shard** depends on `m`:
  - `m ≥ 2`: RS can correct unknown-location errors (up to `⌊m/2⌋`); reconstruct the stripe directly.
  - `m = 1`: parity detects but cannot locate. Fall back to reading the **full particles** and using
    each shard's whole-particle `PayloadCRC32C` (`footer.go`) to identify the bad shard, converting
    the unknown error into a known **erasure**, which RS then corrects. This slow path only runs on
    actual corruption (rare).

This composes with the existing whole-particle CRC and reconstruct/heal code
(`ReconstructDataFromShards`, `reconstructMissingShards`).

### Virtual-padding note

Data shards store only non-padding bytes per stripe (`DataShardFragLen ≤ S`); parity shards store
the full `S`-byte fragment. The data fragments **must** be zero-extended to `S` before verifying, to
match what the encoder produced. The existing layout helpers
([`payloadlayout.go`](../payloadlayout.go)) already encode this geometry, so verification reuses the
same math as encode/reconstruct.

## Integration points (when implemented)

- `fill()` / `readStripeFragmentsParallel` ([`object.go`](../object.go)): after gathering data
  fragments, when verification is enabled, also fetch the stripe's parity fragments and `Verify`; on
  success continue to `Join`, on failure route into the reconstruct entry.
- Reconstruct/heal already read parity, so the recovery branch is mostly wiring.
- Footer, encode, and `payloadlayout` size math are **unchanged**.

## Cost / trade-offs

- **Read amplification.** Verification operates on whole stripes, so a range touching stripe `t`
  reads `(k+m)·S` bytes for that stripe even for a 1-byte request.
  - Large/sequential reads: cheap amortized (mostly the `m` parity shards' bandwidth).
  - Tiny random reads with large `S`: expensive relative to bytes requested.
  - → Make it **opt-in** (e.g. a `--rs-verify-range`-style flag) and/or enable only for
    "untrusted" shard targets (local/SFTP) vs. high-durability providers (S3/B2/GCS).
- **CPU.** One RS encode/verify per touched stripe; negligible vs. I/O.
- **Partly negates the data-only fast path** (now reads parity too) — but only when verification is
  on; off ⇒ exactly today's behavior.
- **Accidental-only threat model.** Parity consistency catches bitrot/torn writes, not a *malicious*
  provider that rewrites data and parity consistently. Adversarial integrity remains crypt's job
  (layer `rs` with `crypt`). Same limitation a CRC table would have.

## Alternative considered: store CRCs in parity particles only

If persisted/offline verification is wanted (verify without recomputing, or at heal time), store the
per-stripe CRC table **only in the parity particles** (and/or a designated metadata particle), never
in the data shards. Data-shard sizes stay `payload + FooterSize`, so the List fast-path is still
preserved. Cost: a data-only range read must fetch the small table from a parity shard at `Open`
(one extra small range GET, cacheable).

| | Per-stripe CRC in **data** shards | Parity verification (this proposal) | CRC table in **parity** shards |
|---|---|---|---|
| On-disk format change | yes (variable table) | **none** | yes (parity only) |
| List size fast-path | **broken** | **preserved** | preserved |
| Read I/O for tiny range | low | high (`(k+m)·S`/stripe) | medium (table + data) |
| Offline/heal-time verify | yes | no (recompute needed) | yes |

**Recommendation:** parity verification as the primary design (zero format change, fast-path safe,
opt-in cost); CRC-in-parity as the alternative if offline verifiability becomes a requirement.
