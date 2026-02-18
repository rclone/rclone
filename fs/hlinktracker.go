package fs

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type HLinkRootInfo struct {
	lock             sync.Mutex
	remotePath       string
	willTransferRoot bool
	remoteHLinkInfo  any      // One of nil, WindowsHLinkInfo, UnixHLinkInfo, Plan9HLinkInfo
	pendingLinkDests []string // Link destinations on the remote encountered while transferring (indicated by remoteHLinkInfo == nil)
}

// A generic hardlink tracker for use in filesystems that support hardlinks
type HLinkTracker struct {
	hardlinks   sync.Map // map[HLinkInfo]*HLinkRootInfo
	dstToSrcMap sync.Map // map[HLinkInfo]HLinkInfo
}

func (t *HLinkTracker) RegisterHLinkRoot(ctx context.Context, src Object, fsrc Hardlinker, dst Object, fdst Hardlinker, dstPath string, willTransfer bool) (bool, error) {
	srcLinkInfo, srcHasLinkInfo := fsrc.HLinkID(ctx, src)
	if !srcHasLinkInfo {
		return false, fmt.Errorf("hlinkInfo is unexpectedly null for %v", src)
	}

	newInfo := HLinkRootInfo{
		lock:             sync.Mutex{},
		remotePath:       dstPath,
		willTransferRoot: willTransfer,
		remoteHLinkInfo:  nil,
		pendingLinkDests: []string{},
	}
	newInfo.lock.Lock()

	rawInfo, isExisting := t.hardlinks.LoadOrStore(srcLinkInfo, &newInfo)
	info, ok := rawInfo.(*HLinkRootInfo)
	if !ok {
		return false, fmt.Errorf("HLinkTracker hardlinks map unexpectedly returned non-HLinkRootInfo value")
	}

	var dstLinkInfo any
	dstHasLinkInfo := false
	if !isExisting && dst != nil && !willTransfer {
		dstLinkInfo, dstHasLinkInfo = fdst.HLinkID(ctx, dst)

		if !dstHasLinkInfo {
			return false, fmt.Errorf("destination %v unexpectedly has no hlink info", dst)
		}
	} else if isExisting && info.remoteHLinkInfo != nil {
		dstHasLinkInfo = true
		dstLinkInfo = info.remoteHLinkInfo
	}

	if dstHasLinkInfo {
		srcInodeInfoFromDst, haveSrcInodeInfoFromDst := t.dstToSrcMap.LoadOrStore(dstLinkInfo, srcLinkInfo)

		if (haveSrcInodeInfoFromDst && !isExisting) || (haveSrcInodeInfoFromDst && isExisting && info.remoteHLinkInfo != nil && (srcInodeInfoFromDst != srcLinkInfo)) {
			willTransfer = true
			isExisting = false
		}
	}

	defer info.lock.Unlock()
	if isExisting {
		// We are not the root file, so we should link the root to dst
		info.lock.Lock()

		var dstLinkInfo any
		dstHasLinkInfo := false

		if dst != nil {
			dstLinkInfo, dstHasLinkInfo = fdst.HLinkID(ctx, dst)

			if !dstHasLinkInfo {
				return false, fmt.Errorf("destination %v unexpectedly has no hlink info", dst)
			}
		}

		// If this case is true, we have fully transferred the root and can immediately link
		if info.remoteHLinkInfo != nil {
			if info.remoteHLinkInfo != dstLinkInfo {
				dstPathNoSuffix := strings.TrimSuffix(dstPath, ".rclonelink")
				srcPathNoSuffix := strings.TrimSuffix(info.remotePath, ".rclonelink")
				Debugf(fdst, "performing hardlink %v->%v", srcPathNoSuffix, dstPathNoSuffix)

				err := fdst.HLink(ctx, srcPathNoSuffix, dstPathNoSuffix)

				if err != nil {
					return false, fmt.Errorf("RegisterHlinkRoot failed to perform link %v->%v: %w", info.remotePath, dstPath, err)
				}
			}
		} else {
			// Otherwise, we add the path to the queue, to be linked post-transfer
			Debugf(fdst, "queueing hardlink %v->%v", info.remotePath, dstPath)
			info.pendingLinkDests = append(info.pendingLinkDests, dstPath)
		}
	} else {
		// This is the root
		// If we don't need to transfer, we can immediately use the existing file on the remote to link against
		Debugf(dstPath, "registering as link root")
		if !willTransfer {
			Debugf(dstPath, "not transferring root")
			if dst == nil {
				return false, fmt.Errorf("RegisterHLinkRoot destination is unexpectedly null")
			}
			dstLinkInfo, dstHasLinkInfo := fdst.HLinkID(ctx, dst)
			if !dstHasLinkInfo {
				return false, fmt.Errorf("destination %v unexpectedly has no hlink info", dstLinkInfo)
			}

			info.remoteHLinkInfo = dstLinkInfo

			return false, nil
		}

		// Otherwise, we need to transfer and indicate the caller as such
		return true, nil
	}

	return false, nil
}

func (t *HLinkTracker) FlushLinkrootLinkQueue(ctx context.Context, src Object, fsrc Hardlinker, dst Object, fdst Hardlinker) error {
	srcHLinkInfo, srcHasHLinkInfo := fsrc.HLinkID(ctx, src)
	if !srcHasHLinkInfo {
		return fmt.Errorf("failed to load hardlink info for src %v", src)
	}

	val, ok := t.hardlinks.Load(srcHLinkInfo)
	if !ok {
		return fmt.Errorf("failed to load hardlink root data for src %v", src)
	}

	info, ok := val.(*HLinkRootInfo)
	if !ok {
		return fmt.Errorf("unexpected return type from hardlinks map for src %v", src)
	}

	info.lock.Lock()
	defer info.lock.Unlock()

	dstHLinkInfo, dstHasHLinkInfo := fdst.HLinkID(ctx, dst)
	if !dstHasHLinkInfo {
		return fmt.Errorf("failed to load hardlink info for dst %v", dst)
	}
	info.remoteHLinkInfo = dstHLinkInfo

	// We probably can't count on the source's inode remaining after a transfer, so we just go ahead and relink all
	for _, tgt := range info.pendingLinkDests {
		dstPathNoSuffix := strings.TrimSuffix(tgt, ".rclonelink")
		srcPathNoSuffix := strings.TrimSuffix(info.remotePath, ".rclonelink")
		Debugf(src, "performing pending link %v -> %v", srcPathNoSuffix, dstPathNoSuffix)
		err := fdst.HLink(context.Background(), srcPathNoSuffix, dstPathNoSuffix)
		if err != nil {
			return fmt.Errorf("failed to perform link %v -> %v: %v", srcPathNoSuffix, dstPathNoSuffix, err)
		}
	}

	info.pendingLinkDests = nil
	return nil
}
