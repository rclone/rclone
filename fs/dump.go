package fs

// DumpFlags describes the Dump options in force
type DumpFlags = Bits[dumpChoices]

// DumpFlags definitions
const (
	DumpHeaders DumpFlags = 1 << iota
	DumpBodies
	DumpRequests
	DumpResponses
	DumpAuth
	DumpFilters
	DumpGoRoutines
	DumpOpenFiles
	DumpMapper
)

type dumpChoices struct{}

func (dumpChoices) Choices() []BitsChoicesInfo {
	return []BitsChoicesInfo{
		{uint64(DumpHeaders), "headers"},
		{uint64(DumpBodies), "bodies"},
		{uint64(DumpRequests), "requests"},
		{uint64(DumpResponses), "responses"},
		{uint64(DumpAuth), "auth"},
		{uint64(DumpFilters), "filters"},
		{uint64(DumpGoRoutines), "goroutines"},
		{uint64(DumpOpenFiles), "openfiles"},
		{uint64(DumpMapper), "mapper"},
	}
}

func (dumpChoices) Type() string {
	return "DumpFlags"
}

// DumpFlagsList is a list of dump flags used in the help
var DumpFlagsList = DumpHeaders.Help()
