# eventkit

a go library for reporting multidimensional events over UDP

## Build

Project can be built with usual `go` commands (such as `go install` or `go test ./...`), but current CI uses [Earthly](https://earthly.dev), which can be installed with downloading one binary.

Follow the [instructions](https://earthly.dev) to download. For example, on Linux one can use:

```
sudo /bin/sh -c 'wget https://github.com/earthly/earthly/releases/latest/download/earthly-linux-amd64 -O /usr/local/bin/earthly && chmod +x /usr/local/bin/earthly && /usr/local/bin/earthly bootstrap --with-autocomplete'
```

You can decide to turn off all cloud features:

```
earthly config global.disable_analytics true   
earthly config global.disable_log_sharing true
```

Most useful build targets:

* `earthly +lint` --> execute basic linting
* `earthly +test` --> execute unit tests
* `earthly +format` --> format the code (requires full committed state)

In case of something is failing, use `-i` which will give you an interactive environment where you can try to repeat the failing command:

```
earthly -i +lint
```