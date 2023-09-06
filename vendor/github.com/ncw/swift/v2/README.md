Swift
=====

This package provides an easy to use library for interfacing with Swift / Openstack Object Storage / Rackspace cloud
files from the Go Language

[![Build Status](https://github.com/ncw/swift/workflows/build/badge.svg?branch=master)](https://github.com/ncw/swift/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/ncw/v2/swift.svg)](https://pkg.go.dev/github.com/ncw/swift/v2)

Install
-------

Use go to install the library

    go get github.com/ncw/swift/v2

Usage
-----

See here for full package docs

- https://pkg.go.dev/github.com/ncw/swift/v2

Here is a short example from the docs

```go
import "github.com/ncw/swift/v2"

// Create a connection
c := swift.Connection{
UserName: "user",
ApiKey:   "key",
AuthUrl:  "auth_url",
Domain:   "domain", // Name of the domain (v3 auth only)
Tenant:   "tenant", // Name of the tenant (v2 auth only)
}
// Authenticate
err := c.Authenticate()
if err != nil {
panic(err)
}
// List all the containers
containers, err := c.ContainerNames(nil)
fmt.Println(containers)
// etc...
```

Migrating from `v1`
-----
The library has current major version v2. If you want to migrate from the first version of
library `github.com/ncw/swift` you have to explicitly add the `/v2` suffix to the imports.

Most of the exported functions were added a new `context.Context` parameter in the `v2`, which you will have to provide
when migrating.

Additions
---------

The `rs` sub project contains a wrapper for the Rackspace specific CDN Management interface.

Testing
-------

To run the tests you can either use an embedded fake Swift server either use a real Openstack Swift server or a
Rackspace Cloud files account.

When using a real Swift server, you need to set these environment variables before running the tests

    export SWIFT_API_USER='user'
    export SWIFT_API_KEY='key'
    export SWIFT_AUTH_URL='https://url.of.auth.server/v1.0'

And optionally these if using v2 authentication

    export SWIFT_TENANT='TenantName'
    export SWIFT_TENANT_ID='TenantId'

And optionally these if using v3 authentication

    export SWIFT_TENANT='TenantName'
    export SWIFT_TENANT_ID='TenantId'
    export SWIFT_API_DOMAIN_ID='domain id'
    export SWIFT_API_DOMAIN='domain name'

And optionally these if using v3 trust

    export SWIFT_TRUST_ID='TrustId'

And optionally this if you want to skip server certificate validation

    export SWIFT_AUTH_INSECURE=1

And optionally this to configure the connect channel timeout, in seconds

    export SWIFT_CONNECTION_CHANNEL_TIMEOUT=60

And optionally this to configure the data channel timeout, in seconds

    export SWIFT_DATA_CHANNEL_TIMEOUT=60

Then run the tests with `go test`

License
-------

This is free software under the terms of MIT license (check COPYING file included in this package).

Contact and support
-------------------

The project website is at:

- https://github.com/ncw/swift

There you can file bug reports, ask for help or contribute patches.

Authors
-------

- Nick Craig-Wood <nick@craig-wood.com>

Contributors
------------

- Brian "bojo" Jones <mojobojo@gmail.com>
- Janika Liiv <janika@toggl.com>
- Yamamoto, Hirotaka <ymmt2005@gmail.com>
- Stephen <yo@groks.org>
- platformpurple <stephen@platformpurple.com>
- Paul Querna <pquerna@apache.org>
- Livio Soares <liviobs@gmail.com>
- thesyncim <thesyncim@gmail.com>
- lsowen <lsowen@s1network.com> <logan@s1network.com>
- Sylvain Baubeau <sbaubeau@redhat.com>
- Chris Kastorff <encryptio@gmail.com>
- Dai HaoJun <haojun.dai@hp.com>
- Hua Wang <wanghua.humble@gmail.com>
- Fabian Ruff <fabian@progra.de> <fabian.ruff@sap.com>
- Arturo Reuschenbach Puncernau <reuschenbach@gmail.com>
- Petr Kotek <petr.kotek@bigcommerce.com>
- Stefan Majewsky <stefan.majewsky@sap.com> <majewsky@gmx.net>
- Cezar Sa Espinola <cezarsa@gmail.com>
- Sam Gunaratne <samgzeit@gmail.com>
- Richard Scothern <richard.scothern@gmail.com>
- Michel Couillard <!--<couillard.michel@voxlog.ca>--> <michel.couillard@gmail.com>
- Christopher Waldon <ckwaldon@us.ibm.com>
- dennis <dai.haojun@gmail.com>
- hag <hannes.georg@xing.com>
- Alexander Neumann <alexander@bumpern.de>
- eclipseo <30413512+eclipseo@users.noreply.github.com>
- Yuri Per <yuri@acronis.com>
- Falk Reimann <falk.reimann@sap.com>
- Arthur Paim Arnold <arthurpaimarnold@gmail.com>
- Bruno Michel <bmichel@menfin.info>
- Charles Hsu <charles0126@gmail.com>
- Omar Ali <omarali@users.noreply.github.com>
- Andreas Andersen <andreas@softwaredesign.se>
- kayrus <kay.diam@gmail.com>
- CodeLingo Bot <bot@codelingo.io>
- Jérémy Clerc <jeremy.clerc@tagpay.fr>
- 4xicom <37339705+4xicom@users.noreply.github.com>
- Bo <bo@4xi.com>
- Thiago da Silva <thiagodasilva@users.noreply.github.com>
- Brandon WELSCH <dev@brandon-welsch.eu>
- Damien Tournoud <damien@platform.sh>
- Pedro Kiefer <pedro@kiefer.com.br>
- Martin Chodur <m.chodur@seznam.cz>
- Devendra <devendranath.thadi3@gmail.com>
- timss <timsateroy@gmail.com>
- Jos Houtman <jos@houtman.it>
- Paul Collins <paul.collins@canonical.com>
- Joe Cai <joe.cai@bigcommerce.com>
