package filename

import (
	"encoding/base64"
	"sync"

	"github.com/klauspost/compress/huff0"
)

// encodeURL is base64 url encoding values.
const encodeURL = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

// decodeMap will return x = decodeMap[encodeURL[byte(x)]] - 1 if x >= 0 and x < 64, otherwise -1 is returned.
var decodeMap [256]byte

// maxLength is the maximum length that will be attempted to be compressed.
const maxLength = 256

var (
	initOnce sync.Once // Used to control init of tables.

	encTables     [64]*huff0.Scratch // Encoders.
	encTableLocks [64]sync.Mutex     // Temporary locks for encoders since they are stateful.
	decTables     [64]*huff0.Decoder // Stateless decoders.
)

const (
	tableUncompressed = 0
	tableRLE          = 61
	tableCustom       = 62
	tableReserved     = 63
)

// predefined tables as base64 URL encoded string.
var tablesData = [64]string{
	// Uncompressed
	tableUncompressed: "",
	// ncw home directory
	1: "MRDIEtAAMAzDMAzDSjX_ybu0w97bb-L3b2mR-rUl5LXW3lZII43kIDMzM1NXu3okgQs=",
	// ncw images
	2: "IhDIAEAA______-Pou_4Sf5z-uS-39MVWjullFLKM7EBECs=",
	// ncw Google Drive:
	3: "JxDQAIIBMDMzMwOzbv7nJJCyd_m_9D2llCarnQX33nvvlFKEhUxAAQ==",
	// Hex
	4: "ExDoSTD___-tfXfhJ0hKSkryTxU=",
	// Base64
	5: "JRDIcQf_______8PgIiIiIgINkggARHlkQwSSCCBxHFYINHdfXI=",

	// Special tables:
	// Compressed data has its own table.
	tableCustom: "",
	// Reserved for extension.
	tableReserved: "",
}

func initCoders() {
	initOnce.Do(func() {
		// Init base 64 decoder.
		for i, v := range encodeURL {
			decodeMap[v] = byte(i) + 1
		}

		// Initialize encoders and decoders.
		for i, dataString := range tablesData {
			if len(dataString) == 0 {
				continue
			}
			data, err := base64.URLEncoding.DecodeString(dataString)
			if err != nil {
				panic(err)
			}
			s, _, err := huff0.ReadTable(data, nil)
			if err != nil {
				panic(err)
			}

			// We want to save at least len(in) >> 5
			s.WantLogLess = 5
			s.Reuse = huff0.ReusePolicyMust
			encTables[i] = s
			decTables[i] = s.Decoder()
		}
		// Add custom table type.
		var s huff0.Scratch
		s.Reuse = huff0.ReusePolicyNone
		encTables[tableCustom] = &s
		decTables[tableCustom] = nil
	})
}
