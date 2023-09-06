# Proton API Bridge

Thanks to Proton open sourcing [proton-go-api](https://github.com/ProtonMail/go-proton-api) and the web, iOS, and Android client codebases, we don't need to completely reverse engineer the APIs by observing the web client traffic!

[proton-go-api](https://github.com/ProtonMail/go-proton-api) provides the basic building blocks of API calls and error handling, such as 429 exponential back-off, but it is pretty much just a barebone interface to the Proton API. For example, the encryption and decryption of the Proton Drive file are not provided in this library. 

This codebase, Proton API Bridge, bridges the gap, so software like [rclone](https://github.com/rclone/rclone) can be built on top of this quickly. This codebase handles the intricate tasks before and after calling Proton APIs, particularly the complex encryption scheme, allowing developers to implement features for other software on top of this codebase.

Currently, only Proton Drive APIs are bridged, as we are aiming to implement a backend for rclone.

## Sidenotes

We are using a fork of the [proton-go-api](https://github.com/henrybear327/go-proton-api), as we are adding quite some new code to it. We are actively rebasing on top of the master branch of the upstream, as we will try to commit back to the upstream once we feel like the code changes are stable.

# Unit testing and linting 

`golangci-lint run && go test -race -failfast -v ./...`

# Drive APIs

> In collaboration with Azimjon Pulatov, in memory of our good old days at Meta, London, in the summer of 2022.
>
> Thanks to Anson Chen for the motivation and some initial help on various matters!

Currently, the development are split into 2 versions. 
V1 supports the features [required by rclone](https://github.com/henrybear327/rclone/blob/master/fs/types.go), such as `file listing`. As the unit and integration tests from rclone have all been passed, we would stabilize this and then move onto developing V2.
V2 will bring in optimizations and enhancements, esp. supporting thumbnails. Please see the list below.

## V1

### Features

- [x] Log in to an account without 2FA using username and password 
- [x] Obtain keyring
- [x] Cache access token, etc. to be able to reuse the session
    - [x] Bug: 403: Access token does not have sufficient scope - used the wrong newClient function
- [x] Volume actions
    - [x] List all volumes
- [x] Share actions
    - [x] Get all shares
    - [x] Get default share
- [x] Fix context with proper propagation instead of using `ctx` everywhere
- [x] Folder actions
    - [x] List all folders and files within the root folder
        - [x] BUG: listing directory - missing signature when there are more than 1 share -> we need to check for the "active" folder type first
    - [x] List all folders and files recursively within the root folder
    - [x] Delete
    - [x] Create
    - [x] (Feature) Update
    - [x] (Feature) Move
- [x] File actions
    - [x] Download
        - [x] Download empty file
        - [x] Improve large file download handling
        - [x] Properly handle large files and empty files (check iOS codebase)
            - esp. large files, where buffering in-memory will screw up the runtime
        - [x] Check signature and hash
    - [x] Delete
    - [x] Upload
        - [x] Handle empty file        
        - [x] Parse mime type 
        - [x] Add revision
        - [x] Modified time
        - [x] Handle failed / interrupted upload
    - [x] List file metadata 
- [x] Duplicated file name handling: 422: A file or folder with that name already exists (Code=2500, Status=422)
- [x] Init ProtonDrive with config passed in as Map
- [x] Remove all `log.Fatalln` and use proper error propagation (basically remove `HandleError` and we go from there)
- [x] Integration tests
    - [x] Remove drive demo code
    - [x] Create a Drive struct to encapsulate all the functions (maybe?)
    - [x] Move comments to proper places
    - [x] Modify `shouldRejectDestructiveActions()`
    - [x] Refactor 
- [x] Reduce config options on caching access token
- [x] Remove integration test safeguarding

### TODO

- [x] address go dependencies
    - Fixed by doing the following in the `go-proton-api` repo to bump to use the latest commit
        - `go get github.com/ProtonMail/go-proton-api@ea8de5f674b7f9b0cca8e3a5076ffe3c5a867e01`
        - `go get github.com/ProtonMail/gluon@fb7689b15ae39c3efec3ff3c615c3d2dac41cec8`
- [x] Remove mail-related apis (to reduce dependencies) 
- [x] Make a "super class" and expose all necessary methods for the outside to call
- [x] Add 2FA login
- [x] Fix the function argument passing (using pointers)
- [x] Handle account with
    - [x] multiple addresses
    - [x] multiple keys per addresses
- [x] Update RClone's contribution.md file
- [x] Remove delete all's hardcoded string
- [x] Point to the right proton-go-api branch
    - [x] Run `go get github.com/henrybear327/go-proton-api@dev` to update go mod
- [x] Pass in AppVersion as a config option
- [x] Proper error handling by looking at the return code instead of the error string
    - [x] Duplicated folder name handling: 422: A file or folder with that name already exists (Code=2500, Status=422)
    - [x] Not found: ERROR RESTY 422: File or folder was not found. (Code=2501, Status=422), Attempt 1
    - [x] Failed upload: Draft already exists on this revision (Code=2500, Status=409)
- [x] Fix file upload progress -> If the upload failed, please Replace file. If the upload is still in progress, replacing it will cancel the ongoing upload
- [x] Concurrency control on file encryption, decryption, and block upload

### Known limitations

- No thumbnails, respecting accepted MIME types, max upload size, can't init Proton Drive, etc.
- Assumptions
    - only one main share per account
    - only operate on active links

## V2

- [ ] Support thumbnail
- [ ] Potential bugs
    - [ ] Confirm the HMAC algorithm -> if you create a draft using integration test, and then use the web frontend to finish the upload (you will see overwrite pop-up), and then use the web frontend to upload again the same file, but this time you will have 2 files with duplicated names
    - [ ] Might have missing signature issues on some old accounts, e.g. GetHashKey on rootLink might fail -> currently have a quick patch, but might need to double check the behavior
    - [ ] Double check the attrs field parsing, esp. for size
    - [ ] Double check the attrs field, esp. for size
- [ ] Crypto-related operations, e.g. signature verification, still needs to cross check with iOS or web open source codebase 
- [ ] Mimetype detection by [using the file content itself](github.com/gabriel-vasile/mimetype), or Google content sniffer
- [ ] Remove e.g. proton.link related exposures in the function signature (this library should abstract them all)
- [ ] Improve documentation
- [ ] Go through Drive iOS source code and check the logic control flow
- [ ] File
    - [ ] Parallel download / upload -> enc/dec is expensive
    - [ ] [Filename encoding](https://github.com/ProtonMail/WebClients/blob/b4eba99d241af4fdae06ff7138bd651a40ef5d3c/applications/drive/src/app/store/_links/validation.ts#L51)
- [ ] Commit back to proton-go-api and switch to using upstream (make sure the tag is at the tip though)
- [ ] Support legacy 2-password mode
- [ ] Proton Drive init (no prior Proton Drive login before -> probably will have no key, volume, etc. to start with at all)
- [ ] linkID caching -> would need to listen to the event api though
- [ ] Integration tests
    - [ ] Check file metadata
    - [ ] Try to check if all functions are used at least once so we know if it's functioning or not
- [ ] Handle accounts with multiple shares
- [ ] Use CI to run integration tests
- [ ] Some error handling from [here](https://github.com/ProtonMail/WebClients/blob/main/packages/shared/lib/drive/constants.ts) MAX_NAME_LENGTH, TIMEOUT
- [ ] [Mimetype restrictions](https://github.com/ProtonMail/WebClients/blob/main/packages/shared/lib/drive/constants.ts#LL47C14-L47C42)
- [ ] Address TODO and FIXME

# Questions

- [x] rclone's folder / file rename detection? -> just implement the interface and rclone will deal with the rest!

# Notes

- Due to caching, functions using `...ByID` needs to perform `protonDrive.removeLinkIDFromCache(linkID, false)` in order to get the latest data!
	