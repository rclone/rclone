// Copyright (c) 2012-2016, Martin Angers & Contributors
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without modification,
// are permitted provided that the following conditions are met:
//
// * Redistributions of source code must retain the above copyright notice,
// this list of conditions and the following disclaimer.
// * Redistributions in binary form must reproduce the above copyright notice,
// this list of conditions and the following disclaimer in the documentation and/or
// other materials provided with the distribution.
// * Neither the name of the author nor the names of its contributors may be used to
// endorse or promote products derived from this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS
// OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY
// AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR
// CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
// DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY,
// WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY
// WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

/*
Package goquery implements features similar to jQuery, including the chainable
syntax, to manipulate and query an HTML document.

It brings a syntax and a set of features similar to jQuery to the Go language.
It is based on Go's net/html package and the CSS Selector library cascadia.
Since the net/html parser returns nodes, and not a full-featured DOM
tree, jQuery's stateful manipulation functions (like height(), css(), detach())
have been left off.

Also, because the net/html parser requires UTF-8 encoding, so does goquery: it is
the caller's responsibility to ensure that the source document provides UTF-8 encoded HTML.
See the repository's wiki for various options on how to do this.

Syntax-wise, it is as close as possible to jQuery, with the same method names when
possible, and that warm and fuzzy chainable interface. jQuery being the
ultra-popular library that it is, writing a similar HTML-manipulating
library was better to follow its API than to start anew (in the same spirit as
Go's fmt package), even though some of its methods are less than intuitive (looking
at you, index()...).

It is hosted on GitHub, along with additional documentation in the README.md
file: https://github.com/puerkitobio/goquery

Please note that because of the net/html dependency, goquery requires Go1.1+.

The various methods are split into files based on the category of behavior.
The three dots (...) indicate that various "overloads" are available.

* array.go : array-like positional manipulation of the selection.
    - Eq()
    - First()
    - Get()
    - Index...()
    - Last()
    - Slice()

* expand.go : methods that expand or augment the selection's set.
    - Add...()
    - AndSelf()
    - Union(), which is an alias for AddSelection()

* filter.go : filtering methods, that reduce the selection's set.
    - End()
    - Filter...()
    - Has...()
    - Intersection(), which is an alias of FilterSelection()
    - Not...()

* iteration.go : methods to loop over the selection's nodes.
    - Each()
    - EachWithBreak()
    - Map()

* manipulation.go : methods for modifying the document
    - After...()
    - Append...()
    - Before...()
    - Clone()
    - Empty()
    - Prepend...()
    - Remove...()
    - ReplaceWith...()
    - Unwrap()
    - Wrap...()
    - WrapAll...()
    - WrapInner...()

* property.go : methods that inspect and get the node's properties values.
    - Attr*(), RemoveAttr(), SetAttr()
    - AddClass(), HasClass(), RemoveClass(), ToggleClass()
    - Html()
    - Length()
    - Size(), which is an alias for Length()
    - Text()

* query.go : methods that query, or reflect, a node's identity.
    - Contains()
    - Is...()

* traversal.go : methods to traverse the HTML document tree.
    - Children...()
    - Contents()
    - Find...()
    - Next...()
    - Parent[s]...()
    - Prev...()
    - Siblings...()

* type.go : definition of the types exposed by goquery.
    - Document
    - Selection
    - Matcher

* utilities.go : definition of helper functions (and not methods on a *Selection)
that are not part of jQuery, but are useful to goquery.
    - NodeName
    - OuterHtml
*/
package goquery
