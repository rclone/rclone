package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/rclone/rclone/fs"
)

const albumMembershipCacheFile = "album-membership-v1.json"

var albumMembershipDesiredKeys = []string{
	"containerId", "itemId", "isDeleted", "masterRef", "remappedRef",
	"isSparsePrivateRecord", "linkedShareRecordName", "linkedShareZoneName", "linkedShareZoneOwner",
}

type membershipRecord struct {
	RecordName string                     `json:"recordName"`
	RecordType string                     `json:"recordType"`
	Deleted    bool                       `json:"deleted,omitempty"`
	Fields     map[string]json.RawMessage `json:"fields,omitempty"`
}

type albumMembershipState struct {
	Version  int                         `json:"version"`
	Scope    string                      `json:"scope"`
	Token    string                      `json:"token"`
	Complete bool                        `json:"complete"`
	Records  map[string]membershipRecord `json:"records"`
}

func (lib *Library) membershipScope() string {
	return strings.Join([]string{lib.area, lib.zoneID, lib.ownerRecordName, lib.zoneType}, "|")
}

func (lib *Library) loadAlbumMembership() (*albumMembershipState, error) {
	state := &albumMembershipState{Version: 1, Scope: lib.membershipScope(), Records: make(map[string]membershipRecord)}
	data, err := os.ReadFile(filepath.Join(lib.zoneCacheDir(), albumMembershipCacheFile))
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("invalid album membership checkpoint: %w", err)
	}
	if state.Version != 1 || state.Scope != lib.membershipScope() || state.Records == nil {
		return nil, errors.New("album membership checkpoint has incompatible version or zone scope")
	}
	return state, nil
}

func saveAlbumMembership(filename string, state *albumMembershipState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return atomicWriteFile(filename, data)
}

func applyAlbumMembershipRecords(state *albumMembershipState, records []json.RawMessage) error {
	for _, raw := range records {
		var record membershipRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return fmt.Errorf("invalid membership change record: %w", err)
		}
		if record.RecordName == "" {
			return errors.New("membership change record has no identity")
		}
		if record.Deleted {
			if record.RecordType == "CPLAsset" || state.Records[record.RecordName].RecordType == "CPLAsset" {
				record.RecordType, record.Fields = "CPLAsset", nil
				state.Records[record.RecordName] = record
			} else {
				delete(state.Records, record.RecordName)
			}
			continue
		}
		if record.RecordType != "CPLAsset" && record.RecordType != "CPLContainerRelation" {
			continue
		}
		minimal := make(map[string]json.RawMessage)
		for _, name := range albumMembershipDesiredKeys {
			if value, ok := record.Fields[name]; ok {
				minimal[name] = value
			}
		}
		record.Fields = minimal
		state.Records[record.RecordName] = record
	}
	return nil
}

func (lib *Library) fetchAlbumMembershipPage(ctx context.Context, token string) (*changesZoneResult, error) {
	zone := map[string]any{"zoneID": lib.zoneIDMap(), "desiredKeys": albumMembershipDesiredKeys, "resultsLimit": 200}
	if token != "" {
		zone["syncToken"] = token
	}
	var response changesZoneResponse
	if err := lib.request(ctx, "changes/zone", map[string]any{"zones": []map[string]any{zone}}, &response); err != nil {
		return nil, err
	}
	if len(response.Zones) != 1 || response.Zones[0].ZoneID.ZoneName != lib.zoneID {
		return nil, errors.New("membership response omitted the requested zone")
	}
	page := response.Zones[0]
	if page.SyncToken == "" || (page.MoreComing && page.SyncToken == token) {
		return nil, errors.New("membership response has no advancing sync token")
	}
	return &page, nil
}

func (lib *Library) refreshAlbumMembership(ctx context.Context) (*albumMembershipState, error) {
	if err := os.MkdirAll(lib.zoneCacheDir(), 0700); err != nil {
		return nil, err
	}
	filename := filepath.Join(lib.zoneCacheDir(), albumMembershipCacheFile)
	lock := flock.New(filename + ".lock")
	locked, err := lock.TryLockContext(ctx, 250*time.Millisecond)
	if err != nil || !locked {
		return nil, fmt.Errorf("lock album membership checkpoint: %w", err)
	}
	defer func() { _ = lock.Close() }()
	state, err := lib.loadAlbumMembership()
	if err != nil {
		return nil, err
	}
	for pageNumber := 1; ; pageNumber++ {
		page, err := lib.fetchAlbumMembershipPage(ctx, state.Token)
		if err != nil {
			return nil, fmt.Errorf("album membership page %d: %w", pageNumber, err)
		}
		if err = applyAlbumMembershipRecords(state, page.Records); err != nil {
			return nil, err
		}
		state.Token, state.Complete = page.SyncToken, !page.MoreComing
		if err = saveAlbumMembership(filename, state); err != nil {
			return nil, err
		}
		fs.Infof(nil, "iclouddrive photos: membership zone=%s page=%d changes=%d retained=%d complete=%t", lib.zoneID, pageNumber, len(page.Records), len(state.Records), state.Complete)
		if state.Complete {
			return state, nil
		}
	}
}

func membershipString(record membershipRecord, key string) string {
	var field struct {
		Value json.RawMessage `json:"value"`
	}
	if json.Unmarshal(record.Fields[key], &field) != nil {
		return ""
	}
	var value string
	if json.Unmarshal(field.Value, &value) == nil {
		return value
	}
	var reference struct{ RecordName string }
	_ = json.Unmarshal(field.Value, &reference)
	return reference.RecordName
}

func albumMemberIDs(state *albumMembershipState, albumID string) ([]string, error) {
	members := make(map[string]bool)
	for _, record := range state.Records {
		if record.RecordType != "CPLContainerRelation" {
			continue
		}
		parts := strings.SplitN(record.RecordName, "-IN-", 2)
		container, member := membershipString(record, "containerId"), membershipString(record, "itemId")
		if len(parts) == 2 {
			if container == "" {
				container = parts[1]
			}
			if member == "" {
				member = parts[0]
			}
		}
		if container == albumID {
			if member == "" {
				return nil, fmt.Errorf("album relation %q has no member identity", record.RecordName)
			}
			deleted, err := membershipIsDeleted(record, state.Records[member])
			if err != nil {
				return nil, err
			}
			if deleted {
				continue
			}
			members[member] = true
		}
	}
	result := make([]string, 0, len(members))
	for member := range members {
		result = append(result, member)
	}
	sort.Strings(result)
	return result, nil
}

func membershipIsDeleted(records ...membershipRecord) (bool, error) {
	for _, record := range records {
		if record.Deleted {
			return true, nil
		}
		if raw, ok := record.Fields["isDeleted"]; ok {
			var deleted ckBoolField
			if json.Unmarshal(raw, &deleted) != nil {
				return false, errors.New("invalid membership deletion field")
			}
			if deleted.Value {
				return true, nil
			}
		}
	}
	return false, nil
}
