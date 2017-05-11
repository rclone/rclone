package examples

import (
	"github.com/tsenart/tb"
	"io"
	"log"
	"net"
	"time"
)

func main() {
	ln, err := net.Listen("tcp", ":6789")
	if err != nil {
		log.Fatal(err)
	}
	th := tb.NewThrottler(100 * time.Millisecond)

	echo := func(conn net.Conn) {
		defer conn.Close()

		host, port, err := net.SplitHostPort(conn.RemoteAddr().String())
		if err != nil {
			panic(err)
		}
		// Throttle to 10 connection per second from the same host
		// Handle non-conformity by dropping the connection
		if th.Halt(host, 1, 10) {
			log.Printf("Throttled %s", host)
			return
		}
		log.Printf("Echoing payload from %s:%s", host, port)
		io.Copy(conn, conn)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go echo(conn)
	}
}
