package tb

import (
	"io"
	"log"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestThrottler_Bucket(t *testing.T) {
	t.Parallel()

	th := NewThrottler(0)
	defer th.Close()

	b := th.Bucket("a", 1000)

	ex := [...]int64{100, 100, 1000, 900, 1, 0}
	for i := 0; i < len(ex)-1; i += 2 {
		if got, want := b.Take(ex[i]), ex[i+1]; got != want {
			t.Errorf("Want: %d, Got: %d", want, got)
		}
	}

	for i := 0; i < len(ex)-1; i += 2 {
		if got, want := b.Put(ex[i]), ex[i+1]; got != want {
			t.Errorf("Want: %d, Got: %d", want, got)
		}
	}
}

func TestThrottler_Halt(t *testing.T) {
	t.Parallel()

	th := NewThrottler(0)
	defer th.Close()

	if th.Halt("a", 1000, 1000) {
		t.Fatal("Didn't expect halt")
	}

	if !th.Halt("a", 1, 1000) {
		t.Fatal("Expected halt")
	}

	if th.Halt("b", 1000, 1000) {
		t.Fatal("Didn't expect halt")
	}
}

func TestThrottler_Wait(t *testing.T) {
	t.Parallel()

	th := NewThrottler(1 * time.Millisecond)
	defer th.Close()

	if wait := th.Wait("a", 1000, 1000); wait > 0 {
		t.Fatal("Didn't expect wait")
	}

	if wait := th.Wait("a", 2000, 1000); int(wait.Seconds()) != 2 {
		t.Fatalf("Expected wait of 2s. Got: %s", wait)
	}
}

func BenchmarkThrottler_Bucket(b *testing.B) {
	keys := make([]string, 10000)
	for i := 0; i < len(keys); i++ {
		keys[i] = strconv.Itoa(i)
	}

	th := NewThrottler(1 * time.Millisecond)
	defer th.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		th.Bucket(keys[i%(len(keys)-1)], 1000)
	}
}

func ExampleThrottler() {
	ln, err := net.Listen("tcp", ":6789")
	if err != nil {
		log.Fatal(err)
	}
	th := NewThrottler(100 * time.Millisecond)
	defer th.Close()

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
