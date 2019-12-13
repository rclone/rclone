// Package contextkeys reveals contextkeys used by fs/sync
package contextkeys

type syncCopyMoveContextKey int

// import SyncCoveMoveSrcNameKey and SyncCopyMoveDstNameKey
const (
	SyncCopyMoveSrcNameKey syncCopyMoveContextKey = iota
	SyncCopyMoveDstNameKey
)
