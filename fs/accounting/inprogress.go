package accounting

import (
	"context"
	"sync"

	"github.com/rclone/rclone/fs"
)

// inProgress holds a synchronized map of in progress transfers
type inProgress struct {
	mu sync.Mutex
	m  map[string]*Account
}

// newInProgress makes a new inProgress object
func newInProgress(ctx context.Context) *inProgress {
	ci := fs.GetConfig(ctx)
	return &inProgress{
		m: make(map[string]*Account, ci.Transfers),
	}
}

// set marks the name as in progress
func (ip *inProgress) set(name string, acc *Account) {
	ip.mu.Lock()
	defer ip.mu.Unlock()
	ip.m[name] = acc
}

// clear marks the name as no longer in progress
func (ip *inProgress) clear(name string) {
	ip.mu.Lock()
	defer ip.mu.Unlock()
	delete(ip.m, name)
}

// get gets the account for name, of nil if not found
func (ip *inProgress) get(name string) *Account {
	ip.mu.Lock()
	defer ip.mu.Unlock()
	return ip.m[name]
}

// merge adds items from another inProgress
func (ip *inProgress) merge(m *inProgress) {
	ip.mu.Lock()
	defer ip.mu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, val := range m.m {
		ip.m[key] = val
	}
}
