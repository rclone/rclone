package onedrive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/onedrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/errcount"
	"golang.org/x/exp/slices" // replace with slices after go1.21 is the minimum version
)

const (
	dirMimeType   = "inode/directory"
	timeFormatIn  = time.RFC3339
	timeFormatOut = "2006-01-02T15:04:05.999Z" // mS for OneDrive Personal, otherwise only S
)

// system metadata keys which this backend owns
var systemMetadataInfo = map[string]fs.MetadataHelp{
	"content-type": {
		Help:     "The MIME type of the file.",
		Type:     "string",
		Example:  "text/plain",
		ReadOnly: true,
	},
	"mtime": {
		Help:    "Time of last modification with S accuracy (mS for OneDrive Personal).",
		Type:    "RFC 3339",
		Example: "2006-01-02T15:04:05Z",
	},
	"btime": {
		Help:    "Time of file birth (creation) with S accuracy (mS for OneDrive Personal).",
		Type:    "RFC 3339",
		Example: "2006-01-02T15:04:05Z",
	},
	"utime": {
		Help:     "Time of upload with S accuracy (mS for OneDrive Personal).",
		Type:     "RFC 3339",
		Example:  "2006-01-02T15:04:05Z",
		ReadOnly: true,
	},
	"created-by-display-name": {
		Help:     "Display name of the user that created the item.",
		Type:     "string",
		Example:  "John Doe",
		ReadOnly: true,
	},
	"created-by-id": {
		Help:     "ID of the user that created the item.",
		Type:     "string",
		Example:  "48d31887-5fad-4d73-a9f5-3c356e68a038",
		ReadOnly: true,
	},
	"description": {
		Help:    "A short description of the file. Max 1024 characters. Only supported for OneDrive Personal.",
		Type:    "string",
		Example: "Contract for signing",
	},
	"id": {
		Help:     "The unique identifier of the item within OneDrive.",
		Type:     "string",
		Example:  "01BYE5RZ6QN3ZWBTUFOFD3GSPGOHDJD36K",
		ReadOnly: true,
	},
	"last-modified-by-display-name": {
		Help:     "Display name of the user that last modified the item.",
		Type:     "string",
		Example:  "John Doe",
		ReadOnly: true,
	},
	"last-modified-by-id": {
		Help:     "ID of the user that last modified the item.",
		Type:     "string",
		Example:  "48d31887-5fad-4d73-a9f5-3c356e68a038",
		ReadOnly: true,
	},
	"malware-detected": {
		Help:     "Whether OneDrive has detected that the item contains malware.",
		Type:     "boolean",
		Example:  "true",
		ReadOnly: true,
	},
	"package-type": {
		Help:     "If present, indicates that this item is a package instead of a folder or file. Packages are treated like files in some contexts and folders in others.",
		Type:     "string",
		Example:  "oneNote",
		ReadOnly: true,
	},
	"shared-owner-id": {
		Help:     "ID of the owner of the shared item (if shared).",
		Type:     "string",
		Example:  "48d31887-5fad-4d73-a9f5-3c356e68a038",
		ReadOnly: true,
	},
	"shared-by-id": {
		Help:     "ID of the user that shared the item (if shared).",
		Type:     "string",
		Example:  "48d31887-5fad-4d73-a9f5-3c356e68a038",
		ReadOnly: true,
	},
	"shared-scope": {
		Help:     "If shared, indicates the scope of how the item is shared: anonymous, organization, or users.",
		Type:     "string",
		Example:  "users",
		ReadOnly: true,
	},
	"shared-time": {
		Help:     "Time when the item was shared, with S accuracy (mS for OneDrive Personal).",
		Type:     "RFC 3339",
		Example:  "2006-01-02T15:04:05Z",
		ReadOnly: true,
	},
	"permissions": {
		Help:    "Permissions in a JSON dump of OneDrive format. Enable with --onedrive-metadata-permissions. Properties: id, grantedTo, grantedToIdentities, invitation, inheritedFrom, link, roles, shareId",
		Type:    "JSON",
		Example: "{}",
	},
}

// rwChoices type for fs.Bits
type rwChoices struct{}

func (rwChoices) Choices() []fs.BitsChoicesInfo {
	return []fs.BitsChoicesInfo{
		{Bit: uint64(rwOff), Name: "off"},
		{Bit: uint64(rwRead), Name: "read"},
		{Bit: uint64(rwWrite), Name: "write"},
		{Bit: uint64(rwFailOK), Name: "failok"},
	}
}

// rwChoice type alias
type rwChoice = fs.Bits[rwChoices]

const (
	rwRead rwChoice = 1 << iota
	rwWrite
	rwFailOK
	rwOff rwChoice = 0
)

// Examples for the options
var rwExamples = fs.OptionExamples{{
	Value: rwOff.String(),
	Help:  "Do not read or write the value",
}, {
	Value: rwRead.String(),
	Help:  "Read the value only",
}, {
	Value: rwWrite.String(),
	Help:  "Write the value only",
}, {
	Value: (rwRead | rwWrite).String(),
	Help:  "Read and Write the value.",
}, {
	Value: rwFailOK.String(),
	Help:  "If writing fails log errors only, don't fail the transfer",
}}

// Metadata describes metadata properties shared by both Objects and Directories
type Metadata struct {
	fs                *Fs                    // what this object/dir is part of
	remote            string                 // remote, for convenience when obj/dir not in scope
	mimeType          string                 // Content-Type of object from server (may not be as uploaded)
	description       string                 // Provides a user-visible description of the item. Read-write. Only on OneDrive Personal
	mtime             time.Time              // Time of last modification with S accuracy.
	btime             time.Time              // Time of file birth (creation) with S accuracy.
	utime             time.Time              // Time of upload with S accuracy.
	createdBy         api.IdentitySet        // user that created the item
	lastModifiedBy    api.IdentitySet        // user that last modified the item
	malwareDetected   bool                   // Whether OneDrive has detected that the item contains malware.
	packageType       string                 // If present, indicates that this item is a package instead of a folder or file.
	shared            *api.SharedType        // information about the shared state of the item, if shared
	normalizedID      string                 // the normalized ID of the object or dir
	permissions       []*api.PermissionsType // The current set of permissions for the item. Note that to save API calls, this is not guaranteed to be cached on the object. Use m.Get() to refresh.
	queuedPermissions []*api.PermissionsType // The set of permissions queued to be updated.
	permsAddOnly      bool                   // Whether to disable "update" and "remove" (for example, during server-side copy when the dst will have new IDs)
}

// Get retrieves the cached metadata and converts it to fs.Metadata.
// This is most typically used when OneDrive is the source (as opposed to the dest).
// If m.fs.opt.MetadataPermissions includes "read" then this will also include permissions, which requires an API call.
// Get does not use an API call otherwise.
func (m *Metadata) Get(ctx context.Context) (metadata fs.Metadata, err error) {
	metadata = make(fs.Metadata, 17)
	metadata["content-type"] = m.mimeType
	metadata["mtime"] = m.mtime.Format(timeFormatOut)
	metadata["btime"] = m.btime.Format(timeFormatOut)
	metadata["utime"] = m.utime.Format(timeFormatOut)
	metadata["created-by-display-name"] = m.createdBy.User.DisplayName
	metadata["created-by-id"] = m.createdBy.User.ID
	if m.description != "" {
		metadata["description"] = m.description
	}
	metadata["id"] = m.normalizedID
	metadata["last-modified-by-display-name"] = m.lastModifiedBy.User.DisplayName
	metadata["last-modified-by-id"] = m.lastModifiedBy.User.ID
	metadata["malware-detected"] = fmt.Sprint(m.malwareDetected)
	if m.packageType != "" {
		metadata["package-type"] = m.packageType
	}
	if m.shared != nil {
		metadata["shared-owner-id"] = m.shared.Owner.User.ID
		metadata["shared-by-id"] = m.shared.SharedBy.User.ID
		metadata["shared-scope"] = m.shared.Scope
		metadata["shared-time"] = time.Time(m.shared.SharedDateTime).Format(timeFormatOut)
	}
	if m.fs.opt.MetadataPermissions.IsSet(rwRead) {
		p, _, err := m.fs.getPermissions(ctx, m.normalizedID)
		if err != nil {
			return nil, fmt.Errorf("failed to get permissions: %w", err)
		}
		m.permissions = p

		if len(p) > 0 {
			fs.PrettyPrint(m.permissions, "perms", fs.LogLevelDebug)
			buf, err := json.Marshal(m.permissions)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal permissions: %w", err)
			}
			metadata["permissions"] = string(buf)
		}
	}
	return metadata, nil
}

// Set takes fs.Metadata and parses/converts it to cached Metadata.
// This is most typically used when OneDrive is the destination (as opposed to the source).
// It does not actually update the remote (use Write for that.)
// It sets only the writeable metadata properties (i.e. read-only properties are skipped.)
// Permissions are included if m.fs.opt.MetadataPermissions includes "write".
// It returns errors if writeable properties can't be parsed.
// It does not return errors for unsupported properties that may be passed in.
// It returns the number of writeable properties set (if it is 0, we can skip the Write API call.)
func (m *Metadata) Set(ctx context.Context, metadata fs.Metadata) (numSet int, err error) {
	numSet = 0
	for k, v := range metadata {
		k, v := k, v
		switch k {
		case "mtime":
			t, err := time.Parse(timeFormatIn, v)
			if err != nil {
				return numSet, fmt.Errorf("failed to parse metadata %q = %q: %w", k, v, err)
			}
			m.mtime = t
			numSet++
		case "btime":
			t, err := time.Parse(timeFormatIn, v)
			if err != nil {
				return numSet, fmt.Errorf("failed to parse metadata %q = %q: %w", k, v, err)
			}
			m.btime = t
			numSet++
		case "description":
			if m.fs.driveType != driveTypePersonal {
				fs.Debugf(m.remote, "metadata description is only supported for OneDrive Personal -- skipping: %s", v)
				continue
			}
			m.description = v
			numSet++
		case "permissions":
			if !m.fs.opt.MetadataPermissions.IsSet(rwWrite) {
				continue
			}
			var perms []*api.PermissionsType
			err := json.Unmarshal([]byte(v), &perms)
			if err != nil {
				return numSet, fmt.Errorf("failed to unmarshal permissions: %w", err)
			}
			m.queuedPermissions = perms
			numSet++
		default:
			fs.Debugf(m.remote, "skipping unsupported metadata item: %s: %s", k, v)
		}
	}
	if numSet == 0 {
		fs.Infof(m.remote, "no writeable metadata found: %v", metadata)
	}
	return numSet, nil
}

// toAPIMetadata converts object/dir Metadata to api.Metadata for API calls.
// If btime is missing but mtime is present, mtime is also used as the btime, as otherwise it would get overwritten.
func (m *Metadata) toAPIMetadata() api.Metadata {
	update := api.Metadata{
		FileSystemInfo: &api.FileSystemInfoFacet{},
	}
	if m.description != "" && m.fs.driveType == driveTypePersonal {
		update.Description = m.description
	}
	if !m.mtime.IsZero() {
		update.FileSystemInfo.LastModifiedDateTime = api.Timestamp(m.mtime)
	}
	if !m.btime.IsZero() {
		update.FileSystemInfo.CreatedDateTime = api.Timestamp(m.btime)
	}

	if m.btime.IsZero() && !m.mtime.IsZero() { // use mtime as btime if missing
		m.btime = m.mtime
		update.FileSystemInfo.CreatedDateTime = api.Timestamp(m.btime)
	}
	return update
}

// Write takes the cached Metadata and sets it on the remote, using API calls.
// If m.fs.opt.MetadataPermissions includes "write" and updatePermissions == true, permissions are also set.
// Calling Write without any writeable metadata will result in an error.
func (m *Metadata) Write(ctx context.Context, updatePermissions bool) (*api.Item, error) {
	update := m.toAPIMetadata()
	if update.IsEmpty() {
		return nil, fmt.Errorf("%v: no writeable metadata found: %v", m.remote, m)
	}
	opts := m.fs.newOptsCallWithPath(ctx, m.remote, "PATCH", "")
	var info *api.Item
	err := m.fs.pacer.Call(func() (bool, error) {
		resp, err := m.fs.srv.CallJSON(ctx, &opts, &update, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		fs.Debugf(m.remote, "errored metadata: %v", m)
		return nil, fmt.Errorf("%v: error updating metadata: %v", m.remote, err)
	}

	if m.fs.opt.MetadataPermissions.IsSet(rwWrite) && updatePermissions {
		m.normalizedID = info.GetID()
		err = m.WritePermissions(ctx)
		if err != nil {
			fs.Errorf(m.remote, "error writing permissions: %v", err)
			return info, err
		}
	}

	// update the struct since we have fresh info
	m.fs.setSystemMetadata(info, m, m.remote, m.mimeType)

	return info, err
}

// RefreshPermissions fetches the current permissions from the remote and caches them as Metadata
func (m *Metadata) RefreshPermissions(ctx context.Context) (err error) {
	if m.normalizedID == "" {
		return errors.New("internal error: normalizedID is missing")
	}
	p, _, err := m.fs.getPermissions(ctx, m.normalizedID)
	if err != nil {
		return fmt.Errorf("failed to refresh permissions: %w", err)
	}
	m.permissions = p
	return nil
}

// WritePermissions sets the permissions (and no other metadata) on the remote.
// m.permissions (the existing perms) and m.queuedPermissions (the new perms to be set) must be set correctly before calling this.
// m.permissions == nil will not error, as it is valid to add permissions when there were previously none.
// If successful, m.permissions will be set with the new current permissions and m.queuedPermissions will be nil.
func (m *Metadata) WritePermissions(ctx context.Context) (err error) {
	if !m.fs.opt.MetadataPermissions.IsSet(rwWrite) {
		return errors.New("can't write permissions without --onedrive-metadata-permissions write")
	}
	if m.normalizedID == "" {
		return errors.New("internal error: normalizedID is missing")
	}
	if m.fs.opt.MetadataPermissions.IsSet(rwFailOK) {
		// If failok is set, allow the permissions setting to fail and only log an ERROR
		defer func() {
			if err != nil {
				fs.Errorf(m.fs, "Ignoring error as failok is set: %v", err)
				err = nil
			}
		}()
	}

	// compare current to queued and sort into add/update/remove queues
	add, update, remove := m.sortPermissions()
	fs.Debugf(m.remote, "metadata permissions: to add: %d to update: %d to remove: %d", len(add), len(update), len(remove))
	_, err = m.processPermissions(ctx, add, update, remove)
	if err != nil {
		return fmt.Errorf("failed to process permissions: %w", err)
	}

	err = m.RefreshPermissions(ctx)
	fs.Debugf(m.remote, "updated permissions (now has %d permissions)", len(m.permissions))
	if err != nil {
		return fmt.Errorf("failed to get permissions: %w", err)
	}
	m.queuedPermissions = nil

	return nil
}

// sortPermissions sorts the permissions (to be written) into add, update, and remove queues
func (m *Metadata) sortPermissions() (add, update, remove []*api.PermissionsType) {
	new, old := m.queuedPermissions, m.permissions
	if len(old) == 0 || m.permsAddOnly {
		return new, nil, nil // they must all be "add"
	}

	for _, n := range new {
		if n == nil {
			continue
		}
		if n.ID != "" {
			// sanity check: ensure there's a matching "old" id with a non-matching role
			if !slices.ContainsFunc(old, func(o *api.PermissionsType) bool {
				return o.ID == n.ID && slices.Compare(o.Roles, n.Roles) != 0 && len(o.Roles) > 0 && len(n.Roles) > 0 && !slices.Contains(o.Roles, api.OwnerRole)
			}) {
				fs.Debugf(m.remote, "skipping update for invalid roles: %v (perm ID: %v)", n.Roles, n.ID)
				continue
			}
			if m.fs.driveType != driveTypePersonal && n.Link != nil && n.Link.WebURL != "" {
				// special case to work around API limitation -- can't update a sharing link perm so need to remove + add instead
				// https://learn.microsoft.com/en-us/answers/questions/986279/why-is-update-permission-graph-api-for-files-not-w
				// https://github.com/microsoftgraph/msgraph-sdk-dotnet/issues/1135
				fs.Debugf(m.remote, "sortPermissions: can't update due to API limitation, will remove + add instead: %v", n.Roles)
				remove = append(remove, n)
				add = append(add, n)
				continue
			}
			fs.Debugf(m.remote, "sortPermissions: will update role to %v", n.Roles)
			update = append(update, n)
		} else {
			fs.Debugf(m.remote, "sortPermissions: will add permission: %v %v", n, n.Roles)
			add = append(add, n)
		}
	}
	for _, o := range old {
		if slices.Contains(o.Roles, api.OwnerRole) {
			fs.Debugf(m.remote, "skipping remove permission -- can't remove 'owner' role")
			continue
		}
		newHasOld := slices.ContainsFunc(new, func(n *api.PermissionsType) bool {
			if n == nil || n.ID == "" {
				return false // can't remove perms without an ID
			}
			return n.ID == o.ID
		})
		if !newHasOld && o.ID != "" && !slices.Contains(add, o) && !slices.Contains(update, o) {
			fs.Debugf(m.remote, "sortPermissions: will remove permission: %v %v  (perm ID: %v)", o, o.Roles, o.ID)
			remove = append(remove, o)
		}
	}
	return add, update, remove
}

// processPermissions executes the add, update, and remove queues for writing permissions
func (m *Metadata) processPermissions(ctx context.Context, add, update, remove []*api.PermissionsType) (newPermissions []*api.PermissionsType, err error) {
	errs := errcount.New()
	for _, p := range remove { // remove (need to do these first because of remove + add workaround)
		_, err := m.removePermission(ctx, p)
		if err != nil {
			fs.Errorf(m.remote, "Failed to remove permission: %v", err)
			errs.Add(err)
		}
	}

	for _, p := range add { // add
		newPs, _, err := m.addPermission(ctx, p)
		if err != nil {
			fs.Errorf(m.remote, "Failed to add permission: %v", err)
			errs.Add(err)
			continue
		}
		newPermissions = append(newPermissions, newPs...)
	}

	for _, p := range update { // update
		newP, _, err := m.updatePermission(ctx, p)
		if err != nil {
			fs.Errorf(m.remote, "Failed to update permission: %v", err)
			errs.Add(err)
			continue
		}
		newPermissions = append(newPermissions, newP)
	}

	err = errs.Err("failed to set permissions")
	if err != nil {
		err = fserrors.NoRetryError(err)
	}
	return newPermissions, err
}

// fillRecipients looks for recipients to add from the permission passed in.
// It looks for an email address in identity.User.Email, ID, and DisplayName, otherwise it uses the identity.User.ID as r.ObjectID.
// It considers both "GrantedTo" and "GrantedToIdentities".
func fillRecipients(p *api.PermissionsType, driveType string) (recipients []api.DriveRecipient) {
	if p == nil {
		return recipients
	}
	ids := make(map[string]struct{}, len(p.GetGrantedToIdentities(driveType))+1)
	isUnique := func(s string) bool {
		_, ok := ids[s]
		return !ok && s != ""
	}

	addRecipient := func(identity *api.IdentitySet) {
		r := api.DriveRecipient{}

		id := ""
		if strings.ContainsRune(identity.User.Email, '@') {
			id = identity.User.Email
			r.Email = id
		} else if strings.ContainsRune(identity.User.ID, '@') {
			id = identity.User.ID
			r.Email = id
		} else if strings.ContainsRune(identity.User.DisplayName, '@') {
			id = identity.User.DisplayName
			r.Email = id
		} else {
			id = identity.User.ID
			r.ObjectID = id
		}
		if !isUnique(id) {
			return
		}
		ids[id] = struct{}{}
		recipients = append(recipients, r)
	}

	forIdentitySet := func(iSet *api.IdentitySet) {
		if iSet == nil {
			return
		}
		iS := *iSet
		forIdentity := func(i api.Identity) {
			if i != (api.Identity{}) {
				iS.User = i
				addRecipient(&iS)
			}
		}
		forIdentity(iS.User)
		forIdentity(iS.SiteUser)
		forIdentity(iS.Group)
		forIdentity(iS.SiteGroup)
		forIdentity(iS.Application)
		forIdentity(iS.Device)
	}

	for _, identitySet := range p.GetGrantedToIdentities(driveType) {
		forIdentitySet(identitySet)
	}
	forIdentitySet(p.GetGrantedTo(driveType))

	return recipients
}

// addPermission adds new permissions to an object or dir.
// if p.Link.Scope == "anonymous" then it will also create a Public Link.
func (m *Metadata) addPermission(ctx context.Context, p *api.PermissionsType) (newPs []*api.PermissionsType, resp *http.Response, err error) {
	opts := m.fs.newOptsCall(m.normalizedID, "POST", "/invite")

	req := &api.AddPermissionsRequest{
		Recipients:    fillRecipients(p, m.fs.driveType),
		RequireSignIn: m.fs.driveType != driveTypePersonal, // personal and business have conflicting requirements
		Roles:         p.Roles,
	}
	if m.fs.driveType != driveTypePersonal {
		req.RetainInheritedPermissions = false // not supported for personal
	}

	if p.Link != nil && p.Link.Scope == api.AnonymousScope {
		link, err := m.fs.PublicLink(ctx, m.remote, fs.DurationOff, false)
		if err != nil {
			return nil, nil, err
		}
		p.Link.WebURL = link
		newPs = append(newPs, p)
		if len(req.Recipients) == 0 {
			return newPs, nil, nil
		}
	}

	if len(req.Recipients) == 0 {
		fs.Debugf(m.remote, "skipping add permission -- at least one valid recipient is required")
		return nil, nil, nil
	}
	if len(req.Roles) == 0 {
		return nil, nil, errors.New("at least one role is required to add a permission (choices: read, write, owner, member)")
	}
	if slices.Contains(req.Roles, api.OwnerRole) {
		fs.Debugf(m.remote, "skipping add permission -- can't invite a user with 'owner' role")
		return nil, nil, nil
	}

	newP := &api.PermissionsResponse{}
	err = m.fs.pacer.Call(func() (bool, error) {
		resp, err = m.fs.srv.CallJSON(ctx, &opts, &req, &newP)
		return shouldRetry(ctx, resp, err)
	})

	return newP.Value, resp, err
}

// updatePermission updates an existing permission on an object or dir.
// This requires the permission ID and a role to update (which will error if it is the same as the existing role.)
// Role is the only property that can be updated.
func (m *Metadata) updatePermission(ctx context.Context, p *api.PermissionsType) (newP *api.PermissionsType, resp *http.Response, err error) {
	opts := m.fs.newOptsCall(m.normalizedID, "PATCH", "/permissions/"+p.ID)
	req := api.UpdatePermissionsRequest{Roles: p.Roles} // roles is the only property that can be updated

	if len(req.Roles) == 0 {
		return nil, nil, errors.New("at least one role is required to update a permission (choices: read, write, owner, member)")
	}

	newP = &api.PermissionsType{}
	err = m.fs.pacer.Call(func() (bool, error) {
		resp, err = m.fs.srv.CallJSON(ctx, &opts, &req, &newP)
		return shouldRetry(ctx, resp, err)
	})

	return newP, resp, err
}

// removePermission removes an existing permission on an object or dir.
// This requires the permission ID.
func (m *Metadata) removePermission(ctx context.Context, p *api.PermissionsType) (resp *http.Response, err error) {
	opts := m.fs.newOptsCall(m.normalizedID, "DELETE", "/permissions/"+p.ID)
	opts.NoResponse = true

	err = m.fs.pacer.Call(func() (bool, error) {
		resp, err = m.fs.srv.CallJSON(ctx, &opts, nil, nil)
		return shouldRetry(ctx, resp, err)
	})
	return resp, err
}

// getPermissions gets the current permissions for an object or dir, from the API.
func (f *Fs) getPermissions(ctx context.Context, normalizedID string) (p []*api.PermissionsType, resp *http.Response, err error) {
	opts := f.newOptsCall(normalizedID, "GET", "/permissions")

	permResp := &api.PermissionsResponse{}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &permResp)
		return shouldRetry(ctx, resp, err)
	})

	return permResp.Value, resp, err
}

func (f *Fs) newMetadata(remote string) *Metadata {
	return &Metadata{fs: f, remote: remote}
}

// returns true if metadata includes a "permissions" key and f.opt.MetadataPermissions includes "write".
func (f *Fs) needsUpdatePermissions(metadata fs.Metadata) bool {
	_, ok := metadata["permissions"]
	return ok && f.opt.MetadataPermissions.IsSet(rwWrite)
}

// returns a non-zero btime if we have one
// otherwise falls back to mtime
func (o *Object) tryGetBtime(modTime time.Time) time.Time {
	if o.meta != nil && !o.meta.btime.IsZero() {
		return o.meta.btime
	}
	return modTime
}

// adds metadata (except permissions) if --metadata is in use
func (o *Object) fetchMetadataForCreate(ctx context.Context, src fs.ObjectInfo, options []fs.OpenOption, modTime time.Time) (createRequest api.CreateUploadRequest, metadata fs.Metadata, err error) {
	createRequest = api.CreateUploadRequest{ // we set mtime no matter what
		Item: api.Metadata{
			FileSystemInfo: &api.FileSystemInfoFacet{
				CreatedDateTime:      api.Timestamp(o.tryGetBtime(modTime)),
				LastModifiedDateTime: api.Timestamp(modTime),
			},
		},
	}

	meta, err := fs.GetMetadataOptions(ctx, o.fs, src, options)
	if err != nil {
		return createRequest, nil, fmt.Errorf("failed to read metadata from source object: %w", err)
	}
	if meta == nil {
		return createRequest, nil, nil // no metadata or --metadata not in use, so just return mtime
	}
	if o.meta == nil {
		o.meta = o.fs.newMetadata(o.Remote())
	}
	o.meta.mtime = modTime
	numSet, err := o.meta.Set(ctx, meta)
	if err != nil {
		return createRequest, meta, err
	}
	if numSet == 0 {
		return createRequest, meta, nil
	}
	createRequest.Item = o.meta.toAPIMetadata()
	return createRequest, meta, nil
}

// Fetch metadata and update updateInfo if --metadata is in use
// modtime will still be set when there is no metadata to set
func (f *Fs) fetchAndUpdateMetadata(ctx context.Context, src fs.ObjectInfo, options []fs.OpenOption, updateInfo *Object) (info *api.Item, err error) {
	meta, err := fs.GetMetadataOptions(ctx, f, src, options)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata from source object: %w", err)
	}
	if meta == nil {
		return updateInfo.setModTime(ctx, src.ModTime(ctx)) // no metadata or --metadata not in use, so just set modtime
	}
	if updateInfo.meta == nil {
		updateInfo.meta = f.newMetadata(updateInfo.Remote())
	}
	newInfo, err := updateInfo.updateMetadata(ctx, meta)
	if newInfo == nil {
		return info, err
	}
	return newInfo, err
}

// updateMetadata calls Get, Set, and Write
func (o *Object) updateMetadata(ctx context.Context, meta fs.Metadata) (info *api.Item, err error) {
	_, err = o.meta.Get(ctx) // refresh permissions
	if err != nil {
		return nil, err
	}

	numSet, err := o.meta.Set(ctx, meta)
	if err != nil {
		return nil, err
	}
	if numSet == 0 {
		return nil, nil
	}
	info, err = o.meta.Write(ctx, o.fs.needsUpdatePermissions(meta))
	if err != nil {
		return info, err
	}
	err = o.setMetaData(info)
	if err != nil {
		return info, err
	}

	// Remove versions if required
	if o.fs.opt.NoVersions {
		err := o.deleteVersions(ctx)
		if err != nil {
			return info, fmt.Errorf("%v: Failed to remove versions: %v", o, err)
		}
	}
	return info, nil
}

// MkdirMetadata makes the directory passed in as dir.
//
// It shouldn't return an error if it already exists.
//
// If the metadata is not nil it is set.
//
// It returns the directory that was created.
func (f *Fs) MkdirMetadata(ctx context.Context, dir string, metadata fs.Metadata) (fs.Directory, error) {
	var info *api.Item
	var meta *Metadata
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err == fs.ErrorDirNotFound {
		// Directory does not exist so create it
		var leaf, parentID string
		leaf, parentID, err = f.dirCache.FindPath(ctx, dir, true)
		if err != nil {
			return nil, err
		}
		info, meta, err = f.createDir(ctx, parentID, dir, leaf, metadata)
		if err != nil {
			return nil, err
		}
		if f.driveType != driveTypePersonal {
			// for some reason, OneDrive Business needs this extra step to set modtime, while Personal does not. Seems like a bug...
			fs.Debugf(dir, "setting time %v", meta.mtime)
			info, err = meta.Write(ctx, false)
		}
	} else if err == nil {
		// Directory exists and needs updating
		info, meta, err = f.updateDir(ctx, dirID, dir, metadata)
	}
	if err != nil {
		return nil, err
	}

	// Convert the info into a directory entry
	parent, _ := dircache.SplitPath(dir)
	entry, err := f.itemToDirEntry(ctx, parent, info)
	if err != nil {
		return nil, err
	}
	directory, ok := entry.(*Directory)
	if !ok {
		return nil, fmt.Errorf("internal error: expecting %T to be a *Directory", entry)
	}
	directory.meta = meta
	f.setSystemMetadata(info, directory.meta, entry.Remote(), dirMimeType)

	dirEntry, ok := entry.(fs.Directory)
	if !ok {
		return nil, fmt.Errorf("internal error: expecting %T to be an fs.Directory", entry)
	}

	return dirEntry, nil
}

// createDir makes a directory with pathID as parent and name leaf with optional metadata
func (f *Fs) createDir(ctx context.Context, pathID, dirWithLeaf, leaf string, metadata fs.Metadata) (info *api.Item, meta *Metadata, err error) {
	// fs.Debugf(f, "CreateDir(%q, %q)\n", dirID, leaf)
	var resp *http.Response
	opts := f.newOptsCall(pathID, "POST", "/children")

	mkdir := api.CreateItemWithMetadataRequest{
		CreateItemRequest: api.CreateItemRequest{
			Name:             f.opt.Enc.FromStandardName(leaf),
			ConflictBehavior: "fail",
		},
	}
	m := f.newMetadata(dirWithLeaf)
	m.mimeType = dirMimeType
	numSet := 0
	if len(metadata) > 0 {

		numSet, err = m.Set(ctx, metadata)
		if err != nil {
			return nil, m, err
		}
		if numSet > 0 {
			mkdir.Metadata = m.toAPIMetadata()
		}
	}

	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &mkdir, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, m, err
	}

	if f.needsUpdatePermissions(metadata) && numSet > 0 { // permissions must be done as a separate step
		m.normalizedID = info.GetID()
		err = m.RefreshPermissions(ctx)
		if err != nil {
			return info, m, err
		}

		err = m.WritePermissions(ctx)
		if err != nil {
			fs.Errorf(m.remote, "error writing permissions: %v", err)
			return info, m, err
		}
	}
	return info, m, nil
}

// updateDir updates an existing a directory with the metadata passed in
func (f *Fs) updateDir(ctx context.Context, dirID, remote string, metadata fs.Metadata) (info *api.Item, meta *Metadata, err error) {
	d := f.newDir(dirID, remote)
	_, err = d.meta.Set(ctx, metadata)
	if err != nil {
		return nil, nil, err
	}
	info, err = d.meta.Write(ctx, f.needsUpdatePermissions(metadata))
	return info, d.meta, err
}

func (f *Fs) newDir(dirID, remote string) (d *Directory) {
	d = &Directory{
		fs:     f,
		remote: remote,
		size:   -1,
		items:  -1,
		id:     dirID,
		meta:   f.newMetadata(remote),
	}
	d.meta.normalizedID = dirID
	return d
}

// Metadata returns metadata for a DirEntry
//
// It should return nil if there is no Metadata
func (o *Object) Metadata(ctx context.Context) (metadata fs.Metadata, err error) {
	err = o.readMetaData(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return nil, err
	}
	return o.meta.Get(ctx)
}

// DirSetModTime sets the directory modtime for dir
func (f *Fs) DirSetModTime(ctx context.Context, dir string, modTime time.Time) error {
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	d := f.newDir(dirID, dir)
	return d.SetModTime(ctx, modTime)
}

// SetModTime sets the metadata on the DirEntry to set the modification date
//
// If there is any other metadata it does not overwrite it.
func (d *Directory) SetModTime(ctx context.Context, t time.Time) error {
	btime := t
	if d.meta != nil && !d.meta.btime.IsZero() {
		btime = d.meta.btime // if we already have a non-zero btime, preserve it
	}
	d.meta = d.fs.newMetadata(d.remote) // set only the mtime and btime
	d.meta.mtime = t
	d.meta.btime = btime
	_, err := d.meta.Write(ctx, false)
	return err
}

// Metadata returns metadata for a DirEntry
//
// It should return nil if there is no Metadata
func (d *Directory) Metadata(ctx context.Context) (metadata fs.Metadata, err error) {
	return d.meta.Get(ctx)
}

// SetMetadata sets metadata for a Directory
//
// It should return fs.ErrorNotImplemented if it can't set metadata
func (d *Directory) SetMetadata(ctx context.Context, metadata fs.Metadata) error {
	_, meta, err := d.fs.updateDir(ctx, d.id, d.remote, metadata)
	d.meta = meta
	return err
}

// Fs returns read only access to the Fs that this object is part of
func (d *Directory) Fs() fs.Info {
	return d.fs
}

// String returns the name
func (d *Directory) String() string {
	return d.remote
}

// Remote returns the remote path
func (d *Directory) Remote() string {
	return d.remote
}

// ModTime returns the modification date of the file
//
// If one isn't available it returns the configured --default-dir-time
func (d *Directory) ModTime(ctx context.Context) time.Time {
	if !d.meta.mtime.IsZero() {
		return d.meta.mtime
	}
	ci := fs.GetConfig(ctx)
	return time.Time(ci.DefaultTime)
}

// Size returns the size of the file
func (d *Directory) Size() int64 {
	return d.size
}

// Items returns the count of items in this directory or this
// directory and subdirectories if known, -1 for unknown
func (d *Directory) Items() int64 {
	return d.items
}

// ID gets the optional ID
func (d *Directory) ID() string {
	return d.id
}

// MimeType returns the content type of the Object if
// known, or "" if not
func (d *Directory) MimeType(ctx context.Context) string {
	return dirMimeType
}
