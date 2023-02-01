package estuary

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/rclone/rclone/lib/rest"
)

const (
	colDir    = "dir"
	overwrite = "overwrite"
)

func (f *Fs) createCollection(ctx context.Context, name string) (id string, err error) {
	var resp *http.Response
	var collection collection
	opts := rest.Opts{
		Method: "POST",
		Path:   "/collections",
	}
	create := collectionCreate{
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

func (f *Fs) listCollections(ctx context.Context) ([]collection, error) {
	var collections []collection
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

func (f *Fs) getCollectionContents(ctx context.Context, collectionID, path string) ([]collectionFsItem, error) {

	params := url.Values{}
	params.Set(colDir, path)

	var items []collectionFsItem
	if err := f.pacer.Call(func() (bool, error) {
		response, err := f.client.CallJSON(ctx, &rest.Opts{
			Method:     "GET",
			Path:       fmt.Sprintf("/collections/%v", collectionID),
			Parameters: params,
		}, nil, &items)
		return shouldRetry(ctx, response, err)
	}); err != nil {
		return nil, err
	}
	return items, nil
}

func (f *Fs) deleteCollection(ctx context.Context, collectionID string) error {
	var collection collection
	opts := rest.Opts{
		Method: "DELETE",
		Path:   "/collections/" + collectionID,
	}
	err := f.pacer.Call(func() (bool, error) {
		resp, err2 := f.client.CallJSON(ctx, &opts, nil, &collection)
		return shouldRetry(ctx, resp, err2)
	})
	return err
}

func (f *Fs) getContentByCid(ctx context.Context, cid string) ([]content, error) {
	var result []content
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

func (o *Object) removeContentFromCollection(ctx context.Context, collectionID string) error {
	opts := rest.Opts{
		Method: "DELETE",
		Path:   fmt.Sprintf("/collections/%s/contents", collectionID),
	}

	deleteBody := deleteContentFromCollectionBody{
		By:    "content_id",
		Value: o.estuaryID,
	}

	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.client.CallJSON(ctx, &opts, &deleteBody, nil)
		return shouldRetry(ctx, resp, err)
	})

	return err
}

func (o *Object) addContent(ctx context.Context, opts rest.Opts) (result contentAdd, err error) {
	params := url.Values{}
	params.Set(overwrite, "true")
	opts.Parameters = params

	var response *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		response, err = o.fs.client.CallJSON(ctx, &opts, nil, &result)

		return shouldRetry(ctx, response, err)
	})
	return result, err
}

func (f *Fs) getPin(ctx context.Context, id uint) (ipfsPin, error) {
	var result ipfsPinStatusResponse
	opts := rest.Opts{
		Method: "GET",
		Path:   fmt.Sprintf("/pinning/pins/%v", id),
	}

	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.client.CallJSON(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})

	return result.Pin, err
}

func (f *Fs) replacePin(ctx context.Context, id uint, pin ipfsPin) (string, error) {
	var result ipfsPinStatusResponse
	opts := rest.Opts{
		Method: "POST",
		Path:   fmt.Sprintf("/pinning/pins/%v", id),
	}

	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.client.CallJSON(ctx, &opts, &pin, &result)
		return shouldRetry(ctx, resp, err)
	})

	return result.RequestID, err
}

func (f *Fs) addContentsToCollection(ctx context.Context, coluuid, dir string, contentIds []uint) error {
	params := url.Values{}
	params.Set(colDir, dir)
	params.Set(overwrite, "true")

	opts := rest.Opts{
		Method:     "POST",
		Path:       fmt.Sprintf("/collections/%s", coluuid),
		Parameters: params,
	}

	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.client.CallJSON(ctx, &opts, &contentIds, nil)
		return shouldRetry(ctx, resp, err)
	})

	return err
}
