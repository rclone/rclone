package azblob

import (
	"net"
	"net/url"
	"strings"
	"time"
)

// SASVersion indicates the SAS version.
const SASVersion = ServiceVersion

type SASProtocol string

const (
	// SASProtocolHTTPS can be specified for a SAS protocol
	SASProtocolHTTPS SASProtocol = "https"

	// SASProtocolHTTPSandHTTP can be specified for a SAS protocol
	SASProtocolHTTPSandHTTP SASProtocol = "https,http"
)

// FormatTimesForSASSigning converts a time.Time to a snapshotTimeFormat string suitable for a
// SASField's StartTime or ExpiryTime fields. Returns "" if value.IsZero().
func FormatTimesForSASSigning(startTime, expiryTime time.Time) (string, string) {
	ss := ""
	if !startTime.IsZero() {
		ss = startTime.Format(SASTimeFormat) // "yyyy-MM-ddTHH:mm:ssZ"
	}
	se := ""
	if !expiryTime.IsZero() {
		se = expiryTime.Format(SASTimeFormat) // "yyyy-MM-ddTHH:mm:ssZ"
	}
	return ss, se
}

// SASTimeFormat represents the format of a SAS start or expiry time. Use it when formatting/parsing a time.Time.
const SASTimeFormat = "2006-01-02T15:04:05Z" //"2017-07-27T00:00:00Z" // ISO 8601

// https://docs.microsoft.com/en-us/rest/api/storageservices/constructing-a-service-sas

// A SASQueryParameters object represents the components that make up an Azure Storage SAS' query parameters.
// You parse a map of query parameters into its fields by calling NewSASQueryParameters(). You add the components
// to a query parameter map by calling AddToValues().
// NOTE: Changing any field requires computing a new SAS signature using a XxxSASSignatureValues type.
//
// This type defines the components used by all Azure Storage resources (Containers, Blobs, Files, & Queues).
type SASQueryParameters struct {
	// All members are immutable or values so copies of this struct are goroutine-safe.
	version       string      `param:"sv"`
	services      string      `param:"ss"`
	resourceTypes string      `param:"srt"`
	protocol      SASProtocol `param:"spr"`
	startTime     time.Time   `param:"st"`
	expiryTime    time.Time   `param:"se"`
	ipRange       IPRange     `param:"sip"`
	identifier    string      `param:"si"`
	resource      string      `param:"sr"`
	permissions   string      `param:"sp"`
	signature     string      `param:"sig"`
}

func (p *SASQueryParameters) Version() string {
	return p.version
}

func (p *SASQueryParameters) Services() string {
	return p.services
}
func (p *SASQueryParameters) ResourceTypes() string {
	return p.resourceTypes
}
func (p *SASQueryParameters) Protocol() SASProtocol {
	return p.protocol
}
func (p *SASQueryParameters) StartTime() time.Time {
	return p.startTime
}
func (p *SASQueryParameters) ExpiryTime() time.Time {
	return p.expiryTime
}

func (p *SASQueryParameters) IPRange() IPRange {
	return p.ipRange
}

func (p *SASQueryParameters) Identifier() string {
	return p.identifier
}

func (p *SASQueryParameters) Resource() string {
	return p.resource
}
func (p *SASQueryParameters) Permissions() string {
	return p.permissions
}

func (p *SASQueryParameters) Signature() string {
	return p.signature
}

// IPRange represents a SAS IP range's start IP and (optionally) end IP.
type IPRange struct {
	Start net.IP // Not specified if length = 0
	End   net.IP // Not specified if length = 0
}

// String returns a string representation of an IPRange.
func (ipr *IPRange) String() string {
	if len(ipr.Start) == 0 {
		return ""
	}
	start := ipr.Start.String()
	if len(ipr.End) == 0 {
		return start
	}
	return start + "-" + ipr.End.String()
}

// NewSASQueryParameters creates and initializes a SASQueryParameters object based on the
// query parameter map's passed-in values. If deleteSASParametersFromValues is true,
// all SAS-related query parameters are removed from the passed-in map. If
// deleteSASParametersFromValues is false, the map passed-in map is unaltered.
func newSASQueryParameters(values url.Values, deleteSASParametersFromValues bool) SASQueryParameters {
	p := SASQueryParameters{}
	for k, v := range values {
		val := v[0]
		isSASKey := true
		switch strings.ToLower(k) {
		case "sv":
			p.version = val
		case "ss":
			p.services = val
		case "srt":
			p.resourceTypes = val
		case "spr":
			p.protocol = SASProtocol(val)
		case "st":
			p.startTime, _ = time.Parse(SASTimeFormat, val)
		case "se":
			p.expiryTime, _ = time.Parse(SASTimeFormat, val)
		case "sip":
			dashIndex := strings.Index(val, "-")
			if dashIndex == -1 {
				p.ipRange.Start = net.ParseIP(val)
			} else {
				p.ipRange.Start = net.ParseIP(val[:dashIndex])
				p.ipRange.End = net.ParseIP(val[dashIndex+1:])
			}
		case "si":
			p.identifier = val
		case "sr":
			p.resource = val
		case "sp":
			p.permissions = val
		case "sig":
			p.signature = val
		default:
			isSASKey = false // We didn't recognize the query parameter
		}
		if isSASKey && deleteSASParametersFromValues {
			delete(values, k)
		}
	}
	return p
}

// AddToValues adds the SAS components to the specified query parameters map.
func (p *SASQueryParameters) addToValues(v url.Values) url.Values {
	if p.version != "" {
		v.Add("sv", p.version)
	}
	if p.services != "" {
		v.Add("ss", p.services)
	}
	if p.resourceTypes != "" {
		v.Add("srt", p.resourceTypes)
	}
	if p.protocol != "" {
		v.Add("spr", string(p.protocol))
	}
	if !p.startTime.IsZero() {
		v.Add("st", p.startTime.Format(SASTimeFormat))
	}
	if !p.expiryTime.IsZero() {
		v.Add("se", p.expiryTime.Format(SASTimeFormat))
	}
	if len(p.ipRange.Start) > 0 {
		v.Add("sip", p.ipRange.String())
	}
	if p.identifier != "" {
		v.Add("si", p.identifier)
	}
	if p.resource != "" {
		v.Add("sr", p.resource)
	}
	if p.permissions != "" {
		v.Add("sp", p.permissions)
	}
	if p.signature != "" {
		v.Add("sig", p.signature)
	}
	return v
}

// Encode encodes the SAS query parameters into URL encoded form sorted by key.
func (p *SASQueryParameters) Encode() string {
	v := url.Values{}
	p.addToValues(v)
	return v.Encode()
}
