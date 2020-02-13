package fuse

// Maximum file write size we are prepared to receive from the kernel.
//
// Linux 4.2.0 has been observed to cap this value at 128kB
// (FUSE_MAX_PAGES_PER_REQ=32, 4kB pages).
// From Linux 4.20, the cap has been increased to 1MiB
// (FUSE_MAX_PAGES_PER_REQ=256, 4kB pages).
const maxWrite = 1 * 1024 * 1024 // 1 MiB
