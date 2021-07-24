module github.com/rclone/rclone

go 1.14

require (
	bazil.org/fuse v0.0.0-20200524192727-fb710f7dfd05
	cloud.google.com/go v0.93.3 // indirect
	github.com/Azure/azure-pipeline-go v0.2.3
	github.com/Azure/azure-storage-blob-go v0.14.0
	github.com/Azure/go-autorest/autorest/adal v0.9.14
	github.com/Azure/go-ntlmssp v0.0.0-20200615164410-66371956d46c
	github.com/Unknwon/goconfig v0.0.0-20200908083735-df7de6a44db8
	github.com/a8m/tree v0.0.0-20210414114729-ce3525c5c2ef
	github.com/aalpar/deheap v0.0.0-20200318053559-9a0c2883bd56
	github.com/abbot/go-http-auth v0.4.0
	github.com/anacrolix/dms v1.2.2
	github.com/artyom/mtab v0.0.0-20141107123140-74b6fd01d416
	github.com/atotto/clipboard v0.1.4
	github.com/aws/aws-sdk-go v1.40.27
	github.com/billziss-gh/cgofuse v1.5.0
	github.com/buengese/sgzip v0.1.1
	github.com/calebcase/tmpfile v1.0.3 // indirect
	github.com/colinmarc/hdfs/v2 v2.2.0
	github.com/coreos/go-semver v0.3.0
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf
	github.com/cpuguy83/go-md2man/v2 v2.0.1 // indirect
	github.com/dop251/scsu v0.0.0-20200422003335-8fadfb689669
	github.com/dropbox/dropbox-sdk-go-unofficial v1.0.1-0.20210114204226-41fdcdae8a53
	github.com/form3tech-oss/jwt-go v3.2.5+incompatible // indirect
	github.com/gabriel-vasile/mimetype v1.3.1
	github.com/go-chi/chi/v5 v5.0.3
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/uuid v1.3.0
	github.com/hanwen/go-fuse/v2 v2.1.0
	github.com/iguanesolutions/go-systemd/v5 v5.1.0
	github.com/jcmturner/gokrb5/v8 v8.4.2
	github.com/jlaffaye/ftp v0.0.0-20210307004419-5d4190119067
	github.com/jzelinskie/whirlpool v0.0.0-20201016144138-0675e54bb004
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/klauspost/compress v1.13.4
	github.com/koofr/go-httpclient v0.0.0-20200420163713-93aa7c75b348
	github.com/koofr/go-koofrclient v0.0.0-20190724113126-8e5366da203a
	github.com/mattn/go-colorable v0.1.8
	github.com/mattn/go-isatty v0.0.13 // indirect
	github.com/mattn/go-runewidth v0.0.13
	github.com/mitchellh/go-homedir v1.1.0
	github.com/ncw/go-acd v0.0.0-20201019170801-fe55f33415b1
	github.com/ncw/swift/v2 v2.0.0
	github.com/nsf/termbox-go v1.1.1
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/pkg/sftp v1.13.2
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/common v0.30.0 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/putdotio/go-putio/putio v0.0.0-20200123120452-16d982cac2b8
	github.com/rfjakob/eme v1.1.2
	github.com/sevlyar/go-daemon v0.1.5
	github.com/shirou/gopsutil/v3 v3.21.8
	github.com/sirupsen/logrus v1.8.1
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966
	github.com/spacemonkeygo/monkit/v3 v3.0.15 // indirect
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/t3rm1n4l/go-mega v0.0.0-20200416171014-ffad7fcb44b8
	github.com/xanzy/ssh-agent v0.3.1
	github.com/youmark/pkcs8 v0.0.0-20201027041543-1326539a0a0a
	github.com/yunify/qingstor-sdk-go/v3 v3.2.0
	go.etcd.io/bbolt v1.3.6
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.19.0 // indirect
	goftp.io/server v0.4.1
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5
	golang.org/x/net v0.0.0-20210813160813-60bc85c4be6d
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210820121016-41cdb8703e55
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b // indirect
	golang.org/x/text v0.3.7
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac
	google.golang.org/api v0.54.0
	google.golang.org/genproto v0.0.0-20210820002220-43fce44e7af1 // indirect
	gopkg.in/yaml.v2 v2.4.0
	storj.io/common v0.0.0-20210818163656-4667d2cafb27 // indirect
	storj.io/uplink v1.4.6
)
