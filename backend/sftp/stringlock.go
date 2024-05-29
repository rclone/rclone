//go:build !plan9

package sftp

import "sync"

// stringLock locks for string IDs passed in
type stringLock struct {
	mu    sync.Mutex               // mutex to protect below
	locks map[string]chan struct{} // map of locks
}

// newStringLock creates a stringLock
func newStringLock() *stringLock {
	return &stringLock{
		locks: make(map[string]chan struct{}),
	}
}

// Lock locks on the id passed in
func (l *stringLock) Lock(ID string) {
	l.mu.Lock()
	for {
		ch, ok := l.locks[ID]
		if !ok {
			break
		}
		// Wait for the channel to be closed
		l.mu.Unlock()
		// fs.Logf(nil, "Waiting for stringLock on %q", ID)
		<-ch
		l.mu.Lock()
	}
	l.locks[ID] = make(chan struct{})
	l.mu.Unlock()
}

// Unlock unlocks on the id passed in.  Will panic if Lock with the
// given id wasn't called first.
func (l *stringLock) Unlock(ID string) {
	l.mu.Lock()
	ch, ok := l.locks[ID]
	if !ok {
		panic("stringLock: Unlock before Lock")
	}
	close(ch)
	delete(l.locks, ID)
	l.mu.Unlock()
}
