// Package s3 provides an interface to Amazon S3 object storage
package s3

//go:generate go run gen_setfrom.go -o setfrom.go

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/service/s3/s3manager"

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
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/ncw/swift/v2"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
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
	"github.com/rclone/rclone/lib/multipart"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/pool"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/lib/rest"
	"github.com/rclone/rclone/lib/version"
	"golang.org/x/net/http/httpguts"
	"golang.org/x/sync/errgroup"
)

// The S3 providers
//
// Please keep these in alphabetical order, but with AWS first and
// Other last.
//
// NB if you add a new provider here, then add it in the setQuirks
// function and set the correct quirks. Test the quirks are correct by
// running the integration tests "go test -v -remote NewS3Provider:".
//
// See https://github.com/rclone/rclone/blob/master/CONTRIBUTING.md#adding-a-new-s3-provider
// for full information about how to add a new s3 provider.
var providerOption = fs.Option{
	Name: fs.ConfigProvider,
	Help: "Choose your S3 provider.",
	Examples: []fs.OptionExample{{
		Value: "AWS",
		Help:  "Amazon Web Services (AWS) S3",
	}, {
		Value: "Alibaba",
		Help:  "Alibaba Cloud Object Storage System (OSS) formerly Aliyun",
	}, {
		Value: "ArvanCloud",
		Help:  "Arvan Cloud Object Storage (AOS)",
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
		Value: "DigitalOcean",
		Help:  "DigitalOcean Spaces",
	}, {
		Value: "Dreamhost",
		Help:  "Dreamhost DreamObjects",
	}, {
		Value: "GCS",
		Help:  "Google Cloud Storage",
	}, {
		Value: "HuaweiOBS",
		Help:  "Huawei Object Storage Service",
	}, {
		Value: "IBMCOS",
		Help:  "IBM COS S3",
	}, {
		Value: "IDrive",
		Help:  "IDrive e2",
	}, {
		Value: "IONOS",
		Help:  "IONOS Cloud",
	}, {
		Value: "LyveCloud",
		Help:  "Seagate Lyve Cloud",
	}, {
		Value: "Leviia",
		Help:  "Leviia Object Storage",
	}, {
		Value: "Liara",
		Help:  "Liara Object Storage",
	}, {
		Value: "Linode",
		Help:  "Linode Object Storage",
	}, {
		Value: "Minio",
		Help:  "Minio Object Storage",
	}, {
		Value: "Netease",
		Help:  "Netease Object Storage (NOS)",
	}, {
		Value: "Petabox",
		Help:  "Petabox Object Storage",
	}, {
		Value: "RackCorp",
		Help:  "RackCorp Object Storage",
	}, {
		Value: "Rclone",
		Help:  "Rclone S3 Server",
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
		Value: "Synology",
		Help:  "Synology C2 Object Storage",
	}, {
		Value: "TencentCOS",
		Help:  "Tencent Cloud Object Storage (COS)",
	}, {
		Value: "Wasabi",
		Help:  "Wasabi Object Storage",
	}, {
		Value: "Qiniu",
		Help:  "Qiniu Object Storage (Kodo)",
	}, {
		Value: "Other",
		Help:  "Any other S3 compatible provider",
	}},
}

var providersList string

// Register with Fs
func init() {
	var s strings.Builder
	for i, provider := range providerOption.Examples {
		if provider.Value == "Other" {
			_, _ = s.WriteString(" and others")
		} else {
			if i != 0 {
				_, _ = s.WriteString(", ")
			}
			_, _ = s.WriteString(provider.Value)
		}
	}
	providersList = s.String()
	fs.Register(&fs.RegInfo{
		Name:        "s3",
		Description: "Amazon S3 Compliant Storage Providers including " + providersList,
		NewFs:       NewFs,
		CommandHelp: commandHelp,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			switch config.State {
			case "":
				return nil, setEndpointValueForIDriveE2(m)
			}
			return nil, fmt.Errorf("unknown state %q", config.State)
		},
		MetadataInfo: &fs.MetadataInfo{
			System: systemMetadataInfo,
			Help:   `User metadata is stored as x-amz-meta- keys. S3 metadata keys are case insensitive and are always returned in lower case.`,
		},
		Options: []fs.Option{providerOption, {
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
			Name:      "access_key_id",
			Help:      "AWS Access Key ID.\n\nLeave blank for anonymous access or runtime credentials.",
			Sensitive: true,
		}, {
			Name:      "secret_access_key",
			Help:      "AWS Secret Access Key (password).\n\nLeave blank for anonymous access or runtime credentials.",
			Sensitive: true,
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
			// References:
			// https://developer.qiniu.com/kodo/4088/s3-access-domainname
			Name:     "region",
			Help:     "Region to connect to.",
			Provider: "Qiniu",
			Examples: []fs.OptionExample{{
				Value: "cn-east-1",
				Help:  "The default endpoint - a good choice if you are unsure.\nEast China Region 1.\nNeeds location constraint cn-east-1.",
			}, {
				Value: "cn-east-2",
				Help:  "East China Region 2.\nNeeds location constraint cn-east-2.",
			}, {
				Value: "cn-north-1",
				Help:  "North China Region 1.\nNeeds location constraint cn-north-1.",
			}, {
				Value: "cn-south-1",
				Help:  "South China Region 1.\nNeeds location constraint cn-south-1.",
			}, {
				Value: "us-north-1",
				Help:  "North America Region.\nNeeds location constraint us-north-1.",
			}, {
				Value: "ap-southeast-1",
				Help:  "Southeast Asia Region 1.\nNeeds location constraint ap-southeast-1.",
			}, {
				Value: "ap-northeast-1",
				Help:  "Northeast Asia Region 1.\nNeeds location constraint ap-northeast-1.",
			}},
		}, {
			Name:     "region",
			Help:     "Region where your bucket will be created and your data stored.\n",
			Provider: "IONOS",
			Examples: []fs.OptionExample{{
				Value: "de",
				Help:  "Frankfurt, Germany",
			}, {
				Value: "eu-central-2",
				Help:  "Berlin, Germany",
			}, {
				Value: "eu-south-2",
				Help:  "Logrono, Spain",
			}},
		}, {
			Name:     "region",
			Help:     "Region where your bucket will be created and your data stored.\n",
			Provider: "Petabox",
			Examples: []fs.OptionExample{{
				Value: "us-east-1",
				Help:  "US East (N. Virginia)",
			}, {
				Value: "eu-central-1",
				Help:  "Europe (Frankfurt)",
			}, {
				Value: "ap-southeast-1",
				Help:  "Asia Pacific (Singapore)",
			}, {
				Value: "me-south-1",
				Help:  "Middle East (Bahrain)",
			}, {
				Value: "sa-east-1",
				Help:  "South America (São Paulo)",
			}},
		}, {
			Name:     "region",
			Help:     "Region where your data stored.\n",
			Provider: "Synology",
			Examples: []fs.OptionExample{{
				Value: "eu-001",
				Help:  "Europe Region 1",
			}, {
				Value: "eu-002",
				Help:  "Europe Region 2",
			}, {
				Value: "us-001",
				Help:  "US Region 1",
			}, {
				Value: "us-002",
				Help:  "US Region 2",
			}, {
				Value: "tw-001",
				Help:  "Asia (Taiwan)",
			}},
		}, {
			Name:     "region",
			Help:     "Region to connect to.\n\nLeave blank if you are using an S3 clone and you don't have a region.",
			Provider: "!AWS,Alibaba,ArvanCloud,ChinaMobile,Cloudflare,IONOS,Petabox,Liara,Linode,Qiniu,RackCorp,Scaleway,Storj,Synology,TencentCOS,HuaweiOBS,IDrive",
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
			// ArvanCloud endpoints: https://www.arvancloud.ir/en/products/cloud-storage
			Name:     "endpoint",
			Help:     "Endpoint for Arvan Cloud Object Storage (AOS) API.",
			Provider: "ArvanCloud",
			Examples: []fs.OptionExample{{
				Value: "s3.ir-thr-at1.arvanstorage.ir",
				Help:  "The default endpoint - a good choice if you are unsure.\nTehran Iran (Simin)",
			}, {
				Value: "s3.ir-tbz-sh1.arvanstorage.ir",
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
			Name:     "endpoint",
			Help:     "Endpoint for IONOS S3 Object Storage.\n\nSpecify the endpoint from the same region.",
			Provider: "IONOS",
			Examples: []fs.OptionExample{{
				Value: "s3-eu-central-1.ionoscloud.com",
				Help:  "Frankfurt, Germany",
			}, {
				Value: "s3-eu-central-2.ionoscloud.com",
				Help:  "Berlin, Germany",
			}, {
				Value: "s3-eu-south-2.ionoscloud.com",
				Help:  "Logrono, Spain",
			}},
		}, {
			Name:     "endpoint",
			Help:     "Endpoint for Petabox S3 Object Storage.\n\nSpecify the endpoint from the same region.",
			Provider: "Petabox",
			Required: true,
			Examples: []fs.OptionExample{{
				Value: "s3.petabox.io",
				Help:  "US East (N. Virginia)",
			}, {
				Value: "s3.us-east-1.petabox.io",
				Help:  "US East (N. Virginia)",
			}, {
				Value: "s3.eu-central-1.petabox.io",
				Help:  "Europe (Frankfurt)",
			}, {
				Value: "s3.ap-southeast-1.petabox.io",
				Help:  "Asia Pacific (Singapore)",
			}, {
				Value: "s3.me-south-1.petabox.io",
				Help:  "Middle East (Bahrain)",
			}, {
				Value: "s3.sa-east-1.petabox.io",
				Help:  "South America (São Paulo)",
			}},
		}, {
			// Leviia endpoints: https://www.leviia.com/object-storage/
			Name:     "endpoint",
			Help:     "Endpoint for Leviia Object Storage API.",
			Provider: "Leviia",
			Examples: []fs.OptionExample{{
				Value: "s3.leviia.com",
				Help:  "The default endpoint\nLeviia",
			}},
		}, {
			// Liara endpoints: https://liara.ir/landing/object-storage
			Name:     "endpoint",
			Help:     "Endpoint for Liara Object Storage API.",
			Provider: "Liara",
			Examples: []fs.OptionExample{{
				Value: "storage.iran.liara.space",
				Help:  "The default endpoint\nIran",
			}},
		}, {
			// Linode endpoints: https://www.linode.com/docs/products/storage/object-storage/guides/urls/#cluster-url-s3-endpoint
			Name:     "endpoint",
			Help:     "Endpoint for Linode Object Storage API.",
			Provider: "Linode",
			Examples: []fs.OptionExample{{
				Value: "us-southeast-1.linodeobjects.com",
				Help:  "Atlanta, GA (USA), us-southeast-1",
			}, {
				Value: "us-ord-1.linodeobjects.com",
				Help:  "Chicago, IL (USA), us-ord-1",
			}, {
				Value: "eu-central-1.linodeobjects.com",
				Help:  "Frankfurt (Germany), eu-central-1",
			}, {
				Value: "it-mil-1.linodeobjects.com",
				Help:  "Milan (Italy), it-mil-1",
			}, {
				Value: "us-east-1.linodeobjects.com",
				Help:  "Newark, NJ (USA), us-east-1",
			}, {
				Value: "fr-par-1.linodeobjects.com",
				Help:  "Paris (France), fr-par-1",
			}, {
				Value: "us-sea-1.linodeobjects.com",
				Help:  "Seattle, WA (USA), us-sea-1",
			}, {
				Value: "ap-south-1.linodeobjects.com",
				Help:  "Singapore ap-south-1",
			}, {
				Value: "se-sto-1.linodeobjects.com",
				Help:  "Stockholm (Sweden), se-sto-1",
			}, {
				Value: "us-iad-1.linodeobjects.com",
				Help:  "Washington, DC, (USA), us-iad-1",
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
			Help:     "Endpoint for Google Cloud Storage.",
			Provider: "GCS",
			Examples: []fs.OptionExample{{
				Value: "https://storage.googleapis.com",
				Help:  "Google Cloud Storage endpoint",
			}},
		}, {
			Name:     "endpoint",
			Help:     "Endpoint for Storj Gateway.",
			Provider: "Storj",
			Examples: []fs.OptionExample{{
				Value: "gateway.storjshare.io",
				Help:  "Global Hosted Gateway",
			}},
		}, {
			Name:     "endpoint",
			Help:     "Endpoint for Synology C2 Object Storage API.",
			Provider: "Synology",
			Examples: []fs.OptionExample{{
				Value: "eu-001.s3.synologyc2.net",
				Help:  "EU Endpoint 1",
			}, {
				Value: "eu-002.s3.synologyc2.net",
				Help:  "EU Endpoint 2",
			}, {
				Value: "us-001.s3.synologyc2.net",
				Help:  "US Endpoint 1",
			}, {
				Value: "us-002.s3.synologyc2.net",
				Help:  "US Endpoint 2",
			}, {
				Value: "tw-001.s3.synologyc2.net",
				Help:  "TW Endpoint 1",
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
			// Qiniu endpoints: https://developer.qiniu.com/kodo/4088/s3-access-domainname
			Name:     "endpoint",
			Help:     "Endpoint for Qiniu Object Storage.",
			Provider: "Qiniu",
			Examples: []fs.OptionExample{{
				Value: "s3-cn-east-1.qiniucs.com",
				Help:  "East China Endpoint 1",
			}, {
				Value: "s3-cn-east-2.qiniucs.com",
				Help:  "East China Endpoint 2",
			}, {
				Value: "s3-cn-north-1.qiniucs.com",
				Help:  "North China Endpoint 1",
			}, {
				Value: "s3-cn-south-1.qiniucs.com",
				Help:  "South China Endpoint 1",
			}, {
				Value: "s3-us-north-1.qiniucs.com",
				Help:  "North America Endpoint 1",
			}, {
				Value: "s3-ap-southeast-1.qiniucs.com",
				Help:  "Southeast Asia Endpoint 1",
			}, {
				Value: "s3-ap-northeast-1.qiniucs.com",
				Help:  "Northeast Asia Endpoint 1",
			}},
		}, {
			Name:     "endpoint",
			Help:     "Endpoint for S3 API.\n\nRequired when using an S3 clone.",
			Provider: "!AWS,ArvanCloud,IBMCOS,IDrive,IONOS,TencentCOS,HuaweiOBS,Alibaba,ChinaMobile,GCS,Liara,Linode,Scaleway,StackPath,Storj,Synology,RackCorp,Qiniu,Petabox",
			Examples: []fs.OptionExample{{
				Value:    "objects-us-east-1.dream.io",
				Help:     "Dream Objects endpoint",
				Provider: "Dreamhost",
			}, {
				Value:    "syd1.digitaloceanspaces.com",
				Help:     "DigitalOcean Spaces Sydney 1",
				Provider: "DigitalOcean",
			}, {
				Value:    "sfo3.digitaloceanspaces.com",
				Help:     "DigitalOcean Spaces San Francisco 3",
				Provider: "DigitalOcean",
			}, {
				Value:    "fra1.digitaloceanspaces.com",
				Help:     "DigitalOcean Spaces Frankfurt 1",
				Provider: "DigitalOcean",
			}, {
				Value:    "nyc3.digitaloceanspaces.com",
				Help:     "DigitalOcean Spaces New York 3",
				Provider: "DigitalOcean",
			}, {
				Value:    "ams3.digitaloceanspaces.com",
				Help:     "DigitalOcean Spaces Amsterdam 3",
				Provider: "DigitalOcean",
			}, {
				Value:    "sgp1.digitaloceanspaces.com",
				Help:     "DigitalOcean Spaces Singapore 1",
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
				Help:     "Wasabi US East 1 (N. Virginia)",
				Provider: "Wasabi",
			}, {
				Value:    "s3.us-east-2.wasabisys.com",
				Help:     "Wasabi US East 2 (N. Virginia)",
				Provider: "Wasabi",
			}, {
				Value:    "s3.us-central-1.wasabisys.com",
				Help:     "Wasabi US Central 1 (Texas)",
				Provider: "Wasabi",
			}, {
				Value:    "s3.us-west-1.wasabisys.com",
				Help:     "Wasabi US West 1 (Oregon)",
				Provider: "Wasabi",
			}, {
				Value:    "s3.ca-central-1.wasabisys.com",
				Help:     "Wasabi CA Central 1 (Toronto)",
				Provider: "Wasabi",
			}, {
				Value:    "s3.eu-central-1.wasabisys.com",
				Help:     "Wasabi EU Central 1 (Amsterdam)",
				Provider: "Wasabi",
			}, {
				Value:    "s3.eu-central-2.wasabisys.com",
				Help:     "Wasabi EU Central 2 (Frankfurt)",
				Provider: "Wasabi",
			}, {
				Value:    "s3.eu-west-1.wasabisys.com",
				Help:     "Wasabi EU West 1 (London)",
				Provider: "Wasabi",
			}, {
				Value:    "s3.eu-west-2.wasabisys.com",
				Help:     "Wasabi EU West 2 (Paris)",
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
				Value:    "s3.ap-southeast-1.wasabisys.com",
				Help:     "Wasabi AP Southeast 1 (Singapore)",
				Provider: "Wasabi",
			}, {
				Value:    "s3.ap-southeast-2.wasabisys.com",
				Help:     "Wasabi AP Southeast 2 (Sydney)",
				Provider: "Wasabi",
			}, {
				Value:    "storage.iran.liara.space",
				Help:     "Liara Iran endpoint",
				Provider: "Liara",
			}, {
				Value:    "s3.ir-thr-at1.arvanstorage.ir",
				Help:     "ArvanCloud Tehran Iran (Simin) endpoint",
				Provider: "ArvanCloud",
			}, {
				Value:    "s3.ir-tbz-sh1.arvanstorage.ir",
				Help:     "ArvanCloud Tabriz Iran (Shahriar) endpoint",
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
				Help:  "Tehran Iran (Simin)",
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
			Help:     "Location constraint - must be set to match the Region.\n\nUsed when creating buckets only.",
			Provider: "Qiniu",
			Examples: []fs.OptionExample{{
				Value: "cn-east-1",
				Help:  "East China Region 1",
			}, {
				Value: "cn-east-2",
				Help:  "East China Region 2",
			}, {
				Value: "cn-north-1",
				Help:  "North China Region 1",
			}, {
				Value: "cn-south-1",
				Help:  "South China Region 1",
			}, {
				Value: "us-north-1",
				Help:  "North America Region 1",
			}, {
				Value: "ap-southeast-1",
				Help:  "Southeast Asia Region 1",
			}, {
				Value: "ap-northeast-1",
				Help:  "Northeast Asia Region 1",
			}},
		}, {
			Name:     "location_constraint",
			Help:     "Location constraint - must be set to match the Region.\n\nLeave blank if not sure. Used when creating buckets only.",
			Provider: "!AWS,Alibaba,ArvanCloud,HuaweiOBS,ChinaMobile,Cloudflare,IBMCOS,IDrive,IONOS,Leviia,Liara,Linode,Qiniu,RackCorp,Scaleway,StackPath,Storj,TencentCOS,Petabox",
		}, {
			Name: "acl",
			Help: `Canned ACL used when creating buckets and storing or copying objects.

This ACL is used for creating objects and if bucket_acl isn't set, for creating buckets too.

For more info visit https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl

Note that this ACL is applied when server-side copying objects as S3
doesn't copy the ACL from the source but rather writes a fresh one.

If the acl is an empty string then no X-Amz-Acl: header is added and
the default (private) will be used.
`,
			Provider: "!Storj,Synology,Cloudflare",
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
isn't set then "acl" is used instead.

If the "acl" and "bucket_acl" are empty strings then no X-Amz-Acl:
header is added and the default (private) will be used.
`,
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
			Sensitive: true,
		}, {
			Name: "sse_customer_key",
			Help: `To use SSE-C you may provide the secret encryption key used to encrypt/decrypt your data.

Alternatively you can provide --sse-customer-key-base64.`,
			Provider: "AWS,Ceph,ChinaMobile,Minio",
			Advanced: true,
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "None",
			}},
			Sensitive: true,
		}, {
			Name: "sse_customer_key_base64",
			Help: `If using SSE-C you must provide the secret encryption key encoded in base64 format to encrypt/decrypt your data.

Alternatively you can provide --sse-customer-key.`,
			Provider: "AWS,Ceph,ChinaMobile,Minio",
			Advanced: true,
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "None",
			}},
			Sensitive: true,
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
			Sensitive: true,
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
			// Mapping from here: https://liara.ir/landing/object-storage
			Name:     "storage_class",
			Help:     "The storage class to use when storing new objects in Liara",
			Provider: "Liara",
			Examples: []fs.OptionExample{{
				Value: "STANDARD",
				Help:  "Standard storage class",
			}},
		}, {
			// Mapping from here: https://www.arvancloud.ir/en/products/cloud-storage
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
			// Mapping from here: https://www.scaleway.com/en/docs/storage/object/quickstart/
			Name:     "storage_class",
			Help:     "The storage class to use when storing new objects in S3.",
			Provider: "Scaleway",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "Default.",
			}, {
				Value: "STANDARD",
				Help:  "The Standard class for any upload.\nSuitable for on-demand content like streaming or CDN.\nAvailable in all regions.",
			}, {
				Value: "GLACIER",
				Help:  "Archived storage.\nPrices are lower, but it needs to be restored first to be accessed.\nAvailable in FR-PAR and NL-AMS regions.",
			}, {
				Value: "ONEZONE_IA",
				Help:  "One Zone - Infrequent Access.\nA good choice for storing secondary backup copies or easily re-creatable data.\nAvailable in the FR-PAR region only.",
			}},
		}, {
			// Mapping from here: https://developer.qiniu.com/kodo/5906/storage-type
			Name:     "storage_class",
			Help:     "The storage class to use when storing new objects in Qiniu.",
			Provider: "Qiniu",
			Examples: []fs.OptionExample{{
				Value: "STANDARD",
				Help:  "Standard storage class",
			}, {
				Value: "LINE",
				Help:  "Infrequent access storage mode",
			}, {
				Value: "GLACIER",
				Help:  "Archive storage mode",
			}, {
				Value: "DEEP_ARCHIVE",
				Help:  "Deep archive storage mode",
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
larger files then you will need to increase chunk_size.

Increasing the chunk size decreases the accuracy of the progress
statistics displayed with "-P" flag. Rclone treats chunk as sent when
it's buffered by the AWS SDK, when in fact it may still be uploading.
A bigger chunk size means a bigger AWS SDK buffer and progress
reporting more deviating from the truth.
`,
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
			Name:      "session_token",
			Help:      "An AWS session token.",
			Advanced:  true,
			Sensitive: true,
		}, {
			Name: "upload_concurrency",
			Help: `Concurrency for multipart uploads and copies.

This is the number of chunks of the same file that are uploaded
concurrently for multipart uploads and copies.

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
			Name: "use_dual_stack",
			Help: `If true use AWS S3 dual-stack endpoint (IPv6 support).

See [AWS Docs on Dualstack Endpoints](https://docs.aws.amazon.com/AmazonS3/latest/userguide/dual-stack-endpoints.html)`,
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
			Default:  fs.Duration(time.Minute),
			Advanced: true,
			Hide:     fs.OptionHideBoth,
			Help:     `How often internal memory buffer pools will be flushed. (no longer used)`,
		}, {
			Name:     "memory_pool_use_mmap",
			Default:  false,
			Advanced: true,
			Hide:     fs.OptionHideBoth,
			Help:     `Whether to use mmap buffers in internal memory pool. (no longer used)`,
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
			Name:     "directory_markers",
			Default:  false,
			Advanced: true,
			Help: `Upload an empty object with a trailing slash when a new directory is created

Empty folders are unsupported for bucket based remotes, this option creates an empty
object ending with "/", to persist the folder.
`,
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
		}, {
			Name:     "versions",
			Help:     "Include old versions in directory listings.",
			Default:  false,
			Advanced: true,
		}, {
			Name: "version_at",
			Help: `Show file versions as they were at the specified time.

The parameter should be a date, "2006-01-02", datetime "2006-01-02
15:04:05" or a duration for that long ago, eg "100d" or "1h".

Note that when using this no file write operations are permitted,
so you can't upload files or delete them.

See [the time option docs](/docs/#time-option) for valid formats.
`,
			Default:  fs.Time{},
			Advanced: true,
		}, {
			Name: "version_deleted",
			Help: `Show deleted file markers when using versions.

This shows deleted file markers in the listing when using versions. These will appear
as 0 size files. The only operation which can be performed on them is deletion.

Deleting a delete marker will reveal the previous version.

Deleted files will always show with a timestamp.
`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "decompress",
			Help: `If set this will decompress gzip encoded objects.

It is possible to upload objects to S3 with "Content-Encoding: gzip"
set. Normally rclone will download these files as compressed objects.

If this flag is set then rclone will decompress these files with
"Content-Encoding: gzip" as they are received. This means that rclone
can't check the size and hash but the file contents will be decompressed.
`,
			Advanced: true,
			Default:  false,
		}, {
			Name: "might_gzip",
			Help: strings.ReplaceAll(`Set this if the backend might gzip objects.

Normally providers will not alter objects when they are downloaded. If
an object was not uploaded with |Content-Encoding: gzip| then it won't
be set on download.

However some providers may gzip objects even if they weren't uploaded
with |Content-Encoding: gzip| (eg Cloudflare).

A symptom of this would be receiving errors like

    ERROR corrupted on transfer: sizes differ NNN vs MMM

If you set this flag and rclone downloads an object with
Content-Encoding: gzip set and chunked transfer encoding, then rclone
will decompress the object on the fly.

If this is set to unset (the default) then rclone will choose
according to the provider setting what to apply, but you can override
rclone's choice here.
`, "|", "`"),
			Default:  fs.Tristate{},
			Advanced: true,
		}, {
			Name: "use_accept_encoding_gzip",
			Help: strings.ReplaceAll(`Whether to send |Accept-Encoding: gzip| header.

By default, rclone will append |Accept-Encoding: gzip| to the request to download
compressed objects whenever possible.

However some providers such as Google Cloud Storage may alter the HTTP headers, breaking
the signature of the request.

A symptom of this would be receiving errors like

	SignatureDoesNotMatch: The request signature we calculated does not match the signature you provided.

In this case, you might want to try disabling this option.
`, "|", "`"),
			Default:  fs.Tristate{},
			Advanced: true,
		}, {
			Name:     "no_system_metadata",
			Help:     `Suppress setting and reading of system metadata`,
			Advanced: true,
			Default:  false,
		}, {
			Name:     "sts_endpoint",
			Help:     "Endpoint for STS.\n\nLeave blank if using AWS to use the default endpoint for the region.",
			Provider: "AWS",
			Advanced: true,
		}, {
			Name: "use_already_exists",
			Help: strings.ReplaceAll(`Set if rclone should report BucketAlreadyExists errors on bucket creation.

At some point during the evolution of the s3 protocol, AWS started
returning an |AlreadyOwnedByYou| error when attempting to create a
bucket that the user already owned, rather than a
|BucketAlreadyExists| error.

Unfortunately exactly what has been implemented by s3 clones is a
little inconsistent, some return |AlreadyOwnedByYou|, some return
|BucketAlreadyExists| and some return no error at all.

This is important to rclone because it ensures the bucket exists by
creating it on quite a lot of operations (unless
|--s3-no-check-bucket| is used).

If rclone knows the provider can return |AlreadyOwnedByYou| or returns
no error then it can report |BucketAlreadyExists| errors when the user
attempts to create a bucket not owned by them. Otherwise rclone
ignores the |BucketAlreadyExists| error which can lead to confusion.

This should be automatically set correctly for all providers rclone
knows about - please make a bug report if not.
`, "|", "`"),
			Default:  fs.Tristate{},
			Advanced: true,
		}, {
			Name: "use_multipart_uploads",
			Help: `Set if rclone should use multipart uploads.

You can change this if you want to disable the use of multipart uploads.
This shouldn't be necessary in normal operation.

This should be automatically set correctly for all providers rclone
knows about - please make a bug report if not.
`,
			Default:  fs.Tristate{},
			Advanced: true,
		},
		}})
}

// Constants
const (
	metaMtime   = "mtime"     // the meta key to store mtime in - e.g. X-Amz-Meta-Mtime
	metaMD5Hash = "md5chksum" // the meta key to store md5hash in
	// The maximum size of object we can COPY - this should be 5 GiB but is < 5 GB for b2 compatibility
	// See https://forum.rclone.org/t/copying-files-within-a-b2-bucket/16680/76
	maxSizeForCopy      = 4768 * 1024 * 1024
	maxUploadParts      = 10000 // maximum allowed number of parts in a multi-part upload
	minChunkSize        = fs.SizeSuffix(1024 * 1024 * 5)
	defaultUploadCutoff = fs.SizeSuffix(200 * 1024 * 1024)
	maxUploadCutoff     = fs.SizeSuffix(5 * 1024 * 1024 * 1024)
	minSleep            = 10 * time.Millisecond           // In case of error, start at 10ms sleep.
	maxExpireDuration   = fs.Duration(7 * 24 * time.Hour) // max expiry is 1 week
)

// globals
var (
	errNotWithVersionAt = errors.New("can't modify or delete files in --s3-version-at mode")
)

// system metadata keys which this backend owns
var systemMetadataInfo = map[string]fs.MetadataHelp{
	"cache-control": {
		Help:    "Cache-Control header",
		Type:    "string",
		Example: "no-cache",
	},
	"content-disposition": {
		Help:    "Content-Disposition header",
		Type:    "string",
		Example: "inline",
	},
	"content-encoding": {
		Help:    "Content-Encoding header",
		Type:    "string",
		Example: "gzip",
	},
	"content-language": {
		Help:    "Content-Language header",
		Type:    "string",
		Example: "en-US",
	},
	"content-type": {
		Help:    "Content-Type header",
		Type:    "string",
		Example: "text/plain",
	},
	// "tagging": {
	// 	Help:    "x-amz-tagging header",
	// 	Type:    "string",
	// 	Example: "tag1=value1&tag2=value2",
	// },
	"tier": {
		Help:     "Tier of the object",
		Type:     "string",
		Example:  "GLACIER",
		ReadOnly: true,
	},
	"mtime": {
		Help:    "Time of last modification, read from rclone metadata",
		Type:    "RFC 3339",
		Example: "2006-01-02T15:04:05.999999999Z07:00",
	},
	"btime": {
		Help:     "Time of file birth (creation) read from Last-Modified header",
		Type:     "RFC 3339",
		Example:  "2006-01-02T15:04:05.999999999Z07:00",
		ReadOnly: true,
	},
}

// Options defines the configuration for this backend
type Options struct {
	Provider              string               `config:"provider"`
	EnvAuth               bool                 `config:"env_auth"`
	AccessKeyID           string               `config:"access_key_id"`
	SecretAccessKey       string               `config:"secret_access_key"`
	Region                string               `config:"region"`
	Endpoint              string               `config:"endpoint"`
	STSEndpoint           string               `config:"sts_endpoint"`
	UseDualStack          bool                 `config:"use_dual_stack"`
	LocationConstraint    string               `config:"location_constraint"`
	ACL                   string               `config:"acl"`
	BucketACL             string               `config:"bucket_acl"`
	RequesterPays         bool                 `config:"requester_pays"`
	ServerSideEncryption  string               `config:"server_side_encryption"`
	SSEKMSKeyID           string               `config:"sse_kms_key_id"`
	SSECustomerAlgorithm  string               `config:"sse_customer_algorithm"`
	SSECustomerKey        string               `config:"sse_customer_key"`
	SSECustomerKeyBase64  string               `config:"sse_customer_key_base64"`
	SSECustomerKeyMD5     string               `config:"sse_customer_key_md5"`
	StorageClass          string               `config:"storage_class"`
	UploadCutoff          fs.SizeSuffix        `config:"upload_cutoff"`
	CopyCutoff            fs.SizeSuffix        `config:"copy_cutoff"`
	ChunkSize             fs.SizeSuffix        `config:"chunk_size"`
	MaxUploadParts        int                  `config:"max_upload_parts"`
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
	DisableHTTP2          bool                 `config:"disable_http2"`
	DownloadURL           string               `config:"download_url"`
	DirectoryMarkers      bool                 `config:"directory_markers"`
	UseMultipartEtag      fs.Tristate          `config:"use_multipart_etag"`
	UsePresignedRequest   bool                 `config:"use_presigned_request"`
	Versions              bool                 `config:"versions"`
	VersionAt             fs.Time              `config:"version_at"`
	VersionDeleted        bool                 `config:"version_deleted"`
	Decompress            bool                 `config:"decompress"`
	MightGzip             fs.Tristate          `config:"might_gzip"`
	UseAcceptEncodingGzip fs.Tristate          `config:"use_accept_encoding_gzip"`
	NoSystemMetadata      bool                 `config:"no_system_metadata"`
	UseAlreadyExists      fs.Tristate          `config:"use_already_exists"`
	UseMultipartUploads   fs.Tristate          `config:"use_multipart_uploads"`
}

// Fs represents a remote s3 server
type Fs struct {
	name           string           // the name of the remote
	root           string           // root of the bucket - ignore all objects above this
	opt            Options          // parsed options
	ci             *fs.ConfigInfo   // global config
	ctx            context.Context  // global context for reading config
	features       *fs.Features     // optional features
	c              *s3.S3           // the connection to the s3 server
	ses            *session.Session // the s3 session
	rootBucket     string           // bucket part of root (if any)
	rootDirectory  string           // directory part of root (if any)
	cache          *bucket.Cache    // cache for bucket creation status
	pacer          *fs.Pacer        // To pace the API calls
	srv            *http.Client     // a plain http client
	srvRest        *rest.Client     // the rest connection to the server
	etagIsNotMD5   bool             // if set ETags are not MD5s
	versioningMu   sync.Mutex
	versioning     fs.Tristate // if set bucket is using versions
	warnCompressed sync.Once   // warn once about compressed files
}

// Object describes a s3 object
type Object struct {
	// Will definitely have everything but meta which may be nil
	//
	// List will read everything but meta & mimeType - to fill
	// that in you need to call readMetaData
	fs           *Fs               // what this object is part of
	remote       string            // The remote path
	md5          string            // md5sum of the object
	bytes        int64             // size of the object
	lastModified time.Time         // Last modified
	meta         map[string]string // The object metadata if known - may be nil - with lower case keys
	mimeType     string            // MimeType of object - may be ""
	versionID    *string           // If present this points to an object version

	// Metadata as pointers to strings as they often won't be present
	storageClass       *string // e.g. GLACIER
	cacheControl       *string // Cache-Control: header
	contentDisposition *string // Content-Disposition: header
	contentEncoding    *string // Content-Encoding: header
	contentLanguage    *string // Content-Language: header
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

// S3 is pretty resilient, and the built in retry handling is probably sufficient
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
		// If it is a timeout then we want to retry that
		if awsError.Code() == "RequestTimeout" {
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
	bucketName, bucketPath = bucket.Split(bucket.Join(f.root, rootRelativePath))
	return f.opt.Enc.FromStandardName(bucketName), f.opt.Enc.FromStandardPath(bucketPath)
}

// split returns bucket and bucketPath from the object
func (o *Object) split() (bucket, bucketPath string) {
	bucket, bucketPath = o.fs.split(o.remote)
	// If there is an object version, then the path may have a
	// version suffix, if so remove it.
	//
	// If we are unlucky enough to have a file name with a valid
	// version path where this wasn't required (eg using
	// --s3-version-at) then this will go wrong.
	if o.versionID != nil {
		_, bucketPath = version.Remove(bucketPath)
	}
	return bucket, bucketPath
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

// Default name resolver
var defaultResolver = endpoints.DefaultResolver()

// resolve (service, region) to endpoint
//
// Used to set endpoint for s3 services and not for other services
type resolver map[string]string

// Add a service to the resolver, ignoring empty urls
func (r resolver) addService(service, url string) {
	if url == "" {
		return
	}
	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}
	r[service] = url
}

// EndpointFor return the endpoint for s3 if set or the default if not
func (r resolver) EndpointFor(service, region string, opts ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
	fs.Debugf(nil, "Resolving service %q region %q", service, region)
	url, ok := r[service]
	if ok {
		return endpoints.ResolvedEndpoint{
			URL:           url,
			SigningRegion: region,
		}, nil
	}
	return defaultResolver.EndpointFor(service, region, opts...)
}

// s3Connection makes a connection to s3
func s3Connection(ctx context.Context, opt *Options, client *http.Client) (*s3.S3, *session.Session, error) {
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
		return nil, nil, fmt.Errorf("NewSession: %w", err)
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
		fs.Debugf(nil, "Using anonymous credentials - did you mean to set env_auth=true?")
	case v.AccessKeyID == "":
		return nil, nil, errors.New("access_key_id not found")
	case v.SecretAccessKey == "":
		return nil, nil, errors.New("secret_access_key not found")
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
	if opt.Endpoint != "" || opt.STSEndpoint != "" {
		// If endpoints are set, override the relevant services only
		r := make(resolver)
		r.addService("s3", opt.Endpoint)
		r.addService("sts", opt.STSEndpoint)
		awsConfig.WithEndpointResolver(r)
	}
	if opt.UseDualStack {
		awsConfig.UseDualStackEndpoint = endpoints.DualStackEndpointStateEnabled
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
	ses, err := session.NewSessionWithOptions(awsSessionOpts)
	if err != nil {
		return nil, nil, err
	}
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
	}
	return c, ses, nil
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
	if f.opt.Provider != "Rclone" {
		err = checkUploadCutoff(cs)
	}
	if err == nil {
		old, f.opt.UploadCutoff = f.opt.UploadCutoff, cs
	}
	return
}

func (f *Fs) setCopyCutoff(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	err = checkUploadChunkSize(cs)
	if err == nil {
		old, f.opt.CopyCutoff = f.opt.CopyCutoff, cs
	}
	return
}

// setEndpointValueForIDriveE2 gets user region endpoint against the Access Key details by calling the API
func setEndpointValueForIDriveE2(m configmap.Mapper) (err error) {
	value, ok := m.Get(fs.ConfigProvider)
	if !ok || value != "IDrive" {
		return
	}
	value, ok = m.Get("access_key_id")
	if !ok || value == "" {
		return
	}
	client := &http.Client{Timeout: time.Second * 3}
	// API to get user region endpoint against the Access Key details: https://www.idrive.com/e2/guides/get_region_endpoint
	resp, err := client.Post("https://api.idrivee2.com/api/service/get_region_end_point",
		"application/json",
		strings.NewReader(`{"access_key": "`+value+`"}`))
	if err != nil {
		return
	}
	defer fs.CheckClose(resp.Body, &err)
	decoder := json.NewDecoder(resp.Body)
	var data = &struct {
		RespCode   int    `json:"resp_code"`
		RespMsg    string `json:"resp_msg"`
		DomainName string `json:"domain_name"`
	}{}
	if err = decoder.Decode(data); err == nil && data.RespCode == 0 {
		m.Set("endpoint", data.DomainName)
	}
	return
}

// Set the provider quirks
//
// There should be no testing against opt.Provider anywhere in the
// code except in here to localise the setting of the quirks.
//
// Run the integration tests to check you have the quirks correct.
//
//	go test -v -remote NewS3Provider:
func setQuirks(opt *Options) {
	var (
		listObjectsV2         = true // Always use ListObjectsV2 instead of ListObjects
		virtualHostStyle      = true // Use bucket.provider.com instead of putting the bucket in the URL
		urlEncodeListings     = true // URL encode the listings to help with control characters
		useMultipartEtag      = true // Set if Etags for multpart uploads are compatible with AWS
		useAcceptEncodingGzip = true // Set Accept-Encoding: gzip
		mightGzip             = true // assume all providers might use content encoding gzip until proven otherwise
		useAlreadyExists      = true // Set if provider returns AlreadyOwnedByYou or no error if you try to remake your own bucket
		useMultipartUploads   = true // Set if provider supports multipart uploads
	)
	switch opt.Provider {
	case "AWS":
		// No quirks
		mightGzip = false // Never auto gzips objects
	case "Alibaba":
		useMultipartEtag = false // Alibaba seems to calculate multipart Etags differently from AWS
		useAlreadyExists = true  // returns 200 OK
	case "HuaweiOBS":
		// Huawei OBS PFS is not support listObjectV2, and if turn on the urlEncodeListing, marker will not work and keep list same page forever.
		urlEncodeListings = false
		listObjectsV2 = false
		useAlreadyExists = false // untested
	case "Ceph":
		listObjectsV2 = false
		virtualHostStyle = false
		urlEncodeListings = false
		useAlreadyExists = false // untested
	case "ChinaMobile":
		listObjectsV2 = false
		virtualHostStyle = false
		urlEncodeListings = false
		useAlreadyExists = false // untested
	case "Cloudflare":
		virtualHostStyle = false
		useMultipartEtag = false // currently multipart Etags are random
	case "ArvanCloud":
		listObjectsV2 = false
		virtualHostStyle = false
		urlEncodeListings = false
		useAlreadyExists = false // untested
	case "DigitalOcean":
		urlEncodeListings = false
		useAlreadyExists = false // untested
	case "Dreamhost":
		urlEncodeListings = false
		useAlreadyExists = false // untested
	case "IBMCOS":
		listObjectsV2 = false // untested
		virtualHostStyle = false
		urlEncodeListings = false
		useMultipartEtag = false // untested
		useAlreadyExists = false // returns BucketAlreadyExists
	case "IDrive":
		virtualHostStyle = false
		useAlreadyExists = false // untested
	case "IONOS":
		// listObjectsV2 supported - https://api.ionos.com/docs/s3/#Basic-Operations-get-Bucket-list-type-2
		virtualHostStyle = false
		urlEncodeListings = false
		useAlreadyExists = false // untested
	case "Petabox":
		useAlreadyExists = false // untested
	case "Liara":
		virtualHostStyle = false
		urlEncodeListings = false
		useMultipartEtag = false
		useAlreadyExists = false // untested
	case "Linode":
		useAlreadyExists = true // returns 200 OK
	case "LyveCloud":
		useMultipartEtag = false // LyveCloud seems to calculate multipart Etags differently from AWS
		useAlreadyExists = false // untested
	case "Minio":
		virtualHostStyle = false
	case "Netease":
		listObjectsV2 = false // untested
		urlEncodeListings = false
		useMultipartEtag = false // untested
		useAlreadyExists = false // untested
	case "RackCorp":
		// No quirks
		useMultipartEtag = false // untested
		useAlreadyExists = false // untested
	case "Rclone":
		listObjectsV2 = true
		urlEncodeListings = true
		virtualHostStyle = false
		useMultipartEtag = false
		useAlreadyExists = false
		// useMultipartUploads = false - set this manually
	case "Scaleway":
		// Scaleway can only have 1000 parts in an upload
		if opt.MaxUploadParts > 1000 {
			opt.MaxUploadParts = 1000
		}
		urlEncodeListings = false
		useAlreadyExists = false // untested
	case "SeaweedFS":
		listObjectsV2 = false // untested
		virtualHostStyle = false
		urlEncodeListings = false
		useMultipartEtag = false // untested
		useAlreadyExists = false // untested
	case "StackPath":
		listObjectsV2 = false // untested
		virtualHostStyle = false
		urlEncodeListings = false
		useAlreadyExists = false // untested
	case "Storj":
		// Force chunk size to >= 64 MiB
		if opt.ChunkSize < 64*fs.Mebi {
			opt.ChunkSize = 64 * fs.Mebi
		}
		useAlreadyExists = false // returns BucketAlreadyExists
	case "Synology":
		useMultipartEtag = false
		useAlreadyExists = false // untested
	case "TencentCOS":
		listObjectsV2 = false    // untested
		useMultipartEtag = false // untested
		useAlreadyExists = false // untested
	case "Wasabi":
		useAlreadyExists = true // returns 200 OK
	case "Leviia":
		useAlreadyExists = false // untested
	case "Qiniu":
		useMultipartEtag = false
		urlEncodeListings = false
		virtualHostStyle = false
		useAlreadyExists = false // untested
	case "GCS":
		// Google break request Signature by mutating accept-encoding HTTP header
		// https://github.com/rclone/rclone/issues/6670
		useAcceptEncodingGzip = false
		useAlreadyExists = true // returns BucketNameUnavailable instead of BucketAlreadyExists but good enough!
		// GCS S3 doesn't support multi-part server side copy:
		// See: https://issuetracker.google.com/issues/323465186
		// So make cutoff very large which it does seem to support
		opt.CopyCutoff = math.MaxInt64
	default:
		fs.Logf("s3", "s3 provider %q not known - please set correctly", opt.Provider)
		fallthrough
	case "Other":
		listObjectsV2 = false
		virtualHostStyle = false
		urlEncodeListings = false
		useMultipartEtag = false
		useAlreadyExists = false
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

	// set MightGzip if not manually set
	if !opt.MightGzip.Valid {
		opt.MightGzip.Valid = true
		opt.MightGzip.Value = mightGzip
	}

	// set UseAcceptEncodingGzip if not manually set
	if !opt.UseAcceptEncodingGzip.Valid {
		opt.UseAcceptEncodingGzip.Valid = true
		opt.UseAcceptEncodingGzip.Value = useAcceptEncodingGzip
	}

	// Has the provider got AlreadyOwnedByYou error?
	if !opt.UseAlreadyExists.Valid {
		opt.UseAlreadyExists.Valid = true
		opt.UseAlreadyExists.Value = useAlreadyExists
	}

	// Set the correct use multipart uploads if not manually set
	if !opt.UseMultipartUploads.Valid {
		opt.UseMultipartUploads.Valid = true
		opt.UseMultipartUploads.Value = useMultipartUploads
	}
	if !opt.UseMultipartUploads.Value {
		opt.UploadCutoff = math.MaxInt64
	}

}

// setRoot changes the root of the Fs
func (f *Fs) setRoot(root string) {
	f.root = parsePath(root)
	f.rootBucket, f.rootDirectory = bucket.Split(f.root)
}

// return a pointer to the string if non empty or nil if it is empty
func stringPointerOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
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
	if opt.Versions && opt.VersionAt.IsSet() {
		return nil, errors.New("s3: can't use --s3-versions and --s3-version-at at the same time")
	}
	if opt.BucketACL == "" {
		opt.BucketACL = opt.ACL
	}
	if opt.SSECustomerKeyBase64 != "" && opt.SSECustomerKey != "" {
		return nil, errors.New("s3: can't use sse_customer_key and sse_customer_key_base64 at the same time")
	} else if opt.SSECustomerKeyBase64 != "" {
		// Decode the base64-encoded key and store it in the SSECustomerKey field
		decoded, err := base64.StdEncoding.DecodeString(opt.SSECustomerKeyBase64)
		if err != nil {
			return nil, fmt.Errorf("s3: Could not decode sse_customer_key_base64: %w", err)
		}
		opt.SSECustomerKey = string(decoded)
	}
	if opt.SSECustomerKey != "" && opt.SSECustomerKeyMD5 == "" {
		// calculate CustomerKeyMD5 if not supplied
		md5sumBinary := md5.Sum([]byte(opt.SSECustomerKey))
		opt.SSECustomerKeyMD5 = base64.StdEncoding.EncodeToString(md5sumBinary[:])
	}
	srv := getClient(ctx, opt)
	c, ses, err := s3Connection(ctx, opt, srv)
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
		ses:     ses,
		pacer:   pc,
		cache:   bucket.NewCache(),
		srv:     srv,
		srvRest: rest.NewClient(fshttp.NewClient(ctx)),
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
		ReadMetadata:      true,
		WriteMetadata:     true,
		UserMetadata:      true,
		BucketBased:       true,
		BucketBasedRootOK: true,
		SetTier:           true,
		GetTier:           true,
		SlowModTime:       true,
	}).Fill(ctx, f)
	if opt.Provider == "Storj" {
		f.features.SetTier = false
		f.features.GetTier = false
	}
	if opt.Provider == "IDrive" {
		f.features.SetTier = false
	}
	if opt.DirectoryMarkers {
		f.features.CanHaveEmptyDirectories = true
	}
	// f.listMultipartUploads()
	if !opt.UseMultipartUploads.Value {
		fs.Debugf(f, "Disabling multipart uploads")
		f.features.OpenChunkWriter = nil
	}

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
	return f, nil
}

// getMetaDataListing gets the metadata from the object unconditionally from the listing
//
// This is needed to find versioned objects from their paths.
//
// It may return info == nil and err == nil if a HEAD would be more appropriate
func (f *Fs) getMetaDataListing(ctx context.Context, wantRemote string) (info *s3.Object, versionID *string, err error) {
	bucket, bucketPath := f.split(wantRemote)

	// Strip the version string off if using versions
	if f.opt.Versions {
		var timestamp time.Time
		timestamp, bucketPath = version.Remove(bucketPath)
		// If the path had no version string return no info, to force caller to look it up
		if timestamp.IsZero() {
			return nil, nil, nil
		}
	}

	err = f.list(ctx, listOpt{
		bucket:       bucket,
		directory:    bucketPath,
		prefix:       f.rootDirectory,
		recurse:      true,
		withVersions: f.opt.Versions,
		findFile:     true,
		versionAt:    f.opt.VersionAt,
		hidden:       f.opt.VersionDeleted,
	}, func(gotRemote string, object *s3.Object, objectVersionID *string, isDirectory bool) error {
		if isDirectory {
			return nil
		}
		if wantRemote != gotRemote {
			return nil
		}
		info = object
		versionID = objectVersionID
		return errEndList // read only 1 item
	})
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, nil, fs.ErrorObjectNotFound
		}
		return nil, nil, err
	}
	if info == nil {
		return nil, nil, fs.ErrorObjectNotFound
	}
	return info, versionID, nil
}

// stringClonePointer clones the string pointed to by sp into new
// memory. This is useful to stop us keeping references to small
// strings carved out of large XML responses.
func stringClonePointer(sp *string) *string {
	if sp == nil {
		return nil
	}
	var s = *sp
	return &s
}

// Return an Object from a path
//
// If it can't be found it returns the error ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *s3.Object, versionID *string) (obj fs.Object, err error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info == nil && ((f.opt.Versions && version.Match(remote)) || f.opt.VersionAt.IsSet()) {
		// If versions, have to read the listing to find the correct version ID
		info, versionID, err = f.getMetaDataListing(ctx, remote)
		if err != nil {
			return nil, err
		}
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
		o.storageClass = stringClonePointer(info.StorageClass)
		o.versionID = stringClonePointer(versionID)
		// If is delete marker, show that metadata has been read as there is none to read
		if info.Size == isDeleteMarker {
			o.meta = map[string]string{}
		}
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
	return f.newObjectWithInfo(ctx, remote, nil, nil)
}

// Gets the bucket location
func (f *Fs) getBucketLocation(ctx context.Context, bucket string) (string, error) {
	region, err := s3manager.GetBucketRegion(ctx, f.ses, bucket, "", func(r *request.Request) {
		r.Config.S3ForcePathStyle = aws.Bool(f.opt.ForcePathStyle)
	})
	if err != nil {
		return "", err
	}
	return region, nil
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
	c, ses, err := s3Connection(f.ctx, &f.opt, f.srv)
	if err != nil {
		return fmt.Errorf("creating new session failed: %w", err)
	}
	f.c = c
	f.ses = ses

	fs.Logf(f, "Switched region to %q from %q", region, oldRegion)
	return nil
}

// Common interface for bucket listers
type bucketLister interface {
	List(ctx context.Context) (resp *s3.ListObjectsV2Output, versionIDs []*string, err error)
	URLEncodeListings(bool)
}

// V1 bucket lister
type v1List struct {
	f   *Fs
	req s3.ListObjectsInput
}

// Create a new V1 bucket lister
func (f *Fs) newV1List(req *s3.ListObjectsV2Input) bucketLister {
	l := &v1List{
		f: f,
	}
	// Convert v2 req into v1 req
	//structs.SetFrom(&l.req, req)
	setFrom_s3ListObjectsInput_s3ListObjectsV2Input(&l.req, req)
	return l
}

// List a bucket with V1 listing
func (ls *v1List) List(ctx context.Context) (resp *s3.ListObjectsV2Output, versionIDs []*string, err error) {
	respv1, err := ls.f.c.ListObjectsWithContext(ctx, &ls.req)
	if err != nil {
		return nil, nil, err
	}

	// Set up the request for next time
	ls.req.Marker = respv1.NextMarker
	if aws.BoolValue(respv1.IsTruncated) && ls.req.Marker == nil {
		if len(respv1.Contents) == 0 {
			return nil, nil, errors.New("s3 protocol error: received listing v1 with IsTruncated set, no NextMarker and no Contents")
		}
		// Use the last Key received if no NextMarker and isTruncated
		ls.req.Marker = respv1.Contents[len(respv1.Contents)-1].Key

	}

	// If we are URL encoding then must decode the marker
	if ls.req.Marker != nil && ls.req.EncodingType != nil {
		*ls.req.Marker, err = url.QueryUnescape(*ls.req.Marker)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to URL decode Marker %q: %w", *ls.req.Marker, err)
		}
	}

	// convert v1 resp into v2 resp
	resp = new(s3.ListObjectsV2Output)
	//structs.SetFrom(resp, respv1)
	setFrom_s3ListObjectsV2Output_s3ListObjectsOutput(resp, respv1)

	return resp, nil, nil
}

// URL Encode the listings
func (ls *v1List) URLEncodeListings(encode bool) {
	if encode {
		ls.req.EncodingType = aws.String(s3.EncodingTypeUrl)
	} else {
		ls.req.EncodingType = nil
	}
}

// V2 bucket lister
type v2List struct {
	f   *Fs
	req s3.ListObjectsV2Input
}

// Create a new V2 bucket lister
func (f *Fs) newV2List(req *s3.ListObjectsV2Input) bucketLister {
	return &v2List{
		f:   f,
		req: *req,
	}
}

// Do a V2 listing
func (ls *v2List) List(ctx context.Context) (resp *s3.ListObjectsV2Output, versionIDs []*string, err error) {
	resp, err = ls.f.c.ListObjectsV2WithContext(ctx, &ls.req)
	if err != nil {
		return nil, nil, err
	}
	if aws.BoolValue(resp.IsTruncated) && (resp.NextContinuationToken == nil || *resp.NextContinuationToken == "") {
		return nil, nil, errors.New("s3 protocol error: received listing v2 with IsTruncated set and no NextContinuationToken. Should you be using `--s3-list-version 1`?")
	}
	ls.req.ContinuationToken = resp.NextContinuationToken
	return resp, nil, nil
}

// URL Encode the listings
func (ls *v2List) URLEncodeListings(encode bool) {
	if encode {
		ls.req.EncodingType = aws.String(s3.EncodingTypeUrl)
	} else {
		ls.req.EncodingType = nil
	}
}

// Versions bucket lister
type versionsList struct {
	f              *Fs
	req            s3.ListObjectVersionsInput
	versionAt      time.Time // set if we want only versions before this
	usingVersionAt bool      // set if we need to use versionAt
	hidden         bool      // set to see hidden versions
	lastKeySent    string    // last Key sent to the receiving function
}

// Create a new Versions bucket lister
func (f *Fs) newVersionsList(req *s3.ListObjectsV2Input, hidden bool, versionAt time.Time) bucketLister {
	l := &versionsList{
		f:              f,
		versionAt:      versionAt,
		usingVersionAt: !versionAt.IsZero(),
		hidden:         hidden,
	}
	// Convert v2 req into withVersions req
	//structs.SetFrom(&l.req, req)
	setFrom_s3ListObjectVersionsInput_s3ListObjectsV2Input(&l.req, req)
	return l
}

// Any s3.Object or s3.ObjectVersion with this as their Size are delete markers
var isDeleteMarker = new(int64)

// Compare two s3.ObjectVersions, sorted alphabetically by key with
// the newest first if the Keys match or the one with IsLatest set if
// everything matches.
func versionLess(a, b *s3.ObjectVersion) bool {
	if a == nil || a.Key == nil || a.LastModified == nil {
		return true
	}
	if b == nil || b.Key == nil || b.LastModified == nil {
		return false
	}
	if *a.Key < *b.Key {
		return true
	}
	if *a.Key > *b.Key {
		return false
	}
	dt := (*a.LastModified).Sub(*b.LastModified)
	if dt > 0 {
		return true
	}
	if dt < 0 {
		return false
	}
	if aws.BoolValue(a.IsLatest) {
		return true
	}
	return false
}

// Merge the DeleteMarkers into the Versions.
//
// These are delivered by S3 sorted by key then by LastUpdated
// newest first but annoyingly the SDK splits them up into two
// so we need to merge them back again
//
// We do this by converting the s3.DeleteEntry into
// s3.ObjectVersion with Size = isDeleteMarker to tell them apart
//
// We then merge them back into the Versions in the correct order
func mergeDeleteMarkers(oldVersions []*s3.ObjectVersion, deleteMarkers []*s3.DeleteMarkerEntry) (newVersions []*s3.ObjectVersion) {
	newVersions = make([]*s3.ObjectVersion, 0, len(oldVersions)+len(deleteMarkers))
	for _, deleteMarker := range deleteMarkers {
		var obj = new(s3.ObjectVersion)
		//structs.SetFrom(obj, deleteMarker)
		setFrom_s3ObjectVersion_s3DeleteMarkerEntry(obj, deleteMarker)
		obj.Size = isDeleteMarker
		for len(oldVersions) > 0 && versionLess(oldVersions[0], obj) {
			newVersions = append(newVersions, oldVersions[0])
			oldVersions = oldVersions[1:]
		}
		newVersions = append(newVersions, obj)
	}
	// Merge any remaining versions
	newVersions = append(newVersions, oldVersions...)
	return newVersions
}

// List a bucket with versions
func (ls *versionsList) List(ctx context.Context) (resp *s3.ListObjectsV2Output, versionIDs []*string, err error) {
	respVersions, err := ls.f.c.ListObjectVersionsWithContext(ctx, &ls.req)
	if err != nil {
		return nil, nil, err
	}

	// Set up the request for next time
	ls.req.KeyMarker = respVersions.NextKeyMarker
	ls.req.VersionIdMarker = respVersions.NextVersionIdMarker
	if aws.BoolValue(respVersions.IsTruncated) && ls.req.KeyMarker == nil {
		return nil, nil, errors.New("s3 protocol error: received versions listing with IsTruncated set with no NextKeyMarker")
	}

	// If we are URL encoding then must decode the marker
	if ls.req.KeyMarker != nil && ls.req.EncodingType != nil {
		*ls.req.KeyMarker, err = url.QueryUnescape(*ls.req.KeyMarker)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to URL decode KeyMarker %q: %w", *ls.req.KeyMarker, err)
		}
	}

	// convert Versions resp into v2 resp
	resp = new(s3.ListObjectsV2Output)
	//structs.SetFrom(resp, respVersions)
	setFrom_s3ListObjectsV2Output_s3ListObjectVersionsOutput(resp, respVersions)

	// Merge in delete Markers as s3.ObjectVersion if we need them
	if ls.hidden || ls.usingVersionAt {
		respVersions.Versions = mergeDeleteMarkers(respVersions.Versions, respVersions.DeleteMarkers)
	}

	// Convert the Versions and the DeleteMarkers into an array of s3.Object
	//
	// These are returned in the order that they are stored with the most recent first.
	// With the annoyance that the Versions and DeleteMarkers are split into two
	objs := make([]*s3.Object, 0, len(respVersions.Versions))
	for _, objVersion := range respVersions.Versions {
		if ls.usingVersionAt {
			if objVersion.LastModified.After(ls.versionAt) {
				// Ignore versions that were created after the specified time
				continue
			}
			if *objVersion.Key == ls.lastKeySent {
				// Ignore versions before the already returned version
				continue
			}
		}
		ls.lastKeySent = *objVersion.Key
		// Don't send delete markers if we don't want hidden things
		if !ls.hidden && objVersion.Size == isDeleteMarker {
			continue
		}
		var obj = new(s3.Object)
		//structs.SetFrom(obj, objVersion)
		setFrom_s3Object_s3ObjectVersion(obj, objVersion)
		// Adjust the file names
		if !ls.usingVersionAt && (!aws.BoolValue(objVersion.IsLatest) || objVersion.Size == isDeleteMarker) {
			if obj.Key != nil && objVersion.LastModified != nil {
				*obj.Key = version.Add(*obj.Key, *objVersion.LastModified)
			}
		}
		objs = append(objs, obj)
		versionIDs = append(versionIDs, objVersion.VersionId)
	}

	resp.Contents = objs
	return resp, versionIDs, nil
}

// URL Encode the listings
func (ls *versionsList) URLEncodeListings(encode bool) {
	if encode {
		ls.req.EncodingType = aws.String(s3.EncodingTypeUrl)
	} else {
		ls.req.EncodingType = nil
	}
}

// listFn is called from list to handle an object.
type listFn func(remote string, object *s3.Object, versionID *string, isDirectory bool) error

// errEndList is a sentinel used to end the list iteration now.
// listFn should return it to end the iteration with no errors.
var errEndList = errors.New("end list")

// list options
type listOpt struct {
	bucket        string  // bucket to list
	directory     string  // directory with bucket
	prefix        string  // prefix to remove from listing
	addBucket     bool    // if set, the bucket is added to the start of the remote
	recurse       bool    // if set, recurse to read sub directories
	withVersions  bool    // if set, versions are produced
	hidden        bool    // if set, return delete markers as objects with size == isDeleteMarker
	findFile      bool    // if set, it will look for files called (bucket, directory)
	versionAt     fs.Time // if set only show versions <= this time
	noSkipMarkers bool    // if set return dir marker objects
	restoreStatus bool    // if set return restore status in listing too
}

// list lists the objects into the function supplied with the opt
// supplied.
func (f *Fs) list(ctx context.Context, opt listOpt, fn listFn) error {
	if opt.prefix != "" {
		opt.prefix += "/"
	}
	if !opt.findFile {
		if opt.directory != "" {
			opt.directory += "/"
		}
	}
	delimiter := ""
	if !opt.recurse {
		delimiter = "/"
	}
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
	req := s3.ListObjectsV2Input{
		Bucket:    &opt.bucket,
		Delimiter: &delimiter,
		Prefix:    &opt.directory,
		MaxKeys:   &f.opt.ListChunk,
	}
	if opt.restoreStatus {
		restoreStatus := "RestoreStatus"
		req.OptionalObjectAttributes = []*string{&restoreStatus}
	}
	if f.opt.RequesterPays {
		req.RequestPayer = aws.String(s3.RequestPayerRequester)
	}
	var listBucket bucketLister
	switch {
	case opt.withVersions || opt.versionAt.IsSet():
		listBucket = f.newVersionsList(&req, opt.hidden, time.Time(opt.versionAt))
	case f.opt.ListVersion == 1:
		listBucket = f.newV1List(&req)
	default:
		listBucket = f.newV2List(&req)
	}
	foundItems := 0
	for {
		var resp *s3.ListObjectsV2Output
		var err error
		var versionIDs []*string
		err = f.pacer.Call(func() (bool, error) {
			listBucket.URLEncodeListings(urlEncodeListings)
			resp, versionIDs, err = listBucket.List(ctx)
			if err != nil && !urlEncodeListings {
				if awsErr, ok := err.(awserr.RequestFailure); ok {
					if origErr := awsErr.OrigErr(); origErr != nil {
						if _, ok := origErr.(*xml.SyntaxError); ok {
							// Retry the listing with URL encoding as there were characters that XML can't encode
							urlEncodeListings = true
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
						fs.Errorf(f, "Can't change region for bucket %q with no bucket specified", opt.bucket)
						return nil
					}
				}
			}
			return err
		}
		if !opt.recurse {
			foundItems += len(resp.CommonPrefixes)
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
				if !strings.HasPrefix(remote, opt.prefix) {
					fs.Logf(f, "Odd name received %q", remote)
					continue
				}
				remote = remote[len(opt.prefix):]
				if opt.addBucket {
					remote = bucket.Join(opt.bucket, remote)
				}
				remote = strings.TrimSuffix(remote, "/")
				err = fn(remote, &s3.Object{Key: &remote}, nil, true)
				if err != nil {
					if err == errEndList {
						return nil
					}
					return err
				}
			}
		}
		foundItems += len(resp.Contents)
		for i, object := range resp.Contents {
			remote := aws.StringValue(object.Key)
			if urlEncodeListings {
				remote, err = url.QueryUnescape(remote)
				if err != nil {
					fs.Logf(f, "failed to URL decode %q in listing: %v", aws.StringValue(object.Key), err)
					continue
				}
			}
			remote = f.opt.Enc.ToStandardPath(remote)
			if !strings.HasPrefix(remote, opt.prefix) {
				fs.Logf(f, "Odd name received %q", remote)
				continue
			}
			isDirectory := (remote == "" || strings.HasSuffix(remote, "/")) && object.Size != nil && *object.Size == 0
			// is this a directory marker?
			if isDirectory {
				if opt.noSkipMarkers {
					// process directory markers as files
					isDirectory = false
				} else {
					// Don't insert the root directory
					if remote == opt.directory {
						continue
					}
				}
			}
			remote = remote[len(opt.prefix):]
			if isDirectory {
				// process directory markers as directories
				remote = strings.TrimRight(remote, "/")
			}
			if opt.addBucket {
				remote = bucket.Join(opt.bucket, remote)
			}
			if versionIDs != nil {
				err = fn(remote, object, versionIDs[i], isDirectory)
			} else {
				err = fn(remote, object, nil, isDirectory)
			}
			if err != nil {
				if err == errEndList {
					return nil
				}
				return err
			}
		}
		if !aws.BoolValue(resp.IsTruncated) {
			break
		}
	}
	if f.opt.DirectoryMarkers && foundItems == 0 && opt.directory != "" {
		// Determine whether the directory exists or not by whether it has a marker
		req := s3.HeadObjectInput{
			Bucket: &opt.bucket,
			Key:    &opt.directory,
		}
		_, err := f.headObject(ctx, &req)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				return fs.ErrorDirNotFound
			}
			return err
		}
	}
	return nil
}

// Convert a list item into a DirEntry
func (f *Fs) itemToDirEntry(ctx context.Context, remote string, object *s3.Object, versionID *string, isDirectory bool) (fs.DirEntry, error) {
	if isDirectory {
		size := int64(0)
		if object.Size != nil {
			size = *object.Size
		}
		d := fs.NewDir(remote, time.Time{}).SetSize(size)
		return d, nil
	}
	o, err := f.newObjectWithInfo(ctx, remote, object, versionID)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// listDir lists files and directories to out
func (f *Fs) listDir(ctx context.Context, bucket, directory, prefix string, addBucket bool) (entries fs.DirEntries, err error) {
	// List the objects and directories
	err = f.list(ctx, listOpt{
		bucket:       bucket,
		directory:    directory,
		prefix:       prefix,
		addBucket:    addBucket,
		withVersions: f.opt.Versions,
		versionAt:    f.opt.VersionAt,
		hidden:       f.opt.VersionDeleted,
	}, func(remote string, object *s3.Object, versionID *string, isDirectory bool) error {
		entry, err := f.itemToDirEntry(ctx, remote, object, versionID, isDirectory)
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
		return f.list(ctx, listOpt{
			bucket:       bucket,
			directory:    directory,
			prefix:       prefix,
			addBucket:    addBucket,
			recurse:      true,
			withVersions: f.opt.Versions,
			versionAt:    f.opt.VersionAt,
			hidden:       f.opt.VersionDeleted,
		}, func(remote string, object *s3.Object, versionID *string, isDirectory bool) error {
			entry, err := f.itemToDirEntry(ctx, remote, object, versionID, isDirectory)
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

// Create directory marker file and parents
func (f *Fs) createDirectoryMarker(ctx context.Context, bucket, dir string) error {
	if !f.opt.DirectoryMarkers || bucket == "" {
		return nil
	}

	// Object to be uploaded
	o := &Object{
		fs: f,
		meta: map[string]string{
			metaMtime: swift.TimeToFloatString(time.Now()),
		},
	}

	for {
		_, bucketPath := f.split(dir)
		// Don't create the directory marker if it is the bucket or at the very root
		if bucketPath == "" {
			break
		}
		o.remote = dir + "/"

		// Check to see if object already exists
		_, err := o.headObject(ctx)
		if err == nil {
			return nil
		}

		// Upload it if not
		fs.Debugf(o, "Creating directory marker")
		content := io.Reader(strings.NewReader(""))
		err = o.Update(ctx, content, o)
		if err != nil {
			return fmt.Errorf("creating directory marker failed: %w", err)
		}

		// Now check parent directory exists
		dir = path.Dir(dir)
		if dir == "/" || dir == "." {
			break
		}
	}

	return nil
}

// Mkdir creates the bucket if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	bucket, _ := f.split(dir)
	e := f.makeBucket(ctx, bucket)
	if e != nil {
		return e
	}
	return f.createDirectoryMarker(ctx, bucket, dir)
}

// mkdirParent creates the parent bucket/directory if it doesn't exist
func (f *Fs) mkdirParent(ctx context.Context, remote string) error {
	remote = strings.TrimRight(remote, "/")
	dir := path.Dir(remote)
	if dir == "/" || dir == "." {
		dir = ""
	}
	return f.Mkdir(ctx, dir)
}

// makeBucket creates the bucket if it doesn't exist
func (f *Fs) makeBucket(ctx context.Context, bucket string) error {
	if f.opt.NoCheckBucket {
		return nil
	}
	return f.cache.Create(bucket, func() error {
		req := s3.CreateBucketInput{
			Bucket: &bucket,
			ACL:    stringPointerOrNil(f.opt.BucketACL),
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
			switch awsErr.Code() {
			case "BucketAlreadyOwnedByYou":
				err = nil
			case "BucketAlreadyExists", "BucketNameUnavailable":
				if f.opt.UseAlreadyExists.Value {
					// We can trust BucketAlreadyExists to mean not owned by us, so make it non retriable
					err = fserrors.NoRetryError(err)
				} else {
					// We can't trust BucketAlreadyExists to mean not owned by us, so ignore it
					err = nil
				}
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
	// Remove directory marker file
	if f.opt.DirectoryMarkers && bucket != "" && dir != "" {
		o := &Object{
			fs:     f,
			remote: dir + "/",
		}
		fs.Debugf(o, "Removing directory marker")
		err := o.Remove(ctx)
		if err != nil {
			return fmt.Errorf("removing directory marker failed: %w", err)
		}
	}
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
	req.ACL = stringPointerOrNil(f.opt.ACL)
	req.Key = &dstPath
	source := pathEscape(bucket.Join(srcBucket, srcPath))
	if src.versionID != nil {
		source += fmt.Sprintf("?versionId=%s", *src.versionID)
	}
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
	//structs.SetFrom(req, info)
	setFrom_s3CreateMultipartUploadInput_s3HeadObjectOutput(req, info)

	// If copy metadata was set then set the Metadata to that read
	// from the head request
	if aws.StringValue(copyReq.MetadataDirective) == s3.MetadataDirectiveCopy {
		copyReq.Metadata = info.Metadata
	}

	// Overwrite any from the copyReq
	//structs.SetFrom(req, copyReq)
	setFrom_s3CreateMultipartUploadInput_s3CopyObjectInput(req, copyReq)

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

	var (
		parts   = make([]*s3.CompletedPart, numParts)
		g, gCtx = errgroup.WithContext(ctx)
	)
	g.SetLimit(f.opt.UploadConcurrency)
	for partNum := int64(1); partNum <= numParts; partNum++ {
		// Fail fast, in case an errgroup managed function returns an error
		// gCtx is cancelled. There is no point in uploading all the other parts.
		if gCtx.Err() != nil {
			break
		}
		partNum := partNum // for closure
		g.Go(func() error {
			var uout *s3.UploadPartCopyOutput
			uploadPartReq := &s3.UploadPartCopyInput{}
			//structs.SetFrom(uploadPartReq, copyReq)
			setFrom_s3UploadPartCopyInput_s3CopyObjectInput(uploadPartReq, copyReq)
			uploadPartReq.Bucket = &dstBucket
			uploadPartReq.Key = &dstPath
			uploadPartReq.PartNumber = &partNum
			uploadPartReq.UploadId = uid
			uploadPartReq.CopySourceRange = aws.String(calculateRange(partSize, partNum-1, numParts, srcSize))
			err := f.pacer.Call(func() (bool, error) {
				uout, err = f.c.UploadPartCopyWithContext(gCtx, uploadPartReq)
				return f.shouldRetry(gCtx, err)
			})
			if err != nil {
				return err
			}
			parts[partNum-1] = &s3.CompletedPart{
				PartNumber: &partNum,
				ETag:       uout.CopyPartResult.ETag,
			}
			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		return err
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
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	if f.opt.VersionAt.IsSet() {
		return nil, errNotWithVersionAt
	}
	dstBucket, dstPath := f.split(remote)
	err := f.mkdirParent(ctx, remote)
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

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (link string, err error) {
	if strings.HasSuffix(remote, "/") {
		return "", fs.ErrorCantShareDirectories
	}
	obj, err := f.NewObject(ctx, remote)
	if err != nil {
		return "", err
	}
	o := obj.(*Object)
	if expire > maxExpireDuration {
		fs.Logf(f, "Public Link: Reducing expiry to %v as %v is greater than the max time allowed", maxExpireDuration, expire)
		expire = maxExpireDuration
	}
	bucket, bucketPath := o.split()
	httpReq, _ := f.c.GetObjectRequest(&s3.GetObjectInput{
		Bucket:    &bucket,
		Key:       &bucketPath,
		VersionId: o.versionID,
	})

	return httpReq.Presign(time.Duration(expire))
}

var commandHelp = []fs.CommandHelp{{
	Name:  "restore",
	Short: "Restore objects from GLACIER to normal storage",
	Long: `This command can be used to restore one or more objects from GLACIER
to normal storage.

Usage Examples:

    rclone backend restore s3:bucket/path/to/object -o priority=PRIORITY -o lifetime=DAYS
    rclone backend restore s3:bucket/path/to/directory -o priority=PRIORITY -o lifetime=DAYS
    rclone backend restore s3:bucket -o priority=PRIORITY -o lifetime=DAYS

This flag also obeys the filters. Test first with --interactive/-i or --dry-run flags

    rclone --interactive backend restore --include "*.txt" s3:bucket/path -o priority=Standard -o lifetime=1

All the objects shown will be marked for restore, then

    rclone backend restore --include "*.txt" s3:bucket/path -o priority=Standard -o lifetime=1

It returns a list of status dictionaries with Remote and Status
keys. The Status will be OK if it was successful or an error message
if not.

    [
        {
            "Status": "OK",
            "Remote": "test.txt"
        },
        {
            "Status": "OK",
            "Remote": "test/file4.txt"
        }
    ]

`,
	Opts: map[string]string{
		"priority":    "Priority of restore: Standard|Expedited|Bulk",
		"lifetime":    "Lifetime of the active copy in days",
		"description": "The optional description for the job.",
	},
}, {
	Name:  "restore-status",
	Short: "Show the restore status for objects being restored from GLACIER to normal storage",
	Long: `This command can be used to show the status for objects being restored from GLACIER
to normal storage.

Usage Examples:

    rclone backend restore-status s3:bucket/path/to/object
    rclone backend restore-status s3:bucket/path/to/directory
    rclone backend restore-status -o all s3:bucket/path/to/directory

This command does not obey the filters.

It returns a list of status dictionaries.

    [
        {
            "Remote": "file.txt",
            "VersionID": null,
            "RestoreStatus": {
                "IsRestoreInProgress": true,
                "RestoreExpiryDate": "2023-09-06T12:29:19+01:00"
            },
            "StorageClass": "GLACIER"
        },
        {
            "Remote": "test.pdf",
            "VersionID": null,
            "RestoreStatus": {
                "IsRestoreInProgress": false,
                "RestoreExpiryDate": "2023-09-06T12:29:19+01:00"
            },
            "StorageClass": "DEEP_ARCHIVE"
        }
    ]
`,
	Opts: map[string]string{
		"all": "if set then show all objects, not just ones with restore status",
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

Note that you can use --interactive/-i or --dry-run with this command to see what
it would do.

    rclone backend cleanup s3:bucket/path/to/object
    rclone backend cleanup -o max-age=7w s3:bucket/path/to/object

Durations are parsed as per the rest of rclone, 2h, 7d, 7w etc.
`,
	Opts: map[string]string{
		"max-age": "Max age of upload to delete",
	},
}, {
	Name:  "cleanup-hidden",
	Short: "Remove old versions of files.",
	Long: `This command removes any old hidden versions of files
on a versions enabled bucket.

Note that you can use --interactive/-i or --dry-run with this command to see what
it would do.

    rclone backend cleanup-hidden s3:bucket/path/to/dir
`,
}, {
	Name:  "versioning",
	Short: "Set/get versioning support for a bucket.",
	Long: `This command sets versioning support if a parameter is
passed and then returns the current versioning status for the bucket
supplied.

    rclone backend versioning s3:bucket # read status only
    rclone backend versioning s3:bucket Enabled
    rclone backend versioning s3:bucket Suspended

It may return "Enabled", "Suspended" or "Unversioned". Note that once versioning
has been enabled the status can't be set back to "Unversioned".
`,
}, {
	Name:  "set",
	Short: "Set command for updating the config parameters.",
	Long: `This set command can be used to update the config parameters
for a running s3 backend.

Usage Examples:

    rclone backend set s3: [-o opt_name=opt_value] [-o opt_name2=opt_value2]
    rclone rc backend/command command=set fs=s3: [-o opt_name=opt_value] [-o opt_name2=opt_value2]
    rclone rc backend/command command=set fs=s3: -o session_token=X -o access_key_id=X -o secret_access_key=X

The option keys are named as they are in the config file.

This rebuilds the connection to the s3 backend when it is called with
the new parameters. Only new parameters need be passed as the values
will default to those currently in use.

It doesn't return anything.
`,
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
			if o.storageClass == nil || (*o.storageClass != "GLACIER" && *o.storageClass != "DEEP_ARCHIVE") {
				st.Status = "Not GLACIER or DEEP_ARCHIVE storage class"
				return
			}
			bucket, bucketPath := o.split()
			reqCopy := req
			reqCopy.Bucket = &bucket
			reqCopy.Key = &bucketPath
			reqCopy.VersionId = o.versionID
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
	case "restore-status":
		_, all := opt["all"]
		return f.restoreStatus(ctx, all)
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
	case "cleanup-hidden":
		return nil, f.CleanUpHidden(ctx)
	case "versioning":
		return f.setGetVersioning(ctx, arg...)
	case "set":
		newOpt := f.opt
		err := configstruct.Set(configmap.Simple(opt), &newOpt)
		if err != nil {
			return nil, fmt.Errorf("reading config: %w", err)
		}
		c, ses, err := s3Connection(f.ctx, &newOpt, f.srv)
		if err != nil {
			return nil, fmt.Errorf("updating session: %w", err)
		}
		f.c = c
		f.ses = ses
		f.opt = newOpt
		keys := []string{}
		for k := range opt {
			keys = append(keys, k)
		}
		fs.Logf(f, "Updated config values: %s", strings.Join(keys, ", "))
		return nil, nil
	default:
		return nil, fs.ErrorCommandNotFound
	}
}

// Returned from "restore-status"
type restoreStatusOut struct {
	Remote        string
	VersionID     *string
	RestoreStatus *s3.RestoreStatus
	StorageClass  *string
}

// Recursively enumerate the current fs to find objects with a restore status
func (f *Fs) restoreStatus(ctx context.Context, all bool) (out []restoreStatusOut, err error) {
	fs.Debugf(f, "all = %v", all)
	bucket, directory := f.split("")
	out = []restoreStatusOut{}
	err = f.list(ctx, listOpt{
		bucket:        bucket,
		directory:     directory,
		prefix:        f.rootDirectory,
		addBucket:     f.rootBucket == "",
		recurse:       true,
		withVersions:  f.opt.Versions,
		versionAt:     f.opt.VersionAt,
		hidden:        f.opt.VersionDeleted,
		restoreStatus: true,
	}, func(remote string, object *s3.Object, versionID *string, isDirectory bool) error {
		entry, err := f.itemToDirEntry(ctx, remote, object, versionID, isDirectory)
		if err != nil {
			return err
		}
		if entry != nil {
			if o, ok := entry.(*Object); ok && (all || object.RestoreStatus != nil) {
				out = append(out, restoreStatusOut{
					Remote:        o.remote,
					VersionID:     o.versionID,
					RestoreStatus: object.RestoreStatus,
					StorageClass:  object.StorageClass,
				})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// bucket must be present if listing succeeded
	f.cache.MarkOK(bucket)
	return out, nil
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

// Read whether the bucket is versioned or not
func (f *Fs) isVersioned(ctx context.Context) bool {
	f.versioningMu.Lock()
	defer f.versioningMu.Unlock()
	if !f.versioning.Valid {
		_, _ = f.setGetVersioning(ctx)
		fs.Debugf(f, "bucket is versioned: %v", f.versioning.Value)
	}
	return f.versioning.Value
}

// Set or get bucket versioning.
//
// Pass no arguments to get, or pass "Enabled" or "Suspended"
//
// Updates f.versioning
func (f *Fs) setGetVersioning(ctx context.Context, arg ...string) (status string, err error) {
	if len(arg) > 1 {
		return "", errors.New("too many arguments")
	}
	if f.rootBucket == "" {
		return "", errors.New("need a bucket")
	}
	if len(arg) == 1 {
		var versioning = s3.VersioningConfiguration{
			Status: aws.String(arg[0]),
		}
		// Disabled is indicated by the parameter missing
		if *versioning.Status == "Disabled" {
			versioning.Status = aws.String("")
		}
		req := s3.PutBucketVersioningInput{
			Bucket:                  &f.rootBucket,
			VersioningConfiguration: &versioning,
		}
		err := f.pacer.Call(func() (bool, error) {
			_, err = f.c.PutBucketVersioningWithContext(ctx, &req)
			return f.shouldRetry(ctx, err)
		})
		if err != nil {
			return "", err
		}
	}
	req := s3.GetBucketVersioningInput{
		Bucket: &f.rootBucket,
	}
	var resp *s3.GetBucketVersioningOutput
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.c.GetBucketVersioningWithContext(ctx, &req)
		return f.shouldRetry(ctx, err)
	})
	f.versioning.Valid = true
	f.versioning.Value = false
	if err != nil {
		fs.Errorf(f, "Failed to read versioning status, assuming unversioned: %v", err)
		return "", err
	}
	if resp.Status == nil {
		return "Unversioned", err
	}
	f.versioning.Value = true
	return *resp.Status, err
}

// CleanUp removes all pending multipart uploads older than 24 hours
func (f *Fs) CleanUp(ctx context.Context) (err error) {
	return f.cleanUp(ctx, 24*time.Hour)
}

// purge deletes all the files and directories
//
// if oldOnly is true then it deletes only non current files.
//
// Implemented here so we can make sure we delete old versions.
func (f *Fs) purge(ctx context.Context, dir string, oldOnly bool) error {
	if f.opt.VersionAt.IsSet() {
		return errNotWithVersionAt
	}
	bucket, directory := f.split(dir)
	if bucket == "" {
		return errors.New("can't purge from root")
	}
	versioned := f.isVersioned(ctx)
	if !versioned && oldOnly {
		fs.Infof(f, "bucket is not versioned so not removing old versions")
		return nil
	}
	var errReturn error
	var checkErrMutex sync.Mutex
	var checkErr = func(err error) {
		if err == nil {
			return
		}
		checkErrMutex.Lock()
		defer checkErrMutex.Unlock()
		if errReturn == nil {
			errReturn = err
		}
	}

	// Delete Config.Transfers in parallel
	delChan := make(fs.ObjectsChan, f.ci.Transfers)
	delErr := make(chan error, 1)
	go func() {
		delErr <- operations.DeleteFiles(ctx, delChan)
	}()
	checkErr(f.list(ctx, listOpt{
		bucket:        bucket,
		directory:     directory,
		prefix:        f.rootDirectory,
		addBucket:     f.rootBucket == "",
		recurse:       true,
		withVersions:  versioned,
		hidden:        true,
		noSkipMarkers: true,
	}, func(remote string, object *s3.Object, versionID *string, isDirectory bool) error {
		if isDirectory {
			return nil
		}
		// If the root is a dirmarker it will have lost its trailing /
		if remote == "" {
			remote = "/"
		}
		oi, err := f.newObjectWithInfo(ctx, remote, object, versionID)
		if err != nil {
			fs.Errorf(object, "Can't create object %+v", err)
			return nil
		}
		tr := accounting.Stats(ctx).NewCheckingTransfer(oi, "checking")
		// Work out whether the file is the current version or not
		isCurrentVersion := !versioned || !version.Match(remote)
		fs.Debugf(nil, "%q version %v", remote, version.Match(remote))
		if oldOnly && isCurrentVersion {
			// Check current version of the file
			if object.Size == isDeleteMarker {
				fs.Debugf(remote, "Deleting current version (id %q) as it is a delete marker", aws.StringValue(versionID))
				delChan <- oi
			} else {
				fs.Debugf(remote, "Not deleting current version %q", aws.StringValue(versionID))
			}
		} else {
			if object.Size == isDeleteMarker {
				fs.Debugf(remote, "Deleting delete marker (id %q)", aws.StringValue(versionID))
			} else {
				fs.Debugf(remote, "Deleting (id %q)", aws.StringValue(versionID))
			}
			delChan <- oi
		}
		tr.Done(ctx, nil)
		return nil
	}))
	close(delChan)
	checkErr(<-delErr)

	if !oldOnly {
		checkErr(f.Rmdir(ctx, dir))
	}
	return errReturn
}

// Purge deletes all the files and directories including the old versions.
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purge(ctx, dir, false)
}

// CleanUpHidden deletes all the hidden files.
func (f *Fs) CleanUpHidden(ctx context.Context) error {
	return f.purge(ctx, "", true)
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
	// If decompressing, erase the hash
	if o.bytes < 0 {
		return "", nil
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
		Bucket:    &bucket,
		Key:       &bucketPath,
		VersionId: o.versionID,
	}
	return o.fs.headObject(ctx, &req)
}

func (f *Fs) headObject(ctx context.Context, req *s3.HeadObjectInput) (resp *s3.HeadObjectOutput, err error) {
	if f.opt.RequesterPays {
		req.RequestPayer = aws.String(s3.RequestPayerRequester)
	}
	if f.opt.SSECustomerAlgorithm != "" {
		req.SSECustomerAlgorithm = &f.opt.SSECustomerAlgorithm
	}
	if f.opt.SSECustomerKey != "" {
		req.SSECustomerKey = &f.opt.SSECustomerKey
	}
	if f.opt.SSECustomerKeyMD5 != "" {
		req.SSECustomerKeyMD5 = &f.opt.SSECustomerKeyMD5
	}
	err = f.pacer.Call(func() (bool, error) {
		var err error
		resp, err = f.c.HeadObjectWithContext(ctx, req)
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		if awsErr, ok := err.(awserr.RequestFailure); ok {
			if awsErr.StatusCode() == http.StatusNotFound {
				return nil, fs.ErrorObjectNotFound
			}
		}
		return nil, err
	}
	if req.Bucket != nil {
		f.cache.MarkOK(*req.Bucket)
	}
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
	o.setMetaData(resp)
	// resp.ETag, resp.ContentLength, resp.LastModified, resp.Metadata, resp.ContentType, resp.StorageClass)
	return nil
}

// Convert S3 metadata with pointers into a map[string]string
// while lowercasing the keys
func s3MetadataToMap(s3Meta map[string]*string) map[string]string {
	meta := make(map[string]string, len(s3Meta))
	for k, v := range s3Meta {
		if v != nil {
			meta[strings.ToLower(k)] = *v
		}
	}
	return meta
}

// Convert our metadata back into S3 metadata
func mapToS3Metadata(meta map[string]string) map[string]*string {
	s3Meta := make(map[string]*string, len(meta))
	for k, v := range meta {
		s3Meta[k] = aws.String(v)
	}
	return s3Meta
}

func (o *Object) setMetaData(resp *s3.HeadObjectOutput) {
	// Ignore missing Content-Length assuming it is 0
	// Some versions of ceph do this due their apache proxies
	if resp.ContentLength != nil {
		o.bytes = *resp.ContentLength
	}
	o.setMD5FromEtag(aws.StringValue(resp.ETag))
	o.meta = s3MetadataToMap(resp.Metadata)
	// Read MD5 from metadata if present
	if md5sumBase64, ok := o.meta[metaMD5Hash]; ok {
		md5sumBytes, err := base64.StdEncoding.DecodeString(md5sumBase64)
		if err != nil {
			fs.Debugf(o, "Failed to read md5sum from metadata %q: %v", md5sumBase64, err)
		} else if len(md5sumBytes) != 16 {
			fs.Debugf(o, "Failed to read md5sum from metadata %q: wrong length", md5sumBase64)
		} else {
			o.md5 = hex.EncodeToString(md5sumBytes)
		}
	}
	if resp.LastModified == nil {
		o.lastModified = time.Now()
		fs.Logf(o, "Failed to read last modified")
	} else {
		// Try to keep the maximum precision in lastModified. If we read
		// it from listings then it may have millisecond precision, but
		// if we read it from a HEAD/GET request then it will have
		// second precision.
		equalToWithinOneSecond := o.lastModified.Truncate(time.Second).Equal((*resp.LastModified).Truncate(time.Second))
		newHasNs := (*resp.LastModified).Nanosecond() != 0
		if !equalToWithinOneSecond || newHasNs {
			o.lastModified = *resp.LastModified
		}
	}
	o.mimeType = aws.StringValue(resp.ContentType)

	// Set system metadata
	o.storageClass = resp.StorageClass
	o.cacheControl = resp.CacheControl
	o.contentDisposition = resp.ContentDisposition
	o.contentEncoding = resp.ContentEncoding
	o.contentLanguage = resp.ContentLanguage

	// If decompressing then size and md5sum are unknown
	if o.fs.opt.Decompress && aws.StringValue(o.contentEncoding) == "gzip" {
		o.bytes = -1
		o.md5 = ""
	}
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
	if !ok {
		// fs.Debugf(o, "No metadata")
		return o.lastModified
	}
	modTime, err := swift.FloatStringToTime(d)
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
	o.meta[metaMtime] = swift.TimeToFloatString(modTime)

	// Can't update metadata here, so return this error to force a recopy
	if o.storageClass != nil && (*o.storageClass == "GLACIER" || *o.storageClass == "DEEP_ARCHIVE") {
		return fs.ErrorCantSetModTime
	}

	// Copy the object to itself to update the metadata
	bucket, bucketPath := o.split()
	req := s3.CopyObjectInput{
		ContentType:       aws.String(fs.MimeType(ctx, o)), // Guess the content type
		Metadata:          mapToS3Metadata(o.meta),
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

	contentLength := rest.ParseSizeFromHeaders(resp.Header)
	if contentLength < 0 {
		fs.Debugf(o, "Failed to parse file size from headers")
	}

	lastModified, err := http.ParseTime(resp.Header.Get("Last-Modified"))
	if err != nil {
		fs.Debugf(o, "Failed to parse last modified from string %s, %v", resp.Header.Get("Last-Modified"), err)
	}

	metaData := make(map[string]*string)
	for key, value := range resp.Header {
		key = strings.ToLower(key)
		if strings.HasPrefix(key, "x-amz-meta-") {
			metaKey := strings.TrimPrefix(key, "x-amz-meta-")
			metaData[metaKey] = &value[0]
		}
	}

	header := func(k string) *string {
		v := resp.Header.Get(k)
		if v == "" {
			return nil
		}
		return &v
	}

	var head = s3.HeadObjectOutput{
		ETag:               header("Etag"),
		ContentLength:      &contentLength,
		LastModified:       &lastModified,
		Metadata:           metaData,
		CacheControl:       header("Cache-Control"),
		ContentDisposition: header("Content-Disposition"),
		ContentEncoding:    header("Content-Encoding"),
		ContentLanguage:    header("Content-Language"),
		ContentType:        header("Content-Type"),
		StorageClass:       header("X-Amz-Storage-Class"),
	}
	o.setMetaData(&head)
	return resp.Body, err
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	bucket, bucketPath := o.split()

	if o.fs.opt.DownloadURL != "" {
		return o.downloadFromURL(ctx, bucketPath, options...)
	}

	req := s3.GetObjectInput{
		Bucket:    &bucket,
		Key:       &bucketPath,
		VersionId: o.versionID,
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

	// Override the automatic decompression in the transport to
	// download compressed files as-is
	if o.fs.opt.UseAcceptEncodingGzip.Value {
		httpReq.HTTPRequest.Header.Set("Accept-Encoding", "gzip")
	}

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
	var head s3.HeadObjectOutput
	//structs.SetFrom(&head, resp)
	setFrom_s3HeadObjectOutput_s3GetObjectOutput(&head, resp)
	head.ContentLength = size
	o.setMetaData(&head)

	// Decompress body if necessary
	if aws.StringValue(resp.ContentEncoding) == "gzip" {
		if o.fs.opt.Decompress || (resp.ContentLength == nil && o.fs.opt.MightGzip.Value) {
			return readers.NewGzipReader(resp.Body)
		}
		o.fs.warnCompressed.Do(func() {
			fs.Logf(o, "Not decompressing 'Content-Encoding: gzip' compressed file. Use --s3-decompress to override")
		})
	}

	return resp.Body, nil
}

var warnStreamUpload sync.Once

// state of ChunkWriter
type s3ChunkWriter struct {
	chunkSize            int64
	size                 int64
	f                    *Fs
	bucket               *string
	key                  *string
	uploadID             *string
	multiPartUploadInput *s3.CreateMultipartUploadInput
	completedPartsMu     sync.Mutex
	completedParts       []*s3.CompletedPart
	eTag                 string
	versionID            string
	md5sMu               sync.Mutex
	md5s                 []byte
	ui                   uploadInfo
	o                    *Object
}

// OpenChunkWriter returns the chunk size and a ChunkWriter
//
// Pass in the remote and the src object
// You can also use options to hint at the desired chunk size
func (f *Fs) OpenChunkWriter(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (info fs.ChunkWriterInfo, writer fs.ChunkWriter, err error) {
	// Temporary Object under construction
	o := &Object{
		fs:     f,
		remote: remote,
	}
	ui, err := o.prepareUpload(ctx, src, options)
	if err != nil {
		return info, nil, fmt.Errorf("failed to prepare upload: %w", err)
	}

	//structs.SetFrom(&mReq, req)
	var mReq s3.CreateMultipartUploadInput
	setFrom_s3CreateMultipartUploadInput_s3PutObjectInput(&mReq, ui.req)

	uploadParts := f.opt.MaxUploadParts
	if uploadParts < 1 {
		uploadParts = 1
	} else if uploadParts > maxUploadParts {
		uploadParts = maxUploadParts
	}
	size := src.Size()

	// calculate size of parts
	chunkSize := f.opt.ChunkSize

	// size can be -1 here meaning we don't know the size of the incoming file. We use ChunkSize
	// buffers here (default 5 MiB). With a maximum number of parts (10,000) this will be a file of
	// 48 GiB which seems like a not too unreasonable limit.
	if size == -1 {
		warnStreamUpload.Do(func() {
			fs.Logf(f, "Streaming uploads using chunk size %v will have maximum file size of %v",
				f.opt.ChunkSize, fs.SizeSuffix(int64(chunkSize)*int64(uploadParts)))
		})
	} else {
		chunkSize = chunksize.Calculator(src, size, uploadParts, chunkSize)
	}

	var mOut *s3.CreateMultipartUploadOutput
	err = f.pacer.Call(func() (bool, error) {
		mOut, err = f.c.CreateMultipartUploadWithContext(ctx, &mReq)
		if err == nil {
			if mOut == nil {
				err = fserrors.RetryErrorf("internal error: no info from multipart upload")
			} else if mOut.UploadId == nil {
				err = fserrors.RetryErrorf("internal error: no UploadId in multpart upload: %#v", *mOut)
			}
		}
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return info, nil, fmt.Errorf("create multipart upload failed: %w", err)
	}

	chunkWriter := &s3ChunkWriter{
		chunkSize:            int64(chunkSize),
		size:                 size,
		f:                    f,
		bucket:               mOut.Bucket,
		key:                  mOut.Key,
		uploadID:             mOut.UploadId,
		multiPartUploadInput: &mReq,
		completedParts:       make([]*s3.CompletedPart, 0),
		ui:                   ui,
		o:                    o,
	}
	info = fs.ChunkWriterInfo{
		ChunkSize:         int64(chunkSize),
		Concurrency:       o.fs.opt.UploadConcurrency,
		LeavePartsOnError: o.fs.opt.LeavePartsOnError,
	}
	fs.Debugf(o, "open chunk writer: started multipart upload: %v", *mOut.UploadId)
	return info, chunkWriter, err
}

// add a part number and etag to the completed parts
func (w *s3ChunkWriter) addCompletedPart(partNum *int64, eTag *string) {
	w.completedPartsMu.Lock()
	defer w.completedPartsMu.Unlock()
	w.completedParts = append(w.completedParts, &s3.CompletedPart{
		PartNumber: partNum,
		ETag:       eTag,
	})
}

// addMd5 adds a binary md5 to the md5 calculated so far
func (w *s3ChunkWriter) addMd5(md5binary *[]byte, chunkNumber int64) {
	w.md5sMu.Lock()
	defer w.md5sMu.Unlock()
	start := chunkNumber * md5.Size
	end := start + md5.Size
	if extend := end - int64(len(w.md5s)); extend > 0 {
		w.md5s = append(w.md5s, make([]byte, extend)...)
	}
	copy(w.md5s[start:end], (*md5binary)[:])
}

// WriteChunk will write chunk number with reader bytes, where chunk number >= 0
func (w *s3ChunkWriter) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (int64, error) {
	if chunkNumber < 0 {
		err := fmt.Errorf("invalid chunk number provided: %v", chunkNumber)
		return -1, err
	}
	// Only account after the checksum reads have been done
	if do, ok := reader.(pool.DelayAccountinger); ok {
		// To figure out this number, do a transfer and if the accounted size is 0 or a
		// multiple of what it should be, increase or decrease this number.
		do.DelayAccounting(3)
	}

	// create checksum of buffer for integrity checking
	// currently there is no way to calculate the md5 without reading the chunk a 2nd time (1st read is in uploadMultipart)
	// possible in AWS SDK v2 with trailers?
	m := md5.New()
	currentChunkSize, err := io.Copy(m, reader)
	if err != nil {
		return -1, err
	}
	// If no data read and not the first chunk, don't write the chunk
	if currentChunkSize == 0 && chunkNumber != 0 {
		return 0, nil
	}
	md5sumBinary := m.Sum([]byte{})
	w.addMd5(&md5sumBinary, int64(chunkNumber))
	md5sum := base64.StdEncoding.EncodeToString(md5sumBinary[:])

	// S3 requires 1 <= PartNumber <= 10000
	s3PartNumber := aws.Int64(int64(chunkNumber + 1))
	uploadPartReq := &s3.UploadPartInput{
		Body:                 reader,
		Bucket:               w.bucket,
		Key:                  w.key,
		PartNumber:           s3PartNumber,
		UploadId:             w.uploadID,
		ContentMD5:           &md5sum,
		ContentLength:        aws.Int64(currentChunkSize),
		RequestPayer:         w.multiPartUploadInput.RequestPayer,
		SSECustomerAlgorithm: w.multiPartUploadInput.SSECustomerAlgorithm,
		SSECustomerKey:       w.multiPartUploadInput.SSECustomerKey,
		SSECustomerKeyMD5:    w.multiPartUploadInput.SSECustomerKeyMD5,
	}
	var uout *s3.UploadPartOutput
	err = w.f.pacer.Call(func() (bool, error) {
		// rewind the reader on retry and after reading md5
		_, err = reader.Seek(0, io.SeekStart)
		if err != nil {
			return false, err
		}
		uout, err = w.f.c.UploadPartWithContext(ctx, uploadPartReq)
		if err != nil {
			if chunkNumber <= 8 {
				return w.f.shouldRetry(ctx, err)
			}
			// retry all chunks once have done the first few
			return true, err
		}
		return false, nil
	})
	if err != nil {
		return -1, fmt.Errorf("failed to upload chunk %d with %v bytes: %w", chunkNumber+1, currentChunkSize, err)
	}

	w.addCompletedPart(s3PartNumber, uout.ETag)

	fs.Debugf(w.o, "multipart upload wrote chunk %d with %v bytes and etag %v", chunkNumber+1, currentChunkSize, *uout.ETag)
	return currentChunkSize, err
}

// Abort the multipart upload
func (w *s3ChunkWriter) Abort(ctx context.Context) error {
	err := w.f.pacer.Call(func() (bool, error) {
		_, err := w.f.c.AbortMultipartUploadWithContext(context.Background(), &s3.AbortMultipartUploadInput{
			Bucket:       w.bucket,
			Key:          w.key,
			UploadId:     w.uploadID,
			RequestPayer: w.multiPartUploadInput.RequestPayer,
		})
		return w.f.shouldRetry(ctx, err)
	})
	if err != nil {
		return fmt.Errorf("failed to abort multipart upload %q: %w", *w.uploadID, err)
	}
	fs.Debugf(w.o, "multipart upload %q aborted", *w.uploadID)
	return err
}

// Close and finalise the multipart upload
func (w *s3ChunkWriter) Close(ctx context.Context) (err error) {
	// sort the completed parts by part number
	sort.Slice(w.completedParts, func(i, j int) bool {
		return *w.completedParts[i].PartNumber < *w.completedParts[j].PartNumber
	})
	var resp *s3.CompleteMultipartUploadOutput
	err = w.f.pacer.Call(func() (bool, error) {
		resp, err = w.f.c.CompleteMultipartUploadWithContext(ctx, &s3.CompleteMultipartUploadInput{
			Bucket: w.bucket,
			Key:    w.key,
			MultipartUpload: &s3.CompletedMultipartUpload{
				Parts: w.completedParts,
			},
			RequestPayer: w.multiPartUploadInput.RequestPayer,
			UploadId:     w.uploadID,
		})
		return w.f.shouldRetry(ctx, err)
	})
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload %q: %w", *w.uploadID, err)
	}
	if resp != nil {
		if resp.ETag != nil {
			w.eTag = *resp.ETag
		}
		if resp.VersionId != nil {
			w.versionID = *resp.VersionId
		}
	}
	fs.Debugf(w.o, "multipart upload %q finished", *w.uploadID)
	return err
}

func (o *Object) uploadMultipart(ctx context.Context, src fs.ObjectInfo, in io.Reader, options ...fs.OpenOption) (wantETag, gotETag string, versionID *string, ui uploadInfo, err error) {
	chunkWriter, err := multipart.UploadMultipart(ctx, src, in, multipart.UploadMultipartOptions{
		Open:        o.fs,
		OpenOptions: options,
	})
	if err != nil {
		return wantETag, gotETag, versionID, ui, err
	}

	var s3cw *s3ChunkWriter = chunkWriter.(*s3ChunkWriter)
	gotETag = s3cw.eTag
	versionID = aws.String(s3cw.versionID)

	hashOfHashes := md5.Sum(s3cw.md5s)
	wantETag = fmt.Sprintf("%s-%d", hex.EncodeToString(hashOfHashes[:]), len(s3cw.completedParts))

	return wantETag, gotETag, versionID, s3cw.ui, nil
}

// unWrapAwsError unwraps AWS errors, looking for a non AWS error
//
// It returns true if one was found and the error, or false and the
// error passed in.
func unWrapAwsError(err error) (found bool, outErr error) {
	if awsErr, ok := err.(awserr.Error); ok {
		var origErrs []error
		if batchErr, ok := awsErr.(awserr.BatchedErrors); ok {
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
func (o *Object) uploadSinglepartPutObject(ctx context.Context, req *s3.PutObjectInput, size int64, in io.Reader) (etag string, lastModified time.Time, versionID *string, err error) {
	r, resp := o.fs.c.PutObjectRequest(req)
	if req.ContentLength != nil && *req.ContentLength == 0 {
		// Can't upload zero length files like this for some reason
		r.Body = bytes.NewReader([]byte{})
	} else {
		r.SetStreamingBody(io.NopCloser(in))
	}
	r.SetContext(ctx)
	r.HTTPRequest.Header.Set("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")

	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		err := r.Send()
		return o.fs.shouldRetry(ctx, err)
	})
	if err != nil {
		// Return the underlying error if we have a
		// Serialization or RequestError error if possible
		//
		// These errors are synthesized locally in the SDK
		// (not returned from the server) and we'd rather have
		// the underlying error if there is one.
		if do, ok := err.(awserr.Error); ok && (do.Code() == request.ErrCodeSerialization || do.Code() == request.ErrCodeRequestError) {
			if found, newErr := unWrapAwsError(err); found {
				err = newErr
			}
		}
		return etag, lastModified, nil, err
	}
	lastModified = time.Now()
	if resp != nil {
		etag = aws.StringValue(resp.ETag)
		versionID = resp.VersionId
	}
	return etag, lastModified, versionID, nil
}

// Upload a single part using a presigned request
func (o *Object) uploadSinglepartPresignedRequest(ctx context.Context, req *s3.PutObjectInput, size int64, in io.Reader) (etag string, lastModified time.Time, versionID *string, err error) {
	// Create the request
	putObj, _ := o.fs.c.PutObjectRequest(req)

	// Sign it so we can upload using a presigned request.
	//
	// Note the SDK didn't used to support streaming to
	// PutObject so we used this work-around.
	url, headers, err := putObj.PresignRequest(15 * time.Minute)
	if err != nil {
		return etag, lastModified, nil, fmt.Errorf("s3 upload: sign request: %w", err)
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
		return etag, lastModified, nil, fmt.Errorf("s3 upload: new request: %w", err)
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
		return etag, lastModified, nil, err
	}
	if resp != nil {
		if date, err := http.ParseTime(resp.Header.Get("Date")); err != nil {
			lastModified = date
		}
		etag = resp.Header.Get("Etag")
		vID := resp.Header.Get("x-amz-version-id")
		if vID != "" {
			versionID = &vID
		}
	}
	return etag, lastModified, versionID, nil
}

// Info needed for an upload
type uploadInfo struct {
	req       *s3.PutObjectInput
	md5sumHex string
}

// Prepare object for being uploaded
func (o *Object) prepareUpload(ctx context.Context, src fs.ObjectInfo, options []fs.OpenOption) (ui uploadInfo, err error) {
	bucket, bucketPath := o.split()
	// Create parent dir/bucket if not saving directory marker
	if !strings.HasSuffix(o.remote, "/") {
		err := o.fs.mkdirParent(ctx, o.remote)
		if err != nil {
			return ui, err
		}
	}
	modTime := src.ModTime(ctx)

	ui.req = &s3.PutObjectInput{
		Bucket: &bucket,
		ACL:    stringPointerOrNil(o.fs.opt.ACL),
		Key:    &bucketPath,
	}

	// Fetch metadata if --metadata is in use
	meta, err := fs.GetMetadataOptions(ctx, o.fs, src, options)
	if err != nil {
		return ui, fmt.Errorf("failed to read metadata from source object: %w", err)
	}
	ui.req.Metadata = make(map[string]*string, len(meta)+2)
	// merge metadata into request and user metadata
	for k, v := range meta {
		pv := aws.String(v)
		k = strings.ToLower(k)
		if o.fs.opt.NoSystemMetadata {
			ui.req.Metadata[k] = pv
			continue
		}
		switch k {
		case "cache-control":
			ui.req.CacheControl = pv
		case "content-disposition":
			ui.req.ContentDisposition = pv
		case "content-encoding":
			ui.req.ContentEncoding = pv
		case "content-language":
			ui.req.ContentLanguage = pv
		case "content-type":
			ui.req.ContentType = pv
		case "x-amz-tagging":
			ui.req.Tagging = pv
		case "tier":
			// ignore
		case "mtime":
			// mtime in meta overrides source ModTime
			metaModTime, err := time.Parse(time.RFC3339Nano, v)
			if err != nil {
				fs.Debugf(o, "failed to parse metadata %s: %q: %v", k, v, err)
			} else {
				modTime = metaModTime
			}
		case "btime":
			// write as metadata since we can't set it
			ui.req.Metadata[k] = pv
		default:
			ui.req.Metadata[k] = pv
		}
	}

	// Set the mtime in the meta data
	ui.req.Metadata[metaMtime] = aws.String(swift.TimeToFloatString(modTime))

	// read the md5sum if available
	// - for non multipart
	//    - so we can add a ContentMD5
	//    - so we can add the md5sum in the metadata as metaMD5Hash if using SSE/SSE-C
	// - for multipart provided checksums aren't disabled
	//    - so we can add the md5sum in the metadata as metaMD5Hash
	var md5sumBase64 string
	size := src.Size()
	multipart := size < 0 || size >= int64(o.fs.opt.UploadCutoff)
	if !multipart || !o.fs.opt.DisableChecksum {
		ui.md5sumHex, err = src.Hash(ctx, hash.MD5)
		if err == nil && matchMd5.MatchString(ui.md5sumHex) {
			hashBytes, err := hex.DecodeString(ui.md5sumHex)
			if err == nil {
				md5sumBase64 = base64.StdEncoding.EncodeToString(hashBytes)
				if (multipart || o.fs.etagIsNotMD5) && !o.fs.opt.DisableChecksum {
					// Set the md5sum as metadata on the object if
					// - a multipart upload
					// - the Etag is not an MD5, eg when using SSE/SSE-C
					// provided checksums aren't disabled
					ui.req.Metadata[metaMD5Hash] = &md5sumBase64
				}
			}
		}
	}

	// Set the content type if it isn't set already
	if ui.req.ContentType == nil {
		ui.req.ContentType = aws.String(fs.MimeType(ctx, src))
	}
	if size >= 0 {
		ui.req.ContentLength = &size
	}
	if md5sumBase64 != "" {
		ui.req.ContentMD5 = &md5sumBase64
	}
	if o.fs.opt.RequesterPays {
		ui.req.RequestPayer = aws.String(s3.RequestPayerRequester)
	}
	if o.fs.opt.ServerSideEncryption != "" {
		ui.req.ServerSideEncryption = &o.fs.opt.ServerSideEncryption
	}
	if o.fs.opt.SSECustomerAlgorithm != "" {
		ui.req.SSECustomerAlgorithm = &o.fs.opt.SSECustomerAlgorithm
	}
	if o.fs.opt.SSECustomerKey != "" {
		ui.req.SSECustomerKey = &o.fs.opt.SSECustomerKey
	}
	if o.fs.opt.SSECustomerKeyMD5 != "" {
		ui.req.SSECustomerKeyMD5 = &o.fs.opt.SSECustomerKeyMD5
	}
	if o.fs.opt.SSEKMSKeyID != "" {
		ui.req.SSEKMSKeyId = &o.fs.opt.SSEKMSKeyID
	}
	if o.fs.opt.StorageClass != "" {
		ui.req.StorageClass = &o.fs.opt.StorageClass
	}
	// Apply upload options
	for _, option := range options {
		key, value := option.Header()
		lowerKey := strings.ToLower(key)
		switch lowerKey {
		case "":
			// ignore
		case "cache-control":
			ui.req.CacheControl = aws.String(value)
		case "content-disposition":
			ui.req.ContentDisposition = aws.String(value)
		case "content-encoding":
			ui.req.ContentEncoding = aws.String(value)
		case "content-language":
			ui.req.ContentLanguage = aws.String(value)
		case "content-type":
			ui.req.ContentType = aws.String(value)
		case "x-amz-tagging":
			ui.req.Tagging = aws.String(value)
		default:
			const amzMetaPrefix = "x-amz-meta-"
			if strings.HasPrefix(lowerKey, amzMetaPrefix) {
				metaKey := lowerKey[len(amzMetaPrefix):]
				ui.req.Metadata[metaKey] = aws.String(value)
			} else {
				fs.Errorf(o, "Don't know how to set key %q on upload", key)
			}
		}
	}

	// Check metadata keys and values are valid
	for key, value := range ui.req.Metadata {
		if !httpguts.ValidHeaderFieldName(key) {
			fs.Errorf(o, "Dropping invalid metadata key %q", key)
			delete(ui.req.Metadata, key)
		} else if value == nil {
			fs.Errorf(o, "Dropping nil metadata value for key %q", key)
			delete(ui.req.Metadata, key)
		} else if !httpguts.ValidHeaderFieldValue(*value) {
			fs.Errorf(o, "Dropping invalid metadata value %q for key %q", *value, key)
			delete(ui.req.Metadata, key)
		}
	}

	return ui, nil
}

// Update the Object from in with modTime and size
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	if o.fs.opt.VersionAt.IsSet() {
		return errNotWithVersionAt
	}
	size := src.Size()
	multipart := size < 0 || size >= int64(o.fs.opt.UploadCutoff)

	var wantETag string        // Multipart upload Etag to check
	var gotETag string         // Etag we got from the upload
	var lastModified time.Time // Time we got from the upload
	var versionID *string      // versionID we got from the upload
	var err error
	var ui uploadInfo
	if multipart {
		wantETag, gotETag, versionID, ui, err = o.uploadMultipart(ctx, src, in, options...)
	} else {
		ui, err = o.prepareUpload(ctx, src, options)
		if err != nil {
			return fmt.Errorf("failed to prepare upload: %w", err)
		}

		if o.fs.opt.UsePresignedRequest {
			gotETag, lastModified, versionID, err = o.uploadSinglepartPresignedRequest(ctx, ui.req, size, in)
		} else {
			gotETag, lastModified, versionID, err = o.uploadSinglepartPutObject(ctx, ui.req, size, in)
		}
	}
	if err != nil {
		return err
	}
	// Only record versionID if we are using --s3-versions or --s3-version-at
	if o.fs.opt.Versions || o.fs.opt.VersionAt.IsSet() {
		o.versionID = versionID
	} else {
		o.versionID = nil
	}

	// User requested we don't HEAD the object after uploading it
	// so make up the object as best we can assuming it got
	// uploaded properly. If size < 0 then we need to do the HEAD.
	var head *s3.HeadObjectOutput
	if o.fs.opt.NoHead && size >= 0 {
		head = new(s3.HeadObjectOutput)
		//structs.SetFrom(head, &req)
		setFrom_s3HeadObjectOutput_s3PutObjectInput(head, ui.req)
		head.ETag = &ui.md5sumHex // doesn't matter quotes are missing
		head.ContentLength = &size
		// We get etag back from single and multipart upload so fill it in here
		if gotETag != "" {
			head.ETag = &gotETag
		}
		if lastModified.IsZero() {
			lastModified = time.Now()
		}
		head.LastModified = &lastModified
		head.VersionId = versionID
	} else {
		// Read the metadata from the newly created object
		o.meta = nil // wipe old metadata
		head, err = o.headObject(ctx)
		if err != nil {
			return err
		}
	}
	o.setMetaData(head)

	// Check multipart upload ETag if required
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
	if o.fs.opt.VersionAt.IsSet() {
		return errNotWithVersionAt
	}
	bucket, bucketPath := o.split()
	req := s3.DeleteObjectInput{
		Bucket:    &bucket,
		Key:       &bucketPath,
		VersionId: o.versionID,
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
	o.storageClass = &tier
	return err
}

// GetTier returns storage class as string
func (o *Object) GetTier() string {
	if o.storageClass == nil || *o.storageClass == "" {
		return "STANDARD"
	}
	return *o.storageClass
}

// Metadata returns metadata for an object
//
// It should return nil if there is no Metadata
func (o *Object) Metadata(ctx context.Context) (metadata fs.Metadata, err error) {
	err = o.readMetaData(ctx)
	if err != nil {
		return nil, err
	}
	metadata = make(fs.Metadata, len(o.meta)+7)
	for k, v := range o.meta {
		switch k {
		case metaMtime:
			if modTime, err := swift.FloatStringToTime(v); err == nil {
				metadata["mtime"] = modTime.Format(time.RFC3339Nano)
			}
		case metaMD5Hash:
			// don't write hash metadata
		default:
			metadata[k] = v
		}
	}
	if o.mimeType != "" {
		metadata["content-type"] = o.mimeType
	}
	// metadata["x-amz-tagging"] = ""
	if !o.lastModified.IsZero() {
		metadata["btime"] = o.lastModified.Format(time.RFC3339Nano)
	}

	// Set system metadata
	setMetadata := func(k string, v *string) {
		if o.fs.opt.NoSystemMetadata {
			return
		}
		if v == nil || *v == "" {
			return
		}
		metadata[k] = *v
	}
	setMetadata("cache-control", o.cacheControl)
	setMetadata("content-disposition", o.contentDisposition)
	setMetadata("content-encoding", o.contentEncoding)
	setMetadata("content-language", o.contentLanguage)
	metadata["tier"] = o.GetTier()

	return metadata, nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = &Fs{}
	_ fs.Purger          = &Fs{}
	_ fs.Copier          = &Fs{}
	_ fs.PutStreamer     = &Fs{}
	_ fs.ListRer         = &Fs{}
	_ fs.Commander       = &Fs{}
	_ fs.CleanUpper      = &Fs{}
	_ fs.OpenChunkWriter = &Fs{}
	_ fs.Object          = &Object{}
	_ fs.MimeTyper       = &Object{}
	_ fs.GetTierer       = &Object{}
	_ fs.SetTierer       = &Object{}
	_ fs.Metadataer      = &Object{}
)
