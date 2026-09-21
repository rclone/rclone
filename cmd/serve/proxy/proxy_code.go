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

	// Note each run in a file if asked to
	if runLog := os.Getenv("RCLONE_TEST_PROXY_RUN_LOG"); runLog != "" {
		f, err := os.OpenFile(runLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatal(err)
		}
		_, _ = f.WriteString(in["user"] + "\n")
		_ = f.Close()
	}

	// Write the output
	var out = map[string]string{}
	for k, v := range in {
		switch k {
		case "user":
			v += "-test"
		case "error":
			log.Fatal(v)
		case "pass":
			// Simulate a key-only policy which refuses every password
			if os.Getenv("RCLONE_TEST_PROXY_KEY_ONLY") != "" {
				log.Fatalf("password refused for %q", in["user"])
			}
		case "public_key":
			if os.Getenv("RCLONE_TEST_PROXY_REFUSE_KEYS") != "" {
				log.Fatalf("public key refused for %q", in["user"])
			}
		}
		out[k] = v
	}
	if out["type"] == "" {
		out["type"] = "local"
	}
	if out["_root"] == "" {
		out["_root"] = ""
	}
	// A user or access key ID can be revoked
	if revoked := os.Getenv("RCLONE_TEST_PROXY_REVOKED"); revoked != "" && in["user"] == revoked {
		log.Fatalf("user %q revoked", in["user"])
	}
	// S3 access key auth has neither pass nor public_key and needs
	// the secret returned, unless the user asks for it to be omitted
	// or empty. The secret's suffix can be changed to simulate a
	// rotation.
	_, havePass := in["pass"]
	_, havePublicKey := in["public_key"]
	switch {
	case havePass || havePublicKey || in["user"] == "nosecret":
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
