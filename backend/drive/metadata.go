package drive

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/errcount"
	"golang.org/x/sync/errgroup"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// system metadata keys which this backend owns
var systemMetadataInfo = map[string]fs.MetadataHelp{
	"content-type": {
		Help:    "The MIME type of the file.",
		Type:    "string",
		Example: "text/plain",
	},
	"mtime": {
		Help:    "Time of last modification with mS accuracy.",
		Type:    "RFC 3339",
		Example: "2006-01-02T15:04:05.999Z07:00",
	},
	"btime": {
		Help:    "Time of file birth (creation) with mS accuracy. Note that this is only writable on fresh uploads - it can't be written for updates.",
		Type:    "RFC 3339",
		Example: "2006-01-02T15:04:05.999Z07:00",
	},
	"copy-requires-writer-permission": {
		Help:    "Whether the options to copy, print, or download this file, should be disabled for readers and commenters.",
		Type:    "boolean",
		Example: "true",
	},
	"writers-can-share": {
		Help:    "Whether users with only writer permission can modify the file's permissions. Not populated and ignored when setting for items in shared drives.",
		Type:    "boolean",
		Example: "false",
	},
	"viewed-by-me": {
		Help:     "Whether the file has been viewed by this user.",
		Type:     "boolean",
		Example:  "true",
		ReadOnly: true,
	},
	"owner": {
		Help:    "The owner of the file. Usually an email address. Enable with --drive-metadata-owner.",
		Type:    "string",
		Example: "user@example.com",
	},
	"permissions": {
		Help:    "Permissions in a JSON dump of Google drive format. On shared drives these will only be present if they aren't inherited. Enable with --drive-metadata-permissions.",
		Type:    "JSON",
		Example: "{}",
	},
	"folder-color-rgb": {
		Help:    "The color for a folder or a shortcut to a folder as an RGB hex string.",
		Type:    "string",
		Example: "881133",
	},
	"description": {
		Help:    "A short description of the file.",
		Type:    "string",
		Example: "Contract for signing",
	},
	"starred": {
		Help:    "Whether the user has starred the file.",
		Type:    "boolean",
		Example: "false",
	},
	"labels": {
		Help:    "Labels attached to this file in a JSON dump of Googled drive format. Enable with --drive-metadata-labels.",
		Type:    "JSON",
		Example: "[]",
	},
}

// Extra fields we need to fetch to implement the system metadata above
var metadataFields = googleapi.Field(strings.Join([]string{
	"copyRequiresWriterPermission",
	"description",
	"folderColorRgb",
	"hasAugmentedPermissions",
	"owners",
	"permissionIds",
	"permissions",
	"properties",
	"starred",
	"viewedByMe",
	"viewedByMeTime",
	"writersCanShare",
}, ","))

// Fields we need to read from permissions
var permissionsFields = googleapi.Field(strings.Join([]string{
	"*",
	"permissionDetails/*",
}, ","))

// getPermission returns permissions for the fileID and permissionID passed in
func (f *Fs) getPermission(ctx context.Context, fileID, permissionID string, useCache bool) (perm *drive.Permission, inherited bool, err error) {
	f.permissionsMu.Lock()
	defer f.permissionsMu.Unlock()
	if useCache {
		perm = f.permissions[permissionID]
		if perm != nil {
			return perm, false, nil
		}
	}
	fs.Debugf(f, "Fetching permission %q", permissionID)
	err = f.pacer.Call(func() (bool, error) {
		perm, err = f.svc.Permissions.Get(fileID, permissionID).
			Fields(permissionsFields).
			SupportsAllDrives(true).
			Context(ctx).Do()
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, false, err
	}

	inherited = len(perm.PermissionDetails) > 0 && perm.PermissionDetails[0].Inherited

	cleanPermission(perm)

	// cache the permission
	f.permissions[permissionID] = perm

	return perm, inherited, err
}

// Set the permissions on the info
func (f *Fs) setPermissions(ctx context.Context, info *drive.File, permissions []*drive.Permission) (err error) {
	errs := errcount.New()
	for _, perm := range permissions {
		if perm.Role == "owner" {
			// ignore owner permissions - these are set with owner
			continue
		}
		cleanPermissionForWrite(perm)
		err := f.pacer.Call(func() (bool, error) {
			_, err := f.svc.Permissions.Create(info.Id, perm).
				SupportsAllDrives(true).
				SendNotificationEmail(false).
				Context(ctx).Do()
			return f.shouldRetry(ctx, err)
		})
		if err != nil {
			fs.Errorf(f, "Failed to set permission %s for %q: %v", perm.Role, perm.EmailAddress, err)
			errs.Add(err)
		}
	}
	err = errs.Err("failed to set permission")
	if err != nil {
		err = fserrors.NoRetryError(err)
	}
	return err
}

// Clean attributes from permissions which we can't write
func cleanPermissionForWrite(perm *drive.Permission) {
	perm.Deleted = false
	perm.DisplayName = ""
	perm.Id = ""
	perm.Kind = ""
	perm.PermissionDetails = nil
	perm.TeamDrivePermissionDetails = nil
}

// Clean and cache the permission if not already cached
func (f *Fs) cleanAndCachePermission(perm *drive.Permission) {
	f.permissionsMu.Lock()
	defer f.permissionsMu.Unlock()
	cleanPermission(perm)
	if _, found := f.permissions[perm.Id]; !found {
		f.permissions[perm.Id] = perm
	}
}

// Clean fields we don't need to keep from the permission
func cleanPermission(perm *drive.Permission) {
	// DisplayName: Output only. The "pretty" name of the value of the
	// permission. The following is a list of examples for each type of
	// permission: * `user` - User's full name, as defined for their Google
	// account, such as "Joe Smith." * `group` - Name of the Google Group,
	// such as "The Company Administrators." * `domain` - String domain
	// name, such as "thecompany.com." * `anyone` - No `displayName` is
	// present.
	perm.DisplayName = ""

	// Kind: Output only. Identifies what kind of resource this is. Value:
	// the fixed string "drive#permission".
	perm.Kind = ""

	// PermissionDetails: Output only. Details of whether the permissions on
	// this shared drive item are inherited or directly on this item. This
	// is an output-only field which is present only for shared drive items.
	perm.PermissionDetails = nil

	// PhotoLink: Output only. A link to the user's profile photo, if
	// available.
	perm.PhotoLink = ""

	// TeamDrivePermissionDetails: Output only. Deprecated: Output only. Use
	// `permissionDetails` instead.
	perm.TeamDrivePermissionDetails = nil
}

// Fields we need to read from labels
var labelsFields = googleapi.Field(strings.Join([]string{
	"*",
}, ","))

// getLabels returns labels for the fileID passed in
func (f *Fs) getLabels(ctx context.Context, fileID string) (labels []*drive.Label, err error) {
	fs.Debugf(f, "Fetching labels for %q", fileID)
	listLabels := f.svc.Files.ListLabels(fileID).
		Fields(labelsFields).
		Context(ctx)
	for {
		var info *drive.LabelList
		err = f.pacer.Call(func() (bool, error) {
			info, err = listLabels.Do()
			return f.shouldRetry(ctx, err)
		})
		if err != nil {
			return nil, err
		}
		labels = append(labels, info.Labels...)
		if info.NextPageToken == "" {
			break
		}
		listLabels.PageToken(info.NextPageToken)
	}
	for _, label := range labels {
		cleanLabel(label)
	}
	return labels, nil
}

// Set the labels on the info
func (f *Fs) setLabels(ctx context.Context, info *drive.File, labels []*drive.Label) (err error) {
	if len(labels) == 0 {
		return nil
	}
	req := drive.ModifyLabelsRequest{}
	for _, label := range labels {
		req.LabelModifications = append(req.LabelModifications, &drive.LabelModification{
			FieldModifications: labelFieldsToFieldModifications(label.Fields),
			LabelId:            label.Id,
		})
	}
	err = f.pacer.Call(func() (bool, error) {
		_, err = f.svc.Files.ModifyLabels(info.Id, &req).
			Context(ctx).Do()
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return fmt.Errorf("failed to set labels: %w", err)
	}
	return nil
}

// Convert label fields into something which can set the fields
func labelFieldsToFieldModifications(fields map[string]drive.LabelField) (out []*drive.LabelFieldModification) {
	for id, field := range fields {
		var emails []string
		for _, user := range field.User {
			emails = append(emails, user.EmailAddress)
		}
		out = append(out, &drive.LabelFieldModification{
			// FieldId: The ID of the field to be modified.
			FieldId: id,

			// SetDateValues: Replaces the value of a dateString Field with these
			// new values. The string must be in the RFC 3339 full-date format:
			// YYYY-MM-DD.
			SetDateValues: field.DateString,

			// SetIntegerValues: Replaces the value of an `integer` field with these
			// new values.
			SetIntegerValues: field.Integer,

			// SetSelectionValues: Replaces a `selection` field with these new
			// values.
			SetSelectionValues: field.Selection,

			// SetTextValues: Sets the value of a `text` field.
			SetTextValues: field.Text,

			// SetUserValues: Replaces a `user` field with these new values. The
			// values must be valid email addresses.
			SetUserValues: emails,
		})
	}
	return out
}

// Clean fields we don't need to keep from the label
func cleanLabel(label *drive.Label) {
	// Kind: This is always drive#label
	label.Kind = ""

	for name, field := range label.Fields {
		// Kind: This is always drive#labelField.
		field.Kind = ""

		// Note the fields are copies so we need to write them
		// back to the map
		label.Fields[name] = field
	}
}

// Parse the metadata from drive item
//
// It should return nil if there is no Metadata
func (o *baseObject) parseMetadata(ctx context.Context, info *drive.File) (err error) {
	metadata := make(fs.Metadata, 16)

	// Dump user metadata first as it overrides system metadata
	for k, v := range info.Properties {
		metadata[k] = v
	}

	// System metadata
	metadata["copy-requires-writer-permission"] = fmt.Sprint(info.CopyRequiresWriterPermission)
	metadata["writers-can-share"] = fmt.Sprint(info.WritersCanShare)
	metadata["viewed-by-me"] = fmt.Sprint(info.ViewedByMe)
	metadata["content-type"] = info.MimeType

	// Owners: Output only. The owner of this file. Only certain legacy
	// files may have more than one owner. This field isn't populated for
	// items in shared drives.
	if o.fs.opt.MetadataOwner.IsSet(rwRead) && len(info.Owners) > 0 {
		user := info.Owners[0]
		if len(info.Owners) > 1 {
			fs.Logf(o, "Ignoring more than 1 owner")
		}
		if user != nil {
			id := user.EmailAddress
			if id == "" {
				id = user.DisplayName
			}
			metadata["owner"] = id
		}
	}

	if o.fs.opt.MetadataPermissions.IsSet(rwRead) {
		// We only write permissions out if they are not inherited.
		//
		// On My Drives permissions seem to be attached to every item
		// so they will always be written out.
		//
		// On Shared Drives only non-inherited permissions will be
		// written out.

		// To read the inherited permissions flag will mean we need to
		// read the permissions for each object and the cache will be
		// useless. However shared drives don't return permissions
		// only permissionIds so will need to fetch them for each
		// object. We use HasAugmentedPermissions to see if there are
		// special permissions before fetching them to save transactions.

		// HasAugmentedPermissions: Output only. Whether there are permissions
		// directly on this file. This field is only populated for items in
		// shared drives.
		if o.fs.isTeamDrive && !info.HasAugmentedPermissions {
			// Don't process permissions if there aren't any specifically set
			fs.Debugf(o, "Ignoring %d permissions and %d permissionIds as is shared drive with hasAugmentedPermissions false", len(info.Permissions), len(info.PermissionIds))
			info.Permissions = nil
			info.PermissionIds = nil
		}

		// PermissionIds: Output only. List of permission IDs for users with
		// access to this file.
		//
		// Only process these if we have no Permissions
		if len(info.PermissionIds) > 0 && len(info.Permissions) == 0 {
			info.Permissions = make([]*drive.Permission, 0, len(info.PermissionIds))
			g, gCtx := errgroup.WithContext(ctx)
			g.SetLimit(o.fs.ci.Checkers)
			var mu sync.Mutex // protect the info.Permissions from concurrent writes
			for _, permissionID := range info.PermissionIds {
				permissionID := permissionID
				g.Go(func() error {
					// must fetch the team drive ones individually to check the inherited flag
					perm, inherited, err := o.fs.getPermission(gCtx, actualID(info.Id), permissionID, !o.fs.isTeamDrive)
					if err != nil {
						return fmt.Errorf("failed to read permission: %w", err)
					}
					// Don't write inherited permissions out
					if inherited {
						return nil
					}
					// Don't write owner role out - these are covered by the owner metadata
					if perm.Role == "owner" {
						return nil
					}
					mu.Lock()
					info.Permissions = append(info.Permissions, perm)
					mu.Unlock()
					return nil
				})
			}
			err = g.Wait()
			if err != nil {
				return err
			}
		} else {
			// Clean the fetched permissions
			for _, perm := range info.Permissions {
				o.fs.cleanAndCachePermission(perm)
			}
		}

		// Permissions: Output only. The full list of permissions for the file.
		// This is only available if the requesting user can share the file. Not
		// populated for items in shared drives.
		if len(info.Permissions) > 0 {
			buf, err := json.Marshal(info.Permissions)
			if err != nil {
				return fmt.Errorf("failed to marshal permissions: %w", err)
			}
			metadata["permissions"] = string(buf)
		}

		// Permission propagation
		// https://developers.google.com/drive/api/guides/manage-sharing#permission-propagation
		// Leads me to believe that in non shared drives, permissions
		// are added to each item when you set permissions for a
		// folder whereas in shared drives they are inherited and
		// placed on the item directly.
	}

	if info.FolderColorRgb != "" {
		metadata["folder-color-rgb"] = info.FolderColorRgb
	}
	if info.Description != "" {
		metadata["description"] = info.Description
	}
	metadata["starred"] = fmt.Sprint(info.Starred)
	metadata["btime"] = info.CreatedTime
	metadata["mtime"] = info.ModifiedTime

	if o.fs.opt.MetadataLabels.IsSet(rwRead) {
		// FIXME would be really nice if we knew if files had labels
		// before listing but we need to know all possible label IDs
		// to get it in the listing.

		labels, err := o.fs.getLabels(ctx, actualID(info.Id))
		if err != nil {
			return fmt.Errorf("failed to fetch labels: %w", err)
		}
		buf, err := json.Marshal(labels)
		if err != nil {
			return fmt.Errorf("failed to marshal labels: %w", err)
		}
		metadata["labels"] = string(buf)
	}

	o.metadata = &metadata
	return nil
}

// Set the owner on the info
func (f *Fs) setOwner(ctx context.Context, info *drive.File, owner string) (err error) {
	perm := drive.Permission{
		Role:         "owner",
		EmailAddress: owner,
		// Type: The type of the grantee. Valid values are: * `user` * `group` *
		// `domain` * `anyone` When creating a permission, if `type` is `user`
		// or `group`, you must provide an `emailAddress` for the user or group.
		// When `type` is `domain`, you must provide a `domain`. There isn't
		// extra information required for an `anyone` type.
		Type: "user",
	}
	err = f.pacer.Call(func() (bool, error) {
		_, err = f.svc.Permissions.Create(info.Id, &perm).
			SupportsAllDrives(true).
			TransferOwnership(true).
			// SendNotificationEmail(false). - required apparently!
			Context(ctx).Do()
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return fmt.Errorf("failed to set owner: %w", err)
	}
	return nil
}

// Call back to set metadata that can't be set on the upload/update
//
// The *drive.File passed in holds the current state of the drive.File
// and this should update it with any modifications.
type updateMetadataFn func(context.Context, *drive.File) error

// read the metadata from meta and write it into updateInfo
//
// update should be true if this is being used to create metadata for
// an update/PATCH call as the rules on what can be updated are
// slightly different there.
//
// It returns a callback which should be called to finish the updates
// after the data is uploaded.
func (f *Fs) updateMetadata(ctx context.Context, updateInfo *drive.File, meta fs.Metadata, update bool) (callback updateMetadataFn, err error) {
	callbackFns := []updateMetadataFn{}
	callback = func(ctx context.Context, info *drive.File) error {
		for _, fn := range callbackFns {
			err := fn(ctx, info)
			if err != nil {
				return err
			}
		}
		return nil
	}
	// merge metadata into request and user metadata
	for k, v := range meta {
		k, v := k, v
		// parse a boolean from v and write into out
		parseBool := func(out *bool) error {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return fmt.Errorf("can't parse metadata %q = %q: %w", k, v, err)
			}
			*out = b
			return nil
		}
		switch k {
		case "copy-requires-writer-permission":
			if err := parseBool(&updateInfo.CopyRequiresWriterPermission); err != nil {
				return nil, err
			}
		case "writers-can-share":
			if !f.isTeamDrive {
				if err := parseBool(&updateInfo.WritersCanShare); err != nil {
					return nil, err
				}
			} else {
				fs.Debugf(f, "Ignoring %s=%s as can't set on shared drives", k, v)
			}
		case "viewed-by-me":
			// Can't write this
		case "content-type":
			updateInfo.MimeType = v
		case "owner":
			if !f.opt.MetadataOwner.IsSet(rwWrite) {
				continue
			}
			// Can't set Owner on upload so need to set afterwards
			callbackFns = append(callbackFns, func(ctx context.Context, info *drive.File) error {
				err := f.setOwner(ctx, info, v)
				if err != nil && f.opt.MetadataOwner.IsSet(rwFailOK) {
					fs.Errorf(f, "Ignoring error as failok is set: %v", err)
					return nil
				}
				return err
			})
		case "permissions":
			if !f.opt.MetadataPermissions.IsSet(rwWrite) {
				continue
			}
			var perms []*drive.Permission
			err := json.Unmarshal([]byte(v), &perms)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal permissions: %w", err)
			}
			// Can't set Permissions on upload so need to set afterwards
			callbackFns = append(callbackFns, func(ctx context.Context, info *drive.File) error {
				err := f.setPermissions(ctx, info, perms)
				if err != nil && f.opt.MetadataPermissions.IsSet(rwFailOK) {
					// We've already logged the permissions errors individually here
					fs.Debugf(f, "Ignoring error as failok is set: %v", err)
					return nil
				}
				return err
			})
		case "labels":
			if !f.opt.MetadataLabels.IsSet(rwWrite) {
				continue
			}
			var labels []*drive.Label
			err := json.Unmarshal([]byte(v), &labels)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal labels: %w", err)
			}
			// Can't set Labels on upload so need to set afterwards
			callbackFns = append(callbackFns, func(ctx context.Context, info *drive.File) error {
				err := f.setLabels(ctx, info, labels)
				if err != nil && f.opt.MetadataLabels.IsSet(rwFailOK) {
					fs.Errorf(f, "Ignoring error as failok is set: %v", err)
					return nil
				}
				return err
			})
		case "folder-color-rgb":
			updateInfo.FolderColorRgb = v
		case "description":
			updateInfo.Description = v
		case "starred":
			if err := parseBool(&updateInfo.Starred); err != nil {
				return nil, err
			}
		case "btime":
			if update {
				fs.Debugf(f, "Skipping btime metadata as can't update it on an existing file: %v", v)
			} else {
				updateInfo.CreatedTime = v
			}
		case "mtime":
			updateInfo.ModifiedTime = v
		default:
			if updateInfo.Properties == nil {
				updateInfo.Properties = make(map[string]string, 1)
			}
			updateInfo.Properties[k] = v
		}
	}
	return callback, nil
}

// Fetch metadata and update updateInfo if --metadata is in use
func (f *Fs) fetchAndUpdateMetadata(ctx context.Context, src fs.ObjectInfo, options []fs.OpenOption, updateInfo *drive.File, update bool) (callback updateMetadataFn, err error) {
	meta, err := fs.GetMetadataOptions(ctx, f, src, options)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata from source object: %w", err)
	}
	callback, err = f.updateMetadata(ctx, updateInfo, meta, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update metadata from source object: %w", err)
	}
	return callback, nil
}
