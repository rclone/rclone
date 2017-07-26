package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/a8m/tree"
	"os"
)

var (
	// List
	a          = flag.Bool("a", false, "")
	d          = flag.Bool("d", false, "")
	f          = flag.Bool("f", false, "")
	ignorecase = flag.Bool("ignore-case", false, "")
	noreport   = flag.Bool("noreport", false, "")
	l          = flag.Bool("l", false, "")
	L          = flag.Int("L", 3, "")
	P          = flag.String("P", "", "")
	I          = flag.String("I", "", "")
	o          = flag.String("o", "", "")
	// Files
	s      = flag.Bool("s", false, "")
	h      = flag.Bool("h", false, "")
	p      = flag.Bool("p", false, "")
	u      = flag.Bool("u", false, "")
	g      = flag.Bool("g", false, "")
	Q      = flag.Bool("Q", false, "")
	D      = flag.Bool("D", false, "")
	inodes = flag.Bool("inodes", false, "")
	device = flag.Bool("device", false, "")
	// Sort
	U         = flag.Bool("U", false, "")
	v         = flag.Bool("v", false, "")
	t         = flag.Bool("t", false, "")
	c         = flag.Bool("c", false, "")
	r         = flag.Bool("r", false, "")
	dirsfirst = flag.Bool("dirsfirst", false, "")
	sort      = flag.String("sort", "", "")
	// Graphics
	i = flag.Bool("i", false, "")
	C = flag.Bool("C", false, "")
)

var usage = `Usage: tree [options...] [paths...]

Options:
    ------- Listing options -------
    -a		    All files are listed.
    -d		    List directories only.
    -l		    Follow symbolic links like directories.
    -f		    Print the full path prefix for each file.
    -L		    Descend only level directories deep.
    -P		    List only those files that match the pattern given.
    -I		    Do not list files that match the given pattern.
    --ignore-case   Ignore case when pattern matching.
    --noreport	    Turn off file/directory count at end of tree listing.
    -o filename	    Output to file instead of stdout.
    -------- File options ---------
    -Q		    Quote filenames with double quotes.
    -p		    Print the protections for each file.
    -u		    Displays file owner or UID number.
    -g		    Displays file group owner or GID number.
    -s		    Print the size in bytes of each file.
    -h		    Print the size in a more human readable way.
    -D		    Print the date of last modification or (-c) status change.
    --inodes	    Print inode number of each file.
    --device	    Print device ID number to which each file belongs.
    ------- Sorting options -------
    -v		    Sort files alphanumerically by version.
    -t		    Sort files by last modification time.
    -c		    Sort files by last status change time.
    -U		    Leave files unsorted.
    -r		    Reverse the order of the sort.
    --dirsfirst	    List directories before files (-U disables).
    --sort X	    Select sort: name,version,size,mtime,ctime.
    ------- Graphics options ------
    -i		    Don't print indentation lines.
    -C		    Turn colorization on always.
`

type fs struct{}

func (f *fs) Stat(path string) (os.FileInfo, error) {
	return os.Lstat(path)
}
func (f *fs) ReadDir(path string) ([]string, error) {
	dir, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	names, err := dir.Readdirnames(-1)
	dir.Close()
	if err != nil {
		return nil, err
	}
	return names, nil
}

func main() {
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	var nd, nf int
	var dirs = []string{"."}
	flag.Parse()
	// Make it work with leading dirs
	if args := flag.Args(); len(args) > 0 {
		dirs = args
	}
	// Output file
	var outFile = os.Stdout
	var err error
	if *o != "" {
		outFile, err = os.Create(*o)
		if err != nil {
			errAndExit(err)
		}
	}
	defer outFile.Close()
	// Check sort-type
	if *sort != "" {
		switch *sort {
		case "version", "mtime", "ctime", "name", "size":
		default:
			msg := fmt.Sprintf("sort type '%s' not valid, should be one of: "+
				"name,version,size,mtime,ctime", *sort)
			errAndExit(errors.New(msg))
		}
	}
	// Set options
	opts := &tree.Options{
		// Required
		Fs:      new(fs),
		OutFile: outFile,
		// List
		All:        *a,
		DirsOnly:   *d,
		FullPath:   *f,
		DeepLevel:  *L,
		FollowLink: *l,
		Pattern:    *P,
		IPattern:   *I,
		IgnoreCase: *ignorecase,
		// Files
		ByteSize: *s,
		UnitSize: *h,
		FileMode: *p,
		ShowUid:  *u,
		ShowGid:  *g,
		LastMod:  *D,
		Quotes:   *Q,
		Inodes:   *inodes,
		Device:   *device,
		// Sort
		NoSort:    *U,
		ReverSort: *r,
		DirSort:   *dirsfirst,
		VerSort:   *v || *sort == "version",
		ModSort:   *t || *sort == "mtime",
		CTimeSort: *c || *sort == "ctime",
		NameSort:  *sort == "name",
		SizeSort:  *sort == "size",
		// Graphics
		NoIndent: *i,
		Colorize: *C,
	}
	for _, dir := range dirs {
		inf := tree.New(dir)
		if d, f := inf.Visit(opts); f != 0 {
			if d > 0 {
				d -= 1
			}
			nd, nf = nd+d, nf+f
		}
		inf.Print(opts)
	}
	// Print footer report
	if !*noreport {
		footer := fmt.Sprintf("\n%d directories", nd)
		if !opts.DirsOnly {
			footer += fmt.Sprintf(", %d files", nf)
		}
		fmt.Fprintln(outFile, footer)
	}
}

func usageAndExit(msg string) {
	if msg != "" {
		fmt.Fprintf(os.Stderr, msg)
		fmt.Fprintf(os.Stderr, "\n\n")
	}
	flag.Usage()
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(1)
}

func errAndExit(err error) {
	fmt.Fprintf(os.Stderr, "tree: \"%s\"\n", err)
	os.Exit(1)
}
