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
	"maps"
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
	var out = map[string]any{
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
		if out["_secret_access_key"] == nil || out["_secret_access_key"] == "" {
			log.Fatalf("unknown access key ID %q", in["user"])
		}
	}

	if ovRaw := os.Getenv("RCLONE_TEST_PROXY_USER_OVERRIDES"); ovRaw != "" {
		var overrides map[string]map[string]any
		if err := json.Unmarshal([]byte(ovRaw), &overrides); err != nil {
			log.Fatalf("RCLONE_TEST_PROXY_USER_OVERRIDES invalid JSON: %v", err)
		}
		if extra, ok := overrides[in["user"]]; ok {
			maps.Copy(out, extra)
		}
	}

	if err := json.NewEncoder(os.Stdout).Encode(&out); err != nil {
		log.Fatal(err)
	}
}
