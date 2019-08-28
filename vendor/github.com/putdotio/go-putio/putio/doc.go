// Package putio is the Put.io API v2 client for Go.
//
// The go-putio package does not directly handle authentication. Instead, when
// creating a new client, pass an http.Client that can handle authentication for
// you. The easiest and recommended way to do this is using the golang.org/x/oauth2
// library, but you can always use any other library that provides an http.Client.
//
// Note that when using an authenticated Client, all calls made by the client will
// include the specified OAuth token. Therefore, authenticated clients should
// almost never be shared between different users.
package putio
