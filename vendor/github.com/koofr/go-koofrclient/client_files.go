package koofrclient

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"

	"github.com/koofr/go-httpclient"
)

var ErrCannotOverwrite = fmt.Errorf("Can not overwrite (filter constraint fails)")
var ErrCannotRemove = fmt.Errorf("Can not remove (filter constraint fails)")

func (c *KoofrClient) FilesInfo(mountId string, path string) (info FileInfo, err error) {
	params := url.Values{}
	params.Set("path", path)

	request := httpclient.RequestData{
		Method:         "GET",
		Path:           "/api/v2/mounts/" + mountId + "/files/info",
		Params:         params,
		ExpectedStatus: []int{http.StatusOK},
		RespEncoding:   httpclient.EncodingJSON,
		RespValue:      &info,
	}

	_, err = c.Request(&request)

	return
}

func (c *KoofrClient) FilesList(mountId string, basePath string) (files []FileInfo, err error) {
	f := &struct {
		Files *[]FileInfo
	}{&files}

	params := url.Values{}
	params.Set("path", basePath)

	request := httpclient.RequestData{
		Method:         "GET",
		Path:           "/api/v2/mounts/" + mountId + "/files/list",
		Params:         params,
		ExpectedStatus: []int{http.StatusOK},
		RespEncoding:   httpclient.EncodingJSON,
		RespValue:      &f,
	}

	_, err = c.Request(&request)

	if err != nil {
		return
	}

	for i := range files {
		files[i].Path = path.Join(basePath, files[i].Name)
	}

	return
}

func (c *KoofrClient) FilesTree(mountId string, path string) (tree FileTree, err error) {
	params := url.Values{}
	params.Set("path", path)

	request := httpclient.RequestData{
		Method:         "GET",
		Path:           "/api/v2/mounts/" + mountId + "/files/tree",
		Params:         params,
		ExpectedStatus: []int{http.StatusOK},
		RespEncoding:   httpclient.EncodingJSON,
		RespValue:      &tree,
	}

	_, err = c.Request(&request)

	return
}

func (c *KoofrClient) FilesDelete(mountId string, path string) (err error) {
	return c.filesDelete(mountId, path, nil)
}

func (c *KoofrClient) FilesDeleteIf(mountId string, path string, deleteFilter *DeleteFilter) (err error) {
	return c.filesDelete(mountId, path, deleteFilter)
}

func (c *KoofrClient) filesDelete(mountId string, path string, deleteFilter *DeleteFilter) (err error) {
	params := url.Values{}
	params.Set("path", path)

	if deleteFilter != nil {
		if deleteFilter.Size != nil {
			params.Set("removeIfSize", fmt.Sprintf("%d", *deleteFilter.Size))
		}
		if deleteFilter.Modified != nil {
			params.Set("removeIfModified", fmt.Sprintf("%d", *deleteFilter.Modified))
		}
		if deleteFilter.Hash != nil {
			params.Set("removeIfHash", fmt.Sprintf("%s", *deleteFilter.Hash))
		}
		if deleteFilter.IfEmpty {
			params.Set("removeIfEmpty", "")
		}
	}

	request := httpclient.RequestData{
		Method:         "DELETE",
		Path:           "/api/v2/mounts/" + mountId + "/files/remove",
		Params:         params,
		ExpectedStatus: []int{http.StatusOK},
		RespConsume:    true,
	}

	_, err = c.Request(&request)

	if err != nil {
		switch err := err.(type) {
		case httpclient.InvalidStatusError:
			if err.Got == http.StatusConflict {
				return ErrCannotRemove
			}
		default:
			return err
		}

	}

	return
}

func (c *KoofrClient) FilesNewFolder(mountId string, path string, name string) (err error) {
	reqData := FolderCreate{name}

	params := url.Values{}
	params.Set("path", path)

	request := httpclient.RequestData{
		Method:         "POST",
		Path:           "/api/v2/mounts/" + mountId + "/files/folder",
		Params:         params,
		ExpectedStatus: []int{http.StatusOK, http.StatusCreated},
		ReqEncoding:    httpclient.EncodingJSON,
		ReqValue:       reqData,
		RespConsume:    true,
	}

	_, err = c.Request(&request)

	return
}

func (c *KoofrClient) FilesCopy(mountId string, path string, toMountId string, toPath string) (err error) {
	reqData := FileCopy{toMountId, toPath}

	params := url.Values{}
	params.Set("path", path)

	request := httpclient.RequestData{
		Method:         "PUT",
		Path:           "/api/v2/mounts/" + mountId + "/files/copy",
		Params:         params,
		ExpectedStatus: []int{http.StatusOK},
		ReqEncoding:    httpclient.EncodingJSON,
		ReqValue:       reqData,
		RespConsume:    true,
	}

	_, err = c.Request(&request)

	return
}

func (c *KoofrClient) FilesMove(mountId string, path string, toMountId string, toPath string) (err error) {
	reqData := FileMove{toMountId, toPath}

	params := url.Values{}
	params.Set("path", path)

	request := httpclient.RequestData{
		Method:         "PUT",
		Path:           "/api/v2/mounts/" + mountId + "/files/move",
		Params:         params,
		ExpectedStatus: []int{http.StatusOK},
		ReqEncoding:    httpclient.EncodingJSON,
		ReqValue:       reqData,
		RespConsume:    true,
	}

	_, err = c.Request(&request)

	return
}

func (c *KoofrClient) FilesGetRange(mountId string, path string, span *FileSpan) (reader io.ReadCloser, err error) {
	params := url.Values{}
	params.Set("path", path)

	request := httpclient.RequestData{
		Method:         "GET",
		Path:           "/content/api/v2/mounts/" + mountId + "/files/get",
		Params:         params,
		Headers:        make(http.Header),
		ExpectedStatus: []int{http.StatusOK, http.StatusPartialContent},
	}

	if span != nil {
		if span.End == -1 {
			request.Headers.Set("Range", fmt.Sprintf("bytes=%d-", span.Start))
		} else {
			request.Headers.Set("Range", fmt.Sprintf("bytes=%d-%d", span.Start, span.End))
		}
	}

	res, err := c.Request(&request)

	if err != nil {
		return
	}

	reader = res.Body

	return
}

func (c *KoofrClient) FilesGet(mountId string, path string) (reader io.ReadCloser, err error) {
	return c.FilesGetRange(mountId, path, nil)
}

func (c *KoofrClient) FilesPut(mountId string, path string, name string, reader io.Reader) (newName string, err error) {
	info, err := c.FilesPutOptions(mountId, path, name, reader, nil)
	return info.Name, err
}

func (c *KoofrClient) FilesPutOptions(mountId string, path string, name string, reader io.Reader, putFilter *PutFilter) (fileInfo *FileInfo, err error) {
	params := url.Values{}
	params.Set("path", path)
	params.Set("filename", name)
	params.Set("info", "true")

	if putFilter != nil {
		if putFilter.Size != nil {
			params.Set("overwriteIfSize", fmt.Sprintf("%d", *putFilter.Size))
		}
		if putFilter.Modified != nil {
			params.Set("overwriteIfModified", fmt.Sprintf("%d", *putFilter.Modified))
		}
		if putFilter.Hash != nil {
			params.Set("overwriteIfHash", fmt.Sprintf("%s", *putFilter.Hash))
		}
		if putFilter.IgnoreNonExisting {
			params.Set("overwriteIgnoreNonexisting", "")
		}
		if putFilter.NoRename {
			params.Set("autorename", "false")
		}
		if putFilter.ForceOverwrite {
			params.Set("overwrite", "true")
		}
	}

	request := httpclient.RequestData{
		Method:         "POST",
		Path:           "/content/api/v2/mounts/" + mountId + "/files/put",
		Params:         params,
		ExpectedStatus: []int{http.StatusOK},
		RespEncoding:   httpclient.EncodingJSON,
		RespValue:      &fileInfo,
	}

	err = request.UploadFile("file", "dummy", reader)

	if err != nil {
		return
	}

	_, err = c.Request(&request)

	if err != nil {

		switch err := err.(type) {
		case httpclient.InvalidStatusError:
			if err.Got == http.StatusConflict {
				return nil, ErrCannotOverwrite
			}
		default:
			return nil, err
		}
	}

	return
}
