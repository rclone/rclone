//go:build ignore

// A test auth proxy that maps the supplied password to a backend root.
//
// Both roots require the same FTP username ("shared") but different
// passwords, so it exercises two credentials that share a username but
// resolve to different backends. The roots are passed in the environment.
package main

import (
	"encoding/json"
	"log"
	"os"
)

func main() {
	var in map[string]string
	if err := json.NewDecoder(os.Stdin).Decode(&in); err != nil {
		log.Fatal(err)
	}

	roots := map[string]string{
		"attacker-token": os.Getenv("RCLONE_TEST_ATTACKER_ROOT"),
		"victim-token":   os.Getenv("RCLONE_TEST_VICTIM_ROOT"),
	}
	root, ok := roots[in["pass"]]
	if in["user"] != "shared" || !ok {
		os.Exit(1)
	}

	out := map[string]string{
		"type":  "local",
		"_root": root,
	}
	if err := json.NewEncoder(os.Stdout).Encode(&out); err != nil {
		log.Fatal(err)
	}
}
