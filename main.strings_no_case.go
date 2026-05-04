package main

// it compares two Unicode code points for case-insensitive equality.
func no_case_areCharsEqual(chr1, chr2 uint) bool {
	// If characters are identical, return true immediately
	if chr1 == chr2 {
		return true
	}
	// sort them
	if chr1 > chr2 {
		chr1, chr2 = chr2, chr1
	}
	// check if they are in the same case-insensitive equivalence class
	return (chr1 >= 'A') && (chr1 <= 'Z') && (chr2 == chr1+('a'-'A'))
}

// like strings.HasPrefix but with case-insensitive comparison and Unicode support.
func no_case_HasPrefix(str, prefix string) bool {
	return no_case_IsMatching(str, 0, prefix)
}

// like strings.HasSuffix but the comparison is done at the end of the string.
func no_case_HasSuffix(str, suffix string) bool {
	strLen := len(str)
	suffixLen := len(suffix)
	return (suffixLen > 0) && (strLen > suffixLen) &&
		no_case_IsMatching(str, strLen-suffixLen, suffix)
}

// like strings.EqualFold but with case-insensitive comparison and Unicode support.
func no_case_IsEqual(str1, str2 string) bool {
	str1Len := len(str1)
	str2Len := len(str2)
	return (str1Len == str2Len) && no_case_IsMatching(str1, 0, str2)
}

// like strings.HasPrefix but with case-insensitive comparison and Unicode support.
func no_case_IsMatching(str string, offset int, prefix string) bool {
	// check offset
	if offset < 0 {
		return false
	}
	// get strings lengths
	strLen := len(str)
	prefixLen := len(prefix)
	// check offset & prefix length
	if (strLen <= offset) || (prefixLen > (strLen - offset)) {
		return false
	}
	// compare chrs
	for i := 0; i < prefixLen; i++ {
		if !no_case_areCharsEqual(uint(str[offset+i]), uint(prefix[i])) {
			return false
		}
	}
	return true
}

// like strings.Index but with case-insensitive comparison and Unicode support.
func no_case_SeekSubstring(str, substr string) int {

	// get & check substring length
	substrLen := len(substr)
	if substrLen == 0 {
		return 0
	}
	// get string length
	strLen := len(str)
	// if substring is longer than string, it can't be found
	if (strLen == 0) || (substrLen > strLen) {
		return -1
	}
	// iterate over string and compare with substring
	strLen -= substrLen
	for i := 0; i <= strLen; i++ {
		match := true
		for j := 0; j < substrLen; j++ {
			if !no_case_areCharsEqual(uint(str[i+j]), uint(substr[j])) {
				match = false
				break
			}
		}
		// return index if a match is found
		if match {
			return i
		}
	}
	return -1
}
