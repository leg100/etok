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

// Delete string from slice of strings and return new slice
func DeleteString(slice []string, str string) []string {
	for i, item := range slice {
		if item == str {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	// Return slice unchanged if string not found
	return slice
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

// Chunk slice of strings, returning a slice of slices of strings, each one a maximum of length
// `size`
func ChunkStrings(slice []string, size int) (chunked [][]string) {
	for i := 0; i < len(slice); i += size {
		end := i + size
		if end > len(slice) {
			end = len(slice)
		}
		chunked = append(chunked, slice[i:end])
	}
	return chunked
}

// Chunk slice of strings, returning a slice of slices of strings, each one of length `size`. If
// slice does not divide cleanly into size chunks, the last chunk is filled with elements of empty
// strings to make it so
func EqualChunkStrings(slice []string, size int) (chunked [][]string) {
	chunks := ChunkStrings(slice, size)
	if len(chunks) > 0 {
		// Keep appending empty strings until last chunk is of length size
		last := &chunks[len(chunks)-1]
		for i := 0; i < size; i++ {
			if len(*last) < size {
				*last = append(*last, "")
			}
		}
	}
	return chunks
}
