// +build go1.8

package httpserver

import (
	"context"
	"net/http"
	"time"

	"github.com/ncw/rclone/fs"
)

func stopServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		fs.Errorf("httpserver", "error during stop: %v", err)
	}
}
