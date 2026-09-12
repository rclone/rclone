package gadb

import (
	"os"
	"testing"
)

// TestFixupMode covers every Unix mode_t type bit class produced by the ADB
// SYNC protocol's wire format and verifies that the resulting os.FileMode
// reports the correct semantic via the standard library predicate methods.
//
// The bug this helper fixes: upstream gadb writes the wire mode bits
// directly into entry.Mode (an os.FileMode field). Go's os.FileMode puts
// type bits at the HIGH end (ModeDir = 1<<31, ModeSymlink = 1<<27, etc.)
// while the wire format uses the Unix mode_t low-bit layout (S_IFDIR =
// 0x4000, S_IFLNK = 0xa000, etc.). Without the translation this helper
// performs, every directory returned by SYNC LIST classifies as a regular
// file and breaks rclone lsd, recursive copy, and sync.
func TestFixupMode(t *testing.T) {
	tests := []struct {
		name        string
		raw         uint32
		wantPerm    os.FileMode
		isDir       bool
		isSymlink   bool
		isRegular   bool
		isDevice    bool
		isCharDev   bool
		isNamedPipe bool
		isSocket    bool
	}{
		{
			// Regular file 0644
			name:      "regular_0644",
			raw:       0x8000 | 0o644,
			wantPerm:  0o644,
			isRegular: true,
		},
		{
			// Directory 0755
			name:     "directory_0755",
			raw:      0x4000 | 0o755,
			wantPerm: 0o755,
			isDir:    true,
		},
		{
			// Symbolic link 0777
			name:      "symlink_0777",
			raw:       0xa000 | 0o777,
			wantPerm:  0o777,
			isSymlink: true,
		},
		{
			// Character device 0660
			name:      "char_device_0660",
			raw:       0x2000 | 0o660,
			wantPerm:  0o660,
			isDevice:  true,
			isCharDev: true,
		},
		{
			// Block device 0660
			name:     "block_device_0660",
			raw:      0x6000 | 0o660,
			wantPerm: 0o660,
			isDevice: true,
		},
		{
			// Named pipe (FIFO) 0644
			name:        "fifo_0644",
			raw:         0x1000 | 0o644,
			wantPerm:    0o644,
			isNamedPipe: true,
		},
		{
			// Unix domain socket 0666
			name:     "socket_0666",
			raw:      0xc000 | 0o666,
			wantPerm: 0o666,
			isSocket: true,
		},
		{
			// Real-world /sdcard entry: directory 0775 (the FUSE-backed
			// scoped-storage mount typically reports group-writable dirs)
			name:     "sdcard_dir_0775",
			raw:      0x4000 | 0o775,
			wantPerm: 0o775,
			isDir:    true,
		},
		{
			// Real-world /sdcard entry: regular file 0660 (FUSE-backed
			// /sdcard files are typically u+rw,g+rw,o-rwx)
			name:      "sdcard_file_0660",
			raw:       0x8000 | 0o660,
			wantPerm:  0o660,
			isRegular: true,
		},
		{
			// Permission bits zero, regular file
			name:      "regular_no_perms",
			raw:       0x8000,
			wantPerm:  0,
			isRegular: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode := fixupMode(tt.raw)

			gotPerm := mode.Perm()
			if gotPerm != tt.wantPerm {
				t.Errorf("perm = %o, want %o", gotPerm, tt.wantPerm)
			}

			if got := mode.IsDir(); got != tt.isDir {
				t.Errorf("IsDir = %v, want %v (mode = %v)", got, tt.isDir, mode)
			}
			if got := (mode & os.ModeSymlink) != 0; got != tt.isSymlink {
				t.Errorf("isSymlink = %v, want %v (mode = %v)", got, tt.isSymlink, mode)
			}
			if got := mode.IsRegular(); got != tt.isRegular {
				t.Errorf("IsRegular = %v, want %v (mode = %v)", got, tt.isRegular, mode)
			}
			if got := (mode & os.ModeDevice) != 0; got != tt.isDevice {
				t.Errorf("isDevice = %v, want %v (mode = %v)", got, tt.isDevice, mode)
			}
			if got := (mode & os.ModeCharDevice) != 0; got != tt.isCharDev {
				t.Errorf("isCharDevice = %v, want %v (mode = %v)", got, tt.isCharDev, mode)
			}
			if got := (mode & os.ModeNamedPipe) != 0; got != tt.isNamedPipe {
				t.Errorf("isNamedPipe = %v, want %v (mode = %v)", got, tt.isNamedPipe, mode)
			}
			if got := (mode & os.ModeSocket) != 0; got != tt.isSocket {
				t.Errorf("isSocket = %v, want %v (mode = %v)", got, tt.isSocket, mode)
			}
		})
	}
}

// TestFixupMode_StripsHighBits verifies that bits above the Unix type field
// (everything above 0xffff) are not propagated into os.FileMode. The wire
// format never sets these, but a defensive check guards against future
// protocol drift.
func TestFixupMode_StripsHighBits(t *testing.T) {
	// 0x10000 set, regular file, 0644 perms — the high bit must be ignored
	mode := fixupMode(0x10000 | 0x8000 | 0o644)
	if !mode.IsRegular() {
		t.Errorf("high bit propagated; mode = %v", mode)
	}
	if mode.Perm() != 0o644 {
		t.Errorf("perm = %o, want 0644", mode.Perm())
	}
}
