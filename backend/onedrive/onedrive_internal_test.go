package onedrive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/backend/onedrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// go test -timeout 30m -run ^TestIntegration/FsMkdir/FsPutFiles/Internal$ github.com/rclone/rclone/backend/onedrive -remote TestOneDrive:meta -v
// go test -timeout 30m -run ^TestIntegration/FsMkdir/FsPutFiles/Internal$ github.com/rclone/rclone/backend/onedrive -remote TestOneDriveBusiness:meta -v
// go run ./fstest/test_all -remotes TestOneDriveBusiness:meta,TestOneDrive:meta -verbose -maxtries 1

var (
	t1      = fstest.Time("2023-08-26T23:13:06.499999999Z")
	t2      = fstest.Time("2020-02-29T12:34:56.789Z")
	t3      = time.Date(1994, time.December, 24, 9+12, 0, 0, 525600, time.FixedZone("Eastern Standard Time", -5))
	ctx     = context.Background()
	content = "hello"
)

const (
	testUserID = "ryan@contoso.com" // demo user from doc examples (can't share files with yourself)
	// https://learn.microsoft.com/en-us/onedrive/developer/rest-api/api/driveitem_invite?view=odsp-graph-online#http-request-1
)

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

func TestTenantAPIEndpoint(t *testing.T) {
	for _, tc := range []struct {
		name             string
		tenantURL        string
		tenantAPIVersion string
		want             string
	}{
		{
			name:      "default",
			tenantURL: "https://example-my.sharepoint.com/_api",
			want:      "https://example-my.sharepoint.com/_api/v2.0",
		}, {
			name:             "explicit v2.1",
			tenantURL:        "https://example-my.sharepoint.com/_api",
			tenantAPIVersion: "v2.1",
			want:             "https://example-my.sharepoint.com/_api/v2.1",
		}, {
			name:             "future version",
			tenantURL:        "https://example-my.sharepoint.com/_api",
			tenantAPIVersion: "v2.2",
			want:             "https://example-my.sharepoint.com/_api/v2.2",
		}, {
			name:             "trailing slash",
			tenantURL:        "https://example-my.sharepoint.com/_api/",
			tenantAPIVersion: "/v2.1",
			want:             "https://example-my.sharepoint.com/_api/v2.1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tenantAPIEndpoint(tc.tenantURL, tc.tenantAPIVersion))
		})
	}
}

// sharingRefused caches whether the remote refuses sharing invitations.
var sharingRefused *bool

// skipIfSharingRefused skips t if the remote cannot add permissions via
// the driveItem invite API.
//
// Microsoft has been progressively disabling sharing invitations on
// both Business and Personal accounts (the invite API returns 400
// sharingFailed for any recipient on affected accounts), which makes
// the permission writing tests impossible.
func (f *Fs) skipIfSharingRefused(t *testing.T, r *fstest.Run) {
	if sharingRefused == nil {
		file := r.WriteObject(ctx, randomFilename(), "sharing probe", t2)
		obj, err := r.Fremote.NewObject(ctx, file.Path)
		require.NoError(t, err)
		m := f.newMetadata(obj.Remote())
		m.normalizedID = obj.(*Object).id
		p := defaultPermissions(f.driveType)[0]
		p.Roles[0] = api.ReadRole
		_, _, err = m.addPermission(ctx, p)
		_ = obj.Remove(ctx)
		refused := err != nil && strings.Contains(err.Error(), "sharingFailed")
		sharingRefused = &refused
	}
	if *sharingRefused {
		t.Skip("skipping test: server refuses sharing invitations (sharingFailed)")
	}
}

// TestWritePermissions tests reading and writing permissions
func (f *Fs) TestWritePermissions(t *testing.T, r *fstest.Run) {
	f.skipIfSharingRefused(t, r)

	// setup
	ctx, ci := fs.AddConfig(ctx)
	ci.Metadata = true
	_ = f.opt.MetadataPermissions.Set("read,write")
	file1 := r.WriteFile(randomFilename(), content, t2)

	// add a permission with "read" role
	permissions := defaultPermissions(f.driveType)
	permissions[0].Roles[0] = api.ReadRole
	expectedMeta, actualMeta := f.putWithMeta(ctx, t, &file1, permissions)
	f.compareMeta(t, expectedMeta, actualMeta, false)
	expectedP, actualP := unmarshalPerms(t, expectedMeta["permissions"]), unmarshalPerms(t, actualMeta["permissions"])

	found, num := false, 0
	foundCount := 0
	for i, p := range actualP {
		for _, identity := range p.GetGrantedToIdentities(f.driveType) {
			if identity.User.DisplayName == testUserID {
				// note: expected will always be element 0 here, but actual may be variable based on org settings
				assert.Equal(t, expectedP[0].Roles, p.Roles)
				found, num = true, i
				foundCount++
			}
		}
		if f.driveType == driveTypePersonal {
			if p.GetGrantedTo(f.driveType) != nil && p.GetGrantedTo(f.driveType).User != (api.Identity{}) && p.GetGrantedTo(f.driveType).User.ID == testUserID { // shows up in a different place on biz vs. personal
				assert.Equal(t, expectedP[0].Roles, p.Roles)
				found, num = true, i
				foundCount++
			}
		}
	}
	assert.True(t, found, fmt.Sprintf("no permission found with expected role (want: \n\n%v \n\ngot: \n\n%v\n\n)", indent(t, expectedMeta["permissions"]), indent(t, actualMeta["permissions"])))
	assert.Equal(t, 1, foundCount, "expected to find exactly 1 match")

	// update it to "write"
	permissions = actualP
	permissions[num].Roles[0] = api.WriteRole
	expectedMeta, actualMeta = f.putWithMeta(ctx, t, &file1, permissions)
	f.compareMeta(t, expectedMeta, actualMeta, false)
	if f.driveType != driveTypePersonal {
		// zero out some things we expect to be different
		expectedP, actualP = unmarshalPerms(t, expectedMeta["permissions"]), unmarshalPerms(t, actualMeta["permissions"])
		normalize(expectedP)
		normalize(actualP)
		expectedMeta.Set("permissions", marshalPerms(t, expectedP))
		actualMeta.Set("permissions", marshalPerms(t, actualP))
	}
	assert.JSONEq(t, expectedMeta["permissions"], actualMeta["permissions"])

	// remove it
	permissions[num] = nil
	_, actualMeta = f.putWithMeta(ctx, t, &file1, permissions)
	if f.driveType == driveTypePersonal {
		perms, ok := actualMeta["permissions"]
		assert.False(t, ok, fmt.Sprintf("permissions metadata key was unexpectedly found: %v", perms))
		return
	}
	_, actualP = unmarshalPerms(t, expectedMeta["permissions"]), unmarshalPerms(t, actualMeta["permissions"])

	found = false
	var foundP *api.PermissionsType
	for _, p := range actualP {
		if p.GetGrantedTo(f.driveType) == nil || p.GetGrantedTo(f.driveType).User == (api.Identity{}) || p.GetGrantedTo(f.driveType).User.ID != testUserID {
			continue
		}
		found = true
		foundP = p
	}
	assert.False(t, found, fmt.Sprintf("permission was found but expected to be removed: %v", foundP))
}

// TestUploadSinglePart tests reading/writing permissions using uploadSinglepart()
// This is only used when file size is exactly 0.
func (f *Fs) TestUploadSinglePart(t *testing.T, r *fstest.Run) {
	content = ""
	f.TestWritePermissions(t, r)
	content = "hello"
}

// TestReadPermissions tests that no permissions are written when --onedrive-metadata-permissions has "read" but not "write"
func (f *Fs) TestReadPermissions(t *testing.T, r *fstest.Run) {
	// setup
	ctx, ci := fs.AddConfig(ctx)
	ci.Metadata = true
	file1 := r.WriteFile(randomFilename(), "hello", t2)

	// try adding a permission without --onedrive-metadata-permissions -- should fail
	// test that what we got before vs. after is the same
	_ = f.opt.MetadataPermissions.Set("read")
	_, expectedMeta := f.putWithMeta(ctx, t, &file1, []*api.PermissionsType{}) // return var intentionally switched here
	permissions := defaultPermissions(f.driveType)
	_, actualMeta := f.putWithMeta(ctx, t, &file1, permissions)
	assert.JSONEq(t, expectedMeta["permissions"], actualMeta["permissions"])
}

// TestReadMetadata tests that all the read-only system properties are present and non-blank
func (f *Fs) TestReadMetadata(t *testing.T, r *fstest.Run) {
	f.skipIfSharingRefused(t, r)

	// setup
	ctx, ci := fs.AddConfig(ctx)
	ci.Metadata = true
	file1 := r.WriteFile(randomFilename(), "hello", t2)
	permissions := defaultPermissions(f.driveType)

	_ = f.opt.MetadataPermissions.Set("read,write")
	_, actualMeta := f.putWithMeta(ctx, t, &file1, permissions)
	optionals := []string{"package-type", "shared-by-id", "shared-scope", "shared-time", "shared-owner-id"} // not always present
	for k := range systemMetadataInfo {
		if slices.Contains(optionals, k) {
			continue
		}
		if k == "description" {
			continue // not supported
		}
		gotV, ok := actualMeta[k]
		assert.True(t, ok, fmt.Sprintf("property is missing: %v", k))
		assert.NotEmpty(t, gotV, fmt.Sprintf("property is blank: %v", k))
	}
}

// TestDirectoryMetadata tests reading and writing modtime and other metadata and permissions for directories
func (f *Fs) TestDirectoryMetadata(t *testing.T, r *fstest.Run) {
	f.skipIfSharingRefused(t, r)

	// setup
	ctx, ci := fs.AddConfig(ctx)
	ci.Metadata = true
	_ = f.opt.MetadataPermissions.Set("read,write")
	permissions := defaultPermissions(f.driveType)
	permissions[0].Roles[0] = api.ReadRole

	expectedMeta := fs.Metadata{
		"mtime":        t1.Format(timeFormatOut),
		"btime":        t2.Format(timeFormatOut),
		"content-type": dirMimeType,
		"description":  "that is so meta!",
	}
	b, err := json.MarshalIndent(permissions, "", "\t")
	assert.NoError(t, err)
	expectedMeta.Set("permissions", string(b))

	compareDirMeta := func(expectedMeta, actualMeta fs.Metadata, ignoreID bool) {
		f.compareMeta(t, expectedMeta, actualMeta, ignoreID)

		// check that all required system properties are present
		optionals := []string{"package-type", "shared-by-id", "shared-scope", "shared-time", "shared-owner-id"} // not always present
		for k := range systemMetadataInfo {
			if slices.Contains(optionals, k) {
				continue
			}
			if k == "description" {
				continue // not supported
			}
			gotV, ok := actualMeta[k]
			assert.True(t, ok, fmt.Sprintf("property is missing: %v", k))
			assert.NotEmpty(t, gotV, fmt.Sprintf("property is blank: %v", k))
		}
	}
	newDst, err := operations.MkdirMetadata(ctx, f, "subdir", expectedMeta)
	assert.NoError(t, err)
	require.NotNil(t, newDst)
	assert.Equal(t, "subdir", newDst.Remote())

	actualMeta, err := fs.GetMetadata(ctx, newDst)
	assert.NoError(t, err)
	assert.NotNil(t, actualMeta)
	compareDirMeta(expectedMeta, actualMeta, false)

	// modtime
	fstest.AssertTimeEqualWithPrecision(t, newDst.Remote(), t1, newDst.ModTime(ctx), f.Precision())
	// try changing it and re-check it
	newDst, err = operations.SetDirModTime(ctx, f, newDst, "", t2)
	assert.NoError(t, err)
	fstest.AssertTimeEqualWithPrecision(t, newDst.Remote(), t2, newDst.ModTime(ctx), f.Precision())
	// ensure that f.DirSetModTime also works
	err = f.DirSetModTime(ctx, "subdir", t3)
	assert.NoError(t, err)
	entries, err := f.List(ctx, "")
	assert.NoError(t, err)
	entries.ForDir(func(dir fs.Directory) {
		if dir.Remote() == "subdir" {
			fstest.AssertTimeEqualWithPrecision(t, dir.Remote(), t3, dir.ModTime(ctx), f.Precision())
		}
	})

	// test updating metadata on existing dir
	actualMeta, err = fs.GetMetadata(ctx, newDst) // get fresh info as we've been changing modtimes
	assert.NoError(t, err)
	expectedMeta = actualMeta
	expectedMeta.Set("description", "metadata is fun!")
	expectedMeta.Set("btime", t3.Format(timeFormatOut))
	expectedMeta.Set("mtime", t1.Format(timeFormatOut))
	expectedMeta.Set("content-type", dirMimeType)
	perms := unmarshalPerms(t, expectedMeta["permissions"])
	perms[0].Roles[0] = api.WriteRole
	b, err = json.MarshalIndent(perms, "", "\t")
	assert.NoError(t, err)
	expectedMeta.Set("permissions", string(b))

	newDst, err = operations.MkdirMetadata(ctx, f, "subdir", expectedMeta)
	assert.NoError(t, err)
	require.NotNil(t, newDst)
	assert.Equal(t, "subdir", newDst.Remote())

	actualMeta, err = fs.GetMetadata(ctx, newDst)
	assert.NoError(t, err)
	assert.NotNil(t, actualMeta)
	compareDirMeta(expectedMeta, actualMeta, false)

	// test copying metadata from one dir to another
	copiedDir, err := operations.CopyDirMetadata(ctx, f, nil, "subdir2", newDst)
	assert.NoError(t, err)
	require.NotNil(t, copiedDir)
	assert.Equal(t, "subdir2", copiedDir.Remote())

	actualMeta, err = fs.GetMetadata(ctx, copiedDir)
	assert.NoError(t, err)
	assert.NotNil(t, actualMeta)
	compareDirMeta(expectedMeta, actualMeta, true)

	// test DirModTimeUpdatesOnWrite
	expectedTime := copiedDir.ModTime(ctx)
	assert.True(t, !expectedTime.IsZero())
	r.WriteObject(ctx, copiedDir.Remote()+"/"+randomFilename(), "hi there", t3)
	entries, err = f.List(ctx, "")
	assert.NoError(t, err)
	entries.ForDir(func(dir fs.Directory) {
		if dir.Remote() == copiedDir.Remote() {
			assert.True(t, expectedTime.Equal(dir.ModTime(ctx)), fmt.Sprintf("want %v got %v", expectedTime, dir.ModTime(ctx)))
		}
	})
}

// TestServerSideCopyMove tests server-side Copy and Move
func (f *Fs) TestServerSideCopyMove(t *testing.T, r *fstest.Run) {
	f.skipIfSharingRefused(t, r)

	// setup
	ctx, ci := fs.AddConfig(ctx)
	ci.Metadata = true
	_ = f.opt.MetadataPermissions.Set("read,write")
	file1 := r.WriteFile(randomFilename(), content, t2)

	// add a permission with "read" role
	permissions := defaultPermissions(f.driveType)
	permissions[0].Roles[0] = api.ReadRole
	expectedMeta, actualMeta := f.putWithMeta(ctx, t, &file1, permissions)
	f.compareMeta(t, expectedMeta, actualMeta, false)

	comparePerms := func(expectedMeta, actualMeta fs.Metadata) (newExpectedMeta, newActualMeta fs.Metadata) {
		expectedP, actualP := unmarshalPerms(t, expectedMeta["permissions"]), unmarshalPerms(t, actualMeta["permissions"])
		normalize(expectedP)
		normalize(actualP)
		expectedMeta.Set("permissions", marshalPerms(t, expectedP))
		actualMeta.Set("permissions", marshalPerms(t, actualP))
		assert.JSONEq(t, expectedMeta["permissions"], actualMeta["permissions"])
		return expectedMeta, actualMeta
	}

	// Copy
	obj1, err := f.NewObject(ctx, file1.Path)
	assert.NoError(t, err)
	originalMeta := actualMeta
	obj2, err := f.Copy(ctx, obj1, randomFilename())
	assert.NoError(t, err)
	actualMeta, err = fs.GetMetadata(ctx, obj2)
	assert.NoError(t, err)
	expectedMeta, actualMeta = comparePerms(originalMeta, actualMeta)
	f.compareMeta(t, expectedMeta, actualMeta, true)

	// Move
	obj3, err := f.Move(ctx, obj1, randomFilename())
	assert.NoError(t, err)
	actualMeta, err = fs.GetMetadata(ctx, obj3)
	assert.NoError(t, err)
	expectedMeta, actualMeta = comparePerms(originalMeta, actualMeta)
	f.compareMeta(t, expectedMeta, actualMeta, true)
}

// TestMetadataMapper tests adding permissions with the --metadata-mapper
func (f *Fs) TestMetadataMapper(t *testing.T, r *fstest.Run) {
	f.skipIfSharingRefused(t, r)

	// setup
	ctx, ci := fs.AddConfig(ctx)
	ci.Metadata = true
	_ = f.opt.MetadataPermissions.Set("read,write")
	file1 := r.WriteFile(randomFilename(), content, t2)

	blob := `{"Metadata":{"permissions":"[{\"grantedToIdentities\":[{\"user\":{\"id\":\"ryan@contoso.com\"}}],\"roles\":[\"read\"]}]"}}`
	if f.driveType != driveTypePersonal {
		blob = `{"Metadata":{"permissions":"[{\"grantedToIdentitiesV2\":[{\"user\":{\"id\":\"ryan@contoso.com\"}}],\"roles\":[\"read\"]}]"}}`
	}

	// Copy
	ci.MetadataMapper = []string{"echo", blob}
	require.NoError(t, ci.Dump.Set("mapper"))
	obj1, err := r.Flocal.NewObject(ctx, file1.Path)
	assert.NoError(t, err)
	obj2, err := operations.Copy(ctx, f, nil, randomFilename(), obj1)
	assert.NoError(t, err)
	actualMeta, err := fs.GetMetadata(ctx, obj2)
	assert.NoError(t, err)

	actualP := unmarshalPerms(t, actualMeta["permissions"])
	found := false
	foundCount := 0
	for _, p := range actualP {
		for _, identity := range p.GetGrantedToIdentities(f.driveType) {
			if identity.User.DisplayName == testUserID {
				assert.Equal(t, []api.Role{api.ReadRole}, p.Roles)
				found = true
				foundCount++
			}
		}
		if f.driveType == driveTypePersonal {
			if p.GetGrantedTo(f.driveType) != nil && p.GetGrantedTo(f.driveType).User != (api.Identity{}) && p.GetGrantedTo(f.driveType).User.ID == testUserID { // shows up in a different place on biz vs. personal
				assert.Equal(t, []api.Role{api.ReadRole}, p.Roles)
				found = true
				foundCount++
			}
		}
	}
	assert.True(t, found, fmt.Sprintf("no permission found with expected role (want: \n\n%v \n\ngot: \n\n%v\n\n)", blob, actualMeta))
	assert.Equal(t, 1, foundCount, "expected to find exactly 1 match")
}

// helper function to put an object with metadata and permissions
func (f *Fs) putWithMeta(ctx context.Context, t *testing.T, file *fstest.Item, perms []*api.PermissionsType) (expectedMeta, actualMeta fs.Metadata) {
	t.Helper()
	expectedMeta = fs.Metadata{
		"mtime":       t1.Format(timeFormatOut),
		"btime":       t2.Format(timeFormatOut),
		"description": "that is so meta!",
	}

	expectedMeta.Set("permissions", marshalPerms(t, perms))
	obj := fstests.PutTestContentsMetadata(ctx, t, f, file, false, content, true, "plain/text", expectedMeta)
	do, ok := obj.(fs.Metadataer)
	require.True(t, ok)
	actualMeta, err := do.Metadata(ctx)
	require.NoError(t, err)
	return expectedMeta, actualMeta
}

func randomFilename() string {
	return "some file-" + random.String(8) + ".txt"
}

func (f *Fs) compareMeta(t *testing.T, expectedMeta, actualMeta fs.Metadata, ignoreID bool) {
	t.Helper()
	for k, v := range expectedMeta {
		gotV, ok := actualMeta[k]
		switch k {
		case "shared-owner-id", "shared-time", "shared-by-id", "shared-scope":
			continue
		case "permissions":
			continue
		case "utime":
			assert.True(t, ok, fmt.Sprintf("expected metadata key is missing: %v", k))
			if f.driveType == driveTypePersonal {
				compareTimeStrings(t, k, v, gotV, time.Minute) // read-only upload time, so slight difference expected -- use larger precision
				continue
			}
			compareTimeStrings(t, k, expectedMeta["btime"], gotV, time.Minute) // another bizarre difference between personal and business...
			continue
		case "id":
			if ignoreID {
				continue // different id is expected when copying meta from one item to another
			}
		case "mtime", "btime":
			assert.True(t, ok, fmt.Sprintf("expected metadata key is missing: %v", k))
			compareTimeStrings(t, k, v, gotV, time.Second)
			continue
		case "description":
			continue // not supported
		}
		assert.True(t, ok, fmt.Sprintf("expected metadata key is missing: %v", k))
		assert.Equal(t, v, gotV, actualMeta)
	}
}

func compareTimeStrings(t *testing.T, remote, want, got string, precision time.Duration) {
	wantT, err := time.Parse(timeFormatIn, want)
	assert.NoError(t, err)
	gotT, err := time.Parse(timeFormatIn, got)
	assert.NoError(t, err)
	fstest.AssertTimeEqualWithPrecision(t, remote, wantT, gotT, precision)
}

func marshalPerms(t *testing.T, p []*api.PermissionsType) string {
	b, err := json.MarshalIndent(p, "", "\t")
	assert.NoError(t, err)
	return string(b)
}

func unmarshalPerms(t *testing.T, perms string) (p []*api.PermissionsType) {
	t.Helper()
	err := json.Unmarshal([]byte(perms), &p)
	assert.NoError(t, err)
	return p
}

func indent(t *testing.T, s string) string {
	p := unmarshalPerms(t, s)
	return marshalPerms(t, p)
}

func defaultPermissions(driveType string) []*api.PermissionsType {
	if driveType == driveTypePersonal {
		return []*api.PermissionsType{{
			GrantedTo:           &api.IdentitySet{User: api.Identity{}},
			GrantedToIdentities: []*api.IdentitySet{{User: api.Identity{ID: testUserID}}},
			Roles:               []api.Role{api.WriteRole},
		}}
	}
	return []*api.PermissionsType{{
		GrantedToV2:           &api.IdentitySet{User: api.Identity{}},
		GrantedToIdentitiesV2: []*api.IdentitySet{{User: api.Identity{ID: testUserID}}},
		Roles:                 []api.Role{api.WriteRole},
	}}
}

// zeroes out some things we expect to be different when copying/moving between objects
func normalize(Ps []*api.PermissionsType) {
	for _, ep := range Ps {
		ep.ID = ""
		ep.Link = nil
		ep.ShareID = ""
	}
}

func (f *Fs) resetTestDefaults(r *fstest.Run) {
	ci := fs.GetConfig(ctx)
	ci.Metadata = false
	_ = f.opt.MetadataPermissions.Set("off")
	r.Finalise()
}

// TestCheckUploadCutoff checks the bounds of the --onedrive-upload-cutoff
// validation, in particular that the documented API maximum is accepted.
func TestCheckUploadCutoff(t *testing.T) {
	for _, test := range []struct {
		in      fs.SizeSuffix
		wantErr bool
	}{
		{in: 0, wantErr: false},
		{in: 4 * fs.Mebi, wantErr: false},
		// maxSinglePartSize is a legal cutoff even though the server refuses a
		// body of that size, because the cutoff is an exclusive threshold: at
		// this setting a file of exactly maxSinglePartSize goes multipart.
		{in: maxSinglePartSize, wantErr: false},
		{in: maxSinglePartSize + 1, wantErr: true},
	} {
		err := checkUploadCutoff(test.in)
		if test.wantErr {
			assert.Error(t, err, test.in.String())
		} else {
			assert.NoError(t, err, test.in.String())
		}
	}
}

// InternalTest dispatches all internal tests
func (f *Fs) InternalTest(t *testing.T) {
	newTestF := func() (*Fs, *fstest.Run) {
		r := fstest.NewRunIndividual(t)
		testF, ok := r.Fremote.(*Fs)
		if !ok {
			t.FailNow()
		}
		return testF, r
	}

	testF, r := newTestF()
	t.Run("TestWritePermissions", func(t *testing.T) { testF.TestWritePermissions(t, r) })
	testF.resetTestDefaults(r)
	testF, r = newTestF()
	t.Run("TestUploadSinglePart", func(t *testing.T) { testF.TestUploadSinglePart(t, r) })
	testF.resetTestDefaults(r)
	testF, r = newTestF()
	t.Run("TestReadPermissions", func(t *testing.T) { testF.TestReadPermissions(t, r) })
	testF.resetTestDefaults(r)
	testF, r = newTestF()
	t.Run("TestReadMetadata", func(t *testing.T) { testF.TestReadMetadata(t, r) })
	testF.resetTestDefaults(r)
	testF, r = newTestF()
	t.Run("TestDirectoryMetadata", func(t *testing.T) { testF.TestDirectoryMetadata(t, r) })
	testF.resetTestDefaults(r)
	testF, r = newTestF()
	t.Run("TestServerSideCopyMove", func(t *testing.T) { testF.TestServerSideCopyMove(t, r) })
	testF.resetTestDefaults(r)
	t.Run("TestMetadataMapper", func(t *testing.T) { testF.TestMetadataMapper(t, r) })
	testF.resetTestDefaults(r)
}

var _ fstests.InternalTester = (*Fs)(nil)

// newChangeNotifyTestFs returns an Fs whose delta listing is served by
// handler on a local test server, plus the server itself. The tenant URL
// is pointed at the server so no request leaves the test.
func newChangeNotifyTestFs(t *testing.T, handler http.Handler) (*Fs, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	ctx := context.Background()
	f := &Fs{
		ci:      fs.GetConfig(ctx),
		root:    "",
		driveID: "testdrive",
		srv:     rest.NewClient(server.Client()).SetRoot(server.URL),
		pacer:   fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(time.Millisecond), pacer.MaxSleep(10*time.Millisecond))),
	}
	f.opt.TenantURL = server.URL
	f.opt.TenantAPIVersion = defaultTenantAPIVersion
	return f, server
}

// deltaItem builds a delta entry for a file named name inside a folder.
func deltaItem(name, folder string) api.Item {
	return api.Item{
		ID:   name,
		Name: name,
		File: &api.FileFacet{},
		ParentReference: &api.ItemReference{
			ID:   "parentid",
			Path: "driveid:/" + folder,
		},
	}
}

// TestChangeNotifyRetriesAndKeepsToken covers a transient failure of the
// delta poll: the request must go through the pacer so it is retried, and
// a poll which fails outright must not hand back an empty token, since the
// caller stores whatever it returns. An empty token would reset the delta
// listing to the start and silently lose every change in between.
func TestChangeNotifyRetriesAndKeepsToken(t *testing.T) {
	ctx := context.Background()
	var attempts atomic.Int32
	var tokens []string
	fail := true
	f, _ := newChangeNotifyTestFs(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		tokens = append(tokens, r.URL.Query().Get("token"))
		if fail {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
				"code":    "serviceNotAvailable",
				"message": "Service unavailable",
			}})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.DeltaResponse{
			Value:     []api.Item{deltaItem("file1.txt", "folder")},
			DeltaLink: "https://example.com/delta?token=newtoken",
		})
	}))

	var notified []string
	notifyFunc := func(path string, entryType fs.EntryType) {
		notified = append(notified, path)
	}

	// The poll fails, but is retried by the pacer first
	token, err := f.changeNotifyRunner(ctx, notifyFunc, "tok1")
	require.Error(t, err)
	assert.Empty(t, token, "a failed poll must not return a token")
	assert.Equal(t, fs.GetConfig(ctx).LowLevelRetries, int(attempts.Load()), "the failed poll should have been retried by the pacer")
	assert.Equal(t, slices.Repeat([]string{"tok1"}, int(attempts.Load())), tokens, "every retry should resume from the same token")
	assert.Empty(t, notified)

	// Keeping the previous token (as ChangeNotify does) resumes correctly
	fail = false
	attempts.Store(0)
	tokens = nil
	token, err = f.changeNotifyRunner(ctx, notifyFunc, "tok1")
	require.NoError(t, err)
	assert.Equal(t, "newtoken", token)
	assert.Equal(t, []string{"tok1"}, tokens, "the second poll should resume from the retained token")
	assert.Equal(t, []string{"folder/file1.txt"}, notified)
}

// TestChangeNotifyFollowsNextLink covers a change set which spans more than
// one page. Graph returns @odata.nextLink and no @odata.deltaLink until the
// last page, so a runner which stops after the first page both loses the
// remaining changes and ends up with an empty resume token.
func TestChangeNotifyFollowsNextLink(t *testing.T) {
	ctx := context.Background()
	var urls []string
	var server *httptest.Server
	f, server := newChangeNotifyTestFs(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urls = append(urls, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch len(urls) {
		case 1:
			_ = json.NewEncoder(w).Encode(api.DeltaResponse{
				Value:    []api.Item{deltaItem("page1.txt", "folder")},
				NextLink: server.URL + "/nextpage?$skiptoken=abc",
			})
		case 2:
			_ = json.NewEncoder(w).Encode(api.DeltaResponse{
				Value:     []api.Item{deltaItem("page2.txt", "folder")},
				DeltaLink: "https://example.com/delta?token=lasttoken",
			})
		default:
			t.Errorf("unexpected extra request to %s", r.URL)
		}
	}))

	var notified []string
	token, err := f.changeNotifyRunner(ctx, func(path string, entryType fs.EntryType) {
		notified = append(notified, path)
	}, "tok1")
	require.NoError(t, err)

	assert.Equal(t, "lasttoken", token, "the resume token should come from the last page")
	assert.Equal(t, []string{"folder/page1.txt", "folder/page2.txt"}, notified, "changes from every page should be notified")
	require.Len(t, urls, 2, "the nextLink page should have been fetched")
	assert.Contains(t, urls[0], "/testdrive/root/delta")
	assert.Contains(t, urls[0], "token=tok1")
	assert.Equal(t, "/nextpage?$skiptoken=abc", urls[1], "the nextLink should be used as-is, without the delta token")
}
