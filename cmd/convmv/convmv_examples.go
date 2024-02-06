package convmv

import (
	"fmt"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/encoder"
)

type example struct {
	Opt  ConvOpt
	Path string
}

var examples = []example{
	{Path: `stories/The Quick Brown Fox!.txt`, Opt: ConvOpt{ConvertAlgo: ConvUppercase}},
	{Path: `stories/The Quick Brown Fox!.txt`, Opt: ConvOpt{ConvertAlgo: ConvFindReplace, FindReplace: []string{"Fox,Turtle", "Quick,Slow"}}},
	{Path: `stories/The Quick Brown Fox!.txt`, Opt: ConvOpt{ConvertAlgo: ConvBase64Encode}},
	{Path: `c3Rvcmllcw==/VGhlIFF1aWNrIEJyb3duIEZveCEudHh0`, Opt: ConvOpt{ConvertAlgo: ConvBase64Decode}},
	{Path: `stories/The Quick Brown 🦊 Fox Went to the Café!.txt`, Opt: ConvOpt{ConvertAlgo: ConvToNFC}},
	{Path: `stories/The Quick Brown 🦊 Fox Went to the Café!.txt`, Opt: ConvOpt{ConvertAlgo: ConvToNFD}},
	{Path: `stories/The Quick Brown 🦊 Fox!.txt`, Opt: ConvOpt{ConvertAlgo: ConvASCII}},
	{Path: `stories/The Quick Brown Fox!.txt`, Opt: ConvOpt{ConvertAlgo: ConvTrimSuffix, Suffix: ".txt"}},
	{Path: `stories/The Quick Brown Fox!.txt`, Opt: ConvOpt{ConvertAlgo: ConvPrefix, Prefix: "OLD_"}},
	{Path: `stories/The Quick Brown 🦊 Fox Went to the Café!.txt`, Opt: ConvOpt{ConvertAlgo: ConvCharmap, CmapFlag: 20}},
	{Path: `stories/The Quick Brown Fox: A Memoir [draft].txt`, Opt: ConvOpt{ConvertAlgo: ConvEncoder, Enc: encoder.EncodeColon | encoder.EncodeSquareBracket}},
	{Path: `stories/The Quick Brown 🦊 Fox Went to the Café!.txt`, Opt: ConvOpt{ConvertAlgo: ConvTruncate, Max: 21}},
}

func (e example) command() string {
	s := fmt.Sprintf(`rclone convmv %q -t %s`, e.Path, e.Opt.ConvertAlgo)
	switch e.Opt.ConvertAlgo {
	case ConvFindReplace:
		for _, r := range e.Opt.FindReplace {
			s += fmt.Sprintf(` -r %q`, r)
		}
	case ConvTrimPrefix, ConvPrefix:
		s += fmt.Sprintf(` --prefix %q`, e.Opt.Prefix)
	case ConvTrimSuffix, ConvSuffix:
		s += fmt.Sprintf(` --suffix %q`, e.Opt.Suffix)
	case ConvCharmap:
		s += fmt.Sprintf(` --charmap %q`, e.Opt.CmapFlag.String())
	case ConvEncoder:
		s += fmt.Sprintf(` --encoding %q`, e.Opt.Enc.String())
	case ConvTruncate:
		s += fmt.Sprintf(` --max %d`, e.Opt.Max)
	}
	return s
}

func (e example) output() string {
	_ = e.Opt.validate()
	Opt = e.Opt
	s, err := ConvertPath(e.Path, e.Opt.ConvertAlgo, false)
	if err != nil {
		fs.Errorf(s, "error: %v", err)
	}
	return s
}

// go run ./ convmv --help
func sprintExamples() string {
	s := "Examples: \n\n"
	for _, e := range examples {
		s += fmt.Sprintf("```\n%s\n", e.command())
		s += fmt.Sprintf("// Output: %s\n```\n\n", e.output())
	}
	Opt = ConvOpt{} // reset
	return s
}

/* func sprintAllCharmapExamples() string {
	s := ""
	e := example{Path: `stories/The Quick Brown 🦊 Fox Went to the Café!.txt`, Opt: ConvOpt{ConvertAlgo: ConvCharmap, CmapFlag: 0}}
	for i := range Cmaps {
		e.Opt.CmapFlag++
		_ = e.Opt.validate()
		Opt = e.Opt
		s += fmt.Sprintf("%d Command: %s \n", i, e.command())
		s += fmt.Sprintf("Result: %s \n\n", e.output())
	}
	return s
} */
