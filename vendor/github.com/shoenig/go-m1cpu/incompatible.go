//go:build !darwin || !arm64 || !cgo

package m1cpu

// IsAppleSilicon return false on this platform.
func IsAppleSilicon() bool {
	return false
}

// PCoreHZ requires darwin/arm64
func PCoreHz() uint64 {
	panic("m1cpu: not a darwin/arm64 system")
}

// ECoreHZ requires darwin/arm64
func ECoreHz() uint64 {
	panic("m1cpu: not a darwin/arm64 system")
}

// PCoreGHz requires darwin/arm64
func PCoreGHz() float64 {
	panic("m1cpu: not a darwin/arm64 system")
}

// ECoreGHz requires darwin/arm64
func ECoreGHz() float64 {
	panic("m1cpu: not a darwin/arm64 system")
}

// PCoreCount requires darwin/arm64
func PCoreCount() int {
	panic("m1cpu: not a darwin/arm64 system")
}

// ECoreCount requires darwin/arm64
func ECoreCount() int {
	panic("m1cpu: not a darwin/arm64 system")
}

// PCoreCacheSize requires darwin/arm64
func PCoreCache() (int, int, int) {
	panic("m1cpu: not a darwin/arm64 system")
}

// ECoreCacheSize requires darwin/arm64
func ECoreCache() (int, int, int) {
	panic("m1cpu: not a darwin/arm64 system")
}

// ModelName requires darwin/arm64
func ModelName() string {
	panic("m1cpu: not a darwin/arm64 system")
}
