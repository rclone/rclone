package check

// StringSliceContains iterates over the slice to find the target.
func StringSliceContains(slice []string, target string) bool {
	for _, v := range slice {
		if v == target {
			return true
		}
	}
	return false
}

// IntSliceContains iterates over the slice to find the target.
func IntSliceContains(slice []int, target int) bool {
	for _, v := range slice {
		if v == target {
			return true
		}
	}
	return false
}

// Int32SliceContains iterates over the slice to find the target.
func Int32SliceContains(slice []int32, target int32) bool {
	for _, v := range slice {
		if v == target {
			return true
		}
	}
	return false
}

// Int64SliceContains iterates over the slice to find the target.
func Int64SliceContains(slice []int64, target int64) bool {
	for _, v := range slice {
		if v == target {
			return true
		}
	}
	return false
}
