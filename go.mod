module github.com/rclone/rclone

go 1.17

replace github.com/jlaffaye/ftp => github.com/rclone/ftp v1.0.0-210902f

require (
	bazil.org/fuse v0.0.0-20200524192727-fb710f7dfd05
	github.com/Azure/azure-pipeline-go v0.2.3
	github.com/Azure/azure-storage-blob-go v0.14.0
	github.com/Azure/go-autorest/autorest/adal v0.9.17
	github.com/Azure/go-ntlmssp v0.0.0-20200615164410-66371956d46c
	github.com/Unknwon/goconfig v0.0.0-20200908083735-df7de6a44db8
	github.com/a8m/tree v0.0.0-20210414114729-ce3525c5c2ef
	github.com/aalpar/deheap v0.0.0-20210914013432-0cc84d79dec3
	github.com/abbot/go-http-auth v0.4.0
	github.com/anacrolix/dms v1.3.0
	github.com/artyom/mtab v0.0.0-20141107123140-74b6fd01d416
	github.com/atotto/clipboard v0.1.4
	github.com/aws/aws-sdk-go v1.42.1
	github.com/billziss-gh/cgofuse v1.5.0
	github.com/buengese/sgzip v0.1.1
	github.com/colinmarc/hdfs/v2 v2.2.0
	github.com/coreos/go-semver v0.3.0
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf
	github.com/dop251/scsu v0.0.0-20210824104153-f677eef82431
	github.com/dropbox/dropbox-sdk-go-unofficial/v6 v6.0.3
	github.com/gabriel-vasile/mimetype v1.4.0
	github.com/go-chi/chi/v5 v5.0.5
	github.com/google/uuid v1.3.0
	github.com/hanwen/go-fuse/v2 v2.1.0
	github.com/iguanesolutions/go-systemd/v5 v5.1.0
	github.com/jcmturner/gokrb5/v8 v8.4.2
	github.com/jlaffaye/ftp v0.0.0-20211029032751-b1140299f4df
	github.com/jzelinskie/whirlpool v0.0.0-20201016144138-0675e54bb004
	github.com/klauspost/compress v1.13.6
	github.com/koofr/go-httpclient v0.0.0-20200420163713-93aa7c75b348
	github.com/koofr/go-koofrclient v0.0.0-20190724113126-8e5366da203a
	github.com/mattn/go-colorable v0.1.11
	github.com/mattn/go-runewidth v0.0.13
	github.com/mitchellh/go-homedir v1.1.0
	github.com/ncw/go-acd v0.0.0-20201019170801-fe55f33415b1
	github.com/ncw/swift/v2 v2.0.1
	github.com/nsf/termbox-go v1.1.1
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/sftp v1.13.4
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_golang v1.11.0
	github.com/putdotio/go-putio/putio v0.0.0-20200123120452-16d982cac2b8
	github.com/rfjakob/eme v1.1.2
	github.com/shirou/gopsutil/v3 v3.21.10
	github.com/sirupsen/logrus v1.8.1
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/t3rm1n4l/go-mega v0.0.0-20200416171014-ffad7fcb44b8
	github.com/xanzy/ssh-agent v0.3.1
	github.com/youmark/pkcs8 v0.0.0-20201027041543-1326539a0a0a
	github.com/yunify/qingstor-sdk-go/v3 v3.2.0
	go.etcd.io/bbolt v1.3.6
	goftp.io/server v0.4.1
	golang.org/x/crypto v0.0.0-20211108221036-ceb1ce70b4fa
	golang.org/x/net v0.0.0-20211109214657-ef0fda0de508
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20211109184856-51b60fd695b3
	golang.org/x/text v0.3.7
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac
	google.golang.org/api v0.60.0
	gopkg.in/yaml.v2 v2.4.0
	storj.io/uplink v1.7.0
)

require (
	cloud.google.com/go v0.97.0 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/Microsoft/go-winio v0.5.1 // indirect
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/btcsuite/btcutil v1.0.3-0.20201208143702-a53e38424cce // indirect
	github.com/calebcase/tmpfile v1.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.1.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/googleapis/gax-go/v2 v2.1.1 // indirect
	github.com/hashicorp/go-uuid v1.0.2 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.0.0 // indirect
	github.com/jcmturner/goidentity/v6 v6.0.1 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mattn/go-ieproxy v0.0.1 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/pengsrc/go-shared v0.2.1-0.20190131101655-1999055a4a14 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.32.1 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/spacemonkeygo/monkit/v3 v3.0.17 // indirect
	github.com/tklauser/go-sysconf v0.3.9 // indirect
	github.com/tklauser/numcpus v0.3.0 // indirect
	github.com/vivint/infectious v0.0.0-20200605153912-25a574ae18a3 // indirect
	github.com/zeebo/errs v1.2.2 // indirect
	go.opencensus.io v0.23.0 // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20211104193956-4c6863e31247 // indirect
	google.golang.org/grpc v1.42.0 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	storj.io/common v0.0.0-20210916151047-6aaeb34bb916 // indirect
	storj.io/drpc v0.0.26 // indirect
)
