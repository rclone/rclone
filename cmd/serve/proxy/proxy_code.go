//go:build ignore

// A simple auth proxy for testing purposes
package main

import (
	"encoding/json"
	"log"
	"os"
)

func main() {
	// Read the input
	var in map[string]string
	err := json.NewDecoder(os.Stdin).Decode(&in)
	if err != nil {
		log.Fatal(err)
	}

	// Write the output
	var out = map[string]string{}
	for k, v := range in {
		switch k {
		case "user":
			v += "-test"
		case "error":
			log.Fatal(v)
		}
		out[k] = v
	}
	if out["type"] == "" {
		out["type"] = "local"
	}
	if out["_root"] == "" {
		out["_root"] = ""
	}
	json.NewEncoder(os.Stdout).Encode(&out)
	if err != nil {
		log.Fatal(err)
	}
}
