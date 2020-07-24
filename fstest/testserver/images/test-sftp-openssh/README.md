# Test SFTP Openssh

This is a docker image for rclone's integration tests which runs an
openssh server in a docker image.

## Build

```
docker build --rm -t rclone/test-sftp-openssh .
docker push rclone/test-sftp-openssh
```

# Test

```
rclone lsf -R --sftp-host 172.17.0.2 --sftp-user rclone --sftp-pass $(rclone obscure password) :sftp:
```
