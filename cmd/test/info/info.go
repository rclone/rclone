// Package info provides the info test command.
package info

// FIXME once translations are implemented will need a no-escape
// option for Put so we can make these tests work again

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/test"
	"github.com/rclone/rclone/cmd/test/info/internal"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/random"
	"github.com/spf13/cobra"
)

var (
	writeJSON          string
	keepTestFiles      bool
	checkNormalization bool
	checkControl       bool
	checkLength        bool
	checkStreaming     bool
	checkBase32768     bool
	all                bool
	uploadWait         time.Duration
	positionLeftRe     = regexp.MustCompile(`(?s)^(.*)-position-left-([[:xdigit:]]+)$`)
	positionMiddleRe   = regexp.MustCompile(`(?s)^position-middle-([[:xdigit:]]+)-(.*)-$`)
	positionRightRe    = regexp.MustCompile(`(?s)^position-right-([[:xdigit:]]+)-(.*)$`)
)

func init() {
	test.Command.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.StringVarP(cmdFlags, &writeJSON, "write-json", "", "", "Write results to file", "")
	flags.BoolVarP(cmdFlags, &checkNormalization, "check-normalization", "", false, "Check UTF-8 Normalization", "")
	flags.BoolVarP(cmdFlags, &checkControl, "check-control", "", false, "Check control characters", "")
	flags.DurationVarP(cmdFlags, &uploadWait, "upload-wait", "", 0, "Wait after writing a file", "")
	flags.BoolVarP(cmdFlags, &checkLength, "check-length", "", false, "Check max filename length", "")
	flags.BoolVarP(cmdFlags, &checkStreaming, "check-streaming", "", false, "Check uploads with indeterminate file size", "")
	flags.BoolVarP(cmdFlags, &checkBase32768, "check-base32768", "", false, "Check can store all possible base32768 characters", "")
	flags.BoolVarP(cmdFlags, &all, "all", "", false, "Run all tests", "")
	flags.BoolVarP(cmdFlags, &keepTestFiles, "keep-test-files", "", false, "Keep test files after execution", "")
}

var commandDefinition = &cobra.Command{
	Use:   "info [remote:path]+",
	Short: `Discovers file name or other limitations for paths.`,
	Long: `Discovers what filenames and upload methods are possible to write to the
paths passed in and how long they can be.  It can take some time.  It will
write test files into the remote:path passed in.  It outputs a bit of go
code for each one.

**NB** this can create undeletable files and other hazards - use with care
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.55",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1e6, command, args)
		if !checkNormalization && !checkControl && !checkLength && !checkStreaming && !checkBase32768 && !all {
			fs.Fatalf(nil, "no tests selected - select a test or use --all")
		}
		if all {
			checkNormalization = true
			checkControl = true
			checkLength = true
			checkStreaming = true
			checkBase32768 = true
		}
		for i := range args {
			tempDirName := "rclone-test-info-" + random.String(8)
			tempDirPath := path.Join(args[i], tempDirName)
			f := cmd.NewFsDir([]string{tempDirPath})
			fs.Infof(f, "Created temporary directory for test files: %s", tempDirPath)
			err := f.Mkdir(context.Background(), "")
			if err != nil {
				fs.Fatalf(nil, "couldn't create temporary directory: %v", err)
			}

			cmd.Run(false, false, command, func() error {
				return readInfo(context.Background(), f)
			})
		}
	},
}

type results struct {
	ctx                  context.Context
	f                    fs.Fs
	mu                   sync.Mutex
	stringNeedsEscaping  map[string]internal.Position
	controlResults       map[string]internal.ControlResult
	maxFileLength        [4]int
	canWriteUnnormalized bool
	canReadUnnormalized  bool
	canReadRenormalized  bool
	canStream            bool
	canBase32768         bool
}

func newResults(ctx context.Context, f fs.Fs) *results {
	return &results{
		ctx:                 ctx,
		f:                   f,
		stringNeedsEscaping: make(map[string]internal.Position),
		controlResults:      make(map[string]internal.ControlResult),
	}
}

// Print the results to stdout
func (r *results) Print() {
	fmt.Printf("// %s\n", r.f.Name())
	if checkControl {
		escape := []string{}
		for c, needsEscape := range r.stringNeedsEscaping {
			if needsEscape != internal.PositionNone {
				k := strconv.Quote(c)
				k = k[1 : len(k)-1]
				escape = append(escape, fmt.Sprintf("'%s'", k))
			}
		}
		sort.Strings(escape)
		fmt.Printf("stringNeedsEscaping = []rune{\n")
		fmt.Printf("\t%s\n", strings.Join(escape, ", "))
		fmt.Printf("}\n")
	}
	if checkLength {
		for i := range r.maxFileLength {
			fmt.Printf("maxFileLength = %d // for %d byte unicode characters\n", r.maxFileLength[i], i+1)
		}
	}
	if checkNormalization {
		fmt.Printf("canWriteUnnormalized = %v\n", r.canWriteUnnormalized)
		fmt.Printf("canReadUnnormalized   = %v\n", r.canReadUnnormalized)
		fmt.Printf("canReadRenormalized   = %v\n", r.canReadRenormalized)
	}
	if checkStreaming {
		fmt.Printf("canStream = %v\n", r.canStream)
	}
	if checkBase32768 {
		fmt.Printf("base32768isOK = %v // make sure maxFileLength for 2 byte unicode chars is the same as for 1 byte characters\n", r.canBase32768)
	}
}

// WriteJSON writes the results to a JSON file when requested
func (r *results) WriteJSON() {
	if writeJSON == "" {
		return
	}

	report := internal.InfoReport{
		Remote: r.f.Name(),
	}
	if checkControl {
		report.ControlCharacters = &r.controlResults
	}
	if checkLength {
		report.MaxFileLength = &r.maxFileLength[0]
	}
	if checkNormalization {
		report.CanWriteUnnormalized = &r.canWriteUnnormalized
		report.CanReadUnnormalized = &r.canReadUnnormalized
		report.CanReadRenormalized = &r.canReadRenormalized
	}
	if checkStreaming {
		report.CanStream = &r.canStream
	}

	if f, err := os.Create(writeJSON); err != nil {
		fs.Errorf(r.f, "Creating JSON file failed: %s", err)
	} else {
		defer fs.CheckClose(f, &err)
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		err := enc.Encode(report)
		if err != nil {
			fs.Errorf(r.f, "Writing JSON file failed: %s", err)
		}
	}
	fs.Infof(r.f, "Wrote JSON file: %s", writeJSON)
}

// writeFile writes a file with some random contents
func (r *results) writeFile(path string) (fs.Object, error) {
	contents := random.String(50)
	src := object.NewStaticObjectInfo(path, time.Now(), int64(len(contents)), true, nil, r.f)
	obj, err := r.f.Put(r.ctx, bytes.NewBufferString(contents), src)
	if uploadWait > 0 {
		time.Sleep(uploadWait)
	}
	return obj, err
}

// check whether normalization is enforced and check whether it is
// done on the files anyway
func (r *results) checkUTF8Normalization() {
	unnormalized := "Héroique"
	normalized := "Héroique"
	_, err := r.writeFile(unnormalized)
	if err != nil {
		r.canWriteUnnormalized = false
		return
	}
	r.canWriteUnnormalized = true
	_, err = r.f.NewObject(r.ctx, unnormalized)
	if err == nil {
		r.canReadUnnormalized = true
	}
	_, err = r.f.NewObject(r.ctx, normalized)
	if err == nil {
		r.canReadRenormalized = true
	}
}

func (r *results) checkStringPositions(k, s string) {
	fs.Infof(r.f, "Writing position file 0x%0X", s)
	positionError := internal.PositionNone
	res := internal.ControlResult{
		Text:       s,
		WriteError: make(map[internal.Position]string, 3),
		GetError:   make(map[internal.Position]string, 3),
		InList:     make(map[internal.Position]internal.Presence, 3),
	}

	for _, pos := range internal.PositionList {
		path := ""
		switch pos {
		case internal.PositionMiddle:
			path = fmt.Sprintf("position-middle-%0X-%s-", s, s)
		case internal.PositionLeft:
			path = fmt.Sprintf("%s-position-left-%0X", s, s)
		case internal.PositionRight:
			path = fmt.Sprintf("position-right-%0X-%s", s, s)
		default:
			panic("invalid position: " + pos.String())
		}
		_, writeError := r.writeFile(path)
		if writeError != nil {
			res.WriteError[pos] = writeError.Error()
			fs.Infof(r.f, "Writing %s position file 0x%0X Error: %s", pos.String(), s, writeError)
		} else {
			fs.Infof(r.f, "Writing %s position file 0x%0X OK", pos.String(), s)
		}
		obj, getErr := r.f.NewObject(r.ctx, path)
		if getErr != nil {
			res.GetError[pos] = getErr.Error()
			fs.Infof(r.f, "Getting %s position file 0x%0X Error: %s", pos.String(), s, getErr)
		} else {
			if obj.Size() != 50 {
				res.GetError[pos] = fmt.Sprintf("invalid size %d", obj.Size())
				fs.Infof(r.f, "Getting %s position file 0x%0X Invalid Size: %d", pos.String(), s, obj.Size())
			} else {
				fs.Infof(r.f, "Getting %s position file 0x%0X OK", pos.String(), s)
			}
		}
		if writeError != nil || getErr != nil {
			positionError += pos
		}
	}

	r.mu.Lock()
	r.stringNeedsEscaping[k] = positionError
	r.controlResults[k] = res
	r.mu.Unlock()
}

// check we can write a file with the control chars
func (r *results) checkControls() {
	fs.Infof(r.f, "Trying to create control character file names")
	ci := fs.GetConfig(context.Background())

	// Concurrency control
	tokens := make(chan struct{}, ci.Checkers)
	for i := 0; i < ci.Checkers; i++ {
		tokens <- struct{}{}
	}
	var wg sync.WaitGroup
	for i := rune(0); i < 128; i++ {
		s := string(i)
		if i == 0 || i == '/' {
			// We're not even going to check NULL or /
			r.stringNeedsEscaping[s] = internal.PositionAll
			continue
		}
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			token := <-tokens
			k := s
			r.checkStringPositions(k, s)
			tokens <- token
		}(s)
	}
	for _, s := range []string{"＼", "\u00A0", "\xBF", "\xFE"} {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			token := <-tokens
			k := s
			r.checkStringPositions(k, s)
			tokens <- token
		}(s)
	}
	wg.Wait()
	r.checkControlsList()
	fs.Infof(r.f, "Done trying to create control character file names")
}

func (r *results) checkControlsList() {
	l, err := r.f.List(context.TODO(), "")
	if err != nil {
		fs.Errorf(r.f, "Listing control character file names failed: %s", err)
		return
	}

	namesMap := make(map[string]struct{}, len(l))
	for _, s := range l {
		namesMap[path.Base(s.Remote())] = struct{}{}
	}

	for path := range namesMap {
		var pos internal.Position
		var hex, value string
		if g := positionLeftRe.FindStringSubmatch(path); g != nil {
			pos, hex, value = internal.PositionLeft, g[2], g[1]
		} else if g := positionMiddleRe.FindStringSubmatch(path); g != nil {
			pos, hex, value = internal.PositionMiddle, g[1], g[2]
		} else if g := positionRightRe.FindStringSubmatch(path); g != nil {
			pos, hex, value = internal.PositionRight, g[1], g[2]
		} else {
			fs.Infof(r.f, "Unknown path %q", path)
			continue
		}
		var hexValue []byte
		for ; len(hex) >= 2; hex = hex[2:] {
			if b, err := strconv.ParseUint(hex[:2], 16, 8); err != nil {
				fs.Infof(r.f, "Invalid path %q: %s", path, err)
				continue
			} else {
				hexValue = append(hexValue, byte(b))
			}
		}
		if hex != "" {
			fs.Infof(r.f, "Invalid path %q", path)
			continue
		}

		hexStr := string(hexValue)
		k := hexStr
		switch r.controlResults[k].InList[pos] {
		case internal.Absent:
			if hexStr == value {
				r.controlResults[k].InList[pos] = internal.Present
			} else {
				r.controlResults[k].InList[pos] = internal.Renamed
			}
		case internal.Present:
			r.controlResults[k].InList[pos] = internal.Multiple
		case internal.Renamed:
			r.controlResults[k].InList[pos] = internal.Multiple
		}
		delete(namesMap, path)
	}

	if len(namesMap) > 0 {
		fs.Infof(r.f, "Found additional control character file names:")
		for name := range namesMap {
			fs.Infof(r.f, "%q", name)
		}
	}
}

// find the max file name size we can use
func (r *results) findMaxLength(characterLength int) {
	var character rune
	switch characterLength {
	case 1:
		character = 'a'
	case 2:
		character = 'á'
	case 3:
		character = '世'
	case 4:
		character = '🙂'
	default:
		panic("Bad characterLength")
	}
	if characterLength != len(string(character)) {
		panic(fmt.Sprintf("Chose the wrong character length %q is %d not %d", character, len(string(character)), characterLength))
	}
	const maxLen = 16 * 1024
	name := make([]rune, maxLen)
	for i := range name {
		name[i] = character
	}
	// Find the first size of filename we can't write
	i := sort.Search(len(name), func(i int) (fail bool) {
		defer func() {
			if err := recover(); err != nil {
				fs.Infof(r.f, "Couldn't write file with name length %d: %v", i, err)
				fail = true
			}
		}()

		path := string(name[:i])
		o, err := r.writeFile(path)
		if err != nil {
			fs.Infof(r.f, "Couldn't write file with name length %d: %v", i, err)
			return true
		}
		fs.Infof(r.f, "Wrote file with name length %d", i)
		err = o.Remove(context.Background())
		if err != nil {
			fs.Errorf(o, "Failed to remove test file")
		}
		return false
	})
	r.maxFileLength[characterLength-1] = i - 1
	fs.Infof(r.f, "Max file length is %d when writing %d byte characters %q", r.maxFileLength[characterLength-1], characterLength, character)
}

func (r *results) checkStreaming() {
	putter := r.f.Put
	if r.f.Features().PutStream != nil {
		fs.Infof(r.f, "Given remote has specialized streaming function. Using that to test streaming.")
		putter = r.f.Features().PutStream
	}

	contents := "thinking of test strings is hard"
	buf := bytes.NewBufferString(contents)
	hashIn := hash.NewMultiHasher()
	in := io.TeeReader(buf, hashIn)

	objIn := object.NewStaticObjectInfo("checkStreamingTest", time.Now(), -1, true, nil, r.f)
	objR, err := putter(r.ctx, in, objIn)
	if err != nil {
		fs.Infof(r.f, "Streamed file failed to upload (%v)", err)
		r.canStream = false
		return
	}

	hashes := hashIn.Sums()
	types := objR.Fs().Hashes().Array()
	for _, Hash := range types {
		sum, err := objR.Hash(r.ctx, Hash)
		if err != nil {
			fs.Infof(r.f, "Streamed file failed when getting hash %v (%v)", Hash, err)
			r.canStream = false
			return
		}
		if !hash.Equals(hashes[Hash], sum) {
			fs.Infof(r.f, "Streamed file has incorrect hash %v: expecting %q got %q", Hash, hashes[Hash], sum)
			r.canStream = false
			return
		}
	}
	if int64(len(contents)) != objR.Size() {
		fs.Infof(r.f, "Streamed file has incorrect file size: expecting %d got %d", len(contents), objR.Size())
		r.canStream = false
		return
	}
	r.canStream = true
}

func readInfo(ctx context.Context, f fs.Fs) error {
	// Ensure cleanup unless --keep-test-files is specified
	if !keepTestFiles {
		defer func() {
			err := operations.Purge(ctx, f, "")
			if err != nil {
				fs.Errorf(f, "Failed to purge temporary directory: %v", err)
			} else {
				fs.Infof(f, "Removed temporary directory for test files: %s", f.Root())
			}
		}()
	}

	r := newResults(ctx, f)
	if checkControl {
		r.checkControls()
	}
	if checkLength {
		for i := range r.maxFileLength {
			r.findMaxLength(i + 1)
		}
	}
	if checkNormalization {
		r.checkUTF8Normalization()
	}
	if checkStreaming {
		r.checkStreaming()
	}
	if checkBase32768 {
		r.checkBase32768()
	}
	r.Print()
	r.WriteJSON()
	return nil
}
