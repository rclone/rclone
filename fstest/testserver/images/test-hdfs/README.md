# Test HDFS

This is a docker image for rclone's integration tests which runs an
hdfs filesystem in a docker image.

## Build

```
docker build --rm -t rclone/test-hdfs .
docker push rclone/test-hdfs
```

# Test

configure remote:
```
[TestHdfs]
type = hdfs
namenode = 127.0.0.1:8020
username = root
```

run tests
```
cd backend/hdfs
GO111MODULE=on go test -v
```

stop docker image:
```
docker kill rclone-hdfs
```
