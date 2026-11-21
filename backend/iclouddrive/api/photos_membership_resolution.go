package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
)

type membershipReference struct {
	RecordName string `json:"recordName"`
	ZoneID     struct {
		ZoneName        string `json:"zoneName"`
		OwnerRecordName string `json:"ownerRecordName"`
	} `json:"zoneID"`
}

func membershipRedirect(record membershipRecord) (*membershipReference, error) {
	if raw, ok := record.Fields["remappedRef"]; ok {
		var field struct{ Value membershipReference }
		if err := json.Unmarshal(raw, &field); err != nil || field.Value.RecordName == "" {
			return nil, errors.New("invalid asset remappedRef")
		}
		return &field.Value, nil
	}
	zone := membershipString(record, "linkedShareZoneName")
	if zone == "" {
		return nil, nil
	}
	target := &membershipReference{RecordName: membershipString(record, "linkedShareRecordName")}
	target.ZoneID.ZoneName = zone
	target.ZoneID.OwnerRecordName = membershipString(record, "linkedShareZoneOwner")
	if target.RecordName == "" || target.ZoneID.OwnerRecordName == "" {
		return nil, errors.New("shared asset alias has incomplete target identity")
	}
	return target, nil
}

func (lib *Library) membershipTargetLibrary(target *membershipReference) (*Library, error) {
	zone := target.ZoneID.ZoneName
	if zone == "" {
		zone = lib.zoneID
	}
	owner := target.ZoneID.OwnerRecordName
	lib.service.mu.Lock()
	found := lib.service.libraries[zone]
	lib.service.mu.Unlock()
	if zone == lib.zoneID {
		found = lib
	}
	if found == nil || (owner != "" && found.ownerRecordName != "" && owner != found.ownerRecordName) {
		return nil, fmt.Errorf("asset alias references undiscovered zone or mismatched owner: %q", zone)
	}
	if owner == "" {
		owner = found.ownerRecordName
	}
	return &Library{service: lib.service, zoneID: zone, area: found.area, ownerRecordName: owner, zoneType: found.zoneType}, nil
}

// errMembershipRecordGone marks a lookup that returned no live record: the
// asset was deleted in the cloud while a stale relation still references it.
var errMembershipRecordGone = errors.New("membership record not found")

func isMembershipRecordGone(err error) bool {
	return errors.Is(err, errMembershipRecordGone)
}

func (lib *Library) lookupMembershipRecord(ctx context.Context, name string) (*membershipRecord, error) {
	// CloudKit lookups intermittently omit live records; retry so a
	// transient miss is not mistaken for a deletion.
	for attempt := range 3 {
		var response struct {
			Records []membershipRecord `json:"records"`
		}
		query := map[string]any{"zoneID": lib.zoneIDMap(), "records": []map[string]string{{"recordName": name}}}
		if err := lib.request(ctx, "records/lookup", query, &response); err != nil {
			return nil, err
		}
		for _, record := range response.Records {
			if record.RecordName == name {
				if record.Deleted || record.RecordType == "" {
					return nil, fmt.Errorf("%w: %q is deleted", errMembershipRecordGone, name)
				}
				return &record, nil
			}
		}
		if attempt < 2 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil, fmt.Errorf("%w: %q", errMembershipRecordGone, name)
}

func (lib *Library) resolveMembershipRecord(ctx context.Context, name string, cached map[string]membershipRecord) (*Library, *membershipRecord, error) {
	current := lib
	seen := make(map[string]bool)
	for range 32 {
		identity := current.membershipScope() + "/" + name
		if seen[identity] {
			return nil, nil, fmt.Errorf("%w: %q", errMembershipRecordGone, name)
		}
		seen[identity] = true
		record, available := cached[name]
		redirect, err := membershipRedirect(record)
		if err != nil {
			return nil, nil, err
		}
		if !available || redirect == nil || current.zoneID != lib.zoneID {
			fresh, err := current.lookupMembershipRecord(ctx, name)
			if err != nil {
				return nil, nil, err
			}
			record = *fresh
			redirect, err = membershipRedirect(record)
			if err != nil {
				return nil, nil, err
			}
		}
		if redirect == nil {
			return current, &record, nil
		}
		current, err = current.membershipTargetLibrary(redirect)
		if err != nil {
			return nil, nil, err
		}
		name = redirect.RecordName
	}
	return nil, nil, fmt.Errorf("%w: %q exceeds alias depth", errMembershipRecordGone, name)
}

func (album *Album) resolveMemberPhotos(ctx context.Context, name string, state *albumMembershipState) ([]*Photo, error) {
	lib, asset, err := album.lib.resolveMembershipRecord(ctx, name, state.Records)
	if err != nil {
		return nil, err
	}
	masterID := membershipString(*asset, "masterRef")
	if asset.RecordType != "CPLAsset" || masterID == "" {
		return nil, fmt.Errorf("album member %q has no master reference", name)
	}
	masterLib, master, err := lib.resolveMembershipRecord(ctx, masterID, nil)
	if err != nil {
		return nil, err
	}
	if master.RecordType != "CPLMaster" || masterLib.membershipScope() != lib.membershipScope() {
		return nil, errors.New("album member master has incompatible type or zone")
	}
	var parsedAsset, parsedMaster photoRecord
	encodedAsset, _ := json.Marshal(asset)
	encodedMaster, _ := json.Marshal(master)
	if json.Unmarshal(encodedAsset, &parsedAsset) != nil || json.Unmarshal(encodedMaster, &parsedMaster) != nil {
		return nil, errors.New("invalid album member resource metadata")
	}
	photos := buildPhotos(&parsedMaster, &parsedAsset)
	if len(photos) == 0 {
		return nil, fmt.Errorf("album member %q has no downloadable original", name)
	}
	for _, photo := range photos {
		photo.AssetID = name
		photo.SourceZone, photo.SourceArea = lib.zoneID, lib.area
		photo.SourceOwner, photo.SourceZoneType = lib.ownerRecordName, lib.zoneType
	}
	return photos, nil
}

func (album *Album) resolveAllMemberPhotos(ctx context.Context, members []string, state *albumMembershipState) ([]*Photo, error) {
	results := make([][]*Photo, len(members))
	errorsByMember := make([]error, len(members))
	workers := max(1, fs.GetConfig(ctx).Checkers)
	semaphore := make(chan struct{}, workers)
	var group sync.WaitGroup
	for index, name := range members {
		group.Add(1)
		go func(index int, name string) {
			defer group.Done()
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				errorsByMember[index] = ctx.Err()
				return
			}
			defer func() { <-semaphore }()
			results[index], errorsByMember[index] = album.resolveMemberPhotos(ctx, name, state)
		}(index, name)
	}
	group.Wait()
	var photos []*Photo
	for index, result := range results {
		if err := errorsByMember[index]; err != nil {
			// A stale relation to a deleted asset must not hide the album's
			// live members; anything else is a real failure.
			if isMembershipRecordGone(err) {
				fs.Logf(nil, "iclouddrive photos: album %q skips deleted member %q", album.Name, members[index])
				continue
			}
			return nil, fmt.Errorf("resolve album %q member %q: %w", album.Name, members[index], err)
		}
		photos = append(photos, result...)
	}
	return photos, nil
}

func (album *Album) getMembershipPhotos(ctx context.Context) ([]*Photo, error) {
	state, err := album.lib.refreshAlbumMembership(ctx)
	if err != nil {
		return nil, err
	}
	members, err := albumMemberIDs(state, album.RecordName)
	if err != nil {
		return nil, err
	}
	photos, err := album.resolveAllMemberPhotos(ctx, members, state)
	if err != nil {
		return nil, err
	}
	album.saveDiskCache(photos)
	deduplicateFilenames(photos)
	album.mu.Lock()
	album.photoCache = buildPhotoCache(photos)
	album.mu.Unlock()
	fs.Infof(nil, "iclouddrive photos: album=%q members=%d resources=%d", album.Name, len(members), len(photos))
	return photos, nil
}

// LookupPhotoDownloadURL resolves a photo resource in its recorded source zone.
func (ps *PhotosService) LookupPhotoDownloadURL(ctx context.Context, photo *Photo) (string, error) {
	if photo.SourceZone == "" {
		return ps.LookupDownloadURL(ctx, photo.ID, "PrimarySync", photo.ResourceKey)
	}
	zone := map[string]any{"zoneName": photo.SourceZone}
	if photo.SourceOwner != "" {
		zone["ownerRecordName"] = photo.SourceOwner
	}
	if photo.SourceZoneType != "" {
		zone["zoneType"] = photo.SourceZoneType
	}
	if photo.SourceArea != areaPrivate && photo.SourceArea != areaShared {
		return "", errors.New("photo resource has invalid source database area")
	}
	return ps.lookupDownloadURL(ctx, photo.ID, photo.SourceArea, zone, photo.ResourceKey)
}

func customAlbum(album *Album) bool {
	return strings.HasPrefix(album.ListType, "CPLContainerRelation")
}
