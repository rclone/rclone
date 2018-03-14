---
title: "Remote Control"
description: "Remote controlling rclone"
date: "2018-03-05"
---

# Remote controlling rclone #

If rclone is run with the `--rc` flag then it starts an http server
which can be used to remote control rclone.

## Supported parameters

#### --rc ####
Flag to start the http server listen on remote requests
      
#### --rc-addr=IP ####
IPaddress:Port or :Port to bind server to. (default "localhost:5572")

#### --rc-cert=KEY ####
SSL PEM key (concatenation of certificate and CA certificate)

#### --rc-client-ca=PATH ####
Client certificate authority to verify clients with

#### --rc-htpasswd=PATH ####
htpasswd file - if not provided no authentication is done

#### --rc-key=PATH ####
SSL PEM Private key

#### --rc-max-header-bytes=VALUE ####
Maximum size of request header (default 4096)

#### --rc-user=VALUE ####
User name for authentication.

#### --rc-pass=VALUE ####
Password for authentication.

#### --rc-realm=VALUE ####
Realm for authentication (default "rclone")

#### --rc-server-read-timeout=DURATION ####
Timeout for server reading data (default 1h0m0s)

#### --rc-server-write-timeout=DURATION ####
Timeout for server writing data (default 1h0m0s)

## Accessing the remote control via the rclone rc command

Rclone itself implements the remote control protocol in its `rclone
rc` command.

You can use it like this

## Accessing the remote control via HTTP

Rclone implements a simple HTTP based protocol.

Each endpoint takes an JSON object and returns a JSON object or an
error.  The JSON objects are essentially a map of string names to
values.

All calls must made using POST.

The input objects can be supplied using URL parameters, POST
parameters or by supplying "Content-Type: application/json" and a JSON
blob in the body.  There are examples of these below using `curl`.

The response will be a JSON blob in the body of the response.  This is
formatted to be reasonably human readable.

If an error occurs then there will be an HTTP error status (usually
400) and the body of the response will contain a JSON encoded error
object.

### Using POST with URL parameters only

```
curl -X POST 'http://localhost:5572/rc/noop/?potato=1&sausage=2'
```

Response

```
{
	"potato": "1",
	"sausage": "2"
}
```

Here is what an error response looks like:

```
curl -X POST 'http://localhost:5572/rc/error/?potato=1&sausage=2'
```

```
{
	"error": "arbitrary error on input map[potato:1 sausage:2]",
	"input": {
		"potato": "1",
		"sausage": "2"
	}
}
```

Note that curl doesn't return errors to the shell unless you use the `-f` option

```
$ curl -f -X POST 'http://localhost:5572/rc/error/?potato=1&sausage=2'
curl: (22) The requested URL returned error: 400 Bad Request
$ echo $?
22
```

### Using POST with a form

```
curl --data "potato=1" --data "sausage=2" http://localhost:5572/rc/noop/
```

Response

```
{
	"potato": "1",
	"sausage": "2"
}
```

Note that you can combine these with URL parameters too with the POST
parameters taking precedence.

```
curl --data "potato=1" --data "sausage=2" "http://localhost:5572/rc/noop/?rutabaga=3&sausage=4"
```

Response

```
{
	"potato": "1",
	"rutabaga": "3",
	"sausage": "4"
}

```

### Using POST with a JSON blob

```
curl -H "Content-Type: application/json" -X POST -d '{"potato":2,"sausage":1}' http://localhost:5572/rc/noop/
```

response

```
{
	"password": "xyz",
	"username": "xyz"
}
```

This can be combined with URL parameters too if required.  The JSON
blob takes precedence.

```
curl -H "Content-Type: application/json" -X POST -d '{"potato":2,"sausage":1}' 'http://localhost:5572/rc/noop/?rutabaga=3&potato=4'
```

```
{
	"potato": 2,
	"rutabaga": "3",
	"sausage": 1
}
```
