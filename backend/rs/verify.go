package rs

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"strings"

	"github.com/klauspost/reedsolomon"
	"github.com/rclone/rclone/fs"
	"golang.org/x/sync/errgroup"
)

type verifyStatus int

const (
	verifyOK verifyStatus = iota
	verifyDegraded
	verifySkew
	verifyCorrupt
	verifyFail
)

func (s verifyStatus) String() string {
	switch s {
	case verifyOK:
		return "OK"
	case verifyDegraded:
		return "DEGRADED"
	case verifySkew:
		return "SKEW"
	case verifyCorrupt:
		return "CORRUPT"
	case verifyFail:
		return "FAIL"
	default:
		return "FAIL"
	}
}

type verifyObjectResult struct {
	remote string
	status verifyStatus
	line   string
	failed bool
}

type verifySummary struct {
	scanned  int
	ok       int
	degraded int
	skew     int
	corrupt  int
	fail     int
	failed   int
}

func (f *Fs) verifyCommand(ctx context.Context, arg []string, opt map[string]string) (any, error) {
	optMap := opt
	if optMap == nil {
		optMap = map[string]string{}
	}
	hashes := optMap["hashes"] == "true"
	strict := optMap["strict"] == "true"
	for k := range optMap {
		switch k {
		case "hashes", "strict":
		default:
			return nil, fmt.Errorf("rs: unknown verify option %q", k)
		}
	}

	if len(arg) > 0 && strings.TrimSpace(arg[0]) != "" {
		remote := strings.TrimSpace(arg[0])
		res := f.verifyObject(ctx, remote, hashes, strict)
		sum := verifySummary{scanned: 1}
		f.countVerifyResult(&sum, res, strict)
		return formatVerifyReport(sum, []verifyObjectResult{{remote: remote, status: res.status, line: res.line, failed: res.failed}}), nil
	}

	remotes, err := f.listAllObjectRemotes(ctx)
	if err != nil {
		return nil, err
	}
	sum := verifySummary{}
	details := make([]verifyObjectResult, 0, len(remotes))
	for _, remote := range remotes {
		sum.scanned++
		res := f.verifyObject(ctx, remote, hashes, strict)
		f.countVerifyResult(&sum, res, strict)
		details = append(details, verifyObjectResult{remote: remote, status: res.status, line: res.line, failed: res.failed})
	}
	return formatVerifyReport(sum, details), nil
}

func (f *Fs) countVerifyResult(sum *verifySummary, res verifyObjectResult, strict bool) {
	switch res.status {
	case verifyOK:
		sum.ok++
	case verifyDegraded:
		sum.degraded++
	case verifySkew:
		sum.skew++
	case verifyCorrupt:
		sum.corrupt++
	case verifyFail:
		sum.fail++
	}
	if res.failed {
		sum.failed++
	}
}

func formatVerifyReport(sum verifySummary, details []verifyObjectResult) string {
	var sb strings.Builder
	sb.WriteString("RS Verify Summary\n")
	sb.WriteString("========================================\n")
	sb.WriteString(fmt.Sprintf("Scanned: %d\nOK: %d\nDEGRADED: %d\nSKEW: %d\nCORRUPT: %d\nFAIL: %d\nFailed: %d\n",
		sum.scanned, sum.ok, sum.degraded, sum.skew, sum.corrupt, sum.fail, sum.failed))
	if len(details) > 0 {
		sb.WriteString("\nDetails:\n")
		for _, d := range details {
			sb.WriteString(fmt.Sprintf("%-8s %s: %s\n", d.status.String(), d.remote, d.line))
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (f *Fs) verifyObject(ctx context.Context, remote string, hashes, strict bool) verifyObjectResult {
	k := f.opt.DataShards
	m := f.opt.ParityShards
	total := k + m

	if len(f.backends) == 0 {
		return verifyObjectResult{status: verifyOK, line: "shards=0/0 writeID=0x0", failed: false}
	}

	type shardInfo struct {
		present  bool
		size     int64
		footer   *Footer
		parseErr error
	}
	shards := make([]shardInfo, total)
	g, gctx := errgroup.WithContext(ctx)
	for i := 0; i < total; i++ {
		i := i
		g.Go(func() error {
			obj, err := f.backends[i].NewObject(gctx, remote)
			if err != nil {
				return nil
			}
			sz := obj.Size()
			if sz < int64(FooterSize) {
				shards[i].present = true
				shards[i].size = sz
				shards[i].parseErr = fmt.Errorf("truncated footer")
				return nil
			}
			ft, err := readFooterFromParticle(gctx, obj)
			shards[i].present = true
			shards[i].size = sz
			if err != nil {
				shards[i].parseErr = err
				return nil
			}
			if int(ft.CurrentShard) != i {
				shards[i].parseErr = fmt.Errorf("shard index mismatch: expected=%d got=%d", i, ft.CurrentShard)
				return nil
			}
			shards[i].footer = ft
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return verifyFailResult(fmt.Sprintf("%v", err))
	}

	presentCount := 0
	for i := 0; i < total; i++ {
		if shards[i].present {
			presentCount++
			if shards[i].parseErr != nil {
				if strings.Contains(shards[i].parseErr.Error(), "truncated") || strings.Contains(shards[i].parseErr.Error(), "too small") {
					return verifyFailResult("truncated footer")
				}
				return verifyFailResult(shards[i].parseErr.Error())
			}
		}
	}
	if presentCount == 0 {
		return verifyFailResult("no shards found")
	}

	var ref *Footer
	for i := 0; i < total; i++ {
		if shards[i].footer != nil {
			ref = shards[i].footer
			break
		}
	}
	for i := 0; i < total; i++ {
		ft := shards[i].footer
		if ft == nil {
			continue
		}
		if ft.Algorithm != AlgorithmSYMM {
			return verifyFailResult("unsupported algorithm (want SYMM)")
		}
		if int(ft.DataShards) != k || int(ft.ParityShards) != m {
			return verifyFailResult(fmt.Sprintf("footer k/m=%d/%d != configured %d/%d", ft.DataShards, ft.ParityShards, k, m))
		}
		if !footerLayoutCompatible(ref, ft, i) {
			return verifyFailResult("footer metadata mismatch across shards")
		}
		if ft.NumStripes == 0 || ft.StripeSize == 0 {
			if shards[i].size != int64(FooterSize) {
				return verifyFailResult(fmt.Sprintf("shard %d particle size %d, want %d", i, shards[i].size, FooterSize))
			}
		} else {
			want := ExpectedParticleSize(ref.ContentLength, i, k, m, int(ft.StripeSize), true)
			if shards[i].size != want {
				return verifyFailResult(fmt.Sprintf("shard %d particle size %d, want %d", i, shards[i].size, want))
			}
		}
	}

	sel, err := probeAndSelectWriteIDGroup(ctx, f, remote, ref, k, m)
	if err != nil {
		if errors.Is(err, errWriteIDSkew) {
			return verifyObjectResult{
				status: verifySkew,
				line:   "no unique WriteID group >= k",
				failed: true,
			}
		}
		return verifyFailResult(err.Error())
	}

	winPresent := 0
	var missing []int
	for i := 0; i < total; i++ {
		if sel.present[i] {
			winPresent++
		} else {
			missing = append(missing, i)
		}
	}

	crcg, crcctx := errgroup.WithContext(ctx)
	for i := 0; i < total; i++ {
		if !sel.present[i] {
			continue
		}
		i := i
		ft := shards[i].footer
		crcg.Go(func() error {
			obj, err := f.backends[i].NewObject(crcctx, remote)
			if err != nil {
				return fmt.Errorf("shard %d: %w", i, err)
			}
			if err := verifyParticlePayloadCRC32C(crcctx, obj, ft.PayloadCRC32C); err != nil {
				return fmt.Errorf("shard %d PayloadCRC32C mismatch", i)
			}
			return nil
		})
	}
	if err := crcg.Wait(); err != nil {
		return verifyObjectResult{
			status: verifyCorrupt,
			line:   err.Error(),
			failed: true,
		}
	}

	if strict && winPresent != total {
		return verifyObjectResult{
			status: verifyDegraded,
			line:   formatVerifyPresence(winPresent, total, missing, sel.writeID),
			failed: true,
		}
	}

	if hashes {
		if err := f.verifyObjectHashes(ctx, remote, sel.refFooter, sel.present, k, m); err != nil {
			return verifyObjectResult{
				status: verifyCorrupt,
				line:   err.Error(),
				failed: true,
			}
		}
	}

	if winPresent < total {
		return verifyObjectResult{
			status: verifyDegraded,
			line:   formatVerifyPresence(winPresent, total, missing, sel.writeID),
			failed: false,
		}
	}

	return verifyObjectResult{
		status: verifyOK,
		line:   fmt.Sprintf("shards=%d/%d writeID=0x%x", winPresent, total, sel.writeID),
		failed: false,
	}
}

func verifyFailResult(msg string) verifyObjectResult {
	return verifyObjectResult{status: verifyFail, line: msg, failed: true}
}

func formatVerifyPresence(present, total int, missing []int, writeID uint64) string {
	parts := make([]string, len(missing))
	for i, idx := range missing {
		parts[i] = fmt.Sprintf("%d", idx)
	}
	return fmt.Sprintf("present=%d/%d missing=[%s] writeID=0x%x", present, total, strings.Join(parts, ","), writeID)
}

func verifyParticlePayloadCRC32C(ctx context.Context, obj fs.Object, want uint32) error {
	sz := obj.Size()
	payloadLen := sz - int64(FooterSize)
	if payloadLen < 0 {
		return fmt.Errorf("particle too small for footer")
	}
	var got uint32
	if payloadLen == 0 {
		got = crc32cChecksum(nil)
	} else {
		rd, err := obj.Open(ctx, &fs.RangeOption{Start: 0, End: payloadLen - 1})
		if err != nil {
			return err
		}
		h := crc32.New(crc32cTable)
		if _, err := io.Copy(h, rd); err != nil {
			_ = rd.Close()
			return err
		}
		if err := rd.Close(); err != nil {
			return err
		}
		got = h.Sum32()
	}
	if got != want {
		return fmt.Errorf("PayloadCRC32C mismatch")
	}
	return nil
}

func (f *Fs) verifyObjectHashes(ctx context.Context, remote string, ref *Footer, present []bool, k, m int) error {
	if ref.ContentLength == 0 {
		if ref.MD5 != emptyFileMD5 || ref.SHA256 != emptyFileSHA256 {
			return fmt.Errorf("empty object hash mismatch")
		}
		return nil
	}
	N := int(ref.NumStripes)
	S := int64(ref.StripeSize)
	intS := int(ref.StripeSize)
	if N <= 0 || intS <= 0 {
		return fmt.Errorf("invalid stripe metadata")
	}
	enc, err := reedsolomon.New(k, m)
	if err != nil {
		return err
	}
	total := k + m
	md5h := md5.New()
	sha256h := sha256.New()
	rowBuf := make([]byte, total*intS)
	row := make([][]byte, total)
	for t := 0; t < N; t++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		logLen := StripeLogicalLen(k, S, ref.ContentLength, t)
		for i := 0; i < total; i++ {
			row[i] = nil
		}
		for i := 0; i < total; i++ {
			if !present[i] {
				continue
			}
			row[i] = rowBuf[i*intS : (i+1)*intS]
		}
		if err := readStripeFragmentsParallel(ctx, f, remote, t, k, ref.ContentLength, S, logLen, row, false); err != nil {
			return err
		}
		if _, err := reconstructInto(row, k, m); err != nil {
			return err
		}
		stripeBytes, err := reconstructStripeJoined(enc, row, k, m, intS, logLen, t)
		if err != nil {
			return err
		}
		md5h.Write(stripeBytes)
		sha256h.Write(stripeBytes)
	}
	var md5Sum [16]byte
	copy(md5Sum[:], md5h.Sum(nil))
	var sha256Sum [32]byte
	copy(sha256Sum[:], sha256h.Sum(nil))
	if md5Sum != ref.MD5 {
		return fmt.Errorf("MD5 mismatch vs footer")
	}
	if sha256Sum != ref.SHA256 {
		return fmt.Errorf("SHA256 mismatch vs footer")
	}
	return nil
}
