package tus

import (
	up "github.com/rclone/rclone/lib/http/upload"
	tus "github.com/tus/tusd/pkg/handler"
	"net/http"
)

func init() {
	up.Handlers = append(up.Handlers, HandleUpload)
}

func HandleUpload(r *http.Request, create up.CreateCallback, get up.GetCallback) http.Handler {
	if v := r.Header.Get("Tus-Resumable"); v != "" {
		method := r.Method
		if m := r.Header.Get("X-HTTP-Method-Override"); m != "" {
			method = m
		}

		switch method { // Ensure method is supported
		case "POST", "HEAD", "PATCH", "GET", "OPTIONS":
			break
		default:
			return nil
		}

		// Tus upload request confirmed

		// TODO explore not having to regenerate the composer or handler on every request
		var storeComposer = tus.NewStoreComposer()
		var store = fsStore{create, get}
		store.UseIn(storeComposer)

		var config = tus.Config{
			StoreComposer:           storeComposer,
			MaxSize:                 0, // TODO make configurable
			BasePath:                "",
			NotifyCompleteUploads:   false, // TODO expose channels
			NotifyTerminatedUploads: false,
			NotifyUploadProgress:    false,
			NotifyCreatedUploads:    false,
			Logger:                  nil,   // TODO
			RespectForwardedHeaders: false, // TODO true?
			PreUploadCreateCallback: nil,
		}

		tusHandler, err := tus.NewUnroutedHandler(config)
		if err != nil {
			panic(err.Error())
		}

		// TODO implement GetID and GetURL
		// https://github.com/tus/tusd/issues/340

		return tusHandler.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method { // the tus handler should have overidden the method at this point
			case "POST":
				tusHandler.PostFile(w, r)
			case "HEAD":
				tusHandler.HeadFile(w, r)
			case "PATCH":
				tusHandler.PatchFile(w, r)
			case "GET":
				tusHandler.GetFile(w, r)
			}
		}))
	}
	return nil
}
