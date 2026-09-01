// Package trackrenames matches source and destination objects for server-side renames.
package trackrenames

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
)

// Strategy identifies the object attributes used to match renames.
type Strategy byte

const (
	// StrategyHash includes the configured hash in the rename identifier.
	StrategyHash Strategy = 1 << iota
	// StrategyModtime requires matching modification times within the modify window.
	StrategyModtime
	// StrategyLeaf includes the leaf name in the rename identifier.
	StrategyLeaf
)

// UsesHash reports whether the strategy compares hashes.
func (strategy Strategy) UsesHash() bool {
	return strategy&StrategyHash != 0
}

// UsesModtime reports whether the strategy compares modification times.
func (strategy Strategy) UsesModtime() bool {
	return strategy&StrategyModtime != 0
}

// UsesLeaf reports whether the strategy compares leaf names.
func (strategy Strategy) UsesLeaf() bool {
	return strategy&StrategyLeaf != 0
}

// ParseStrategy parses a comma-separated track-renames strategy.
func ParseStrategy(strategies string) (strategy Strategy, err error) {
	if strategies == "" {
		return 0, nil
	}
	for value := range strings.SplitSeq(strategies, ",") {
		switch value {
		case "hash":
			strategy |= StrategyHash
		case "modtime":
			strategy |= StrategyModtime
		case "leaf":
			strategy |= StrategyLeaf
		case "size":
			// Size is always part of the identifier.
		default:
			return strategy, fmt.Errorf("unknown track renames strategy %q", value)
		}
	}
	return strategy, nil
}

// CheckCapabilities returns the filesystem capabilities and any reasons strategy cannot be used.
func CheckCapabilities(ctx context.Context, fsrc, fdst fs.Fs, strategy Strategy) (hashType hash.Type, modifyWindow time.Duration, unsupported []string) {
	if !operations.CanServerSideMove(fdst) {
		unsupported = append(unsupported, "the destination does not support server-side move or copy")
	}

	hashType = fsrc.Hashes().Overlap(fdst.Hashes()).GetOne()
	if strategy.UsesHash() && hashType == hash.None {
		unsupported = append(unsupported, "the source and destination do not have a common hash")
	}

	modifyWindow = fs.GetModifyWindow(ctx, fsrc, fdst)
	if strategy.UsesModtime() && modifyWindow == fs.ModTimeNotSupported {
		unsupported = append(unsupported, "either the source or destination does not support modtime")
	}

	return hashType, modifyWindow, unsupported
}

// Candidate contains the object attributes used to match a tracked rename.
type Candidate struct {
	Remote  string    // Remote is the object path.
	Size    int64     // Size is the object size in bytes.
	ModTime time.Time // ModTime is the object modification time.
	Hash    string    // Hash is the value of the common hash type.
}

// Matcher stores destination objects and consumes them when a source object matches.
type Matcher struct {
	ctx          context.Context
	strategy     Strategy
	hashType     hash.Type
	modifyWindow time.Duration
	mu           sync.Mutex
	objects      map[string][]fs.Object
}

// NewMatcher constructs an empty rename matcher.
func NewMatcher(ctx context.Context, strategy Strategy, hashType hash.Type, modifyWindow time.Duration) *Matcher {
	return &Matcher{
		ctx:          ctx,
		strategy:     strategy,
		hashType:     hashType,
		modifyWindow: modifyWindow,
		objects:      make(map[string][]fs.Object),
	}
}

// Add stores a destination object and reports whether it produced a usable identifier.
func (m *Matcher) Add(obj fs.Object) bool {
	id := m.renameID(obj)
	if id == "" {
		return false
	}
	m.mu.Lock()
	m.objects[id] = append(m.objects[id], obj)
	m.mu.Unlock()
	return true
}

// Match consumes and returns the first destination object matching src.
func (m *Matcher) Match(src fs.Object) (dst fs.Object) {
	id := m.renameID(src)
	if id == "" {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	dsts := m.objects[id]
	if len(dsts) == 0 {
		return nil
	}

	match := 0
	if m.strategy.UsesModtime() {
		match = -1
		srcModTime := src.ModTime(m.ctx)
		for i, candidate := range dsts {
			delta := candidate.ModTime(m.ctx).Sub(srcModTime)
			if delta < m.modifyWindow && delta > -m.modifyWindow {
				match = i
				break
			}
		}
		if match < 0 {
			return nil
		}
	}

	dst = dsts[match]
	dsts = slices.Delete(dsts, match, match+1)
	if len(dsts) == 0 {
		delete(m.objects, id)
	} else {
		m.objects[id] = dsts
	}
	return dst
}

// CountGuaranteedMatches returns a lower bound on matches independent of matching order.
func CountGuaranteedMatches(strategy Strategy, modifyWindow time.Duration, sources, destinations []Candidate) int {
	sourceGroups := make(map[string][]Candidate)
	for _, src := range sources {
		id := candidateID(strategy, src)
		if id != "" {
			sourceGroups[id] = append(sourceGroups[id], src)
		}
	}
	destinationGroups := make(map[string][]Candidate)
	for _, dst := range destinations {
		id := candidateID(strategy, dst)
		if id != "" {
			destinationGroups[id] = append(destinationGroups[id], dst)
		}
	}

	matches := 0
	for id, srcs := range sourceGroups {
		dsts := destinationGroups[id]
		if len(dsts) == 0 {
			continue
		}
		if !strategy.UsesModtime() || allModtimesMatch(srcs, dsts, modifyWindow) {
			matches += min(len(srcs), len(dsts))
		}
	}
	return matches
}

func allModtimesMatch(srcs, dsts []Candidate, modifyWindow time.Duration) bool {
	minSrc, maxSrc := srcs[0].ModTime, srcs[0].ModTime
	for _, src := range srcs[1:] {
		modTime := src.ModTime
		if modTime.Before(minSrc) {
			minSrc = modTime
		}
		if modTime.After(maxSrc) {
			maxSrc = modTime
		}
	}
	minDst, maxDst := dsts[0].ModTime, dsts[0].ModTime
	for _, dst := range dsts[1:] {
		modTime := dst.ModTime
		if modTime.Before(minDst) {
			minDst = modTime
		}
		if modTime.After(maxDst) {
			maxDst = modTime
		}
	}
	return maxDst.Sub(minSrc) < modifyWindow && minDst.Sub(maxSrc) > -modifyWindow
}

func candidateID(strategy Strategy, candidate Candidate) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "%d", candidate.Size)

	if strategy.UsesHash() {
		if candidate.Hash == "" {
			return ""
		}
		builder.WriteRune(',')
		builder.WriteString(candidate.Hash)
	}

	if strategy.UsesLeaf() {
		builder.WriteRune(',')
		builder.WriteString(path.Base(candidate.Remote))
	}

	return builder.String()
}

func (m *Matcher) renameID(obj fs.Object) string {
	var sum string
	if m.strategy.UsesHash() {
		var err error
		sum, err = obj.Hash(m.ctx, m.hashType)
		if err != nil {
			fs.Debugf(obj, "Hash failed: %v", err)
			return ""
		}
	}
	return candidateID(m.strategy, Candidate{
		Remote: obj.Remote(),
		Size:   obj.Size(),
		Hash:   sum,
	})
}
