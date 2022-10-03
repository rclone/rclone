package s3

import (
	"net/http"

	"github.com/rclone/rclone/cmd/serve/s3/signature"
)

func (p *Server) authMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, rq *http.Request) {
		// var formatted, err = httputil.DumpRequest(rq, true)
		// if err != nil {
		// 	fmt.Fprint(w, err)
		// }

		// fmt.Println(rq.Header.Clone())
		if Opt.authPair != "" {
			if result := signature.Verify(rq); result != signature.ErrNone {
				resp := signature.GetAPIError(result)
				w.WriteHeader(resp.HTTPStatusCode)
				w.Header().Add("content-type", "application/xml")
				_, _ = w.Write(signature.EncodeAPIErrorToResponse(resp))
				return
			}
		}

		handler.ServeHTTP(w, rq)
	})
}
