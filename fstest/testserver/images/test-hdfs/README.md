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

hdfs logs will be available in `.stdout.log` and `.stderr.log`

# Kerberos

test can be run against kerberos-enabled hdfs

1. configure local krb5.conf
    ```
    [libdefaults]
        default_realm = KERBEROS.RCLONE
    [realms]
        KERBEROS.RCLONE = {
            kdc = localhost
        }
    ```

2. enable kerberos in remote configuration
    ```
    [TestHdfs]
    ...
    service_principal_name = hdfs/localhost
    data_transfer_protection = privacy
    ```

3. run test
    ```
    cd backend/hdfs
    KERBEROS=true GO111MODULE=on go test -v
    ```