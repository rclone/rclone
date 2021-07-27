module github.com/rclone/rclone

go 1.14

require (
	bazil.org/fuse v0.0.0-20200524192727-fb710f7dfd05
	github.com/Azure/azure-pipeline-go v0.2.3
	github.com/Azure/azure-storage-blob-go v0.13.0
	github.com/Azure/go-autorest/autorest/adal v0.9.13
	github.com/Azure/go-ntlmssp v0.0.0-20200615164410-66371956d46c
	github.com/Microsoft/go-winio v0.4.17 // indirect
	github.com/StackExchange/wmi v0.0.0-20210224194228-fe8f1750fd46 // indirect
	github.com/Unknwon/goconfig v0.0.0-20200908083735-df7de6a44db8
	github.com/a8m/tree v0.0.0-20210414114729-ce3525c5c2ef
	github.com/aalpar/deheap v0.0.0-20200318053559-9a0c2883bd56
	github.com/abbot/go-http-auth v0.4.0
	github.com/anacrolix/dms v1.2.2
	github.com/atotto/clipboard v0.1.4
	github.com/aws/aws-sdk-go v1.38.22
	github.com/billziss-gh/cgofuse v1.5.0
	github.com/buengese/sgzip v0.1.1
	github.com/calebcase/tmpfile v1.0.2 // indirect
	github.com/colinmarc/hdfs/v2 v2.2.0
	github.com/coreos/go-semver v0.3.0
	github.com/coreos/go-systemd v0.0.0-20190321100706-95778dfbb74e
	github.com/dop251/scsu v0.0.0-20200422003335-8fadfb689669
	github.com/dropbox/dropbox-sdk-go-unofficial v1.0.1-0.20210114204226-41fdcdae8a53
	github.com/gabriel-vasile/mimetype v1.2.0
	github.com/go-chi/chi/v5 v5.0.2
	github.com/go-ole/go-ole v1.2.5 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/uuid v1.2.0
	github.com/hanwen/go-fuse/v2 v2.1.0
	github.com/iguanesolutions/go-systemd/v5 v5.0.0
	github.com/jcmturner/gokrb5/v8 v8.4.2
	github.com/jlaffaye/ftp v0.0.0-20210307004419-5d4190119067
	github.com/jzelinskie/whirlpool v0.0.0-20201016144138-0675e54bb004
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/klauspost/compress v1.12.1
	github.com/koofr/go-httpclient v0.0.0-20200420163713-93aa7c75b348
	github.com/koofr/go-koofrclient v0.0.0-20190724113126-8e5366da203a
	github.com/mattn/go-colorable v0.1.8
	github.com/mattn/go-runewidth v0.0.12
	github.com/mitchellh/go-homedir v1.1.0
	github.com/ncw/go-acd v0.0.0-20201019170801-fe55f33415b1
	github.com/ncw/swift/v2 v2.0.0
	github.com/nsf/termbox-go v1.1.1-0.20210421210813-2ff630277754
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/pkg/sftp v1.13.1-0.20210424083437-2b80967078b8
	github.com/prometheus/client_golang v1.10.0
	github.com/prometheus/common v0.20.0 // indirect
	github.com/putdotio/go-putio/putio v0.0.0-20200123120452-16d982cac2b8
	github.com/rfjakob/eme v1.1.1
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sevlyar/go-daemon v0.1.5
	github.com/shirou/gopsutil/v3 v3.21.3
	github.com/sirupsen/logrus v1.8.1
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966
	github.com/spacemonkeygo/monkit/v3 v3.0.11 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/t3rm1n4l/go-mega v0.0.0-20200416171014-ffad7fcb44b8
	github.com/tklauser/go-sysconf v0.3.5 // indirect
	github.com/valyala/fastjson v1.6.3
	github.com/xanzy/ssh-agent v0.3.0
	github.com/youmark/pkcs8 v0.0.0-20201027041543-1326539a0a0a
	github.com/yunify/qingstor-sdk-go/v3 v3.2.0
	go.etcd.io/bbolt v1.3.5
	go.uber.org/zap v1.16.0 // indirect
	goftp.io/server v0.4.1
	golang.org/x/crypto v0.0.0-20210421170649-83a5a9bb288b
	golang.org/x/net v0.0.0-20210415231046-e915ea6b2b7d
	golang.org/x/oauth2 v0.0.0-20210413134643-5e61552d6c78
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210423185535-09eb48e85fd7
	golang.org/x/term v0.0.0-20210406210042-72f3dc4e9b72 // indirect
	golang.org/x/text v0.3.6
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba
	google.golang.org/api v0.44.0
	google.golang.org/genproto v0.0.0-20210416161957-9910b6c460de // indirect
	google.golang.org/grpc v1.37.0 // indirect
	gopkg.in/dc0d/tinykv.v4 v4.0.1
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	storj.io/common v0.0.0-20210419115916-eabb53ea1332 // indirect
	storj.io/uplink v1.4.6
)
