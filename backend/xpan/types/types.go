package types

// ContextKey the key type used by fs operation context
type ContextKey string

const (
	// ContextKeyModTime hold file modtime
	ContextKeyModTime = ContextKey("rclone_ck_modtime")
)
