# librclone

This directory contains code to build rclone as a C library and the
shims for accessing rclone from C and other languages.

**Note** for the moment, the interfaces defined here are experimental
and may change in the future. Eventually they will stabilise and this
notice will be removed.

## C

The shims are a thin wrapper over the rclone RPC.

The implementation is based on cgo; to build it you need Go and a GCC compatible
C compiler (GCC or Clang). On Windows you can use the MinGW ports, e.g. by installing
in a [MSYS2](https://www.msys2.org) distribution (you may now install GCC in the newer
and recommended UCRT64 subsystem, however there were compatibility issues with previous
versions of cgo where, if not force rebuild with go build option `-a` helped, you had
to resort to the classic MINGW64 subsystem).

Build a shared library like this (change from .so to .dll on Windows):

    go build --buildmode=c-shared -o librclone.so github.com/rclone/rclone/librclone

Build a static library like this (change from .a to .lib on Windows):

    go build --buildmode=c-archive -o librclone.a github.com/rclone/rclone/librclone

Both the above commands will also generate `librclone.h` which should
be `#include`d in `C` programs wishing to use the library (with some
[exceptions](#include-file)).

The library will depend on `libdl` and `libpthread` on Linux/macOS, unless
linking with a C standard library where their functionality is integrated,
which is the case for glibc version 2.34 and newer.

You may add arguments `-ldflags -s` to make the library file smaller. This will
omit symbol table and debug information, reducing size by about 25% on Linux and
50% on Windows.

Note that on macOS and Windows the mount functions will not be available unless
you add additional argument `-tags cmount`. On Windows this also requires you to
first install the third party utility [WinFsp](http://www.secfs.net/winfsp/),
with the "Developer" feature selected, and to set environment variable CPATH
pointing to the fuse include directory within the WinFsp installation
(typically `C:\Program Files (x86)\WinFsp\inc\fuse`). See also the
[mount](/commands/rclone_mount/#installing-on-windows) documentation.

On Windows, when you build a shared library, you can embed version information
as binary resource. To do that you need to run the following command **before**
the build command.

```
go run bin/resource_windows.go -binary librclone.dll -dir librclone
```

### Documentation

For documentation see the Go documentation for:

- [RcloneInitialize](https://pkg.go.dev/github.com/rclone/rclone/librclone#RcloneInitialize)
- [RcloneFinalize](https://pkg.go.dev/github.com/rclone/rclone/librclone#RcloneFinalize)
- [RcloneRPC](https://pkg.go.dev/github.com/rclone/rclone/librclone#RcloneRPC)
- [RcloneFreeString](https://pkg.go.dev/github.com/rclone/rclone/librclone#RcloneFreeString)

### Linux C example

There is an example program `ctest.c`, with `Makefile`, in the `ctest`
subdirectory. It can be built on Linux/macOS, but not Windows without
changes - as described next.

### Windows C/C++ guidelines

The official [C example](#linux-c-example) is targeting Linux/macOS, and will
not work on Windows. It is very possible to use `librclone` from a C/C++
application on Windows, but there are some pitfalls that you can avoid by
following these guidelines:
- Build `librclone` as shared library, and use run-time dynamic linking (see [linking](#linking)).
- Do not try to unload the library with `FreeLibrary` (see [unloading](#unloading)).
- Deallocate returned strings with API function `RcloneFreeString` (see [memory management](#memory-management)).
- Define struct `RcloneRPCResult`, instead of including `librclone.h` (see [include file](#include-file)).
- Use UTF-8 encoded strings (see [encoding](#encoding)).
- Properly escape JSON strings, beware of the native path separator (see [escaping](#escaping)).

#### Linking

Use of different compilers, compiler versions, build configuration, and
dependency on different C runtime libraries for a library and the application
that references it, may easily break compatibility. When building the librclone
library with MinGW GCC compiler (via go build command), if you link it into an
application built with Visual C++ for example, there will be more than enough
differences to cause problems.

Linking with static library requires most compatibility, and is less likely to
work. Linking with shared library is therefore recommended. The library exposes
a plain C interface, and by using run-time dynamic linking (by using Windows API
functions `LoadLibrary` and `GetProcAddress`), you can make a boundary that
ensures compatibility (and in any case, you will not have an import library).
The only remaining concern is then memory allocations; you should make sure
memory is deallocated in the same library where it was allocated, as explained
[below](#memory-management).

#### Unloading

Do not try to unload the library with `FreeLibrary`, when using run-time dynamic
linking. The library includes Go-specific runtime components, with garbage
collection and other background threads, which do not handle unloading. Trying
to call `FreeLibrary` will crash the application. I.e. after you have loaded
`librclone.dll` into your application it must stay loaded until your application
exits.

#### Memory management

The output string returned from `RcloneRPC` is allocated within the `librclone`
library, and caller is responsible for freeing the memory. Due to C runtime
library differences, as mentioned [above](#linking), it is not recommended to do
this by calling `free` from the consuming application. You should instead use
the API function `RcloneFreeString`, which will call `free` from within the
`librclone` library, using the same runtime that allocated it in the first
place.

#### Include file

Do not include `librclone.h`. It contains some plain C, golang/cgo and GCC
specific type definitions that will not compile with all other compilers
without adjustments, where Visual C++ is one notable example. When using
run-time dynamic linking, you have no use of the extern declared functions
either.

The interface of librclone is so simple, that all you need is to define the
small struct `RcloneRPCResult`, from [librclone.go](librclone.go):

```C++
struct RcloneRPCResult {
    char* Output;
    int	Status;
};
```

#### Encoding

The API uses plain C strings (type `char*`, called "narrow" strings), and rclone
assumes content is UTF-8 encoded. On Linux systems this normally matches the
standard string representation, and no special considerations must be made. On
Windows it is more complex.

On Windows, narrow strings are traditionally used with native non-Unicode
encoding, the so-called ANSI code page, while Unicode strings are instead
represented with the alternative `wchar_t*` type, called "wide" strings, and
encoded as UTF-16. This means, to correctly handle characters that are encoded
differently in UTF-8, you will need to perform conversion at some level:
Conversion between UTF-8 encoded narrow strings used by rclone, and either ANSI
encoded narrow strings or wide UTF-16 encoded strings used in runtime function,
Windows API, third party APIs, etc.

#### Escaping

The RPC method takes a string containing JSON. In addition to the normal
escaping of strings constants in your C/C++ source code, the JSON needs its
own escaping. This is not a Windows-specific issue, but there is the
additional challenge that native filesystem path separator is the same as
the escape character, and you may end up with strings like this:

```C++
const char* input = "{"
"\"fs\": \"C:\\\\Temp\","
"\"remote\": \"sub/folder\","
"\"opt\": \"{\\\"showHash\\\": true}\""
"}";
```

With C++11 you can use raw string literals to avoid the C++ escaping of string
constants, leaving escaping only necessary for the contained JSON.

## Example in golang

Here is a go example to help you move files : 

```go
func main() {
  librclone.Initialize()
    syncRequest: = syncRequest {
    SrcFs: "<absolute_path>",
    DstFs: ":s3,env_auth=false,access_key_id=<access>,secret_access_key=<secret>,endpoint='<endpoint>':<bucket>",
    }

    syncRequestJSON, err: = json.Marshal(syncRequest)
    if err != nil {
    fmt.Println(err)
    }
		
    out, status: = librclone.RPC("sync/copy", string(syncRequestJSON))
    fmt.Println("Got status : %d and output %q", status, out)
}

```

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

## Rust

Rust bindings are available in the `librclone` crate: https://crates.io/crates/librclone

## PHP

The `php` subdirectory contains how to use the C library librclone in php through foreign 
function interface (FFI).

Useful docs:
- [PHP / FFI](https://www.php.net/manual/en/book.ffi.php)

## TODO

- Async jobs must currently be cancelled manually at the moment - RcloneFinalize doesn't do it.
- This will use the rclone config system and rclone logging system.
- Need examples showing how to configure things,

