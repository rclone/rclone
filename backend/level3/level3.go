// Package level3 implements a backend that splits data across two remotes using byte-level striping
package level3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"golang.org/x/sync/errgroup"
)

// Register with Fs
func init() {
	fsi := &fs.RegInfo{
		Name:        "level3",
		Description: "Level 3 storage with byte-level data striping across two remotes",
		NewFs:       NewFs,
		MetadataInfo: &fs.MetadataInfo{
			Help: `Any metadata supported by the underlying remotes is read and written.`,
		},
		Options: []fs.Option{{
			Name: "even",
			Help: `Remote for even-indexed bytes (indices 0, 2, 4, ...).

This should be in the form 'remote:path'.`,
			Required: true,
		}, {
			Name: "odd",
			Help: `Remote for odd-indexed bytes (indices 1, 3, 5, ...).

This should be in the form 'remote:path'.`,
			Required: true,
		}, {
			Name: "parity",
			Help: `Remote for parity data (XOR of even and odd bytes).

This should be in the form 'remote:path'.`,
			Required: true,
		}, {
			Name:    "timeout_mode",
			Help:    "Timeout behavior for backend operations",
			Default: "standard",
			Examples: []fs.OptionExample{
				{
					Value: "standard",
					Help:  "Use global timeout settings (best for local/file storage)",
				},
				{
					Value: "balanced",
					Help:  "Moderate timeouts (3 retries, 30s) - good for reliable S3",
				},
				{
					Value: "aggressive",
					Help:  "Fast failover (1 retry, 10s) - best for S3 degraded mode",
				},
			},
		}},
	}
	fs.Register(fsi)
}

// Options defines the configuration for this backend
type Options struct {
	Even        string `config:"even"`
	Odd         string `config:"odd"`
	Parity      string `config:"parity"`
	TimeoutMode string `config:"timeout_mode"`
}

// uploadJob represents a particle that needs to be uploaded for self-healing
type uploadJob struct {
	remote       string
	particleType string // "even", "odd", or "parity"
	data         []byte
	isOddLength  bool
}

// uploadQueue manages pending self-healing uploads
type uploadQueue struct {
	mu      sync.Mutex
	pending map[string]bool // key: remote+particleType, value: true if queued
	jobs    chan *uploadJob
}

// newUploadQueue creates a new upload queue
func newUploadQueue() *uploadQueue {
	return &uploadQueue{
		pending: make(map[string]bool),
		jobs:    make(chan *uploadJob, 100), // Buffer up to 100 pending uploads
	}
}

// add adds a job to the queue (deduplicates)
func (q *uploadQueue) add(job *uploadJob) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	key := job.remote + ":" + job.particleType
	if q.pending[key] {
		return false // Already queued
	}

	q.pending[key] = true
	q.jobs <- job
	return true
}

// remove removes a job from the pending map
func (q *uploadQueue) remove(job *uploadJob) {
	q.mu.Lock()
	defer q.mu.Unlock()

	key := job.remote + ":" + job.particleType
	delete(q.pending, key)
}

// len returns the number of pending uploads
func (q *uploadQueue) len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending)
}

// Fs represents a level3 backend with striped storage and parity
type Fs struct {
	name     string       // name of this remote
	root     string       // the path we are working on
	opt      Options      // options for this Fs
	features *fs.Features // optional features
	even     fs.Fs        // remote for even-indexed bytes
	odd      fs.Fs        // remote for odd-indexed bytes
	parity   fs.Fs        // remote for parity data
	hashSet  hash.Set     // intersection of hash types

	// Self-healing infrastructure
	uploadQueue   *uploadQueue
	uploadWg      sync.WaitGroup
	uploadCtx     context.Context
	uploadCancel  context.CancelFunc
	uploadWorkers int
}

// SplitBytes splits data into even and odd indexed bytes
func SplitBytes(data []byte) (even []byte, odd []byte) {
	evenLen := (len(data) + 1) / 2
	oddLen := len(data) / 2

	even = make([]byte, evenLen)
	odd = make([]byte, oddLen)

	for i := 0; i < len(data); i++ {
		if i%2 == 0 {
			even[i/2] = data[i]
		} else {
			odd[i/2] = data[i]
		}
	}
	return even, odd
}

// MergeBytes merges even and odd indexed bytes back into original data
func MergeBytes(even []byte, odd []byte) ([]byte, error) {
	// Validate sizes: even should equal odd or be one byte larger
	if len(even) != len(odd) && len(even) != len(odd)+1 {
		return nil, fmt.Errorf("invalid particle sizes: even=%d, odd=%d (expected even=odd or even=odd+1)", len(even), len(odd))
	}

	result := make([]byte, len(even)+len(odd))
	for i := 0; i < len(result); i++ {
		if i%2 == 0 {
			result[i] = even[i/2]
		} else {
			result[i] = odd[i/2]
		}
	}
	return result, nil
}

// CalculateParity calculates XOR parity for even and odd particles
// For even-length data: parity[i] = even[i] XOR odd[i]
// For odd-length data: parity[i] = even[i] XOR odd[i], except last byte which is just even[last]
func CalculateParity(even []byte, odd []byte) []byte {
	parityLen := len(even) // Parity size always equals even size
	parity := make([]byte, parityLen)

	// XOR pairs of even and odd bytes
	for i := 0; i < len(odd); i++ {
		parity[i] = even[i] ^ odd[i]
	}

	// If odd length, last parity byte is just the last even byte (no XOR partner)
	if len(even) > len(odd) {
		parity[len(even)-1] = even[len(even)-1]
	}

	return parity
}

// ReconstructFromEvenAndParity reconstructs the original data from even + parity.
// If isOddLength is true, the last even byte equals the last parity byte.
func ReconstructFromEvenAndParity(even []byte, parity []byte, isOddLength bool) ([]byte, error) {
	if len(even) != len(parity) {
		return nil, fmt.Errorf("invalid sizes for reconstruction (even=%d parity=%d)", len(even), len(parity))
	}

	// Reconstruct odd bytes from parity ^ even
	odd := make([]byte, len(even))
	for i := 0; i < len(even); i++ {
		odd[i] = parity[i] ^ even[i]
	}
	// If original length was odd, odd has one fewer byte
	if isOddLength {
		odd = odd[:len(odd)-1]
	}

	return MergeBytes(even, odd)
}

// ReconstructFromOddAndParity reconstructs the original data from odd + parity.
// If isOddLength is true, the last even byte equals the last parity byte.
func ReconstructFromOddAndParity(odd []byte, parity []byte, isOddLength bool) ([]byte, error) {
	// Even length equals parity length. For odd original, even has one extra byte.
	even := make([]byte, len(parity))
	// Reconstruct pairs where odd exists
	for i := 0; i < len(odd); i++ {
		even[i] = parity[i] ^ odd[i]
	}
	// If original length was odd, last even byte equals last parity byte
	if isOddLength {
		even[len(even)-1] = parity[len(parity)-1]
	}

	return MergeBytes(even, odd)
}

// GetParityFilename returns the parity filename with appropriate suffix
func GetParityFilename(original string, isOddLength bool) string {
	if isOddLength {
		return original + ".parity-ol"
	}
	return original + ".parity-el"
}

// StripParitySuffix removes the parity suffix from a filename
func StripParitySuffix(filename string) (string, bool, bool) {
	if strings.HasSuffix(filename, ".parity-ol") {
		return strings.TrimSuffix(filename, ".parity-ol"), true, true
	}
	if strings.HasSuffix(filename, ".parity-el") {
		return strings.TrimSuffix(filename, ".parity-el"), true, false
	}
	return filename, false, false
}

// ValidateParticleSizes checks if particle sizes are valid
func ValidateParticleSizes(evenSize, oddSize int64) bool {
	return evenSize == oddSize || evenSize == oddSize+1
}

// applyTimeoutMode creates a context with timeout settings based on the configured mode
func applyTimeoutMode(ctx context.Context, mode string) context.Context {
	switch mode {
	case "standard", "":
		// Don't modify context - use global settings
		fs.Debugf(nil, "level3: Using standard timeout mode (global settings)")
		return ctx

	case "balanced":
		newCtx, ci := fs.AddConfig(ctx)
		ci.LowLevelRetries = 3
		ci.ConnectTimeout = fs.Duration(15 * time.Second)
		ci.Timeout = fs.Duration(30 * time.Second)
		fs.Logf(nil, "level3: Using balanced timeout mode (retries=%d, contimeout=%v, timeout=%v)",
			ci.LowLevelRetries, ci.ConnectTimeout, ci.Timeout)
		return newCtx

	case "aggressive":
		newCtx, ci := fs.AddConfig(ctx)
		ci.LowLevelRetries = 1
		ci.ConnectTimeout = fs.Duration(5 * time.Second)
		ci.Timeout = fs.Duration(10 * time.Second)
		fs.Logf(nil, "level3: Using aggressive timeout mode (retries=%d, contimeout=%v, timeout=%v)",
			ci.LowLevelRetries, ci.ConnectTimeout, ci.Timeout)
		return newCtx

	default:
		fs.Errorf(nil, "level3: Unknown timeout_mode %q, using standard", mode)
		return ctx
	}
}

// NewFs constructs an Fs from the path.
//
// The returned Fs is the actual Fs, referenced by remote in the config
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (outFs fs.Fs, err error) {
	// Parse config into Options struct
	opt := new(Options)
	err = configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	if opt.Even == "" {
		return nil, errors.New("even must be set")
	}
	if opt.Odd == "" {
		return nil, errors.New("odd must be set")
	}
	if opt.Parity == "" {
		return nil, errors.New("parity must be set")
	}
	if strings.HasPrefix(opt.Even, name+":") {
		return nil, errors.New("can't point level3 remote at itself - check the value of the even setting")
	}
	if strings.HasPrefix(opt.Odd, name+":") {
		return nil, errors.New("can't point level3 remote at itself - check the value of the odd setting")
	}
	if strings.HasPrefix(opt.Parity, name+":") {
		return nil, errors.New("can't point level3 remote at itself - check the value of the parity setting")
	}

	// Trim trailing slashes
	for strings.HasSuffix(root, "/") {
		root = root[:len(root)-1]
	}

	// Apply timeout mode to context
	ctx = applyTimeoutMode(ctx, opt.TimeoutMode)

	f := &Fs{
		name: name,
		root: root,
		opt:  *opt,
	}

	var evenErr, oddErr, parityErr error

	// Create remotes concurrently to avoid blocking on unavailable backends
	// Use a timeout context to avoid waiting forever for unavailable remotes
	// Adjust timeout based on timeout_mode
	var initTimeout time.Duration
	switch opt.TimeoutMode {
	case "aggressive":
		initTimeout = 10 * time.Second
	case "balanced":
		initTimeout = 60 * time.Second
	case "standard", "":
		initTimeout = 5 * time.Minute
	default:
		initTimeout = 10 * time.Second
	}
	initCtx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()

	type fsResult struct {
		name string
		fs   fs.Fs
		err  error
	}
	fsCh := make(chan fsResult, 3)

	// Create even remote (even-indexed bytes)
	go func() {
		evenPath := opt.Even
		if root != "" {
			evenPath = path.Join(opt.Even, root)
		}
		fs, err := cache.Get(initCtx, evenPath)
		fsCh <- fsResult{"even", fs, err}
	}()

	// Create odd remote (odd-indexed bytes)
	go func() {
		oddPath := opt.Odd
		if root != "" {
			oddPath = path.Join(opt.Odd, root)
		}
		fs, err := cache.Get(initCtx, oddPath)
		fsCh <- fsResult{"odd", fs, err}
	}()

	// Create parity remote
	go func() {
		parityPath := opt.Parity
		if root != "" {
			parityPath = path.Join(opt.Parity, root)
		}
		fs, err := cache.Get(initCtx, parityPath)
		fsCh <- fsResult{"parity", fs, err}
	}()

	// Collect results - wait for all 3 with timeout
	// In RAID 3, we can tolerate one backend being unavailable
	successCount := 0
	for i := 0; i < 3; i++ {
		select {
		case res := <-fsCh:
			switch res.name {
			case "even":
				f.even = res.fs
				evenErr = res.err
				if evenErr == nil || evenErr == fs.ErrorIsFile {
					successCount++
				} else {
					fs.Logf(f, "Warning: even remote unavailable: %v", evenErr)
				}
			case "odd":
				f.odd = res.fs
				oddErr = res.err
				if oddErr == nil || oddErr == fs.ErrorIsFile {
					successCount++
				} else {
					fs.Logf(f, "Warning: odd remote unavailable: %v", oddErr)
				}
			case "parity":
				f.parity = res.fs
				parityErr = res.err
				if parityErr == nil || parityErr == fs.ErrorIsFile {
					successCount++
				} else {
					fs.Logf(f, "Warning: parity remote unavailable: %v", parityErr)
				}
			}
		case <-initCtx.Done():
			// Context cancelled/timeout
			fs.Logf(f, "Timeout waiting for remotes, have %d/%d available", successCount, 3)
			if successCount < 2 {
				return nil, fmt.Errorf("insufficient remotes available (%d/3): %w", successCount, initCtx.Err())
			}
			// Have at least 2, can proceed in degraded mode
			goto checkRemotes
		}
	}

checkRemotes:
	// Must have at least 2 of 3 remotes for RAID 3 to function
	if successCount < 2 {
		return nil, fmt.Errorf("need at least 2 of 3 remotes available, only have %d", successCount)
	}

	// If ErrorIsFile returned, the path points to a file.
	// We need to adjust the root to point to the parent directory.
	var returnErrorIsFile bool
	if evenErr == fs.ErrorIsFile || oddErr == fs.ErrorIsFile {
		returnErrorIsFile = true
		adjustedRoot := path.Dir(f.root)
		if adjustedRoot == "." || adjustedRoot == "/" {
			adjustedRoot = ""
		}

		// Recreate upstreams with adjusted root concurrently
		initCtx2, cancel2 := context.WithTimeout(ctx, 10*time.Second)
		defer cancel2()
		fsCh2 := make(chan fsResult, 3)

		go func() {
			evenPath := opt.Even
			if adjustedRoot != "" {
				evenPath = path.Join(opt.Even, adjustedRoot)
			}
			fs, err := cache.Get(initCtx2, evenPath)
			fsCh2 <- fsResult{"even", fs, err}
		}()

		go func() {
			oddPath := opt.Odd
			if adjustedRoot != "" {
				oddPath = path.Join(opt.Odd, adjustedRoot)
			}
			fs, err := cache.Get(initCtx2, oddPath)
			fsCh2 <- fsResult{"odd", fs, err}
		}()

		go func() {
			parityPath := opt.Parity
			if adjustedRoot != "" {
				parityPath = path.Join(opt.Parity, adjustedRoot)
			}
			fs, err := cache.Get(initCtx2, parityPath)
			fsCh2 <- fsResult{"parity", fs, err}
		}()

		// Collect adjusted results
		for i := 0; i < 3; i++ {
			res := <-fsCh2
			switch res.name {
			case "even":
				f.even = res.fs
				evenErr = res.err
				if evenErr != nil && evenErr != fs.ErrorIsFile {
					return nil, fmt.Errorf("failed to create even remote: %w", evenErr)
				}
			case "odd":
				f.odd = res.fs
				oddErr = res.err
				if oddErr != nil && oddErr != fs.ErrorIsFile {
					return nil, fmt.Errorf("failed to create odd remote: %w", oddErr)
				}
			case "parity":
				f.parity = res.fs
				parityErr = res.err
				if parityErr != nil && parityErr != fs.ErrorIsFile {
					return nil, fmt.Errorf("failed to create parity remote: %w", parityErr)
				}
			}
		}

		// Update root to adjusted value
		f.root = adjustedRoot
	}

	// Get the intersection of hash types
	f.hashSet = f.even.Hashes().Overlap(f.odd.Hashes()).Overlap(f.parity.Hashes())

	f.features = (&fs.Features{
		CaseInsensitive:         f.even.Features().CaseInsensitive || f.odd.Features().CaseInsensitive || f.parity.Features().CaseInsensitive,
		DuplicateFiles:          false,
		ReadMimeType:            f.even.Features().ReadMimeType && f.odd.Features().ReadMimeType,
		WriteMimeType:           f.even.Features().WriteMimeType && f.odd.Features().WriteMimeType,
		CanHaveEmptyDirectories: f.even.Features().CanHaveEmptyDirectories && f.odd.Features().CanHaveEmptyDirectories && f.parity.Features().CanHaveEmptyDirectories,
		BucketBased:             f.even.Features().BucketBased && f.odd.Features().BucketBased && f.parity.Features().BucketBased,
	}).Fill(ctx, f)

	// Enable Move if all backends support it
	if f.even.Features().Move != nil && f.odd.Features().Move != nil && f.parity.Features().Move != nil {
		f.features.Move = f.Move
	}

	// Initialize self-healing infrastructure
	f.uploadQueue = newUploadQueue()
	f.uploadCtx, f.uploadCancel = context.WithCancel(context.Background())
	f.uploadWorkers = 2 // 2 concurrent upload workers

	// Start background upload workers
	for i := 0; i < f.uploadWorkers; i++ {
		go f.backgroundUploader(f.uploadCtx, i)
	}

	// Return ErrorIsFile if we adjusted the root for a file path
	if returnErrorIsFile {
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("level3 root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Hashes returns the supported hash types
func (f *Fs) Hashes() hash.Set {
	return f.hashSet
}

// Precision returns the precision
func (f *Fs) Precision() time.Duration {
	p1 := f.even.Precision()
	p2 := f.odd.Precision()
	p3 := f.parity.Precision()

	// Return the maximum precision
	max := p1
	if p2 > max {
		max = p2
	}
	if p3 > max {
		max = p3
	}
	return max
}

// disableRetriesForWrites creates a context with retries disabled to enforce
// strict write policy. This prevents rclone's command-level retry logic from
// creating degraded files when a backend is unavailable.
//
// Hardware RAID 3 blocks writes in degraded mode. By disabling retries, we
// ensure that if a write fails due to unavailable backend, it fails immediately
// without retry attempts that could create partial/degraded files.
func (f *Fs) disableRetriesForWrites(ctx context.Context) context.Context {
	newCtx, ci := fs.AddConfig(ctx)
	ci.LowLevelRetries = 0 // Disable retries - fail fast
	fs.Debugf(f, "Disabled retries for write operation (strict RAID 3 policy)")
	return newCtx
}

// checkAllBackendsAvailable performs a quick health check to see if all three
// backends are reachable. This is used to enforce strict write policy.
//
// Returns: error if any backend is unavailable
func (f *Fs) checkAllBackendsAvailable(ctx context.Context) error {
	// Quick timeout for health check
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	type healthResult struct {
		name string
		err  error
	}
	results := make(chan healthResult, 3)

	// Check each backend by attempting to list root
	go func() {
		_, err := f.even.List(checkCtx, "")
		results <- healthResult{"even", err}
	}()
	go func() {
		_, err := f.odd.List(checkCtx, "")
		results <- healthResult{"odd", err}
	}()
	go func() {
		_, err := f.parity.List(checkCtx, "")
		results <- healthResult{"parity", err}
	}()

	// Collect results
	for i := 0; i < 3; i++ {
		result := <-results
		if result.err != nil {
			return fmt.Errorf("%s backend unavailable: %w", result.name, result.err)
		}
	}

	return nil
}

// Shutdown waits for pending self-healing uploads to complete
func (f *Fs) Shutdown(ctx context.Context) error {
	// Check if there are pending uploads
	if f.uploadQueue.len() == 0 {
		f.uploadCancel() // Cancel workers
		return nil
	}

	fs.Infof(f, "Waiting for %d self-healing upload(s) to complete...", f.uploadQueue.len())

	// Close the job channel to signal no more jobs
	close(f.uploadQueue.jobs)

	// Wait for pending uploads with timeout
	done := make(chan struct{})
	go func() {
		f.uploadWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fs.Infof(f, "Self-healing complete")
		f.uploadCancel()
		return nil
	case <-time.After(60 * time.Second):
		fs.Errorf(f, "Timeout waiting for self-healing uploads (some may be incomplete)")
		f.uploadCancel()
		return errors.New("timeout waiting for self-healing uploads")
	case <-ctx.Done():
		fs.Errorf(f, "Context cancelled while waiting for self-healing uploads")
		f.uploadCancel()
		return ctx.Err()
	}
}

// backgroundUploader runs as a goroutine to process self-healing uploads
func (f *Fs) backgroundUploader(ctx context.Context, workerID int) {
	fs.Debugf(f, "Self-healing worker %d started", workerID)
	defer fs.Debugf(f, "Self-healing worker %d stopped", workerID)

	for {
		select {
		case job, ok := <-f.uploadQueue.jobs:
			if !ok {
				// Channel closed, exit
				return
			}

			fs.Infof(f, "Self-healing: uploading %s particle for %s", job.particleType, job.remote)

			err := f.uploadParticle(ctx, job)
			if err != nil {
				fs.Errorf(f, "Self-healing upload failed for %s (%s): %v", job.remote, job.particleType, err)
				// TODO: Could implement retry logic here
			} else {
				fs.Infof(f, "Self-healing upload completed for %s (%s)", job.remote, job.particleType)
			}

			// Remove from pending map and mark as done
			f.uploadQueue.remove(job)
			f.uploadWg.Done()

		case <-ctx.Done():
			// Context cancelled, exit
			return
		}
	}
}

// uploadParticle uploads a single particle to its backend
func (f *Fs) uploadParticle(ctx context.Context, job *uploadJob) error {
	var targetFs fs.Fs
	var filename string

	switch job.particleType {
	case "even":
		targetFs = f.even
		filename = job.remote
	case "odd":
		targetFs = f.odd
		filename = job.remote
	case "parity":
		targetFs = f.parity
		filename = GetParityFilename(job.remote, job.isOddLength)
	default:
		return fmt.Errorf("unknown particle type: %s", job.particleType)
	}

	// Create a basic ObjectInfo for the particle
	baseInfo := object.NewStaticObjectInfo(filename, time.Now(), int64(len(job.data)), true, nil, nil)

	src := &particleObjectInfo{
		ObjectInfo: baseInfo,
		remote:     filename,
		size:       int64(len(job.data)),
	}

	// Upload the particle
	reader := bytes.NewReader(job.data)
	_, err := targetFs.Put(ctx, reader, src)
	return err
}

// queueParticleUpload queues a particle for background upload
func (f *Fs) queueParticleUpload(remote, particleType string, data []byte, isOddLength bool) {
	job := &uploadJob{
		remote:       remote,
		particleType: particleType,
		data:         data,
		isOddLength:  isOddLength,
	}

	if f.uploadQueue.add(job) {
		f.uploadWg.Add(1)
		fs.Infof(f, "Queued %s particle for self-healing upload: %s", particleType, remote)
	} else {
		fs.Debugf(f, "Upload already queued for %s particle: %s", particleType, remote)
	}
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	// Get entries from all three remotes concurrently
	type listResult struct {
		name    string
		entries fs.DirEntries
		err     error
	}
	listCh := make(chan listResult, 3)

	go func() {
		entries, err := f.even.List(ctx, dir)
		listCh <- listResult{"even", entries, err}
	}()

	go func() {
		entries, err := f.odd.List(ctx, dir)
		listCh <- listResult{"odd", entries, err}
	}()

	go func() {
		entries, err := f.parity.List(ctx, dir)
		listCh <- listResult{"parity", entries, err}
	}()

	// Collect results
	var entriesEven, entriesOdd, entriesParity fs.DirEntries
	var errEven, errOdd error
	for i := 0; i < 3; i++ {
		res := <-listCh
		switch res.name {
		case "even":
			entriesEven = res.entries
			errEven = res.err
		case "odd":
			entriesOdd = res.entries
			errOdd = res.err
		case "parity":
			entriesParity = res.entries
			// Ignore parity errors
		}
	}

	// If even fails, try odd
	if errEven != nil {
		if errOdd != nil {
			return nil, errEven // Return even error
		}
		// Continue with odd entries
	}

	// Create a map to track all entries (excluding parity files with suffixes)
	entryMap := make(map[string]fs.DirEntry)

	// Add even entries
	for _, entry := range entriesEven {
		entryMap[entry.Remote()] = entry
	}

	// Add odd entries (merge with even)
	for _, entry := range entriesOdd {
		if _, exists := entryMap[entry.Remote()]; !exists {
			entryMap[entry.Remote()] = entry
		}
	}

	// Filter out parity files from parity backend (they have .parity-el or .parity-ol suffix)
	// but include directories
	for _, entry := range entriesParity {
		remote := entry.Remote()
		// Strip parity suffix if it's a parity file
		_, isParity, _ := StripParitySuffix(remote)
		if isParity {
			// Don't add parity files to the list
			continue
		}
		// Add non-parity entries (directories mainly)
		if _, exists := entryMap[remote]; !exists {
			entryMap[remote] = entry
		}
	}

	// Convert map back to slice
	entries = make(fs.DirEntries, 0, len(entryMap))
	for _, entry := range entryMap {
		switch e := entry.(type) {
		case fs.Object:
			entries = append(entries, &Object{
				fs:     f,
				remote: e.Remote(),
			})
		case fs.Directory:
			entries = append(entries, &Directory{
				fs:     f,
				remote: e.Remote(),
			})
		}
	}

	return entries, nil
}

// NewObject creates a new remote Object
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	// Probe particles - must have at least 2 of 3 for RAID 3
	_, errEven := f.even.NewObject(ctx, remote)
	_, errOdd := f.odd.NewObject(ctx, remote)
	// Try both parity suffixes to detect presence
	_, errParityOL := f.parity.NewObject(ctx, GetParityFilename(remote, true))
	_, errParityEL := f.parity.NewObject(ctx, GetParityFilename(remote, false))

	parityPresent := errParityOL == nil || errParityEL == nil
	evenPresent := errEven == nil
	oddPresent := errOdd == nil

	// Allow object if any two of the three are present
	presentCount := 0
	if evenPresent {
		presentCount++
	}
	if oddPresent {
		presentCount++
	}
	if parityPresent {
		presentCount++
	}
	if presentCount < 2 {
		// Prefer returning a not found style error
		if errEven != nil {
			return nil, errEven
		}
		if errOdd != nil {
			return nil, errOdd
		}
		if !parityPresent {
			return nil, fs.ErrorObjectNotFound
		}
	}

	return &Object{fs: f, remote: remote}, nil
}

// particleObjectInfo wraps fs.ObjectInfo with a different size and optionally different remote name for particles
type particleObjectInfo struct {
	fs.ObjectInfo
	size   int64
	remote string // Override remote name (for parity files with suffix)
}

func (p *particleObjectInfo) Size() int64 {
	return p.size
}

func (p *particleObjectInfo) Remote() string {
	if p.remote != "" {
		return p.remote
	}
	return p.ObjectInfo.Remote()
}

// Hash returns an empty string to force the backend to recalculate hashes
// from the actual particle data instead of using the original file's hash
func (p *particleObjectInfo) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", nil
}

// Put uploads an object
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Pre-flight check: Enforce strict RAID 3 write policy
	// Fail immediately if any backend is unavailable to prevent degraded writes
	if err := f.checkAllBackendsAvailable(ctx); err != nil {
		return nil, fmt.Errorf("write blocked in degraded mode (RAID 3 policy): %w", err)
	}

	// Disable retries for strict RAID 3 write policy
	// This prevents rclone's retry logic from creating degraded files
	ctx = f.disableRetriesForWrites(ctx)

	// Read all data
	data, err := io.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	// Split into even and odd bytes
	evenData, oddData := SplitBytes(data)

	// Calculate parity
	parityData := CalculateParity(evenData, oddData)

	// Determine if original data is odd length
	isOddLength := len(data)%2 == 1

	// Create wrapper ObjectInfo for each particle
	evenInfo := &particleObjectInfo{
		ObjectInfo: src,
		size:       int64(len(evenData)),
	}
	oddInfo := &particleObjectInfo{
		ObjectInfo: src,
		size:       int64(len(oddData)),
	}
	parityInfo := &particleObjectInfo{
		ObjectInfo: src,
		size:       int64(len(parityData)),
		remote:     GetParityFilename(src.Remote(), isOddLength),
	}

	g, gCtx := errgroup.WithContext(ctx)

	// Upload even bytes
	g.Go(func() error {
		reader := bytes.NewReader(evenData)
		_, err := f.even.Put(gCtx, reader, evenInfo, options...)
		if err != nil {
			return fmt.Errorf("failed to upload even particle: %w", err)
		}
		return nil
	})

	// Upload odd bytes
	g.Go(func() error {
		reader := bytes.NewReader(oddData)
		_, err := f.odd.Put(gCtx, reader, oddInfo, options...)
		if err != nil {
			return fmt.Errorf("failed to upload odd particle: %w", err)
		}
		return nil
	})

	// Upload parity
	g.Go(func() error {
		reader := bytes.NewReader(parityData)
		_, err := f.parity.Put(gCtx, reader, parityInfo, options...)
		if err != nil {
			return fmt.Errorf("failed to upload parity particle: %w", err)
		}
		return nil
	})

	err = g.Wait()
	if err != nil {
		return nil, err
	}

	return &Object{
		fs:     f,
		remote: src.Remote(),
	}, nil
}

// Mkdir makes a directory
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		err := f.even.Mkdir(gCtx, dir)
		if err != nil {
			return fmt.Errorf("even mkdir failed: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		err := f.odd.Mkdir(gCtx, dir)
		if err != nil {
			return fmt.Errorf("odd mkdir failed: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		err := f.parity.Mkdir(gCtx, dir)
		if err != nil {
			return fmt.Errorf("parity mkdir failed: %w", err)
		}
		return nil
	})

	return g.Wait()
}

// Rmdir removes a directory
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return f.even.Rmdir(gCtx, dir)
	})

	g.Go(func() error {
		return f.odd.Rmdir(gCtx, dir)
	})

	g.Go(func() error {
		return f.parity.Rmdir(gCtx, dir)
	})

	return g.Wait()
}

// Move src to this remote using server-side move operations if possible
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	// Pre-flight check: Enforce strict RAID 3 write policy
	// Fail immediately if any backend is unavailable to prevent degraded moves
	if err := f.checkAllBackendsAvailable(ctx); err != nil {
		return nil, fmt.Errorf("move blocked in degraded mode (RAID 3 policy): %w", err)
	}

	// Disable retries for strict RAID 3 write policy
	ctx = f.disableRetriesForWrites(ctx)

	// Check if src is from this level3 backend
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}

	// We need to determine the suffix for source parity file
	// by checking both possible suffixes
	var srcParityName string
	parityOddSrc := GetParityFilename(srcObj.remote, true)
	parityEvenSrc := GetParityFilename(srcObj.remote, false)

	// Check which parity file exists
	_, errOdd := f.parity.NewObject(ctx, parityOddSrc)
	if errOdd == nil {
		srcParityName = parityOddSrc
	} else {
		srcParityName = parityEvenSrc
	}

	// Determine suffix from source parity name
	_, isParity, isOddLength := StripParitySuffix(srcParityName)
	if !isParity {
		isOddLength = false // Default to even if no parity found
	}

	// Get destination parity name
	dstParityName := GetParityFilename(remote, isOddLength)

	g, gCtx := errgroup.WithContext(ctx)

	// Move on even
	g.Go(func() error {
		obj, err := f.even.NewObject(gCtx, srcObj.remote)
		if err != nil {
			return nil // Ignore if not found
		}
		if do := f.even.Features().Move; do != nil {
			_, err = do(gCtx, obj, remote)
			return err
		}
		return fs.ErrorCantMove
	})

	// Move on odd
	g.Go(func() error {
		obj, err := f.odd.NewObject(gCtx, srcObj.remote)
		if err != nil {
			return nil // Ignore if not found
		}
		if do := f.odd.Features().Move; do != nil {
			_, err = do(gCtx, obj, remote)
			return err
		}
		return fs.ErrorCantMove
	})

	// Move parity
	g.Go(func() error {
		obj, err := f.parity.NewObject(gCtx, srcParityName)
		if err != nil {
			return nil // Ignore if not found
		}
		if do := f.parity.Features().Move; do != nil {
			_, err = do(gCtx, obj, dstParityName)
			return err
		}
		return fs.ErrorCantMove
	})

	err := g.Wait()
	if err != nil {
		return nil, err
	}

	return &Object{
		fs:     f,
		remote: remote,
	}, nil
}

// Object represents a striped object
type Object struct {
	fs     *Fs
	remote string
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// String returns a description of the Object
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// ModTime returns the modification time
func (o *Object) ModTime(ctx context.Context) time.Time {
	// Get from even
	obj, err := o.fs.even.NewObject(ctx, o.remote)
	if err == nil {
		return obj.ModTime(ctx)
	}
	// Fallback to odd
	obj, err = o.fs.odd.NewObject(ctx, o.remote)
	if err == nil {
		return obj.ModTime(ctx)
	}
	return time.Now()
}

// Size returns the size of the reconstructed object
func (o *Object) Size() int64 {
	ctx := context.Background()
	// Try to fetch particles
	evenObj, errEven := o.fs.even.NewObject(ctx, o.remote)
	oddObj, errOdd := o.fs.odd.NewObject(ctx, o.remote)

	// Fast path: both data particles exist
	if errEven == nil && errOdd == nil {
		return evenObj.Size() + oddObj.Size()
	}

	// Otherwise try parity with either suffix
	// Determine which parity exists and whether original length is odd
	parityObj, errParity := o.fs.parity.NewObject(ctx, GetParityFilename(o.remote, true))
	isOddLength := false
	if errParity != nil {
		parityObj, errParity = o.fs.parity.NewObject(ctx, GetParityFilename(o.remote, false))
		if errParity != nil {
			return -1
		}
		isOddLength = false // .parity-el
	} else {
		isOddLength = true // .parity-ol
	}

	// If we have one data particle and parity, we can compute size
	if errEven == nil {
		// Missing odd: N = even + parity - (isOdd ? 1 : 0)
		if isOddLength {
			return evenObj.Size() + parityObj.Size() - 1
		}
		return evenObj.Size() + parityObj.Size()
	}
	if errOdd == nil {
		// Missing even: N = odd + parity (regardless of odd/even length)
		return oddObj.Size() + parityObj.Size()
	}

	return -1
}

// Hash returns the hash of the reconstructed object
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	// Must reconstruct the full file to calculate hash
	reader, err := o.Open(ctx)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	// Calculate hash of merged data
	hasher, err := hash.NewMultiHasherTypes(hash.NewHashSet(ty))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(hasher, reader); err != nil {
		return "", err
	}

	return hasher.SumString(ty, false)
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the modification time
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		obj, err := o.fs.even.NewObject(gCtx, o.remote)
		if err != nil {
			return err
		}
		return obj.SetModTime(gCtx, t)
	})

	g.Go(func() error {
		obj, err := o.fs.odd.NewObject(gCtx, o.remote)
		if err != nil {
			return err
		}
		return obj.SetModTime(gCtx, t)
	})

	// Also set mod time on parity files
	g.Go(func() error {
		// Try both suffixes to find the parity file
		parityOdd := GetParityFilename(o.remote, true)
		obj, err := o.fs.parity.NewObject(gCtx, parityOdd)
		if err == nil {
			return obj.SetModTime(gCtx, t)
		}

		parityEven := GetParityFilename(o.remote, false)
		obj, err = o.fs.parity.NewObject(gCtx, parityEven)
		if err == nil {
			return obj.SetModTime(gCtx, t)
		}
		return nil // Ignore if parity not found
	})

	return g.Wait()
}

// Open opens the object for read, reconstructing from particles
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	fs.Debugf(o, "Open: attempting to get even and odd particles")

	// Attempt to open both data particles concurrently to avoid blocking
	type objResult struct {
		obj fs.Object
		err error
	}
	evenCh := make(chan objResult, 1)
	oddCh := make(chan objResult, 1)

	go func() {
		fs.Debugf(o, "Open: fetching even particle")
		obj, err := o.fs.even.NewObject(ctx, o.remote)
		fs.Debugf(o, "Open: even particle result: err=%v", err)
		evenCh <- objResult{obj, err}
	}()

	go func() {
		fs.Debugf(o, "Open: fetching odd particle")
		obj, err := o.fs.odd.NewObject(ctx, o.remote)
		fs.Debugf(o, "Open: odd particle result: err=%v", err)
		oddCh <- objResult{obj, err}
	}()

	// Wait for both results
	fs.Debugf(o, "Open: waiting for particle results")
	evenRes := <-evenCh
	oddRes := <-oddCh
	evenObj, errEven := evenRes.obj, evenRes.err
	oddObj, errOdd := oddRes.obj, oddRes.err
	fs.Debugf(o, "Open: got results - even err=%v, odd err=%v", errEven, errOdd)

	var merged []byte
	if errEven == nil && errOdd == nil {
		// Validate sizes
		if !ValidateParticleSizes(evenObj.Size(), oddObj.Size()) {
			return nil, fmt.Errorf("invalid particle sizes: even=%d, odd=%d", evenObj.Size(), oddObj.Size())
		}
		// Read both and merge
		evenReader, err := evenObj.Open(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to open even particle: %w", err)
		}
		oddReader, err := oddObj.Open(ctx)
		if err != nil {
			evenReader.Close()
			return nil, fmt.Errorf("failed to open odd particle: %w", err)
		}
		evenData, err := io.ReadAll(evenReader)
		evenReader.Close()
		if err != nil {
			oddReader.Close()
			return nil, fmt.Errorf("failed to read even particle: %w", err)
		}
		oddData, err := io.ReadAll(oddReader)
		oddReader.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read odd particle: %w", err)
		}
		merged, err = MergeBytes(evenData, oddData)
		if err != nil {
			return nil, fmt.Errorf("failed to merge particles: %w", err)
		}
	} else {
		// One particle missing - attempt reconstruction using parity
		// Find which parity exists and infer original length type
		parityNameOL := GetParityFilename(o.remote, true)
		parityObj, errParity := o.fs.parity.NewObject(ctx, parityNameOL)
		isOddLength := false
		if errParity == nil {
			isOddLength = true
		} else {
			parityNameEL := GetParityFilename(o.remote, false)
			parityObj, errParity = o.fs.parity.NewObject(ctx, parityNameEL)
			if errParity != nil {
				// Can't reconstruct - not enough particles
				if errEven != nil && errOdd != nil {
					return nil, fmt.Errorf("missing particles: even and odd unavailable and no parity found")
				}
				if errEven != nil {
					return nil, fmt.Errorf("missing even particle and no parity found: %w", errEven)
				}
				return nil, fmt.Errorf("missing odd particle and no parity found: %w", errOdd)
			}
			isOddLength = false
		}

		// Open known data + parity and reconstruct
		if errEven == nil {
			evenReader, err := evenObj.Open(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to open even particle: %w", err)
			}
			parityReader, err := parityObj.Open(ctx)
			if err != nil {
				evenReader.Close()
				return nil, fmt.Errorf("failed to open parity particle: %w", err)
			}
			evenData, err := io.ReadAll(evenReader)
			evenReader.Close()
			if err != nil {
				parityReader.Close()
				return nil, fmt.Errorf("failed to read even particle: %w", err)
			}
			parityData, err := io.ReadAll(parityReader)
			parityReader.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to read parity particle: %w", err)
			}
			merged, err = ReconstructFromEvenAndParity(evenData, parityData, isOddLength)
			if err != nil {
				return nil, err
			}
			fs.Infof(o, "Reconstructed %s from even+parity (degraded mode)", o.remote)

			// Self-healing: queue missing odd particle for upload
			_, oddData := SplitBytes(merged)
			o.fs.queueParticleUpload(o.remote, "odd", oddData, isOddLength)

		} else if errOdd == nil {
			oddReader, err := oddObj.Open(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to open odd particle: %w", err)
			}
			parityReader, err := parityObj.Open(ctx)
			if err != nil {
				oddReader.Close()
				return nil, fmt.Errorf("failed to open parity particle: %w", err)
			}
			oddData, err := io.ReadAll(oddReader)
			oddReader.Close()
			if err != nil {
				parityReader.Close()
				return nil, fmt.Errorf("failed to read odd particle: %w", err)
			}
			parityData, err := io.ReadAll(parityReader)
			parityReader.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to read parity particle: %w", err)
			}
			merged, err = ReconstructFromOddAndParity(oddData, parityData, isOddLength)
			if err != nil {
				return nil, err
			}
			fs.Infof(o, "Reconstructed %s from odd+parity (degraded mode)", o.remote)

			// Self-healing: queue missing even particle for upload
			evenData, _ := SplitBytes(merged)
			o.fs.queueParticleUpload(o.remote, "even", evenData, isOddLength)

		} else {
			return nil, fmt.Errorf("cannot reconstruct: no data particle available")
		}
	}

	// Handle range/seek options on the merged data
	reader := bytes.NewReader(merged)

	// Parse range option if present
	var rangeStart, rangeEnd int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.RangeOption:
			rangeStart, rangeEnd = x.Start, x.End
		case *fs.SeekOption:
			rangeStart = x.Offset
			rangeEnd = -1
		}
	}

	// Apply range if specified
	if rangeStart > 0 || rangeEnd >= 0 {
		totalSize := int64(len(merged))

		// Handle negative start (from end)
		if rangeStart < 0 && rangeEnd >= 0 {
			rangeStart = totalSize - rangeEnd
			rangeEnd = -1
		}

		// Validate range
		if rangeStart < 0 {
			rangeStart = 0
		}
		if rangeStart > totalSize {
			rangeStart = totalSize
		}

		// Calculate end
		var rangedData []byte
		if rangeEnd < 0 || rangeEnd >= totalSize {
			// Read to end
			rangedData = merged[rangeStart:]
		} else {
			// Read specific range (end is inclusive)
			if rangeEnd >= rangeStart {
				rangedData = merged[rangeStart : rangeEnd+1]
			} else {
				rangedData = []byte{}
			}
		}

		reader = bytes.NewReader(rangedData)
	}

	// Return as ReadCloser (bytes.NewReader supports seeking)
	return io.NopCloser(reader), nil
}

// Update updates the object
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// Pre-flight check: Enforce strict RAID 3 write policy
	// Fail immediately if any backend is unavailable to prevent corrupted updates
	if err := o.fs.checkAllBackendsAvailable(ctx); err != nil {
		return fmt.Errorf("update blocked in degraded mode (RAID 3 policy): %w", err)
	}

	// Disable retries for strict RAID 3 write policy
	ctx = o.fs.disableRetriesForWrites(ctx)

	// Read original particle sizes for rollback validation
	originalEvenObj, errEven := o.fs.even.NewObject(ctx, o.remote)
	originalOddObj, errOdd := o.fs.odd.NewObject(ctx, o.remote)

	var originalEvenSize, originalOddSize int64
	if errEven == nil {
		originalEvenSize = originalEvenObj.Size()
	}
	if errOdd == nil {
		originalOddSize = originalOddObj.Size()
	}

	// Read data once
	data, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	// Split into even and odd bytes
	evenData, oddData := SplitBytes(data)

	// Calculate parity
	parityData := CalculateParity(evenData, oddData)

	// Determine if original data is odd length
	isOddLength := len(data)%2 == 1

	// Create wrapper ObjectInfo for each particle
	evenInfo := &particleObjectInfo{
		ObjectInfo: src,
		size:       int64(len(evenData)),
	}
	oddInfo := &particleObjectInfo{
		ObjectInfo: src,
		size:       int64(len(oddData)),
	}
	parityInfo := &particleObjectInfo{
		ObjectInfo: src,
		size:       int64(len(parityData)),
		remote:     GetParityFilename(o.remote, isOddLength),
	}

	g, gCtx := errgroup.WithContext(ctx)

	// Update even
	g.Go(func() error {
		obj, err := o.fs.even.NewObject(gCtx, o.remote)
		if err != nil {
			return fmt.Errorf("even particle not found: %w", err)
		}
		reader := bytes.NewReader(evenData)
		err = obj.Update(gCtx, reader, evenInfo, options...)
		if err != nil {
			return fmt.Errorf("failed to update even particle: %w", err)
		}
		return nil
	})

	// Update odd
	g.Go(func() error {
		obj, err := o.fs.odd.NewObject(gCtx, o.remote)
		if err != nil {
			return fmt.Errorf("odd particle not found: %w", err)
		}
		reader := bytes.NewReader(oddData)
		err = obj.Update(gCtx, reader, oddInfo, options...)
		if err != nil {
			return fmt.Errorf("failed to update odd particle: %w", err)
		}
		return nil
	})

	// Update or create parity
	g.Go(func() error {
		parityName := GetParityFilename(o.remote, isOddLength)
		obj, err := o.fs.parity.NewObject(gCtx, parityName)
		reader := bytes.NewReader(parityData)
		if err != nil {
			// Parity doesn't exist, create it
			_, err = o.fs.parity.Put(gCtx, reader, parityInfo, options...)
			if err != nil {
				return fmt.Errorf("failed to create parity particle: %w", err)
			}
			return nil
		}
		// Parity exists, update it
		err = obj.Update(gCtx, reader, parityInfo, options...)
		if err != nil {
			return fmt.Errorf("failed to update parity particle: %w", err)
		}
		return nil
	})

	err = g.Wait()
	if err != nil {
		return err
	}

	// CRITICAL: Validate particle sizes after update to prevent corruption
	// This catches cases where partial updates occurred before error
	evenObj, errEvenNew := o.fs.even.NewObject(ctx, o.remote)
	oddObj, errOddNew := o.fs.odd.NewObject(ctx, o.remote)

	if errEvenNew != nil || errOddNew != nil {
		return fmt.Errorf("update validation failed: particles missing after update")
	}

	if !ValidateParticleSizes(evenObj.Size(), oddObj.Size()) {
		fs.Errorf(o, "CORRUPTION DETECTED: invalid particle sizes after update: even=%d, odd=%d (expected %d, %d)",
			evenObj.Size(), oddObj.Size(), len(evenData), len(oddData))

		// Attempt to restore original sizes (best effort)
		if originalEvenSize > 0 && originalOddSize > 0 {
			fs.Errorf(o, "Update created corrupted state - original sizes were even=%d, odd=%d",
				originalEvenSize, originalOddSize)
		}

		return fmt.Errorf("update failed: invalid particle sizes (even=%d, odd=%d) - FILE MAY BE CORRUPTED",
			evenObj.Size(), oddObj.Size())
	}

	fs.Debugf(o, "Update successful, validated particle sizes: even=%d, odd=%d", evenObj.Size(), oddObj.Size())
	return nil
}

// Remove removes the object
func (o *Object) Remove(ctx context.Context) error {
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		obj, err := o.fs.even.NewObject(gCtx, o.remote)
		if err != nil {
			return nil // Ignore if not found
		}
		return obj.Remove(gCtx)
	})

	g.Go(func() error {
		obj, err := o.fs.odd.NewObject(gCtx, o.remote)
		if err != nil {
			return nil // Ignore if not found
		}
		return obj.Remove(gCtx)
	})

	// Remove parity (try both suffixes)
	g.Go(func() error {
		parityOdd := GetParityFilename(o.remote, true)
		obj, err := o.fs.parity.NewObject(gCtx, parityOdd)
		if err == nil {
			return obj.Remove(gCtx)
		}

		parityEven := GetParityFilename(o.remote, false)
		obj, err = o.fs.parity.NewObject(gCtx, parityEven)
		if err == nil {
			return obj.Remove(gCtx)
		}
		return nil // Ignore if parity not found
	})

	return g.Wait()
}

// Directory represents a directory in the level3 backend
type Directory struct {
	fs     *Fs
	remote string
}

// Fs returns the parent Fs
func (d *Directory) Fs() fs.Info {
	return d.fs
}

// String returns a description of the Directory
func (d *Directory) String() string {
	if d == nil {
		return "<nil>"
	}
	return d.remote
}

// Remote returns the remote path
func (d *Directory) Remote() string {
	return d.remote
}

// ModTime returns the modification time
func (d *Directory) ModTime(ctx context.Context) time.Time {
	return time.Now()
}

// Size returns the size (always 0 for directories)
func (d *Directory) Size() int64 {
	return 0
}

// Items returns the count of items in the directory
func (d *Directory) Items() int64 {
	return -1
}

// ID returns the internal ID of the directory
func (d *Directory) ID() string {
	return ""
}

// Check the interfaces are satisfied
var (
	_ fs.Fs     = (*Fs)(nil)
	_ fs.Object = (*Object)(nil)
)
