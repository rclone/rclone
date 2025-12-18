package mediavfs

// buildGetLibraryStateMessage creates the message for get_library_state
// This uses the FULL template from Python to get complete data
func buildGetLibraryStateMessage(stateToken, pageToken string) map[string]interface{} {
	// Full template structure from Python's get_library_state
	// This tells the API which fields to return

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
			"2": map[string]interface{}{"3": map[string]interface{}{"2": map[string]interface{}{}}, "4": map[string]interface{}{"2": map[string]interface{}{}, "4": map[string]interface{}{}}},
			"4": map[string]interface{}{"2": map[string]interface{}{"2": 1}},
			"5": map[string]interface{}{"2": map[string]interface{}{}},
			"6": 1,
		},
		"3": map[string]interface{}{
			"2": map[string]interface{}{"3": map[string]interface{}{}, "4": map[string]interface{}{}},
			"3": map[string]interface{}{"2": map[string]interface{}{}, "3": map[string]interface{}{"2": 1, "3": map[string]interface{}{}}},
			"4": map[string]interface{}{},
			"5": map[string]interface{}{"2": map[string]interface{}{"2": 1}},
			"7": map[string]interface{}{},
		},
		"4": map[string]interface{}{"2": map[string]interface{}{"2": map[string]interface{}{}}},
		"5": map[string]interface{}{
			"1": map[string]interface{}{
				"2": map[string]interface{}{"3": map[string]interface{}{}, "4": map[string]interface{}{}},
				"3": map[string]interface{}{"2": map[string]interface{}{}, "3": map[string]interface{}{"2": 1, "3": map[string]interface{}{}}},
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
		"21": map[string]interface{}{"1": map[string]interface{}{}},
		"22": map[string]interface{}{},
		"23": map[string]interface{}{},
		"24": map[string]interface{}{},
	}

	// Add page_token if provided
	if pageToken != "" {
		field1_1_full["4"] = pageToken
	}

	field1_2 := map[string]interface{}{
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
		"4":  map[string]interface{}{"1": map[string]interface{}{}},
		"9":  map[string]interface{}{},
		"11": map[string]interface{}{"1": map[string]interface{}{"1": map[string]interface{}{}, "4": map[string]interface{}{}, "5": map[string]interface{}{}, "6": map[string]interface{}{}, "9": map[string]interface{}{}}},
		"14": map[string]interface{}{
			"1": map[string]interface{}{
				"1": map[string]interface{}{
					"1": map[string]interface{}{},
					"2": map[string]interface{}{"2": map[string]interface{}{"1": map[string]interface{}{"1": map[string]interface{}{}}, "3": map[string]interface{}{}}},
					"3": map[string]interface{}{
						"4": map[string]interface{}{"1": map[string]interface{}{"1": map[string]interface{}{}}, "3": map[string]interface{}{}},
						"5": map[string]interface{}{"1": map[string]interface{}{"1": map[string]interface{}{}}, "3": map[string]interface{}{}},
					},
				},
				"2": map[string]interface{}{},
			},
		},
		"17": map[string]interface{}{},
		"18": map[string]interface{}{"1": map[string]interface{}{}, "2": map[string]interface{}{"1": map[string]interface{}{}}},
		"20": map[string]interface{}{"2": map[string]interface{}{"1": map[string]interface{}{}, "2": map[string]interface{}{}}},
		"22": map[string]interface{}{},
		"23": map[string]interface{}{},
		"24": map[string]interface{}{},
	}

	field1_3 := map[string]interface{}{
		"2": map[string]interface{}{},
		"3": map[string]interface{}{
			"2":  map[string]interface{}{},
			"3":  map[string]interface{}{},
			"7":  map[string]interface{}{},
			"8":  map[string]interface{}{},
			"14": map[string]interface{}{"1": map[string]interface{}{}},
			"16": map[string]interface{}{},
			"17": map[string]interface{}{"2": map[string]interface{}{}},
			"18": map[string]interface{}{},
			"19": map[string]interface{}{},
			"20": map[string]interface{}{},
			"21": map[string]interface{}{},
			"22": map[string]interface{}{},
			"23": map[string]interface{}{},
			"27": map[string]interface{}{"1": map[string]interface{}{}, "2": map[string]interface{}{"1": map[string]interface{}{}}},
			"29": map[string]interface{}{},
			"30": map[string]interface{}{},
			"31": map[string]interface{}{},
			"32": map[string]interface{}{},
			"34": map[string]interface{}{},
			"37": map[string]interface{}{},
			"38": map[string]interface{}{},
			"39": map[string]interface{}{},
			"41": map[string]interface{}{},
			"43": map[string]interface{}{"1": map[string]interface{}{}},
			"45": map[string]interface{}{"1": map[string]interface{}{"1": map[string]interface{}{}}},
			"46": map[string]interface{}{"1": map[string]interface{}{}, "2": map[string]interface{}{}, "3": map[string]interface{}{}},
			"47": map[string]interface{}{},
		},
		"4": map[string]interface{}{"2": map[string]interface{}{}, "3": map[string]interface{}{"1": map[string]interface{}{}}, "4": map[string]interface{}{}, "5": map[string]interface{}{"1": map[string]interface{}{}}},
		"7": map[string]interface{}{},
		"12": map[string]interface{}{},
		"13": map[string]interface{}{},
		"14": map[string]interface{}{
			"1": map[string]interface{}{},
			"2": map[string]interface{}{"1": map[string]interface{}{}, "2": map[string]interface{}{"1": map[string]interface{}{}}, "3": map[string]interface{}{}, "4": map[string]interface{}{"1": map[string]interface{}{}}},
			"3": map[string]interface{}{"1": map[string]interface{}{}, "2": map[string]interface{}{"1": map[string]interface{}{}}, "3": map[string]interface{}{}, "4": map[string]interface{}{}},
		},
		"15": map[string]interface{}{},
		"16": map[string]interface{}{"1": map[string]interface{}{}},
		"18": map[string]interface{}{},
		"19": map[string]interface{}{
			"4": map[string]interface{}{"2": map[string]interface{}{}},
			"6": map[string]interface{}{"2": map[string]interface{}{}, "3": map[string]interface{}{}},
			"7": map[string]interface{}{"2": map[string]interface{}{}, "3": map[string]interface{}{}},
			"8": map[string]interface{}{},
			"9": map[string]interface{}{},
		},
		"20": map[string]interface{}{},
		"22": map[string]interface{}{},
		"24": map[string]interface{}{},
		"25": map[string]interface{}{},
		"26": map[string]interface{}{},
	}

	field1 := map[string]interface{}{
		"1": field1_1_full,
		"2": field1_2,
		"3": field1_3,
		"7": 2, // Mode field
	}

	// Add state_token if provided
	if stateToken != "" {
		field1["6"] = stateToken
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
