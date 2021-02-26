# librclone

This directory contains code to build rclone as a C library and the
shims for accessing rclone from C.

The shims are a thin wrapper over the rclone RPC.

Build a shared library like this:

    go build --buildmode=c-shared -o librclone.so github.com/rclone/rclone/librclone

Build a static library like this:

    go build --buildmode=c-archive -o librclone.a github.com/rclone/rclone/librclone

Both the above commands will also generate `librclone.h` which should
be `#include`d in `C` programs wishing to use the library.

The library will depend on `libdl` and `libpthread`.

## Documentation

For documentation see the Go documentation for:

- [RcloneInitialize](https://pkg.go.dev/github.com/rclone/rclone/librclone#RcloneInitialize)
- [RcloneFinalize](https://pkg.go.dev/github.com/rclone/rclone/librclone#RcloneFinalize)
- [RcloneRPC](https://pkg.go.dev/github.com/rclone/rclone/librclone#RcloneRPC)

## C Example

There is an example program `ctest.c` with Makefile in the `ctest` subdirectory
