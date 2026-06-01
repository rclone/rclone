package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

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
type CloudDocs struct {
	icloud *Client
	url    string
	zone   *CKZoneID
}

// CKZoneID identifies a CloudKit zone.
type CKZoneID struct {
	ZoneName        string `json:"zoneName"`
	OwnerRecordName string `json:"ownerRecordName"`
	ZoneType        string `json:"zoneType"`
}

// CloudDocsService returns a CloudDocs client, or nil if the account has no
// ckdatabasews web service (in which case shared-subfolder writes are unsupported).
func (c *Client) CloudDocsService() *CloudDocs {
	ws := c.Session.AccountInfo.Webservices["ckdatabasews"]
	if ws == nil || ws.URL == "" {
		return nil
	}
	return &CloudDocs{icloud: c, url: ws.URL}
}

func (cd *CloudDocs) request(ctx context.Context, sub string, body, response any) (*http.Response, error) {
	rootURL := fmt.Sprintf("%s/database/1/com.apple.clouddocs/production/shared/%s?remapEnums=true&getCurrentSyncToken=true", cd.url, sub)
	reader, err := IntoReader(body)
	if err != nil {
		return nil, err
	}
	opts := rest.Opts{
		Method:       "POST",
		RootURL:      rootURL,
		ExtraHeaders: cd.icloud.Session.GetHeaders(map[string]string{"Content-Type": "text/plain"}),
		Body:         reader,
	}
	return cd.icloud.Request(ctx, opts, nil, response)
}

// Zone returns the (single) shared CloudDocs zone, discovering and caching it on
// first use via zones/list.
func (cd *CloudDocs) Zone(ctx context.Context) (*CKZoneID, error) {
	if cd.zone != nil {
		return cd.zone, nil
	}
	var out struct {
		Zones []struct {
			ZoneID CKZoneID `json:"zoneID"`
		} `json:"zones"`
	}
	if _, err := cd.request(ctx, "zones/list", map[string]any{}, &out); err != nil {
		return nil, err
	}
	for i := range out.Zones {
		if out.Zones[i].ZoneID.ZoneName == defaultZone {
			cd.zone = &out.Zones[i].ZoneID
			return cd.zone, nil
		}
	}
	if len(out.Zones) > 0 {
		cd.zone = &out.Zones[0].ZoneID
		return cd.zone, nil
	}
	return nil, fmt.Errorf("no shared clouddocs zone found")
}

// ckRecord is the subset of a CloudKit record we read back.
type ckRecord struct {
	RecordName      string `json:"recordName"`
	RecordChangeTag string `json:"recordChangeTag"`
	Reason          string `json:"reason"`
	ServerErrorCode string `json:"serverErrorCode"`
	Deleted         bool   `json:"deleted"`
}

func (cd *CloudDocs) lookup(ctx context.Context, recordNames ...string) ([]ckRecord, error) {
	zone, err := cd.Zone(ctx)
	if err != nil {
		return nil, err
	}
	recs := make([]map[string]any, 0, len(recordNames))
	for _, rn := range recordNames {
		recs = append(recs, map[string]any{"recordName": rn})
	}
	var out struct {
		Records []ckRecord `json:"records"`
	}
	if _, err := cd.request(ctx, "records/lookup", map[string]any{"zoneID": zone, "records": recs}, &out); err != nil {
		return nil, err
	}
	return out.Records, nil
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
	dirRef := map[string]any{
		"recordName": "directory/" + strings.ToUpper(targetDirUUID),
		"action":     "VALIDATE",
		"zoneID":     zone,
	}
	record := map[string]any{
		"recordName":      structRec,
		"recordType":      "structure",
		"recordChangeTag": recs[0].RecordChangeTag,
		"fields": map[string]any{
			"parent": map[string]any{"value": dirRef, "type": "REFERENCE"},
		},
		"parent": map[string]any{"recordName": "directory/" + strings.ToUpper(targetDirUUID)},
	}
	body := map[string]any{
		"atomic":     true,
		"zoneID":     zone,
		"operations": []any{map[string]any{"operationType": "update", "record": record}},
	}
	var out struct {
		Records []ckRecord `json:"records"`
	}
	if _, err := cd.request(ctx, "records/modify", body, &out); err != nil {
		return err
	}
	for _, r := range out.Records {
		if r.ServerErrorCode != "" {
			return fmt.Errorf("clouddocs re-parent failed for %s: %s (%s)", r.RecordName, r.Reason, r.ServerErrorCode)
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
	uuid = strings.ToUpper(uuid)
	contentRec := "documentContent/" + uuid
	structRec := "documentStructure/" + uuid
	recs, err := cd.lookup(ctx, contentRec, structRec)
	if err != nil {
		return err
	}
	tags := map[string]string{}
	for _, r := range recs {
		tags[r.RecordName] = r.RecordChangeTag
	}
	ops := []any{}
	for _, rn := range []string{contentRec, structRec} {
		rec := map[string]any{"recordName": rn}
		if t := tags[rn]; t != "" {
			rec["recordChangeTag"] = t
		}
		ops = append(ops, map[string]any{"operationType": "delete", "record": rec})
	}
	var out struct {
		Records []ckRecord `json:"records"`
	}
	_, err = cd.request(ctx, "records/modify", map[string]any{"atomic": false, "zoneID": zone, "operations": ops}, &out)
	return err
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
	basename := leaf
	if i := strings.LastIndexByte(leaf, '.'); i > 0 {
		basename = leaf[:i]
	}
	sum := sha256.Sum256([]byte(basename))
	wantHash := base64.StdEncoding.EncodeToString(sum[:])
	wantParent := "directory/" + strings.ToUpper(dirUUID)

	syncToken := ""
	for {
		zoneReq := map[string]any{"zoneID": zone}
		if syncToken != "" {
			zoneReq["syncToken"] = syncToken
		}
		var out struct {
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
		if _, err := cd.request(ctx, "changes/zone", map[string]any{"zones": []any{zoneReq}}, &out); err != nil {
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
		if !z.MoreComing {
			return "", nil
		}
		syncToken = z.SyncToken
	}
}
