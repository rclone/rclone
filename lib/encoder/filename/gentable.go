//go:build ignore

package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"math"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/dop251/scsu"
	"github.com/klauspost/compress"
	"github.com/klauspost/compress/huff0"
)

// execute go run gentable.go
var indexFile = flag.String("index", "", "Index this file for table")

// Allow non-represented characters.
var addUnused = flag.Bool("all", true, "Make all bytes possible")
var scsuEncode = flag.Bool("scsu", false, "SCSU encode on each line before table")

func main() {
	flag.Parse()

	histogram := [256]uint64{
		// Replace/add histogram data and execute go run gentable.go
		// ncw home directory
		//0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19442, 760, 0, 349, 570, 1520, 199, 76, 685, 654, 0, 40377, 1605, 395132, 935270, 0, 1156377, 887730, 811737, 712241, 693240, 689139, 675964, 656417, 666577, 657413, 532, 24, 0, 145, 0, 3, 946, 44932, 37362, 46126, 36752, 76346, 19338, 47457, 14288, 38163, 4350, 7867, 36541, 65011, 30255, 26792, 22097, 1803, 39191, 61965, 76585, 11887, 12896, 5931, 1935, 1731, 1385, 1279, 9, 1278, 1, 420185, 0, 1146359, 746359, 968896, 868703, 1393640, 745019, 354147, 159462, 483979, 169092, 75937, 385858, 322166, 466635, 571268, 447132, 13792, 446484, 736844, 732675, 170232, 112983, 63184, 142357, 173945, 21521, 250, 0, 250, 4140, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 39, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3, 0, 4, 6, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 15, 0, 0, 0, 10, 0, 5, 0, 0, 0, 0, 0, 0, 283, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		//Images:
		//0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 765, 0, 0, 0, 0, 0, 8, 7, 3, 3, 0, 29, 53, 247265, 83587, 0, 265952, 233552, 229781, 71156, 78374, 65141, 46152, 43767, 55603, 39411, 0, 0, 0, 0, 0, 88, 84, 141, 70, 222, 191, 51, 52, 101, 60, 53, 23, 17, 49, 93, 53, 17, 92, 0, 158, 109, 41, 19, 43, 28, 10, 5, 1, 0, 0, 0, 0, 879, 0, 3415, 6770, 39823, 3566, 2491, 964, 42115, 825, 5178, 40755, 483, 1290, 3294, 1720, 6309, 42983, 10, 37739, 3454, 7028, 5077, 854, 227, 1259, 767, 218, 0, 0, 0, 163, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		// Google Drive:
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 459, 0, 0, 7, 0, 0, 0, 7, 1, 1, 0, 2, 1, 506, 706, 0, 3903, 3552, 3694, 3338, 3262, 3257, 3222, 3249, 3325, 3261, 5, 0, 0, 1, 0, 0, 0, 48, 31, 61, 53, 46, 17, 17, 34, 32, 9, 22, 17, 31, 27, 19, 52, 5, 46, 84, 38, 14, 5, 19, 2, 2, 0, 8, 0, 8, 0, 180, 0, 5847, 3282, 3729, 3695, 3842, 3356, 316, 139, 487, 117, 95, 476, 289, 428, 609, 467, 5, 446, 592, 955, 130, 112, 57, 390, 168, 14, 0, 2, 0, 44, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	}

	if *indexFile != "" {
		for i := range histogram[:] {
			histogram[i] = 0
		}
		b, err := os.ReadFile(*indexFile)
		if err != nil {
			panic(err)
		}
		if *scsuEncode {
			br := bufio.NewReader(bytes.NewBuffer(b))
			var encoded []byte
			for {
				line, err := br.ReadString('\n')
				if err != nil {
					break
				}
				line = strings.TrimSpace(line)
				if len(line) < 3 || !utf8.ValidString(line) {
					continue
				}
				e, err := scsu.Encode(line, nil)
				if err != nil {
					panic(err)
				}
				if len(e) >= len([]byte(line)) {
					continue
				}
				encoded = append(encoded, e...)
			}
			fmt.Println("scsu", len(b), "->", len(encoded), "(excluding bigger)")
			b = encoded
		}
		for _, v := range b {
			histogram[v]++
		}
	}

	// Sum up distributions
	var total uint64
	for _, v := range histogram[:] {
		total += v
	}

	// Scale the distribution to approx this size.
	const scale = 100 << 10
	var tmp []byte
	for i, v := range histogram[:] {
		if v == 0 && !*addUnused {
			continue
		}
		nf := float64(v) / float64(total) * scale
		if nf < 1 {
			nf = 1
		}
		t2 := make([]byte, int(math.Ceil(nf)))
		for j := range t2 {
			t2[j] = byte(i)
		}
		tmp = append(tmp, t2...)
	}

	var s huff0.Scratch
	s.Reuse = huff0.ReusePolicyNone
	_, _, err := huff0.Compress1X(tmp, &s)
	if err != nil {
		panic(err)
	}
	fmt.Println("table:", base64.URLEncoding.EncodeToString(s.OutTable))

	// Encode without ones:
	s.Reuse = huff0.ReusePolicyPrefer
	tmp = tmp[:0]
	for i, v := range histogram[:] {
		nf := float64(v) / float64(total) * scale
		t2 := make([]byte, int(math.Ceil(nf)))
		for j := range t2 {
			t2[j] = byte(i)
		}
		tmp = append(tmp, t2...)
	}
	_, _, err = huff0.Compress1X(tmp, &s)
	fmt.Println("sample", len(tmp), "byte, compressed size:", len(s.OutData))
	fmt.Println("Shannon limit:", compress.ShannonEntropyBits(tmp)/8, "bytes")
	if err != nil {
		panic(err)
	}

	fmt.Printf("avg size: 1 -> %.02f", float64(len(s.OutData))/float64(len(tmp)))
}
