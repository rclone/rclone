package utility

import (
	"log"
)

func SetupLog() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
