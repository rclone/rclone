package docker

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rclone/rclone/fs"
)

const (
	contentType  = "application/vnd.docker.plugins.v1.1+json"
	activatePath = "/Plugin.Activate"
	createPath   = "/VolumeDriver.Create"
	getPath      = "/VolumeDriver.Get"
	listPath     = "/VolumeDriver.List"
	removePath   = "/VolumeDriver.Remove"
	pathPath     = "/VolumeDriver.Path"
	mountPath    = "/VolumeDriver.Mount"
	unmountPath  = "/VolumeDriver.Unmount"
	capsPath     = "/VolumeDriver.Capabilities"
)

// CreateRequest is the structure that docker's requests are deserialized to.
type CreateRequest struct {
	Name    string
	Options map[string]string `json:"Opts,omitempty"`
}

// RemoveRequest structure for a volume remove request
type RemoveRequest struct {
	Name string
}

// MountRequest structure for a volume mount request
type MountRequest struct {
	Name string
	ID   string
}

// MountResponse structure for a volume mount response
type MountResponse struct {
	Mountpoint string
}

// UnmountRequest structure for a volume unmount request
type UnmountRequest struct {
	Name string
	ID   string
}

// PathRequest structure for a volume path request
type PathRequest struct {
	Name string
}

// PathResponse structure for a volume path response
type PathResponse struct {
	Mountpoint string
}

// GetRequest structure for a volume get request
type GetRequest struct {
	Name string
}

// GetResponse structure for a volume get response
type GetResponse struct {
	Volume *VolInfo
}

// ListResponse structure for a volume list response
type ListResponse struct {
	Volumes []*VolInfo
}

// CapabilitiesResponse structure for a volume capability response
type CapabilitiesResponse struct {
	Capabilities Capability
}

// Capability represents the list of capabilities a volume driver can return
type Capability struct {
	Scope string
}

// ErrorResponse is a formatted error message that docker can understand
type ErrorResponse struct {
	Err string
}

func newRouter(drv *Driver) http.Handler {
	r := chi.NewRouter()
	r.Post(activatePath, func(w http.ResponseWriter, r *http.Request) {
		res := map[string]interface{}{
			"Implements": []string{"VolumeDriver"},
		}
		encodeResponse(w, res, nil, activatePath)
	})
	r.Post(createPath, func(w http.ResponseWriter, r *http.Request) {
		var req CreateRequest
		if decodeRequest(w, r, &req) {
			err := drv.Create(&req)
			encodeResponse(w, nil, err, createPath)
		}
	})
	r.Post(removePath, func(w http.ResponseWriter, r *http.Request) {
		var req RemoveRequest
		if decodeRequest(w, r, &req) {
			err := drv.Remove(&req)
			encodeResponse(w, nil, err, removePath)
		}
	})
	r.Post(mountPath, func(w http.ResponseWriter, r *http.Request) {
		var req MountRequest
		if decodeRequest(w, r, &req) {
			res, err := drv.Mount(&req)
			encodeResponse(w, res, err, mountPath)
		}
	})
	r.Post(pathPath, func(w http.ResponseWriter, r *http.Request) {
		var req PathRequest
		if decodeRequest(w, r, &req) {
			res, err := drv.Path(&req)
			encodeResponse(w, res, err, pathPath)
		}
	})
	r.Post(getPath, func(w http.ResponseWriter, r *http.Request) {
		var req GetRequest
		if decodeRequest(w, r, &req) {
			res, err := drv.Get(&req)
			encodeResponse(w, res, err, getPath)
		}
	})
	r.Post(unmountPath, func(w http.ResponseWriter, r *http.Request) {
		var req UnmountRequest
		if decodeRequest(w, r, &req) {
			err := drv.Unmount(&req)
			encodeResponse(w, nil, err, unmountPath)
		}
	})
	r.Post(listPath, func(w http.ResponseWriter, r *http.Request) {
		res, err := drv.List()
		encodeResponse(w, res, err, listPath)
	})
	r.Post(capsPath, func(w http.ResponseWriter, r *http.Request) {
		res := &CapabilitiesResponse{
			Capabilities: Capability{Scope: pluginScope},
		}
		encodeResponse(w, res, nil, capsPath)
	})
	return r
}

func decodeRequest(w http.ResponseWriter, r *http.Request, req interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

func encodeResponse(w http.ResponseWriter, res interface{}, err error, path string) {
	w.Header().Set("Content-Type", contentType)
	if err != nil {
		fs.Debugf(path, "Request returned error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		res = &ErrorResponse{Err: err.Error()}
	} else if res == nil {
		res = struct{}{}
	}
	if err = json.NewEncoder(w).Encode(res); err != nil {
		fs.Debugf(path, "Response encoding failed: %v", err)
	}
}
