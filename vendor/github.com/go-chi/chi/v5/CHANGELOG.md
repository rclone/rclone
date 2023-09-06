# Changelog

## v5.0.10 (2023-07-13)

- Fixed small edge case in tests of v5.0.9 for older Go versions
- History of changes: see https://github.com/go-chi/chi/compare/v5.0.8...v5.0.10


## v5.0.9 (2023-07-13)

- History of changes: see https://github.com/go-chi/chi/compare/v5.0.8...v5.0.9


## v5.0.8 (2022-12-07)

- History of changes: see https://github.com/go-chi/chi/compare/v5.0.7...v5.0.8


## v5.0.7 (2021-11-18)

- History of changes: see https://github.com/go-chi/chi/compare/v5.0.6...v5.0.7


## v5.0.6 (2021-11-15)

- History of changes: see https://github.com/go-chi/chi/compare/v5.0.5...v5.0.6


## v5.0.5 (2021-10-27)

- History of changes: see https://github.com/go-chi/chi/compare/v5.0.4...v5.0.5


## v5.0.4 (2021-08-29)

- History of changes: see https://github.com/go-chi/chi/compare/v5.0.3...v5.0.4


## v5.0.3 (2021-04-29)

- History of changes: see https://github.com/go-chi/chi/compare/v5.0.2...v5.0.3


## v5.0.2 (2021-03-25)

- History of changes: see https://github.com/go-chi/chi/compare/v5.0.1...v5.0.2


## v5.0.1 (2021-03-10)

- Small improvements
- History of changes: see https://github.com/go-chi/chi/compare/v5.0.0...v5.0.1


## v5.0.0 (2021-02-27)

- chi v5, `github.com/go-chi/chi/v5` introduces the adoption of Go's SIV to adhere to the current state-of-the-tools in Go.
- chi v1.5.x did not work out as planned, as the Go tooling is too powerful and chi's adoption is too wide.
  The most responsible thing to do for everyone's benefit is to just release v5 with SIV, so I present to you all,
  chi v5 at `github.com/go-chi/chi/v5`. I hope someday the developer experience and ergonomics I've been seeking
  will still come to fruition in some form, see https://github.com/golang/go/issues/44550
- History of changes: see https://github.com/go-chi/chi/compare/v1.5.4...v5.0.0


## v1.5.4 (2021-02-27)

- Undo prior retraction in v1.5.3 as we prepare for v5.0.0 release
- History of changes: see https://github.com/go-chi/chi/compare/v1.5.3...v1.5.4


## v1.5.3 (2021-02-21)

- Update go.mod to go 1.16 with new retract directive marking all versions without prior go.mod support
- History of changes: see https://github.com/go-chi/chi/compare/v1.5.2...v1.5.3


## v1.5.2 (2021-02-10)

- Reverting allocation optimization as a precaution as go test -race fails.
- Minor improvements, see history below
- History of changes: see https://github.com/go-chi/chi/compare/v1.5.1...v1.5.2


## v1.5.1 (2020-12-06)

- Performance improvement: removing 1 allocation by foregoing context.WithValue, thank you @bouk for
  your contribution (https://github.com/go-chi/chi/pull/555). Note: new benchmarks posted in README.
- `middleware.CleanPath`: new middleware that clean's request path of double slashes
- deprecate & remove `chi.ServerBaseContext` in favour of stdlib `http.Server#BaseContext`
- plus other tiny improvements, see full commit history below
- History of changes: see https://github.com/go-chi/chi/compare/v4.1.2...v1.5.1


## v1.5.0 (2020-11-12) - now with go.mod support

`chi` dates back to 2016 with it's original implementation as one of the first routers to adopt the newly introduced
context.Context api to the stdlib -- set out to design a router that is faster, more modular and simpler than anything
else out there -- while not introducing any custom handler types or dependencies. Today, `chi` still has zero dependencies,
and in many ways is future proofed from changes, given it's minimal nature. Between versions, chi's iterations have been very
incremental, with the architecture and api being the same today as it was originally designed in 2016. For this reason it 
makes chi a pretty easy project to maintain, as well thanks to the many amazing community contributions over the years
to who all help make chi better (total of 86 contributors to date -- thanks all!).

Chi has been a labour of love, art and engineering, with the goals to offer beautiful ergonomics, flexibility, performance
and simplicity when building HTTP services with Go. I've strived to keep the router very minimal in surface area / code size,
and always improving the code wherever possible -- and as of today the `chi` package is just 1082 lines of code (not counting
middlewares, which are all optional). As well, I don't have the exact metrics, but from my analysis and email exchanges from
companies and developers, chi is used by thousands of projects around the world -- thank you all as there is no better form of
joy for me than to have art I had started be helpful and enjoyed by others. And of course I use chi in all of my own projects too :)

For me, the aesthetics of chi's code and usage are very important. With the introduction of Go's module support
(which I'm a big fan of), chi's past versioning scheme choice to v2, v3 and v4 would mean I'd require the import path
of "github.com/go-chi/chi/v4", leading to the lengthy discussion at https://github.com/go-chi/chi/issues/462.
Haha, to some, you may be scratching your head why I've spent > 1 year stalling to adopt "/vXX" convention in the import
path -- which isn't horrible in general -- but for chi, I'm unable to accept it as I strive for perfection in it's API design,
aesthetics and simplicity. It just doesn't feel good to me given chi's simple nature -- I do not foresee a "v5" or "v6",
and upgrading between versions in the future will also be just incremental.

I do understand versioning is a part of the API design as well, which is why the solution for a while has been to "do nothing",
as Go supports both old and new import paths with/out go.mod. However, now that Go module support has had time to iron out kinks and
is adopted everywhere, it's time for chi to get with the times. Luckily, I've discovered a path forward that will make me happy,
while also not breaking anyone's app who adopted a prior versioning from tags in v2/v3/v4. I've made an experimental release of
v1.5.0 with go.mod silently, and tested it with new and old projects, to ensure the developer experience is preserved, and it's
largely unnoticed. Fortunately, Go's toolchain will check the tags of a repo and consider the "latest" tag the one with go.mod.
However, you can still request a specific older tag such as v4.1.2, and everything will "just work". But new users can just
`go get github.com/go-chi/chi` or `go get github.com/go-chi/chi@latest` and they will get the latest version which contains
go.mod support, which is v1.5.0+. `chi` will not change very much over the years, just like it hasn't changed much from 4 years ago.
Therefore, we will stay on v1.x from here on, starting from v1.5.0. Any breaking changes will bump a "minor" release and
backwards-compatible improvements/fixes will bump a "tiny" release.

For existing projects who want to upgrade to the latest go.mod version, run: `go get -u github.com/go-chi/chi@v1.5.0`,
which will get you on the go.mod version line (as Go's mod cache may still remember v4.x). Brand new systems can run
`go get -u github.com/go-chi/chi` or `go get -u github.com/go-chi/chi@latest` to install chi, which will install v1.5.0+
built with go.mod support.

My apologies to the developers who will disagree with the decisions above, but, hope you'll try it and see it's a very
minor request which is backwards compatible and won't break your existing installations.

Cheers all, happy coding!


---


## v4.1.2 (2020-06-02)

- fix that handles MethodNotAllowed with path variables, thank you @caseyhadden for your contribution
- fix to replace nested wildcards correctly in RoutePattern, thank you @@unmultimedio for your contribution
- History of changes: see https://github.com/go-chi/chi/compare/v4.1.1...v4.1.2


## v4.1.1 (2020-04-16)

- fix for issue https://github.com/go-chi/chi/issues/411 which allows for overlapping regexp
  route to the correct handler through a recursive tree search, thanks to @Jahaja for the PR/fix!
- new middleware.RouteHeaders as a simple router for request headers with wildcard support
- History of changes: see https://github.com/go-chi/chi/compare/v4.1.0...v4.1.1


## v4.1.0 (2020-04-1)

- middleware.LogEntry: Write method on interface now passes the response header
  and an extra interface type useful for custom logger implementations.
- middleware.WrapResponseWriter: minor fix
- middleware.Recoverer: a bit prettier
- History of changes: see https://github.com/go-chi/chi/compare/v4.0.4...v4.1.0

## v4.0.4 (2020-03-24)

- middleware.Recoverer: new pretty stack trace printing (https://github.com/go-chi/chi/pull/496)
- a few minor improvements and fixes
- History of changes: see https://github.com/go-chi/chi/compare/v4.0.3...v4.0.4


## v4.0.3 (2020-01-09)

- core: fix regexp routing to include default value when param is not matched
- middleware: rewrite of middleware.Compress
- middleware: suppress http.ErrAbortHandler in middleware.Recoverer
- History of changes: see https://github.com/go-chi/chi/compare/v4.0.2...v4.0.3


## v4.0.2 (2019-02-26)

- Minor fixes
- History of changes: see https://github.com/go-chi/chi/compare/v4.0.1...v4.0.2


## v4.0.1 (2019-01-21)

- Fixes issue with compress middleware: #382 #385
- History of changes: see https://github.com/go-chi/chi/compare/v4.0.0...v4.0.1


## v4.0.0 (2019-01-10)

- chi v4 requires Go 1.10.3+ (or Go 1.9.7+) - we have deprecated support for Go 1.7 and 1.8
- router: respond with 404 on router with no routes (#362)
- router: additional check to ensure wildcard is at the end of a url pattern (#333)
- middleware: deprecate use of http.CloseNotifier (#347)
- middleware: fix RedirectSlashes to include query params on redirect (#334)
- History of changes: see https://github.com/go-chi/chi/compare/v3.3.4...v4.0.0


## v3.3.4 (2019-01-07)

- Minor middleware improvements. No changes to core library/router. Moving v3 into its
- own branch as a version of chi for Go 1.7, 1.8, 1.9, 1.10, 1.11
- History of changes: see https://github.com/go-chi/chi/compare/v3.3.3...v3.3.4


## v3.3.3 (2018-08-27)

- Minor release
- See https://github.com/go-chi/chi/compare/v3.3.2...v3.3.3


## v3.3.2 (2017-12-22)

- Support to route trailing slashes on mounted sub-routers (#281)
- middleware: new `ContentCharset` to check matching charsets. Thank you
  @csucu for your community contribution!


## v3.3.1 (2017-11-20)

- middleware: new `AllowContentType` handler for explicit whitelist of accepted request Content-Types
- middleware: new `SetHeader` handler for short-hand middleware to set a response header key/value
- Minor bug fixes


## v3.3.0 (2017-10-10)

- New chi.RegisterMethod(method) to add support for custom HTTP methods, see _examples/custom-method for usage
- Deprecated LINK and UNLINK methods from the default list, please use `chi.RegisterMethod("LINK")` and `chi.RegisterMethod("UNLINK")` in an `init()` function


## v3.2.1 (2017-08-31)

- Add new `Match(rctx *Context, method, path string) bool` method to `Routes` interface
  and `Mux`. Match searches the mux's routing tree for a handler that matches the method/path
- Add new `RouteMethod` to `*Context`
- Add new `Routes` pointer to `*Context`
- Add new `middleware.GetHead` to route missing HEAD requests to GET handler
- Updated benchmarks (see README)


## v3.1.5 (2017-08-02)

- Setup golint and go vet for the project
- As per golint, we've redefined `func ServerBaseContext(h http.Handler, baseCtx context.Context) http.Handler`
  to `func ServerBaseContext(baseCtx context.Context, h http.Handler) http.Handler`


## v3.1.0 (2017-07-10)

- Fix a few minor issues after v3 release
- Move `docgen` sub-pkg to https://github.com/go-chi/docgen
- Move `render` sub-pkg to https://github.com/go-chi/render
- Add new `URLFormat` handler to chi/middleware sub-pkg to make working with url mime 
  suffixes easier, ie. parsing `/articles/1.json` and `/articles/1.xml`. See comments in
  https://github.com/go-chi/chi/blob/master/middleware/url_format.go for example usage.


## v3.0.0 (2017-06-21)

- Major update to chi library with many exciting updates, but also some *breaking changes*
- URL parameter syntax changed from `/:id` to `/{id}` for even more flexible routing, such as
  `/articles/{month}-{day}-{year}-{slug}`, `/articles/{id}`, and `/articles/{id}.{ext}` on the
  same router
- Support for regexp for routing patterns, in the form of `/{paramKey:regExp}` for example:
  `r.Get("/articles/{name:[a-z]+}", h)` and `chi.URLParam(r, "name")`
- Add `Method` and `MethodFunc` to `chi.Router` to allow routing definitions such as
  `r.Method("GET", "/", h)` which provides a cleaner interface for custom handlers like
  in `_examples/custom-handler`
- Deprecating `mux#FileServer` helper function. Instead, we encourage users to create their
  own using file handler with the stdlib, see `_examples/fileserver` for an example
- Add support for LINK/UNLINK http methods via `r.Method()` and `r.MethodFunc()`
- Moved the chi project to its own organization, to allow chi-related community packages to
  be easily discovered and supported, at: https://github.com/go-chi
- *NOTE:* please update your import paths to `"github.com/go-chi/chi"`
- *NOTE:* chi v2 is still available at https://github.com/go-chi/chi/tree/v2


## v2.1.0 (2017-03-30)

- Minor improvements and update to the chi core library
- Introduced a brand new `chi/render` sub-package to complete the story of building
  APIs to offer a pattern for managing well-defined request / response payloads. Please
  check out the updated `_examples/rest` example for how it works.
- Added `MethodNotAllowed(h http.HandlerFunc)` to chi.Router interface


## v2.0.0 (2017-01-06)

- After many months of v2 being in an RC state with many companies and users running it in
  production, the inclusion of some improvements to the middlewares, we are very pleased to
  announce v2.0.0 of chi.


## v2.0.0-rc1 (2016-07-26)

- Huge update! chi v2 is a large refactor targeting Go 1.7+. As of Go 1.7, the popular
  community `"net/context"` package has been included in the standard library as `"context"` and
  utilized by `"net/http"` and `http.Request` to managing deadlines, cancelation signals and other
  request-scoped values. We're very excited about the new context addition and are proud to
  introduce chi v2, a minimal and powerful routing package for building large HTTP services,
  with zero external dependencies. Chi focuses on idiomatic design and encourages the use of 
  stdlib HTTP handlers and middlwares.
- chi v2 deprecates its `chi.Handler` interface and requires `http.Handler` or `http.HandlerFunc`
- chi v2 stores URL routing parameters and patterns in the standard request context: `r.Context()`
- chi v2 lower-level routing context is accessible by `chi.RouteContext(r.Context()) *chi.Context`,
  which provides direct access to URL routing parameters, the routing path and the matching
  routing patterns.
- Users upgrading from chi v1 to v2, need to:
  1. Update the old chi.Handler signature, `func(ctx context.Context, w http.ResponseWriter, r *http.Request)` to
     the standard http.Handler: `func(w http.ResponseWriter, r *http.Request)`
  2. Use `chi.URLParam(r *http.Request, paramKey string) string`
     or `URLParamFromCtx(ctx context.Context, paramKey string) string` to access a url parameter value


## v1.0.0 (2016-07-01)

- Released chi v1 stable https://github.com/go-chi/chi/tree/v1.0.0 for Go 1.6 and older.


## v0.9.0 (2016-03-31)

- Reuse context objects via sync.Pool for zero-allocation routing [#33](https://github.com/go-chi/chi/pull/33)
- BREAKING NOTE: due to subtle API changes, previously `chi.URLParams(ctx)["id"]` used to access url parameters
  has changed to: `chi.URLParam(ctx, "id")`
