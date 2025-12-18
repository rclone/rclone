package main

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	"github.com/rclone/rclone/backend/mediavfs"
)

func main() {
	ctx := context.Background()

	// Create API client
	api := mediavfs.NewGPhotoAPI("test@example.com", "https://m.alicuxi.net", http.DefaultClient)

	// Get auth token
	if err := api.GetAuthToken(ctx, false); err != nil {
		fmt.Printf("ERROR: Failed to get auth token: %v\n", err)
		return
	}

	fmt.Println("=== Calling GetLibraryState ===")
	response, err := api.GetLibraryState(ctx, "", "")
	if err != nil {
		fmt.Printf("ERROR: Failed to get library state: %v\n", err)
		return
	}

	fmt.Printf("Response size: %d bytes\n", len(response))

	// Decode to map
	data, err := mediavfs.DecodeToMap(response)
	if err != nil {
		fmt.Printf("ERROR: Failed to decode response: %v\n", err)
		return
	}

	// Inspect top-level structure
	fmt.Println("\n=== Top-level fields ===")
	printMapStructure(data, 0, 3)

	// Get field 1
	field1, ok := data["1"].(map[string]interface{})
	if !ok {
		fmt.Printf("ERROR: field 1 is not a map, type: %T\n", data["1"])
		return
	}

	fmt.Println("\n=== Field 1 structure ===")
	for _, key := range []string{"1", "2", "3", "4", "5", "6", "9", "12"} {
		if val, ok := field1[key]; ok {
			fmt.Printf("field1[\"%s\"] = %s", key, describeValue(val))
			// If it's an array, show a sample of the first item
			if arr, isArray := val.([]interface{}); isArray && len(arr) > 0 {
				fmt.Printf("  First item: %s", describeValue(arr[0]))
			}
			fmt.Println()
		}
	}
}

func describeValue(v interface{}) string {
	switch val := v.(type) {
	case []interface{}:
		return fmt.Sprintf("ARRAY[%d items]", len(val))
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		return fmt.Sprintf("MAP{%d keys: %v}", len(val), keys)
	case string:
		if len(val) > 50 {
			return fmt.Sprintf("string[%d chars]: %q...", len(val), val[:50])
		}
		return fmt.Sprintf("string: %q", val)
	case uint64:
		return fmt.Sprintf("uint64: %d", val)
	case int64:
		return fmt.Sprintf("int64: %d", val)
	default:
		return fmt.Sprintf("%T: %v", v, v)
	}
}

func printMapStructure(m map[string]interface{}, depth int, maxDepth int) {
	if depth >= maxDepth {
		return
	}

	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}

	for k, v := range m {
		fmt.Printf("%s%s: ", indent, k)
		switch val := v.(type) {
		case map[string]interface{}:
			fmt.Printf("MAP{%d keys}\n", len(val))
			printMapStructure(val, depth+1, maxDepth)
		case []interface{}:
			fmt.Printf("ARRAY[%d items]\n", len(val))
			if len(val) > 0 && depth < maxDepth-1 {
				fmt.Printf("%s  [0]: %s\n", indent, reflect.TypeOf(val[0]))
			}
		case string:
			if len(val) > 40 {
				fmt.Printf("string[%d]: %q...\n", len(val), val[:40])
			} else {
				fmt.Printf("string: %q\n", val)
			}
		default:
			fmt.Printf("%T: %v\n", v, v)
		}
	}
}
