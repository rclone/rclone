package httpclient

import (
	"io"
	"mime/multipart"
	"net/http"
)

func (req *RequestData) UploadFile(fieldName string, fileName string, reader io.Reader) (err error) {
	return req.UploadFileExtra(fieldName, fileName, reader, nil)
}

func (req *RequestData) UploadFileExtra(fieldName string, fileName string, reader io.Reader, extra map[string]string) (err error) {
	r, w := io.Pipe()

	writer := multipart.NewWriter(w)

	go func() {
		var err error

		defer func() {
			if err == nil {
				w.Close()
			}
		}()

		for k, v := range extra {
			err = writer.WriteField(k, v)

			if err != nil {
				w.CloseWithError(err)
				return
			}
		}

		part, err := writer.CreateFormFile(fieldName, fileName)

		if err != nil {
			w.CloseWithError(err)
			return
		}

		defer writer.Close()

		_, err = io.Copy(part, reader)

		if err != nil {
			w.CloseWithError(err)
			return
		}
	}()

	req.ReqReader = r

	if req.Headers == nil {
		req.Headers = make(http.Header)
	}

	req.Headers.Set("Content-Type", writer.FormDataContentType())

	return
}
