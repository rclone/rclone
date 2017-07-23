# Google APIs Client Library for Go

## Library maintenance
This client library is supported, but in maintenance mode only.
We are fixing necessary bugs and adding essential features to ensure this
library continues to meet your needs for accessing Google APIs.
Non-critical issues will be closed.
Any issue may be reopened if it is causing ongoing problems.

## Status
[![Build Status](https://travis-ci.org/google/google-api-go-client.png)](https://travis-ci.org/google/google-api-go-client)

These are auto-generated Go libraries from the Google Discovery Service's JSON description files of the available "new style" Google APIs.

Due to the auto-generated nature of this collection of libraries, complete APIs or specific versions can appear or go away without notice.
As a result, you should always locally vendor any API(s) that your code relies upon.

If you're working with Google Cloud Platform APIs such as Datastore or Pub/Sub,
consider using the
[Cloud Client Libraries for Go](https://github.com/GoogleCloudPlatform/google-cloud-go)
instead. These are the new and
idiomatic Go libraries targeted specifically at Google Cloud Platform Services.

The generator itself and the code it produces are beta. Some APIs are
alpha/beta, and indicated as such in the import path (e.g.,
"google.golang.org/api/someapi/v1alpha").

Announcement email:

* http://groups.google.com/group/golang-nuts/browse_thread/thread/6c7281450be9a21e

Getting started documentation:

* https://github.com/google/google-api-go-client/blob/master/GettingStarted.md

In summary:

```
$ go get google.golang.org/api/storage/v1
$ go get google.golang.org/api/tasks/v1
$ go get google.golang.org/api/moderator/v1
... etc ...
```

For docs, see e.g.:

* https://godoc.org/google.golang.org/api/storage/v1

The package of a given import is the second-to-last component, before the version number.

For examples, see:

* https://github.com/google/google-api-go-client/tree/master/examples

For support, use the golang-nuts@ mailing list:

* https://groups.google.com/group/golang-nuts

## Application Default Credentials Example

Application Default Credentials provide a simplified way to obtain credentials
for authenticating with Google APIs.

The Application Default Credentials authenticate as the application itself,
which make them great for working with Google Cloud APIs like Storage or
Datastore. They are the recommended form of authentication when building
applications that run on Google Compute Engine or Google App Engine.

Default credentials are provided by the `golang.org/x/oauth2/google` package. To use them, add the following import:

```
import "golang.org/x/oauth2/google"
```

Some credentials types require you to specify scopes, and service entry points may not inject them. If you encounter this situation you may need to specify scopes as follows:

```
import (
        "golang.org/x/net/context"
        "golang.org/x/oauth2/google"
        "google.golang.org/api/compute/v1"
)

func main() {
        // Use oauth2.NoContext if there isn't a good context to pass in.
        ctx := context.Background()

        client, err := google.DefaultClient(ctx, compute.ComputeScope)
        if err != nil {
                //...
        }
        computeService, err := compute.New(client)
        if err != nil {
                //...
        }
}
```

If you need a `oauth2.TokenSource`, use the `DefaultTokenSource` function:

```
ts, err := google.DefaultTokenSource(ctx, scope1, scope2, ...)
if err != nil {
        //...
}
client := oauth2.NewClient(ctx, ts)
```

See also: [golang.org/x/oauth2/google](https://godoc.org/golang.org/x/oauth2/google) package documentation.
