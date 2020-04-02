package slice

// Return true if the given slice contains the given string
func ContainsString(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}

	return false
}
