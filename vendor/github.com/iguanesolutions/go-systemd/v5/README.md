# go-systemd

[![Go Report Card](https://goreportcard.com/badge/github.com/iguanesolutions/go-systemd)](https://goreportcard.com/report/github.com/iguanesolutions/go-systemd) [![PkgGoDev](https://pkg.go.dev/badge/github.com/iguanesolutions/go-systemd/v5)](https://pkg.go.dev/github.com/iguanesolutions/go-systemd/v5)

Easily communicate with systemd when run as daemon within a service unit.

## Notifier

[![PkgGoDev](https://pkg.go.dev/badge/github.com/iguanesolutions/go-systemd/v5/notify)](https://pkg.go.dev/github.com/iguanesolutions/go-systemd/v5/notify)

With notifier you can notify to systemd that your program is starting, stopping, reloading...

For example, if your daemon needs some time for initializing its controllers before really being considered as ready, you can specify to systemd that this is a "notify" service and send it a notification when ready.

It is safe to use it even if systemd notify support is disabled (noop call).

```systemdunit
[Service]
Type=notify
```

```go
import (
    sysdnotify "github.com/iguanesolutions/go-systemd/v5/notify"
)

// Init http server
server := &http.Server{
    Addr:    "host:port",
    Handler: myHTTPHandler,
}

/*
    Do some more inits
*/

// Notify ready to systemd
if err = sysdnotify.Ready(); err != nil {
    log.Printf("failed to notify ready to systemd: %v\n", err)
}

// Start the server
if err = server.ListenAndServe(); err != nil {
    log.Printf("failed to start http server: %v\n", err)
}
```

When stopping, you can notify systemd that you have indeed received the SIGTERM and you have launched the stop procedure

```go
import (
    sysdnotify "github.com/iguanesolutions/go-systemd/v5/notify"
)

// Notify to systemd that we are stopping
var err error
if err = sysdnotify.Stopping(); err != nil {
    log.Printf("failed to notify stopping to systemd: %v\n", err)
}

/*
    Stop others things
*/

// Stop the server (with timeout)
ctx, cancelCtx := context.WithTimeout(context.Background(), 5*time.Second)
defer cancelCtx()
if err = server.Shutdown(ctx); err != nil {
    log.Printf("failed to shutdown http server: %v\n", err)
}
```

You can also notify status to systemd

```go
import (
    sysdnotify "github.com/iguanesolutions/go-systemd/v5/notify"
)

if err := sysdnotify.Status(fmt.Sprintf("There is currently %d active connections", activeConns)); err != nil {
    log.Printf("failed to notify status to systemd: %v\n", err)
}

```

systemctl status output example:

```systemctlstatus
user@host:~$ systemctl status superapp.service
● superapp.service - superapp
   Loaded: loaded (/lib/systemd/system/superapp.service; enabled)
   Active: active (running) since Mon 2018-06-25 08:54:35 UTC; 3 days ago
 Main PID: 2604 (superapp)
   Status: "There is currently 1506 active connections"
   ...
```

### Watchdog

[![PkgGoDev](https://pkg.go.dev/badge/github.com/iguanesolutions/go-systemd/v5/notify/watchdog)](https://pkg.go.dev/github.com/iguanesolutions/go-systemd/v5/notify/watchdog)

```systemdunit
[Service]
Type=notify
WatchdogSec=30s
```

```go
import (
    sysdwatchdog "github.com/iguanesolutions/go-systemd/v5/notify/watchdog"
)

// Init systemd watchdog, same as the notifier, it can be nil if your os does not support it
watchdog, err := sysdwatchdog.New()
if err != nil {
    log.Printf("failed to initialize systemd watchdog controller: %v\n", err)
}

if watchdog != nil {
    // Then start a watcher worker
    go func() {
        ticker := watchdog.NewTicker()
        defer ticker.Stop()
        for {
            select {
            // Ticker chan
            case <-ticker.C:
                // Check if something wrong, if not send heartbeat
                if allGood {
                    if err = watchdog.SendHeartbeat(); err != nil {
                        log.Printf("failed to send systemd watchdog heartbeat: %v\n", err)
                    }
                }
            // Some stop signal chan
            case <-stopSig:
                return
            }
        }
    }()
}
```

## Resolved

[![PkgGoDev](https://pkg.go.dev/badge/github.com/iguanesolutions/go-systemd/resolved/resolved)](https://pkg.go.dev/github.com/iguanesolutions/go-systemd/v5/resolved)

This package is still under development and very experimental, do not use it in production.
We started this package in order to go deep into the DNS world. So we are opened to any suggestions/contributions on this.
DNS is not trivial at all so there can be some stuff that are not rfc compliant (like sorting addresses etc...).

The resolved package features:
 * Pure Go implementation of `org.freedesktop.resolve1` dbus interface
 * Resolver type (which uses the underlying dbus interface) that tries to implement the same methods as `net.Resolver` from Go standard library
 * Unit tests (make sure Go resolver and systemd-resolved query the same dns server)

### Dbus

The following example shows how to use the resolve1 dbus connection to resolve an host:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"syscall"

	"github.com/iguanesolutions/go-systemd/v5/resolved"
)

func main() {
	c, err := resolved.NewConn()
	if err != nil {
		log.Fatal("ERROR: ", err)
	}
	ctx := context.Background()
	addrs, canonical, flags, err := c.ResolveHostname(ctx, 0, "google.com", syscall.AF_UNSPEC, 0)
	if err != nil {
		log.Println("ERROR: ", err)
	} else {
		fmt.Println("Addresses: ", addrs)
		fmt.Println("Canonical: ", canonical)
		fmt.Println("OutputFlags: ", flags)
	}
	err = c.Close()
	if err != nil {
		log.Println("ERROR: ", err)
	}
}
```

Output:

```output
Addresses:  [{
        IfIndex: 2,
        Family:  2,
        IP:      142.250.74.238,
} {
        IfIndex: 2,
        Family:  10,
        IP:      2a00:1450:4007:80b::200e,
}]
Canonical:  google.com
Flags:  1
```

### Resolver

The following example shows how to use the resolved Resolver to resolve an host:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/iguanesolutions/go-systemd/v5/resolved"
)

func main() {
	r, err := resolved.NewResolver()
	if err != nil {
		log.Fatal("ERROR: ", err)
	}
	ctx := context.Background()
	addrs, err := r.LookupHost(ctx, "google.com")
	if err != nil {
		log.Println("ERROR: ", err)
	} else {
		fmt.Println("Addresses: ", addrs)
	}
	err = r.Close()
	if err != nil {
		log.Println("ERROR: ", err)
	}
}
```

Output:

```output
Addresses:  [2a00:1450:4007:80b::200e 142.250.74.238]
```

### HTTP Client

The following example shows how to use the systemd-resolved Resolver with the Go http client from the standard library:

```go
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/iguanesolutions/go-systemd/v5/resolved"
)

func main() {
	r, err := resolved.NewResolver()
	if err != nil {
		log.Fatal("ERROR: ", err)
	}
	// if you want to make a custom http client using systemd-resolved as resolver
	httpCli := &http.Client{
		Transport: &http.Transport{
			DialContext: r.DialContext,
		},
	}
	// or if you don't have an http client you can call HTTPClient method on resolver
	// it comes with some nice default values.
	httpCli = r.HTTPClient()
	resp, err := httpCli.Get("https://google.com")
	if err != nil {
		log.Println("ERROR: ", err)
	} else {
		fmt.Println("Status: ", resp.Status)
		err = resp.Body.Close()
		if err != nil {
			log.Println("ERROR: ", err)
		}
	}
	err = r.Close()
	if err != nil {
		log.Println("ERROR: ", err)
	}
}
```

Output:

```output
Status:  200 OK
```
