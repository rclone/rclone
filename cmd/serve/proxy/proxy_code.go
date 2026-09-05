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
	// S3 access key auth has neither pass nor public_key and needs
	// the secret returned, unless the user asks for it to be omitted
	// or empty. The secret's suffix can be changed to simulate a
	// rotation and an access key ID can be revoked.
	_, havePass := in["pass"]
	_, havePublicKey := in["public_key"]
	switch {
	case havePass || havePublicKey || in["user"] == "nosecret":
	case in["user"] == os.Getenv("RCLONE_TEST_PROXY_REVOKED"):
		log.Fatalf("access key ID %q revoked", in["user"])
	case in["user"] == "emptysecret":
		out["_secret_access_key"] = ""
	default:
		suffix := os.Getenv("RCLONE_TEST_PROXY_SECRET_SUFFIX")
		if suffix == "" {
			suffix = "-secret"
		}
		out["_secret_access_key"] = in["user"] + suffix
	}
	json.NewEncoder(os.Stdout).Encode(&out)
	if err != nil {
		log.Fatal(err)
	}
}
