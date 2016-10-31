package fuse

// Maximum file write size we are prepared to receive from the kernel.
//
// Linux 4.2.0 has been observed to cap this value at 128kB
// (FUSE_MAX_PAGES_PER_REQ=32, 4kB pages).
const maxWrite = 128 * 1024
