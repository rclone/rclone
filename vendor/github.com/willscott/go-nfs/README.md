Golang Network File Server
===

NFSv3 protocol implementation in pure Golang.

Current Status:
* Minimally tested
* Mounts, read-only and read-write support

Usage
===

The most interesting demo is currently in `example/osview`. 

Start the server
`go run ./example/osview .`.

The local folder at `.` will be the initial view in the mount. mutations to metadata or contents
will be stored purely in memory and not written back to the OS. When run, this
demo will print the port it is listening on.

The mount can be accessed using a command similar to 
`mount -o port=<n>,mountport=<n> -t nfs localhost:/mount <mountpoint>` (For Mac users)

or

`mount -o port=<n>,mountport=<n>,nfsvers=3,noacl,tcp -t nfs localhost:/mount <mountpoint>` (For Linux users)

API
===

The NFS server runs on a `net.Listener` to export a file system to NFS clients.
Usage is structured similarly to many other golang network servers.

```golang
package main

import (
	"fmt"
	"log"
	"net"

	"github.com/go-git/go-billy/v5/memfs"
	nfs "github.com/willscott/go-nfs"
	nfshelper "github.com/willscott/go-nfs/helpers"
)

func main() {
	listener, err := net.Listen("tcp", ":0")
	panicOnErr(err, "starting TCP listener")
	fmt.Printf("Server running at %s\n", listener.Addr())
	mem := memfs.New()
	f, err := mem.Create("hello.txt")
	panicOnErr(err, "creating file")
	_, err = f.Write([]byte("hello world"))
	panicOnErr(err, "writing data")
	f.Close()
	handler := nfshelper.NewNullAuthHandler(mem)
	cacheHelper := nfshelper.NewCachingHandler(handler, 1)
	panicOnErr(nfs.Serve(listener, cacheHelper), "serving nfs")
}

func panicOnErr(err error, desc ...interface{}) {
	if err == nil {
		return
	}
	log.Println(desc...)
	log.Panicln(err)
}
```

Notes
---

* Ports are typically determined through portmap. The need for running portmap 
(which is the only part that needs a privileged listening port) can be avoided
through specific mount options. e.g. 
`mount -o port=n,mountport=n -t nfs host:/mount /localmount`

* This server currently uses [billy](https://github.com/go-git/go-billy/) to
provide a file system abstraction layer. There are some edges of the NFS protocol
which do not translate to this abstraction.
  * NFS expects access to an `inode` or equivalent unique identifier to reference
  files in a file system. These are considered opaque identifiers here, which
  means they will not work as expected in cases of hard linking.
  * The billy abstraction layer does not extend to exposing `uid` and `gid`
  ownership of files. If ownership is important to your file system, you
  will need to ensure that the `os.FileInfo` meets additional constraints.
  In particular, the `Sys()` escape hatch is queried by this library, and
  if your file system populates a [`syscall.Stat_t`](https://golang.org/pkg/syscall/#Stat_t)
  concrete struct, the ownership specified in that object will be used.

* Relevant RFCS:
[5531 - RPC protocol](https://tools.ietf.org/html/rfc5531),
[1813 - NFSv3](https://tools.ietf.org/html/rfc1813),
[1094 - NFS](https://tools.ietf.org/html/rfc1094)
