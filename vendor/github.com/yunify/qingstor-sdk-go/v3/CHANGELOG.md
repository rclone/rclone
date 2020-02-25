# Change Log
All notable changes to QingStor SDK for Go will be documented in this file.

## [v3.2.0] - 2020-2-13

### Added

- Add int64 type converter (#92)
- config: Implement DisableURICleaning config (#95)

### Fixed

- Compatible for content type containing application/json (#91)
- Fix signed url invalid when non-ascii used (#85)

## [v3.1.1] - 2019-10-9

### Changed

- interface: Refactor iface, make sure current struct implements them (#89)

## [v3.1.0] - 2019-10-8

### Added

- Add metadata support for PutObject (#70)
- Add has_more field support in ListMultipartUploads api (#83)
- Add generated qinsgtor api interface (#87)

### Changed

- template: Update to fit API spec change (#80)
- request: Remove http retry logic to avoid body length 0 error (#81)
- client/upload: Switch to normal upload while fileSize small enough (#82)
- docs: Refactor usages and add zh-CN docs (#86)

### Fixed

- template: Fix sdk major version not bumped (#78)
- Fix notification subresource not signed (#84)

## [v3.0.2] - 2019-7-15

### Fixed

- request/builder: Fix content length can't exceed int32 (#74)

## [v3.0.1] - 2019-7-11

### Fixed

- request/unpacker: Fix empty nginx error not parsed correctly (#72)

## [v3.0.0] - 2019-6-26

### Changed

- Use correct go module version
- From this release, go sdk will only provided by module, not zip anymore

## [v2.3.0] - 2019-4-25

### Added

- Add Content-Encoding header to put and post requests
- Add object cache control header
- Add `storage_class` in response element of API spec

## [v2.2.15] - 2018-8-19

### Changed

- Remove object content type detection

## [v2.2.14] - 2018-6-9

### Fixed

- Fix head application/json file failed

## [v2.2.13] - 2018-5-31

### Added

- Add storage class support

## [v2.2.12] - 2018-4-8

### Changed

- Skip empty header while unpacking

## [v2.2.11] - 2018-3-28

### Changed

- Inject request id for HEAD request

### Fixed

- Fix a read timeout bug introduced in v2.2.10

## [v2.2.10] - 2018-3-14

### Changed

- Close body for every API except GetObject and ImageProcess
- Add correct i/o timeout behavior for http client

## [v2.2.9] - 2017-11-25

### Changed

- Refactor logger.

## [v2.2.8] - 2017-09-25

### Added

- Support setting custom SDK logger.

## [v2.2.7] - 2017-09-01

### Added

- Support image process APIs.
- Add advanced client for image process.

### Changed

- Force the zone ID to be lowercase.

### Fixed

- Add support for the X-QS-Date header.

## [v2.2.6] - 2017-07-21

### Fixed

- Fix concurrency issue in object related operations.

## [v2.2.5] - 2017-05-22

### Fixed

- Fix error in request URL query.
- Fix error in request header value.

## [v2.2.4] - 2017-03-28

### Fixed

- Fix type of Content-Type header.
- Add Content-Length to GetObjectOutput.
- Fix status code of DELETE CORS API.
- Fix type of object size for GET Bucket API.

### BREAKING CHANGES

- The type of content length and object size has been changed from `*int` to `*int64`.

## [v2.2.3] - 2017-03-10

### Added

- Allow user to append additional info to User-Agent.

## [v2.2.2] - 2017-03-08

### Fixed

- Resource is not mandatory in bucket policy statement.

## [v2.2.1] - 2017-03-05

### Changed

- Add "Encrypted" field to "KeyType" struct.

## [v2.2.0] - 2017-02-28

### Added

- Add ListMultipartUploads API.

### Fixed

- Fix request builder & signer.

## [v2.1.2] - 2017-01-16

### Fixed

- Fix request signer.

## [v2.1.1] - 2017-01-05

### Changed

- Fix logger output format, don't parse special characters.
- Rename package "errs" to "errors".

### Added

- Add type converters.

### BREAKING CHANGES

- Change value type in input and output to pointer.

## [v2.1.0] - 2016-12-23

### Changed

- Fix signer bug.
- Add more parameters to sign.

### Added

- Add request parameters for GET Object.
- Add IP address conditions for bucket policy.

## [v2.0.1] - 2016-12-15

### Changed

- Improve the implementation of deleting multiple objects.

## [v2.0.0] - 2016-12-14

### Added

- QingStor SDK for the Go programming language.

[v3.2.0]: https://github.com/yunify/qingstor-sdk-go/compare/v3.1.1...v3.2.0
[v3.1.1]: https://github.com/yunify/qingstor-sdk-go/compare/v3.1.0...v3.1.1
[v3.1.0]: https://github.com/yunify/qingstor-sdk-go/compare/v3.0.2...v3.1.0
[v3.0.2]: https://github.com/yunify/qingstor-sdk-go/compare/v3.0.1...v3.0.2
[v3.0.1]: https://github.com/yunify/qingstor-sdk-go/compare/v3.0.0...v3.0.1
[v3.0.0]: https://github.com/yunify/qingstor-sdk-go/compare/v2.3.0...v3.0.0
[v2.3.0]: https://github.com/yunify/qingstor-sdk-go/compare/v2.2.15...v2.3.0
[v2.2.15]: https://github.com/yunify/qingstor-sdk-go/compare/v2.2.14...v2.2.15
[v2.2.14]: https://github.com/yunify/qingstor-sdk-go/compare/v2.2.13...v2.2.14
[v2.2.13]: https://github.com/yunify/qingstor-sdk-go/compare/v2.2.12...v2.2.13
[v2.2.12]: https://github.com/yunify/qingstor-sdk-go/compare/v2.2.11...v2.2.12
[v2.2.11]: https://github.com/yunify/qingstor-sdk-go/compare/v2.2.10...v2.2.11
[v2.2.10]: https://github.com/yunify/qingstor-sdk-go/compare/v2.2.9...v2.2.10
[v2.2.9]: https://github.com/yunify/qingstor-sdk-go/compare/v2.2.8...v2.2.9
[v2.2.8]: https://github.com/yunify/qingstor-sdk-go/compare/v2.2.7...v2.2.8
[v2.2.7]: https://github.com/yunify/qingstor-sdk-go/compare/v2.2.6...v2.2.7
[v2.2.6]: https://github.com/yunify/qingstor-sdk-go/compare/v2.2.5...v2.2.6
[v2.2.5]: https://github.com/yunify/qingstor-sdk-go/compare/v2.2.4...v2.2.5
[v2.2.4]: https://github.com/yunify/qingstor-sdk-go/compare/v2.2.3...v2.2.4
[v2.2.3]: https://github.com/yunify/qingstor-sdk-go/compare/v2.2.2...v2.2.3
[v2.2.2]: https://github.com/yunify/qingstor-sdk-go/compare/v2.2.1...v2.2.2
[v2.2.1]: https://github.com/yunify/qingstor-sdk-go/compare/v2.2.0...v2.2.1
[v2.2.0]: https://github.com/yunify/qingstor-sdk-go/compare/v2.1.2...v2.2.0
[v2.1.2]: https://github.com/yunify/qingstor-sdk-go/compare/v2.1.1...v2.1.2
[v2.1.1]: https://github.com/yunify/qingstor-sdk-go/compare/v2.1.0...v2.1.1
[v2.1.0]: https://github.com/yunify/qingstor-sdk-go/compare/v2.0.1...v2.1.0
[v2.0.1]: https://github.com/yunify/qingstor-sdk-go/compare/v2.0.0...v2.0.1
[v2.0.0]: https://github.com/yunify/qingstor-sdk-go/compare/v2.0.0...v2.0.0
