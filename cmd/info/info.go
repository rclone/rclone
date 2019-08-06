package info

// FIXME once translations are implemented will need a no-escape
// option for Put so we can make these tests work agaig

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/random"
	"github.com/spf13/cobra"
)

type position int

const (
	positionMiddle position = 1 << iota
	positionLeft
	positionRight
	positionNone position = 0
	positionAll  position = positionRight<<1 - 1
)

var (
	checkNormalization bool
	checkControl       bool
	checkLength        bool
	checkStreaming     bool
	positionList       = []position{positionMiddle, positionLeft, positionRight}
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
	commandDefintion.Flags().BoolVarP(&checkNormalization, "check-normalization", "", true, "Check UTF-8 Normalization.")
	commandDefintion.Flags().BoolVarP(&checkControl, "check-control", "", true, "Check control characters.")
	commandDefintion.Flags().BoolVarP(&checkLength, "check-length", "", true, "Check max filename length.")
	commandDefintion.Flags().BoolVarP(&checkStreaming, "check-streaming", "", true, "Check uploads with indeterminate file size.")
}

var commandDefintion = &cobra.Command{
	Use:   "info [remote:path]+",
	Short: `Discovers file name or other limitations for paths.`,
	Long: `rclone info discovers what filenames and upload methods are possible
to write to the paths passed in and how long they can be.  It can take some
time.  It will write test files into the remote:path passed in.  It outputs
a bit of go code for each one.
`,
	Hidden: true,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1E6, command, args)
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
	stringNeedsEscaping  map[string]position
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
		stringNeedsEscaping: make(map[string]position),
	}
}

// Print the results to stdout
func (r *results) Print() {
	fmt.Printf("// %s\n", r.f.Name())
	if checkControl {
		escape := []string{}
		for c, needsEscape := range r.stringNeedsEscaping {
			if needsEscape != positionNone {
				escape = append(escape, fmt.Sprintf("0x%02X", c))
			}
		}
		sort.Strings(escape)
		fmt.Printf("stringNeedsEscaping = []byte{\n")
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

// writeFile writes a file with some random contents
func (r *results) writeFile(path string) (fs.Object, error) {
	contents := random.String(50)
	src := object.NewStaticObjectInfo(path, time.Now(), int64(len(contents)), true, nil, r.f)
	return r.f.Put(r.ctx, bytes.NewBufferString(contents), src)
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

func (r *results) checkStringPositions(s string) {
	fs.Infof(r.f, "Writing position file 0x%0X", s)
	positionError := positionNone

	for _, pos := range positionList {
		path := ""
		switch pos {
		case positionMiddle:
			path = fmt.Sprintf("position-middle-%0X-%s-", s, s)
		case positionLeft:
			path = fmt.Sprintf("%s-position-left-%0X", s, s)
		case positionRight:
			path = fmt.Sprintf("position-right-%0X-%s", s, s)
		default:
			panic("invalid position: " + pos.String())
		}
		_, writeErr := r.writeFile(path)
		if writeErr != nil {
			fs.Infof(r.f, "Writing %s position file 0x%0X Error: %s", pos.String(), s, writeErr)
		} else {
			fs.Infof(r.f, "Writing %s position file 0x%0X OK", pos.String(), s)
		}
		obj, getErr := r.f.NewObject(r.ctx, path)
		if getErr != nil {
			fs.Infof(r.f, "Getting %s position file 0x%0X Error: %s", pos.String(), s, getErr)
		} else {
			if obj.Size() != 50 {
				fs.Infof(r.f, "Getting %s position file 0x%0X Invalid Size: %d", pos.String(), s, obj.Size())
			} else {
				fs.Infof(r.f, "Getting %s position file 0x%0X OK", pos.String(), s)
			}
		}
		if writeErr != nil || getErr != nil {
			positionError += pos
		}
	}

	r.mu.Lock()
	r.stringNeedsEscaping[s] = positionError
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
			r.stringNeedsEscaping[s] = positionAll
			continue
		}
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			token := <-tokens
			r.checkStringPositions(s)
			tokens <- token
		}(s)
	}
	for _, s := range []string{"＼", "\xBF", "\xFE"} {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			token := <-tokens
			r.checkStringPositions(s)
			tokens <- token
		}(s)
	}
	wg.Wait()
	fs.Infof(r.f, "Done trying to create control character file names")
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
	return nil
}

func (e position) String() string {
	switch e {
	case positionNone:
		return "none"
	case positionAll:
		return "all"
	}
	var buf bytes.Buffer
	if e&positionMiddle != 0 {
		buf.WriteString("middle")
		e &= ^positionMiddle
	}
	if e&positionLeft != 0 {
		if buf.Len() != 0 {
			buf.WriteRune(',')
		}
		buf.WriteString("left")
		e &= ^positionLeft
	}
	if e&positionRight != 0 {
		if buf.Len() != 0 {
			buf.WriteRune(',')
		}
		buf.WriteString("right")
		e &= ^positionRight
	}
	if e != positionNone {
		panic("invalid position")
	}
	return buf.String()
}
