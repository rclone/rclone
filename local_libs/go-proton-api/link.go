package proton

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-resty/resty/v2"
)

func (c *Client) GetLink(ctx context.Context, shareID, linkID string) (Link, error) {
	var res struct {
		Link Link
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/drive/shares/" + shareID + "/links/" + linkID)
	}); err != nil {
		return Link{}, err
	}

	return res.Link, nil
}

func (c *Client) MoveLink(ctx context.Context, shareID, linkID string, req MoveLinkReq) error {
	var res struct {
		Code int
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).SetBody(req).Put("/drive/shares/" + shareID + "/links/" + linkID + "/move")
	}); err != nil {
		return err
	}

	return nil
}

func (c *Client) CreateFile(ctx context.Context, shareID string, req CreateFileReq) (CreateFileRes, error) {
	var res struct {
		Code int
		File CreateFileRes
	}

	resp, err := c.doRes(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).SetBody(req).Post("/drive/shares/" + shareID + "/files")
	})
	if err != nil { // if the status code is not 200~299, it's considered an error

		// handle the file or folder name exists error
		if resp.StatusCode() == http.StatusUnprocessableEntity /* 422 */ {
			var apiError APIError
			err := json.Unmarshal(resp.Body(), &apiError)
			if err != nil {
				return CreateFileRes{}, err
			}
			if apiError.Code == AFileOrFolderNameExist {
				return CreateFileRes{}, ErrFileNameExist // since we are in CreateFile, so we return this error
			}
		}

		// handle draft exists error
		if resp.StatusCode() == http.StatusConflict /* 409 */ {
			var apiError APIError
			err := json.Unmarshal(resp.Body(), &apiError)
			if err != nil {
				return CreateFileRes{}, err
			}
			if apiError.Code == ADraftExist {
				return CreateFileRes{}, ErrADraftExist
			}
		}

		return CreateFileRes{}, err
	}

	return res.File, nil
}

func (c *Client) CreateFolder(ctx context.Context, shareID string, req CreateFolderReq) (CreateFolderRes, error) {
	var res struct {
		Folder CreateFolderRes
	}

	resp, err := c.doRes(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).SetBody(req).Post("/drive/shares/" + shareID + "/folders")
	})
	if err != nil { // if the status code is not 200~299, it's considered an error

		// handle the file or folder name exists error
		if resp.StatusCode() == http.StatusUnprocessableEntity /* 422 */ {
			// log.Println(resp.String())
			var apiError APIError
			err := json.Unmarshal(resp.Body(), &apiError)
			if err != nil {
				return CreateFolderRes{}, err
			}
			if apiError.Code == AFileOrFolderNameExist {
				return CreateFolderRes{}, ErrFolderNameExist // since we are in CreateFolder, so we return this error
			}
		}

		return CreateFolderRes{}, err
	}

	return res.Folder, nil
}

func (c *Client) CheckAvailableHashes(ctx context.Context, shareID, linkID string, req CheckAvailableHashesReq) (CheckAvailableHashesRes, error) {
	var res struct {
		AvailableHashes   []string
		PendingHashesData []PendingHashData
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).SetBody(req).Post("/drive/shares/" + shareID + "/links/" + linkID + "/checkAvailableHashes")
	}); err != nil {
		return CheckAvailableHashesRes{}, err
	}

	return CheckAvailableHashesRes{
		AvailableHashes:   res.AvailableHashes,
		PendingHashesData: res.PendingHashesData,
	}, nil
}
