package log

import (
	"log"
)

// Deprecated: Logging shouldn't include control flow.
var (
	Panicf = log.Panicf
	Fatalf = log.Fatalf
	Fatal  = log.Fatal
)
