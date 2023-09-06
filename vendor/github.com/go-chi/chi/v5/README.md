# <img alt="chi" src="https://cdn.rawgit.com/go-chi/chi/master/_examples/chi.svg" width="220" />


[![GoDoc Widget]][GoDoc]

`chi` is a lightweight, idiomatic and composable router for building Go HTTP services. It's
especially good at helping you write large REST API services that are kept maintainable as your
project grows and changes. `chi` is built on the new `context` package introduced in Go 1.7 to
handle signaling, cancelation and request-scoped values across a handler chain.

The focus of the project has been to seek out an elegant and comfortable design for writing
REST API servers, written during the development of the Pressly API service that powers our
public API service, which in turn powers all of our client-side applications.

The key considerations of chi's design are: project structure, maintainability, standard http
handlers (stdlib-only), developer productivity, and deconstructing a large system into many small
parts. The core router `github.com/go-chi/chi` is quite small (less than 1000 LOC), but we've also
included some useful/optional subpackages: [middleware](/middleware), [render](https://github.com/go-chi/render)
and [docgen](https://github.com/go-chi/docgen). We hope you enjoy it too!

## Install

`go get -u github.com/go-chi/chi/v5`


## Features

* **Lightweight** - cloc'd in ~1000 LOC for the chi router
* **Fast** - yes, see [benchmarks](#benchmarks)
* **100% compatible with net/http** - use any http or middleware pkg in the ecosystem that is also compatible with `net/http`
* **Designed for modular/composable APIs** - middlewares, inline middlewares, route groups and sub-router mounting
* **Context control** - built on new `context` package, providing value chaining, cancellations and timeouts
* **Robust** - in production at Pressly, Cloudflare, Heroku, 99Designs, and many others (see [discussion](https://github.com/go-chi/chi/issues/91))
* **Doc generation** - `docgen` auto-generates routing documentation from your source to JSON or Markdown
* **Go.mod support** - as of v5, go.mod support (see [CHANGELOG](https://github.com/go-chi/chi/blob/master/CHANGELOG.md))
* **No external dependencies** - plain ol' Go stdlib + net/http


## Examples

See [_examples/](https://github.com/go-chi/chi/blob/master/_examples/) for a variety of examples.


**As easy as:**

```go
package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("welcome"))
	})
	http.ListenAndServe(":3000", r)
}
```

**REST Preview:**

Here is a little preview of how routing looks like with chi. Also take a look at the generated routing docs
in JSON ([routes.json](https://github.com/go-chi/chi/blob/master/_examples/rest/routes.json)) and in
Markdown ([routes.md](https://github.com/go-chi/chi/blob/master/_examples/rest/routes.md)).

I highly recommend reading the source of the [examples](https://github.com/go-chi/chi/blob/master/_examples/) listed
above, they will show you all the features of chi and serve as a good form of documentation.

```go
import (
  //...
  "context"
  "github.com/go-chi/chi/v5"
  "github.com/go-chi/chi/v5/middleware"
)

func main() {
  r := chi.NewRouter()

  // A good base middleware stack
  r.Use(middleware.RequestID)
  r.Use(middleware.RealIP)
  r.Use(middleware.Logger)
  r.Use(middleware.Recoverer)

  // Set a timeout value on the request context (ctx), that will signal
  // through ctx.Done() that the request has timed out and further
  // processing should be stopped.
  r.Use(middleware.Timeout(60 * time.Second))

  r.Get("/", func(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("hi"))
  })

  // RESTy routes for "articles" resource
  r.Route("/articles", func(r chi.Router) {
    r.With(paginate).Get("/", listArticles)                           // GET /articles
    r.With(paginate).Get("/{month}-{day}-{year}", listArticlesByDate) // GET /articles/01-16-2017

    r.Post("/", createArticle)                                        // POST /articles
    r.Get("/search", searchArticles)                                  // GET /articles/search

    // Regexp url parameters:
    r.Get("/{articleSlug:[a-z-]+}", getArticleBySlug)                // GET /articles/home-is-toronto

    // Subrouters:
    r.Route("/{articleID}", func(r chi.Router) {
      r.Use(ArticleCtx)
      r.Get("/", getArticle)                                          // GET /articles/123
      r.Put("/", updateArticle)                                       // PUT /articles/123
      r.Delete("/", deleteArticle)                                    // DELETE /articles/123
    })
  })

  // Mount the admin sub-router
  r.Mount("/admin", adminRouter())

  http.ListenAndServe(":3333", r)
}

func ArticleCtx(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    articleID := chi.URLParam(r, "articleID")
    article, err := dbGetArticle(articleID)
    if err != nil {
      http.Error(w, http.StatusText(404), 404)
      return
    }
    ctx := context.WithValue(r.Context(), "article", article)
    next.ServeHTTP(w, r.WithContext(ctx))
  })
}

func getArticle(w http.ResponseWriter, r *http.Request) {
  ctx := r.Context()
  article, ok := ctx.Value("article").(*Article)
  if !ok {
    http.Error(w, http.StatusText(422), 422)
    return
  }
  w.Write([]byte(fmt.Sprintf("title:%s", article.Title)))
}

// A completely separate router for administrator routes
func adminRouter() http.Handler {
  r := chi.NewRouter()
  r.Use(AdminOnly)
  r.Get("/", adminIndex)
  r.Get("/accounts", adminListAccounts)
  return r
}

func AdminOnly(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    perm, ok := ctx.Value("acl.permission").(YourPermissionType)
    if !ok || !perm.IsAdmin() {
      http.Error(w, http.StatusText(403), 403)
      return
    }
    next.ServeHTTP(w, r)
  })
}
```


## Router interface

chi's router is based on a kind of [Patricia Radix trie](https://en.wikipedia.org/wiki/Radix_tree).
The router is fully compatible with `net/http`.

Built on top of the tree is the `Router` interface:

```go
// Router consisting of the core routing methods used by chi's Mux,
// using only the standard net/http.
type Router interface {
	http.Handler
	Routes

	// Use appends one or more middlewares onto the Router stack.
	Use(middlewares ...func(http.Handler) http.Handler)

	// With adds inline middlewares for an endpoint handler.
	With(middlewares ...func(http.Handler) http.Handler) Router

	// Group adds a new inline-Router along the current routing
	// path, with a fresh middleware stack for the inline-Router.
	Group(fn func(r Router)) Router

	// Route mounts a sub-Router along a `pattern`` string.
	Route(pattern string, fn func(r Router)) Router

	// Mount attaches another http.Handler along ./pattern/*
	Mount(pattern string, h http.Handler)

	// Handle and HandleFunc adds routes for `pattern` that matches
	// all HTTP methods.
	Handle(pattern string, h http.Handler)
	HandleFunc(pattern string, h http.HandlerFunc)

	// Method and MethodFunc adds routes for `pattern` that matches
	// the `method` HTTP method.
	Method(method, pattern string, h http.Handler)
	MethodFunc(method, pattern string, h http.HandlerFunc)

	// HTTP-method routing along `pattern`
	Connect(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
	Get(pattern string, h http.HandlerFunc)
	Head(pattern string, h http.HandlerFunc)
	Options(pattern string, h http.HandlerFunc)
	Patch(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
	Trace(pattern string, h http.HandlerFunc)

	// NotFound defines a handler to respond whenever a route could
	// not be found.
	NotFound(h http.HandlerFunc)

	// MethodNotAllowed defines a handler to respond whenever a method is
	// not allowed.
	MethodNotAllowed(h http.HandlerFunc)
}

// Routes interface adds two methods for router traversal, which is also
// used by the github.com/go-chi/docgen package to generate documentation for Routers.
type Routes interface {
	// Routes returns the routing tree in an easily traversable structure.
	Routes() []Route

	// Middlewares returns the list of middlewares in use by the router.
	Middlewares() Middlewares

	// Match searches the routing tree for a handler that matches
	// the method/path - similar to routing a http request, but without
	// executing the handler thereafter.
	Match(rctx *Context, method, path string) bool
}
```

Each routing method accepts a URL `pattern` and chain of `handlers`. The URL pattern
supports named params (ie. `/users/{userID}`) and wildcards (ie. `/admin/*`). URL parameters
can be fetched at runtime by calling `chi.URLParam(r, "userID")` for named parameters
and `chi.URLParam(r, "*")` for a wildcard parameter.


### Middleware handlers

chi's middlewares are just stdlib net/http middleware handlers. There is nothing special
about them, which means the router and all the tooling is designed to be compatible and
friendly with any middleware in the community. This offers much better extensibility and reuse
of packages and is at the heart of chi's purpose.

Here is an example of a standard net/http middleware where we assign a context key `"user"`
the value of `"123"`. This middleware sets a hypothetical user identifier on the request
context and calls the next handler in the chain.

```go
// HTTP middleware setting a value on the request context
func MyMiddleware(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // create new context from `r` request context, and assign key `"user"`
    // to value of `"123"`
    ctx := context.WithValue(r.Context(), "user", "123")

    // call the next handler in the chain, passing the response writer and
    // the updated request object with the new context value.
    //
    // note: context.Context values are nested, so any previously set
    // values will be accessible as well, and the new `"user"` key
    // will be accessible from this point forward.
    next.ServeHTTP(w, r.WithContext(ctx))
  })
}
```


### Request handlers

chi uses standard net/http request handlers. This little snippet is an example of a http.Handler
func that reads a user identifier from the request context - hypothetically, identifying
the user sending an authenticated request, validated+set by a previous middleware handler.

```go
// HTTP handler accessing data from the request context.
func MyRequestHandler(w http.ResponseWriter, r *http.Request) {
  // here we read from the request context and fetch out `"user"` key set in
  // the MyMiddleware example above.
  user := r.Context().Value("user").(string)

  // respond to the client
  w.Write([]byte(fmt.Sprintf("hi %s", user)))
}
```


### URL parameters

chi's router parses and stores URL parameters right onto the request context. Here is
an example of how to access URL params in your net/http handlers. And of course, middlewares
are able to access the same information.

```go
// HTTP handler accessing the url routing parameters.
func MyRequestHandler(w http.ResponseWriter, r *http.Request) {
  // fetch the url parameter `"userID"` from the request of a matching
  // routing pattern. An example routing pattern could be: /users/{userID}
  userID := chi.URLParam(r, "userID")

  // fetch `"key"` from the request context
  ctx := r.Context()
  key := ctx.Value("key").(string)

  // respond to the client
  w.Write([]byte(fmt.Sprintf("hi %v, %v", userID, key)))
}
```


## Middlewares

chi comes equipped with an optional `middleware` package, providing a suite of standard
`net/http` middlewares. Please note, any middleware in the ecosystem that is also compatible
with `net/http` can be used with chi's mux.

### Core middlewares

----------------------------------------------------------------------------------------------------
| chi/middleware Handler | description                                                             |
| :--------------------- | :---------------------------------------------------------------------- |
| [AllowContentEncoding] | Enforces a whitelist of request Content-Encoding headers                |
| [AllowContentType]     | Explicit whitelist of accepted request Content-Types                    |
| [BasicAuth]            | Basic HTTP authentication                                               |
| [Compress]             | Gzip compression for clients that accept compressed responses           |
| [ContentCharset]       | Ensure charset for Content-Type request headers                         |
| [CleanPath]            | Clean double slashes from request path                                  |
| [GetHead]              | Automatically route undefined HEAD requests to GET handlers             |
| [Heartbeat]            | Monitoring endpoint to check the servers pulse                          |
| [Logger]               | Logs the start and end of each request with the elapsed processing time |
| [NoCache]              | Sets response headers to prevent clients from caching                   |
| [Profiler]             | Easily attach net/http/pprof to your routers                            |
| [RealIP]               | Sets a http.Request's RemoteAddr to either X-Real-IP or X-Forwarded-For |
| [Recoverer]            | Gracefully absorb panics and prints the stack trace                     |
| [RequestID]            | Injects a request ID into the context of each request                   |
| [RedirectSlashes]      | Redirect slashes on routing paths                                       |
| [RouteHeaders]         | Route handling for request headers                                      |
| [SetHeader]            | Short-hand middleware to set a response header key/value                |
| [StripSlashes]         | Strip slashes on routing paths                                          |
| [Throttle]             | Puts a ceiling on the number of concurrent requests                     |
| [Timeout]              | Signals to the request context when the timeout deadline is reached     |
| [URLFormat]            | Parse extension from url and put it on request context                  |
| [WithValue]            | Short-hand middleware to set a key/value on the request context         |
----------------------------------------------------------------------------------------------------

[AllowContentEncoding]: https://pkg.go.dev/github.com/go-chi/chi/middleware#AllowContentEncoding
[AllowContentType]: https://pkg.go.dev/github.com/go-chi/chi/middleware#AllowContentType
[BasicAuth]: https://pkg.go.dev/github.com/go-chi/chi/middleware#BasicAuth
[Compress]: https://pkg.go.dev/github.com/go-chi/chi/middleware#Compress
[ContentCharset]: https://pkg.go.dev/github.com/go-chi/chi/middleware#ContentCharset
[CleanPath]: https://pkg.go.dev/github.com/go-chi/chi/middleware#CleanPath
[GetHead]: https://pkg.go.dev/github.com/go-chi/chi/middleware#GetHead
[GetReqID]: https://pkg.go.dev/github.com/go-chi/chi/middleware#GetReqID
[Heartbeat]: https://pkg.go.dev/github.com/go-chi/chi/middleware#Heartbeat
[Logger]: https://pkg.go.dev/github.com/go-chi/chi/middleware#Logger
[NoCache]: https://pkg.go.dev/github.com/go-chi/chi/middleware#NoCache
[Profiler]: https://pkg.go.dev/github.com/go-chi/chi/middleware#Profiler
[RealIP]: https://pkg.go.dev/github.com/go-chi/chi/middleware#RealIP
[Recoverer]: https://pkg.go.dev/github.com/go-chi/chi/middleware#Recoverer
[RedirectSlashes]: https://pkg.go.dev/github.com/go-chi/chi/middleware#RedirectSlashes
[RequestLogger]: https://pkg.go.dev/github.com/go-chi/chi/middleware#RequestLogger
[RequestID]: https://pkg.go.dev/github.com/go-chi/chi/middleware#RequestID
[RouteHeaders]: https://pkg.go.dev/github.com/go-chi/chi/middleware#RouteHeaders
[SetHeader]: https://pkg.go.dev/github.com/go-chi/chi/middleware#SetHeader
[StripSlashes]: https://pkg.go.dev/github.com/go-chi/chi/middleware#StripSlashes
[Throttle]: https://pkg.go.dev/github.com/go-chi/chi/middleware#Throttle
[ThrottleBacklog]: https://pkg.go.dev/github.com/go-chi/chi/middleware#ThrottleBacklog
[ThrottleWithOpts]: https://pkg.go.dev/github.com/go-chi/chi/middleware#ThrottleWithOpts
[Timeout]: https://pkg.go.dev/github.com/go-chi/chi/middleware#Timeout
[URLFormat]: https://pkg.go.dev/github.com/go-chi/chi/middleware#URLFormat
[WithLogEntry]: https://pkg.go.dev/github.com/go-chi/chi/middleware#WithLogEntry
[WithValue]: https://pkg.go.dev/github.com/go-chi/chi/middleware#WithValue
[Compressor]: https://pkg.go.dev/github.com/go-chi/chi/middleware#Compressor
[DefaultLogFormatter]: https://pkg.go.dev/github.com/go-chi/chi/middleware#DefaultLogFormatter
[EncoderFunc]: https://pkg.go.dev/github.com/go-chi/chi/middleware#EncoderFunc
[HeaderRoute]: https://pkg.go.dev/github.com/go-chi/chi/middleware#HeaderRoute
[HeaderRouter]: https://pkg.go.dev/github.com/go-chi/chi/middleware#HeaderRouter
[LogEntry]: https://pkg.go.dev/github.com/go-chi/chi/middleware#LogEntry
[LogFormatter]: https://pkg.go.dev/github.com/go-chi/chi/middleware#LogFormatter
[LoggerInterface]: https://pkg.go.dev/github.com/go-chi/chi/middleware#LoggerInterface
[ThrottleOpts]: https://pkg.go.dev/github.com/go-chi/chi/middleware#ThrottleOpts
[WrapResponseWriter]: https://pkg.go.dev/github.com/go-chi/chi/middleware#WrapResponseWriter

### Extra middlewares & packages

Please see https://github.com/go-chi for additional packages.

--------------------------------------------------------------------------------------------------------------------
| package                                            | description                                                 |
|:---------------------------------------------------|:-------------------------------------------------------------
| [cors](https://github.com/go-chi/cors)             | Cross-origin resource sharing (CORS)                        |
| [docgen](https://github.com/go-chi/docgen)         | Print chi.Router routes at runtime                          |
| [jwtauth](https://github.com/go-chi/jwtauth)       | JWT authentication                                          |
| [hostrouter](https://github.com/go-chi/hostrouter) | Domain/host based request routing                           |
| [httplog](https://github.com/go-chi/httplog)       | Small but powerful structured HTTP request logging          |
| [httprate](https://github.com/go-chi/httprate)     | HTTP request rate limiter                                   |
| [httptracer](https://github.com/go-chi/httptracer) | HTTP request performance tracing library                    |
| [httpvcr](https://github.com/go-chi/httpvcr)       | Write deterministic tests for external sources              |
| [stampede](https://github.com/go-chi/stampede)     | HTTP request coalescer                                      |
--------------------------------------------------------------------------------------------------------------------


## context?

`context` is a tiny pkg that provides simple interface to signal context across call stacks
and goroutines. It was originally written by [Sameer Ajmani](https://github.com/Sajmani)
and is available in stdlib since go1.7.

Learn more at https://blog.golang.org/context

and..
* Docs: https://golang.org/pkg/context
* Source: https://github.com/golang/go/tree/master/src/context


## Benchmarks

The benchmark suite: https://github.com/pkieltyka/go-http-routing-benchmark

Results as of Nov 29, 2020 with Go 1.15.5 on Linux AMD 3950x

```shell
BenchmarkChi_Param          	3075895	        384 ns/op	      400 B/op      2 allocs/op
BenchmarkChi_Param5         	2116603	        566 ns/op	      400 B/op      2 allocs/op
BenchmarkChi_Param20        	 964117	       1227 ns/op	      400 B/op      2 allocs/op
BenchmarkChi_ParamWrite     	2863413	        420 ns/op	      400 B/op      2 allocs/op
BenchmarkChi_GithubStatic   	3045488	        395 ns/op	      400 B/op      2 allocs/op
BenchmarkChi_GithubParam    	2204115	        540 ns/op	      400 B/op      2 allocs/op
BenchmarkChi_GithubAll      	  10000	     113811 ns/op	    81203 B/op    406 allocs/op
BenchmarkChi_GPlusStatic    	3337485	        359 ns/op	      400 B/op      2 allocs/op
BenchmarkChi_GPlusParam     	2825853	        423 ns/op	      400 B/op      2 allocs/op
BenchmarkChi_GPlus2Params   	2471697	        483 ns/op	      400 B/op      2 allocs/op
BenchmarkChi_GPlusAll       	 194220	       5950 ns/op	     5200 B/op     26 allocs/op
BenchmarkChi_ParseStatic    	3365324	        356 ns/op	      400 B/op      2 allocs/op
BenchmarkChi_ParseParam     	2976614	        404 ns/op	      400 B/op      2 allocs/op
BenchmarkChi_Parse2Params   	2638084	        439 ns/op	      400 B/op      2 allocs/op
BenchmarkChi_ParseAll       	 109567	      11295 ns/op	    10400 B/op     52 allocs/op
BenchmarkChi_StaticAll      	  16846	      71308 ns/op	    62802 B/op    314 allocs/op
```

Comparison with other routers: https://gist.github.com/pkieltyka/123032f12052520aaccab752bd3e78cc

NOTE: the allocs in the benchmark above are from the calls to http.Request's
`WithContext(context.Context)` method that clones the http.Request, sets the `Context()`
on the duplicated (alloc'd) request and returns it the new request object. This is just
how setting context on a request in Go works.


## Credits

* Carl Jackson for https://github.com/zenazn/goji
  * Parts of chi's thinking comes from goji, and chi's middleware package
    sources from goji.
* Armon Dadgar for https://github.com/armon/go-radix
* Contributions: [@VojtechVitek](https://github.com/VojtechVitek)

We'll be more than happy to see [your contributions](./CONTRIBUTING.md)!


## Beyond REST

chi is just a http router that lets you decompose request handling into many smaller layers.
Many companies use chi to write REST services for their public APIs. But, REST is just a convention
for managing state via HTTP, and there's a lot of other pieces required to write a complete client-server
system or network of microservices.

Looking beyond REST, I also recommend some newer works in the field:
* [webrpc](https://github.com/webrpc/webrpc) - Web-focused RPC client+server framework with code-gen
* [gRPC](https://github.com/grpc/grpc-go) - Google's RPC framework via protobufs
* [graphql](https://github.com/99designs/gqlgen) - Declarative query language
* [NATS](https://nats.io) - lightweight pub-sub


## License

Copyright (c) 2015-present [Peter Kieltyka](https://github.com/pkieltyka)

Licensed under [MIT License](./LICENSE)

[GoDoc]: https://pkg.go.dev/github.com/go-chi/chi?tab=versions
[GoDoc Widget]: https://godoc.org/github.com/go-chi/chi?status.svg
[Travis]: https://travis-ci.org/go-chi/chi
[Travis Widget]: https://travis-ci.org/go-chi/chi.svg?branch=master
