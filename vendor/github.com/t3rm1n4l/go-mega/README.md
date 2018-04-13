go-mega
=======

A client library in go for mega.co.nz storage service.

An implementation of command-line utility can be found at [https://github.com/t3rm1n4l/megacmd](https://github.com/t3rm1n4l/megacmd)

[![Build Status](https://secure.travis-ci.org/t3rm1n4l/go-mega.png?branch=master)](http://travis-ci.org/t3rm1n4l/go-mega)

### What can i do with this library?
This is an API client library for MEGA storage service. Currently, the library supports the basic APIs and operations as follows:
  - User login
  - Fetch filesystem tree
  - Upload file
  - Download file
  - Create directory
  - Move file or directory
  - Rename file or directory
  - Delete file or directory
  - Parallel split download and upload
  - Filesystem events auto sync
  - Unit tests

### API methods

Please find full doc at [http://godoc.org/github.com/t3rm1n4l/go-mega](http://godoc.org/github.com/t3rm1n4l/go-mega)

### Testing

    export MEGA_USER=<user_email>
    export MEGA_PASSWD=<user_passwd>
    $ make test
    go test -v
    === RUN TestLogin
    --- PASS: TestLogin (1.90 seconds)
    === RUN TestGetUser
    --- PASS: TestGetUser (1.65 seconds)
    === RUN TestUploadDownload
    --- PASS: TestUploadDownload (12.28 seconds)
    === RUN TestMove
    --- PASS: TestMove (9.31 seconds)
    === RUN TestRename
    --- PASS: TestRename (9.16 seconds)
    === RUN TestDelete
    --- PASS: TestDelete (3.87 seconds)
    === RUN TestCreateDir
    --- PASS: TestCreateDir (2.34 seconds)
    === RUN TestConfig
    --- PASS: TestConfig (0.01 seconds)
    === RUN TestPathLookup
    --- PASS: TestPathLookup (8.54 seconds)
    === RUN TestEventNotify
    --- PASS: TestEventNotify (19.65 seconds)
    PASS
    ok  github.com/t3rm1n4l/go-mega68.745s

### TODO
  - Implement APIs for public download url generation
  - Implement download from public url
  - Add shared user content management APIs
  - Add contact list management APIs

### License

MIT
