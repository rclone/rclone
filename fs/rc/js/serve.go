//go:build none

package main

import (
	"fmt"
	"log"
	"mime"
	"net/http"
)

func main() {
	mime.AddExtensionType(".wasm", "application/wasm")
	mime.AddExtensionType(".js", "application/javascript")
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	fmt.Printf("Serving on http://localhost:3000/\n")
	log.Fatal(http.ListenAndServe(":3000", mux))
}
