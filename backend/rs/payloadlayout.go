package rs

import "fmt"

// Virtual-padding layout helpers for SYMM RS particles (footer v1).
//
// Data shards store only the non-padding fragment bytes per stripe; parity shards
// store the full S-byte RS fragment per stripe. Logical content length is the sum
// of data-shard payload lengths when all k data shards are present.

// StripeLogicalLen returns how many logical bytes are in stripe stripeIdx (0-based).
func StripeLogicalLen(k int, S, contentLength int64, stripeIdx int) int64 {
	kS := int64(k) * S
	var pos int64
	for s := 0; s < stripeIdx; s++ {
		chunk := min64(kS, contentLength-pos)
		if chunk <= 0 {
			return 0
		}
		pos += chunk
	}
	return min64(kS, contentLength-pos)
}

// StripeLogicalBase returns the logical byte offset where stripe stripeIdx starts.
func StripeLogicalBase(k int, S, contentLength int64, stripeIdx int) int64 {
	var base int64
	for s := 0; s < stripeIdx; s++ {
		base += StripeLogicalLen(k, S, contentLength, s)
	}
	return base
}

// DataShardFragLen returns bytes stored for data shard shard in a stripe with logical length stripeLogicalLen.
func DataShardFragLen(shard, k, S, stripeLogicalLen int) int {
	if shard < 0 || shard >= k || stripeLogicalLen <= 0 {
		return 0
	}
	start := shard * S
	if start >= stripeLogicalLen {
		return 0
	}
	return min(S, stripeLogicalLen-start)
}

// DataShardStripeOffset returns the payload byte offset where stripe stripeIdx begins on data shard shard.
func DataShardStripeOffset(shard, k, S, stripeIdx int, contentLength int64) int64 {
	var off int64
	for s := 0; s < stripeIdx; s++ {
		L := StripeLogicalLen(k, int64(S), contentLength, s)
		off += int64(DataShardFragLen(shard, k, S, int(L)))
	}
	return off
}

// DataShardPayloadLen returns total payload bytes for data shard shard.
func DataShardPayloadLen(contentLength int64, shard, k, S int) int64 {
	if contentLength == 0 || shard < 0 || shard >= k {
		return 0
	}
	n := NumStripesForContent(contentLength, k, S)
	var sum int64
	for t := 0; t < n; t++ {
		L := StripeLogicalLen(k, int64(S), contentLength, t)
		sum += int64(DataShardFragLen(shard, k, S, int(L)))
	}
	return sum
}

// ParityShardPayloadLen returns total payload bytes for a parity shard (NumStripes × S).
func ParityShardPayloadLen(contentLength int64, k, S int) int64 {
	if contentLength == 0 {
		return 0
	}
	n := NumStripesForContent(contentLength, k, S)
	return int64(n) * int64(S)
}

// ShardPayloadLen returns payload bytes for shard shardIndex (data or parity).
func ShardPayloadLen(contentLength int64, shardIndex, k, m, S int) int64 {
	if contentLength == 0 {
		return 0
	}
	if shardIndex < k {
		return DataShardPayloadLen(contentLength, shardIndex, k, S)
	}
	return ParityShardPayloadLen(contentLength, k, S)
}

// ValidateShardParticleFile reports whether fileSize matches virtual-padding layout for shardIndex.
func ValidateShardParticleFile(fileSize, contentLength int64, shardIndex, k, m, stripeFragmentSize int) error {
	want := ExpectedParticleSize(contentLength, shardIndex, k, m, stripeFragmentSize, true)
	if fileSize != want {
		return fmt.Errorf("rs: shard %d particle size %d, want %d", shardIndex, fileSize, want)
	}
	return nil
}

// ExpectedParticleSize returns on-disk particle size for shard shardIndex.
func ExpectedParticleSize(contentLength int64, shardIndex, k, m, stripeFragmentSize int, withFooter bool) int64 {
	S := normalizeStripeFragmentSize(stripeFragmentSize)
	payload := ShardPayloadLen(contentLength, shardIndex, k, m, S)
	if withFooter {
		return payload + FooterSize
	}
	return payload
}

// ContentLengthFromDataShardPayloads sums (size − FooterSize) for data shards 0..k−1.
// Returns ok=false when any data shard size is missing or negative.
func ContentLengthFromDataShardPayloads(dataShardSizes []int64, k int) (sum int64, ok bool) {
	if len(dataShardSizes) < k {
		return 0, false
	}
	for i := 0; i < k; i++ {
		if dataShardSizes[i] < 0 {
			return 0, false
		}
		sum += dataShardSizes[i] - FooterSize
	}
	return sum, true
}

func shardStripeFragmentFromPayload(payload []byte, shardIdx, k, S, stripeIdx int, contentLength, logLen int64) []byte {
	if shardIdx < k {
		off := DataShardStripeOffset(shardIdx, k, S, stripeIdx, contentLength)
		flen := DataShardFragLen(shardIdx, k, S, int(logLen))
		end := off + int64(flen)
		if int(end) > len(payload) {
			return nil
		}
		return payload[off:end]
	}
	off := int64(stripeIdx) * int64(S)
	end := off + int64(S)
	if int(end) > len(payload) {
		return nil
	}
	return payload[off:end]
}

func writeShardStripeFragment(dst []byte, shardIdx, k, S, stripeIdx int, contentLength, logLen int64, frag []byte) {
	if shardIdx < k {
		off := DataShardStripeOffset(shardIdx, k, S, stripeIdx, contentLength)
		flen := DataShardFragLen(shardIdx, k, S, int(logLen))
		copy(dst[off:int(off)+flen], frag[:flen])
		return
	}
	off := stripeIdx * S
	copy(dst[off:off+S], frag)
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
