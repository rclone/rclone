//go:build ignore

// A simple auth proxy for testing purposes
//
// For S3 access key auth (no "pass" or "public_key" in the input) the
// access key IDs and secrets are read from the environment variable
// RCLONE_TEST_PROXY_AUTH_KEY as "accessKeyID,secretAccessKey" pairs
// separated by ";" and any other access key ID is refused.
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
		for pair := range strings.SplitSeq(os.Getenv("RCLONE_TEST_PROXY_AUTH_KEY"), ";") {
			if accessKeyID, secret, ok := strings.Cut(pair, ","); ok && in["user"] == accessKeyID {
				out["_secret_access_key"] = secret
			}
		}
		if out["_secret_access_key"] == "" {
			log.Fatalf("unknown access key ID %q", in["user"])
		}
	}
	json.NewEncoder(os.Stdout).Encode(&out)
	if err != nil {
		log.Fatal(err)
	}
}
