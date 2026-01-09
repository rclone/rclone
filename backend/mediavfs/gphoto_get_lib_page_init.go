package mediavfs

// buildGetLibraryPageInitMessage creates the message for get_library_page_init
// This is used during INITIAL SYNC to fetch batches of media items
// The extensive template tells the API which fields to return
func buildGetLibraryPageInitMessage(pageToken string) map[string]interface{} {
	// Based on Python's api.py get_library_page_init method
	// This template structure is critical for getting batch results

	field1_1 := map[string]interface{}{
		"1":  map[string]interface{}{},
		"3":  map[string]interface{}{},
		"4":  map[string]interface{}{},
		"5":  map[string]interface{}{"1": map[string]interface{}{}, "2": map[string]interface{}{}, "3": map[string]interface{}{}, "4": map[string]interface{}{}, "5": map[string]interface{}{}, "7": map[string]interface{}{}},
		"6":  map[string]interface{}{},
		"7":  map[string]interface{}{"2": map[string]interface{}{}},
		"15": map[string]interface{}{},
		"16": map[string]interface{}{},
		"17": map[string]interface{}{},
		"19": map[string]interface{}{},
		"20": map[string]interface{}{},
		"21": map[string]interface{}{"5": map[string]interface{}{"3": map[string]interface{}{}}, "6": map[string]interface{}{}},
		"25": map[string]interface{}{},
		"30": map[string]interface{}{"2": map[string]interface{}{}},
		"31": map[string]interface{}{},
		"32": map[string]interface{}{},
		"33": map[string]interface{}{"1": map[string]interface{}{}},
		"34": map[string]interface{}{},
		"36": map[string]interface{}{},
		"37": map[string]interface{}{},
		"38": map[string]interface{}{},
		"39": map[string]interface{}{},
		"40": map[string]interface{}{},
		"41": map[string]interface{}{},
	}

	field1_5 := map[string]interface{}{
		"2": map[string]interface{}{
			"2": map[string]interface{}{"3": map[string]interface{}{"2": map[string]interface{}{}}, "4": map[string]interface{}{"2": map[string]interface{}{}}},
			"4": map[string]interface{}{"2": map[string]interface{}{"2": 1}},
			"5": map[string]interface{}{"2": map[string]interface{}{}},
			"6": 1,
		},
		"3": map[string]interface{}{
			"2": map[string]interface{}{"3": map[string]interface{}{}, "4": map[string]interface{}{}},
			"3": map[string]interface{}{"2": map[string]interface{}{}, "3": map[string]interface{}{"2": 1}},
			"4": map[string]interface{}{},
			"5": map[string]interface{}{"2": map[string]interface{}{"2": 1}},
			"7": map[string]interface{}{},
		},
		"4": map[string]interface{}{"2": map[string]interface{}{"2": map[string]interface{}{}}},
		"5": map[string]interface{}{
			"1": map[string]interface{}{
				"2": map[string]interface{}{"3": map[string]interface{}{}, "4": map[string]interface{}{}},
				"3": map[string]interface{}{"2": map[string]interface{}{}, "3": map[string]interface{}{"2": 1}},
			},
			"3": 1,
		},
	}

	field1_1_full := map[string]interface{}{
		"1":  field1_1,
		"5":  field1_5,
		"8":  map[string]interface{}{},
		"9":  map[string]interface{}{"2": map[string]interface{}{}, "3": map[string]interface{}{"1": map[string]interface{}{}, "2": map[string]interface{}{}}},
		"11": map[string]interface{}{"2": map[string]interface{}{}, "3": map[string]interface{}{}, "4": map[string]interface{}{"2": map[string]interface{}{"1": 1, "2": 2}}},
		"12": map[string]interface{}{},
		"14": map[string]interface{}{"2": map[string]interface{}{}, "3": map[string]interface{}{}, "4": map[string]interface{}{"2": map[string]interface{}{"1": 1, "2": 2}}},
		"15": map[string]interface{}{"1": map[string]interface{}{}, "4": map[string]interface{}{}},
		"17": map[string]interface{}{"1": map[string]interface{}{}, "4": map[string]interface{}{}},
		"19": map[string]interface{}{"2": map[string]interface{}{}, "3": map[string]interface{}{}, "4": map[string]interface{}{"2": map[string]interface{}{"1": 1, "2": 2}}},
		"22": map[string]interface{}{},
		"23": map[string]interface{}{},
	}

	// Build field["1"] structure
	field1 := map[string]interface{}{
		"1": field1_1_full,
		"2": map[string]interface{}{
			"1": map[string]interface{}{
				"2":  map[string]interface{}{},
				"3":  map[string]interface{}{},
				"4":  map[string]interface{}{},
				"5":  map[string]interface{}{},
				"6":  map[string]interface{}{"1": map[string]interface{}{}, "2": map[string]interface{}{}, "3": map[string]interface{}{}, "4": map[string]interface{}{}, "5": map[string]interface{}{}, "7": map[string]interface{}{}},
				"7":  map[string]interface{}{},
				"8":  map[string]interface{}{},
				"10": map[string]interface{}{},
				"12": map[string]interface{}{},
				"13": map[string]interface{}{"2": map[string]interface{}{}, "3": map[string]interface{}{}},
				"15": map[string]interface{}{"1": map[string]interface{}{}},
				"18": map[string]interface{}{},
			},
		},
		"3": map[string]interface{}{
			"2": map[string]interface{}{},
			"3": map[string]interface{}{
				"2": map[string]interface{}{},
				"3": map[string]interface{}{},
			},
		},
	}

	// CRITICAL FIX: Add page_token at the correct level ["1"]["4"], not ["1"]["1"]["4"]
	if pageToken != "" {
		field1["4"] = pageToken
	}

	return map[string]interface{}{
		"1": field1,
		"2": map[string]interface{}{
			"1": map[string]interface{}{
				"1": map[string]interface{}{
					"1": map[string]interface{}{
						"1": map[string]interface{}{},
					},
				},
			},
		},
	}
}
