package estuary

import (
	"context"
	"fmt"
	"github.com/rclone/rclone/lib/rest"
	"net/http"
	"net/url"
)

const (
	colUuid = "coluuid"
	colDir  = "dir"
)

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

func (f *Fs) deleteCollection(ctx context.Context, collectiond string) error {
	var collection Collection
	opts := rest.Opts{
		Method: "DELETE",
		Path:   "/collections/" + collectiond,
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
