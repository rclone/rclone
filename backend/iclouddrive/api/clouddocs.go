package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

// CloudDocs is a thin CloudKit (ckdatabasews) client for the com.apple.clouddocs
// container, shared database. It is used for operations that the drivews/docws
// endpoints cannot perform — most importantly placing a document inside a folder
// shared by another Apple ID (a FOLDER_IN_SHARED_FOLDER), which is only reachable
// through the owner's CloudKit zone.
//
// The drivews/docws layer returns HTTP 400 for any write whose parent is a
// FOLDER_IN_SHARED_FOLDER. The workaround implemented here is:
//
//  1. upload the document into the share ROOT the normal way (drivews performs the
//     Protected Cloud Storage chaining server-side, because the share root is an
//     addressable SHARED_FOLDER);
//  2. re-parent the resulting record into the target sub-folder via a CloudKit
//     records/modify "update". The server re-chains PCS itself, because the record
//     already carries its own PCS key — so no client-side PCS crypto is required.
//
// Every request goes through the backend pacer (rate-limit / retry), and re-auths
// on a 401/421/423 once, exactly like the rest of the api package.
type CloudDocs struct {
	icloud      *Client
	url         string
	zone        *CKZoneID
	pacer       *fs.Pacer
	shouldRetry ShouldRetryFunc
}

// CKZoneID identifies a CloudKit zone.
type CKZoneID struct {
	ZoneName        string `json:"zoneName"`
	OwnerRecordName string `json:"ownerRecordName"`
	ZoneType        string `json:"zoneType"`
}

// CloudDocsService returns a CloudDocs client, or nil if the account has no
// ckdatabasews web service (in which case shared-subfolder writes are unsupported).
// The pacer and shouldRetry are threaded through so every ckdatabasews request is
// rate-limited and retried like the rest of the package.
func (c *Client) CloudDocsService(pacer *fs.Pacer, shouldRetry ShouldRetryFunc) *CloudDocs {
	ws := c.Session.AccountInfo.Webservices["ckdatabasews"]
	if ws == nil || ws.URL == "" {
		return nil
	}
	return &CloudDocs{icloud: c, url: ws.URL, pacer: pacer, shouldRetry: shouldRetry}
}

// request POSTs body to the shared ckdatabasews endpoint sub (e.g. "records/modify")
// and decodes the JSON reply into response. The call is rate-limited and retried by
// the pacer, and re-authenticates once on a 401/421/423.
func (cd *CloudDocs) request(ctx context.Context, sub string, body, response any) error {
	rootURL := fmt.Sprintf("%s/database/1/com.apple.clouddocs/production/shared/%s?remapEnums=true&getCurrentSyncToken=true", cd.url, sub)
	reauthDone := false
	return cd.pacer.Call(func() (bool, error) {
		// The body reader is consumed on each attempt, so rebuild it every time.
		reader, err := IntoReader(body)
		if err != nil {
			return false, err
		}
		opts := rest.Opts{
			Method:       "POST",
			RootURL:      rootURL,
			ExtraHeaders: cd.icloud.Session.GetHeaders(map[string]string{"Content-Type": "text/plain"}),
			Body:         reader,
		}
		resp, err := cd.icloud.Session.Request(ctx, opts, nil, response)
		if !reauthDone && err != nil && resp != nil && (resp.StatusCode == 401 || resp.StatusCode == 421 || resp.StatusCode == 423) {
			reauthDone = true
			if authErr := cd.icloud.Authenticate(ctx); authErr != nil {
				return false, authErr
			}
			if cd.icloud.Session.Requires2FA() {
				return false, errors.New("trust token expired, please reauth")
			}
			if reader, err = IntoReader(body); err != nil {
				return false, err
			}
			opts.Body = reader
			resp, err = cd.icloud.Session.Request(ctx, opts, nil, response)
		}
		return cd.shouldRetry(ctx, resp, err)
	})
}

// ckZonesListResponse is the reply to zones/list.
type ckZonesListResponse struct {
	Zones []struct {
		ZoneID CKZoneID `json:"zoneID"`
	} `json:"zones"`
}

// Zone returns the shared CloudDocs zone, discovering and caching it on first use
// via zones/list. It prefers the zone named defaultZone; if the account
// participates in several shared zones and none carries that name, it falls back to
// the first and logs the ambiguity.
func (cd *CloudDocs) Zone(ctx context.Context) (*CKZoneID, error) {
	if cd.zone != nil {
		return cd.zone, nil
	}
	var out ckZonesListResponse
	if err := cd.request(ctx, "zones/list", struct{}{}, &out); err != nil {
		return nil, err
	}
	for i := range out.Zones {
		if out.Zones[i].ZoneID.ZoneName == defaultZone {
			cd.zone = &out.Zones[i].ZoneID
			return cd.zone, nil
		}
	}
	if len(out.Zones) == 0 {
		return nil, errors.New("no shared clouddocs zone found")
	}
	if len(out.Zones) > 1 {
		names := make([]string, len(out.Zones))
		for i := range out.Zones {
			names[i] = out.Zones[i].ZoneID.ZoneName
		}
		fs.Logf(nil, "iclouddrive: %d shared CloudDocs zones %v and none named %q; using first (%q)",
			len(out.Zones), names, defaultZone, out.Zones[0].ZoneID.ZoneName)
	}
	cd.zone = &out.Zones[0].ZoneID
	return cd.zone, nil
}

// ckRecordRef names a single record, for records/lookup.
type ckRecordRef struct {
	RecordName string `json:"recordName"`
}

// ckLookupRequest is the body of a records/lookup request.
type ckLookupRequest struct {
	ZoneID  *CKZoneID     `json:"zoneID"`
	Records []ckRecordRef `json:"records"`
}

// ckRecord is the subset of a CloudKit record we read back.
type ckRecord struct {
	RecordName      string `json:"recordName"`
	RecordChangeTag string `json:"recordChangeTag"`
	Reason          string `json:"reason"`
	ServerErrorCode string `json:"serverErrorCode"`
	Deleted         bool   `json:"deleted"`
}

// ckRecordsResponse is the reply to records/lookup and records/modify.
type ckRecordsResponse struct {
	Records []ckRecord `json:"records"`
}

func (cd *CloudDocs) lookup(ctx context.Context, recordNames ...string) ([]ckRecord, error) {
	zone, err := cd.Zone(ctx)
	if err != nil {
		return nil, err
	}
	refs := make([]ckRecordRef, 0, len(recordNames))
	for _, rn := range recordNames {
		refs = append(refs, ckRecordRef{RecordName: rn})
	}
	var out ckRecordsResponse
	if err := cd.request(ctx, "records/lookup", ckLookupRequest{ZoneID: zone, Records: refs}, &out); err != nil {
		return nil, err
	}
	return out.Records, nil
}

// ckReference is a CloudKit record reference, used both as a field value (with an
// action) and as a record's parent pointer (recordName only).
type ckReference struct {
	RecordName string    `json:"recordName"`
	Action     string    `json:"action,omitempty"`
	ZoneID     *CKZoneID `json:"zoneID,omitempty"`
}

// ckField is a typed CloudKit field value.
type ckField struct {
	Value any    `json:"value"`
	Type  string `json:"type,omitempty"`
}

// ckRecordModify is a record carried by a records/modify operation. Only the fields
// relevant to the operation are populated (delete needs just the name + change tag).
type ckRecordModify struct {
	RecordName      string             `json:"recordName"`
	RecordType      string             `json:"recordType,omitempty"`
	RecordChangeTag string             `json:"recordChangeTag,omitempty"`
	Fields          map[string]ckField `json:"fields,omitempty"`
	Parent          *ckReference       `json:"parent,omitempty"`
}

// ckOperation is a single operation within a records/modify request.
type ckOperation struct {
	OperationType string         `json:"operationType"`
	Record        ckRecordModify `json:"record"`
}

// ckModifyRequest is the body of a records/modify request.
type ckModifyRequest struct {
	Atomic     bool          `json:"atomic"`
	ZoneID     *CKZoneID     `json:"zoneID"`
	Operations []ckOperation `json:"operations"`
}

// ReparentStructure moves the documentStructure/<uuid> record under a new parent
// directory/<targetDirUUID>, within the shared zone. The matching
// documentContent/<uuid> follows automatically (its parent is the structure).
//
// uuid is the bare record UUID (e.g. the docwsid of the just-uploaded file, which
// CloudDocs reuses as the structure/content record name). The current
// recordChangeTag is fetched automatically.
func (cd *CloudDocs) ReparentStructure(ctx context.Context, uuid, targetDirUUID string) error {
	zone, err := cd.Zone(ctx)
	if err != nil {
		return err
	}
	structRec := "documentStructure/" + strings.ToUpper(uuid)
	recs, err := cd.lookup(ctx, structRec)
	if err != nil {
		return err
	}
	if len(recs) == 0 || recs[0].RecordChangeTag == "" {
		reason := "not found"
		if len(recs) > 0 && recs[0].Reason != "" {
			reason = recs[0].Reason
		}
		return fmt.Errorf("clouddocs: could not look up %s: %s", structRec, reason)
	}
	dirRecordName := "directory/" + strings.ToUpper(targetDirUUID)
	body := ckModifyRequest{
		Atomic: true,
		ZoneID: zone,
		Operations: []ckOperation{{
			OperationType: "update",
			Record: ckRecordModify{
				RecordName:      structRec,
				RecordType:      "structure",
				RecordChangeTag: recs[0].RecordChangeTag,
				Fields: map[string]ckField{
					"parent": {
						Type: "REFERENCE",
						Value: ckReference{
							RecordName: dirRecordName,
							Action:     "VALIDATE",
							ZoneID:     zone,
						},
					},
				},
				Parent: &ckReference{RecordName: dirRecordName},
			},
		}},
	}
	var out ckRecordsResponse
	if err := cd.request(ctx, "records/modify", body, &out); err != nil {
		return err
	}
	return checkRecordErrors("clouddocs re-parent", out.Records)
}

// documentRecordNames returns the documentContent and documentStructure record
// names for a bare file UUID. CloudDocs reuses the same UUID (upper-cased) for both
// halves of a document.
func documentRecordNames(uuid string) (content, structure string) {
	uuid = strings.ToUpper(uuid)
	return "documentContent/" + uuid, "documentStructure/" + uuid
}

// directoryRecordName returns the directory record name for a bare folder UUID.
func directoryRecordName(uuid string) string {
	return "directory/" + strings.ToUpper(uuid)
}

// deleteRecordOperations builds the records/modify "delete" operations for a
// document's content and structure records, attaching each record's current change
// tag (a missing tag is sent empty, which the server tolerates for a delete).
func deleteRecordOperations(contentRec, structRec string, tags map[string]string) []ckOperation {
	ops := make([]ckOperation, 0, 2)
	for _, rn := range []string{contentRec, structRec} {
		ops = append(ops, ckOperation{
			OperationType: "delete",
			Record:        ckRecordModify{RecordName: rn, RecordChangeTag: tags[rn]},
		})
	}
	return ops
}

func deleteDirectoryRecordOperation(directoryRec string, tags map[string]string) []ckOperation {
	return []ckOperation{{
		OperationType: "delete",
		Record:        ckRecordModify{RecordName: directoryRec, RecordChangeTag: tags[directoryRec]},
	}}
}

func lookupRecordTags(records []ckRecord) map[string]string {
	tags := map[string]string{}
	for _, r := range records {
		tags[r.RecordName] = r.RecordChangeTag
	}
	return tags
}

func checkRecordErrors(operation string, records []ckRecord) error {
	for _, r := range records {
		if r.ServerErrorCode != "" {
			return fmt.Errorf("%s failed for %s: %s (%s)", operation, r.RecordName, r.Reason, r.ServerErrorCode)
		}
	}
	return nil
}

// DeleteFile deletes the documentStructure/documentContent pair for the given
// record UUID from the shared zone. Used to remove or overwrite files that live
// inside a shared sub-folder, which the drivews trash endpoint cannot touch.
func (cd *CloudDocs) DeleteFile(ctx context.Context, uuid string) error {
	zone, err := cd.Zone(ctx)
	if err != nil {
		return err
	}
	contentRec, structRec := documentRecordNames(uuid)
	recs, err := cd.lookup(ctx, contentRec, structRec)
	if err != nil {
		return err
	}
	ops := deleteRecordOperations(contentRec, structRec, lookupRecordTags(recs))
	var out ckRecordsResponse
	if err := cd.request(ctx, "records/modify", ckModifyRequest{Atomic: false, ZoneID: zone, Operations: ops}, &out); err != nil {
		return err
	}
	return checkRecordErrors("clouddocs delete file", out.Records)
}

// DeleteDirectory deletes an empty directory/<uuid> record from the shared zone.
// It is used for rmdir of shared sub-directories; callers must verify emptiness
// first because this is not a recursive purge operation.
func (cd *CloudDocs) DeleteDirectory(ctx context.Context, uuid string) error {
	zone, err := cd.Zone(ctx)
	if err != nil {
		return err
	}
	dirRec := directoryRecordName(uuid)
	recs, err := cd.lookup(ctx, dirRec)
	if err != nil {
		return err
	}
	ops := deleteDirectoryRecordOperation(dirRec, lookupRecordTags(recs))
	var out ckRecordsResponse
	if err := cd.request(ctx, "records/modify", ckModifyRequest{Atomic: false, ZoneID: zone, Operations: ops}, &out); err != nil {
		return err
	}
	return checkRecordErrors("clouddocs delete directory", out.Records)
}

// ckZoneChangesRequestZone names a zone (and an optional resume token) in a
// changes/zone request.
type ckZoneChangesRequestZone struct {
	ZoneID    *CKZoneID `json:"zoneID"`
	SyncToken string    `json:"syncToken,omitempty"`
}

// ckZoneChangesRequest is the body of a changes/zone request.
type ckZoneChangesRequest struct {
	Zones []ckZoneChangesRequestZone `json:"zones"`
}

// ckZoneChangesResponse is the reply to changes/zone.
type ckZoneChangesResponse struct {
	Zones []struct {
		Records []struct {
			RecordName string `json:"recordName"`
			RecordType string `json:"recordType"`
			Fields     map[string]struct {
				Value any `json:"value"`
			} `json:"fields"`
		} `json:"records"`
		SyncToken  string `json:"syncToken"`
		MoreComing bool   `json:"moreComing"`
	} `json:"zones"`
}

// documentBaseHash returns the CloudKit "basehash" field value for a file: the
// base64 of SHA256 over the basename with its extension stripped. This is how a
// documentStructure records the file's name, and how FindFileUUID matches it.
func documentBaseHash(leaf string) string {
	basename := leaf
	if i := strings.LastIndexByte(leaf, '.'); i > 0 {
		basename = leaf[:i]
	}
	sum := sha256.Sum256([]byte(basename))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// FindFileUUID looks up the record UUID of the file named leaf inside the shared
// directory directory/<dirUUID>. CloudDocs record types are not query-indexable, so
// it enumerates the zone (changes/zone) and matches a documentStructure whose parent
// is that directory and whose basehash equals SHA256(basename-without-ext). Returns
// "" if not found.
func (cd *CloudDocs) FindFileUUID(ctx context.Context, dirUUID, leaf string) (string, error) {
	zone, err := cd.Zone(ctx)
	if err != nil {
		return "", err
	}
	wantHash := documentBaseHash(leaf)
	wantParent := "directory/" + strings.ToUpper(dirUUID)

	syncToken := ""
	for {
		zoneReq := ckZoneChangesRequestZone{ZoneID: zone, SyncToken: syncToken}
		var out ckZoneChangesResponse
		if err := cd.request(ctx, "changes/zone", ckZoneChangesRequest{Zones: []ckZoneChangesRequestZone{zoneReq}}, &out); err != nil {
			return "", err
		}
		if len(out.Zones) == 0 {
			return "", nil
		}
		z := out.Zones[0]
		for _, r := range z.Records {
			if r.RecordType != "structure" || !strings.HasPrefix(r.RecordName, "documentStructure/") {
				continue
			}
			bh, _ := r.Fields["basehash"].Value.(string)
			if bh != wantHash {
				continue
			}
			if pv, ok := r.Fields["parent"].Value.(map[string]any); ok {
				if rn, _ := pv["recordName"].(string); rn == wantParent {
					return strings.TrimPrefix(r.RecordName, "documentStructure/"), nil
				}
			}
		}
		// Stop when the server says there is nothing more, or when it claims more is
		// coming but the resume token did not advance (empty or unchanged) — otherwise
		// we would re-request the same page forever.
		if !z.MoreComing {
			return "", nil
		}
		if z.SyncToken == "" || z.SyncToken == syncToken {
			fs.Debugf(nil, "iclouddrive: clouddocs changes/zone set moreComing but syncToken did not advance; stopping enumeration")
			return "", nil
		}
		syncToken = z.SyncToken
	}
}
