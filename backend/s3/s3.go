// Package s3 provides an interface to Amazon S3 oject storage
package s3

// FIXME need to prevent anything but ListDir working for s3://

/*
Progress of port to aws-sdk

 * Don't really need o.meta at all?

What happens if you CTRL-C a multipart upload
  * get an incomplete upload
  * disappears when you delete the bucket
*/

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
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
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/ncw/swift"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
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
		Description: "Amazon S3 Compliant Storage Provider (AWS, Alibaba, Ceph, Digital Ocean, Dreamhost, IBM COS, Minio, etc)",
		NewFs:       NewFs,
		CommandHelp: commandHelp,
		Options: []fs.Option{{
			Name: fs.ConfigProvider,
			Help: "Choose your S3 provider.",
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
				Value: "DigitalOcean",
				Help:  "Digital Ocean Spaces",
			}, {
				Value: "Dreamhost",
				Help:  "Dreamhost DreamObjects",
			}, {
				Value: "IBMCOS",
				Help:  "IBM COS S3",
			}, {
				Value: "Minio",
				Help:  "Minio Object Storage",
			}, {
				Value: "Netease",
				Help:  "Netease Object Storage (NOS)",
			}, {
				Value: "Scaleway",
				Help:  "Scaleway Object Storage",
			}, {
				Value: "StackPath",
				Help:  "StackPath Object Storage",
			}, {
				Value: "Wasabi",
				Help:  "Wasabi Object Storage",
			}, {
				Value: "Other",
				Help:  "Any other S3 compatible provider",
			}},
		}, {
			Name:    "env_auth",
			Help:    "Get AWS credentials from runtime (environment variables or EC2/ECS meta data if no env vars).\nOnly applies if access_key_id and secret_access_key is blank.",
			Default: false,
			Examples: []fs.OptionExample{{
				Value: "false",
				Help:  "Enter AWS credentials in the next step",
			}, {
				Value: "true",
				Help:  "Get AWS credentials from the environment (env vars or IAM)",
			}},
		}, {
			Name: "access_key_id",
			Help: "AWS Access Key ID.\nLeave blank for anonymous access or runtime credentials.",
		}, {
			Name: "secret_access_key",
			Help: "AWS Secret Access Key (password)\nLeave blank for anonymous access or runtime credentials.",
		}, {
			Name:     "region",
			Help:     "Region to connect to.",
			Provider: "AWS",
			Examples: []fs.OptionExample{{
				Value: "us-east-1",
				Help:  "The default endpoint - a good choice if you are unsure.\nUS Region, Northern Virginia or Pacific Northwest.\nLeave location constraint empty.",
			}, {
				Value: "us-east-2",
				Help:  "US East (Ohio) Region\nNeeds location constraint us-east-2.",
			}, {
				Value: "us-west-2",
				Help:  "US West (Oregon) Region\nNeeds location constraint us-west-2.",
			}, {
				Value: "us-west-1",
				Help:  "US West (Northern California) Region\nNeeds location constraint us-west-1.",
			}, {
				Value: "ca-central-1",
				Help:  "Canada (Central) Region\nNeeds location constraint ca-central-1.",
			}, {
				Value: "eu-west-1",
				Help:  "EU (Ireland) Region\nNeeds location constraint EU or eu-west-1.",
			}, {
				Value: "eu-west-2",
				Help:  "EU (London) Region\nNeeds location constraint eu-west-2.",
			}, {
				Value: "eu-north-1",
				Help:  "EU (Stockholm) Region\nNeeds location constraint eu-north-1.",
			}, {
				Value: "eu-central-1",
				Help:  "EU (Frankfurt) Region\nNeeds location constraint eu-central-1.",
			}, {
				Value: "ap-southeast-1",
				Help:  "Asia Pacific (Singapore) Region\nNeeds location constraint ap-southeast-1.",
			}, {
				Value: "ap-southeast-2",
				Help:  "Asia Pacific (Sydney) Region\nNeeds location constraint ap-southeast-2.",
			}, {
				Value: "ap-northeast-1",
				Help:  "Asia Pacific (Tokyo) Region\nNeeds location constraint ap-northeast-1.",
			}, {
				Value: "ap-northeast-2",
				Help:  "Asia Pacific (Seoul)\nNeeds location constraint ap-northeast-2.",
			}, {
				Value: "ap-south-1",
				Help:  "Asia Pacific (Mumbai)\nNeeds location constraint ap-south-1.",
			}, {
				Value: "ap-east-1",
				Help:  "Asia Patific (Hong Kong) Region\nNeeds location constraint ap-east-1.",
			}, {
				Value: "sa-east-1",
				Help:  "South America (Sao Paulo) Region\nNeeds location constraint sa-east-1.",
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
			}},
		}, {
			Name:     "region",
			Help:     "Region to connect to.\nLeave blank if you are using an S3 clone and you don't have a region.",
			Provider: "!AWS,Alibaba,Scaleway",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "Use this if unsure. Will use v4 signatures and an empty region.",
			}, {
				Value: "other-v2-signature",
				Help:  "Use this only if v4 signatures don't work, eg pre Jewel/v10 CEPH.",
			}},
		}, {
			Name:     "endpoint",
			Help:     "Endpoint for S3 API.\nLeave blank if using AWS to use the default endpoint for the region.",
			Provider: "AWS",
		}, {
			Name:     "endpoint",
			Help:     "Endpoint for IBM COS S3 API.\nSpecify if using an IBM COS On Premise.",
			Provider: "IBMCOS",
			Examples: []fs.OptionExample{{
				Value: "s3-api.us-geo.objectstorage.softlayer.net",
				Help:  "US Cross Region Endpoint",
			}, {
				Value: "s3-api.dal.us-geo.objectstorage.softlayer.net",
				Help:  "US Cross Region Dallas Endpoint",
			}, {
				Value: "s3-api.wdc-us-geo.objectstorage.softlayer.net",
				Help:  "US Cross Region Washington DC Endpoint",
			}, {
				Value: "s3-api.sjc-us-geo.objectstorage.softlayer.net",
				Help:  "US Cross Region San Jose Endpoint",
			}, {
				Value: "s3-api.us-geo.objectstorage.service.networklayer.com",
				Help:  "US Cross Region Private Endpoint",
			}, {
				Value: "s3-api.dal-us-geo.objectstorage.service.networklayer.com",
				Help:  "US Cross Region Dallas Private Endpoint",
			}, {
				Value: "s3-api.wdc-us-geo.objectstorage.service.networklayer.com",
				Help:  "US Cross Region Washington DC Private Endpoint",
			}, {
				Value: "s3-api.sjc-us-geo.objectstorage.service.networklayer.com",
				Help:  "US Cross Region San Jose Private Endpoint",
			}, {
				Value: "s3.us-east.objectstorage.softlayer.net",
				Help:  "US Region East Endpoint",
			}, {
				Value: "s3.us-east.objectstorage.service.networklayer.com",
				Help:  "US Region East Private Endpoint",
			}, {
				Value: "s3.us-south.objectstorage.softlayer.net",
				Help:  "US Region South Endpoint",
			}, {
				Value: "s3.us-south.objectstorage.service.networklayer.com",
				Help:  "US Region South Private Endpoint",
			}, {
				Value: "s3.eu-geo.objectstorage.softlayer.net",
				Help:  "EU Cross Region Endpoint",
			}, {
				Value: "s3.fra-eu-geo.objectstorage.softlayer.net",
				Help:  "EU Cross Region Frankfurt Endpoint",
			}, {
				Value: "s3.mil-eu-geo.objectstorage.softlayer.net",
				Help:  "EU Cross Region Milan Endpoint",
			}, {
				Value: "s3.ams-eu-geo.objectstorage.softlayer.net",
				Help:  "EU Cross Region Amsterdam Endpoint",
			}, {
				Value: "s3.eu-geo.objectstorage.service.networklayer.com",
				Help:  "EU Cross Region Private Endpoint",
			}, {
				Value: "s3.fra-eu-geo.objectstorage.service.networklayer.com",
				Help:  "EU Cross Region Frankfurt Private Endpoint",
			}, {
				Value: "s3.mil-eu-geo.objectstorage.service.networklayer.com",
				Help:  "EU Cross Region Milan Private Endpoint",
			}, {
				Value: "s3.ams-eu-geo.objectstorage.service.networklayer.com",
				Help:  "EU Cross Region Amsterdam Private Endpoint",
			}, {
				Value: "s3.eu-gb.objectstorage.softlayer.net",
				Help:  "Great Britain Endpoint",
			}, {
				Value: "s3.eu-gb.objectstorage.service.networklayer.com",
				Help:  "Great Britain Private Endpoint",
			}, {
				Value: "s3.ap-geo.objectstorage.softlayer.net",
				Help:  "APAC Cross Regional Endpoint",
			}, {
				Value: "s3.tok-ap-geo.objectstorage.softlayer.net",
				Help:  "APAC Cross Regional Tokyo Endpoint",
			}, {
				Value: "s3.hkg-ap-geo.objectstorage.softlayer.net",
				Help:  "APAC Cross Regional HongKong Endpoint",
			}, {
				Value: "s3.seo-ap-geo.objectstorage.softlayer.net",
				Help:  "APAC Cross Regional Seoul Endpoint",
			}, {
				Value: "s3.ap-geo.objectstorage.service.networklayer.com",
				Help:  "APAC Cross Regional Private Endpoint",
			}, {
				Value: "s3.tok-ap-geo.objectstorage.service.networklayer.com",
				Help:  "APAC Cross Regional Tokyo Private Endpoint",
			}, {
				Value: "s3.hkg-ap-geo.objectstorage.service.networklayer.com",
				Help:  "APAC Cross Regional HongKong Private Endpoint",
			}, {
				Value: "s3.seo-ap-geo.objectstorage.service.networklayer.com",
				Help:  "APAC Cross Regional Seoul Private Endpoint",
			}, {
				Value: "s3.mel01.objectstorage.softlayer.net",
				Help:  "Melbourne Single Site Endpoint",
			}, {
				Value: "s3.mel01.objectstorage.service.networklayer.com",
				Help:  "Melbourne Single Site Private Endpoint",
			}, {
				Value: "s3.tor01.objectstorage.softlayer.net",
				Help:  "Toronto Single Site Endpoint",
			}, {
				Value: "s3.tor01.objectstorage.service.networklayer.com",
				Help:  "Toronto Single Site Private Endpoint",
			}},
		}, {
			// oss endpoints: https://help.aliyun.com/document_detail/31837.html
			Name:     "endpoint",
			Help:     "Endpoint for OSS API.",
			Provider: "Alibaba",
			Examples: []fs.OptionExample{{
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
				Help:  "North China 5 (Huhehaote)",
			}, {
				Value: "oss-cn-shenzhen.aliyuncs.com",
				Help:  "South China 1 (Shenzhen)",
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
			Name:     "endpoint",
			Help:     "Endpoint for Scaleway Object Storage.",
			Provider: "Scaleway",
			Examples: []fs.OptionExample{{
				Value: "s3.nl-ams.scw.cloud",
				Help:  "Amsterdam Endpoint",
			}, {
				Value: "s3.fr-par.scw.cloud",
				Help:  "Paris Endpoint",
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
			Help:     "Endpoint for S3 API.\nRequired when using an S3 clone.",
			Provider: "!AWS,IBMCOS,Alibaba,Scaleway,StackPath",
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
			}},
		}, {
			Name:     "location_constraint",
			Help:     "Location constraint - must be set to match the Region.\nUsed when creating buckets only.",
			Provider: "AWS",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "Empty for US Region, Northern Virginia or Pacific Northwest.",
			}, {
				Value: "us-east-2",
				Help:  "US East (Ohio) Region.",
			}, {
				Value: "us-west-2",
				Help:  "US West (Oregon) Region.",
			}, {
				Value: "us-west-1",
				Help:  "US West (Northern California) Region.",
			}, {
				Value: "ca-central-1",
				Help:  "Canada (Central) Region.",
			}, {
				Value: "eu-west-1",
				Help:  "EU (Ireland) Region.",
			}, {
				Value: "eu-west-2",
				Help:  "EU (London) Region.",
			}, {
				Value: "eu-north-1",
				Help:  "EU (Stockholm) Region.",
			}, {
				Value: "EU",
				Help:  "EU Region.",
			}, {
				Value: "ap-southeast-1",
				Help:  "Asia Pacific (Singapore) Region.",
			}, {
				Value: "ap-southeast-2",
				Help:  "Asia Pacific (Sydney) Region.",
			}, {
				Value: "ap-northeast-1",
				Help:  "Asia Pacific (Tokyo) Region.",
			}, {
				Value: "ap-northeast-2",
				Help:  "Asia Pacific (Seoul)",
			}, {
				Value: "ap-south-1",
				Help:  "Asia Pacific (Mumbai)",
			}, {
				Value: "ap-east-1",
				Help:  "Asia Pacific (Hong Kong)",
			}, {
				Value: "sa-east-1",
				Help:  "South America (Sao Paulo) Region.",
			}},
		}, {
			Name:     "location_constraint",
			Help:     "Location constraint - must match endpoint when using IBM Cloud Public.\nFor on-prem COS, do not make a selection from this list, hit enter",
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
			Help:     "Location constraint - must be set to match the Region.\nLeave blank if not sure. Used when creating buckets only.",
			Provider: "!AWS,IBMCOS,Alibaba,Scaleway,StackPath",
		}, {
			Name: "acl",
			Help: `Canned ACL used when creating buckets and storing or copying objects.

This ACL is used for creating objects and if bucket_acl isn't set, for creating buckets too.

For more info visit https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl

Note that this ACL is applied when server side copying objects as S3
doesn't copy the ACL from the source but rather writes a fresh one.`,
			Examples: []fs.OptionExample{{
				Value:    "private",
				Help:     "Owner gets FULL_CONTROL. No one else has access rights (default).",
				Provider: "!IBMCOS",
			}, {
				Value:    "public-read",
				Help:     "Owner gets FULL_CONTROL. The AllUsers group gets READ access.",
				Provider: "!IBMCOS",
			}, {
				Value:    "public-read-write",
				Help:     "Owner gets FULL_CONTROL. The AllUsers group gets READ and WRITE access.\nGranting this on a bucket is generally not recommended.",
				Provider: "!IBMCOS",
			}, {
				Value:    "authenticated-read",
				Help:     "Owner gets FULL_CONTROL. The AuthenticatedUsers group gets READ access.",
				Provider: "!IBMCOS",
			}, {
				Value:    "bucket-owner-read",
				Help:     "Object owner gets FULL_CONTROL. Bucket owner gets READ access.\nIf you specify this canned ACL when creating a bucket, Amazon S3 ignores it.",
				Provider: "!IBMCOS",
			}, {
				Value:    "bucket-owner-full-control",
				Help:     "Both the object owner and the bucket owner get FULL_CONTROL over the object.\nIf you specify this canned ACL when creating a bucket, Amazon S3 ignores it.",
				Provider: "!IBMCOS",
			}, {
				Value:    "private",
				Help:     "Owner gets FULL_CONTROL. No one else has access rights (default). This acl is available on IBM Cloud (Infra), IBM Cloud (Storage), On-Premise COS",
				Provider: "IBMCOS",
			}, {
				Value:    "public-read",
				Help:     "Owner gets FULL_CONTROL. The AllUsers group gets READ access. This acl is available on IBM Cloud (Infra), IBM Cloud (Storage), On-Premise IBM COS",
				Provider: "IBMCOS",
			}, {
				Value:    "public-read-write",
				Help:     "Owner gets FULL_CONTROL. The AllUsers group gets READ and WRITE access. This acl is available on IBM Cloud (Infra), On-Premise IBM COS",
				Provider: "IBMCOS",
			}, {
				Value:    "authenticated-read",
				Help:     "Owner gets FULL_CONTROL. The AuthenticatedUsers group gets READ access. Not supported on Buckets. This acl is available on IBM Cloud (Infra) and On-Premise IBM COS",
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
				Help:  "Owner gets FULL_CONTROL. No one else has access rights (default).",
			}, {
				Value: "public-read",
				Help:  "Owner gets FULL_CONTROL. The AllUsers group gets READ access.",
			}, {
				Value: "public-read-write",
				Help:  "Owner gets FULL_CONTROL. The AllUsers group gets READ and WRITE access.\nGranting this on a bucket is generally not recommended.",
			}, {
				Value: "authenticated-read",
				Help:  "Owner gets FULL_CONTROL. The AuthenticatedUsers group gets READ access.",
			}},
		}, {
			Name:     "server_side_encryption",
			Help:     "The server-side encryption algorithm used when storing this object in S3.",
			Provider: "AWS,Ceph,Minio",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "None",
			}, {
				Value: "AES256",
				Help:  "AES256",
			}, {
				Value: "aws:kms",
				Help:  "aws:kms",
			}},
		}, {
			Name:     "sse_customer_algorithm",
			Help:     "If using SSE-C, the server-side encryption algorithm used when storing this object in S3.",
			Provider: "AWS,Ceph,Minio",
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
			Provider: "AWS,Ceph,Minio",
			Advanced: true,
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "None",
			}},
		}, {
			Name:     "sse_customer_key_md5",
			Help:     "If using SSE-C you must provide the secret encryption key MD5 checksum.",
			Provider: "AWS,Ceph,Minio",
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
				Help:  "Archive storage mode.",
			}, {
				Value: "STANDARD_IA",
				Help:  "Infrequent access storage mode.",
			}},
		}, {
			// Mapping from here: https://www.scaleway.com/en/docs/object-storage-glacier/#-Scaleway-Storage-Classes
			Name:     "storage_class",
			Help:     "The storage class to use when storing new objects in S3.",
			Provider: "Scaleway",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "Default",
			}, {
				Value: "STANDARD",
				Help:  "The Standard class for any upload; suitable for on-demand content like streaming or CDN.",
			}, {
				Value: "GLACIER",
				Help:  "Archived storage; prices are lower, but it needs to be restored first to be accessed.",
			}},
		}, {
			Name: "upload_cutoff",
			Help: `Cutoff for switching to chunked upload

Any files larger than this will be uploaded in chunks of chunk_size.
The minimum is 0 and the maximum is 5GB.`,
			Default:  defaultUploadCutoff,
			Advanced: true,
		}, {
			Name: "chunk_size",
			Help: `Chunk size to use for uploading.

When uploading files larger than upload_cutoff or files with unknown
size (eg from "rclone rcat" or uploaded with "rclone mount" or google
photos or google docs) they will be uploaded as multipart uploads
using this chunk size.

Note that "--s3-upload-concurrency" chunks of this size are buffered
in memory per transfer.

If you are transferring large files over high speed links and you have
enough memory, then increasing this will speed up the transfers.

Rclone will automatically increase the chunk size when uploading a
large file of known size to stay below the 10,000 chunks limit.

Files of unknown size are uploaded with the configured
chunk_size. Since the default chunk size is 5MB and there can be at
most 10,000 chunks, this means that by default the maximum size of
file you can stream upload is 48GB.  If you wish to stream upload
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
			Help: `Cutoff for switching to multipart copy

Any files larger than this that need to be server side copied will be
copied in chunks of this size.

The minimum is 0 and the maximum is 5GB.`,
			Default:  fs.SizeSuffix(maxSizeForCopy),
			Advanced: true,
		}, {
			Name: "disable_checksum",
			Help: `Don't store MD5 checksum with object metadata

Normally rclone will calculate the MD5 checksum of the input before
uploading it so it can add it to metadata on the object. This is great
for data integrity checking but can cause long delays for large files
to start uploading.`,
			Default:  false,
			Advanced: true,
		}, {
			Name:     "session_token",
			Help:     "An AWS session token",
			Advanced: true,
		}, {
			Name: "upload_concurrency",
			Help: `Concurrency for multipart uploads.

This is the number of chunks of the same file that are uploaded
concurrently.

If you are uploading small numbers of large file over high speed link
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

Some providers (eg AWS, Aliyun OSS or Netease COS) require this set to
false - rclone will do this automatically based on the provider
setting.`,
			Default:  true,
			Advanced: true,
		}, {
			Name: "v2_auth",
			Help: `If true use v2 authentication.

If this is false (the default) then rclone will use v4 authentication.
If it is set then rclone will use v2 authentication.

Use this only if v4 signatures don't work, eg pre Jewel/v10 CEPH.`,
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
		},
		}})
}

// Constants
const (
	metaMtime           = "Mtime"                // the meta key to store mtime in - eg X-Amz-Meta-Mtime
	metaMD5Hash         = "Md5chksum"            // the meta key to store md5hash in
	maxSizeForCopy      = 5 * 1024 * 1024 * 1024 // The maximum size of object we can COPY
	maxUploadParts      = 10000                  // maximum allowed number of parts in a multi-part upload
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
	SessionToken          string               `config:"session_token"`
	UploadConcurrency     int                  `config:"upload_concurrency"`
	ForcePathStyle        bool                 `config:"force_path_style"`
	V2Auth                bool                 `config:"v2_auth"`
	UseAccelerateEndpoint bool                 `config:"use_accelerate_endpoint"`
	LeavePartsOnError     bool                 `config:"leave_parts_on_error"`
	ListChunk             int64                `config:"list_chunk"`
	Enc                   encoder.MultiEncoder `config:"encoding"`
	MemoryPoolFlushTime   fs.Duration          `config:"memory_pool_flush_time"`
	MemoryPoolUseMmap     bool                 `config:"memory_pool_use_mmap"`
}

// Fs represents a remote s3 server
type Fs struct {
	name          string           // the name of the remote
	root          string           // root of the bucket - ignore all objects above this
	opt           Options          // parsed options
	features      *fs.Features     // optional features
	c             *s3.S3           // the connection to the s3 server
	ses           *session.Session // the s3 session
	rootBucket    string           // bucket part of root (if any)
	rootDirectory string           // directory part of root (if any)
	cache         *bucket.Cache    // cache for bucket creation status
	pacer         *fs.Pacer        // To pace the API calls
	srv           *http.Client     // a plain http client
	pool          *pool.Pool       // memory pool
}

// Object describes a s3 object
type Object struct {
	// Will definitely have everything but meta which may be nil
	//
	// List will read everything but meta & mimeType - to fill
	// that in you need to call readMetaData
	fs           *Fs                // what this object is part of
	remote       string             // The remote path
	etag         string             // md5sum of the object
	bytes        int64              // size of the object
	lastModified time.Time          // Last modified
	meta         map[string]*string // The object metadata if known - may be nil
	mimeType     string             // MimeType of object - may be ""
	storageClass string             // eg GLACIER
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
		return fmt.Sprintf("S3 root")
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
	500, // Internal Server Error - "We encountered an internal error. Please try again."
	503, // Service Unavailable/Slow Down - "Reduce your request rate"
}

//S3 is pretty resilient, and the built in retry handling is probably sufficient
// as it should notice closed connections and timeouts which are the most likely
// sort of failure modes
func (f *Fs) shouldRetry(err error) (bool, error) {
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
					urfbErr := f.updateRegionForBucket(f.rootBucket)
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

// s3Connection makes a connection to s3
func s3Connection(opt *Options) (*s3.S3, *session.Session, error) {
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
		return nil, nil, errors.Wrap(err, "NewSession")
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
		&credentials.SharedCredentialsProvider{},

		// Pick up IAM role if we're in an ECS task
		defaults.RemoteCredProvider(*def.Config, def.Handlers),

		// Pick up IAM role in case we're on EC2
		&ec2rolecreds.EC2RoleProvider{
			Client: ec2metadata.New(awsSession, &aws.Config{
				HTTPClient: lowTimeoutClient,
			}),
			ExpiryWindow: 3 * time.Minute,
		},

		// Pick up IAM role if we are in EKS
		&stscreds.WebIdentityRoleProvider{
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
		return nil, nil, errors.New("access_key_id not found")
	case v.SecretAccessKey == "":
		return nil, nil, errors.New("secret_access_key not found")
	}

	if opt.Region == "" {
		opt.Region = "us-east-1"
	}
	if opt.Provider == "AWS" || opt.Provider == "Alibaba" || opt.Provider == "Netease" || opt.Provider == "Scaleway" || opt.UseAccelerateEndpoint {
		opt.ForcePathStyle = false
	}
	if opt.Provider == "Scaleway" && opt.MaxUploadParts > 1000 {
		opt.MaxUploadParts = 1000
	}
	awsConfig := aws.NewConfig().
		WithMaxRetries(0). // Rely on rclone's retry logic
		WithCredentials(cred).
		WithHTTPClient(fshttp.NewClient(fs.Config)).
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
		return errors.Errorf("%s is less than %s", cs, minChunkSize)
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
		return errors.Errorf("%s is greater than %s", cs, maxUploadCutoff)
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

// setRoot changes the root of the Fs
func (f *Fs) setRoot(root string) {
	f.root = parsePath(root)
	f.rootBucket, f.rootDirectory = bucket.Split(f.root)
}

// NewFs constructs an Fs from the path, bucket:path
func NewFs(name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	err = checkUploadChunkSize(opt.ChunkSize)
	if err != nil {
		return nil, errors.Wrap(err, "s3: chunk size")
	}
	err = checkUploadCutoff(opt.UploadCutoff)
	if err != nil {
		return nil, errors.Wrap(err, "s3: upload cutoff")
	}
	if opt.ACL == "" {
		opt.ACL = "private"
	}
	if opt.BucketACL == "" {
		opt.BucketACL = opt.ACL
	}
	c, ses, err := s3Connection(opt)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:  name,
		opt:   *opt,
		c:     c,
		ses:   ses,
		pacer: fs.NewPacer(pacer.NewS3(pacer.MinSleep(minSleep))),
		cache: bucket.NewCache(),
		srv:   fshttp.NewClient(fs.Config),
		pool: pool.New(
			time.Duration(opt.MemoryPoolFlushTime),
			int(opt.ChunkSize),
			opt.UploadConcurrency*fs.Config.Transfers,
			opt.MemoryPoolUseMmap,
		),
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
	}).Fill(f)
	if f.rootBucket != "" && f.rootDirectory != "" {
		// Check to see if the object exists
		encodedDirectory := f.opt.Enc.FromStandardPath(f.rootDirectory)
		req := s3.HeadObjectInput{
			Bucket: &f.rootBucket,
			Key:    &encodedDirectory,
		}
		err = f.pacer.Call(func() (bool, error) {
			_, err = f.c.HeadObject(&req)
			return f.shouldRetry(err)
		})
		if err == nil {
			newRoot := path.Dir(f.root)
			if newRoot == "." {
				newRoot = ""
			}
			f.setRoot(newRoot)
			// return an error with an fs which points to the parent
			return f, fs.ErrorIsFile
		}
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
		o.etag = aws.StringValue(info.ETag)
		o.bytes = aws.Int64Value(info.Size)
		o.storageClass = aws.StringValue(info.StorageClass)
	} else {
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
func (f *Fs) getBucketLocation(bucket string) (string, error) {
	req := s3.GetBucketLocationInput{
		Bucket: &bucket,
	}
	var resp *s3.GetBucketLocationOutput
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.c.GetBucketLocation(&req)
		return f.shouldRetry(err)
	})
	if err != nil {
		return "", err
	}
	return s3.NormalizeBucketLocation(aws.StringValue(resp.LocationConstraint)), nil
}

// Updates the region for the bucket by reading the region from the
// bucket then updating the session.
func (f *Fs) updateRegionForBucket(bucket string) error {
	region, err := f.getBucketLocation(bucket)
	if err != nil {
		return errors.Wrap(err, "reading bucket location failed")
	}
	if aws.StringValue(f.c.Config.Endpoint) != "" {
		return errors.Errorf("can't set region to %q as endpoint is set", region)
	}
	if aws.StringValue(f.c.Config.Region) == region {
		return errors.Errorf("region is already %q - not updating", region)
	}

	// Make a new session with the new region
	oldRegion := f.opt.Region
	f.opt.Region = region
	c, ses, err := s3Connection(&f.opt)
	if err != nil {
		return errors.Wrap(err, "creating new session failed")
	}
	f.c = c
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
	var marker *string
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
	var urlEncodeListings = (f.opt.Provider == "AWS" || f.opt.Provider == "Wasabi" || f.opt.Provider == "Alibaba" || f.opt.Provider == "Minio")
	for {
		// FIXME need to implement ALL loop
		req := s3.ListObjectsInput{
			Bucket:    &bucket,
			Delimiter: &delimiter,
			Prefix:    &directory,
			MaxKeys:   &f.opt.ListChunk,
			Marker:    marker,
		}
		if urlEncodeListings {
			req.EncodingType = aws.String(s3.EncodingTypeUrl)
		}
		var resp *s3.ListObjectsOutput
		var err error
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.c.ListObjectsWithContext(ctx, &req)
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
			return f.shouldRetry(err)
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
				if strings.HasSuffix(remote, "/") {
					remote = remote[:len(remote)-1]
				}
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
		// Use NextMarker if set, otherwise use last Key
		if resp.NextMarker == nil || *resp.NextMarker == "" {
			if len(resp.Contents) == 0 {
				return errors.New("s3 protocol error: received listing with IsTruncated set, no NextMarker and no Contents")
			}
			marker = resp.Contents[len(resp.Contents)-1].Key
		} else {
			marker = resp.NextMarker
		}
		if urlEncodeListings {
			*marker, err = url.QueryUnescape(*marker)
			if err != nil {
				return errors.Wrapf(err, "failed to URL decode NextMarker %q", *marker)
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
		return f.shouldRetry(err)
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
		return f.shouldRetry(err)
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
			return f.shouldRetry(err)
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
			return f.shouldRetry(err)
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
	return strings.Replace(rest.URLPathEscape(s), "+", "%2B", -1)
}

// copy does a server side copy
//
// It adds the boiler plate to the req passed in and calls the s3
// method
func (f *Fs) copy(ctx context.Context, req *s3.CopyObjectInput, dstBucket, dstPath, srcBucket, srcPath string, srcSize int64) error {
	req.Bucket = &dstBucket
	req.ACL = &f.opt.ACL
	req.Key = &dstPath
	source := pathEscape(path.Join(srcBucket, srcPath))
	req.CopySource = &source
	if f.opt.ServerSideEncryption != "" {
		req.ServerSideEncryption = &f.opt.ServerSideEncryption
	}
	if f.opt.SSEKMSKeyID != "" {
		req.SSEKMSKeyId = &f.opt.SSEKMSKeyID
	}
	if req.StorageClass == nil && f.opt.StorageClass != "" {
		req.StorageClass = &f.opt.StorageClass
	}

	if srcSize >= int64(f.opt.CopyCutoff) {
		return f.copyMultipart(ctx, req, dstBucket, dstPath, srcBucket, srcPath, srcSize)
	}
	return f.pacer.Call(func() (bool, error) {
		_, err := f.c.CopyObjectWithContext(ctx, req)
		return f.shouldRetry(err)
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

func (f *Fs) copyMultipart(ctx context.Context, req *s3.CopyObjectInput, dstBucket, dstPath, srcBucket, srcPath string, srcSize int64) (err error) {
	var cout *s3.CreateMultipartUploadOutput
	if err := f.pacer.Call(func() (bool, error) {
		var err error
		cout, err = f.c.CreateMultipartUploadWithContext(ctx, &s3.CreateMultipartUploadInput{
			Bucket: &dstBucket,
			Key:    &dstPath,
		})
		return f.shouldRetry(err)
	}); err != nil {
		return err
	}
	uid := cout.UploadId

	defer atexit.OnError(&err, func() {
		// Try to abort the upload, but ignore the error.
		fs.Debugf(nil, "Cancelling multipart copy")
		_ = f.pacer.Call(func() (bool, error) {
			_, err := f.c.AbortMultipartUploadWithContext(context.Background(), &s3.AbortMultipartUploadInput{
				Bucket:       &dstBucket,
				Key:          &dstPath,
				UploadId:     uid,
				RequestPayer: req.RequestPayer,
			})
			return f.shouldRetry(err)
		})
	})()

	partSize := int64(f.opt.CopyCutoff)
	numParts := (srcSize-1)/partSize + 1

	var parts []*s3.CompletedPart
	for partNum := int64(1); partNum <= numParts; partNum++ {
		if err := f.pacer.Call(func() (bool, error) {
			partNum := partNum
			uploadPartReq := &s3.UploadPartCopyInput{
				Bucket:          &dstBucket,
				Key:             &dstPath,
				PartNumber:      &partNum,
				UploadId:        uid,
				CopySourceRange: aws.String(calculateRange(partSize, partNum-1, numParts, srcSize)),
				// Args copy from req
				CopySource:                     req.CopySource,
				CopySourceIfMatch:              req.CopySourceIfMatch,
				CopySourceIfModifiedSince:      req.CopySourceIfModifiedSince,
				CopySourceIfNoneMatch:          req.CopySourceIfNoneMatch,
				CopySourceIfUnmodifiedSince:    req.CopySourceIfUnmodifiedSince,
				CopySourceSSECustomerAlgorithm: req.CopySourceSSECustomerAlgorithm,
				CopySourceSSECustomerKey:       req.CopySourceSSECustomerKey,
				CopySourceSSECustomerKeyMD5:    req.CopySourceSSECustomerKeyMD5,
				RequestPayer:                   req.RequestPayer,
				SSECustomerAlgorithm:           req.SSECustomerAlgorithm,
				SSECustomerKey:                 req.SSECustomerKey,
				SSECustomerKeyMD5:              req.SSECustomerKeyMD5,
			}
			uout, err := f.c.UploadPartCopyWithContext(ctx, uploadPartReq)
			if err != nil {
				return f.shouldRetry(err)
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
		return f.shouldRetry(err)
	})
}

// Copy src to this remote using server side copy operations.
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
	err = f.copy(ctx, &req, dstBucket, dstPath, srcBucket, srcPath, srcObj.Size())
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
		f.opt.UploadConcurrency*fs.Config.Transfers,
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
keys. The Status will be OK if it was successfull or an error message
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
				return nil, errors.Wrap(err, "bad lifetime")
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
			bucket, bucketPath := o.split()
			reqCopy := req
			reqCopy.Bucket = &bucket
			reqCopy.Key = &bucketPath
			err = f.pacer.Call(func() (bool, error) {
				_, err = f.c.RestoreObject(&reqCopy)
				return f.shouldRetry(err)
			})
			if err != nil {
				st.Status = err.Error()
			}
		})
		if err != nil {
			return out, err
		}
		return out, nil
	default:
		return nil, fs.ErrorCommandNotFound
	}
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

// Hash returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	hash := strings.Trim(strings.ToLower(o.etag), `"`)
	// Check the etag is a valid md5sum
	if !matchMd5.MatchString(hash) {
		err := o.readMetaData(ctx)
		if err != nil {
			return "", err
		}

		if md5sum, ok := o.meta[metaMD5Hash]; ok {
			md5sumBytes, err := base64.StdEncoding.DecodeString(*md5sum)
			if err != nil {
				return "", err
			}
			hash = hex.EncodeToString(md5sumBytes)
		} else {
			hash = ""
		}
	}
	return hash, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.bytes
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.meta != nil {
		return nil
	}
	bucket, bucketPath := o.split()
	req := s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &bucketPath,
	}
	var resp *s3.HeadObjectOutput
	err = o.fs.pacer.Call(func() (bool, error) {
		var err error
		resp, err = o.fs.c.HeadObjectWithContext(ctx, &req)
		return o.fs.shouldRetry(err)
	})
	if err != nil {
		if awsErr, ok := err.(awserr.RequestFailure); ok {
			if awsErr.StatusCode() == http.StatusNotFound {
				// NotFound indicates bucket was OK
				// NoSuchBucket would be returned if bucket was bad
				if awsErr.Code() == "NotFound" {
					o.fs.cache.MarkOK(bucket)
				}
				return fs.ErrorObjectNotFound
			}
		}
		return err
	}
	o.fs.cache.MarkOK(bucket)
	var size int64
	// Ignore missing Content-Length assuming it is 0
	// Some versions of ceph do this due their apache proxies
	if resp.ContentLength != nil {
		size = *resp.ContentLength
	}
	o.etag = aws.StringValue(resp.ETag)
	o.bytes = size
	o.meta = resp.Metadata
	if o.meta == nil {
		o.meta = map[string]*string{}
	}
	o.storageClass = aws.StringValue(resp.StorageClass)
	if resp.LastModified == nil {
		fs.Logf(o, "Failed to read last modified from HEAD: %v", err)
		o.lastModified = time.Now()
	} else {
		o.lastModified = *resp.LastModified
	}
	o.mimeType = aws.StringValue(resp.ContentType)
	return nil
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	if fs.Config.UseServerModTime {
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
	return o.fs.copy(ctx, &req, bucket, bucketPath, bucket, bucketPath, o.bytes)
}

// Storable raturns a boolean indicating if this object is storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	bucket, bucketPath := o.split()
	req := s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &bucketPath,
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
		return o.fs.shouldRetry(err)
	})
	if err, ok := err.(awserr.RequestFailure); ok {
		if err.Code() == "InvalidObjectState" {
			return nil, errors.Errorf("Object in GLACIER, restore first: bucket=%q, key=%q", bucket, bucketPath)
		}
	}
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

var warnStreamUpload sync.Once

func (o *Object) uploadMultipart(ctx context.Context, req *s3.PutObjectInput, size int64, in io.Reader) (err error) {
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
	partSize := int(f.opt.ChunkSize)

	// size can be -1 here meaning we don't know the size of the incoming file.  We use ChunkSize
	// buffers here (default 5MB). With a maximum number of parts (10,000) this will be a file of
	// 48GB which seems like a not too unreasonable limit.
	if size == -1 {
		warnStreamUpload.Do(func() {
			fs.Logf(f, "Streaming uploads using chunk size %v will have maximum file size of %v",
				f.opt.ChunkSize, fs.SizeSuffix(int64(partSize)*uploadParts))
		})
	} else {
		// Adjust partSize until the number of parts is small enough.
		if size/int64(partSize) >= uploadParts {
			// Calculate partition size rounded up to the nearest MB
			partSize = int((((size / uploadParts) >> 20) + 1) << 20)
		}
	}

	memPool := f.getMemoryPool(int64(partSize))

	var mReq s3.CreateMultipartUploadInput
	structs.SetFrom(&mReq, req)
	var cout *s3.CreateMultipartUploadOutput
	err = f.pacer.Call(func() (bool, error) {
		var err error
		cout, err = f.c.CreateMultipartUploadWithContext(ctx, &mReq)
		return f.shouldRetry(err)
	})
	if err != nil {
		return errors.Wrap(err, "multipart upload failed to initialise")
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
			return f.shouldRetry(err)
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
	)

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
			return errors.Wrap(err, "multipart upload failed to read source")
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
						return f.shouldRetry(err)
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
				return errors.Wrap(err, "multipart upload failed to upload part")
			}
			return nil
		})
	}
	err = g.Wait()
	if err != nil {
		return err
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
		return f.shouldRetry(err)
	})
	if err != nil {
		return errors.Wrap(err, "multipart upload failed to finalise")
	}
	return nil
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
	// - for non multpart
	//    - so we can add a ContentMD5
	// - for multipart provided checksums aren't disabled
	//    - so we can add the md5sum in the metadata as metaMD5Hash
	var md5sum string
	if !multipart || !o.fs.opt.DisableChecksum {
		hash, err := src.Hash(ctx, hash.MD5)
		if err == nil && matchMd5.MatchString(hash) {
			hashBytes, err := hex.DecodeString(hash)
			if err == nil {
				md5sum = base64.StdEncoding.EncodeToString(hashBytes)
				if multipart {
					metadata[metaMD5Hash] = &md5sum
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
	if md5sum != "" {
		req.ContentMD5 = &md5sum
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

	if multipart {
		err = o.uploadMultipart(ctx, &req, size, in)
		if err != nil {
			return err
		}
	} else {

		// Create the request
		putObj, _ := o.fs.c.PutObjectRequest(&req)

		// Sign it so we can upload using a presigned request.
		//
		// Note the SDK doesn't currently support streaming to
		// PutObject so we'll use this work-around.
		url, headers, err := putObj.PresignRequest(15 * time.Minute)
		if err != nil {
			return errors.Wrap(err, "s3 upload: sign request")
		}

		if o.fs.opt.V2Auth && headers == nil {
			headers = putObj.HTTPRequest.Header
		}

		// Set request to nil if empty so as not to make chunked encoding
		if size == 0 {
			in = nil
		}

		// create the vanilla http request
		httpReq, err := http.NewRequest("PUT", url, in)
		if err != nil {
			return errors.Wrap(err, "s3 upload: new request")
		}
		httpReq = httpReq.WithContext(ctx) // go1.13 can use NewRequestWithContext

		// set the headers we signed and the length
		httpReq.Header = headers
		httpReq.ContentLength = size

		err = o.fs.pacer.CallNoRetry(func() (bool, error) {
			resp, err := o.fs.srv.Do(httpReq)
			if err != nil {
				return o.fs.shouldRetry(err)
			}
			body, err := rest.ReadBody(resp)
			if err != nil {
				return o.fs.shouldRetry(err)
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 299 {
				return false, nil
			}
			err = errors.Errorf("s3 upload: %s: %s", resp.Status, body)
			return fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
		})
		if err != nil {
			return err
		}
	}

	// Read the metadata from the newly created object
	o.meta = nil // wipe old metadata
	err = o.readMetaData(ctx)
	return err
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	bucket, bucketPath := o.split()
	req := s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &bucketPath,
	}
	err := o.fs.pacer.Call(func() (bool, error) {
		_, err := o.fs.c.DeleteObjectWithContext(ctx, &req)
		return o.fs.shouldRetry(err)
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
	err = o.fs.copy(ctx, &req, bucket, bucketPath, bucket, bucketPath, o.bytes)
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
	_ fs.Object      = &Object{}
	_ fs.MimeTyper   = &Object{}
	_ fs.GetTierer   = &Object{}
	_ fs.SetTierer   = &Object{}
)
