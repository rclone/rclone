//go:build ignore

// A simple auth proxy for testing purposes
//
// For S3 access key auth (no "pass" or "public_key" in the input) the
// access key ID and secret are read from the environment variable
// RCLONE_TEST_PROXY_AUTH_KEY as "accessKeyID,secretAccessKey" and any
// other access key ID is refused.
package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"
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

	_, havePass := in["pass"]
	_, havePublicKey := in["public_key"]
	if !havePass && !havePublicKey {
		accessKeyID, secret, ok := strings.Cut(os.Getenv("RCLONE_TEST_PROXY_AUTH_KEY"), ",")
		if !ok {
			log.Fatal("RCLONE_TEST_PROXY_AUTH_KEY not set")
		}
		if in["user"] != accessKeyID {
			log.Fatalf("unknown access key ID %q", in["user"])
		}
		out["_secret_access_key"] = secret
	}
	json.NewEncoder(os.Stdout).Encode(&out)
	if err != nil {
		log.Fatal(err)
	}
}
