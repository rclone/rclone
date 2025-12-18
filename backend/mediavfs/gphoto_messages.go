package mediavfs

// buildGetLibraryStateMessage builds the data structure for get_library_state
// This matches the Python implementation from api.py lines 550-832
func buildGetLibraryStateMessage(stateToken, pageToken string) map[string]interface{} {
	return map[string]interface{}{
		"1": buildGetLibraryStateField1(stateToken, pageToken),
		"2": buildGetLibraryStateField2(),
	}
}

// buildGetLibraryStateField1 builds field 1 of the message
func buildGetLibraryStateField1(stateToken, pageToken string) map[string]interface{} {
	field1 := map[string]interface{}{
		"1": map[string]interface{}{
			"1": map[string]interface{}{
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
			},
			"5": map[string]interface{}{
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
			},
			"8": map[string]interface{}{},
			"9": map[string]interface{}{
				"2": map[string]interface{}{},
				"3": map[string]interface{}{"1": map[string]interface{}{}, "2": map[string]interface{}{}},
				"4": map[string]interface{}{
					"1": map[string]interface{}{
						"3": map[string]interface{}{
							"1": map[string]interface{}{
								"1": map[string]interface{}{"5": map[string]interface{}{"1": map[string]interface{}{}}, "6": map[string]interface{}{}, "7": map[string]interface{}{}},
								"2": map[string]interface{}{},
								"3": map[string]interface{}{
									"1": map[string]interface{}{"5": map[string]interface{}{"1": map[string]interface{}{}}, "6": map[string]interface{}{}, "7": map[string]interface{}{}},
									"2": map[string]interface{}{},
								},
							},
						},
						"4": map[string]interface{}{"1": map[string]interface{}{"2": map[string]interface{}{}}},
					},
				},
			},
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
		},
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
			"4": map[string]interface{}{"1": map[string]interface{}{}},
			"9": map[string]interface{}{},
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
		},
		"3": map[string]interface{}{
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
			"7":  map[string]interface{}{},
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
		},
		"7": 2,
		"9": map[string]interface{}{
			"1":  map[string]interface{}{"2": map[string]interface{}{"1": map[string]interface{}{}, "2": map[string]interface{}{}}},
			"2":  map[string]interface{}{"3": map[string]interface{}{"2": 1}},
			"3":  map[string]interface{}{"2": map[string]interface{}{}},
			"4":  map[string]interface{}{},
			"7":  map[string]interface{}{"1": map[string]interface{}{}},
			"8":  map[string]interface{}{"1": 2, "2": "\x01\x02\x03\x05\x06\x07"},
			"9":  map[string]interface{}{},
			"11": map[string]interface{}{"1": map[string]interface{}{}},
		},
		"11": []interface{}{1, 2, 6},
		"12": map[string]interface{}{"2": map[string]interface{}{"1": map[string]interface{}{}, "2": map[string]interface{}{}}, "3": map[string]interface{}{"1": map[string]interface{}{}}, "4": map[string]interface{}{}},
		"13": map[string]interface{}{},
		"15": map[string]interface{}{"3": map[string]interface{}{"1": 1}},
		"18": map[string]interface{}{
			"169945741": map[string]interface{}{
				"1": map[string]interface{}{
					"1": map[string]interface{}{
						"4":  []interface{}{2, 1, 6, 8, 10, 15, 18, 13, 17, 19, 14, 20},
						"5":  6,
						"6":  2,
						"7":  1,
						"8":  2,
						"11": 3,
						"12": 1,
						"13": 3,
						"15": 1,
						"16": 1,
						"17": 1,
						"18": 2,
					},
				},
			},
		},
		"19": map[string]interface{}{
			"1": map[string]interface{}{"1": map[string]interface{}{}, "2": map[string]interface{}{}},
			"2": map[string]interface{}{"1": []interface{}{1, 2, 4, 6, 5, 7}},
			"3": map[string]interface{}{"1": map[string]interface{}{}, "2": map[string]interface{}{}},
			"5": map[string]interface{}{"1": map[string]interface{}{}, "2": map[string]interface{}{}},
			"6": map[string]interface{}{"1": map[string]interface{}{}},
			"7": map[string]interface{}{"1": map[string]interface{}{}, "2": map[string]interface{}{}},
			"8": map[string]interface{}{"1": map[string]interface{}{}},
		},
		"20": map[string]interface{}{
			"1": 1,
			"2": "",
			"3": map[string]interface{}{
				"1": "type.googleapis.com/photos.printing.client.PrintingPromotionSyncOptions",
				"2": map[string]interface{}{
					"1": map[string]interface{}{
						"4":  []interface{}{2, 1, 6, 8, 10, 15, 18, 13, 17, 19, 14, 20},
						"5":  6,
						"6":  2,
						"7":  1,
						"8":  2,
						"11": 3,
						"12": 1,
						"13": 3,
						"15": 1,
						"16": 1,
						"17": 1,
						"18": 2,
					},
				},
			},
		},
		"21": map[string]interface{}{
			"2": map[string]interface{}{"2": map[string]interface{}{"4": map[string]interface{}{}}, "4": map[string]interface{}{}, "5": map[string]interface{}{}},
			"3": map[string]interface{}{"2": map[string]interface{}{"1": 1}, "4": map[string]interface{}{"2": map[string]interface{}{}}},
			"5": map[string]interface{}{"1": map[string]interface{}{}},
			"6": map[string]interface{}{"1": map[string]interface{}{}, "2": map[string]interface{}{"1": map[string]interface{}{}}},
			"7": map[string]interface{}{
				"1": 2,
				"2": "\x01\x07\x08\t\n\r\x0e\x0f\x11\x13\x14\x16\x17-./01:\x06\x18267;>?@A89<GBED",
				"3": "\x01",
			},
			"8": map[string]interface{}{
				"3": map[string]interface{}{"1": map[string]interface{}{"1": map[string]interface{}{"2": map[string]interface{}{"1": 1}, "4": map[string]interface{}{"2": map[string]interface{}{}}}}, "3": map[string]interface{}{}},
				"4": map[string]interface{}{"1": map[string]interface{}{}},
				"5": map[string]interface{}{"1": map[string]interface{}{"2": map[string]interface{}{"1": 1}, "4": map[string]interface{}{"2": map[string]interface{}{}}}},
			},
			"9": map[string]interface{}{"1": map[string]interface{}{}},
			"10": map[string]interface{}{
				"1":  map[string]interface{}{"1": map[string]interface{}{}},
				"3":  map[string]interface{}{},
				"5":  map[string]interface{}{},
				"6":  map[string]interface{}{"1": map[string]interface{}{}},
				"7":  map[string]interface{}{},
				"9":  map[string]interface{}{},
				"10": map[string]interface{}{},
			},
			"11": map[string]interface{}{},
			"12": map[string]interface{}{},
			"13": map[string]interface{}{},
			"14": map[string]interface{}{},
			"16": map[string]interface{}{"1": map[string]interface{}{}},
		},
		"22": map[string]interface{}{"1": 1, "2": "107818234414673686888"},
		"25": map[string]interface{}{"1": map[string]interface{}{"1": map[string]interface{}{"1": map[string]interface{}{"1": map[string]interface{}{}}}}, "2": map[string]interface{}{}},
		"26": map[string]interface{}{},
	}

	// Add state_token if provided
	if stateToken != "" {
		field1["6"] = stateToken
	}

	// Add page_token if provided (for pagination)
	if pageToken != "" {
		field1["4"] = pageToken
	}

	return field1
}

// buildGetLibraryStateField2 builds field 2 of the message
func buildGetLibraryStateField2() map[string]interface{} {
	return map[string]interface{}{
		"1": map[string]interface{}{
			"1": map[string]interface{}{
				"1": map[string]interface{}{"1": map[string]interface{}{}},
				"2": map[string]interface{}{},
			},
		},
		"2": map[string]interface{}{},
	}
}
