package onedrive

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/backend/onedrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices" // replace with slices after go1.21 is the minimum version
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

// TestWritePermissions tests reading and writing permissions
func (f *Fs) TestWritePermissions(t *testing.T, r *fstest.Run) {
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
	if f.driveType == driveTypePersonal {
		perms, ok := actualMeta["permissions"]
		assert.False(t, ok, fmt.Sprintf("permissions metadata key was unexpectedly found: %v", perms))
		return
	}
	assert.JSONEq(t, expectedMeta["permissions"], actualMeta["permissions"])
}

// TestReadMetadata tests that all the read-only system properties are present and non-blank
func (f *Fs) TestReadMetadata(t *testing.T, r *fstest.Run) {
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
		if k == "description" && f.driveType != driveTypePersonal {
			continue // not supported
		}
		gotV, ok := actualMeta[k]
		assert.True(t, ok, fmt.Sprintf("property is missing: %v", k))
		assert.NotEmpty(t, gotV, fmt.Sprintf("property is blank: %v", k))
	}
}

// TestDirectoryMetadata tests reading and writing modtime and other metadata and permissions for directories
func (f *Fs) TestDirectoryMetadata(t *testing.T, r *fstest.Run) {
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
			if k == "description" && f.driveType != driveTypePersonal {
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
	assert.Equal(t, t1.Truncate(f.Precision()), newDst.ModTime(ctx))
	// try changing it and re-check it
	newDst, err = operations.SetDirModTime(ctx, f, newDst, "", t2)
	assert.NoError(t, err)
	assert.Equal(t, t2.Truncate(f.Precision()), newDst.ModTime(ctx))
	// ensure that f.DirSetModTime also works
	err = f.DirSetModTime(ctx, "subdir", t3)
	assert.NoError(t, err)
	entries, err := f.List(ctx, "")
	assert.NoError(t, err)
	entries.ForDir(func(dir fs.Directory) {
		if dir.Remote() == "subdir" {
			assert.True(t, t3.Truncate(f.Precision()).Equal(dir.ModTime(ctx)), fmt.Sprintf("got %v", dir.ModTime(ctx)))
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
			if f.driveType != driveTypePersonal {
				continue // not supported
			}
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
