package azblob

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

// BlobSASSignatureValues is used to generate a Shared Access Signature (SAS) for an Azure Storage container or blob.
// For more information, see https://docs.microsoft.com/rest/api/storageservices/constructing-a-service-sas
type BlobSASSignatureValues struct {
	Version            string      `param:"sv"`  // If not specified, this defaults to SASVersion
	Protocol           SASProtocol `param:"spr"` // See the SASProtocol* constants
	StartTime          time.Time   `param:"st"`  // Not specified if IsZero
	ExpiryTime         time.Time   `param:"se"`  // Not specified if IsZero
	SnapshotTime       time.Time
	Permissions        string  `param:"sp"` // Create by initializing a ContainerSASPermissions or BlobSASPermissions and then call String()
	IPRange            IPRange `param:"sip"`
	Identifier         string  `param:"si"`
	ContainerName      string
	BlobName           string // Use "" to create a Container SAS
	CacheControl       string // rscc
	ContentDisposition string // rscd
	ContentEncoding    string // rsce
	ContentLanguage    string // rscl
	ContentType        string // rsct
}

// NewSASQueryParameters uses an account's StorageAccountCredential to sign this signature values to produce
// the proper SAS query parameters.
// See: StorageAccountCredential. Compatible with both UserDelegationCredential and SharedKeyCredential
func (v BlobSASSignatureValues) NewSASQueryParameters(credential StorageAccountCredential) (SASQueryParameters, error) {
	resource := "c"
	if credential == nil {
		return SASQueryParameters{}, fmt.Errorf("cannot sign SAS query without StorageAccountCredential")
	}

	if !v.SnapshotTime.IsZero() {
		resource = "bs"
		//Make sure the permission characters are in the correct order
		perms := &BlobSASPermissions{}
		if err := perms.Parse(v.Permissions); err != nil {
			return SASQueryParameters{}, err
		}
		v.Permissions = perms.String()
	} else if v.BlobName == "" {
		// Make sure the permission characters are in the correct order
		perms := &ContainerSASPermissions{}
		if err := perms.Parse(v.Permissions); err != nil {
			return SASQueryParameters{}, err
		}
		v.Permissions = perms.String()
	} else {
		resource = "b"
		// Make sure the permission characters are in the correct order
		perms := &BlobSASPermissions{}
		if err := perms.Parse(v.Permissions); err != nil {
			return SASQueryParameters{}, err
		}
		v.Permissions = perms.String()
	}
	if v.Version == "" {
		v.Version = SASVersion
	}
	startTime, expiryTime, snapshotTime := FormatTimesForSASSigning(v.StartTime, v.ExpiryTime, v.SnapshotTime)

	signedIdentifier := v.Identifier

	udk := credential.getUDKParams()

	if udk != nil {
		udkStart, udkExpiry, _ := FormatTimesForSASSigning(udk.SignedStart, udk.SignedExpiry, time.Time{})
		//I don't like this answer to combining the functions
		//But because signedIdentifier and the user delegation key strings share a place, this is an _OK_ way to do it.
		signedIdentifier = strings.Join([]string{
			udk.SignedOid,
			udk.SignedTid,
			udkStart,
			udkExpiry,
			udk.SignedService,
			udk.SignedVersion,
		}, "\n")
	}

	// String to sign: http://msdn.microsoft.com/en-us/library/azure/dn140255.aspx
	stringToSign := strings.Join([]string{
		v.Permissions,
		startTime,
		expiryTime,
		getCanonicalName(credential.AccountName(), v.ContainerName, v.BlobName),
		signedIdentifier,
		v.IPRange.String(),
		string(v.Protocol),
		v.Version,
		resource,
		snapshotTime,         // signed timestamp
		v.CacheControl,       // rscc
		v.ContentDisposition, // rscd
		v.ContentEncoding,    // rsce
		v.ContentLanguage,    // rscl
		v.ContentType},       // rsct
		"\n")

	signature := ""
	signature = credential.ComputeHMACSHA256(stringToSign)

	p := SASQueryParameters{
		// Common SAS parameters
		version:     v.Version,
		protocol:    v.Protocol,
		startTime:   v.StartTime,
		expiryTime:  v.ExpiryTime,
		permissions: v.Permissions,
		ipRange:     v.IPRange,

		// Container/Blob-specific SAS parameters
		resource:           resource,
		identifier:         v.Identifier,
		cacheControl:       v.CacheControl,
		contentDisposition: v.ContentDisposition,
		contentEncoding:    v.ContentEncoding,
		contentLanguage:    v.ContentLanguage,
		contentType:        v.ContentType,
		snapshotTime:       v.SnapshotTime,

		// Calculated SAS signature
		signature: signature,
	}

	//User delegation SAS specific parameters
	if udk != nil {
		p.signedOid = udk.SignedOid
		p.signedTid = udk.SignedTid
		p.signedStart = udk.SignedStart
		p.signedExpiry = udk.SignedExpiry
		p.signedService = udk.SignedService
		p.signedVersion = udk.SignedVersion
	}

	return p, nil
}

// getCanonicalName computes the canonical name for a container or blob resource for SAS signing.
func getCanonicalName(account string, containerName string, blobName string) string {
	// Container: "/blob/account/containername"
	// Blob:      "/blob/account/containername/blobname"
	elements := []string{"/blob/", account, "/", containerName}
	if blobName != "" {
		elements = append(elements, "/", strings.Replace(blobName, "\\", "/", -1))
	}
	return strings.Join(elements, "")
}

// The ContainerSASPermissions type simplifies creating the permissions string for an Azure Storage container SAS.
// Initialize an instance of this type and then call its String method to set BlobSASSignatureValues's Permissions field.
type ContainerSASPermissions struct {
	Read, Add, Create, Write, Delete, List bool
}

// String produces the SAS permissions string for an Azure Storage container.
// Call this method to set BlobSASSignatureValues's Permissions field.
func (p ContainerSASPermissions) String() string {
	var b bytes.Buffer
	if p.Read {
		b.WriteRune('r')
	}
	if p.Add {
		b.WriteRune('a')
	}
	if p.Create {
		b.WriteRune('c')
	}
	if p.Write {
		b.WriteRune('w')
	}
	if p.Delete {
		b.WriteRune('d')
	}
	if p.List {
		b.WriteRune('l')
	}
	return b.String()
}

// Parse initializes the ContainerSASPermissions's fields from a string.
func (p *ContainerSASPermissions) Parse(s string) error {
	*p = ContainerSASPermissions{} // Clear the flags
	for _, r := range s {
		switch r {
		case 'r':
			p.Read = true
		case 'a':
			p.Add = true
		case 'c':
			p.Create = true
		case 'w':
			p.Write = true
		case 'd':
			p.Delete = true
		case 'l':
			p.List = true
		default:
			return fmt.Errorf("Invalid permission: '%v'", r)
		}
	}
	return nil
}

// The BlobSASPermissions type simplifies creating the permissions string for an Azure Storage blob SAS.
// Initialize an instance of this type and then call its String method to set BlobSASSignatureValues's Permissions field.
type BlobSASPermissions struct{ Read, Add, Create, Write, Delete bool }

// String produces the SAS permissions string for an Azure Storage blob.
// Call this method to set BlobSASSignatureValues's Permissions field.
func (p BlobSASPermissions) String() string {
	var b bytes.Buffer
	if p.Read {
		b.WriteRune('r')
	}
	if p.Add {
		b.WriteRune('a')
	}
	if p.Create {
		b.WriteRune('c')
	}
	if p.Write {
		b.WriteRune('w')
	}
	if p.Delete {
		b.WriteRune('d')
	}
	return b.String()
}

// Parse initializes the BlobSASPermissions's fields from a string.
func (p *BlobSASPermissions) Parse(s string) error {
	*p = BlobSASPermissions{} // Clear the flags
	for _, r := range s {
		switch r {
		case 'r':
			p.Read = true
		case 'a':
			p.Add = true
		case 'c':
			p.Create = true
		case 'w':
			p.Write = true
		case 'd':
			p.Delete = true
		default:
			return fmt.Errorf("Invalid permission: '%v'", r)
		}
	}
	return nil
}
