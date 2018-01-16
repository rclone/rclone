# Profile Builder

## Overview

Azure Profiles offer a means of virtualizing the API Versions of services that should be targeted by an application or SDK.
This concept was introduced for [Azure Stack](https://azure.microsoft.com/overview/azure-stack), where the environment in
which applications will be executed is less consistent than when targeting the public cloud. However, its usefulness as a
means of easily snapping to versions of a service is broadly applicapable. Using profiles, it is easy to use a single version
of models and operations throughout an application, or a means of locking to versions of services that have been tested and
are guaranteed to work together.

[Type aliases were introduced in Go 1.9](https://golang.org/doc/go1.9#language), effectively allowing for multiple symbols
to be mapped to a single type. The impact of this for our support of profiles is tremendous. It allows for seamless
interoperability between packages using different profiles, but where those profiles still target the same API Version of a
service. Without type aliases, we would have been forced to generate code in a way that required some ugly casts to be
scattered throughout the consumer's code.

## Installation

*Note:* These installation notes assume that you have [Go 1.9](https://blog.golang.org/go1.9) or higher, [Glide](http://github.com/Masterminds/glide), and [Git](https://git-scm.com/) installed.

The simplest version of installation is simple, just run the following command:

``` bash
go get -u github.com/Azure/azure-sdk-for-go/tools/profileBuilder
```

If that causes you trouble, run the following commands:

``` bash
# bash
go get -d github.com/Azure/azure-sdk-for-go/tools/profileBuilder
cd $GOPATH/src/github.com/Azure/azure-sdk-for-go/tools/profileBuilder
glide install
go install
```

``` PowerShell
# PowerShell
go get -d github.com/Azure/azure-sdk-for-go/tools/profileBuilder
cd $env:GOPATH\src\github.com\Azure\azure-sdk-for-go\tools\profileBuilder
glide install
go install
```
Taking things a step further, if you'd like the profileBuilder to stamp the version of itself in the code it generates, you can install as below:

``` bash
# bash
go get -d github.com/Azure/azure-sdk-for-go/tools/profileBuilder
cd $GOPATH/src/github.com/Azure/azure-sdk-for-go/tools/profileBuilder
glide install
export currentCommit=$(git rev-parse HEAD)
go install -ldflags "-X main.version=$currentCommit"
```

``` PowerShell
# PowerShell
go get -d github.com/Azure/azure-sdk-for-go/tools/profileBuilder
cd $env:GOPATH\src\github.com\Azure\azure-sdk-for-go\tools\profileBuilder
glide install
$currentCommit = git rev-parse HEAD
go install -ldflags "-X main.version=$currentCommit"
```