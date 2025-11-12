package s3

import (
	"embed"
	stdfs "io/fs"
	"os"
	"sort"
	"strings"

	"github.com/rclone/rclone/fs"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"gopkg.in/yaml.v3"
)

// YamlMap is converted to YAML in the correct order
type YamlMap = *orderedmap.OrderedMap[string, string]

// NewYamlMap creates a new ordered map
var NewYamlMap = orderedmap.New[string, string]

// Quirks defines all the S3 provider quirks
type Quirks struct {
	ListVersion                 *int   `yaml:"list_version,omitempty"`     // 1 or 2
	ForcePathStyle              *bool  `yaml:"force_path_style,omitempty"` // true = path-style
	ListURLEncode               *bool  `yaml:"list_url_encode,omitempty"`
	UseMultipartEtag            *bool  `yaml:"use_multipart_etag,omitempty"`
	UseAlreadyExists            *bool  `yaml:"use_already_exists,omitempty"`
	UseAcceptEncodingGzip       *bool  `yaml:"use_accept_encoding_gzip,omitempty"`
	UseDataIntegrityProtections *bool  `yaml:"use_data_integrity_protections,omitempty"`
	MightGzip                   *bool  `yaml:"might_gzip,omitempty"`
	UseMultipartUploads         *bool  `yaml:"use_multipart_uploads,omitempty"`
	UseUnsignedPayload          *bool  `yaml:"use_unsigned_payload,omitempty"`
	UseXID                      *bool  `yaml:"use_x_id,omitempty"`
	SignAcceptEncoding          *bool  `yaml:"sign_accept_encoding,omitempty"`
	CopyCutoff                  *int64 `yaml:"copy_cutoff,omitempty"`
	MaxUploadParts              *int   `yaml:"max_upload_parts,omitempty"`
	MinChunkSize                *int64 `yaml:"min_chunk_size,omitempty"`
}

// Provider defines the configurable data in each provider.yaml
type Provider struct {
	Name                 string  `yaml:"name,omitempty"`
	Description          string  `yaml:"description,omitempty"`
	Region               YamlMap `yaml:"region,omitempty"`
	Endpoint             YamlMap `yaml:"endpoint,omitempty"`
	LocationConstraint   YamlMap `yaml:"location_constraint,omitempty"`
	ACL                  YamlMap `yaml:"acl,omitempty"`
	StorageClass         YamlMap `yaml:"storage_class,omitempty"`
	ServerSideEncryption YamlMap `yaml:"server_side_encryption,omitempty"`

	// other
	IBMApiKey             bool `yaml:"ibm_api_key,omitempty"`
	IBMResourceInstanceID bool `yaml:"ibm_resource_instance_id,omitempty"`

	// advanced
	BucketACL             bool `yaml:"bucket_acl,omitempty"`
	DirectoryBucket       bool `yaml:"directory_bucket,omitempty"`
	LeavePartsOnError     bool `yaml:"leave_parts_on_error,omitempty"`
	RequesterPays         bool `yaml:"requester_pays,omitempty"`
	SSECustomerAlgorithm  bool `yaml:"sse_customer_algorithm,omitempty"`
	SSECustomerKey        bool `yaml:"sse_customer_key,omitempty"`
	SSECustomerKeyBase64  bool `yaml:"sse_customer_key_base64,omitempty"`
	SSECustomerKeyMd5     bool `yaml:"sse_customer_key_md5,omitempty"`
	SSEKmsKeyID           bool `yaml:"sse_kms_key_id,omitempty"`
	STSEndpoint           bool `yaml:"sts_endpoint,omitempty"`
	UseAccelerateEndpoint bool `yaml:"use_accelerate_endpoint,omitempty"`

	Quirks Quirks `yaml:"quirks,omitempty"`
}

//go:embed provider/*.yaml
var providerFS embed.FS

// addProvidersToInfo adds provider information to the fs.RegInfo
func addProvidersToInfo(info *fs.RegInfo) *fs.RegInfo {
	providerMap := loadProviders()
	providerList := constructProviders(info.Options, providerMap)
	info.Description += strings.TrimSuffix(providerList, ", ")
	return info
}

// loadProvider loads a single provider
//
// It returns nil if it could not be found except if "Other" which is a fatal error.
func loadProvider(name string) *Provider {
	data, err := stdfs.ReadFile(providerFS, "provider/"+name+".yaml")
	if err != nil {
		if os.IsNotExist(err) && name != "Other" {
			return nil
		}
		fs.Fatalf(nil, "internal error: failed to load provider %q: %v", name, err)
	}
	var p Provider
	err = yaml.Unmarshal(data, &p)
	if err != nil {
		fs.Fatalf(nil, "internal error: failed to unmarshal provider %q: %v", name, err)
	}
	return &p
}

// loadProviders loads provider definitions from embedded YAML files
func loadProviders() map[string]*Provider {
	providers, err := stdfs.ReadDir(providerFS, "provider")
	if err != nil {
		fs.Fatalf(nil, "internal error: failed to read embedded providers: %v", err)
	}
	providerMap := make(map[string]*Provider, len(providers))

	for _, provider := range providers {
		name, _ := strings.CutSuffix(provider.Name(), ".yaml")
		p := loadProvider(name)
		providerMap[p.Name] = p
	}
	return providerMap
}

// constructProviders populates fs.Options with provider-specific examples and information
func constructProviders(options fs.Options, providerMap map[string]*Provider) string {
	// Defaults for map options set to {}
	defaults := providerMap["Other"]

	// sort providers: AWS first, Other last, rest alphabetically
	providers := make([]*Provider, 0, len(providerMap))
	for _, p := range providerMap {
		providers = append(providers, p)
	}
	sort.Slice(providers, func(i, j int) bool {
		if providers[i].Name == "AWS" {
			return true
		}
		if providers[j].Name == "AWS" {
			return false
		}
		if providers[i].Name == "Other" {
			return false
		}
		if providers[j].Name == "Other" {
			return true
		}
		return strings.ToLower(providers[i].Name) < strings.ToLower(providers[j].Name)
	})

	addProvider := func(sp *string, name string) {
		if *sp != "" {
			*sp += ","
		}
		*sp += name
	}

	addBool := func(opt *fs.Option, p *Provider, flag bool) {
		if flag {
			addProvider(&opt.Provider, p.Name)
		}
	}

	addExample := func(opt *fs.Option, p *Provider, examples, defaultExamples YamlMap) {
		if examples == nil {
			return
		}
		if examples.Len() == 0 {
			examples = defaultExamples
		}
		addProvider(&opt.Provider, p.Name)
	OUTER:
		for pair := examples.Oldest(); pair != nil; pair = pair.Next() {
			// Find an existing example to add to if possible
			for i, example := range opt.Examples {
				if example.Value == pair.Key && example.Help == pair.Value {
					addProvider(&opt.Examples[i].Provider, p.Name)
					continue OUTER
				}
			}
			// Otherwise add a new one
			opt.Examples = append(opt.Examples, fs.OptionExample{
				Value:    pair.Key,
				Help:     pair.Value,
				Provider: p.Name,
			})
		}
	}

	var providerList strings.Builder

	for _, p := range providers {
		for i := range options {
			opt := &options[i]
			switch opt.Name {
			case "provider":
				opt.Examples = append(opt.Examples, fs.OptionExample{
					Value: p.Name,
					Help:  p.Description,
				})
				providerList.WriteString(p.Name + ", ")
			case "region":
				addExample(opt, p, p.Region, defaults.Region)
			case "endpoint":
				addExample(opt, p, p.Endpoint, defaults.Endpoint)
			case "location_constraint":
				addExample(opt, p, p.LocationConstraint, defaults.LocationConstraint)
			case "acl":
				addExample(opt, p, p.ACL, defaults.ACL)
			case "storage_class":
				addExample(opt, p, p.StorageClass, defaults.StorageClass)
			case "server_side_encryption":
				addExample(opt, p, p.ServerSideEncryption, defaults.ServerSideEncryption)
			case "bucket_acl":
				addBool(opt, p, p.BucketACL)
			case "requester_pays":
				addBool(opt, p, p.RequesterPays)
			case "sse_customer_algorithm":
				addBool(opt, p, p.SSECustomerAlgorithm)
			case "sse_kms_key_id":
				addBool(opt, p, p.SSEKmsKeyID)
			case "sse_customer_key":
				addBool(opt, p, p.SSECustomerKey)
			case "sse_customer_key_base64":
				addBool(opt, p, p.SSECustomerKeyBase64)
			case "sse_customer_key_md5":
				addBool(opt, p, p.SSECustomerKeyMd5)
			case "directory_bucket":
				addBool(opt, p, p.DirectoryBucket)
			case "ibm_api_key":
				addBool(opt, p, p.IBMApiKey)
			case "ibm_resource_instance_id":
				addBool(opt, p, p.IBMResourceInstanceID)
			case "leave_parts_on_error":
				addBool(opt, p, p.LeavePartsOnError)
			case "sts_endpoint":
				addBool(opt, p, p.STSEndpoint)
			case "use_accelerate_endpoint":
				addBool(opt, p, p.UseAccelerateEndpoint)
			}
		}
	}

	return strings.TrimSuffix(providerList.String(), ", ")
}
