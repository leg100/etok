package slice

// Return true if the given slices are identical in length and contents
func IdenticalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i, item := range a {
		if item != b[i] {
			return false
		}
	}

	return true
}

// Return true if the given slice contains the given string
func ContainsString(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}

	return false
}

// Return index of matching string in slice; otherwise return -1
func StringIndex(slice []string, str string) int {
	for idx := range slice {
		if slice[idx] == str {
			return idx
		}
	}
	return -1
}
