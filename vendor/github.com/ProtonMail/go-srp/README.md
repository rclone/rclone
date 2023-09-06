# go-srp

## Introduction

Golang implementation of the [SRP protocol](https://datatracker.ietf.org/doc/html/rfc5054), used for authentication of ProtonMail users.

## License

Copyright (c) 2019 Proton Technologies AG

Please see [LICENSE](LICENSE.txt) file for the license.

## Doc 

- [Technical blog post](https://protonmail.com/blog/encrypted_email_authentication/)
- [RFC 5054](https://datatracker.ietf.org/doc/html/rfc5054)

## .NET Wrapper

The `windows` folder contains the wrapper for .net.

## Build for mobile apps

Setup Go Mobile and build/bind the source code:

Go Mobile repo: https://github.com/golang/mobile

Go Mobile wiki: https://github.com/golang/go/wiki/Mobile

1. Install Go: `brew install go`
2. Install Gomobile: `go get -u golang.org/x/mobile/cmd/gomobile`
3. Install Gobind: `go install golang.org/x/mobile/cmd/gobind`
4. Install Android SDK and NDK using Android Studio
5. Set env: `export ANDROID_HOME="/AndroidSDK"` (path to your SDK)
6. Init gomobile: `gomobile init -ndk /AndroidSDK/ndk-bundle/` (path to your NDK)
7. Copy Go module dependencies to the vendor directory: `go mod vendor`
8. Build examples:
   `gomobile build -target=android  #or ios`

   Bind examples:
   `gomobile bind -target ios -o frameworks/name.framework`
   `gomobile bind -target android`

   The bind will create framework for iOS and jar&aar files for Android (x86_64 and ARM).

#### Other notes

If you wish to use `build.sh`, you may need to modify the paths in it.

```go
go mod vendor
```

```bash
./build.sh
```

## Dependencies

[github.com/ProtonMail/bcrypt (fork of github.com/jameskeane/bcrypt)](https://github.com/ProtonMail/bcrypt)

[golang.org/x/mobile](https://golang.org/x/mobile)

[github.com/ProtonMail/go-crypto](https://github.com/ProtonMail/go-crypto)

[github.com/cronokirby/saferith](https://github.com/cronokirby/saferith)
