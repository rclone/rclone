# Quark backend test plan

The backend is considered ready when the mock protocol tests, the standard
rclone backend suite, and the live-account tests all pass.

| Capability | Mock test | Live verification |
| --- | --- | --- |
| QR login, pending polling, expiry, cookie persistence | `TestConfigQRCodeLogin*` | configure `TestQuark:` by scanning a fresh code |
| Root paths, listing, pagination, exact name preservation | `TestFileAPIRequests`, `TestListAllPaginationAndNamePreservation` | standard `fstests` |
| Recursive listing / `--fast-list` | `TestListRRecursesDirectories` | standard `fstests` ListR mode |
| Empty, unknown-size, instant and multipart upload | `TestUploadZeroByte`, `TestUploadInstantAndUnknownSize`, `TestUploadMultipart` | standard Put/PutStream tests plus `TestLiveMultipart` |
| Safe overwrite | `TestUpdateUsesTemporaryNameThenReplaces` | standard update test |
| Download, ranges and async task polling | `TestDownloadURLSyncAndRange`, `TestDownloadURLAsync` | standard read and seek tests |
| Size, modification time and MIME type | upload and listing tests | standard object tests |
| File/folder create, delete, move and rename | `TestFileAPIRequests`, `TestRmdirRejectsNonEmptyDirectory` | standard `fstests` |
| Server-side copy | `TestCopyRequest` | standard copy test |
| Directory move and scoped purge | standard interface tests | standard `fstests` |
| Capacity and account information | `TestAbout`, `TestUserInfo` | standard About test and a live account read |
| Public file/folder links and unlink | `TestCreatePublicLink`, `TestUnlinkPublicLinks` | standard PublicLink test plus `TestLivePublicLinkUnlink` |

Provider-specific features without a Quark equivalent, such as Google-native
document export, OneDrive drive types, organization ACLs, or native change
notifications, are outside the parity denominator. Unsupported features must
remain unadvertised rather than being emulated unreliably.
