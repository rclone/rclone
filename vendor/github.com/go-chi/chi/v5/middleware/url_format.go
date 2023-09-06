package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

var (
	// URLFormatCtxKey is the context.Context key to store the URL format data
	// for a request.
	URLFormatCtxKey = &contextKey{"URLFormat"}
)

// URLFormat is a middleware that parses the url extension from a request path and stores it
// on the context as a string under the key `middleware.URLFormatCtxKey`. The middleware will
// trim the suffix from the routing path and continue routing.
//
// Routers should not include a url parameter for the suffix when using this middleware.
//
// Sample usage.. for url paths: `/articles/1`, `/articles/1.json` and `/articles/1.xml`
//
//  func routes() http.Handler {
//    r := chi.NewRouter()
//    r.Use(middleware.URLFormat)
//
//    r.Get("/articles/{id}", ListArticles)
//
//    return r
//  }
//
//  func ListArticles(w http.ResponseWriter, r *http.Request) {
// 	  urlFormat, _ := r.Context().Value(middleware.URLFormatCtxKey).(string)
//
// 	  switch urlFormat {
// 	  case "json":
// 	  	render.JSON(w, r, articles)
// 	  case "xml:"
// 	  	render.XML(w, r, articles)
// 	  default:
// 	  	render.JSON(w, r, articles)
// 	  }
// }
//
func URLFormat(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var format string
		path := r.URL.Path

		rctx := chi.RouteContext(r.Context())
		if rctx != nil && rctx.RoutePath != "" {
			path = rctx.RoutePath
		}

		if strings.Index(path, ".") > 0 {
			base := strings.LastIndex(path, "/")
			idx := strings.LastIndex(path[base:], ".")

			if idx > 0 {
				idx += base
				format = path[idx+1:]

				rctx.RoutePath = path[:idx]
			}
		}

		r = r.WithContext(context.WithValue(ctx, URLFormatCtxKey, format))

		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
