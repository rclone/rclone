# Go Proton API

<a href="https://github.com/henrybear327/go-proton-api/actions/workflows/check.yml"><img src="https://github.com/henrybear327/go-proton-api/actions/workflows/check.yml/badge.svg?branch=master" alt="CI Status"></a>
<a href="https://pkg.go.dev/github.com/henrybear327/go-proton-api"><img src="https://pkg.go.dev/badge/github.com/henrybear327/go-proton-api" alt="GoDoc"></a>
<a href="https://goreportcard.com/report/github.com/henrybear327/go-proton-api"><img src="https://goreportcard.com/badge/github.com/henrybear327/go-proton-api" alt="Go Report Card"></a>
<a href="LICENSE"><img src="https://img.shields.io/github/license/ProtonMail/go-proton-api.svg" alt="License"></a>

This repository holds Go Proton API, a Go library implementing a client and development server for (a subset of) the Proton REST API.

The license can be found in the [LICENSE](./LICENSE) file.

For the contribution policy, see [CONTRIBUTING](./CONTRIBUTING.md).

## Environment variables

Most of the integration tests run locally. The ones that interact with Proton servers require the following environment variables set:

- ```GO_PROTON_API_TEST_USERNAME```
- ```GO_PROTON_API_TEST_PASSWORD```

## Contribution

This library is forked from [go-proton-api](https://github.com/ProtonMail/go-proton-api) in order to support the [Proton API Bridge](https://github.com/henrybear327/Proton-API-Bridge) project. 

Contribution is welcomed! 

The intention to upstream the changes are planned, once the changes to the codebase has stabalized. 
