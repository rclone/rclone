package fs

import (
	"context"
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
	hardlinks sync.Map // map[HLinkInfo]*HLinkRootInfo
}

func (t *HLinkTracker) RegisterHLinkRoot(ctx context.Context, src Object, fsrc FsEx, dst Object, fdst FsEx, dstPath string, willTransfer bool) bool {
	srcLinkInfo, srcHasLinkInfo := fsrc.HLinkID(ctx, src)
	if !srcHasLinkInfo {
		Debugf(src, "hlinkInfo is unexpectedly null")
		return false
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
		Debugf(t, "hardlinks map unexpectedly returned non-HLinkRootInfo")
		return false
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
				Debugf(dst, "destination unexpectedly has no hlink info")
				return false
			}
		}

		// If this case is true, we have fully transferred the root and can immediately link
		if info.remoteHLinkInfo != nil {
			if info.remoteHLinkInfo != dstLinkInfo {
				Debugf(fdst, "performing hardlink %v->%v", info.remotePath, dstPath)
				err := fdst.HLink(ctx, info.remotePath, dstPath)

				if err != nil {
					Debugf(fdst, "failed to perform link %v->%v: %v\n", info.remotePath, dstPath, err)
					return false
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
			dstLinkInfo, dstHasLinkInfo := fdst.HLinkID(ctx, dst)
			if !dstHasLinkInfo {
				Debugf(dst, "destination unexpectedly has no hlink info")
			}

			info.remoteHLinkInfo = dstLinkInfo

			return false
		}

		// Otherwise, we need to transfer and indicate the caller as such
		return true
	}

	return false
}

func (t *HLinkTracker) FlushLinkrootLinkQueue(ctx context.Context, src Object, fsrc FsEx, dst Object, fdst FsEx) {
	srcHLinkInfo, srcHasHLinkInfo := fsrc.HLinkID(ctx, src)
	if !srcHasHLinkInfo {
		Debugf(src, "failed to load hardlink info")
		return
	}

	val, ok := t.hardlinks.Load(srcHLinkInfo)
	if !ok {
		Debugf(src, "failed to load hardlink root data")
		return
	}

	info, ok := val.(*HLinkRootInfo)
	if !ok {
		Debugf(src, "unexpected return type from hardlinks map")
		return
	}

	info.lock.Lock()
	defer info.lock.Unlock()

	dstHLinkInfo, dstHasHLinkInfo := fdst.HLinkID(ctx, dst)
	if !dstHasHLinkInfo {
		Debugf(dst, "failed to load hardlink info")
		return
	}
	info.remoteHLinkInfo = dstHLinkInfo

	// We probably can't count on the source's inode remaining after a transfer, so we just go ahead and relink all
	for _, tgt := range info.pendingLinkDests {
		Debugf(src, "performing pending link %v -> %v", info.remotePath, tgt)
		err := fdst.HLink(context.Background(), info.remotePath, tgt)
		if err != nil {
			Errorf(fsrc, "failed to perform link %v -> %v: %v", info.remotePath, tgt, err)
			return
		}
	}

	info.pendingLinkDests = nil
}
