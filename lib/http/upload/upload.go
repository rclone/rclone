package upload

import (
	"context"
	"github.com/rclone/rclone/fs"
	"net/http"
)

type MetaData map[string]string
type CreateCallback func(ctx context.Context, Size int64, meta MetaData) fs.Object
type GetCallback func(ctx context.Context) fs.Object
type Handler func(*http.Request, CreateCallback, GetCallback) http.Handler

// Handlers store all upload handlers:
// - tus.HandleUpload
// - TODO multipart/form upload
var Handlers = []Handler{}

func HandleUpload(r *http.Request, create CreateCallback, get GetCallback) http.Handler {
	for _, handler := range Handlers {
		if h := handler(r, create, get); h != nil {
			return h
		}
	}
	return nil
}
