# goquery - a little like that j-thing, only in Go

[![Build Status](https://github.com/PuerkitoBio/goquery/actions/workflows/test.yml/badge.svg?branch=master)](https://github.com/PuerkitoBio/goquery/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/PuerkitoBio/goquery.svg)](https://pkg.go.dev/github.com/PuerkitoBio/goquery)
[![Sourcegraph Badge](https://sourcegraph.com/github.com/PuerkitoBio/goquery/-/badge.svg)](https://sourcegraph.com/github.com/PuerkitoBio/goquery?badge)

goquery brings a syntax and a set of features similar to [jQuery][] to the [Go language][go]. It is based on Go's [net/html package][html] and the CSS Selector library [cascadia][]. Since the net/html parser returns nodes, and not a full-featured DOM tree, jQuery's stateful manipulation functions (like height(), css(), detach()) have been left off.

Also, because the net/html parser requires UTF-8 encoding, so does goquery: it is the caller's responsibility to ensure that the source document provides UTF-8 encoded HTML. See the [wiki][] for various options to do this.

Syntax-wise, it is as close as possible to jQuery, with the same function names when possible, and that warm and fuzzy chainable interface. jQuery being the ultra-popular library that it is, I felt that writing a similar HTML-manipulating library was better to follow its API than to start anew (in the same spirit as Go's `fmt` package), even though some of its methods are less than intuitive (looking at you, [index()][index]...).

## Table of Contents

* [Installation](#installation)
* [Changelog](#changelog)
* [API](#api)
* [Examples](#examples)
* [Related Projects](#related-projects)
* [Support](#support)
* [License](#license)

## Installation

Please note that because of the net/html dependency, goquery requires Go1.1+ and is tested on Go1.7+.

    $ go get github.com/PuerkitoBio/goquery

(optional) To run unit tests:

    $ cd $GOPATH/src/github.com/PuerkitoBio/goquery
    $ go test

(optional) To run benchmarks (warning: it runs for a few minutes):

    $ cd $GOPATH/src/github.com/PuerkitoBio/goquery
    $ go test -bench=".*"

## Changelog

**Note that goquery's API is now stable, and will not break.**

*    **2023-02-18 (v1.8.1)** : Update `go.mod` dependencies, update CI workflow.
*    **2021-10-25 (v1.8.0)** : Add `Render` function to render a `Selection` to an `io.Writer` (thanks [@anthonygedeon](https://github.com/anthonygedeon)).
*    **2021-07-11 (v1.7.1)** : Update go.mod dependencies and add dependabot config (thanks [@jauderho](https://github.com/jauderho)).
*    **2021-06-14 (v1.7.0)** : Add `Single` and `SingleMatcher` functions to optimize first-match selection (thanks [@gdollardollar](https://github.com/gdollardollar)).
*    **2021-01-11 (v1.6.1)** : Fix panic when calling `{Prepend,Append,Set}Html` on a `Selection` that contains non-Element nodes.
*    **2020-10-08 (v1.6.0)** : Parse html in context of the container node for all functions that deal with html strings (`AfterHtml`, `AppendHtml`, etc.). Thanks to [@thiemok][thiemok] and [@davidjwilkins][djw] for their work on this.
*    **2020-02-04 (v1.5.1)** : Update module dependencies.
*    **2018-11-15 (v1.5.0)** : Go module support (thanks @Zaba505).
*    **2018-06-07 (v1.4.1)** : Add `NewDocumentFromReader` examples.
*    **2018-03-24 (v1.4.0)** : Deprecate `NewDocument(url)` and `NewDocumentFromResponse(response)`.
*    **2018-01-28 (v1.3.0)** : Add `ToEnd` constant to `Slice` until the end of the selection (thanks to @davidjwilkins for raising the issue).
*    **2018-01-11 (v1.2.0)** : Add `AddBack*` and deprecate `AndSelf` (thanks to @davidjwilkins).
*    **2017-02-12 (v1.1.0)** : Add `SetHtml` and `SetText` (thanks to @glebtv).
*    **2016-12-29 (v1.0.2)** : Optimize allocations for `Selection.Text` (thanks to @radovskyb).
*    **2016-08-28 (v1.0.1)** : Optimize performance for large documents.
*    **2016-07-27 (v1.0.0)** : Tag version 1.0.0.
*    **2016-06-15** : Invalid selector strings internally compile to a `Matcher` implementation that never matches any node (instead of a panic). So for example, `doc.Find("~")` returns an empty `*Selection` object.
*    **2016-02-02** : Add `NodeName` utility function similar to the DOM's `nodeName` property. It returns the tag name of the first element in a selection, and other relevant values of non-element nodes (see [doc][] for details). Add `OuterHtml` utility function similar to the DOM's `outerHTML` property (named `OuterHtml` in small caps for consistency with the existing `Html` method on the `Selection`).
*    **2015-04-20** : Add `AttrOr` helper method to return the attribute's value or a default value if absent. Thanks to [piotrkowalczuk][piotr].
*    **2015-02-04** : Add more manipulation functions - Prepend* - thanks again to [Andrew Stone][thatguystone].
*    **2014-11-28** : Add more manipulation functions - ReplaceWith*, Wrap* and Unwrap - thanks again to [Andrew Stone][thatguystone].
*    **2014-11-07** : Add manipulation functions (thanks to [Andrew Stone][thatguystone]) and `*Matcher` functions, that receive compiled cascadia selectors instead of selector strings, thus avoiding potential panics thrown by goquery via `cascadia.MustCompile` calls. This results in better performance (selectors can be compiled once and reused) and more idiomatic error handling (you can handle cascadia's compilation errors, instead of recovering from panics, which had been bugging me for a long time). Note that the actual type expected is a `Matcher` interface, that `cascadia.Selector` implements. Other matcher implementations could be used.
*    **2014-11-06** : Change import paths of net/html to golang.org/x/net/html (see https://groups.google.com/forum/#!topic/golang-nuts/eD8dh3T9yyA). Make sure to update your code to use the new import path too when you call goquery with `html.Node`s.
*    **v0.3.2** : Add `NewDocumentFromReader()` (thanks jweir) which allows creating a goquery document from an io.Reader.
*    **v0.3.1** : Add `NewDocumentFromResponse()` (thanks assassingj) which allows creating a goquery document from an http response.
*    **v0.3.0** : Add `EachWithBreak()` which allows to break out of an `Each()` loop by returning false. This function was added instead of changing the existing `Each()` to avoid breaking compatibility.
*    **v0.2.1** : Make go-getable, now that [go.net/html is Go1.0-compatible][gonet] (thanks to @matrixik for pointing this out).
*    **v0.2.0** : Add support for negative indices in Slice(). **BREAKING CHANGE** `Document.Root` is removed, `Document` is now a `Selection` itself (a selection of one, the root element, just like `Document.Root` was before). Add jQuery's Closest() method.
*    **v0.1.1** : Add benchmarks to use as baseline for refactorings, refactor Next...() and Prev...() methods to use the new html package's linked list features (Next/PrevSibling, FirstChild). Good performance boost (40+% in some cases).
*    **v0.1.0** : Initial release.

## API

goquery exposes two structs, `Document` and `Selection`, and the `Matcher` interface. Unlike jQuery, which is loaded as part of a DOM document, and thus acts on its containing document, goquery doesn't know which HTML document to act upon. So it needs to be told, and that's what the `Document` type is for. It holds the root document node as the initial Selection value to manipulate.

jQuery often has many variants for the same function (no argument, a selector string argument, a jQuery object argument, a DOM element argument, ...). Instead of exposing the same features in goquery as a single method with variadic empty interface arguments, statically-typed signatures are used following this naming convention:

*    When the jQuery equivalent can be called with no argument, it has the same name as jQuery for the no argument signature (e.g.: `Prev()`), and the version with a selector string argument is called `XxxFiltered()` (e.g.: `PrevFiltered()`)
*    When the jQuery equivalent **requires** one argument, the same name as jQuery is used for the selector string version (e.g.: `Is()`)
*    The signatures accepting a jQuery object as argument are defined in goquery as `XxxSelection()` and take a `*Selection` object as argument (e.g.: `FilterSelection()`)
*    The signatures accepting a DOM element as argument in jQuery are defined in goquery as `XxxNodes()` and take a variadic argument of type `*html.Node` (e.g.: `FilterNodes()`)
*    The signatures accepting a function as argument in jQuery are defined in goquery as `XxxFunction()` and take a function as argument (e.g.: `FilterFunction()`)
*    The goquery methods that can be called with a selector string have a corresponding version that take a `Matcher` interface and are defined as `XxxMatcher()` (e.g.: `IsMatcher()`)

Utility functions that are not in jQuery but are useful in Go are implemented as functions (that take a `*Selection` as parameter), to avoid a potential naming clash on the `*Selection`'s methods (reserved for jQuery-equivalent behaviour).

The complete [package reference documentation can be found here][doc].

Please note that Cascadia's selectors do not necessarily match all supported selectors of jQuery (Sizzle). See the [cascadia project][cascadia] for details. Invalid selector strings compile to a `Matcher` that fails to match any node. Behaviour of the various functions that take a selector string as argument follows from that fact, e.g. (where `~` is an invalid selector string):

* `Find("~")` returns an empty selection because the selector string doesn't match anything.
* `Add("~")` returns a new selection that holds the same nodes as the original selection, because it didn't add any node (selector string didn't match anything).
* `ParentsFiltered("~")` returns an empty selection because the selector string doesn't match anything.
* `ParentsUntil("~")` returns all parents of the selection because the selector string didn't match any element to stop before the top element.

## Examples

See some tips and tricks in the [wiki][].

Adapted from example_test.go:

```Go
package main

import (
  "fmt"
  "log"
  "net/http"

  "github.com/PuerkitoBio/goquery"
)

func ExampleScrape() {
  // Request the HTML page.
  res, err := http.Get("http://metalsucks.net")
  if err != nil {
    log.Fatal(err)
  }
  defer res.Body.Close()
  if res.StatusCode != 200 {
    log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
  }

  // Load the HTML document
  doc, err := goquery.NewDocumentFromReader(res.Body)
  if err != nil {
    log.Fatal(err)
  }

  // Find the review items
  doc.Find(".left-content article .post-title").Each(func(i int, s *goquery.Selection) {
		// For each item found, get the title
		title := s.Find("a").Text()
		fmt.Printf("Review %d: %s\n", i, title)
	})
}

func main() {
  ExampleScrape()
}
```

## Related Projects

- [Goq][goq], an HTML deserialization and scraping library based on goquery and struct tags.
- [andybalholm/cascadia][cascadia], the CSS selector library used by goquery.
- [suntong/cascadia][cascadiacli], a command-line interface to the cascadia CSS selector library, useful to test selectors.
- [gocolly/colly](https://github.com/gocolly/colly), a lightning fast and elegant Scraping Framework
- [gnulnx/goperf](https://github.com/gnulnx/goperf), a website performance test tool that also fetches static assets.
- [MontFerret/ferret](https://github.com/MontFerret/ferret), declarative web scraping.
- [tacusci/berrycms](https://github.com/tacusci/berrycms), a modern simple to use CMS with easy to write plugins
- [Dataflow kit](https://github.com/slotix/dataflowkit), Web Scraping framework for Gophers.
- [Geziyor](https://github.com/geziyor/geziyor), a fast web crawling & scraping framework for Go. Supports JS rendering.
- [Pagser](https://github.com/foolin/pagser), a simple, easy, extensible, configurable HTML parser to struct based on goquery and struct tags.
- [stitcherd](https://github.com/vhodges/stitcherd), A server for doing server side includes using css selectors and DOM updates.
- [goskyr](https://github.com/jakopako/goskyr), an easily configurable command-line scraper written in Go.
- [goGetJS](https://github.com/davemolk/goGetJS), a tool for extracting, searching, and saving JavaScript files (with optional headless browser).

## Support

There are a number of ways you can support the project:

* Use it, star it, build something with it, spread the word!
  - If you do build something open-source or otherwise publicly-visible, let me know so I can add it to the [Related Projects](#related-projects) section!
* Raise issues to improve the project (note: doc typos and clarifications are issues too!)
  - Please search existing issues before opening a new one - it may have already been adressed.
* Pull requests: please discuss new code in an issue first, unless the fix is really trivial.
  - Make sure new code is tested.
  - Be mindful of existing code - PRs that break existing code have a high probability of being declined, unless it fixes a serious issue.
* Sponsor the developer
  - See the Github Sponsor button at the top of the repo on github
  - or via BuyMeACoffee.com, below

<a href="https://www.buymeacoffee.com/mna" target="_blank"><img src="https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png" alt="Buy Me A Coffee" style="height: 41px !important;width: 174px !important;box-shadow: 0px 3px 2px 0px rgba(190, 190, 190, 0.5) !important;-webkit-box-shadow: 0px 3px 2px 0px rgba(190, 190, 190, 0.5) !important;" ></a>

## License

The [BSD 3-Clause license][bsd], the same as the [Go language][golic]. Cascadia's license is [here][caslic].

[jquery]: http://jquery.com/
[go]: http://golang.org/
[cascadia]: https://github.com/andybalholm/cascadia
[cascadiacli]: https://github.com/suntong/cascadia
[bsd]: http://opensource.org/licenses/BSD-3-Clause
[golic]: http://golang.org/LICENSE
[caslic]: https://github.com/andybalholm/cascadia/blob/master/LICENSE
[doc]: https://pkg.go.dev/github.com/PuerkitoBio/goquery
[index]: http://api.jquery.com/index/
[gonet]: https://github.com/golang/net/
[html]: https://pkg.go.dev/golang.org/x/net/html
[wiki]: https://github.com/PuerkitoBio/goquery/wiki/Tips-and-tricks
[thatguystone]: https://github.com/thatguystone
[piotr]: https://github.com/piotrkowalczuk
[goq]: https://github.com/andrewstuart/goq
[thiemok]: https://github.com/thiemok
[djw]: https://github.com/davidjwilkins
