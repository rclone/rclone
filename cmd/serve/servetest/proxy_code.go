//go:build ignore

// A simple auth proxy for testing purposes
package main

import (
	"encoding/json"
	"log"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Syntax: %s <root>", os.Args[0])
	}
	root := os.Args[1]

	// Read the input
	var in map[string]string
	err := json.NewDecoder(os.Stdin).Decode(&in)
	if err != nil {
		log.Fatal(err)
	}

	// Write the output
	var out = map[string]string{
		"type":     "local",
		"_root":    root,
		"_obscure": "pass",
	}
	json.NewEncoder(os.Stdout).Encode(&out)
	if err != nil {
		log.Fatal(err)
	}
}
