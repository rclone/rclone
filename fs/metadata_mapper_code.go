//go:build ignore

// A simple metadata mapper for testing purposes
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

func check[T comparable](in map[string]any, key string, want T) {
	value, ok := in[key]
	if !ok {
		fmt.Fprintf(os.Stderr, "%s key not found\n", key)
		os.Exit(1)
	}
	if value.(T) != want {
		fmt.Fprintf(os.Stderr, "%s wrong - expecting %s but got %s\n", key, want, value)
		os.Exit(1)
	}
}

func main() {
	// Read the input
	var in map[string]any
	err := json.NewDecoder(os.Stdin).Decode(&in)
	if err != nil {
		log.Fatal(err)
	}

	// Check the input
	metadata, ok := in["Metadata"]
	if !ok {
		fmt.Fprintf(os.Stderr, "Metadata key not found\n")
		os.Exit(1)
	}
	check(in, "Size", 5.0)
	check(in, "SrcFs", "memory:")
	check(in, "SrcFsType", "object.memoryFs")
	check(in, "DstFs", "dstFs:dstFsRoot")
	check(in, "DstFsType", "mockfs")
	check(in, "Remote", "file.txt")
	check(in, "MimeType", "text/plain; charset=utf-8")
	check(in, "ModTime", "2001-02-03T04:05:06.000000007Z")
	check(in, "IsDir", false)
	//check(in, "ID", "Potato")

	// Map the metadata
	metadataOut := map[string]string{}
	var out = map[string]any{
		"Metadata": metadataOut,
	}
	for k, v := range metadata.(map[string]any) {
		switch k {
		case "error":
			fmt.Fprintf(os.Stderr, "Error: %s\n", v)
			os.Exit(1)
		case "key1":
			v = "two " + v.(string)
		case "key3":
			continue
		}
		metadataOut[k] = v.(string)
	}
	metadataOut["key0"] = "cabbage"

	// Write the output
	json.NewEncoder(os.Stdout).Encode(&out)
	if err != nil {
		log.Fatal(err)
	}
}
