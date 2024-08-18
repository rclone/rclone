// Package main provides utilities for the info test command.
package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"

	"github.com/rclone/rclone/cmd/test/info/internal"
	"github.com/rclone/rclone/fs"
)

func main() {
	fOut := flag.String("o", "out.csv", "Output file")
	flag.Parse()

	args := flag.Args()
	remotes := make([]internal.InfoReport, 0, len(args))
	for _, fn := range args {
		f, err := os.Open(fn)
		if err != nil {
			fs.Fatalf(nil, "Unable to open %q: %s", fn, err)
		}
		var remote internal.InfoReport
		dec := json.NewDecoder(f)
		err = dec.Decode(&remote)
		if err != nil {
			fs.Fatalf(nil, "Unable to decode %q: %s", fn, err)
		}
		if remote.ControlCharacters == nil {
			fs.Logf(nil, "Skipping remote %s: no ControlCharacters", remote.Remote)
		} else {
			remotes = append(remotes, remote)
		}
		if err := f.Close(); err != nil {
			fs.Fatalf(nil, "Closing %q failed: %s", fn, err)
		}
	}

	charsMap := make(map[string]string)
	var remoteNames []string
	for _, r := range remotes {
		remoteNames = append(remoteNames, r.Remote)
		for k, v := range *r.ControlCharacters {
			v.Text = k
			quoted := strconv.Quote(k)
			charsMap[k] = quoted[1 : len(quoted)-1]
		}
	}
	sort.Strings(remoteNames)

	chars := make([]string, 0, len(charsMap))
	for k := range charsMap {
		chars = append(chars, k)
	}
	sort.Strings(chars)

	//                     char       remote output
	recordsMap := make(map[string]map[string][]string)
	//                     remote output
	hRemoteMap := make(map[string][]string)
	hOperation := []string{"Write", "Write", "Write", "Get", "Get", "Get", "List", "List", "List"}
	hPosition := []string{"L", "M", "R", "L", "M", "R", "L", "M", "R"}

	// remote
	// write							get 								list
	// left	middle	right	left	middle	right	left	middle	right

	for _, r := range remotes {
		hRemoteMap[r.Remote] = []string{r.Remote, "", "", "", "", "", "", "", ""}
		for k, v := range *r.ControlCharacters {
			cMap, ok := recordsMap[k]
			if !ok {
				cMap = make(map[string][]string, 1)
				recordsMap[k] = cMap
			}

			cMap[r.Remote] = []string{
				sok(v.WriteError[internal.PositionLeft]), sok(v.WriteError[internal.PositionMiddle]), sok(v.WriteError[internal.PositionRight]),
				sok(v.GetError[internal.PositionLeft]), sok(v.GetError[internal.PositionMiddle]), sok(v.GetError[internal.PositionRight]),
				pok(v.InList[internal.PositionLeft]), pok(v.InList[internal.PositionMiddle]), pok(v.InList[internal.PositionRight]),
			}
		}
	}

	records := [][]string{
		{"", ""},
		{"", ""},
		{"Bytes", "Char"},
	}
	for _, r := range remoteNames {
		records[0] = append(records[0], hRemoteMap[r]...)
		records[1] = append(records[1], hOperation...)
		records[2] = append(records[2], hPosition...)
	}
	for _, c := range chars {
		k := charsMap[c]
		row := []string{fmt.Sprintf("%X", c), k}
		for _, r := range remoteNames {
			if m, ok := recordsMap[c][r]; ok {
				row = append(row, m...)
			} else {
				row = append(row, "", "", "", "", "", "", "", "", "")
			}
		}
		records = append(records, row)
	}

	var writer io.Writer
	if *fOut == "-" {
		writer = os.Stdout
	} else {
		f, err := os.Create(*fOut)
		if err != nil {
			fs.Fatalf(nil, "Unable to create %q: %s", *fOut, err)
		}
		defer func() {
			if err := f.Close(); err != nil {
				fs.Fatal(nil, fmt.Sprint("Error writing csv:", err))
			}
		}()
		writer = f
	}

	w := csv.NewWriter(writer)
	err := w.WriteAll(records)
	if err != nil {
		fs.Fatal(nil, fmt.Sprint("Error writing csv:", err))
	} else if err := w.Error(); err != nil {
		fs.Fatal(nil, fmt.Sprint("Error writing csv:", err))
	}
}

func sok(s string) string {
	if s != "" {
		return "ERR"
	}
	return "OK"
}

func pok(p internal.Presence) string {
	switch p {
	case internal.Absent:
		return "MIS"
	case internal.Present:
		return "OK"
	case internal.Renamed:
		return "REN"
	case internal.Multiple:
		return "MUL"
	default:
		return "ERR"
	}
}
