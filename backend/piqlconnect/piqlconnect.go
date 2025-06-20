// Package piqlconnect provides an interface to piqlConnect
package piqlconnect

import (
	"context"
	"fmt"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "piqlconnect",
		Description: "piqlConnect",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			return nil, nil
		},
	})
	fmt.Println("HELLO")
}

func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	return nil, fmt.Errorf("Boop")
}
