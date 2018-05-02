# Dropbox SDK for Go [UNOFFICIAL] [![GoDoc](https://godoc.org/github.com/dropbox/dropbox-sdk-go-unofficial/dropbox?status.svg)](https://godoc.org/github.com/dropbox/dropbox-sdk-go-unofficial/dropbox) [![Build Status](https://travis-ci.org/dropbox/dropbox-sdk-go-unofficial.svg?branch=master)](https://travis-ci.org/dropbox/dropbox-sdk-go-unofficial)

An **UNOFFICIAL** Go SDK for integrating with the Dropbox API v2. Tested with Go 1.5+

:warning: WARNING: This SDK is **NOT yet official**. What does this mean?

  * There is no formal Dropbox [support](https://www.dropbox.com/developers/support) for this SDK at this point
  * Bugs may or may not get fixed
  * Not all SDK features may be implemented and implemented features may be buggy or incorrect


### Uh OK, so why are you releasing this?

  * the SDK, while unofficial, _is_ usable. See [dbxcli](https://github.com/dropbox/dbxcli) for an example application built using the SDK
  * we would like to get feedback from the community and evaluate the level of interest/enthusiasm before investing into official supporting one more SDK

## Installation

```sh
$ go get github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/...
```

For most applications, you should just import the relevant namespace(s) only. The SDK exports the following sub-packages:

* `github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/auth`
* `github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/files`
* `github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/sharing`
* `github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/team`
* `github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/users`

Additionally, the base `github.com/dropbox/dropbox-sdk-go-unofficial/dropbox` package exports some configuration and helper methods.

## Usage

First, you need to [register a new "app"](https://dropbox.com/developers/apps) to start making API requests. Once you have created an app, you can either use the SDK via an access token (useful for testing) or via the regular OAuth2 flow (recommended for production).

### Using OAuth token

Once you've created an app, you can get an access token from the app's console. Note that this token will only work for the Dropbox account the token is associated with.

```go
import "github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
import "github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/users"

func main() {
  config := dropbox.Config{
      Token: token,
      LogLevel: dropbox.LogInfo, // if needed, set the desired logging level. Default is off
  }
  dbx := users.New(config)
  // start making API calls
}
```

### Using OAuth2 flow

For this, you will need your `APP_KEY` and `APP_SECRET` from the developers console. Your app will then have to take users though the oauth flow, as part of which users will explicitly grant permissions to your app. At the end of this process, users will get a token that the app can then use for subsequent authentication. See [this](https://godoc.org/golang.org/x/oauth2#example-Config) for an example of oauth2 flow in Go.

Once you have the token, usage is same as above.

### Making API calls

Each Dropbox API takes in a request type and returns a response type. For instance, [/users/get_account](https://www.dropbox.com/developers/documentation/http/documentation#users-get_account) takes as input a `GetAccountArg` and returns a `BasicAccount`. The typical pattern for making API calls is:

  * Instantiate the argument via the `New*` convenience functions in the SDK
  * Invoke the API
  * Process the response (or handle error, as below)

Here's an example:

```go
  arg := users.NewGetAccountArg(accountId)
  if resp, err := dbx.GetAccount(arg); err != nil {
    return err
  }
  fmt.Printf("Name: %v", resp.Name)
```

### Error Handling

As described in the [API docs](https://www.dropbox.com/developers/documentation/http/documentation#error-handling), all HTTP errors _except_ 409 are returned as-is to the client (with a helpful text message where possible). In case of a 409, the SDK will return an endpoint-specific error as described in the API. This will be made available as `EndpointError` member in the error.

## Note on using the Teams API

To use the Team API, you will need to create a Dropbox Business App. The OAuth token from this app will _only_ work for the Team API.

Please read the [API docs](https://www.dropbox.com/developers/documentation/http/teams) carefully to appropriate secure your apps and tokens when using the Team API.

## Code Generation

This SDK is automatically generated using the public [Dropbox API spec](https://github.com/dropbox/dropbox-api-spec) and [Stone](https://github.com/dropbox/stone). See this [README](https://github.com/dropbox/dropbox-sdk-go-unofficial/blob/master/generator/README.md)
for more details on how code is generated. 

## Caveats

  * To re-iterate, this is an **UNOFFICIAL** SDK and thus has no official support from Dropbox
  * Only supports the v2 API. Parts of the v2 API are still in beta, and thus subject to change
  * This SDK itself is in beta, and so interfaces may change at any point
