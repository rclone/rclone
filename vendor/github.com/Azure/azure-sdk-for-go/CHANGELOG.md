# CHANGELOG

## `v16.2.1`

### Updated Services

| Package Name | API Version |
|-------------:|:-----------:|
|web | 2016-09-01<br/>2018-02-01 |

## `v16.2.0`

### New Services

| Package Name | API Version |
|-------------:|:-----------:|
|eventgrid | 2018-05-01-preview |
|trafficmanager | 2018-03-01 |

### Updated Services

| Package Name | API Version |
|-------------:|:-----------:|
|redis | 2017-10-01<br/>2018-03-01 |
|textanalytics | v2.0 |
|web | 2016-09-01<br/>2018-02-01 |

## `v16.1.0`

- Added a `NewAuthorizerFromEnvironment()` function for the keyvault service.

## `v16.0.0`

### New Services

| Package Name | API Version |
|-------------:|:-----------:|
|batchai | 2018-05-01 |
|botservices | 2017-12-01 |
|containerinstance | 2018-04-01 |
|containerregistry | 2018-02-01 |
|keyvault | v7.0 |
|managedapplications | 2017-09-01 |
|network | 2018-04-01 |
|policyinsights | 2018-04-04 |
|signalr | 2018-03-01-preview |
|storage | 2018-02-0 |
|visualsearch | v1.0 |
|web | 2018-02-01 |

### Updated Services

| Package Name | API Version |
|-------------:|:-----------:|
|apimanagement | 2018-01-01 |
|automation | 2015-10-31<br/>2017-05-15-preview |
|billing | 2018-03-01-preview |
|botservices | 2017-12-01 |
|catalog | 2016-11-01-preview |
|cognitiveservices | 2017-04-18 |
|commerce | 2015-06-01-preview |
|compute | 2018-04-01 |
|consumption | 2018-03-31 |
|contentmoderator | v1.0 |
|datafactory | 2017-09-01-preview |
|datamigration | 2017-11-15-preview|
|eventhub | 2017-04-01 |
|insights | 2015-05-0 |
|iothub | 2017-11-15 |
|network | 2018-02-01 |
|operationalinsights | 2015-03-20<br/>2015-11-01-preview |
|servicebus | 2017-04-01 |
|siterecovery | 2018-01-10 |
|sql | 2017-03-01-preview<br/>2017-10-01-preview<br/>2015-05-01-preview |
|timeseriesinsights | 2017-11-15 |
|luis/runtime | v2.0 |
|web | 2016-09-01 |

## `v15.3.0`

### New Services

| Package Name | API Version |
|-------------:|:-----------:|
|databox | 2018-01-01 |
|devices | 2018-04-01 |
|media | 2018-03-30-preview |
|servicefabric | 6.2 |

### Updated Services

| Package Name | API Version |
|-------------:|:-----------:|
|apimanagement | 2018-01-01 |
|batch | 2018-03-01.6.1 |
|containerregistry | 2017-10-01 |
|documentdb | 2015-04-08 |
|servicebus | 2017-04-01 |

## `v15.2.0`

### New Services

| Package Name | API Version |
|-------------:|:-----------:|
|addons | 2017-05-15 |
|addons | 2018-03-01 |

### Updated Services

| Package Name | API Version |
|-------------:|:-----------:|
|apimanagement | 2017-03-01 |
|apimanagement | 2018-01-01 |
|insights | 2015-05-01 |

## `v15.1.1`

### Bug Fixes

- Drain the response body when retrying a request.
- Avoid allocating when draining a response body.

## `v15.1.0`

### New Services

| Package Name | API Version |
|-------------:|:-----------:|
|datamigration | 2018-03-31-preview |
|devices | 2018-01-22 |
|network | 2018-02-01 |
|cognitiveservices | customvision |

## Updated Services

| Package Name | API Version |
|-------------:|:-----------:|
|compute | 2015-06-15<br/>2016-03-30<br/>2016-04-30-preview<br/>2017-03-30<br/>2018-04-01 |
|datafactory | 2017-09-01-preview |
|insights | 2018-03-01 |
|mysql | 2017-12-01 |
|postgresql | 2017-12-01 |
|web | 2016-09-01 |

## `v15.0.1`

Fixing some build issues, and temporarily reverting CI.

## `v15.0.0`

NOTE: There is a new subdirectory, ./services/preview, which going forward will be used for segregating pre-GA packages.

### New Features

- Added helper func per enum type that returns a slice of all possible values for that type.

### Bug Fixes

- Removed the "arm-" prefix from user-agent strings as not all services are for ARM.
- Fixed missing marshaller for structs with flattened fields.
- Fixed an issue where the incorrect content MIME type was being specified.
- Marshalling of empty values for enum types now works as expected.

### New Services

| Package Name | API Version |
|-------------:|:-----------:|
|apimanagement | 2017-03-01 |
|azurestack | 2017-06-01 |
|billing | 2018-03-01-preview |
|compute | 2018-04-01 |
|consumption | 2018-03-31 |
|databricks | 2018-04-01 |
|dns | 2017-10-01 |
|insights | 2018-03-01 |
|iothub | 2018-01-22 |
|iotspaces | 2017-10-01-preview |
|management | 2018-01-01-preview |
|migrate | 2018-02-02 |
|policyinsights | 2017-08-09-preview<br/>2017-10-17-preview<br/>2017-12-12-preview |
|resources | 2018-02-01 |
|siterecovery | 2018-01-10 |
|sql | 2017-10-01-preview |

### Breaking Changes

| Package Name | API Version |
|-------------:|:-----------:|
| automation | 2017-05-15-preview |
| advisor | 2017-04-19 |
| cognitiveservices | 2017-04-18 |
| compute | 2017-03-30<br/>2017-12-01 |
| consumption | 2018-01-31 |
| containerinstance | 2018-02-01-preview |
| containerservice | 2017-08-31<br/>2017-09-30 |
| customsearch | v1.0 |
| datafactory | 2017-09-01-preview |
| datamigration | 2017-11-15-preview |
| dns | 2018-03-01-preview |
| entitysearch | v1.0 |
| imagesearch | v1.0 |
| insights | 2017-05-01-preview |
| iothub | 2017-11-15 |
| management | 2017-08-31-preview<br/>2017-11-01-preview |
| mysql | 2017-12-01-preview |
| newssearch | v1.0 |
| operationalinsights | 2015-03-20 |
| postgresql | 2017-12-01-preview |
| servicebus | 2015-08-01 |
| servicefabric | 2017-07-01-preview<br/>5.6<br/>6.0<br/>6.1 |
| servicemap | 2015-11-01-preview |
| spellcheck | v1.0 |
| timeseriesinsights | 2017-02-28-preview<br/>2017-11-15 |
| videosearch | v1.0 |
| web | 2016-09-01 |
| websearch | v1.0 |

## `v14.6.0`

### New Services

- Batch 2018-03-01.6.1
- BatchAI 2018-03-01
- Cognitive services custom vision prediction v1.1
- Eventhub 2018-01-01-preview
- MySQL 2017-12-01
- PostgreSQL 2017-12-01
- Redis 2018-03-01
- Subscription 2018-03-01-preview

## `v14.5.0`

### Changes

- Added new preview packages for apimanagement and dns.

## `v14.4.0`

### Changes

- Added new preview API versions for mysql and postgresql.

## `v14.3.0`

### Changes

- Add exports for max file range and sizes for files in storage.
- Updated README regarding blob storage support.
- Add godoc indexer tool.
- Add apidiff tool.

## `v14.2.0`

### Changes

- For blob storage, added GetProperties() method to Container.
- Added PublicAccess field to ContainerProperties struct.

## `v14.1.1`

- Fixing timestamp marshalling bug in the `storage` package.
- Updating `profileBuilder` to clear-output folders when it is run by `go generate`.
- Tweaking Swagger -> SDK config to use "latest" instead of "nightly" and be tied to a particular version of autorest.go.

## `v14.1.0`

### Changes

- Update README with details on new authentication helpers.
- Update `latest` profile to point to latest stable API versions.
- Add new API version for Azure Monitoring service and for Batch Data plane service.

## `v14.0.2`

### Changes

- Updating `profileBuilder list` to accept an `input` flag instead of reading from `stdin`.
- Simplifying CI to have less chatter, be faster, and have no special cases.

## `v14.0.1`

### Changes

- Removed the ./services/search/2016-09-01/search package, it was never intended to be included and doesn't work.

## `v14.0.0`

### Breaking Changes

- Removed the ./arm, ./datalake-store and ./dataplane directories.  You can find the same packages under ./services
- The management package was moved to ./services/classic/management
- Renamed package ./services/redis/mgmt/2017-10-01/cache to ./services/redis/mgmt/2017-10-01/redis

### Changes

- Removed the "-beta" suffix.
- Added various new services.
- Added ./version package for centralized SDK versioning.
