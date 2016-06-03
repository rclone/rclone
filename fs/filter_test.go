package fs

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgeSuffix(t *testing.T) {
	for i, test := range []struct {
		in   string
		want float64
		err  bool
	}{
		{"0", 0, false},
		{"", 0, true},
		{"1ms", float64(time.Millisecond), false},
		{"1s", float64(time.Second), false},
		{"1m", float64(time.Minute), false},
		{"1h", float64(time.Hour), false},
		{"1d", float64(time.Hour) * 24, false},
		{"1w", float64(time.Hour) * 24 * 7, false},
		{"1M", float64(time.Hour) * 24 * 30, false},
		{"1y", float64(time.Hour) * 24 * 365, false},
		{"1.5y", float64(time.Hour) * 24 * 365 * 1.5, false},
		{"-1s", -float64(time.Second), false},
		{"1.s", float64(time.Second), false},
		{"1x", 0, true},
	} {
		duration, err := ParseDuration(test.in)
		if (err != nil) != test.err {
			t.Errorf("%d: Expecting error %v but got error %v", i, test.err, err)
			continue
		}

		got := float64(duration)
		if test.want != got {
			t.Errorf("%d: Want %v got %v", i, test.want, got)
		}
	}
}

func TestNewFilterDefault(t *testing.T) {
	f, err := NewFilter()
	require.NoError(t, err)
	assert.False(t, f.DeleteExcluded)
	assert.Equal(t, int64(-1), f.MinSize)
	assert.Equal(t, int64(-1), f.MaxSize)
	assert.Len(t, f.fileRules.rules, 0)
	assert.Len(t, f.dirRules.rules, 0)
	assert.Nil(t, f.files)
	assert.True(t, f.InActive())
}

// return a pointer to the string
func stringP(s string) *string {
	return &s
}

// testFile creates a temp file with the contents
func testFile(t *testing.T, contents string) *string {
	out, err := ioutil.TempFile("", "filter_test")
	require.NoError(t, err)
	defer func() {
		err := out.Close()
		if err != nil {
			t.Error(err)
		}
	}()
	_, err = out.Write([]byte(contents))
	require.NoError(t, err)
	s := out.Name()
	return &s
}

func TestNewFilterFull(t *testing.T) {
	mins := int64(100 * 1024)
	maxs := int64(1000 * 1024)
	emptyString := ""
	isFalse := false
	isTrue := true

	// Set up the input
	deleteExcluded = &isTrue
	filterRule = stringP("- filter1")
	filterFrom = testFile(t, "#comment\n+ filter2\n- filter3\n")
	excludeRule = stringP("exclude1")
	excludeFrom = testFile(t, "#comment\nexclude2\nexclude3\n")
	includeRule = stringP("include1")
	includeFrom = testFile(t, "#comment\ninclude2\ninclude3\n")
	filesFrom = testFile(t, "#comment\nfiles1\nfiles2\n")
	minSize = SizeSuffix(mins)
	maxSize = SizeSuffix(maxs)

	rm := func(p string) {
		err := os.Remove(p)
		if err != nil {
			t.Logf("error removing %q: %v", p, err)
		}
	}
	// Reset the input
	defer func() {
		rm(*filterFrom)
		rm(*excludeFrom)
		rm(*includeFrom)
		rm(*filesFrom)
		minSize = -1
		maxSize = -1
		deleteExcluded = &isFalse
		filterRule = &emptyString
		filterFrom = &emptyString
		excludeRule = &emptyString
		excludeFrom = &emptyString
		includeRule = &emptyString
		includeFrom = &emptyString
		filesFrom = &emptyString
	}()

	f, err := NewFilter()
	require.NoError(t, err)
	assert.True(t, f.DeleteExcluded)
	assert.Equal(t, f.MinSize, mins)
	assert.Equal(t, f.MaxSize, maxs)
	got := f.DumpFilters()
	want := `--- File filter rules ---
+ (^|/)include1$
+ (^|/)include2$
+ (^|/)include3$
- (^|/)exclude1$
- (^|/)exclude2$
- (^|/)exclude3$
- (^|/)filter1$
+ (^|/)filter2$
- (^|/)filter3$
- ^.*$
--- Directory filter rules ---
+ ^.*$
- ^.*$`
	assert.Equal(t, want, got)
	assert.Len(t, f.files, 2)
	for _, name := range []string{"files1", "files2"} {
		_, ok := f.files[name]
		if !ok {
			t.Errorf("Didn't find file %q in f.files", name)
		}
	}
	assert.False(t, f.InActive())
}

type includeTest struct {
	in      string
	size    int64
	modTime int64
	want    bool
}

func testInclude(t *testing.T, f *Filter, tests []includeTest) {
	for _, test := range tests {
		got := f.Include(test.in, test.size, time.Unix(test.modTime, 0))
		assert.Equal(t, test.want, got, test.in, test.size, test.modTime)
	}
}

type includeDirTest struct {
	in   string
	want bool
}

func testDirInclude(t *testing.T, f *Filter, tests []includeDirTest) {
	for _, test := range tests {
		got := f.IncludeDirectory(test.in)
		assert.Equal(t, test.want, got, test.in)
	}
}

func TestNewFilterIncludeFiles(t *testing.T) {
	f, err := NewFilter()
	require.NoError(t, err)
	err = f.AddFile("file1.jpg")
	if err != nil {
		t.Error(err)
	}
	err = f.AddFile("/file2.jpg")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, filesMap{
		"file1.jpg": {},
		"file2.jpg": {},
	}, f.files)
	assert.Equal(t, filesMap{}, f.dirs)
	testInclude(t, f, []includeTest{
		{"file1.jpg", 0, 0, true},
		{"file2.jpg", 1, 0, true},
		{"potato/file2.jpg", 2, 0, false},
		{"file3.jpg", 3, 0, false},
	})
	if f.InActive() {
		t.Errorf("want !InActive")
	}
}

func TestNewFilterIncludeFilesDirs(t *testing.T) {
	f, err := NewFilter()
	require.NoError(t, err)
	for _, path := range []string{
		"path/to/dir/file1.png",
		"/path/to/dir/file2.png",
		"/path/to/file3.png",
		"/path/to/dir2/file4.png",
	} {
		err = f.AddFile(path)
		if err != nil {
			t.Error(err)
		}
	}
	assert.Equal(t, filesMap{
		"path":         {},
		"path/to":      {},
		"path/to/dir":  {},
		"path/to/dir2": {},
	}, f.dirs)
	testDirInclude(t, f, []includeDirTest{
		{"path", true},
		{"path/to", true},
		{"path/to/", true},
		{"/path/to", true},
		{"/path/to/", true},
		{"path/to/dir", true},
		{"path/to/dir2", true},
		{"path/too", false},
		{"path/three", false},
		{"four", false},
	})
}

func TestNewFilterMinSize(t *testing.T) {
	f, err := NewFilter()
	require.NoError(t, err)
	f.MinSize = 100
	testInclude(t, f, []includeTest{
		{"file1.jpg", 100, 0, true},
		{"file2.jpg", 101, 0, true},
		{"potato/file2.jpg", 99, 0, false},
	})
	if f.InActive() {
		t.Errorf("want !InActive")
	}
}

func TestNewFilterMaxSize(t *testing.T) {
	f, err := NewFilter()
	require.NoError(t, err)
	f.MaxSize = 100
	testInclude(t, f, []includeTest{
		{"file1.jpg", 100, 0, true},
		{"file2.jpg", 101, 0, false},
		{"potato/file2.jpg", 99, 0, true},
	})
	if f.InActive() {
		t.Errorf("want !InActive")
	}
}

func TestNewFilterMinAndMaxAge(t *testing.T) {
	f, err := NewFilter()
	require.NoError(t, err)
	f.ModTimeFrom = time.Unix(1440000002, 0)
	f.ModTimeTo = time.Unix(1440000003, 0)
	testInclude(t, f, []includeTest{
		{"file1.jpg", 100, 1440000000, false},
		{"file2.jpg", 101, 1440000001, false},
		{"file3.jpg", 102, 1440000002, true},
		{"potato/file1.jpg", 98, 1440000003, true},
		{"potato/file2.jpg", 99, 1440000004, false},
	})
	if f.InActive() {
		t.Errorf("want !InActive")
	}
}

func TestNewFilterMinAge(t *testing.T) {
	f, err := NewFilter()
	require.NoError(t, err)
	f.ModTimeTo = time.Unix(1440000002, 0)
	testInclude(t, f, []includeTest{
		{"file1.jpg", 100, 1440000000, true},
		{"file2.jpg", 101, 1440000001, true},
		{"file3.jpg", 102, 1440000002, true},
		{"potato/file1.jpg", 98, 1440000003, false},
		{"potato/file2.jpg", 99, 1440000004, false},
	})
	if f.InActive() {
		t.Errorf("want !InActive")
	}
}

func TestNewFilterMaxAge(t *testing.T) {
	f, err := NewFilter()
	require.NoError(t, err)
	f.ModTimeFrom = time.Unix(1440000002, 0)
	testInclude(t, f, []includeTest{
		{"file1.jpg", 100, 1440000000, false},
		{"file2.jpg", 101, 1440000001, false},
		{"file3.jpg", 102, 1440000002, true},
		{"potato/file1.jpg", 98, 1440000003, true},
		{"potato/file2.jpg", 99, 1440000004, true},
	})
	if f.InActive() {
		t.Errorf("want !InActive")
	}
}

func TestNewFilterMatches(t *testing.T) {
	f, err := NewFilter()
	require.NoError(t, err)
	add := func(s string) {
		err := f.AddRule(s)
		require.NoError(t, err)
	}
	add("+ cleared")
	add("!")
	add("- /file1.jpg")
	add("+ /file2.png")
	add("+ /*.jpg")
	add("- /*.png")
	add("- /potato")
	add("+ /sausage1")
	add("+ /sausage2*")
	add("+ /sausage3**")
	add("+ /a/*.jpg")
	add("- *")
	testInclude(t, f, []includeTest{
		{"cleared", 100, 0, false},
		{"file1.jpg", 100, 0, false},
		{"file2.png", 100, 0, true},
		{"afile2.png", 100, 0, false},
		{"file3.jpg", 101, 0, true},
		{"file4.png", 101, 0, false},
		{"potato", 101, 0, false},
		{"sausage1", 101, 0, true},
		{"sausage1/potato", 101, 0, false},
		{"sausage2potato", 101, 0, true},
		{"sausage2/potato", 101, 0, false},
		{"sausage3/potato", 101, 0, true},
		{"a/one.jpg", 101, 0, true},
		{"a/one.png", 101, 0, false},
		{"unicorn", 99, 0, false},
	})
	testDirInclude(t, f, []includeDirTest{
		{"sausage1", false},
		{"sausage2", false},
		{"sausage2/sub", false},
		{"sausage2/sub/dir", false},
		{"sausage3", true},
		{"sausage3/sub", true},
		{"sausage3/sub/dir", true},
		{"sausage4", false},
		{"a", true},
	})
	if f.InActive() {
		t.Errorf("want !InActive")
	}
}

func TestFilterForEachLine(t *testing.T) {
	file := testFile(t, `; comment
one
# another comment


two
 # indented comment
three  
four    
five
  six  `)
	defer func() {
		err := os.Remove(*file)
		if err != nil {
			t.Error(err)
		}
	}()
	lines := []string{}
	err := forEachLine(*file, func(s string) error {
		lines = append(lines, s)
		return nil
	})
	if err != nil {
		t.Error(err)
	}
	got := strings.Join(lines, ",")
	want := "one,two,three,four,five,six"
	if want != got {
		t.Errorf("want %q got %q", want, got)
	}
}

func TestFilterMatchesFromDocs(t *testing.T) {
	for _, test := range []struct {
		glob     string
		included bool
		file     string
	}{
		{"file.jpg", true, "file.jpg"},
		{"file.jpg", true, "directory/file.jpg"},
		{"file.jpg", false, "afile.jpg"},
		{"file.jpg", false, "directory/afile.jpg"},
		{"/file.jpg", true, "file.jpg"},
		{"/file.jpg", false, "afile.jpg"},
		{"/file.jpg", false, "directory/file.jpg"},
		{"*.jpg", true, "file.jpg"},
		{"*.jpg", true, "directory/file.jpg"},
		{"*.jpg", false, "file.jpg/anotherfile.png"},
		{"dir/**", true, "dir/file.jpg"},
		{"dir/**", true, "dir/dir1/dir2/file.jpg"},
		{"dir/**", false, "directory/file.jpg"},
		{"dir/**", false, "adir/file.jpg"},
		{"l?ss", true, "less"},
		{"l?ss", true, "lass"},
		{"l?ss", false, "floss"},
		{"h[ae]llo", true, "hello"},
		{"h[ae]llo", true, "hallo"},
		{"h[ae]llo", false, "hullo"},
		{"{one,two}_potato", true, "one_potato"},
		{"{one,two}_potato", true, "two_potato"},
		{"{one,two}_potato", false, "three_potato"},
		{"{one,two}_potato", false, "_potato"},
		{"\\*.jpg", true, "*.jpg"},
		{"\\\\.jpg", true, "\\.jpg"},
		{"\\[one\\].jpg", true, "[one].jpg"},
	} {
		f, err := NewFilter()
		require.NoError(t, err)
		err = f.Add(true, test.glob)
		require.NoError(t, err)
		err = f.Add(false, "*")
		require.NoError(t, err)
		included := f.Include(test.file, 0, time.Unix(0, 0))
		if included != test.included {
			t.Logf("%q match %q: want %v got %v", test.glob, test.file, test.included, included)
		}
	}
}
