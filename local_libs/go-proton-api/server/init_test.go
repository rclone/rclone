package server

import "github.com/rclone/go-proton-api/server/backend"

func init() {
	backend.GenerateKey = backend.FastGenerateKey
}
