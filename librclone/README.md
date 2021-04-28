# librclone

This directory contains code to build rclone as a C library and the
shims for accessing rclone from C and other languages.

**Note** for the moment, the interfaces defined here are experimental
and may change in the future. Eventually they will stabilse and this
notice will be removed.

## C

The shims are a thin wrapper over the rclone RPC.

Build a shared library like this:

    go build --buildmode=c-shared -o librclone.so github.com/rclone/rclone/librclone

Build a static library like this:

    go build --buildmode=c-archive -o librclone.a github.com/rclone/rclone/librclone

Both the above commands will also generate `librclone.h` which should
be `#include`d in `C` programs wishing to use the library.

The library will depend on `libdl` and `libpthread`.

### Documentation

For documentation see the Go documentation for:

- [RcloneInitialize](https://pkg.go.dev/github.com/rclone/rclone/librclone#RcloneInitialize)
- [RcloneFinalize](https://pkg.go.dev/github.com/rclone/rclone/librclone#RcloneFinalize)
- [RcloneRPC](https://pkg.go.dev/github.com/rclone/rclone/librclone#RcloneRPC)

### C Example

There is an example program `ctest.c` with `Makefile` in the `ctest` subdirectory.

## gomobile

The `gomobile` subdirectory contains the equivalent of the C binding but
suitable for using with [gomobile](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile)
using something like this.

    gomobile bind -v -target=android -javapkg=org.rclone github.com/rclone/rclone/librclone/gomobile

The command generates an Android library (`aar`) that can be imported
into an Android application project. Librclone will be contained
within `libgojni.so` and loaded automatically.

```java
// imports
import org.rclone.gomobile.Gomobile;
import org.rclone.gomobile.RcloneRPCResult;

// initialize rclone
Gomobile.rcloneInitialize();

// call RC method and log response.
RcloneRPCResult response = Gomobile.rcloneRPC("core/version", "{}");
Log.i("rclone", "response status: " + response.getStatus());
Log.i("rclone", "output: " + response.getOutput());

// Clean up when finished.
Gomobile.rcloneFinalize();
```

This is a low level interface - serialization, job management etc must
be built on top of it.

iOS has not been tested (but should probably work).

Further docs:

- [gomobile main website](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile)
- [gomobile wiki](https://github.com/golang/go/wiki/Mobile)
- [go issue #16876](https://github.com/golang/go/issues/16876) where the feature was added
- [gomobile design doc](https://docs.google.com/document/d/1y9hStonl9wpj-5VM-xWrSTuEJFUAxGOXOhxvAs7GZHE/edit) for extra details not in the docs.

## python

The `python` subdirectory contains a simple Python wrapper for the C
API using rclone linked as a shared library with `ctypes`.

You are welcome to use this directly.

This needs expanding and submitting to pypi...

## TODO

- Async jobs must currently be cancelled manually at the moment - RcloneFinalize doesn't do it.
- This will use the rclone config system and rclone logging system.
- Need examples showing how to configure things,

