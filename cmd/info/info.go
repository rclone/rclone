package info

// FIXME once translations are implemented will need a no-escape
// option for Put so we can make these tests work agaig

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

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/info/internal"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/random"
	"github.com/spf13/cobra"
)

var (
	writeJSON          string
	checkNormalization bool
	checkControl       bool
	checkLength        bool
	checkStreaming     bool
	uploadWait         time.Duration
	positionLeftRe     = regexp.MustCompile(`(?s)^(.*)-position-left-([[:xdigit:]]+)$`)
	positionMiddleRe   = regexp.MustCompile(`(?s)^position-middle-([[:xdigit:]]+)-(.*)-$`)
	positionRightRe    = regexp.MustCompile(`(?s)^position-right-([[:xdigit:]]+)-(.*)$`)
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.StringVarP(cmdFlags, &writeJSON, "write-json", "", "", "Write results to file.")
	flags.BoolVarP(cmdFlags, &checkNormalization, "check-normalization", "", true, "Check UTF-8 Normalization.")
	flags.BoolVarP(cmdFlags, &checkControl, "check-control", "", true, "Check control characters.")
	flags.DurationVarP(cmdFlags, &uploadWait, "upload-wait", "", 0, "Wait after writing a file.")
	flags.BoolVarP(cmdFlags, &checkLength, "check-length", "", true, "Check max filename length.")
	flags.BoolVarP(cmdFlags, &checkStreaming, "check-streaming", "", true, "Check uploadxs with indeterminate file size.")
}

var commandDefinition = &cobra.Command{
	Use:   "info [remote:path]+",
	Short: `Discovers file name or other limitations for paths.`,
	Long: `rclone info discovers what filenames and upload methods are possible
to write to the paths passed in and how long they can be.  It can take some
time.  It will write test files into the remote:path passed in.  It outputs
a bit of go code for each one.
`,
	Hidden: true,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1e6, command, args)
		for i := range args {
			f := cmd.NewFsDir(args[i : i+1])
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
	maxFileLength        int
	canWriteUnnormalized bool
	canReadUnnormalized  bool
	canReadRenormalized  bool
	canStream            bool
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
		fmt.Printf("maxFileLength = %d\n", r.maxFileLength)
	}
	if checkNormalization {
		fmt.Printf("canWriteUnnormalized = %v\n", r.canWriteUnnormalized)
		fmt.Printf("canReadUnnormalized   = %v\n", r.canReadUnnormalized)
		fmt.Printf("canReadRenormalized   = %v\n", r.canReadRenormalized)
	}
	if checkStreaming {
		fmt.Printf("canStream = %v\n", r.canStream)
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
		report.MaxFileLength = &r.maxFileLength
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
	// Concurrency control
	tokens := make(chan struct{}, fs.Config.Checkers)
	for i := 0; i < fs.Config.Checkers; i++ {
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
func (r *results) findMaxLength() {
	const maxLen = 16 * 1024
	name := make([]byte, maxLen)
	for i := range name {
		name[i] = 'a'
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
		_, err := r.writeFile(path)
		if err != nil {
			fs.Infof(r.f, "Couldn't write file with name length %d: %v", i, err)
			return true
		}
		fs.Infof(r.f, "Wrote file with name length %d", i)
		return false
	})
	r.maxFileLength = i - 1
	fs.Infof(r.f, "Max file length is %d", r.maxFileLength)
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
	err := f.Mkdir(ctx, "")
	if err != nil {
		return errors.Wrap(err, "couldn't mkdir")
	}
	r := newResults(ctx, f)
	if checkControl {
		r.checkControls()
	}
	if checkLength {
		r.findMaxLength()
	}
	if checkNormalization {
		r.checkUTF8Normalization()
	}
	if checkStreaming {
		r.checkStreaming()
	}
	r.Print()
	r.WriteJSON()
	return nil
}
