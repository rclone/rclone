// Package s3 provides an interface to Amazon S3 oject storage
package s3

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/corehandlers"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/ncw/swift/v2"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/chunksize"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/bucket"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/pool"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/lib/rest"
	"github.com/rclone/rclone/lib/structs"
	"golang.org/x/sync/errgroup"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "s3",
		Description: "Amazon S3 Compliant Storage Providers including AWS, Alibaba, Ceph, China Mobile, Cloudflare, ArvanCloud, Digital Ocean, Dreamhost, Huawei OBS, IBM COS, Lyve Cloud, Minio, Netease, RackCorp, Scaleway, SeaweedFS, StackPath, Storj, Tencent COS and Wasabi",
		NewFs:       NewFs,
		CommandHelp: commandHelp,
		Options: []fs.Option{{
			Name: fs.ConfigProvider,
			Help: "Choose your S3 provider.",
			// NB if you add a new provider here, then add it in the
			// setQuirks function and set the correct quirks
			Examples: []fs.OptionExample{{
				Value: "AWS",
				Help:  "Amazon Web Services (AWS) S3",
			}, {
				Value: "Alibaba",
				Help:  "Alibaba Cloud Object Storage System (OSS) formerly Aliyun",
			}, {
				Value: "Ceph",
				Help:  "Ceph Object Storage",
			}, {
				Value: "ChinaMobile",
				Help:  "China Mobile Ecloud Elastic Object Storage (EOS)",
			}, {
				Value: "Cloudflare",
				Help:  "Cloudflare R2 Storage",
			}, {
				Value: "ArvanCloud",
				Help:  "Arvan Cloud Object Storage (AOS)",
			}, {
				Value: "DigitalOcean",
				Help:  "Digital Ocean Spaces",
			}, {
				Value: "Dreamhost",
				Help:  "Dreamhost DreamObjects",
			}, {
				Value: "HuaweiOBS",
				Help:  "Huawei Object Storage Service",
			}, {
				Value: "IBMCOS",
				Help:  "IBM COS S3",
			}, {
				Value: "LyveCloud",
				Help:  "Seagate Lyve Cloud",
			}, {
				Value: "Minio",
				Help:  "Minio Object Storage",
			}, {
				Value: "Netease",
				Help:  "Netease Object Storage (NOS)",
			}, {
				Value: "RackCorp",
				Help:  "RackCorp Object Storage",
			}, {
				Value: "Scaleway",
				Help:  "Scaleway Object Storage",
			}, {
				Value: "SeaweedFS",
				Help:  "SeaweedFS S3",
			}, {
				Value: "StackPath",
				Help:  "StackPath Object Storage",
			}, {
				Value: "Storj",
				Help:  "Storj (S3 Compatible Gateway)",
			}, {
				Value: "TencentCOS",
				Help:  "Tencent Cloud Object Storage (COS)",
			}, {
				Value: "Wasabi",
				Help:  "Wasabi Object Storage",
			}, {
				Value: "Other",
				Help:  "Any other S3 compatible provider",
			}},
		}, {
			Name:    "env_auth",
			Help:    "Get AWS credentials from runtime (environment variables or EC2/ECS meta data if no env vars).\n\nOnly applies if access_key_id and secret_access_key is blank.",
			Default: false,
			Examples: []fs.OptionExample{{
				Value: "false",
				Help:  "Enter AWS credentials in the next step.",
			}, {
				Value: "true",
				Help:  "Get AWS credentials from the environment (env vars or IAM).",
			}},
		}, {
			Name: "access_key_id",
			Help: "AWS Access Key ID.\n\nLeave blank for anonymous access or runtime credentials.",
		}, {
			Name: "secret_access_key",
			Help: "AWS Secret Access Key (password).\n\nLeave blank for anonymous access or runtime credentials.",
		}, {
			// References:
			// 1. https://docs.aws.amazon.com/general/latest/gr/rande.html
			// 2. https://docs.aws.amazon.com/general/latest/gr/s3.html
			Name:     "region",
			Help:     "Region to connect to.",
			Provider: "AWS",
			Examples: []fs.OptionExample{{
				Value: "us-east-1",
				Help:  "The default endpoint - a good choice if you are unsure.\nUS Region, Northern Virginia, or Pacific Northwest.\nLeave location constraint empty.",
			}, {
				Value: "us-east-2",
				Help:  "US East (Ohio) Region.\nNeeds location constraint us-east-2.",
			}, {
				Value: "us-west-1",
				Help:  "US West (Northern California) Region.\nNeeds location constraint us-west-1.",
			}, {
				Value: "us-west-2",
				Help:  "US West (Oregon) Region.\nNeeds location constraint us-west-2.",
			}, {
				Value: "ca-central-1",
				Help:  "Canada (Central) Region.\nNeeds location constraint ca-central-1.",
			}, {
				Value: "eu-west-1",
				Help:  "EU (Ireland) Region.\nNeeds location constraint EU or eu-west-1.",
			}, {
				Value: "eu-west-2",
				Help:  "EU (London) Region.\nNeeds location constraint eu-west-2.",
			}, {
				Value: "eu-west-3",
				Help:  "EU (Paris) Region.\nNeeds location constraint eu-west-3.",
			}, {
				Value: "eu-north-1",
				Help:  "EU (Stockholm) Region.\nNeeds location constraint eu-north-1.",
			}, {
				Value: "eu-south-1",
				Help:  "EU (Milan) Region.\nNeeds location constraint eu-south-1.",
			}, {
				Value: "eu-central-1",
				Help:  "EU (Frankfurt) Region.\nNeeds location constraint eu-central-1.",
			}, {
				Value: "ap-southeast-1",
				Help:  "Asia Pacific (Singapore) Region.\nNeeds location constraint ap-southeast-1.",
			}, {
				Value: "ap-southeast-2",
				Help:  "Asia Pacific (Sydney) Region.\nNeeds location constraint ap-southeast-2.",
			}, {
				Value: "ap-northeast-1",
				Help:  "Asia Pacific (Tokyo) Region.\nNeeds location constraint ap-northeast-1.",
			}, {
				Value: "ap-northeast-2",
				Help:  "Asia Pacific (Seoul).\nNeeds location constraint ap-northeast-2.",
			}, {
				Value: "ap-northeast-3",
				Help:  "Asia Pacific (Osaka-Local).\nNeeds location constraint ap-northeast-3.",
			}, {
				Value: "ap-south-1",
				Help:  "Asia Pacific (Mumbai).\nNeeds location constraint ap-south-1.",
			}, {
				Value: "ap-east-1",
				Help:  "Asia Pacific (Hong Kong) Region.\nNeeds location constraint ap-east-1.",
			}, {
				Value: "sa-east-1",
				Help:  "South America (Sao Paulo) Region.\nNeeds location constraint sa-east-1.",
			}, {
				Value: "me-south-1",
				Help:  "Middle East (Bahrain) Region.\nNeeds location constraint me-south-1.",
			}, {
				Value: "af-south-1",
				Help:  "Africa (Cape Town) Region.\nNeeds location constraint af-south-1.",
			}, {
				Value: "cn-north-1",
				Help:  "China (Beijing) Region.\nNeeds location constraint cn-north-1.",
			}, {
				Value: "cn-northwest-1",
				Help:  "China (Ningxia) Region.\nNeeds location constraint cn-northwest-1.",
			}, {
				Value: "us-gov-east-1",
				Help:  "AWS GovCloud (US-East) Region.\nNeeds location constraint us-gov-east-1.",
			}, {
				Value: "us-gov-west-1",
				Help:  "AWS GovCloud (US) Region.\nNeeds location constraint us-gov-west-1.",
			}},
		}, {
			Name:     "region",
			Help:     "region - the location where your bucket will be created and your data stored.\n",
			Provider: "RackCorp",
			Examples: []fs.OptionExample{{
				Value: "global",
				Help:  "Global CDN (All locations) Region",
			}, {
				Value: "au",
				Help:  "Australia (All states)",
			}, {
				Value: "au-nsw",
				Help:  "NSW (Australia) Region",
			}, {
				Value: "au-qld",
				Help:  "QLD (Australia) Region",
			}, {
				Value: "au-vic",
				Help:  "VIC (Australia) Region",
			}, {
				Value: "au-wa",
				Help:  "Perth (Australia) Region",
			}, {
				Value: "ph",
				Help:  "Manila (Philippines) Region",
			}, {
				Value: "th",
				Help:  "Bangkok (Thailand) Region",
			}, {
				Value: "hk",
				Help:  "HK (Hong Kong) Region",
			}, {
				Value: "mn",
				Help:  "Ulaanbaatar (Mongolia) Region",
			}, {
				Value: "kg",
				Help:  "Bishkek (Kyrgyzstan) Region",
			}, {
				Value: "id",
				Help:  "Jakarta (Indonesia) Region",
			}, {
				Value: "jp",
				Help:  "Tokyo (Japan) Region",
			}, {
				Value: "sg",
				Help:  "SG (Singapore) Region",
			}, {
				Value: "de",
				Help:  "Frankfurt (Germany) Region",
			}, {
				Value: "us",
				Help:  "USA (AnyCast) Region",
			}, {
				Value: "us-east-1",
				Help:  "New York (USA) Region",
			}, {
				Value: "us-west-1",
				Help:  "Freemont (USA) Region",
			}, {
				Value: "nz",
				Help:  "Auckland (New Zealand) Region",
			}},
		}, {
			Name:     "region",
			Help:     "Region to connect to.",
			Provider: "Scaleway",
			Examples: []fs.OptionExample{{
				Value: "nl-ams",
				Help:  "Amsterdam, The Netherlands",
			}, {
				Value: "fr-par",
				Help:  "Paris, France",
			}, {
				Value: "pl-waw",
				Help:  "Warsaw, Poland",
			}},
		}, {
			Name:     "region",
			Help:     "Region to connect to. - the location where your bucket will be created and your data stored. Need bo be same with your endpoint.\n",
			Provider: "HuaweiOBS",
			Examples: []fs.OptionExample{{
				Value: "af-south-1",
				Help:  "AF-Johannesburg",
			}, {
				Value: "ap-southeast-2",
				Help:  "AP-Bangkok",
			}, {
				Value: "ap-southeast-3",
				Help:  "AP-Singapore",
			}, {
				Value: "cn-east-3",
				Help:  "CN East-Shanghai1",
			}, {
				Value: "cn-east-2",
				Help:  "CN East-Shanghai2",
			}, {
				Value: "cn-north-1",
				Help:  "CN North-Beijing1",
			}, {
				Value: "cn-north-4",
				Help:  "CN North-Beijing4",
			}, {
				Value: "cn-south-1",
				Help:  "CN South-Guangzhou",
			}, {
				Value: "ap-southeast-1",
				Help:  "CN-Hong Kong",
			}, {
				Value: "sa-argentina-1",
				Help:  "LA-Buenos Aires1",
			}, {
				Value: "sa-peru-1",
				Help:  "LA-Lima1",
			}, {
				Value: "na-mexico-1",
				Help:  "LA-Mexico City1",
			}, {
				Value: "sa-chile-1",
				Help:  "LA-Santiago2",
			}, {
				Value: "sa-brazil-1",
				Help:  "LA-Sao Paulo1",
			}, {
				Value: "ru-northwest-2",
				Help:  "RU-Moscow2",
			}},
		}, {
			Name:     "region",
			Help:     "Region to connect to.",
			Provider: "Cloudflare",
			Examples: []fs.OptionExample{{
				Value: "auto",
				Help:  "R2 buckets are automatically distributed across Cloudflare's data centers for low latency.",
			}},
		}, {
			Name:     "region",
			Help:     "Region to connect to.\n\nLeave blank if you are using an S3 clone and you don't have a region.",
			Provider: "!AWS,Alibaba,ChinaMobile,Cloudflare,ArvanCloud,RackCorp,Scaleway,Storj,TencentCOS,HuaweiOBS",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "Use this if unsure.\nWill use v4 signatures and an empty region.",
			}, {
				Value: "other-v2-signature",
				Help:  "Use this only if v4 signatures don't work.\nE.g. pre Jewel/v10 CEPH.",
			}},
		}, {
			Name:     "endpoint",
			Help:     "Endpoint for S3 API.\n\nLeave blank if using AWS to use the default endpoint for the region.",
			Provider: "AWS",
		}, {
			// ChinaMobile endpoints: https://ecloud.10086.cn/op-help-center/doc/article/24534
			Name:     "endpoint",
			Help:     "Endpoint for China Mobile Ecloud Elastic Object Storage (EOS) API.",
			Provider: "ChinaMobile",
			Examples: []fs.OptionExample{{
				Value: "eos-wuxi-1.cmecloud.cn",
				Help:  "The default endpoint - a good choice if you are unsure.\nEast China (Suzhou)",
			}, {
				Value: "eos-jinan-1.cmecloud.cn",
				Help:  "East China (Jinan)",
			}, {
				Value: "eos-ningbo-1.cmecloud.cn",
				Help:  "East China (Hangzhou)",
			}, {
				Value: "eos-shanghai-1.cmecloud.cn",
				Help:  "East China (Shanghai-1)",
			}, {
				Value: "eos-zhengzhou-1.cmecloud.cn",
				Help:  "Central China (Zhengzhou)",
			}, {
				Value: "eos-hunan-1.cmecloud.cn",
				Help:  "Central China (Changsha-1)",
			}, {
				Value: "eos-zhuzhou-1.cmecloud.cn",
				Help:  "Central China (Changsha-2)",
			}, {
				Value: "eos-guangzhou-1.cmecloud.cn",
				Help:  "South China (Guangzhou-2)",
			}, {
				Value: "eos-dongguan-1.cmecloud.cn",
				Help:  "South China (Guangzhou-3)",
			}, {
				Value: "eos-beijing-1.cmecloud.cn",
				Help:  "North China (Beijing-1)",
			}, {
				Value: "eos-beijing-2.cmecloud.cn",
				Help:  "North China (Beijing-2)",
			}, {
				Value: "eos-beijing-4.cmecloud.cn",
				Help:  "North China (Beijing-3)",
			}, {
				Value: "eos-huhehaote-1.cmecloud.cn",
				Help:  "North China (Huhehaote)",
			}, {
				Value: "eos-chengdu-1.cmecloud.cn",
				Help:  "Southwest China (Chengdu)",
			}, {
				Value: "eos-chongqing-1.cmecloud.cn",
				Help:  "Southwest China (Chongqing)",
			}, {
				Value: "eos-guiyang-1.cmecloud.cn",
				Help:  "Southwest China (Guiyang)",
			}, {
				Value: "eos-xian-1.cmecloud.cn",
				Help:  "Nouthwest China (Xian)",
			}, {
				Value: "eos-yunnan.cmecloud.cn",
				Help:  "Yunnan China (Kunming)",
			}, {
				Value: "eos-yunnan-2.cmecloud.cn",
				Help:  "Yunnan China (Kunming-2)",
			}, {
				Value: "eos-tianjin-1.cmecloud.cn",
				Help:  "Tianjin China (Tianjin)",
			}, {
				Value: "eos-jilin-1.cmecloud.cn",
				Help:  "Jilin China (Changchun)",
			}, {
				Value: "eos-hubei-1.cmecloud.cn",
				Help:  "Hubei China (Xiangyan)",
			}, {
				Value: "eos-jiangxi-1.cmecloud.cn",
				Help:  "Jiangxi China (Nanchang)",
			}, {
				Value: "eos-gansu-1.cmecloud.cn",
				Help:  "Gansu China (Lanzhou)",
			}, {
				Value: "eos-shanxi-1.cmecloud.cn",
				Help:  "Shanxi China (Taiyuan)",
			}, {
				Value: "eos-liaoning-1.cmecloud.cn",
				Help:  "Liaoning China (Shenyang)",
			}, {
				Value: "eos-hebei-1.cmecloud.cn",
				Help:  "Hebei China (Shijiazhuang)",
			}, {
				Value: "eos-fujian-1.cmecloud.cn",
				Help:  "Fujian China (Xiamen)",
			}, {
				Value: "eos-guangxi-1.cmecloud.cn",
				Help:  "Guangxi China (Nanning)",
			}, {
				Value: "eos-anhui-1.cmecloud.cn",
				Help:  "Anhui China (Huainan)",
			}},
		}, {
			// ArvanCloud endpoints: https://www.arvancloud.com/en/products/cloud-storage
			Name:     "endpoint",
			Help:     "Endpoint for Arvan Cloud Object Storage (AOS) API.",
			Provider: "ArvanCloud",
			Examples: []fs.OptionExample{{
				Value: "s3.ir-thr-at1.arvanstorage.com",
				Help:  "The default endpoint - a good choice if you are unsure.\nTehran Iran (Asiatech)",
			}, {
				Value: "s3.ir-tbz-sh1.arvanstorage.com",
				Help:  "Tabriz Iran (Shahriar)",
			}},
		}, {
			Name:     "endpoint",
			Help:     "Endpoint for IBM COS S3 API.\n\nSpecify if using an IBM COS On Premise.",
			Provider: "IBMCOS",
			Examples: []fs.OptionExample{{
				Value: "s3.us.cloud-object-storage.appdomain.cloud",
				Help:  "US Cross Region Endpoint",
			}, {
				Value: "s3.dal.us.cloud-object-storage.appdomain.cloud",
				Help:  "US Cross Region Dallas Endpoint",
			}, {
				Value: "s3.wdc.us.cloud-object-storage.appdomain.cloud",
				Help:  "US Cross Region Washington DC Endpoint",
			}, {
				Value: "s3.sjc.us.cloud-object-storage.appdomain.cloud",
				Help:  "US Cross Region San Jose Endpoint",
			}, {
				Value: "s3.private.us.cloud-object-storage.appdomain.cloud",
				Help:  "US Cross Region Private Endpoint",
			}, {
				Value: "s3.private.dal.us.cloud-object-storage.appdomain.cloud",
				Help:  "US Cross Region Dallas Private Endpoint",
			}, {
				Value: "s3.private.wdc.us.cloud-object-storage.appdomain.cloud",
				Help:  "US Cross Region Washington DC Private Endpoint",
			}, {
				Value: "s3.private.sjc.us.cloud-object-storage.appdomain.cloud",
				Help:  "US Cross Region San Jose Private Endpoint",
			}, {
				Value: "s3.us-east.cloud-object-storage.appdomain.cloud",
				Help:  "US Region East Endpoint",
			}, {
				Value: "s3.private.us-east.cloud-object-storage.appdomain.cloud",
				Help:  "US Region East Private Endpoint",
			}, {
				Value: "s3.us-south.cloud-object-storage.appdomain.cloud",
				Help:  "US Region South Endpoint",
			}, {
				Value: "s3.private.us-south.cloud-object-storage.appdomain.cloud",
				Help:  "US Region South Private Endpoint",
			}, {
				Value: "s3.eu.cloud-object-storage.appdomain.cloud",
				Help:  "EU Cross Region Endpoint",
			}, {
				Value: "s3.fra.eu.cloud-object-storage.appdomain.cloud",
				Help:  "EU Cross Region Frankfurt Endpoint",
			}, {
				Value: "s3.mil.eu.cloud-object-storage.appdomain.cloud",
				Help:  "EU Cross Region Milan Endpoint",
			}, {
				Value: "s3.ams.eu.cloud-object-storage.appdomain.cloud",
				Help:  "EU Cross Region Amsterdam Endpoint",
			}, {
				Value: "s3.private.eu.cloud-object-storage.appdomain.cloud",
				Help:  "EU Cross Region Private Endpoint",
			}, {
				Value: "s3.private.fra.eu.cloud-object-storage.appdomain.cloud",
				Help:  "EU Cross Region Frankfurt Private Endpoint",
			}, {
				Value: "s3.private.mil.eu.cloud-object-storage.appdomain.cloud",
				Help:  "EU Cross Region Milan Private Endpoint",
			}, {
				Value: "s3.private.ams.eu.cloud-object-storage.appdomain.cloud",
				Help:  "EU Cross Region Amsterdam Private Endpoint",
			}, {
				Value: "s3.eu-gb.cloud-object-storage.appdomain.cloud",
				Help:  "Great Britain Endpoint",
			}, {
				Value: "s3.private.eu-gb.cloud-object-storage.appdomain.cloud",
				Help:  "Great Britain Private Endpoint",
			}, {
				Value: "s3.eu-de.cloud-object-storage.appdomain.cloud",
				Help:  "EU Region DE Endpoint",
			}, {
				Value: "s3.private.eu-de.cloud-object-storage.appdomain.cloud",
				Help:  "EU Region DE Private Endpoint",
			}, {
				Value: "s3.ap.cloud-object-storage.appdomain.cloud",
				Help:  "APAC Cross Regional Endpoint",
			}, {
				Value: "s3.tok.ap.cloud-object-storage.appdomain.cloud",
				Help:  "APAC Cross Regional Tokyo Endpoint",
			}, {
				Value: "s3.hkg.ap.cloud-object-storage.appdomain.cloud",
				Help:  "APAC Cross Regional HongKong Endpoint",
			}, {
				Value: "s3.seo.ap.cloud-object-storage.appdomain.cloud",
				Help:  "APAC Cross Regional Seoul Endpoint",
			}, {
				Value: "s3.private.ap.cloud-object-storage.appdomain.cloud",
				Help:  "APAC Cross Regional Private Endpoint",
			}, {
				Value: "s3.private.tok.ap.cloud-object-storage.appdomain.cloud",
				Help:  "APAC Cross Regional Tokyo Private Endpoint",
			}, {
				Value: "s3.private.hkg.ap.cloud-object-storage.appdomain.cloud",
				Help:  "APAC Cross Regional HongKong Private Endpoint",
			}, {
				Value: "s3.private.seo.ap.cloud-object-storage.appdomain.cloud",
				Help:  "APAC Cross Regional Seoul Private Endpoint",
			}, {
				Value: "s3.jp-tok.cloud-object-storage.appdomain.cloud",
				Help:  "APAC Region Japan Endpoint",
			}, {
				Value: "s3.private.jp-tok.cloud-object-storage.appdomain.cloud",
				Help:  "APAC Region Japan Private Endpoint",
			}, {
				Value: "s3.au-syd.cloud-object-storage.appdomain.cloud",
				Help:  "APAC Region Australia Endpoint",
			}, {
				Value: "s3.private.au-syd.cloud-object-storage.appdomain.cloud",
				Help:  "APAC Region Australia Private Endpoint",
			}, {
				Value: "s3.ams03.cloud-object-storage.appdomain.cloud",
				Help:  "Amsterdam Single Site Endpoint",
			}, {
				Value: "s3.private.ams03.cloud-object-storage.appdomain.cloud",
				Help:  "Amsterdam Single Site Private Endpoint",
			}, {
				Value: "s3.che01.cloud-object-storage.appdomain.cloud",
				Help:  "Chennai Single Site Endpoint",
			}, {
				Value: "s3.private.che01.cloud-object-storage.appdomain.cloud",
				Help:  "Chennai Single Site Private Endpoint",
			}, {
				Value: "s3.mel01.cloud-object-storage.appdomain.cloud",
				Help:  "Melbourne Single Site Endpoint",
			}, {
				Value: "s3.private.mel01.cloud-object-storage.appdomain.cloud",
				Help:  "Melbourne Single Site Private Endpoint",
			}, {
				Value: "s3.osl01.cloud-object-storage.appdomain.cloud",
				Help:  "Oslo Single Site Endpoint",
			}, {
				Value: "s3.private.osl01.cloud-object-storage.appdomain.cloud",
				Help:  "Oslo Single Site Private Endpoint",
			}, {
				Value: "s3.tor01.cloud-object-storage.appdomain.cloud",
				Help:  "Toronto Single Site Endpoint",
			}, {
				Value: "s3.private.tor01.cloud-object-storage.appdomain.cloud",
				Help:  "Toronto Single Site Private Endpoint",
			}, {
				Value: "s3.seo01.cloud-object-storage.appdomain.cloud",
				Help:  "Seoul Single Site Endpoint",
			}, {
				Value: "s3.private.seo01.cloud-object-storage.appdomain.cloud",
				Help:  "Seoul Single Site Private Endpoint",
			}, {
				Value: "s3.mon01.cloud-object-storage.appdomain.cloud",
				Help:  "Montreal Single Site Endpoint",
			}, {
				Value: "s3.private.mon01.cloud-object-storage.appdomain.cloud",
				Help:  "Montreal Single Site Private Endpoint",
			}, {
				Value: "s3.mex01.cloud-object-storage.appdomain.cloud",
				Help:  "Mexico Single Site Endpoint",
			}, {
				Value: "s3.private.mex01.cloud-object-storage.appdomain.cloud",
				Help:  "Mexico Single Site Private Endpoint",
			}, {
				Value: "s3.sjc04.cloud-object-storage.appdomain.cloud",
				Help:  "San Jose Single Site Endpoint",
			}, {
				Value: "s3.private.sjc04.cloud-object-storage.appdomain.cloud",
				Help:  "San Jose Single Site Private Endpoint",
			}, {
				Value: "s3.mil01.cloud-object-storage.appdomain.cloud",
				Help:  "Milan Single Site Endpoint",
			}, {
				Value: "s3.private.mil01.cloud-object-storage.appdomain.cloud",
				Help:  "Milan Single Site Private Endpoint",
			}, {
				Value: "s3.hkg02.cloud-object-storage.appdomain.cloud",
				Help:  "Hong Kong Single Site Endpoint",
			}, {
				Value: "s3.private.hkg02.cloud-object-storage.appdomain.cloud",
				Help:  "Hong Kong Single Site Private Endpoint",
			}, {
				Value: "s3.par01.cloud-object-storage.appdomain.cloud",
				Help:  "Paris Single Site Endpoint",
			}, {
				Value: "s3.private.par01.cloud-object-storage.appdomain.cloud",
				Help:  "Paris Single Site Private Endpoint",
			}, {
				Value: "s3.sng01.cloud-object-storage.appdomain.cloud",
				Help:  "Singapore Single Site Endpoint",
			}, {
				Value: "s3.private.sng01.cloud-object-storage.appdomain.cloud",
				Help:  "Singapore Single Site Private Endpoint",
			}},
		}, {
			// oss endpoints: https://help.aliyun.com/document_detail/31837.html
			Name:     "endpoint",
			Help:     "Endpoint for OSS API.",
			Provider: "Alibaba",
			Examples: []fs.OptionExample{{
				Value: "oss-accelerate.aliyuncs.com",
				Help:  "Global Accelerate",
			}, {
				Value: "oss-accelerate-overseas.aliyuncs.com",
				Help:  "Global Accelerate (outside mainland China)",
			}, {
				Value: "oss-cn-hangzhou.aliyuncs.com",
				Help:  "East China 1 (Hangzhou)",
			}, {
				Value: "oss-cn-shanghai.aliyuncs.com",
				Help:  "East China 2 (Shanghai)",
			}, {
				Value: "oss-cn-qingdao.aliyuncs.com",
				Help:  "North China 1 (Qingdao)",
			}, {
				Value: "oss-cn-beijing.aliyuncs.com",
				Help:  "North China 2 (Beijing)",
			}, {
				Value: "oss-cn-zhangjiakou.aliyuncs.com",
				Help:  "North China 3 (Zhangjiakou)",
			}, {
				Value: "oss-cn-huhehaote.aliyuncs.com",
				Help:  "North China 5 (Hohhot)",
			}, {
				Value: "oss-cn-wulanchabu.aliyuncs.com",
				Help:  "North China 6 (Ulanqab)",
			}, {
				Value: "oss-cn-shenzhen.aliyuncs.com",
				Help:  "South China 1 (Shenzhen)",
			}, {
				Value: "oss-cn-heyuan.aliyuncs.com",
				Help:  "South China 2 (Heyuan)",
			}, {
				Value: "oss-cn-guangzhou.aliyuncs.com",
				Help:  "South China 3 (Guangzhou)",
			}, {
				Value: "oss-cn-chengdu.aliyuncs.com",
				Help:  "West China 1 (Chengdu)",
			}, {
				Value: "oss-cn-hongkong.aliyuncs.com",
				Help:  "Hong Kong (Hong Kong)",
			}, {
				Value: "oss-us-west-1.aliyuncs.com",
				Help:  "US West 1 (Silicon Valley)",
			}, {
				Value: "oss-us-east-1.aliyuncs.com",
				Help:  "US East 1 (Virginia)",
			}, {
				Value: "oss-ap-southeast-1.aliyuncs.com",
				Help:  "Southeast Asia Southeast 1 (Singapore)",
			}, {
				Value: "oss-ap-southeast-2.aliyuncs.com",
				Help:  "Asia Pacific Southeast 2 (Sydney)",
			}, {
				Value: "oss-ap-southeast-3.aliyuncs.com",
				Help:  "Southeast Asia Southeast 3 (Kuala Lumpur)",
			}, {
				Value: "oss-ap-southeast-5.aliyuncs.com",
				Help:  "Asia Pacific Southeast 5 (Jakarta)",
			}, {
				Value: "oss-ap-northeast-1.aliyuncs.com",
				Help:  "Asia Pacific Northeast 1 (Japan)",
			}, {
				Value: "oss-ap-south-1.aliyuncs.com",
				Help:  "Asia Pacific South 1 (Mumbai)",
			}, {
				Value: "oss-eu-central-1.aliyuncs.com",
				Help:  "Central Europe 1 (Frankfurt)",
			}, {
				Value: "oss-eu-west-1.aliyuncs.com",
				Help:  "West Europe (London)",
			}, {
				Value: "oss-me-east-1.aliyuncs.com",
				Help:  "Middle East 1 (Dubai)",
			}},
		}, {
			// obs endpoints: https://developer.huaweicloud.com/intl/en-us/endpoint?OBS
			Name:     "endpoint",
			Help:     "Endpoint for OBS API.",
			Provider: "HuaweiOBS",
			Examples: []fs.OptionExample{{
				Value: "obs.af-south-1.myhuaweicloud.com",
				Help:  "AF-Johannesburg",
			}, {
				Value: "obs.ap-southeast-2.myhuaweicloud.com",
				Help:  "AP-Bangkok",
			}, {
				Value: "obs.ap-southeast-3.myhuaweicloud.com",
				Help:  "AP-Singapore",
			}, {
				Value: "obs.cn-east-3.myhuaweicloud.com",
				Help:  "CN East-Shanghai1",
			}, {
				Value: "obs.cn-east-2.myhuaweicloud.com",
				Help:  "CN East-Shanghai2",
			}, {
				Value: "obs.cn-north-1.myhuaweicloud.com",
				Help:  "CN North-Beijing1",
			}, {
				Value: "obs.cn-north-4.myhuaweicloud.com",
				Help:  "CN North-Beijing4",
			}, {
				Value: "obs.cn-south-1.myhuaweicloud.com",
				Help:  "CN South-Guangzhou",
			}, {
				Value: "obs.ap-southeast-1.myhuaweicloud.com",
				Help:  "CN-Hong Kong",
			}, {
				Value: "obs.sa-argentina-1.myhuaweicloud.com",
				Help:  "LA-Buenos Aires1",
			}, {
				Value: "obs.sa-peru-1.myhuaweicloud.com",
				Help:  "LA-Lima1",
			}, {
				Value: "obs.na-mexico-1.myhuaweicloud.com",
				Help:  "LA-Mexico City1",
			}, {
				Value: "obs.sa-chile-1.myhuaweicloud.com",
				Help:  "LA-Santiago2",
			}, {
				Value: "obs.sa-brazil-1.myhuaweicloud.com",
				Help:  "LA-Sao Paulo1",
			}, {
				Value: "obs.ru-northwest-2.myhuaweicloud.com",
				Help:  "RU-Moscow2",
			}},
		}, {
			Name:     "endpoint",
			Help:     "Endpoint for Scaleway Object Storage.",
			Provider: "Scaleway",
			Examples: []fs.OptionExample{{
				Value: "s3.nl-ams.scw.cloud",
				Help:  "Amsterdam Endpoint",
			}, {
				Value: "s3.fr-par.scw.cloud",
				Help:  "Paris Endpoint",
			}, {
				Value: "s3.pl-waw.scw.cloud",
				Help:  "Warsaw Endpoint",
			}},
		}, {
			Name:     "endpoint",
			Help:     "Endpoint for StackPath Object Storage.",
			Provider: "StackPath",
			Examples: []fs.OptionExample{{
				Value: "s3.us-east-2.stackpathstorage.com",
				Help:  "US East Endpoint",
			}, {
				Value: "s3.us-west-1.stackpathstorage.com",
				Help:  "US West Endpoint",
			}, {
				Value: "s3.eu-central-1.stackpathstorage.com",
				Help:  "EU Endpoint",
			}},
		}, {
			Name:     "endpoint",
			Help:     "Endpoint of the Shared Gateway.",
			Provider: "Storj",
			Examples: []fs.OptionExample{{
				Value: "gateway.eu1.storjshare.io",
				Help:  "EU1 Shared Gateway",
			}, {
				Value: "gateway.us1.storjshare.io",
				Help:  "US1 Shared Gateway",
			}, {
				Value: "gateway.ap1.storjshare.io",
				Help:  "Asia-Pacific Shared Gateway",
			}},
		}, {
			// cos endpoints: https://intl.cloud.tencent.com/document/product/436/6224
			Name:     "endpoint",
			Help:     "Endpoint for Tencent COS API.",
			Provider: "TencentCOS",
			Examples: []fs.OptionExample{{
				Value: "cos.ap-beijing.myqcloud.com",
				Help:  "Beijing Region",
			}, {
				Value: "cos.ap-nanjing.myqcloud.com",
				Help:  "Nanjing Region",
			}, {
				Value: "cos.ap-shanghai.myqcloud.com",
				Help:  "Shanghai Region",
			}, {
				Value: "cos.ap-guangzhou.myqcloud.com",
				Help:  "Guangzhou Region",
			}, {
				Value: "cos.ap-nanjing.myqcloud.com",
				Help:  "Nanjing Region",
			}, {
				Value: "cos.ap-chengdu.myqcloud.com",
				Help:  "Chengdu Region",
			}, {
				Value: "cos.ap-chongqing.myqcloud.com",
				Help:  "Chongqing Region",
			}, {
				Value: "cos.ap-hongkong.myqcloud.com",
				Help:  "Hong Kong (China) Region",
			}, {
				Value: "cos.ap-singapore.myqcloud.com",
				Help:  "Singapore Region",
			}, {
				Value: "cos.ap-mumbai.myqcloud.com",
				Help:  "Mumbai Region",
			}, {
				Value: "cos.ap-seoul.myqcloud.com",
				Help:  "Seoul Region",
			}, {
				Value: "cos.ap-bangkok.myqcloud.com",
				Help:  "Bangkok Region",
			}, {
				Value: "cos.ap-tokyo.myqcloud.com",
				Help:  "Tokyo Region",
			}, {
				Value: "cos.na-siliconvalley.myqcloud.com",
				Help:  "Silicon Valley Region",
			}, {
				Value: "cos.na-ashburn.myqcloud.com",
				Help:  "Virginia Region",
			}, {
				Value: "cos.na-toronto.myqcloud.com",
				Help:  "Toronto Region",
			}, {
				Value: "cos.eu-frankfurt.myqcloud.com",
				Help:  "Frankfurt Region",
			}, {
				Value: "cos.eu-moscow.myqcloud.com",
				Help:  "Moscow Region",
			}, {
				Value: "cos.accelerate.myqcloud.com",
				Help:  "Use Tencent COS Accelerate Endpoint",
			}},
		}, {
			// RackCorp endpoints: https://www.rackcorp.com/storage/s3storage
			Name:     "endpoint",
			Help:     "Endpoint for RackCorp Object Storage.",
			Provider: "RackCorp",
			Examples: []fs.OptionExample{{
				Value: "s3.rackcorp.com",
				Help:  "Global (AnyCast) Endpoint",
			}, {
				Value: "au.s3.rackcorp.com",
				Help:  "Australia (Anycast) Endpoint",
			}, {
				Value: "au-nsw.s3.rackcorp.com",
				Help:  "Sydney (Australia) Endpoint",
			}, {
				Value: "au-qld.s3.rackcorp.com",
				Help:  "Brisbane (Australia) Endpoint",
			}, {
				Value: "au-vic.s3.rackcorp.com",
				Help:  "Melbourne (Australia) Endpoint",
			}, {
				Value: "au-wa.s3.rackcorp.com",
				Help:  "Perth (Australia) Endpoint",
			}, {
				Value: "ph.s3.rackcorp.com",
				Help:  "Manila (Philippines) Endpoint",
			}, {
				Value: "th.s3.rackcorp.com",
				Help:  "Bangkok (Thailand) Endpoint",
			}, {
				Value: "hk.s3.rackcorp.com",
				Help:  "HK (Hong Kong) Endpoint",
			}, {
				Value: "mn.s3.rackcorp.com",
				Help:  "Ulaanbaatar (Mongolia) Endpoint",
			}, {
				Value: "kg.s3.rackcorp.com",
				Help:  "Bishkek (Kyrgyzstan) Endpoint",
			}, {
				Value: "id.s3.rackcorp.com",
				Help:  "Jakarta (Indonesia) Endpoint",
			}, {
				Value: "jp.s3.rackcorp.com",
				Help:  "Tokyo (Japan) Endpoint",
			}, {
				Value: "sg.s3.rackcorp.com",
				Help:  "SG (Singapore) Endpoint",
			}, {
				Value: "de.s3.rackcorp.com",
				Help:  "Frankfurt (Germany) Endpoint",
			}, {
				Value: "us.s3.rackcorp.com",
				Help:  "USA (AnyCast) Endpoint",
			}, {
				Value: "us-east-1.s3.rackcorp.com",
				Help:  "New York (USA) Endpoint",
			}, {
				Value: "us-west-1.s3.rackcorp.com",
				Help:  "Freemont (USA) Endpoint",
			}, {
				Value: "nz.s3.rackcorp.com",
				Help:  "Auckland (New Zealand) Endpoint",
			}},
		}, {
			Name:     "endpoint",
			Help:     "Endpoint for S3 API.\n\nRequired when using an S3 clone.",
			Provider: "!AWS,IBMCOS,TencentCOS,HuaweiOBS,Alibaba,ChinaMobile,ArvanCloud,Scaleway,StackPath,Storj,RackCorp",
			Examples: []fs.OptionExample{{
				Value:    "objects-us-east-1.dream.io",
				Help:     "Dream Objects endpoint",
				Provider: "Dreamhost",
			}, {
				Value:    "nyc3.digitaloceanspaces.com",
				Help:     "Digital Ocean Spaces New York 3",
				Provider: "DigitalOcean",
			}, {
				Value:    "ams3.digitaloceanspaces.com",
				Help:     "Digital Ocean Spaces Amsterdam 3",
				Provider: "DigitalOcean",
			}, {
				Value:    "sgp1.digitaloceanspaces.com",
				Help:     "Digital Ocean Spaces Singapore 1",
				Provider: "DigitalOcean",
			}, {
				Value:    "localhost:8333",
				Help:     "SeaweedFS S3 localhost",
				Provider: "SeaweedFS",
			}, {
				Value:    "s3.us-east-1.lyvecloud.seagate.com",
				Help:     "Seagate Lyve Cloud US East 1 (Virginia)",
				Provider: "LyveCloud",
			}, {
				Value:    "s3.us-west-1.lyvecloud.seagate.com",
				Help:     "Seagate Lyve Cloud US West 1 (California)",
				Provider: "LyveCloud",
			}, {
				Value:    "s3.ap-southeast-1.lyvecloud.seagate.com",
				Help:     "Seagate Lyve Cloud AP Southeast 1 (Singapore)",
				Provider: "LyveCloud",
			}, {
				Value:    "s3.wasabisys.com",
				Help:     "Wasabi US East endpoint",
				Provider: "Wasabi",
			}, {
				Value:    "s3.us-west-1.wasabisys.com",
				Help:     "Wasabi US West endpoint",
				Provider: "Wasabi",
			}, {
				Value:    "s3.eu-central-1.wasabisys.com",
				Help:     "Wasabi EU Central endpoint",
				Provider: "Wasabi",
			}, {
				Value:    "s3.ap-northeast-1.wasabisys.com",
				Help:     "Wasabi AP Northeast 1 (Tokyo) endpoint",
				Provider: "Wasabi",
			}, {
				Value:    "s3.ap-northeast-2.wasabisys.com",
				Help:     "Wasabi AP Northeast 2 (Osaka) endpoint",
				Provider: "Wasabi",
			}, {
				Value:    "s3.ir-thr-at1.arvanstorage.com",
				Help:     "ArvanCloud Tehran Iran (Asiatech) endpoint",
				Provider: "ArvanCloud",
			}},
		}, {
			Name:     "location_constraint",
			Help:     "Location constraint - must be set to match the Region.\n\nUsed when creating buckets only.",
			Provider: "AWS",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "Empty for US Region, Northern Virginia, or Pacific Northwest",
			}, {
				Value: "us-east-2",
				Help:  "US East (Ohio) Region",
			}, {
				Value: "us-west-1",
				Help:  "US West (Northern California) Region",
			}, {
				Value: "us-west-2",
				Help:  "US West (Oregon) Region",
			}, {
				Value: "ca-central-1",
				Help:  "Canada (Central) Region",
			}, {
				Value: "eu-west-1",
				Help:  "EU (Ireland) Region",
			}, {
				Value: "eu-west-2",
				Help:  "EU (London) Region",
			}, {
				Value: "eu-west-3",
				Help:  "EU (Paris) Region",
			}, {
				Value: "eu-north-1",
				Help:  "EU (Stockholm) Region",
			}, {
				Value: "eu-south-1",
				Help:  "EU (Milan) Region",
			}, {
				Value: "EU",
				Help:  "EU Region",
			}, {
				Value: "ap-southeast-1",
				Help:  "Asia Pacific (Singapore) Region",
			}, {
				Value: "ap-southeast-2",
				Help:  "Asia Pacific (Sydney) Region",
			}, {
				Value: "ap-northeast-1",
				Help:  "Asia Pacific (Tokyo) Region",
			}, {
				Value: "ap-northeast-2",
				Help:  "Asia Pacific (Seoul) Region",
			}, {
				Value: "ap-northeast-3",
				Help:  "Asia Pacific (Osaka-Local) Region",
			}, {
				Value: "ap-south-1",
				Help:  "Asia Pacific (Mumbai) Region",
			}, {
				Value: "ap-east-1",
				Help:  "Asia Pacific (Hong Kong) Region",
			}, {
				Value: "sa-east-1",
				Help:  "South America (Sao Paulo) Region",
			}, {
				Value: "me-south-1",
				Help:  "Middle East (Bahrain) Region",
			}, {
				Value: "af-south-1",
				Help:  "Africa (Cape Town) Region",
			}, {
				Value: "cn-north-1",
				Help:  "China (Beijing) Region",
			}, {
				Value: "cn-northwest-1",
				Help:  "China (Ningxia) Region",
			}, {
				Value: "us-gov-east-1",
				Help:  "AWS GovCloud (US-East) Region",
			}, {
				Value: "us-gov-west-1",
				Help:  "AWS GovCloud (US) Region",
			}},
		}, {
			Name:     "location_constraint",
			Help:     "Location constraint - must match endpoint.\n\nUsed when creating buckets only.",
			Provider: "ChinaMobile",
			Examples: []fs.OptionExample{{
				Value: "wuxi1",
				Help:  "East China (Suzhou)",
			}, {
				Value: "jinan1",
				Help:  "East China (Jinan)",
			}, {
				Value: "ningbo1",
				Help:  "East China (Hangzhou)",
			}, {
				Value: "shanghai1",
				Help:  "East China (Shanghai-1)",
			}, {
				Value: "zhengzhou1",
				Help:  "Central China (Zhengzhou)",
			}, {
				Value: "hunan1",
				Help:  "Central China (Changsha-1)",
			}, {
				Value: "zhuzhou1",
				Help:  "Central China (Changsha-2)",
			}, {
				Value: "guangzhou1",
				Help:  "South China (Guangzhou-2)",
			}, {
				Value: "dongguan1",
				Help:  "South China (Guangzhou-3)",
			}, {
				Value: "beijing1",
				Help:  "North China (Beijing-1)",
			}, {
				Value: "beijing2",
				Help:  "North China (Beijing-2)",
			}, {
				Value: "beijing4",
				Help:  "North China (Beijing-3)",
			}, {
				Value: "huhehaote1",
				Help:  "North China (Huhehaote)",
			}, {
				Value: "chengdu1",
				Help:  "Southwest China (Chengdu)",
			}, {
				Value: "chongqing1",
				Help:  "Southwest China (Chongqing)",
			}, {
				Value: "guiyang1",
				Help:  "Southwest China (Guiyang)",
			}, {
				Value: "xian1",
				Help:  "Nouthwest China (Xian)",
			}, {
				Value: "yunnan",
				Help:  "Yunnan China (Kunming)",
			}, {
				Value: "yunnan2",
				Help:  "Yunnan China (Kunming-2)",
			}, {
				Value: "tianjin1",
				Help:  "Tianjin China (Tianjin)",
			}, {
				Value: "jilin1",
				Help:  "Jilin China (Changchun)",
			}, {
				Value: "hubei1",
				Help:  "Hubei China (Xiangyan)",
			}, {
				Value: "jiangxi1",
				Help:  "Jiangxi China (Nanchang)",
			}, {
				Value: "gansu1",
				Help:  "Gansu China (Lanzhou)",
			}, {
				Value: "shanxi1",
				Help:  "Shanxi China (Taiyuan)",
			}, {
				Value: "liaoning1",
				Help:  "Liaoning China (Shenyang)",
			}, {
				Value: "hebei1",
				Help:  "Hebei China (Shijiazhuang)",
			}, {
				Value: "fujian1",
				Help:  "Fujian China (Xiamen)",
			}, {
				Value: "guangxi1",
				Help:  "Guangxi China (Nanning)",
			}, {
				Value: "anhui1",
				Help:  "Anhui China (Huainan)",
			}},
		}, {
			Name:     "location_constraint",
			Help:     "Location constraint - must match endpoint.\n\nUsed when creating buckets only.",
			Provider: "ArvanCloud",
			Examples: []fs.OptionExample{{
				Value: "ir-thr-at1",
				Help:  "Tehran Iran (Asiatech)",
			}, {
				Value: "ir-tbz-sh1",
				Help:  "Tabriz Iran (Shahriar)",
			}},
		}, {
			Name:     "location_constraint",
			Help:     "Location constraint - must match endpoint when using IBM Cloud Public.\n\nFor on-prem COS, do not make a selection from this list, hit enter.",
			Provider: "IBMCOS",
			Examples: []fs.OptionExample{{
				Value: "us-standard",
				Help:  "US Cross Region Standard",
			}, {
				Value: "us-vault",
				Help:  "US Cross Region Vault",
			}, {
				Value: "us-cold",
				Help:  "US Cross Region Cold",
			}, {
				Value: "us-flex",
				Help:  "US Cross Region Flex",
			}, {
				Value: "us-east-standard",
				Help:  "US East Region Standard",
			}, {
				Value: "us-east-vault",
				Help:  "US East Region Vault",
			}, {
				Value: "us-east-cold",
				Help:  "US East Region Cold",
			}, {
				Value: "us-east-flex",
				Help:  "US East Region Flex",
			}, {
				Value: "us-south-standard",
				Help:  "US South Region Standard",
			}, {
				Value: "us-south-vault",
				Help:  "US South Region Vault",
			}, {
				Value: "us-south-cold",
				Help:  "US South Region Cold",
			}, {
				Value: "us-south-flex",
				Help:  "US South Region Flex",
			}, {
				Value: "eu-standard",
				Help:  "EU Cross Region Standard",
			}, {
				Value: "eu-vault",
				Help:  "EU Cross Region Vault",
			}, {
				Value: "eu-cold",
				Help:  "EU Cross Region Cold",
			}, {
				Value: "eu-flex",
				Help:  "EU Cross Region Flex",
			}, {
				Value: "eu-gb-standard",
				Help:  "Great Britain Standard",
			}, {
				Value: "eu-gb-vault",
				Help:  "Great Britain Vault",
			}, {
				Value: "eu-gb-cold",
				Help:  "Great Britain Cold",
			}, {
				Value: "eu-gb-flex",
				Help:  "Great Britain Flex",
			}, {
				Value: "ap-standard",
				Help:  "APAC Standard",
			}, {
				Value: "ap-vault",
				Help:  "APAC Vault",
			}, {
				Value: "ap-cold",
				Help:  "APAC Cold",
			}, {
				Value: "ap-flex",
				Help:  "APAC Flex",
			}, {
				Value: "mel01-standard",
				Help:  "Melbourne Standard",
			}, {
				Value: "mel01-vault",
				Help:  "Melbourne Vault",
			}, {
				Value: "mel01-cold",
				Help:  "Melbourne Cold",
			}, {
				Value: "mel01-flex",
				Help:  "Melbourne Flex",
			}, {
				Value: "tor01-standard",
				Help:  "Toronto Standard",
			}, {
				Value: "tor01-vault",
				Help:  "Toronto Vault",
			}, {
				Value: "tor01-cold",
				Help:  "Toronto Cold",
			}, {
				Value: "tor01-flex",
				Help:  "Toronto Flex",
			}},
		}, {
			Name:     "location_constraint",
			Help:     "Location constraint - the location where your bucket will be located and your data stored.\n",
			Provider: "RackCorp",
			Examples: []fs.OptionExample{{
				Value: "global",
				Help:  "Global CDN Region",
			}, {
				Value: "au",
				Help:  "Australia (All locations)",
			}, {
				Value: "au-nsw",
				Help:  "NSW (Australia) Region",
			}, {
				Value: "au-qld",
				Help:  "QLD (Australia) Region",
			}, {
				Value: "au-vic",
				Help:  "VIC (Australia) Region",
			}, {
				Value: "au-wa",
				Help:  "Perth (Australia) Region",
			}, {
				Value: "ph",
				Help:  "Manila (Philippines) Region",
			}, {
				Value: "th",
				Help:  "Bangkok (Thailand) Region",
			}, {
				Value: "hk",
				Help:  "HK (Hong Kong) Region",
			}, {
				Value: "mn",
				Help:  "Ulaanbaatar (Mongolia) Region",
			}, {
				Value: "kg",
				Help:  "Bishkek (Kyrgyzstan) Region",
			}, {
				Value: "id",
				Help:  "Jakarta (Indonesia) Region",
			}, {
				Value: "jp",
				Help:  "Tokyo (Japan) Region",
			}, {
				Value: "sg",
				Help:  "SG (Singapore) Region",
			}, {
				Value: "de",
				Help:  "Frankfurt (Germany) Region",
			}, {
				Value: "us",
				Help:  "USA (AnyCast) Region",
			}, {
				Value: "us-east-1",
				Help:  "New York (USA) Region",
			}, {
				Value: "us-west-1",
				Help:  "Freemont (USA) Region",
			}, {
				Value: "nz",
				Help:  "Auckland (New Zealand) Region",
			}},
		}, {
			Name:     "location_constraint",
			Help:     "Location constraint - must be set to match the Region.\n\nLeave blank if not sure. Used when creating buckets only.",
			Provider: "!AWS,IBMCOS,Alibaba,HuaweiOBS,ChinaMobile,Cloudflare,ArvanCloud,RackCorp,Scaleway,StackPath,Storj,TencentCOS",
		}, {
			Name: "acl",
			Help: `Canned ACL used when creating buckets and storing or copying objects.

This ACL is used for creating objects and if bucket_acl isn't set, for creating buckets too.

For more info visit https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl

Note that this ACL is applied when server-side copying objects as S3
doesn't copy the ACL from the source but rather writes a fresh one.`,
			Provider: "!Storj,Cloudflare",
			Examples: []fs.OptionExample{{
				Value:    "default",
				Help:     "Owner gets Full_CONTROL.\nNo one else has access rights (default).",
				Provider: "TencentCOS",
			}, {
				Value:    "private",
				Help:     "Owner gets FULL_CONTROL.\nNo one else has access rights (default).",
				Provider: "!IBMCOS,TencentCOS",
			}, {
				Value:    "public-read",
				Help:     "Owner gets FULL_CONTROL.\nThe AllUsers group gets READ access.",
				Provider: "!IBMCOS",
			}, {
				Value:    "public-read-write",
				Help:     "Owner gets FULL_CONTROL.\nThe AllUsers group gets READ and WRITE access.\nGranting this on a bucket is generally not recommended.",
				Provider: "!IBMCOS",
			}, {
				Value:    "authenticated-read",
				Help:     "Owner gets FULL_CONTROL.\nThe AuthenticatedUsers group gets READ access.",
				Provider: "!IBMCOS",
			}, {
				Value:    "bucket-owner-read",
				Help:     "Object owner gets FULL_CONTROL.\nBucket owner gets READ access.\nIf you specify this canned ACL when creating a bucket, Amazon S3 ignores it.",
				Provider: "!IBMCOS,ChinaMobile",
			}, {
				Value:    "bucket-owner-full-control",
				Help:     "Both the object owner and the bucket owner get FULL_CONTROL over the object.\nIf you specify this canned ACL when creating a bucket, Amazon S3 ignores it.",
				Provider: "!IBMCOS,ChinaMobile",
			}, {
				Value:    "private",
				Help:     "Owner gets FULL_CONTROL.\nNo one else has access rights (default).\nThis acl is available on IBM Cloud (Infra), IBM Cloud (Storage), On-Premise COS.",
				Provider: "IBMCOS",
			}, {
				Value:    "public-read",
				Help:     "Owner gets FULL_CONTROL.\nThe AllUsers group gets READ access.\nThis acl is available on IBM Cloud (Infra), IBM Cloud (Storage), On-Premise IBM COS.",
				Provider: "IBMCOS",
			}, {
				Value:    "public-read-write",
				Help:     "Owner gets FULL_CONTROL.\nThe AllUsers group gets READ and WRITE access.\nThis acl is available on IBM Cloud (Infra), On-Premise IBM COS.",
				Provider: "IBMCOS",
			}, {
				Value:    "authenticated-read",
				Help:     "Owner gets FULL_CONTROL.\nThe AuthenticatedUsers group gets READ access.\nNot supported on Buckets.\nThis acl is available on IBM Cloud (Infra) and On-Premise IBM COS.",
				Provider: "IBMCOS",
			}},
		}, {
			Name: "bucket_acl",
			Help: `Canned ACL used when creating buckets.

For more info visit https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl

Note that this ACL is applied when only when creating buckets.  If it
isn't set then "acl" is used instead.`,
			Advanced: true,
			Examples: []fs.OptionExample{{
				Value: "private",
				Help:  "Owner gets FULL_CONTROL.\nNo one else has access rights (default).",
			}, {
				Value: "public-read",
				Help:  "Owner gets FULL_CONTROL.\nThe AllUsers group gets READ access.",
			}, {
				Value: "public-read-write",
				Help:  "Owner gets FULL_CONTROL.\nThe AllUsers group gets READ and WRITE access.\nGranting this on a bucket is generally not recommended.",
			}, {
				Value: "authenticated-read",
				Help:  "Owner gets FULL_CONTROL.\nThe AuthenticatedUsers group gets READ access.",
			}},
		}, {
			Name:     "requester_pays",
			Help:     "Enables requester pays option when interacting with S3 bucket.",
			Provider: "AWS",
			Default:  false,
			Advanced: true,
		}, {
			Name:     "server_side_encryption",
			Help:     "The server-side encryption algorithm used when storing this object in S3.",
			Provider: "AWS,Ceph,ChinaMobile,Minio",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "None",
			}, {
				Value: "AES256",
				Help:  "AES256",
			}, {
				Value:    "aws:kms",
				Help:     "aws:kms",
				Provider: "!ChinaMobile",
			}},
		}, {
			Name:     "sse_customer_algorithm",
			Help:     "If using SSE-C, the server-side encryption algorithm used when storing this object in S3.",
			Provider: "AWS,Ceph,ChinaMobile,Minio",
			Advanced: true,
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "None",
			}, {
				Value: "AES256",
				Help:  "AES256",
			}},
		}, {
			Name:     "sse_kms_key_id",
			Help:     "If using KMS ID you must provide the ARN of Key.",
			Provider: "AWS,Ceph,Minio",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "None",
			}, {
				Value: "arn:aws:kms:us-east-1:*",
				Help:  "arn:aws:kms:*",
			}},
		}, {
			Name:     "sse_customer_key",
			Help:     "If using SSE-C you must provide the secret encryption key used to encrypt/decrypt your data.",
			Provider: "AWS,Ceph,ChinaMobile,Minio",
			Advanced: true,
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "None",
			}},
		}, {
			Name: "sse_customer_key_md5",
			Help: `If using SSE-C you may provide the secret encryption key MD5 checksum (optional).

If you leave it blank, this is calculated automatically from the sse_customer_key provided.
`,
			Provider: "AWS,Ceph,ChinaMobile,Minio",
			Advanced: true,
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "None",
			}},
		}, {
			Name:     "storage_class",
			Help:     "The storage class to use when storing new objects in S3.",
			Provider: "AWS",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "Default",
			}, {
				Value: "STANDARD",
				Help:  "Standard storage class",
			}, {
				Value: "REDUCED_REDUNDANCY",
				Help:  "Reduced redundancy storage class",
			}, {
				Value: "STANDARD_IA",
				Help:  "Standard Infrequent Access storage class",
			}, {
				Value: "ONEZONE_IA",
				Help:  "One Zone Infrequent Access storage class",
			}, {
				Value: "GLACIER",
				Help:  "Glacier storage class",
			}, {
				Value: "DEEP_ARCHIVE",
				Help:  "Glacier Deep Archive storage class",
			}, {
				Value: "INTELLIGENT_TIERING",
				Help:  "Intelligent-Tiering storage class",
			}, {
				Value: "GLACIER_IR",
				Help:  "Glacier Instant Retrieval storage class",
			}},
		}, {
			// Mapping from here: https://www.alibabacloud.com/help/doc-detail/64919.htm
			Name:     "storage_class",
			Help:     "The storage class to use when storing new objects in OSS.",
			Provider: "Alibaba",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "Default",
			}, {
				Value: "STANDARD",
				Help:  "Standard storage class",
			}, {
				Value: "GLACIER",
				Help:  "Archive storage mode",
			}, {
				Value: "STANDARD_IA",
				Help:  "Infrequent access storage mode",
			}},
		}, {
			// Mapping from here: https://ecloud.10086.cn/op-help-center/doc/article/24495
			Name:     "storage_class",
			Help:     "The storage class to use when storing new objects in ChinaMobile.",
			Provider: "ChinaMobile",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "Default",
			}, {
				Value: "STANDARD",
				Help:  "Standard storage class",
			}, {
				Value: "GLACIER",
				Help:  "Archive storage mode",
			}, {
				Value: "STANDARD_IA",
				Help:  "Infrequent access storage mode",
			}},
		}, {
			// Mapping from here: https://www.arvancloud.com/en/products/cloud-storage
			Name:     "storage_class",
			Help:     "The storage class to use when storing new objects in ArvanCloud.",
			Provider: "ArvanCloud",
			Examples: []fs.OptionExample{{
				Value: "STANDARD",
				Help:  "Standard storage class",
			}},
		}, {
			// Mapping from here: https://intl.cloud.tencent.com/document/product/436/30925
			Name:     "storage_class",
			Help:     "The storage class to use when storing new objects in Tencent COS.",
			Provider: "TencentCOS",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "Default",
			}, {
				Value: "STANDARD",
				Help:  "Standard storage class",
			}, {
				Value: "ARCHIVE",
				Help:  "Archive storage mode",
			}, {
				Value: "STANDARD_IA",
				Help:  "Infrequent access storage mode",
			}},
		}, {
			// Mapping from here: https://www.scaleway.com/en/docs/object-storage-glacier/#-Scaleway-Storage-Classes
			Name:     "storage_class",
			Help:     "The storage class to use when storing new objects in S3.",
			Provider: "Scaleway",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "Default.",
			}, {
				Value: "STANDARD",
				Help:  "The Standard class for any upload.\nSuitable for on-demand content like streaming or CDN.",
			}, {
				Value: "GLACIER",
				Help:  "Archived storage.\nPrices are lower, but it needs to be restored first to be accessed.",
			}},
		}, {
			Name: "upload_cutoff",
			Help: `Cutoff for switching to chunked upload.

Any files larger than this will be uploaded in chunks of chunk_size.
The minimum is 0 and the maximum is 5 GiB.`,
			Default:  defaultUploadCutoff,
			Advanced: true,
		}, {
			Name: "chunk_size",
			Help: `Chunk size to use for uploading.

When uploading files larger than upload_cutoff or files with unknown
size (e.g. from "rclone rcat" or uploaded with "rclone mount" or google
photos or google docs) they will be uploaded as multipart uploads
using this chunk size.

Note that "--s3-upload-concurrency" chunks of this size are buffered
in memory per transfer.

If you are transferring large files over high-speed links and you have
enough memory, then increasing this will speed up the transfers.

Rclone will automatically increase the chunk size when uploading a
large file of known size to stay below the 10,000 chunks limit.

Files of unknown size are uploaded with the configured
chunk_size. Since the default chunk size is 5 MiB and there can be at
most 10,000 chunks, this means that by default the maximum size of
a file you can stream upload is 48 GiB.  If you wish to stream upload
larger files then you will need to increase chunk_size.`,
			Default:  minChunkSize,
			Advanced: true,
		}, {
			Name: "max_upload_parts",
			Help: `Maximum number of parts in a multipart upload.

This option defines the maximum number of multipart chunks to use
when doing a multipart upload.

This can be useful if a service does not support the AWS S3
specification of 10,000 chunks.

Rclone will automatically increase the chunk size when uploading a
large file of a known size to stay below this number of chunks limit.
`,
			Default:  maxUploadParts,
			Advanced: true,
		}, {
			Name: "copy_cutoff",
			Help: `Cutoff for switching to multipart copy.

Any files larger than this that need to be server-side copied will be
copied in chunks of this size.

The minimum is 0 and the maximum is 5 GiB.`,
			Default:  fs.SizeSuffix(maxSizeForCopy),
			Advanced: true,
		}, {
			Name: "disable_checksum",
			Help: `Don't store MD5 checksum with object metadata.

Normally rclone will calculate the MD5 checksum of the input before
uploading it so it can add it to metadata on the object. This is great
for data integrity checking but can cause long delays for large files
to start uploading.`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "shared_credentials_file",
			Help: `Path to the shared credentials file.

If env_auth = true then rclone can use a shared credentials file.

If this variable is empty rclone will look for the
"AWS_SHARED_CREDENTIALS_FILE" env variable. If the env value is empty
it will default to the current user's home directory.

    Linux/OSX: "$HOME/.aws/credentials"
    Windows:   "%USERPROFILE%\.aws\credentials"
`,
			Advanced: true,
		}, {
			Name: "profile",
			Help: `Profile to use in the shared credentials file.

If env_auth = true then rclone can use a shared credentials file. This
variable controls which profile is used in that file.

If empty it will default to the environment variable "AWS_PROFILE" or
"default" if that environment variable is also not set.
`,
			Advanced: true,
		}, {
			Name:     "session_token",
			Help:     "An AWS session token.",
			Advanced: true,
		}, {
			Name: "upload_concurrency",
			Help: `Concurrency for multipart uploads.

This is the number of chunks of the same file that are uploaded
concurrently.

If you are uploading small numbers of large files over high-speed links
and these uploads do not fully utilize your bandwidth, then increasing
this may help to speed up the transfers.`,
			Default:  4,
			Advanced: true,
		}, {
			Name: "force_path_style",
			Help: `If true use path style access if false use virtual hosted style.

If this is true (the default) then rclone will use path style access,
if false then rclone will use virtual path style. See [the AWS S3
docs](https://docs.aws.amazon.com/AmazonS3/latest/dev/UsingBucket.html#access-bucket-intro)
for more info.

Some providers (e.g. AWS, Aliyun OSS, Netease COS, or Tencent COS) require this set to
false - rclone will do this automatically based on the provider
setting.`,
			Default:  true,
			Advanced: true,
		}, {
			Name: "v2_auth",
			Help: `If true use v2 authentication.

If this is false (the default) then rclone will use v4 authentication.
If it is set then rclone will use v2 authentication.

Use this only if v4 signatures don't work, e.g. pre Jewel/v10 CEPH.`,
			Default:  false,
			Advanced: true,
		}, {
			Name:     "use_accelerate_endpoint",
			Provider: "AWS",
			Help: `If true use the AWS S3 accelerated endpoint.

See: [AWS S3 Transfer acceleration](https://docs.aws.amazon.com/AmazonS3/latest/dev/transfer-acceleration-examples.html)`,
			Default:  false,
			Advanced: true,
		}, {
			Name:     "leave_parts_on_error",
			Provider: "AWS",
			Help: `If true avoid calling abort upload on a failure, leaving all successfully uploaded parts on S3 for manual recovery.

It should be set to true for resuming uploads across different sessions.

WARNING: Storing parts of an incomplete multipart upload counts towards space usage on S3 and will add additional costs if not cleaned up.
`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "list_chunk",
			Help: `Size of listing chunk (response list for each ListObject S3 request).

This option is also known as "MaxKeys", "max-items", or "page-size" from the AWS S3 specification.
Most services truncate the response list to 1000 objects even if requested more than that.
In AWS S3 this is a global maximum and cannot be changed, see [AWS S3](https://docs.aws.amazon.com/cli/latest/reference/s3/ls.html).
In Ceph, this can be increased with the "rgw list buckets max chunk" option.
`,
			Default:  1000,
			Advanced: true,
		}, {
			Name: "list_version",
			Help: `Version of ListObjects to use: 1,2 or 0 for auto.

When S3 originally launched it only provided the ListObjects call to
enumerate objects in a bucket.

However in May 2016 the ListObjectsV2 call was introduced. This is
much higher performance and should be used if at all possible.

If set to the default, 0, rclone will guess according to the provider
set which list objects method to call. If it guesses wrong, then it
may be set manually here.
`,
			Default:  0,
			Advanced: true,
		}, {
			Name: "list_url_encode",
			Help: `Whether to url encode listings: true/false/unset

Some providers support URL encoding listings and where this is
available this is more reliable when using control characters in file
names. If this is set to unset (the default) then rclone will choose
according to the provider setting what to apply, but you can override
rclone's choice here.
`,
			Default:  fs.Tristate{},
			Advanced: true,
		}, {
			Name: "no_check_bucket",
			Help: `If set, don't attempt to check the bucket exists or create it.

This can be useful when trying to minimise the number of transactions
rclone does if you know the bucket exists already.

It can also be needed if the user you are using does not have bucket
creation permissions. Before v1.52.0 this would have passed silently
due to a bug.
`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "no_head",
			Help: `If set, don't HEAD uploaded objects to check integrity.

This can be useful when trying to minimise the number of transactions
rclone does.

Setting it means that if rclone receives a 200 OK message after
uploading an object with PUT then it will assume that it got uploaded
properly.

In particular it will assume:

- the metadata, including modtime, storage class and content type was as uploaded
- the size was as uploaded

It reads the following items from the response for a single part PUT:

- the MD5SUM
- The uploaded date

For multipart uploads these items aren't read.

If an source object of unknown length is uploaded then rclone **will** do a
HEAD request.

Setting this flag increases the chance for undetected upload failures,
in particular an incorrect size, so it isn't recommended for normal
operation. In practice the chance of an undetected upload failure is
very small even with this flag.
`,
			Default:  false,
			Advanced: true,
		}, {
			Name:     "no_head_object",
			Help:     `If set, do not do HEAD before GET when getting objects.`,
			Default:  false,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// Any UTF-8 character is valid in a key, however it can't handle
			// invalid UTF-8 and / have a special meaning.
			//
			// The SDK can't seem to handle uploading files called '.'
			//
			// FIXME would be nice to add
			// - initial / encoding
			// - doubled / encoding
			// - trailing / encoding
			// so that AWS keys are always valid file names
			Default: encoder.EncodeInvalidUtf8 |
				encoder.EncodeSlash |
				encoder.EncodeDot,
		}, {
			Name:     "memory_pool_flush_time",
			Default:  memoryPoolFlushTime,
			Advanced: true,
			Help: `How often internal memory buffer pools will be flushed.

Uploads which requires additional buffers (f.e multipart) will use memory pool for allocations.
This option controls how often unused buffers will be removed from the pool.`,
		}, {
			Name:     "memory_pool_use_mmap",
			Default:  memoryPoolUseMmap,
			Advanced: true,
			Help:     `Whether to use mmap buffers in internal memory pool.`,
		}, {
			Name:     "disable_http2",
			Default:  false,
			Advanced: true,
			Help: `Disable usage of http2 for S3 backends.

There is currently an unsolved issue with the s3 (specifically minio) backend
and HTTP/2.  HTTP/2 is enabled by default for the s3 backend but can be
disabled here.  When the issue is solved this flag will be removed.

See: https://github.com/rclone/rclone/issues/4673, https://github.com/rclone/rclone/issues/3631

`,
		}, {
			Name: "download_url",
			Help: `Custom endpoint for downloads.
This is usually set to a CloudFront CDN URL as AWS S3 offers
cheaper egress for data downloaded through the CloudFront network.`,
			Advanced: true,
		}, {
			Name: "use_multipart_etag",
			Help: `Whether to use ETag in multipart uploads for verification

This should be true, false or left unset to use the default for the provider.
`,
			Default:  fs.Tristate{},
			Advanced: true,
		}, {
			Name: "use_presigned_request",
			Help: `Whether to use a presigned request or PutObject for single part uploads

If this is false rclone will use PutObject from the AWS SDK to upload
an object.

Versions of rclone < 1.59 use presigned requests to upload a single
part object and setting this flag to true will re-enable that
functionality. This shouldn't be necessary except in exceptional
circumstances or for testing.
`,
			Default:  false,
			Advanced: true,
		},
		}})
}

// Constants
const (
	metaMtime   = "Mtime"     // the meta key to store mtime in - e.g. X-Amz-Meta-Mtime
	metaMD5Hash = "Md5chksum" // the meta key to store md5hash in
	// The maximum size of object we can COPY - this should be 5 GiB but is < 5 GB for b2 compatibility
	// See https://forum.rclone.org/t/copying-files-within-a-b2-bucket/16680/76
	maxSizeForCopy      = 4768 * 1024 * 1024
	maxUploadParts      = 10000 // maximum allowed number of parts in a multi-part upload
	minChunkSize        = fs.SizeSuffix(1024 * 1024 * 5)
	defaultUploadCutoff = fs.SizeSuffix(200 * 1024 * 1024)
	maxUploadCutoff     = fs.SizeSuffix(5 * 1024 * 1024 * 1024)
	minSleep            = 10 * time.Millisecond // In case of error, start at 10ms sleep.

	memoryPoolFlushTime = fs.Duration(time.Minute) // flush the cached buffers after this long
	memoryPoolUseMmap   = false
	maxExpireDuration   = fs.Duration(7 * 24 * time.Hour) // max expiry is 1 week
)

// Options defines the configuration for this backend
type Options struct {
	Provider              string               `config:"provider"`
	EnvAuth               bool                 `config:"env_auth"`
	AccessKeyID           string               `config:"access_key_id"`
	SecretAccessKey       string               `config:"secret_access_key"`
	Region                string               `config:"region"`
	Endpoint              string               `config:"endpoint"`
	LocationConstraint    string               `config:"location_constraint"`
	ACL                   string               `config:"acl"`
	BucketACL             string               `config:"bucket_acl"`
	RequesterPays         bool                 `config:"requester_pays"`
	ServerSideEncryption  string               `config:"server_side_encryption"`
	SSEKMSKeyID           string               `config:"sse_kms_key_id"`
	SSECustomerAlgorithm  string               `config:"sse_customer_algorithm"`
	SSECustomerKey        string               `config:"sse_customer_key"`
	SSECustomerKeyMD5     string               `config:"sse_customer_key_md5"`
	StorageClass          string               `config:"storage_class"`
	UploadCutoff          fs.SizeSuffix        `config:"upload_cutoff"`
	CopyCutoff            fs.SizeSuffix        `config:"copy_cutoff"`
	ChunkSize             fs.SizeSuffix        `config:"chunk_size"`
	MaxUploadParts        int64                `config:"max_upload_parts"`
	DisableChecksum       bool                 `config:"disable_checksum"`
	SharedCredentialsFile string               `config:"shared_credentials_file"`
	Profile               string               `config:"profile"`
	SessionToken          string               `config:"session_token"`
	UploadConcurrency     int                  `config:"upload_concurrency"`
	ForcePathStyle        bool                 `config:"force_path_style"`
	V2Auth                bool                 `config:"v2_auth"`
	UseAccelerateEndpoint bool                 `config:"use_accelerate_endpoint"`
	LeavePartsOnError     bool                 `config:"leave_parts_on_error"`
	ListChunk             int64                `config:"list_chunk"`
	ListVersion           int                  `config:"list_version"`
	ListURLEncode         fs.Tristate          `config:"list_url_encode"`
	NoCheckBucket         bool                 `config:"no_check_bucket"`
	NoHead                bool                 `config:"no_head"`
	NoHeadObject          bool                 `config:"no_head_object"`
	Enc                   encoder.MultiEncoder `config:"encoding"`
	MemoryPoolFlushTime   fs.Duration          `config:"memory_pool_flush_time"`
	MemoryPoolUseMmap     bool                 `config:"memory_pool_use_mmap"`
	DisableHTTP2          bool                 `config:"disable_http2"`
	DownloadURL           string               `config:"download_url"`
	UseMultipartEtag      fs.Tristate          `config:"use_multipart_etag"`
	UsePresignedRequest   bool                 `config:"use_presigned_request"`
}

// Fs represents a remote s3 server
type Fs struct {
	name          string           // the name of the remote
	root          string           // root of the bucket - ignore all objects above this
	opt           Options          // parsed options
	ci            *fs.ConfigInfo   // global config
	ctx           context.Context  // global context for reading config
	features      *fs.Features     // optional features
	c             *s3.S3           // the connection to the s3 server
	cu            *s3.S3           // unsigned connection to the s3 server for PutObject
	ses           *session.Session // the s3 session
	rootBucket    string           // bucket part of root (if any)
	rootDirectory string           // directory part of root (if any)
	cache         *bucket.Cache    // cache for bucket creation status
	pacer         *fs.Pacer        // To pace the API calls
	srv           *http.Client     // a plain http client
	srvRest       *rest.Client     // the rest connection to the server
	pool          *pool.Pool       // memory pool
	etagIsNotMD5  bool             // if set ETags are not MD5s
}

// Object describes a s3 object
type Object struct {
	// Will definitely have everything but meta which may be nil
	//
	// List will read everything but meta & mimeType - to fill
	// that in you need to call readMetaData
	fs           *Fs                // what this object is part of
	remote       string             // The remote path
	md5          string             // md5sum of the object
	bytes        int64              // size of the object
	lastModified time.Time          // Last modified
	meta         map[string]*string // The object metadata if known - may be nil
	mimeType     string             // MimeType of object - may be ""
	storageClass string             // e.g. GLACIER
}

// ------------------------------------------------------------

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	if f.rootBucket == "" {
		return "S3 root"
	}
	if f.rootDirectory == "" {
		return fmt.Sprintf("S3 bucket %s", f.rootBucket)
	}
	return fmt.Sprintf("S3 bucket %s path %s", f.rootBucket, f.rootDirectory)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// retryErrorCodes is a slice of error codes that we will retry
// See: https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html
var retryErrorCodes = []int{
	429, // Too Many Requests
	500, // Internal Server Error - "We encountered an internal error. Please try again."
	503, // Service Unavailable/Slow Down - "Reduce your request rate"
}

//S3 is pretty resilient, and the built in retry handling is probably sufficient
// as it should notice closed connections and timeouts which are the most likely
// sort of failure modes
func (f *Fs) shouldRetry(ctx context.Context, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	// If this is an awserr object, try and extract more useful information to determine if we should retry
	if awsError, ok := err.(awserr.Error); ok {
		// Simple case, check the original embedded error in case it's generically retryable
		if fserrors.ShouldRetry(awsError.OrigErr()) {
			return true, err
		}
		// Failing that, if it's a RequestFailure it's probably got an http status code we can check
		if reqErr, ok := err.(awserr.RequestFailure); ok {
			// 301 if wrong region for bucket - can only update if running from a bucket
			if f.rootBucket != "" {
				if reqErr.StatusCode() == http.StatusMovedPermanently {
					urfbErr := f.updateRegionForBucket(ctx, f.rootBucket)
					if urfbErr != nil {
						fs.Errorf(f, "Failed to update region for bucket: %v", urfbErr)
						return false, err
					}
					return true, err
				}
			}
			for _, e := range retryErrorCodes {
				if reqErr.StatusCode() == e {
					return true, err
				}
			}
		}
	}
	// Ok, not an awserr, check for generic failure conditions
	return fserrors.ShouldRetry(err), err
}

// parsePath parses a remote 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// split returns bucket and bucketPath from the rootRelativePath
// relative to f.root
func (f *Fs) split(rootRelativePath string) (bucketName, bucketPath string) {
	bucketName, bucketPath = bucket.Split(path.Join(f.root, rootRelativePath))
	return f.opt.Enc.FromStandardName(bucketName), f.opt.Enc.FromStandardPath(bucketPath)
}

// split returns bucket and bucketPath from the object
func (o *Object) split() (bucket, bucketPath string) {
	return o.fs.split(o.remote)
}

// getClient makes an http client according to the options
func getClient(ctx context.Context, opt *Options) *http.Client {
	// TODO: Do we need cookies too?
	t := fshttp.NewTransportCustom(ctx, func(t *http.Transport) {
		if opt.DisableHTTP2 {
			t.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
		}
	})
	return &http.Client{
		Transport: t,
	}
}

// s3Connection makes a connection to s3
//
// If unsignedBody is set then the connection is configured for
// unsigned bodies which is necessary for PutObject if we don't want
// it to seek
func s3Connection(ctx context.Context, opt *Options, client *http.Client) (*s3.S3, *s3.S3, *session.Session, error) {
	ci := fs.GetConfig(ctx)
	// Make the auth
	v := credentials.Value{
		AccessKeyID:     opt.AccessKeyID,
		SecretAccessKey: opt.SecretAccessKey,
		SessionToken:    opt.SessionToken,
	}

	lowTimeoutClient := &http.Client{Timeout: 1 * time.Second} // low timeout to ec2 metadata service

	def := defaults.Get()
	def.Config.HTTPClient = lowTimeoutClient

	// start a new AWS session
	awsSession, err := session.NewSession()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("NewSession: %w", err)
	}

	// first provider to supply a credential set "wins"
	providers := []credentials.Provider{
		// use static credentials if they're present (checked by provider)
		&credentials.StaticProvider{Value: v},

		// * Access Key ID:     AWS_ACCESS_KEY_ID or AWS_ACCESS_KEY
		// * Secret Access Key: AWS_SECRET_ACCESS_KEY or AWS_SECRET_KEY
		&credentials.EnvProvider{},

		// A SharedCredentialsProvider retrieves credentials
		// from the current user's home directory.  It checks
		// AWS_SHARED_CREDENTIALS_FILE and AWS_PROFILE too.
		&credentials.SharedCredentialsProvider{
			Filename: opt.SharedCredentialsFile, // If empty will look for "AWS_SHARED_CREDENTIALS_FILE" env variable.
			Profile:  opt.Profile,               // If empty will look gor "AWS_PROFILE" env var or "default" if not set.
		},

		// Pick up IAM role if we're in an ECS task
		defaults.RemoteCredProvider(*def.Config, def.Handlers),

		// Pick up IAM role in case we're on EC2
		&ec2rolecreds.EC2RoleProvider{
			Client: ec2metadata.New(awsSession, &aws.Config{
				HTTPClient: lowTimeoutClient,
			}),
			ExpiryWindow: 3 * time.Minute,
		},
	}
	cred := credentials.NewChainCredentials(providers)

	switch {
	case opt.EnvAuth:
		// No need for empty checks if "env_auth" is true
	case v.AccessKeyID == "" && v.SecretAccessKey == "":
		// if no access key/secret and iam is explicitly disabled then fall back to anon interaction
		cred = credentials.AnonymousCredentials
	case v.AccessKeyID == "":
		return nil, nil, nil, errors.New("access_key_id not found")
	case v.SecretAccessKey == "":
		return nil, nil, nil, errors.New("secret_access_key not found")
	}

	if opt.Region == "" {
		opt.Region = "us-east-1"
	}
	setQuirks(opt)
	awsConfig := aws.NewConfig().
		WithMaxRetries(ci.LowLevelRetries).
		WithCredentials(cred).
		WithHTTPClient(client).
		WithS3ForcePathStyle(opt.ForcePathStyle).
		WithS3UseAccelerate(opt.UseAccelerateEndpoint).
		WithS3UsEast1RegionalEndpoint(endpoints.RegionalS3UsEast1Endpoint)

	if opt.Region != "" {
		awsConfig.WithRegion(opt.Region)
	}
	if opt.Endpoint != "" {
		awsConfig.WithEndpoint(opt.Endpoint)
	}

	// awsConfig.WithLogLevel(aws.LogDebugWithSigning)
	awsSessionOpts := session.Options{
		Config: *awsConfig,
	}
	if opt.EnvAuth && opt.AccessKeyID == "" && opt.SecretAccessKey == "" {
		// Enable loading config options from ~/.aws/config (selected by AWS_PROFILE env)
		awsSessionOpts.SharedConfigState = session.SharedConfigEnable
		// Set the name of the profile if supplied
		awsSessionOpts.Profile = opt.Profile
		// Set the shared config file if supplied
		if opt.SharedCredentialsFile != "" {
			awsSessionOpts.SharedConfigFiles = []string{opt.SharedCredentialsFile}
		}
		// The session constructor (aws/session/mergeConfigSrcs) will only use the user's preferred credential source
		// (from the shared config file) if the passed-in Options.Config.Credentials is nil.
		awsSessionOpts.Config.Credentials = nil
	}
	// Setting this stops PutObject reading the body twice and seeking
	// We add our own Content-MD5 for data protection
	awsSessionOpts.Config.S3DisableContentMD5Validation = aws.Bool(true)
	ses, err := session.NewSessionWithOptions(awsSessionOpts)
	if err != nil {
		return nil, nil, nil, err
	}
	newC := func(unsignedBody bool) *s3.S3 {
		c := s3.New(ses)
		if opt.V2Auth || opt.Region == "other-v2-signature" {
			fs.Debugf(nil, "Using v2 auth")
			signer := func(req *request.Request) {
				// Ignore AnonymousCredentials object
				if req.Config.Credentials == credentials.AnonymousCredentials {
					return
				}
				sign(v.AccessKeyID, v.SecretAccessKey, req.HTTPRequest)
			}
			c.Handlers.Sign.Clear()
			c.Handlers.Sign.PushBackNamed(corehandlers.BuildContentLengthHandler)
			c.Handlers.Sign.PushBack(signer)
		} else if unsignedBody {
			// If the body is unsigned then tell the signer that we aren't signing the payload
			c.Handlers.Sign.Clear()
			c.Handlers.Sign.PushBackNamed(corehandlers.BuildContentLengthHandler)
			c.Handlers.Sign.PushBackNamed(v4.BuildNamedHandler("v4.SignRequestHandler.WithUnsignedPayload", v4.WithUnsignedPayload))
		}
		return c
	}
	return newC(false), newC(true), ses, nil
}

func checkUploadChunkSize(cs fs.SizeSuffix) error {
	if cs < minChunkSize {
		return fmt.Errorf("%s is less than %s", cs, minChunkSize)
	}
	return nil
}

func (f *Fs) setUploadChunkSize(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	err = checkUploadChunkSize(cs)
	if err == nil {
		old, f.opt.ChunkSize = f.opt.ChunkSize, cs
	}
	return
}

func checkUploadCutoff(cs fs.SizeSuffix) error {
	if cs > maxUploadCutoff {
		return fmt.Errorf("%s is greater than %s", cs, maxUploadCutoff)
	}
	return nil
}

func (f *Fs) setUploadCutoff(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	err = checkUploadCutoff(cs)
	if err == nil {
		old, f.opt.UploadCutoff = f.opt.UploadCutoff, cs
	}
	return
}

// Set the provider quirks
//
// There should be no testing against opt.Provider anywhere in the
// code except in here to localise the setting of the quirks.
//
// These should be differences from AWS S3
func setQuirks(opt *Options) {
	var (
		listObjectsV2     = true
		virtualHostStyle  = true
		urlEncodeListings = true
		useMultipartEtag  = true
	)
	switch opt.Provider {
	case "AWS":
		// No quirks
	case "Alibaba":
		useMultipartEtag = false // Alibaba seems to calculate multipart Etags differently from AWS
	case "HuaweiOBS":
		// Huawei OBS PFS is not support listObjectV2, and if turn on the urlEncodeListing, marker will not work and keep list same page forever.
		urlEncodeListings = false
		listObjectsV2 = false
	case "Ceph":
		listObjectsV2 = false
		virtualHostStyle = false
		urlEncodeListings = false
	case "ChinaMobile":
		listObjectsV2 = false
		virtualHostStyle = false
		urlEncodeListings = false
	case "Cloudflare":
		virtualHostStyle = false
		useMultipartEtag = false // currently multipart Etags are random
	case "ArvanCloud":
		listObjectsV2 = false
		virtualHostStyle = false
		urlEncodeListings = false
	case "DigitalOcean":
		urlEncodeListings = false
	case "Dreamhost":
		urlEncodeListings = false
	case "IBMCOS":
		listObjectsV2 = false // untested
		virtualHostStyle = false
		urlEncodeListings = false
		useMultipartEtag = false // untested
	case "LyveCloud":
		useMultipartEtag = false // LyveCloud seems to calculate multipart Etags differently from AWS
	case "Minio":
		virtualHostStyle = false
	case "Netease":
		listObjectsV2 = false // untested
		urlEncodeListings = false
		useMultipartEtag = false // untested
	case "RackCorp":
		// No quirks
		useMultipartEtag = false // untested
	case "Scaleway":
		// Scaleway can only have 1000 parts in an upload
		if opt.MaxUploadParts > 1000 {
			opt.MaxUploadParts = 1000
		}
		urlEncodeListings = false
	case "SeaweedFS":
		listObjectsV2 = false // untested
		virtualHostStyle = false
		urlEncodeListings = false
		useMultipartEtag = false // untested
	case "StackPath":
		listObjectsV2 = false // untested
		virtualHostStyle = false
		urlEncodeListings = false
	case "Storj":
		// Force chunk size to >= 64 MiB
		if opt.ChunkSize < 64*fs.Mebi {
			opt.ChunkSize = 64 * fs.Mebi
		}
	case "TencentCOS":
		listObjectsV2 = false    // untested
		useMultipartEtag = false // untested
	case "Wasabi":
		// No quirks
	case "Other":
		listObjectsV2 = false
		virtualHostStyle = false
		urlEncodeListings = false
		useMultipartEtag = false
	default:
		fs.Logf("s3", "s3 provider %q not known - please set correctly", opt.Provider)
		listObjectsV2 = false
		virtualHostStyle = false
		urlEncodeListings = false
		useMultipartEtag = false
	}

	// Path Style vs Virtual Host style
	if virtualHostStyle || opt.UseAccelerateEndpoint {
		opt.ForcePathStyle = false
	}

	// Set to see if we need to URL encode listings
	if !opt.ListURLEncode.Valid {
		opt.ListURLEncode.Valid = true
		opt.ListURLEncode.Value = urlEncodeListings
	}

	// Set the correct list version if not manually set
	if opt.ListVersion == 0 {
		if listObjectsV2 {
			opt.ListVersion = 2
		} else {
			opt.ListVersion = 1
		}
	}

	// Set the correct use multipart Etag for error checking if not manually set
	if !opt.UseMultipartEtag.Valid {
		opt.UseMultipartEtag.Valid = true
		opt.UseMultipartEtag.Value = useMultipartEtag
	}
}

// setRoot changes the root of the Fs
func (f *Fs) setRoot(root string) {
	f.root = parsePath(root)
	f.rootBucket, f.rootDirectory = bucket.Split(f.root)
}

// NewFs constructs an Fs from the path, bucket:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	err = checkUploadChunkSize(opt.ChunkSize)
	if err != nil {
		return nil, fmt.Errorf("s3: chunk size: %w", err)
	}
	err = checkUploadCutoff(opt.UploadCutoff)
	if err != nil {
		return nil, fmt.Errorf("s3: upload cutoff: %w", err)
	}
	if opt.ACL == "" {
		opt.ACL = "private"
	}
	if opt.BucketACL == "" {
		opt.BucketACL = opt.ACL
	}
	if opt.SSECustomerKey != "" && opt.SSECustomerKeyMD5 == "" {
		// calculate CustomerKeyMD5 if not supplied
		md5sumBinary := md5.Sum([]byte(opt.SSECustomerKey))
		opt.SSECustomerKeyMD5 = base64.StdEncoding.EncodeToString(md5sumBinary[:])
	}
	srv := getClient(ctx, opt)
	c, cu, ses, err := s3Connection(ctx, opt, srv)
	if err != nil {
		return nil, err
	}

	ci := fs.GetConfig(ctx)
	pc := fs.NewPacer(ctx, pacer.NewS3(pacer.MinSleep(minSleep)))
	// Set pacer retries to 2 (1 try and 1 retry) because we are
	// relying on SDK retry mechanism, but we allow 2 attempts to
	// retry directory listings after XMLSyntaxError
	pc.SetRetries(2)

	f := &Fs{
		name:    name,
		opt:     *opt,
		ci:      ci,
		ctx:     ctx,
		c:       c,
		cu:      cu,
		ses:     ses,
		pacer:   pc,
		cache:   bucket.NewCache(),
		srv:     srv,
		srvRest: rest.NewClient(fshttp.NewClient(ctx)),
		pool: pool.New(
			time.Duration(opt.MemoryPoolFlushTime),
			int(opt.ChunkSize),
			opt.UploadConcurrency*ci.Transfers,
			opt.MemoryPoolUseMmap,
		),
	}
	if opt.ServerSideEncryption == "aws:kms" || opt.SSECustomerAlgorithm != "" {
		// From: https://docs.aws.amazon.com/AmazonS3/latest/API/RESTCommonResponseHeaders.html
		//
		// Objects encrypted by SSE-S3 or plaintext have ETags that are an MD5
		// digest of their data.
		//
		// Objects encrypted by SSE-C or SSE-KMS have ETags that are not an
		// MD5 digest of their object data.
		f.etagIsNotMD5 = true
	}
	f.setRoot(root)
	f.features = (&fs.Features{
		ReadMimeType:      true,
		WriteMimeType:     true,
		BucketBased:       true,
		BucketBasedRootOK: true,
		SetTier:           true,
		GetTier:           true,
		SlowModTime:       true,
	}).Fill(ctx, f)
	if f.rootBucket != "" && f.rootDirectory != "" && !opt.NoHeadObject && !strings.HasSuffix(root, "/") {
		// Check to see if the (bucket,directory) is actually an existing file
		oldRoot := f.root
		newRoot, leaf := path.Split(oldRoot)
		f.setRoot(newRoot)
		_, err := f.NewObject(ctx, leaf)
		if err != nil {
			// File doesn't exist or is a directory so return old f
			f.setRoot(oldRoot)
			return f, nil
		}
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	if opt.Provider == "Storj" {
		f.features.Copy = nil
		f.features.SetTier = false
		f.features.GetTier = false
	}
	// f.listMultipartUploads()
	return f, nil
}

// Return an Object from a path
//
//If it can't be found it returns the error ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *s3.Object) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		// Set info but not meta
		if info.LastModified == nil {
			fs.Logf(o, "Failed to read last modified")
			o.lastModified = time.Now()
		} else {
			o.lastModified = *info.LastModified
		}
		o.setMD5FromEtag(aws.StringValue(info.ETag))
		o.bytes = aws.Int64Value(info.Size)
		o.storageClass = aws.StringValue(info.StorageClass)
	} else if !o.fs.opt.NoHeadObject {
		err := o.readMetaData(ctx) // reads info and meta, returning an error
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

// Gets the bucket location
func (f *Fs) getBucketLocation(ctx context.Context, bucket string) (string, error) {
	req := s3.GetBucketLocationInput{
		Bucket: &bucket,
	}
	var resp *s3.GetBucketLocationOutput
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.c.GetBucketLocation(&req)
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return "", err
	}
	return s3.NormalizeBucketLocation(aws.StringValue(resp.LocationConstraint)), nil
}

// Updates the region for the bucket by reading the region from the
// bucket then updating the session.
func (f *Fs) updateRegionForBucket(ctx context.Context, bucket string) error {
	region, err := f.getBucketLocation(ctx, bucket)
	if err != nil {
		return fmt.Errorf("reading bucket location failed: %w", err)
	}
	if aws.StringValue(f.c.Config.Endpoint) != "" {
		return fmt.Errorf("can't set region to %q as endpoint is set", region)
	}
	if aws.StringValue(f.c.Config.Region) == region {
		return fmt.Errorf("region is already %q - not updating", region)
	}

	// Make a new session with the new region
	oldRegion := f.opt.Region
	f.opt.Region = region
	c, cu, ses, err := s3Connection(f.ctx, &f.opt, f.srv)
	if err != nil {
		return fmt.Errorf("creating new session failed: %w", err)
	}
	f.c = c
	f.cu = cu
	f.ses = ses

	fs.Logf(f, "Switched region to %q from %q", region, oldRegion)
	return nil
}

// listFn is called from list to handle an object.
type listFn func(remote string, object *s3.Object, isDirectory bool) error

// list lists the objects into the function supplied from
// the bucket and directory supplied.  The remote has prefix
// removed from it and if addBucket is set then it adds the
// bucket to the start.
//
// Set recurse to read sub directories
func (f *Fs) list(ctx context.Context, bucket, directory, prefix string, addBucket bool, recurse bool, fn listFn) error {
	v1 := f.opt.ListVersion == 1
	if prefix != "" {
		prefix += "/"
	}
	if directory != "" {
		directory += "/"
	}
	delimiter := ""
	if !recurse {
		delimiter = "/"
	}
	var continuationToken, startAfter *string
	// URL encode the listings so we can use control characters in object names
	// See: https://github.com/aws/aws-sdk-go/issues/1914
	//
	// However this doesn't work perfectly under Ceph (and hence DigitalOcean/Dreamhost) because
	// it doesn't encode CommonPrefixes.
	// See: https://tracker.ceph.com/issues/41870
	//
	// This does not work under IBM COS also: See https://github.com/rclone/rclone/issues/3345
	// though maybe it does on some versions.
	//
	// This does work with minio but was only added relatively recently
	// https://github.com/minio/minio/pull/7265
	//
	// So we enable only on providers we know supports it properly, all others can retry when a
	// XML Syntax error is detected.
	urlEncodeListings := f.opt.ListURLEncode.Value
	for {
		// FIXME need to implement ALL loop
		req := s3.ListObjectsV2Input{
			Bucket:            &bucket,
			ContinuationToken: continuationToken,
			Delimiter:         &delimiter,
			Prefix:            &directory,
			MaxKeys:           &f.opt.ListChunk,
			StartAfter:        startAfter,
		}
		if urlEncodeListings {
			req.EncodingType = aws.String(s3.EncodingTypeUrl)
		}
		if f.opt.RequesterPays {
			req.RequestPayer = aws.String(s3.RequestPayerRequester)
		}
		var resp *s3.ListObjectsV2Output
		var err error
		err = f.pacer.Call(func() (bool, error) {
			if v1 {
				// Convert v2 req into v1 req
				var reqv1 s3.ListObjectsInput
				structs.SetFrom(&reqv1, &req)
				reqv1.Marker = continuationToken
				if startAfter != nil {
					reqv1.Marker = startAfter
				}
				var respv1 *s3.ListObjectsOutput
				respv1, err = f.c.ListObjectsWithContext(ctx, &reqv1)
				if err == nil && respv1 != nil {
					// convert v1 resp into v2 resp
					resp = new(s3.ListObjectsV2Output)
					structs.SetFrom(resp, respv1)
					resp.NextContinuationToken = respv1.NextMarker
				}
			} else {
				resp, err = f.c.ListObjectsV2WithContext(ctx, &req)
			}
			if err != nil && !urlEncodeListings {
				if awsErr, ok := err.(awserr.RequestFailure); ok {
					if origErr := awsErr.OrigErr(); origErr != nil {
						if _, ok := origErr.(*xml.SyntaxError); ok {
							// Retry the listing with URL encoding as there were characters that XML can't encode
							urlEncodeListings = true
							req.EncodingType = aws.String(s3.EncodingTypeUrl)
							fs.Debugf(f, "Retrying listing because of characters which can't be XML encoded")
							return true, err
						}
					}
				}
			}
			return f.shouldRetry(ctx, err)
		})
		if err != nil {
			if awsErr, ok := err.(awserr.RequestFailure); ok {
				if awsErr.StatusCode() == http.StatusNotFound {
					err = fs.ErrorDirNotFound
				}
			}
			if f.rootBucket == "" {
				// if listing from the root ignore wrong region requests returning
				// empty directory
				if reqErr, ok := err.(awserr.RequestFailure); ok {
					// 301 if wrong region for bucket
					if reqErr.StatusCode() == http.StatusMovedPermanently {
						fs.Errorf(f, "Can't change region for bucket %q with no bucket specified", bucket)
						return nil
					}
				}
			}
			return err
		}
		if !recurse {
			for _, commonPrefix := range resp.CommonPrefixes {
				if commonPrefix.Prefix == nil {
					fs.Logf(f, "Nil common prefix received")
					continue
				}
				remote := *commonPrefix.Prefix
				if urlEncodeListings {
					remote, err = url.QueryUnescape(remote)
					if err != nil {
						fs.Logf(f, "failed to URL decode %q in listing common prefix: %v", *commonPrefix.Prefix, err)
						continue
					}
				}
				remote = f.opt.Enc.ToStandardPath(remote)
				if !strings.HasPrefix(remote, prefix) {
					fs.Logf(f, "Odd name received %q", remote)
					continue
				}
				remote = remote[len(prefix):]
				if addBucket {
					remote = path.Join(bucket, remote)
				}
				remote = strings.TrimSuffix(remote, "/")
				err = fn(remote, &s3.Object{Key: &remote}, true)
				if err != nil {
					return err
				}
			}
		}
		for _, object := range resp.Contents {
			remote := aws.StringValue(object.Key)
			if urlEncodeListings {
				remote, err = url.QueryUnescape(remote)
				if err != nil {
					fs.Logf(f, "failed to URL decode %q in listing: %v", aws.StringValue(object.Key), err)
					continue
				}
			}
			remote = f.opt.Enc.ToStandardPath(remote)
			if !strings.HasPrefix(remote, prefix) {
				fs.Logf(f, "Odd name received %q", remote)
				continue
			}
			remote = remote[len(prefix):]
			isDirectory := remote == "" || strings.HasSuffix(remote, "/")
			if addBucket {
				remote = path.Join(bucket, remote)
			}
			// is this a directory marker?
			if isDirectory && object.Size != nil && *object.Size == 0 {
				continue // skip directory marker
			}
			err = fn(remote, object, false)
			if err != nil {
				return err
			}
		}
		if !aws.BoolValue(resp.IsTruncated) {
			break
		}
		// Use NextContinuationToken if set, otherwise use last Key for StartAfter
		if resp.NextContinuationToken == nil || *resp.NextContinuationToken == "" {
			if len(resp.Contents) == 0 {
				return errors.New("s3 protocol error: received listing with IsTruncated set, no NextContinuationToken/NextMarker and no Contents")
			}
			continuationToken = nil
			startAfter = resp.Contents[len(resp.Contents)-1].Key
		} else {
			continuationToken = resp.NextContinuationToken
			startAfter = nil
		}
		if startAfter != nil && urlEncodeListings {
			*startAfter, err = url.QueryUnescape(*startAfter)
			if err != nil {
				return fmt.Errorf("failed to URL decode StartAfter/NextMarker %q: %w", *continuationToken, err)
			}
		}
	}
	return nil
}

// Convert a list item into a DirEntry
func (f *Fs) itemToDirEntry(ctx context.Context, remote string, object *s3.Object, isDirectory bool) (fs.DirEntry, error) {
	if isDirectory {
		size := int64(0)
		if object.Size != nil {
			size = *object.Size
		}
		d := fs.NewDir(remote, time.Time{}).SetSize(size)
		return d, nil
	}
	o, err := f.newObjectWithInfo(ctx, remote, object)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// listDir lists files and directories to out
func (f *Fs) listDir(ctx context.Context, bucket, directory, prefix string, addBucket bool) (entries fs.DirEntries, err error) {
	// List the objects and directories
	err = f.list(ctx, bucket, directory, prefix, addBucket, false, func(remote string, object *s3.Object, isDirectory bool) error {
		entry, err := f.itemToDirEntry(ctx, remote, object, isDirectory)
		if err != nil {
			return err
		}
		if entry != nil {
			entries = append(entries, entry)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// bucket must be present if listing succeeded
	f.cache.MarkOK(bucket)
	return entries, nil
}

// listBuckets lists the buckets to out
func (f *Fs) listBuckets(ctx context.Context) (entries fs.DirEntries, err error) {
	req := s3.ListBucketsInput{}
	var resp *s3.ListBucketsOutput
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.c.ListBucketsWithContext(ctx, &req)
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}
	for _, bucket := range resp.Buckets {
		bucketName := f.opt.Enc.ToStandardName(aws.StringValue(bucket.Name))
		f.cache.MarkOK(bucketName)
		d := fs.NewDir(bucketName, aws.TimeValue(bucket.CreationDate))
		entries = append(entries, d)
	}
	return entries, nil
}

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	bucket, directory := f.split(dir)
	if bucket == "" {
		if directory != "" {
			return nil, fs.ErrorListBucketRequired
		}
		return f.listBuckets(ctx)
	}
	return f.listDir(ctx, bucket, directory, f.rootDirectory, f.rootBucket == "")
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
//
// It should call callback for each tranche of entries read.
// These need not be returned in any particular order.  If
// callback returns an error then the listing will stop
// immediately.
//
// Don't implement this unless you have a more efficient way
// of listing recursively than doing a directory traversal.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	bucket, directory := f.split(dir)
	list := walk.NewListRHelper(callback)
	listR := func(bucket, directory, prefix string, addBucket bool) error {
		return f.list(ctx, bucket, directory, prefix, addBucket, true, func(remote string, object *s3.Object, isDirectory bool) error {
			entry, err := f.itemToDirEntry(ctx, remote, object, isDirectory)
			if err != nil {
				return err
			}
			return list.Add(entry)
		})
	}
	if bucket == "" {
		entries, err := f.listBuckets(ctx)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			err = list.Add(entry)
			if err != nil {
				return err
			}
			bucket := entry.Remote()
			err = listR(bucket, "", f.rootDirectory, true)
			if err != nil {
				return err
			}
			// bucket must be present if listing succeeded
			f.cache.MarkOK(bucket)
		}
	} else {
		err = listR(bucket, directory, f.rootDirectory, f.rootBucket == "")
		if err != nil {
			return err
		}
		// bucket must be present if listing succeeded
		f.cache.MarkOK(bucket)
	}
	return list.Flush()
}

// Put the Object into the bucket
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Temporary Object under construction
	fs := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return fs, fs.Update(ctx, in, src, options...)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// Check if the bucket exists
//
// NB this can return incorrect results if called immediately after bucket deletion
func (f *Fs) bucketExists(ctx context.Context, bucket string) (bool, error) {
	req := s3.HeadBucketInput{
		Bucket: &bucket,
	}
	err := f.pacer.Call(func() (bool, error) {
		_, err := f.c.HeadBucketWithContext(ctx, &req)
		return f.shouldRetry(ctx, err)
	})
	if err == nil {
		return true, nil
	}
	if err, ok := err.(awserr.RequestFailure); ok {
		if err.StatusCode() == http.StatusNotFound {
			return false, nil
		}
	}
	return false, err
}

// Mkdir creates the bucket if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	bucket, _ := f.split(dir)
	return f.makeBucket(ctx, bucket)
}

// makeBucket creates the bucket if it doesn't exist
func (f *Fs) makeBucket(ctx context.Context, bucket string) error {
	if f.opt.NoCheckBucket {
		return nil
	}
	return f.cache.Create(bucket, func() error {
		req := s3.CreateBucketInput{
			Bucket: &bucket,
			ACL:    &f.opt.BucketACL,
		}
		if f.opt.LocationConstraint != "" {
			req.CreateBucketConfiguration = &s3.CreateBucketConfiguration{
				LocationConstraint: &f.opt.LocationConstraint,
			}
		}
		err := f.pacer.Call(func() (bool, error) {
			_, err := f.c.CreateBucketWithContext(ctx, &req)
			return f.shouldRetry(ctx, err)
		})
		if err == nil {
			fs.Infof(f, "Bucket %q created with ACL %q", bucket, f.opt.BucketACL)
		}
		if awsErr, ok := err.(awserr.Error); ok {
			if code := awsErr.Code(); code == "BucketAlreadyOwnedByYou" || code == "BucketAlreadyExists" {
				err = nil
			}
		}
		return err
	}, func() (bool, error) {
		return f.bucketExists(ctx, bucket)
	})
}

// Rmdir deletes the bucket if the fs is at the root
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	bucket, directory := f.split(dir)
	if bucket == "" || directory != "" {
		return nil
	}
	return f.cache.Remove(bucket, func() error {
		req := s3.DeleteBucketInput{
			Bucket: &bucket,
		}
		err := f.pacer.Call(func() (bool, error) {
			_, err := f.c.DeleteBucketWithContext(ctx, &req)
			return f.shouldRetry(ctx, err)
		})
		if err == nil {
			fs.Infof(f, "Bucket %q deleted", bucket)
		}
		return err
	})
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return time.Nanosecond
}

// pathEscape escapes s as for a URL path.  It uses rest.URLPathEscape
// but also escapes '+' for S3 and Digital Ocean spaces compatibility
func pathEscape(s string) string {
	return strings.ReplaceAll(rest.URLPathEscape(s), "+", "%2B")
}

// copy does a server-side copy
//
// It adds the boiler plate to the req passed in and calls the s3
// method
func (f *Fs) copy(ctx context.Context, req *s3.CopyObjectInput, dstBucket, dstPath, srcBucket, srcPath string, src *Object) error {
	req.Bucket = &dstBucket
	req.ACL = &f.opt.ACL
	req.Key = &dstPath
	source := pathEscape(path.Join(srcBucket, srcPath))
	req.CopySource = &source
	if f.opt.RequesterPays {
		req.RequestPayer = aws.String(s3.RequestPayerRequester)
	}
	if f.opt.ServerSideEncryption != "" {
		req.ServerSideEncryption = &f.opt.ServerSideEncryption
	}
	if f.opt.SSECustomerAlgorithm != "" {
		req.SSECustomerAlgorithm = &f.opt.SSECustomerAlgorithm
		req.CopySourceSSECustomerAlgorithm = &f.opt.SSECustomerAlgorithm
	}
	if f.opt.SSECustomerKey != "" {
		req.SSECustomerKey = &f.opt.SSECustomerKey
		req.CopySourceSSECustomerKey = &f.opt.SSECustomerKey
	}
	if f.opt.SSECustomerKeyMD5 != "" {
		req.SSECustomerKeyMD5 = &f.opt.SSECustomerKeyMD5
		req.CopySourceSSECustomerKeyMD5 = &f.opt.SSECustomerKeyMD5
	}
	if f.opt.SSEKMSKeyID != "" {
		req.SSEKMSKeyId = &f.opt.SSEKMSKeyID
	}
	if req.StorageClass == nil && f.opt.StorageClass != "" {
		req.StorageClass = &f.opt.StorageClass
	}

	if src.bytes >= int64(f.opt.CopyCutoff) {
		return f.copyMultipart(ctx, req, dstBucket, dstPath, srcBucket, srcPath, src)
	}
	return f.pacer.Call(func() (bool, error) {
		_, err := f.c.CopyObjectWithContext(ctx, req)
		return f.shouldRetry(ctx, err)
	})
}

func calculateRange(partSize, partIndex, numParts, totalSize int64) string {
	start := partIndex * partSize
	var ends string
	if partIndex == numParts-1 {
		if totalSize >= 1 {
			ends = strconv.FormatInt(totalSize-1, 10)
		}
	} else {
		ends = strconv.FormatInt(start+partSize-1, 10)
	}
	return fmt.Sprintf("bytes=%v-%v", start, ends)
}

func (f *Fs) copyMultipart(ctx context.Context, copyReq *s3.CopyObjectInput, dstBucket, dstPath, srcBucket, srcPath string, src *Object) (err error) {
	info, err := src.headObject(ctx)
	if err != nil {
		return err
	}

	req := &s3.CreateMultipartUploadInput{}

	// Fill in the request from the head info
	structs.SetFrom(req, info)

	// If copy metadata was set then set the Metadata to that read
	// from the head request
	if aws.StringValue(copyReq.MetadataDirective) == s3.MetadataDirectiveCopy {
		copyReq.Metadata = info.Metadata
	}

	// Overwrite any from the copyReq
	structs.SetFrom(req, copyReq)

	req.Bucket = &dstBucket
	req.Key = &dstPath

	var cout *s3.CreateMultipartUploadOutput
	if err := f.pacer.Call(func() (bool, error) {
		var err error
		cout, err = f.c.CreateMultipartUploadWithContext(ctx, req)
		return f.shouldRetry(ctx, err)
	}); err != nil {
		return err
	}
	uid := cout.UploadId

	defer atexit.OnError(&err, func() {
		// Try to abort the upload, but ignore the error.
		fs.Debugf(src, "Cancelling multipart copy")
		_ = f.pacer.Call(func() (bool, error) {
			_, err := f.c.AbortMultipartUploadWithContext(context.Background(), &s3.AbortMultipartUploadInput{
				Bucket:       &dstBucket,
				Key:          &dstPath,
				UploadId:     uid,
				RequestPayer: req.RequestPayer,
			})
			return f.shouldRetry(ctx, err)
		})
	})()

	srcSize := src.bytes
	partSize := int64(f.opt.CopyCutoff)
	numParts := (srcSize-1)/partSize + 1

	fs.Debugf(src, "Starting  multipart copy with %d parts", numParts)

	var parts []*s3.CompletedPart
	for partNum := int64(1); partNum <= numParts; partNum++ {
		if err := f.pacer.Call(func() (bool, error) {
			partNum := partNum
			uploadPartReq := &s3.UploadPartCopyInput{}
			structs.SetFrom(uploadPartReq, copyReq)
			uploadPartReq.Bucket = &dstBucket
			uploadPartReq.Key = &dstPath
			uploadPartReq.PartNumber = &partNum
			uploadPartReq.UploadId = uid
			uploadPartReq.CopySourceRange = aws.String(calculateRange(partSize, partNum-1, numParts, srcSize))
			uout, err := f.c.UploadPartCopyWithContext(ctx, uploadPartReq)
			if err != nil {
				return f.shouldRetry(ctx, err)
			}
			parts = append(parts, &s3.CompletedPart{
				PartNumber: &partNum,
				ETag:       uout.CopyPartResult.ETag,
			})
			return false, nil
		}); err != nil {
			return err
		}
	}

	return f.pacer.Call(func() (bool, error) {
		_, err := f.c.CompleteMultipartUploadWithContext(ctx, &s3.CompleteMultipartUploadInput{
			Bucket: &dstBucket,
			Key:    &dstPath,
			MultipartUpload: &s3.CompletedMultipartUpload{
				Parts: parts,
			},
			RequestPayer: req.RequestPayer,
			UploadId:     uid,
		})
		return f.shouldRetry(ctx, err)
	})
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	dstBucket, dstPath := f.split(remote)
	err := f.makeBucket(ctx, dstBucket)
	if err != nil {
		return nil, err
	}
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	srcBucket, srcPath := srcObj.split()
	req := s3.CopyObjectInput{
		MetadataDirective: aws.String(s3.MetadataDirectiveCopy),
	}
	err = f.copy(ctx, &req, dstBucket, dstPath, srcBucket, srcPath, srcObj)
	if err != nil {
		return nil, err
	}
	return f.NewObject(ctx, remote)
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

func (f *Fs) getMemoryPool(size int64) *pool.Pool {
	if size == int64(f.opt.ChunkSize) {
		return f.pool
	}

	return pool.New(
		time.Duration(f.opt.MemoryPoolFlushTime),
		int(size),
		f.opt.UploadConcurrency*f.ci.Transfers,
		f.opt.MemoryPoolUseMmap,
	)
}

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (link string, err error) {
	if strings.HasSuffix(remote, "/") {
		return "", fs.ErrorCantShareDirectories
	}
	if _, err := f.NewObject(ctx, remote); err != nil {
		return "", err
	}
	if expire > maxExpireDuration {
		fs.Logf(f, "Public Link: Reducing expiry to %v as %v is greater than the max time allowed", maxExpireDuration, expire)
		expire = maxExpireDuration
	}
	bucket, bucketPath := f.split(remote)
	httpReq, _ := f.c.GetObjectRequest(&s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &bucketPath,
	})

	return httpReq.Presign(time.Duration(expire))
}

var commandHelp = []fs.CommandHelp{{
	Name:  "restore",
	Short: "Restore objects from GLACIER to normal storage",
	Long: `This command can be used to restore one or more objects from GLACIER
to normal storage.

Usage Examples:

    rclone backend restore s3:bucket/path/to/object [-o priority=PRIORITY] [-o lifetime=DAYS]
    rclone backend restore s3:bucket/path/to/directory [-o priority=PRIORITY] [-o lifetime=DAYS]
    rclone backend restore s3:bucket [-o priority=PRIORITY] [-o lifetime=DAYS]

This flag also obeys the filters. Test first with -i/--interactive or --dry-run flags

    rclone -i backend restore --include "*.txt" s3:bucket/path -o priority=Standard

All the objects shown will be marked for restore, then

    rclone backend restore --include "*.txt" s3:bucket/path -o priority=Standard

It returns a list of status dictionaries with Remote and Status
keys. The Status will be OK if it was successful or an error message
if not.

    [
        {
            "Status": "OK",
            "Path": "test.txt"
        },
        {
            "Status": "OK",
            "Path": "test/file4.txt"
        }
    ]

`,
	Opts: map[string]string{
		"priority":    "Priority of restore: Standard|Expedited|Bulk",
		"lifetime":    "Lifetime of the active copy in days",
		"description": "The optional description for the job.",
	},
}, {
	Name:  "list-multipart-uploads",
	Short: "List the unfinished multipart uploads",
	Long: `This command lists the unfinished multipart uploads in JSON format.

    rclone backend list-multipart s3:bucket/path/to/object

It returns a dictionary of buckets with values as lists of unfinished
multipart uploads.

You can call it with no bucket in which case it lists all bucket, with
a bucket or with a bucket and path.

    {
      "rclone": [
        {
          "Initiated": "2020-06-26T14:20:36Z",
          "Initiator": {
            "DisplayName": "XXX",
            "ID": "arn:aws:iam::XXX:user/XXX"
          },
          "Key": "KEY",
          "Owner": {
            "DisplayName": null,
            "ID": "XXX"
          },
          "StorageClass": "STANDARD",
          "UploadId": "XXX"
        }
      ],
      "rclone-1000files": [],
      "rclone-dst": []
    }

`,
}, {
	Name:  "cleanup",
	Short: "Remove unfinished multipart uploads.",
	Long: `This command removes unfinished multipart uploads of age greater than
max-age which defaults to 24 hours.

Note that you can use -i/--dry-run with this command to see what it
would do.

    rclone backend cleanup s3:bucket/path/to/object
    rclone backend cleanup -o max-age=7w s3:bucket/path/to/object

Durations are parsed as per the rest of rclone, 2h, 7d, 7w etc.
`,
	Opts: map[string]string{
		"max-age": "Max age of upload to delete",
	},
}}

// Command the backend to run a named command
//
// The command run is name
// args may be used to read arguments from
// opts may be used to read optional arguments from
//
// The result should be capable of being JSON encoded
// If it is a string or a []string it will be shown to the user
// otherwise it will be JSON encoded and shown to the user like that
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (out interface{}, err error) {
	switch name {
	case "restore":
		req := s3.RestoreObjectInput{
			//Bucket:         &f.rootBucket,
			//Key:            &encodedDirectory,
			RestoreRequest: &s3.RestoreRequest{},
		}
		if lifetime := opt["lifetime"]; lifetime != "" {
			ilifetime, err := strconv.ParseInt(lifetime, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("bad lifetime: %w", err)
			}
			req.RestoreRequest.Days = &ilifetime
		}
		if priority := opt["priority"]; priority != "" {
			req.RestoreRequest.GlacierJobParameters = &s3.GlacierJobParameters{
				Tier: &priority,
			}
		}
		if description := opt["description"]; description != "" {
			req.RestoreRequest.Description = &description
		}
		type status struct {
			Status string
			Remote string
		}
		var (
			outMu sync.Mutex
			out   = []status{}
		)
		err = operations.ListFn(ctx, f, func(obj fs.Object) {
			// Remember this is run --checkers times concurrently
			o, ok := obj.(*Object)
			st := status{Status: "OK", Remote: obj.Remote()}
			defer func() {
				outMu.Lock()
				out = append(out, st)
				outMu.Unlock()
			}()
			if operations.SkipDestructive(ctx, obj, "restore") {
				return
			}
			if !ok {
				st.Status = "Not an S3 object"
				return
			}
			if o.storageClass != "GLACIER" && o.storageClass != "DEEP_ARCHIVE" {
				st.Status = "Not GLACIER or DEEP_ARCHIVE storage class"
				return
			}
			bucket, bucketPath := o.split()
			reqCopy := req
			reqCopy.Bucket = &bucket
			reqCopy.Key = &bucketPath
			err = f.pacer.Call(func() (bool, error) {
				_, err = f.c.RestoreObject(&reqCopy)
				return f.shouldRetry(ctx, err)
			})
			if err != nil {
				st.Status = err.Error()
			}
		})
		if err != nil {
			return out, err
		}
		return out, nil
	case "list-multipart-uploads":
		return f.listMultipartUploadsAll(ctx)
	case "cleanup":
		maxAge := 24 * time.Hour
		if opt["max-age"] != "" {
			maxAge, err = fs.ParseDuration(opt["max-age"])
			if err != nil {
				return nil, fmt.Errorf("bad max-age: %w", err)
			}
		}
		return nil, f.cleanUp(ctx, maxAge)
	default:
		return nil, fs.ErrorCommandNotFound
	}
}

// listMultipartUploads lists all outstanding multipart uploads for (bucket, key)
//
// Note that rather lazily we treat key as a prefix so it matches
// directories and objects. This could surprise the user if they ask
// for "dir" and it returns "dirKey"
func (f *Fs) listMultipartUploads(ctx context.Context, bucket, key string) (uploads []*s3.MultipartUpload, err error) {
	var (
		keyMarker      *string
		uploadIDMarker *string
	)
	uploads = []*s3.MultipartUpload{}
	for {
		req := s3.ListMultipartUploadsInput{
			Bucket:         &bucket,
			MaxUploads:     &f.opt.ListChunk,
			KeyMarker:      keyMarker,
			UploadIdMarker: uploadIDMarker,
			Prefix:         &key,
		}
		var resp *s3.ListMultipartUploadsOutput
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.c.ListMultipartUploads(&req)
			return f.shouldRetry(ctx, err)
		})
		if err != nil {
			return nil, fmt.Errorf("list multipart uploads bucket %q key %q: %w", bucket, key, err)
		}
		uploads = append(uploads, resp.Uploads...)
		if !aws.BoolValue(resp.IsTruncated) {
			break
		}
		keyMarker = resp.NextKeyMarker
		uploadIDMarker = resp.NextUploadIdMarker
	}
	return uploads, nil
}

func (f *Fs) listMultipartUploadsAll(ctx context.Context) (uploadsMap map[string][]*s3.MultipartUpload, err error) {
	uploadsMap = make(map[string][]*s3.MultipartUpload)
	bucket, directory := f.split("")
	if bucket != "" {
		uploads, err := f.listMultipartUploads(ctx, bucket, directory)
		if err != nil {
			return uploadsMap, err
		}
		uploadsMap[bucket] = uploads
		return uploadsMap, nil
	}
	entries, err := f.listBuckets(ctx)
	if err != nil {
		return uploadsMap, err
	}
	for _, entry := range entries {
		bucket := entry.Remote()
		uploads, listErr := f.listMultipartUploads(ctx, bucket, "")
		if listErr != nil {
			err = listErr
			fs.Errorf(f, "%v", err)
		}
		uploadsMap[bucket] = uploads
	}
	return uploadsMap, err
}

// cleanUpBucket removes all pending multipart uploads for a given bucket over the age of maxAge
func (f *Fs) cleanUpBucket(ctx context.Context, bucket string, maxAge time.Duration, uploads []*s3.MultipartUpload) (err error) {
	fs.Infof(f, "cleaning bucket %q of pending multipart uploads older than %v", bucket, maxAge)
	for _, upload := range uploads {
		if upload.Initiated != nil && upload.Key != nil && upload.UploadId != nil {
			age := time.Since(*upload.Initiated)
			what := fmt.Sprintf("pending multipart upload for bucket %q key %q dated %v (%v ago)", bucket, *upload.Key, upload.Initiated, age)
			if age > maxAge {
				fs.Infof(f, "removing %s", what)
				if operations.SkipDestructive(ctx, what, "remove pending upload") {
					continue
				}
				req := s3.AbortMultipartUploadInput{
					Bucket:   &bucket,
					UploadId: upload.UploadId,
					Key:      upload.Key,
				}
				_, abortErr := f.c.AbortMultipartUpload(&req)
				if abortErr != nil {
					err = fmt.Errorf("failed to remove %s: %w", what, abortErr)
					fs.Errorf(f, "%v", err)
				}
			} else {
				fs.Debugf(f, "ignoring %s", what)
			}
		}
	}
	return err
}

// CleanUp removes all pending multipart uploads
func (f *Fs) cleanUp(ctx context.Context, maxAge time.Duration) (err error) {
	uploadsMap, err := f.listMultipartUploadsAll(ctx)
	if err != nil {
		return err
	}
	for bucket, uploads := range uploadsMap {
		cleanErr := f.cleanUpBucket(ctx, bucket, maxAge, uploads)
		if err != nil {
			fs.Errorf(f, "Failed to cleanup bucket %q: %v", bucket, cleanErr)
			err = cleanErr
		}
	}
	return err
}

// CleanUp removes all pending multipart uploads older than 24 hours
func (f *Fs) CleanUp(ctx context.Context) (err error) {
	return f.cleanUp(ctx, 24*time.Hour)
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

var matchMd5 = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Set the MD5 from the etag
func (o *Object) setMD5FromEtag(etag string) {
	if o.fs.etagIsNotMD5 {
		o.md5 = ""
		return
	}
	if etag == "" {
		o.md5 = ""
		return
	}
	hash := strings.Trim(strings.ToLower(etag), `"`)
	// Check the etag is a valid md5sum
	if !matchMd5.MatchString(hash) {
		o.md5 = ""
		return
	}
	o.md5 = hash
}

// Hash returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	// If we haven't got an MD5, then check the metadata
	if o.md5 == "" {
		err := o.readMetaData(ctx)
		if err != nil {
			return "", err
		}
	}
	return o.md5, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.bytes
}

func (o *Object) headObject(ctx context.Context) (resp *s3.HeadObjectOutput, err error) {
	bucket, bucketPath := o.split()
	req := s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &bucketPath,
	}
	if o.fs.opt.RequesterPays {
		req.RequestPayer = aws.String(s3.RequestPayerRequester)
	}
	if o.fs.opt.SSECustomerAlgorithm != "" {
		req.SSECustomerAlgorithm = &o.fs.opt.SSECustomerAlgorithm
	}
	if o.fs.opt.SSECustomerKey != "" {
		req.SSECustomerKey = &o.fs.opt.SSECustomerKey
	}
	if o.fs.opt.SSECustomerKeyMD5 != "" {
		req.SSECustomerKeyMD5 = &o.fs.opt.SSECustomerKeyMD5
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		var err error
		resp, err = o.fs.c.HeadObjectWithContext(ctx, &req)
		return o.fs.shouldRetry(ctx, err)
	})
	if err != nil {
		if awsErr, ok := err.(awserr.RequestFailure); ok {
			if awsErr.StatusCode() == http.StatusNotFound {
				return nil, fs.ErrorObjectNotFound
			}
		}
		return nil, err
	}
	o.fs.cache.MarkOK(bucket)
	return resp, nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.meta != nil {
		return nil
	}
	resp, err := o.headObject(ctx)
	if err != nil {
		return err
	}
	o.setMetaData(resp.ETag, resp.ContentLength, resp.LastModified, resp.Metadata, resp.ContentType, resp.StorageClass)
	return nil
}

func (o *Object) setMetaData(etag *string, contentLength *int64, lastModified *time.Time, meta map[string]*string, mimeType *string, storageClass *string) {
	// Ignore missing Content-Length assuming it is 0
	// Some versions of ceph do this due their apache proxies
	if contentLength != nil {
		o.bytes = *contentLength
	}
	o.setMD5FromEtag(aws.StringValue(etag))
	o.meta = meta
	if o.meta == nil {
		o.meta = map[string]*string{}
	}
	// Read MD5 from metadata if present
	if md5sumBase64, ok := o.meta[metaMD5Hash]; ok {
		md5sumBytes, err := base64.StdEncoding.DecodeString(*md5sumBase64)
		if err != nil {
			fs.Debugf(o, "Failed to read md5sum from metadata %q: %v", *md5sumBase64, err)
		} else if len(md5sumBytes) != 16 {
			fs.Debugf(o, "Failed to read md5sum from metadata %q: wrong length", *md5sumBase64)
		} else {
			o.md5 = hex.EncodeToString(md5sumBytes)
		}
	}
	o.storageClass = aws.StringValue(storageClass)
	if lastModified == nil {
		o.lastModified = time.Now()
		fs.Logf(o, "Failed to read last modified")
	} else {
		o.lastModified = *lastModified
	}
	o.mimeType = aws.StringValue(mimeType)
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	if o.fs.ci.UseServerModTime {
		return o.lastModified
	}
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	// read mtime out of metadata if available
	d, ok := o.meta[metaMtime]
	if !ok || d == nil {
		// fs.Debugf(o, "No metadata")
		return o.lastModified
	}
	modTime, err := swift.FloatStringToTime(*d)
	if err != nil {
		fs.Logf(o, "Failed to read mtime from object: %v", err)
		return o.lastModified
	}
	return modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	err := o.readMetaData(ctx)
	if err != nil {
		return err
	}
	o.meta[metaMtime] = aws.String(swift.TimeToFloatString(modTime))

	// Can't update metadata here, so return this error to force a recopy
	if o.storageClass == "GLACIER" || o.storageClass == "DEEP_ARCHIVE" {
		return fs.ErrorCantSetModTime
	}

	// Copy the object to itself to update the metadata
	bucket, bucketPath := o.split()
	req := s3.CopyObjectInput{
		ContentType:       aws.String(fs.MimeType(ctx, o)), // Guess the content type
		Metadata:          o.meta,
		MetadataDirective: aws.String(s3.MetadataDirectiveReplace), // replace metadata with that passed in
	}
	if o.fs.opt.RequesterPays {
		req.RequestPayer = aws.String(s3.RequestPayerRequester)
	}
	return o.fs.copy(ctx, &req, bucket, bucketPath, bucket, bucketPath, o)
}

// Storable raturns a boolean indicating if this object is storable
func (o *Object) Storable() bool {
	return true
}

func (o *Object) downloadFromURL(ctx context.Context, bucketPath string, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	url := o.fs.opt.DownloadURL + bucketPath
	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		RootURL: url,
		Options: options,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srvRest.Call(ctx, &opts)
		return o.fs.shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}

	contentLength := &resp.ContentLength
	if resp.Header.Get("Content-Range") != "" {
		var contentRange = resp.Header.Get("Content-Range")
		slash := strings.IndexRune(contentRange, '/')
		if slash >= 0 {
			i, err := strconv.ParseInt(contentRange[slash+1:], 10, 64)
			if err == nil {
				contentLength = &i
			} else {
				fs.Debugf(o, "Failed to find parse integer from in %q: %v", contentRange, err)
			}
		} else {
			fs.Debugf(o, "Failed to find length in %q", contentRange)
		}
	}

	lastModified, err := time.Parse(time.RFC1123, resp.Header.Get("Last-Modified"))
	if err != nil {
		fs.Debugf(o, "Failed to parse last modified from string %s, %v", resp.Header.Get("Last-Modified"), err)
	}

	metaData := make(map[string]*string)
	for key, value := range resp.Header {
		if strings.HasPrefix(key, "x-amz-meta") {
			metaKey := strings.TrimPrefix(key, "x-amz-meta-")
			metaData[strings.Title(metaKey)] = &value[0]
		}
	}

	storageClass := resp.Header.Get("X-Amz-Storage-Class")
	contentType := resp.Header.Get("Content-Type")
	etag := resp.Header.Get("Etag")

	o.setMetaData(&etag, contentLength, &lastModified, metaData, &contentType, &storageClass)
	return resp.Body, err
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	bucket, bucketPath := o.split()

	if o.fs.opt.DownloadURL != "" {
		return o.downloadFromURL(ctx, bucketPath, options...)
	}

	req := s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &bucketPath,
	}
	if o.fs.opt.RequesterPays {
		req.RequestPayer = aws.String(s3.RequestPayerRequester)
	}
	if o.fs.opt.SSECustomerAlgorithm != "" {
		req.SSECustomerAlgorithm = &o.fs.opt.SSECustomerAlgorithm
	}
	if o.fs.opt.SSECustomerKey != "" {
		req.SSECustomerKey = &o.fs.opt.SSECustomerKey
	}
	if o.fs.opt.SSECustomerKeyMD5 != "" {
		req.SSECustomerKeyMD5 = &o.fs.opt.SSECustomerKeyMD5
	}
	httpReq, resp := o.fs.c.GetObjectRequest(&req)
	fs.FixRangeOption(options, o.bytes)
	for _, option := range options {
		switch option.(type) {
		case *fs.RangeOption, *fs.SeekOption:
			_, value := option.Header()
			req.Range = &value
		case *fs.HTTPOption:
			key, value := option.Header()
			httpReq.HTTPRequest.Header.Add(key, value)
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		var err error
		httpReq.HTTPRequest = httpReq.HTTPRequest.WithContext(ctx)
		err = httpReq.Send()
		return o.fs.shouldRetry(ctx, err)
	})
	if err, ok := err.(awserr.RequestFailure); ok {
		if err.Code() == "InvalidObjectState" {
			return nil, fmt.Errorf("Object in GLACIER, restore first: bucket=%q, key=%q", bucket, bucketPath)
		}
	}
	if err != nil {
		return nil, err
	}

	// read size from ContentLength or ContentRange
	size := resp.ContentLength
	if resp.ContentRange != nil {
		var contentRange = *resp.ContentRange
		slash := strings.IndexRune(contentRange, '/')
		if slash >= 0 {
			i, err := strconv.ParseInt(contentRange[slash+1:], 10, 64)
			if err == nil {
				size = &i
			} else {
				fs.Debugf(o, "Failed to find parse integer from in %q: %v", contentRange, err)
			}
		} else {
			fs.Debugf(o, "Failed to find length in %q", contentRange)
		}
	}
	o.setMetaData(resp.ETag, size, resp.LastModified, resp.Metadata, resp.ContentType, resp.StorageClass)
	return resp.Body, nil
}

var warnStreamUpload sync.Once

func (o *Object) uploadMultipart(ctx context.Context, req *s3.PutObjectInput, size int64, in io.Reader) (etag string, err error) {
	f := o.fs

	// make concurrency machinery
	concurrency := f.opt.UploadConcurrency
	if concurrency < 1 {
		concurrency = 1
	}
	tokens := pacer.NewTokenDispenser(concurrency)

	uploadParts := f.opt.MaxUploadParts
	if uploadParts < 1 {
		uploadParts = 1
	} else if uploadParts > maxUploadParts {
		uploadParts = maxUploadParts
	}

	// calculate size of parts
	partSize := f.opt.ChunkSize

	// size can be -1 here meaning we don't know the size of the incoming file. We use ChunkSize
	// buffers here (default 5 MiB). With a maximum number of parts (10,000) this will be a file of
	// 48 GiB which seems like a not too unreasonable limit.
	if size == -1 {
		warnStreamUpload.Do(func() {
			fs.Logf(f, "Streaming uploads using chunk size %v will have maximum file size of %v",
				f.opt.ChunkSize, fs.SizeSuffix(int64(partSize)*uploadParts))
		})
	} else {
		partSize = chunksize.Calculator(o, int(uploadParts), f.opt.ChunkSize)
	}

	memPool := f.getMemoryPool(int64(partSize))

	var mReq s3.CreateMultipartUploadInput
	structs.SetFrom(&mReq, req)
	var cout *s3.CreateMultipartUploadOutput
	err = f.pacer.Call(func() (bool, error) {
		var err error
		cout, err = f.c.CreateMultipartUploadWithContext(ctx, &mReq)
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return etag, fmt.Errorf("multipart upload failed to initialise: %w", err)
	}
	uid := cout.UploadId

	defer atexit.OnError(&err, func() {
		if o.fs.opt.LeavePartsOnError {
			return
		}
		fs.Debugf(o, "Cancelling multipart upload")
		errCancel := f.pacer.Call(func() (bool, error) {
			_, err := f.c.AbortMultipartUploadWithContext(context.Background(), &s3.AbortMultipartUploadInput{
				Bucket:       req.Bucket,
				Key:          req.Key,
				UploadId:     uid,
				RequestPayer: req.RequestPayer,
			})
			return f.shouldRetry(ctx, err)
		})
		if errCancel != nil {
			fs.Debugf(o, "Failed to cancel multipart upload: %v", errCancel)
		}
	})()

	var (
		g, gCtx  = errgroup.WithContext(ctx)
		finished = false
		partsMu  sync.Mutex // to protect parts
		parts    []*s3.CompletedPart
		off      int64
		md5sMu   sync.Mutex
		md5s     []byte
	)

	addMd5 := func(md5binary *[md5.Size]byte, partNum int64) {
		md5sMu.Lock()
		defer md5sMu.Unlock()
		start := partNum * md5.Size
		end := start + md5.Size
		if extend := end - int64(len(md5s)); extend > 0 {
			md5s = append(md5s, make([]byte, extend)...)
		}
		copy(md5s[start:end], (*md5binary)[:])
	}

	for partNum := int64(1); !finished; partNum++ {
		// Get a block of memory from the pool and token which limits concurrency.
		tokens.Get()
		buf := memPool.Get()

		free := func() {
			// return the memory and token
			memPool.Put(buf)
			tokens.Put()
		}

		// Fail fast, in case an errgroup managed function returns an error
		// gCtx is cancelled. There is no point in uploading all the other parts.
		if gCtx.Err() != nil {
			free()
			break
		}

		// Read the chunk
		var n int
		n, err = readers.ReadFill(in, buf) // this can never return 0, nil
		if err == io.EOF {
			if n == 0 && partNum != 1 { // end if no data and if not first chunk
				free()
				break
			}
			finished = true
		} else if err != nil {
			free()
			return etag, fmt.Errorf("multipart upload failed to read source: %w", err)
		}
		buf = buf[:n]

		partNum := partNum
		fs.Debugf(o, "multipart upload starting chunk %d size %v offset %v/%v", partNum, fs.SizeSuffix(n), fs.SizeSuffix(off), fs.SizeSuffix(size))
		off += int64(n)
		g.Go(func() (err error) {
			defer free()
			partLength := int64(len(buf))

			// create checksum of buffer for integrity checking
			md5sumBinary := md5.Sum(buf)
			addMd5(&md5sumBinary, partNum-1)
			md5sum := base64.StdEncoding.EncodeToString(md5sumBinary[:])

			err = f.pacer.Call(func() (bool, error) {
				uploadPartReq := &s3.UploadPartInput{
					Body:                 bytes.NewReader(buf),
					Bucket:               req.Bucket,
					Key:                  req.Key,
					PartNumber:           &partNum,
					UploadId:             uid,
					ContentMD5:           &md5sum,
					ContentLength:        &partLength,
					RequestPayer:         req.RequestPayer,
					SSECustomerAlgorithm: req.SSECustomerAlgorithm,
					SSECustomerKey:       req.SSECustomerKey,
					SSECustomerKeyMD5:    req.SSECustomerKeyMD5,
				}
				uout, err := f.c.UploadPartWithContext(gCtx, uploadPartReq)
				if err != nil {
					if partNum <= int64(concurrency) {
						return f.shouldRetry(ctx, err)
					}
					// retry all chunks once have done the first batch
					return true, err
				}
				partsMu.Lock()
				parts = append(parts, &s3.CompletedPart{
					PartNumber: &partNum,
					ETag:       uout.ETag,
				})
				partsMu.Unlock()

				return false, nil
			})
			if err != nil {
				return fmt.Errorf("multipart upload failed to upload part: %w", err)
			}
			return nil
		})
	}
	err = g.Wait()
	if err != nil {
		return etag, err
	}

	// sort the completed parts by part number
	sort.Slice(parts, func(i, j int) bool {
		return *parts[i].PartNumber < *parts[j].PartNumber
	})

	err = f.pacer.Call(func() (bool, error) {
		_, err := f.c.CompleteMultipartUploadWithContext(ctx, &s3.CompleteMultipartUploadInput{
			Bucket: req.Bucket,
			Key:    req.Key,
			MultipartUpload: &s3.CompletedMultipartUpload{
				Parts: parts,
			},
			RequestPayer: req.RequestPayer,
			UploadId:     uid,
		})
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return etag, fmt.Errorf("multipart upload failed to finalise: %w", err)
	}
	hashOfHashes := md5.Sum(md5s)
	etag = fmt.Sprintf("%s-%d", hex.EncodeToString(hashOfHashes[:]), len(parts))
	return etag, nil
}

// unWrapAwsError unwraps AWS errors, looking for a non AWS error
//
// It returns true if one was found and the error, or false and the
// error passed in.
func unWrapAwsError(err error) (found bool, outErr error) {
	if awsErr, ok := err.(awserr.Error); ok {
		var origErrs []error
		if batchErr, ok := awsErr.(awserr.BatchError); ok {
			origErrs = batchErr.OrigErrs()
		} else {
			origErrs = []error{awsErr.OrigErr()}
		}
		for _, origErr := range origErrs {
			found, newErr := unWrapAwsError(origErr)
			if found {
				return found, newErr
			}
		}
		return false, err
	}
	return true, err
}

// Upload a single part using PutObject
func (o *Object) uploadSinglepartPutObject(ctx context.Context, req *s3.PutObjectInput, size int64, in io.Reader) (etag string, lastModified time.Time, err error) {
	req.Body = readers.NewFakeSeeker(in, size)
	var resp *s3.PutObjectOutput
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err = o.fs.cu.PutObject(req)
		return o.fs.shouldRetry(ctx, err)
	})
	if err != nil {
		// Return the underlying error if we have a Serialization error if possible
		//
		// Serialization errors are synthesized locally in the SDK (not returned from the
		// server). We'll get one if the SDK attempts a retry, however the FakeSeeker will
		// remember the previous error from Read and return that.
		if do, ok := err.(awserr.Error); ok && do.Code() == request.ErrCodeSerialization {
			if found, newErr := unWrapAwsError(err); found {
				err = newErr
			}
		}
		return etag, lastModified, err
	}
	lastModified = time.Now()
	etag = aws.StringValue(resp.ETag)
	return etag, lastModified, nil
}

// Upload a single part using a presigned request
func (o *Object) uploadSinglepartPresignedRequest(ctx context.Context, req *s3.PutObjectInput, size int64, in io.Reader) (etag string, lastModified time.Time, err error) {
	// Create the request
	putObj, _ := o.fs.c.PutObjectRequest(req)

	// Sign it so we can upload using a presigned request.
	//
	// Note the SDK didn't used to support streaming to
	// PutObject so we used this work-around.
	url, headers, err := putObj.PresignRequest(15 * time.Minute)
	if err != nil {
		return etag, lastModified, fmt.Errorf("s3 upload: sign request: %w", err)
	}

	if o.fs.opt.V2Auth && headers == nil {
		headers = putObj.HTTPRequest.Header
	}

	// Set request to nil if empty so as not to make chunked encoding
	if size == 0 {
		in = nil
	}

	// create the vanilla http request
	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, in)
	if err != nil {
		return etag, lastModified, fmt.Errorf("s3 upload: new request: %w", err)
	}

	// set the headers we signed and the length
	httpReq.Header = headers
	httpReq.ContentLength = size

	var resp *http.Response
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		var err error
		resp, err = o.fs.srv.Do(httpReq)
		if err != nil {
			return o.fs.shouldRetry(ctx, err)
		}
		body, err := rest.ReadBody(resp)
		if err != nil {
			return o.fs.shouldRetry(ctx, err)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 299 {
			return false, nil
		}
		err = fmt.Errorf("s3 upload: %s: %s", resp.Status, body)
		return fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
	})
	if err != nil {
		return etag, lastModified, err
	}
	if resp != nil {
		if date, err := http.ParseTime(resp.Header.Get("Date")); err != nil {
			lastModified = date
		}
		etag = resp.Header.Get("Etag")
	}
	return etag, lastModified, nil
}

// Update the Object from in with modTime and size
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	bucket, bucketPath := o.split()
	err := o.fs.makeBucket(ctx, bucket)
	if err != nil {
		return err
	}
	modTime := src.ModTime(ctx)
	size := src.Size()

	multipart := size < 0 || size >= int64(o.fs.opt.UploadCutoff)

	// Set the mtime in the meta data
	metadata := map[string]*string{
		metaMtime: aws.String(swift.TimeToFloatString(modTime)),
	}

	// read the md5sum if available
	// - for non multipart
	//    - so we can add a ContentMD5
	//    - so we can add the md5sum in the metadata as metaMD5Hash if using SSE/SSE-C
	// - for multipart provided checksums aren't disabled
	//    - so we can add the md5sum in the metadata as metaMD5Hash
	var md5sumBase64 string
	var md5sumHex string
	if !multipart || !o.fs.opt.DisableChecksum {
		md5sumHex, err = src.Hash(ctx, hash.MD5)
		if err == nil && matchMd5.MatchString(md5sumHex) {
			hashBytes, err := hex.DecodeString(md5sumHex)
			if err == nil {
				md5sumBase64 = base64.StdEncoding.EncodeToString(hashBytes)
				if (multipart || o.fs.etagIsNotMD5) && !o.fs.opt.DisableChecksum {
					// Set the md5sum as metadata on the object if
					// - a multipart upload
					// - the Etag is not an MD5, eg when using SSE/SSE-C
					// provided checksums aren't disabled
					metadata[metaMD5Hash] = &md5sumBase64
				}
			}
		}
	}

	// Guess the content type
	mimeType := fs.MimeType(ctx, src)
	req := s3.PutObjectInput{
		Bucket:      &bucket,
		ACL:         &o.fs.opt.ACL,
		Key:         &bucketPath,
		ContentType: &mimeType,
		Metadata:    metadata,
	}
	if size >= 0 {
		req.ContentLength = &size
	}
	if md5sumBase64 != "" {
		req.ContentMD5 = &md5sumBase64
	}
	if o.fs.opt.RequesterPays {
		req.RequestPayer = aws.String(s3.RequestPayerRequester)
	}
	if o.fs.opt.ServerSideEncryption != "" {
		req.ServerSideEncryption = &o.fs.opt.ServerSideEncryption
	}
	if o.fs.opt.SSECustomerAlgorithm != "" {
		req.SSECustomerAlgorithm = &o.fs.opt.SSECustomerAlgorithm
	}
	if o.fs.opt.SSECustomerKey != "" {
		req.SSECustomerKey = &o.fs.opt.SSECustomerKey
	}
	if o.fs.opt.SSECustomerKeyMD5 != "" {
		req.SSECustomerKeyMD5 = &o.fs.opt.SSECustomerKeyMD5
	}
	if o.fs.opt.SSEKMSKeyID != "" {
		req.SSEKMSKeyId = &o.fs.opt.SSEKMSKeyID
	}
	if o.fs.opt.StorageClass != "" {
		req.StorageClass = &o.fs.opt.StorageClass
	}
	// Apply upload options
	for _, option := range options {
		key, value := option.Header()
		lowerKey := strings.ToLower(key)
		switch lowerKey {
		case "":
			// ignore
		case "cache-control":
			req.CacheControl = aws.String(value)
		case "content-disposition":
			req.ContentDisposition = aws.String(value)
		case "content-encoding":
			req.ContentEncoding = aws.String(value)
		case "content-language":
			req.ContentLanguage = aws.String(value)
		case "content-type":
			req.ContentType = aws.String(value)
		case "x-amz-tagging":
			req.Tagging = aws.String(value)
		default:
			const amzMetaPrefix = "x-amz-meta-"
			if strings.HasPrefix(lowerKey, amzMetaPrefix) {
				metaKey := lowerKey[len(amzMetaPrefix):]
				req.Metadata[metaKey] = aws.String(value)
			} else {
				fs.Errorf(o, "Don't know how to set key %q on upload", key)
			}
		}
	}

	var wantETag string        // Multipart upload Etag to check
	var gotEtag string         // Etag we got from the upload
	var lastModified time.Time // Time we got from the upload
	if multipart {
		wantETag, err = o.uploadMultipart(ctx, &req, size, in)
	} else {
		if o.fs.opt.UsePresignedRequest {
			gotEtag, lastModified, err = o.uploadSinglepartPresignedRequest(ctx, &req, size, in)
		} else {
			gotEtag, lastModified, err = o.uploadSinglepartPutObject(ctx, &req, size, in)
		}
	}
	if err != nil {
		return err
	}

	// User requested we don't HEAD the object after uploading it
	// so make up the object as best we can assuming it got
	// uploaded properly. If size < 0 then we need to do the HEAD.
	if o.fs.opt.NoHead && size >= 0 {
		o.md5 = md5sumHex
		o.bytes = size
		o.lastModified = time.Now()
		o.meta = req.Metadata
		o.mimeType = aws.StringValue(req.ContentType)
		o.storageClass = aws.StringValue(req.StorageClass)
		// If we have done a single part PUT request then we can read these
		if gotEtag != "" {
			o.setMD5FromEtag(gotEtag)
		}
		if !o.lastModified.IsZero() {
			o.lastModified = lastModified
		}
		return nil
	}

	// Read the metadata from the newly created object
	o.meta = nil // wipe old metadata
	head, err := o.headObject(ctx)
	if err != nil {
		return err
	}
	o.setMetaData(head.ETag, head.ContentLength, head.LastModified, head.Metadata, head.ContentType, head.StorageClass)
	if o.fs.opt.UseMultipartEtag.Value && !o.fs.etagIsNotMD5 && wantETag != "" && head.ETag != nil && *head.ETag != "" {
		gotETag := strings.Trim(strings.ToLower(*head.ETag), `"`)
		if wantETag != gotETag {
			return fmt.Errorf("multipart upload corrupted: Etag differ: expecting %s but got %s", wantETag, gotETag)
		}
		fs.Debugf(o, "Multipart upload Etag: %s OK", wantETag)
	}
	return err
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	bucket, bucketPath := o.split()
	req := s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &bucketPath,
	}
	if o.fs.opt.RequesterPays {
		req.RequestPayer = aws.String(s3.RequestPayerRequester)
	}
	err := o.fs.pacer.Call(func() (bool, error) {
		_, err := o.fs.c.DeleteObjectWithContext(ctx, &req)
		return o.fs.shouldRetry(ctx, err)
	})
	return err
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return ""
	}
	return o.mimeType
}

// SetTier performs changing storage class
func (o *Object) SetTier(tier string) (err error) {
	ctx := context.TODO()
	tier = strings.ToUpper(tier)
	bucket, bucketPath := o.split()
	req := s3.CopyObjectInput{
		MetadataDirective: aws.String(s3.MetadataDirectiveCopy),
		StorageClass:      aws.String(tier),
	}
	err = o.fs.copy(ctx, &req, bucket, bucketPath, bucket, bucketPath, o)
	if err != nil {
		return err
	}
	o.storageClass = tier
	return err
}

// GetTier returns storage class as string
func (o *Object) GetTier() string {
	if o.storageClass == "" {
		return "STANDARD"
	}
	return o.storageClass
}

// Check the interfaces are satisfied
var (
	_ fs.Fs          = &Fs{}
	_ fs.Copier      = &Fs{}
	_ fs.PutStreamer = &Fs{}
	_ fs.ListRer     = &Fs{}
	_ fs.Commander   = &Fs{}
	_ fs.CleanUpper  = &Fs{}
	_ fs.Object      = &Object{}
	_ fs.MimeTyper   = &Object{}
	_ fs.GetTierer   = &Object{}
	_ fs.SetTierer   = &Object{}
)
