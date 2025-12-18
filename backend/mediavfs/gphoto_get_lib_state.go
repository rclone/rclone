package mediavfs

// buildGetLibraryStateMessage creates the message for get_library_state
// This is a simplified version using Google's official protobuf library
// We only need to send the essential fields, not the entire template
func buildGetLibraryStateMessage(stateToken, pageToken string) map[string]interface{} {
	// Based on Python's api.py but simplified to only required fields
	field1 := map[string]interface{}{
		"7": 2, // Required: mode field
	}

	// Add state_token if provided
	if stateToken != "" {
		field1["6"] = stateToken
	}

	// Add page_token if provided
	if pageToken != "" {
		field1["4"] = pageToken
	}

	// Add minimal required nested structures
	// These templates tell the API what fields we want back
	field1["1"] = map[string]interface{}{
		"1": map[string]interface{}{ // Media item template
			"1": map[string]interface{}{}, // Basic metadata
		},
	}

	return map[string]interface{}{
		"1": field1,
		"2": map[string]interface{}{ // Client info
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
