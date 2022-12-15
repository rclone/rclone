package estuary

import (
	"context"
	"errors"
	"fmt"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
	"net/http"
	"net/url"
	"time"
)

const (
	colUuid = "coluuid"
	colDir  = "dir"
)

const (
	errNoUploadEndpoint = errors.New("No upload endpoint for object")
)

type CollectionCreate struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type DeleteContentFromCollectionBody struct {
	By    string `json:"by"`
	Value string `json:"value"`
}

type ContentAdd struct {
	ID      uint   `json:"estuaryId"`
	Cid     string `json:"cid,omitempty"`
	Error   string `json:"error"`
	Details string `json:"details"`
}

type IpfsPin struct {
	CID     string                 `json:"cid"`
	Name    string                 `json:"name"`
	Origins []string               `json:"origins"`
	Meta    map[string]interface{} `json:"meta"`
}

type ContentByCID struct {
	Content Content `json:"content"`
}

type Collection struct {
	UUID        string    `json:"uuid"`
	CreatedAt   time.Time `json:"createdAt"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UserID      uint      `json:"userId"`
}

func (f *Fs) fetchViewer(ctx context.Context) (response ViewerResponse, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "/viewer",
	}

	_, err = f.client.CallJSON(ctx, &opts, nil, &response)
	return
}

func (f *Fs) createCollection(ctx context.Context, name string) (id string, err error) {
	var resp *http.Response
	var collection Collection
	opts := rest.Opts{
		Method: "POST",
		Path:   "/collections",
	}
	create := CollectionCreate{
		Name:        name,
		Description: "",
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.client.CallJSON(ctx, &opts, &create, &collection)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return "", err
	}
	return collection.UUID, nil
}

func (f *Fs) listCollections(ctx context.Context) ([]Collection, error) {
	var collections []Collection
	err := f.pacer.Call(func() (bool, error) {
		response, err := f.client.CallJSON(ctx, &rest.Opts{
			Method: "GET",
			Path:   "/collections/",
		}, nil, &collections)
		return shouldRetry(ctx, response, err)
	})

	if err != nil {
		return nil, err
	}
	return collections, nil
}

func (f *Fs) getCollectionContents(ctx context.Context, collectionId, path string) ([]CollectionFsItem, error) {

	params := url.Values{}
	params.Set(colDir, path)

	var items []CollectionFsItem
	if err := f.pacer.Call(func() (bool, error) {
		response, err := f.client.CallJSON(ctx, &rest.Opts{
			Method:     "GET",
			Path:       fmt.Sprintf("/collections/%v", collectionId),
			Parameters: params,
		}, nil, &items)
		return shouldRetry(ctx, response, err)
	}); err != nil {
		return nil, err
	}
	return items, nil
}

func (f *Fs) deleteCollection(ctx context.Context, collectionId string) error {
	var collection Collection
	opts := rest.Opts{
		Method: "DELETE",
		Path:   "/collections/" + collectionId,
	}
	err := f.pacer.Call(func() (bool, error) {
		resp, err2 := f.client.CallJSON(ctx, &opts, nil, &collection)
		return shouldRetry(ctx, resp, err2)
	})
	return err
}

func (f *Fs) getContentByCid(ctx context.Context, cid string) ([]ContentByCID, error) {
	var result []ContentByCID
	opts := rest.Opts{
		Method: "GET",
		Path:   "/content/by-cid/" + cid,
	}
	_, err := f.client.CallJSON(ctx, &opts, nil, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (o *Object) removeContentFromCollection(ctx context.Context, collectionId string) error {
	opts := rest.Opts{
		Method: "DELETE",
		Path:   fmt.Sprintf("/collections/%s/contents", collectionId),
	}

	deleteBody := DeleteContentFromCollectionBody{
		By:    "content_id",
		Value: o.estuaryId,
	}

	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.client.CallJSON(ctx, &opts, &deleteBody, nil)
		return shouldRetry(ctx, resp, err)
	})

	return err
}

func (o *Object) addContent(ctx context.Context, opts rest.Opts) (result ContentAdd, err error) {
	endpoints := o.fs.viewer.Settings.UploadEndpoints

	if len(endpoints) == 0 {
		return ContentAdd{}, errNoUploadEndpoint
	}

	endpoint := 0

	var response *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		if endpoint == len(endpoints) {
			return false, errAllEndpointsFailed
		}

		// Note: "Path" is actually embedded in the upload endpoint, which we use as the RootURL
		opts.RootURL = endpoints[endpoint]
		response, err = o.fs.client.CallJSON(ctx, &opts, nil, &result)
		if contentAddingDisabled(response, err) {
			fs.Debugf(o, "failed upload, retry w/ next upload endpoint")
			endpoint += 1
			return true, err
		}

		return shouldRetry(ctx, response, err)
	})
	return result, err
}

func (o *Object) replacePin(ctx context.Context) {

}
