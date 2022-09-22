package s3

import (
	"net/http"
)

func (p *Server) authMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, rq *http.Request) {
		// var formatted, err = httputil.DumpRequest(rq, true)
		// if err != nil {
		// 	fmt.Fprint(w, err)
		// }

		// fmt.Println(rq.Header.Clone())
		handler.ServeHTTP(w, rq)
	})
}
