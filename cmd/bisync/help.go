package bisync

import (
	"strconv"
	"strings"
)

func makeHelp(help string) string {
	replacer := strings.NewReplacer(
		"|", "`",
		"{MAXDELETE}", strconv.Itoa(DefaultMaxDelete),
		"{CHECKFILE}", DefaultCheckFilename,
		"{WORKDIR}", DefaultWorkdir,
	)
	return replacer.Replace(help)
}

var shortHelp = `Perform bidirectonal synchronization between two paths.`

var rcHelp = makeHelp(`
TODO
`)

var longHelp = shortHelp + makeHelp(`
TODO
`)
