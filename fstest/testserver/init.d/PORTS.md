# Ports for tests

All these tests need to run on a different port.

They should be bound to localhost so they are not accessible externally.

| Port  | Test |
|:-----:|:----:|
|    88 | TestHdfs |
|   750 | TestHdfs |
|  8020 | TestHdfs |
|  8086 | TestSeafileV6 |
|  8087 | TestSeafile |
|  8088 | TestSeafileEncrypted |
|  9866 | TestHdfs |
| 28620 | TestWebdavRclone |
| 28621 | TestSFTPRclone |
| 28622 | TestFTPRclone |
| 28623 | TestSFTPRcloneSSH |
| 28624 | TestS3Rclone |
| 28625 | TestS3Minio |
| 28626 | TestS3MinioEdge |
| 28627 | TestSFTPOpenssh |
| 28628 | TestSwiftAIO |
| 28629 | TestWebdavNextcloud |
| 28630 | TestSMB |
| 28631 | TestFTPProftpd |
| 28632 | TestSwiftAIOsegments |
| 38081 | TestWebdavOwncloud |

## Non localhost tests

All these use `$(docker_ip)` which means they don't work on macOS or
Windows. It is proabably possible to make them work with some effort
but will require port forwarding a range of ports and configuring the
FTP server to only use that range of ports. The FTP server will likely
need know it is behind a NAT so it advertises the correct external IP.

- TestFTPProftpd
- TestFTPPureftpd
- TestFTPVsftpd
- TestFTPVsftpdTLS
