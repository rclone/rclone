package quatrix

import (
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
)

// UploadMemoryManager dynamically calculates every chunk size for the transfer and increases or decreases it
// depending on the upload speed. This makes general upload time smaller, because transfers that are faster
// does not have to wait for the slower ones until they finish upload.
type UploadMemoryManager struct {
	m              sync.Mutex
	useDynamicSize bool
	shared         int64
	reserved       int64
	effectiveTime  time.Duration
	fileUsage      map[string]int64
}

// NewUploadMemoryManager is a constructor for UploadMemoryManager
func NewUploadMemoryManager(ci *fs.ConfigInfo, opt *Options) *UploadMemoryManager {
	useDynamicSize := true

	sharedMemory := int64(opt.MaximalSummaryChunkSize) - int64(opt.MinimalChunkSize)*int64(ci.Transfers)
	if sharedMemory <= 0 {
		sharedMemory = 0
		useDynamicSize = false
	}

	return &UploadMemoryManager{
		useDynamicSize: useDynamicSize,
		shared:         sharedMemory,
		reserved:       int64(opt.MinimalChunkSize),
		effectiveTime:  time.Duration(opt.EffectiveUploadTime),
		fileUsage:      map[string]int64{},
	}
}

// Consume -- decide amount of memory to consume
func (u *UploadMemoryManager) Consume(fileID string, neededMemory int64, speed float64) int64 {
	if !u.useDynamicSize {
		if neededMemory < u.reserved {
			return neededMemory
		}

		return u.reserved
	}

	u.m.Lock()
	defer u.m.Unlock()

	borrowed, found := u.fileUsage[fileID]
	if found {
		u.shared += borrowed
		borrowed = 0
	}

	defer func() { u.fileUsage[fileID] = borrowed }()

	effectiveChunkSize := int64(speed * u.effectiveTime.Seconds())

	if effectiveChunkSize < u.reserved {
		effectiveChunkSize = u.reserved
	}

	if neededMemory < effectiveChunkSize {
		effectiveChunkSize = neededMemory
	}

	if effectiveChunkSize <= u.reserved {
		return effectiveChunkSize
	}

	toBorrow := effectiveChunkSize - u.reserved

	if toBorrow <= u.shared {
		u.shared -= toBorrow
		borrowed = toBorrow

		return effectiveChunkSize
	}

	borrowed = u.shared
	u.shared = 0

	return borrowed + u.reserved
}

// Return returns consumed memory for the previous chunk upload to the memory pool
func (u *UploadMemoryManager) Return(fileID string) {
	if !u.useDynamicSize {
		return
	}

	u.m.Lock()
	defer u.m.Unlock()

	borrowed, found := u.fileUsage[fileID]
	if !found {
		return
	}

	u.shared += borrowed

	delete(u.fileUsage, fileID)
}
