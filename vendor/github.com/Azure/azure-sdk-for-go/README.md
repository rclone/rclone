# Azure SDK for Go

[![godoc](https://godoc.org/github.com/Azure/azure-sdk-for-go?status.svg)](https://godoc.org/github.com/Azure/azure-sdk-for-go)
[![Build Status](https://travis-ci.org/Azure/azure-sdk-for-go.svg?branch=master)](https://travis-ci.org/Azure/azure-sdk-for-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/Azure/azure-sdk-for-go)](https://goreportcard.com/report/github.com/Azure/azure-sdk-for-go)

azure-sdk-for-go provides Go packages for managing and using Azure services. It has been
tested with Go 1.8, 1.9 and 1.10.

To be notified about updates and changes, subscribe to the [Azure update
feed](https://azure.microsoft.com/updates/).

Users of the SDK may prefer to jump right in to our samples repo at
[github.com/Azure-Samples/azure-sdk-for-go-samples][samples_repo].

### Build Details

Most packages in the SDK are generated from [Azure API specs][azure_rest_specs]
using [Azure/autorest.go][] and [Azure/autorest][]. These generated packages
depend on the HTTP client implemented at [Azure/go-autorest][].

[azure_rest_specs]: https://github.com/Azure/azure-rest-api-specs
[Azure/autorest]: https://github.com/Azure/autorest
[Azure/autorest.go]: https://github.com/Azure/autorest.go
[Azure/go-autorest]: https://github.com/Azure/go-autorest

The SDK codebase adheres to [semantic versioning](https://semver.org) and thus
avoids breaking changes other than at major (x.0.0) releases. However,
occasionally Azure API fixes require breaking updates within an individual
package; these exceptions are noted in release changelogs.

To more reliably manage dependencies like the Azure SDK in your applications we
recommend [golang/dep](https://github.com/golang/dep).

# Install and Use:

### Install

```sh
$ go get -u github.com/Azure/azure-sdk-for-go/...
```

or if you use dep, within your repo run:

```sh
$ dep ensure -add github.com/Azure/azure-sdk-for-go
```

If you need to install Go, follow [the official instructions](https://golang.org/dl/).

### Use

For complete examples of many scenarios see [Azure-Samples/azure-sdk-for-go-samples][samples_repo].

1. Import a package from the [services][services_dir] directory.
2. Create and authenticate a client with a `New*Client` func, e.g.
   `c := compute.NewVirtualMachinesClient(...)`.
3. Invoke API methods using the client, e.g. `c.CreateOrUpdate(...)`.
4. Handle responses.

[services_dir]: https://github.com/Azure/azure-sdk-for-go/tree/master/services

For example, to create a new virtual network (substitute your own values for
strings in angle brackets):

Note: For more on authentication and the `Authorizer` interface see [the next
  section](#authentication).

```go
package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2017-09-01/network"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
)

func main() {
	vnetClient := network.NewVirtualNetworksClient("<subscriptionID>")
	authorizer, err := auth.NewAuthorizerFromEnvironment()

	if err == nil {
		vnetClient.Authorizer = authorizer
	}

	vnetClient.CreateOrUpdate(context.Background(),
		"<resourceGroupName>",
		"<vnetName>",
		network.VirtualNetwork{
			Location: to.StringPtr("<azureRegion>"),
			VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
				AddressSpace: &network.AddressSpace{
					AddressPrefixes: &[]string{"10.0.0.0/8"},
				},
				Subnets: &[]network.Subnet{
					{
						Name: to.StringPtr("<subnet1Name>"),
						SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
							AddressPrefix: to.StringPtr("10.0.0.0/16"),
						},
					},
					{
						Name: to.StringPtr("<subnet2Name>"),
						SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
							AddressPrefix: to.StringPtr("10.1.0.0/16"),
						},
					},
				},
			},
		})
}
```

### Authentication

Most SDK operations require an OAuth token for authentication and authorization. These are
made available in the Go SDK For Azure through types implementing the `Authorizer` interface.
You can get one from Azure Active Directory using the SDK's
[authentication](https://godoc.org/github.com/Azure/go-autorest/autorest/azure/auth) package. The `Authorizer` returned  should
be set as the authorizer for the resource client, as shown in the [previous section](#use).

You can get an authorizer in the following ways:
1. From the **Environment**:
  - Use `auth.auth.NewAuthorizerFromEnvironment()`. This call will try to get an authorizer based on the environment
variables with different types of credentials in the following order:
    1. **Client Credentials**: Uses the AAD App Secret for auth.
      - `AZURE_TENANT_ID`: Specifies the Tenant to which to authenticate.
      - `AZURE_CLIENT_ID`: Specifies the app client ID to use.
      - `AZURE_CLIENT_SECRET`: Specifies the app secret to use.
    2. **Client Certificate**: Uses a certificate that was configured on the AAD Service Principal.
      - `AZURE_TENANT_ID`: Specifies the Tenant to which to authenticate.
      - `AZURE_CLIENT_ID`: Specifies the app client ID to use.
      - `AZURE_CERTIFICATE_PATH`: Specifies the certificate Path to use.
      - `AZURE_CERTIFICATE_PASSWORD`: Specifies the certificate password to use.
    3. **Username Pasword**: Uses a username and a password for auth. This is not recommended. Use `Device Flow` Auth instead for user interactive acccess.
      - `AZURE_TENANT_ID`: Specifies the Tenant to which to authenticate.
      - `AZURE_CLIENT_ID`: Specifies the app client ID to use.
      - `AZURE_USERNAME`: Specifies the username to use.
      - `AZURE_PASSWORD`: Specifies the password to use.
    4. **MSI**: Only available for apps running in Azure. No configuration needed as it leverages the fact that the app is running in Azure. See [Azure Managed Service Identity](https://docs.microsoft.com/en-us/azure/active-directory/msi-overview).

  - Optionally, the following environment variables can be defined:
    - `AZURE_ENVIRONMENT`: Specifies the Azure Environment to use. If not set, it defaults to `AzurePublicCloud`. (Not applicable to MSI based auth)
    - `AZURE_AD_RESOURCE`: Specifies the AAD resource ID to use. If not set, it defaults to `ResourceManagerEndpoint`which allows management operations against Azure Resource Manager.

2. From an **Auth File**:
 - Create a service principal and output the file content using `az ad sp create-for-rbac --sdk-auth` from the Azure CLI.For more details see [az ad sp](https://docs.microsoft.com/en-us/cli/azure/ad/sp).
 - Set environment variable `AZURE_AUTH_LOCATION` for finding the file.
 - Use `auth.NewAuthorizerFromFile()` for getting the `Authorizer` based on the auth file.

3. From **Device Flow** by configuring `auth.DeviceFlowConfig` and calling the `Authorizer()` method.

Note: To authenticate you first need to create a service principal in Azure. To create a new service principal, run
`az ad sp create-for-rbac -n "<app_name>"` in the
[azure-cli](https://github.com/Azure/azure-cli). See
[these docs](https://docs.microsoft.com/cli/azure/create-an-azure-service-principal-azure-cli?view=azure-cli-latest)
for more info. Copy the new principal's ID, secret, and tenant ID for use in your app.

Alternatively, if your apps are running in Azure, you can now leverage the [Managed Service Identity](https://docs.microsoft.com/en-us/azure/active-directory/msi-overview).

# Versioning

azure-sdk-for-go provides at least a basic Go binding for every Azure API. To
provide maximum flexibility to users, the SDK even includes previous versions of
Azure APIs which are still in use. This enables us to support users of the
most updated Azure datacenters, regional datacenters with earlier APIs, and
even on-premises installations of Azure Stack.

**SDK versions** apply globally and are tracked by git
[tags](https://github.com/Azure/azure-sdk-for-go/tags). These are in x.y.z form
and generally adhere to [semantic versioning](https://semver.org) specifications.

**Service API versions** are generally represented by a date string and are
tracked by offering separate packages for each version. For example, to choose the
latest API versions for Compute and Network, use the following imports:

```go
import (
    "github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2017-12-01/compute"
    "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2017-09-01/network"
)
```

Occasionally service-side changes require major changes to existing versions.
These cases are noted in the changelog.

All avilable services and versions are listed under the `services/` path in
this repo and in [GoDoc][services_godoc].  Run `find ./services -type d
-mindepth 3` to list all available service packages.

[services_godoc]:       https://godoc.org/github.com/Azure/azure-sdk-for-go/services

### Profiles

Azure **API profiles** specify subsets of Azure APIs and versions. Profiles can provide:

* **stability** for your application by locking to specific API versions; and/or
* **compatibility** for your application with Azure Stack and regional Azure datacenters.

In the Go SDK, profiles are available under the `profiles/` path and their
component API versions are aliases to the true service package under
`services/`. You can use them as follows:

```go
import "github.com/Azure/azure-sdk-for-go/profiles/2017-03-09/compute/mgmt/compute"
import "github.com/Azure/azure-sdk-for-go/profiles/2017-03-09/network/mgmt/network"
import "github.com/Azure/azure-sdk-for-go/profiles/2017-03-09/storage/mgmt/storage"
```

The 2017-03-09 profile is the only one currently available and is for use in
hybrid Azure and Azure Stack environments. More profiles are under development.

In addition to versioned profiles, we also provide two special profiles
`latest` and `preview`. These *always* include the most recent respective stable or
preview API versions for each service, even when updating them to do so causes
breaking changes. That is, these do *not* adhere to semantic versioning rules.

The `latest` and `preview` profiles can help you stay up to date with API
updates as you build applications. Since they are by definition not stable,
however, they **should not** be used in production apps. Instead, choose the
latest specific API version (or an older one if necessary) from the `services/`
path.

As an example, to automatically use the most recent Compute APIs, use one of
the following imports:

```go
import "github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
import "github.com/Azure/azure-sdk-for-go/profiles/preview/compute/mgmt/compute"
```

## Inspecting and Debugging

All clients implement some handy hooks to help inspect the underlying requests being made to Azure.

- `RequestInspector`: View and manipulate the go `http.Request` before it's sent
- `ResponseInspector`: View the `http.Response` received

Here is an example of how these can be used with `net/http/httputil` to see requests and responses.

```go

vnetClient := network.NewVirtualNetworksClient("<subscriptionID>")
vnetClient.RequestInspector = LogRequest()
vnetClient.ResponseInspector = LogResponse()

...

func LogRequest() autorest.PrepareDecorator {
	return func(p autorest.Preparer) autorest.Preparer {
		return autorest.PreparerFunc(func(r *http.Request) (*http.Request, error) {
			r, err := p.Prepare(r)
			if err != nil {
				log.Println(err)
			}
			dump, _ := httputil.DumpRequestOut(r, true)
			log.Println(string(dump))
			return r, err
		})
	}
}

func LogResponse() autorest.RespondDecorator {
	return func(p autorest.Responder) autorest.Responder {
		return autorest.ResponderFunc(func(r *http.Response) error {
			err := p.Respond(r)
			if err != nil {
				log.Println(err)
			}
			dump, _ := httputil.DumpResponse(r, true)
			log.Println(string(dump))
			return err
		})
	}
}
```

# Resources

- SDK docs are at [godoc.org](https://godoc.org/github.com/Azure/azure-sdk-for-go/).
- SDK samples are at [Azure-Samples/azure-sdk-for-go-samples](https://github.com/Azure-Samples/azure-sdk-for-go-samples).
- SDK notifications are published via the [Azure update feed](https://azure.microsoft.com/updates/).
- Azure API docs are at [docs.microsoft.com/rest/api](https://docs.microsoft.com/rest/api/).
- General Azure docs are at [docs.microsoft.com/azure](https://docs.microsoft.com/azure).

### Other Azure packages for Go

- [Azure Storage Blobs](https://azure.microsoft.com/services/storage/blobs) - [github.com/Azure/azure-storage-blob-go](https://github.com/Azure/azure-storage-blob-go)
- [Azure Applications Insights](https://azure.microsoft.com/en-us/services/application-insights/) - [github.com/Microsoft/ApplicationInsights-Go](https://github.com/Microsoft/ApplicationInsights-Go)

## License

Apache 2.0, see [LICENSE](./LICENSE).

## Contribute

See [CONTRIBUTING.md](./CONTRIBUTING.md).

[samples_repo]:         https://github.com/Azure-Samples/azure-sdk-for-go-samples

