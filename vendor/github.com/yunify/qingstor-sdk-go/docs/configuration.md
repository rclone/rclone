# Configuration Guide

## Summary

This SDK uses a structure called "Config" to store and manage configuration, read comments of public functions in ["config/config.go"](https://github.com/yunify/qingstor-sdk-go/blob/master/config/config.go) for details.

Except for Access Key, you can also configure the API endpoint for private cloud usage scenario. All available configurable items are listed in the default configuration file.

___Default Configuration File:___

``` yaml
# QingStor services configuration

access_key_id: 'ACCESS_KEY_ID'
secret_access_key: 'SECRET_ACCESS_KEY'

host: 'qingstor.com'
port: 443
protocol: 'https'
connection_retries: 3

# Valid log levels are "debug", "info", "warn", "error", and "fatal".
log_level: 'warn'

```

## Usage

Just create a config structure instance with your API Access Key, and initialize services you need with Init() function of the target service.

### Code Snippet

Create default configuration

``` go
defaultConfig, _ := config.NewDefault()
```

Create configuration from Access Key

``` go
configuration, _ := config.New("ACCESS_KEY_ID", "SECRET_ACCESS_KEY")

anotherConfiguration := config.NewDefault()
anotherConfiguration.AccessKeyID = "ACCESS_KEY_ID"
anotherConfiguration.SecretAccessKey = "SECRET_ACCESS_KEY"
```

Load user configuration

``` go
userConfig, _ := config.NewDefault().LoadUserConfig()
```

Load configuration from config file

``` go
configFromFile, _ := config.NewDefault().LoadConfigFromFilepath("PATH/TO/FILE")
```

Change API endpoint

``` go
moreConfiguration, _ := config.NewDefault()

moreConfiguration.Protocol = "http"
moreConfiguration.Host = "api.private.com"
moreConfiguration.Port = 80
```
