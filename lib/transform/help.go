package transform

import (
	"context"
	"fmt"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/encoder"
)

type commands struct {
	command     string
	description string
}

type example struct {
	path  string
	flags []string
}

var commandList = []commands{
	{command: "--name-transform prefix=XXXX", description: "Prepends XXXX to the file name."},
	{command: "--name-transform suffix=XXXX", description: "Appends XXXX to the file name after the extension."},
	{command: "--name-transform suffix_keep_extension=XXXX", description: "Appends XXXX to the file name while preserving the original file extension."},
	{command: "--name-transform trimprefix=XXXX", description: "Removes XXXX if it appears at the start of the file name."},
	{command: "--name-transform trimsuffix=XXXX", description: "Removes XXXX if it appears at the end of the file name."},
	{command: "--name-transform regex=/pattern/replacement/", description: "Applies a regex-based transformation."},
	{command: "--name-transform replace=old:new", description: "Replaces occurrences of old with new in the file name."},
	{command: "--name-transform date={YYYYMMDD}", description: "Appends or prefixes the specified date format."},
	{command: "--name-transform truncate=N", description: "Truncates the file name to a maximum of N characters."},
	{command: "--name-transform base64encode", description: "Encodes the file name in Base64."},
	{command: "--name-transform base64decode", description: "Decodes a Base64-encoded file name."},
	{command: "--name-transform encoder=ENCODING", description: "Converts the file name to the specified encoding (e.g., ISO-8859-1, Windows-1252, Macintosh)."},
	{command: "--name-transform decoder=ENCODING", description: "Decodes the file name from the specified encoding."},
	{command: "--name-transform charmap=MAP", description: "Applies a character mapping transformation."},
	{command: "--name-transform lowercase", description: "Converts the file name to lowercase."},
	{command: "--name-transform uppercase", description: "Converts the file name to UPPERCASE."},
	{command: "--name-transform titlecase", description: "Converts the file name to Title Case."},
	{command: "--name-transform ascii", description: "Strips non-ASCII characters."},
	{command: "--name-transform url", description: "URL-encodes the file name."},
	{command: "--name-transform nfc", description: "Converts the file name to NFC Unicode normalization form."},
	{command: "--name-transform nfd", description: "Converts the file name to NFD Unicode normalization form."},
	{command: "--name-transform nfkc", description: "Converts the file name to NFKC Unicode normalization form."},
	{command: "--name-transform nfkd", description: "Converts the file name to NFKD Unicode normalization form."},
	{command: "--name-transform command=/path/to/my/programfile names.", description: "Executes an external program to transform"},
}

var examples = []example{
	{"stories/The Quick Brown Fox!.txt", []string{"all,uppercase"}},
	{"stories/The Quick Brown Fox!.txt", []string{"all,replace=Fox:Turtle", "all,replace=Quick:Slow"}},
	{"stories/The Quick Brown Fox!.txt", []string{"all,base64encode"}},
	{"c3Rvcmllcw==/VGhlIFF1aWNrIEJyb3duIEZveCEudHh0", []string{"all,base64decode"}},
	{"stories/The Quick Brown 🦊 Fox Went to the Café!.txt", []string{"all,nfc"}},
	{"stories/The Quick Brown 🦊 Fox Went to the Café!.txt", []string{"all,nfd"}},
	{"stories/The Quick Brown 🦊 Fox!.txt", []string{"all,ascii"}},
	{"stories/The Quick Brown Fox!.txt", []string{"all,trimsuffix=.txt"}},
	{"stories/The Quick Brown Fox!.txt", []string{"all,prefix=OLD_"}},
	{"stories/The Quick Brown 🦊 Fox Went to the Café!.txt", []string{"all,charmap=ISO-8859-7"}},
	{"stories/The Quick Brown Fox: A Memoir [draft].txt", []string{"all,encoder=Colon,SquareBracket"}},
	{"stories/The Quick Brown 🦊 Fox Went to the Café!.txt", []string{"all,truncate=21"}},
	{"stories/The Quick Brown Fox!.txt", []string{"all,command=echo"}},
	{"stories/The Quick Brown Fox!", []string{"date=-{YYYYMMDD}"}},
	{"stories/The Quick Brown Fox!", []string{"date=-{macfriendlytime}"}},
	{"stories/The Quick Brown Fox!.txt", []string{"all,regex=[\\.\\w]/ab"}},
}

func (e example) command() string {
	s := fmt.Sprintf(`rclone convmv %q`, e.path)
	for _, f := range e.flags {
		s += fmt.Sprintf(" --name-transform %q", f)
	}
	return s
}

func (e example) output() string {
	ctx := context.Background()
	err := SetOptions(ctx, e.flags...)
	if err != nil {
		fs.Errorf(nil, "error generating help text: %v", err)
	}
	return Path(ctx, e.path, false)
}

// go run ./ convmv --help
func sprintExamples() string {
	s := "Examples: \n\n"
	for _, e := range examples {
		s += fmt.Sprintf("```\n%s\n", e.command())
		s += fmt.Sprintf("// Output: %s\n```\n\n", e.output())
	}
	return s
}

func commandTable() string {
	s := `| Command | Description |
|------|------|`
	for _, c := range commandList {
		s += fmt.Sprintf("\n| `%s` | %s |", c.command, c.description)
	}
	s += "\n\n\n"
	return s
}

var generatingHelpText bool

// SprintList returns the example help text as a string
func SprintList() string {
	var algos transformAlgo
	var charmaps fs.Enum[cmapChoices]
	generatingHelpText = true
	s := commandTable()
	s += fmt.Sprintln("Conversion modes:  \n```")
	for _, v := range algos.Choices() {
		s += fmt.Sprintln(v + "  ")
	}
	s += fmt.Sprintln("```")

	s += fmt.Sprintln("Char maps:  \n```")
	for _, v := range charmaps.Choices() {
		s += fmt.Sprintln(v + "  ")
	}
	s += fmt.Sprintln("```")

	s += fmt.Sprintln("Encoding masks:  \n```")
	for _, v := range strings.Split(encoder.ValidStrings(), ",") {
		s += fmt.Sprintln(v + "  ")
	}
	s += fmt.Sprintln("```")

	s += sprintExamples()

	generatingHelpText = false
	return s
}

// PrintList prints the example help text to stdout
func PrintList() {
	fmt.Println(SprintList())
}
