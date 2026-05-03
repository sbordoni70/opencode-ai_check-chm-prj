package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// isLocalHref determines whether an href value is a local file reference.
// It returns false for empty strings, external protocols, and fragment-only links.
func isLocalHref(href string) bool {
	return !no_case_HasPrefix(href, "http://") &&
		!no_case_HasPrefix(href, "https://") &&
		!no_case_HasPrefix(href, "ftp://") &&
		!no_case_HasPrefix(href, "mailto:") &&
		!no_case_HasPrefix(href, "javascript:") &&
		!no_case_HasPrefix(href, "data:") &&
		!no_case_HasPrefix(href, "#")
}

// extractHrefsFromFile reads an HTML file and extracts all href values from <a> tags.
// It strips trailing #fragment anchors and returns only non-empty values.
func extractHrefsFromFile(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", filePath, err)
	}

	var hrefs []string
	content := string(data)

	for {
		aStart := no_case_SeekSubstring(content, "<a ")
		if aStart == -1 {
			aStart = no_case_SeekSubstring(content, "<A ")
		}
		if aStart == -1 {
			break
		}

		aEnd := no_case_SeekSubstring(content[aStart:], ">")
		if aEnd == -1 {
			break
		}
		aTag := content[aStart : aStart+aEnd]
		content = content[aStart+aEnd:]

		hrefIdx := no_case_SeekSubstring(aTag, `href="`)
		if hrefIdx == -1 {
			continue
		}

		valueStart := hrefIdx + len(`href="`)
		valueEnd := no_case_SeekSubstring(aTag[valueStart:], `"`)
		if valueEnd == -1 {
			continue
		}

		href := aTag[valueStart : valueStart+valueEnd]
		href = strings.TrimSpace(href)

		// skip any non-local hrefs
		if !isLocalHref(href) {
			continue
		}

		// remove any trailing #fragment
		idx := no_case_SeekSubstring(href, "#")
		if idx != -1 {
			href = href[:idx]
		}

		// add only if it's non-empty or meaningful
		if (href != "") && isLocalHref(href) {
			hrefs = append(hrefs, href)
		}
	}

	return hrefs, nil
}

// verify href
func verifyHref(href string, origin_dir string, origin_item string) bool {
	// if it's not local, skip
	if !isLocalHref(href) {
		return true
	}
	// look for 'invalid' hrefs and add it in the missing list
	bInvalidValue := false
	if (href == "..") || (href == ".") {
		bInvalidValue = true
	} else {
		// try to filter and weird/invalid item
		switch href[len(href)-1] {
		case '\\', '/', ':', '*', '?', '"', '<', '>', '|':
			bInvalidValue = true
		}
	}
	if bInvalidValue {
		// add it to the invalid
		list_invalid_addIfNew(href, origin_item)
		// set it true because the client doesn't need to update the unlisted reference count
		return true
	}

	// get the absolute hyperlink path...
	href_full := filepath.Join(origin_dir, href)
	// check if it really local link...
	if !no_case_HasPrefix(href_full, project_dir) {
		return true
	}
	// get the project relative href
	href_rel := href_full[project_dir_len2:]

	// is it already present in a list?
	key := strings.ToLower(href_rel)
	if presentSet[key] || missingSet[key] || unlistedSet[key] {
		return true
	}

	// check and add to the proper list
	if _, err := os.Stat(href_full); err == nil {
		list_addIfNew(&unlisted_list, &unlistedSet, href_rel)
	} else {
		list_missing_addIfNew(href_rel, origin_item)
	}
	return false
}

// Step04_PresentList_CheckHyperlinks iterates over every HTML file in the present list,
// extracts local hyperlinks, and checks each target against the present/missing/unlisted lists.
// Targets not in any list are classified by checking disk existence.
func Step04_PresentList_CheckHyperlinks() error {
	fmt.Printf("\nStep 4 - checking hyperlinks in present HTML files...\n")

	itemsMissingBefore := len(missing_list)
	itemsUnlistedBefore := len(unlisted_list)
	totalHrefs := 0

	for _, item := range present_list {
		// check file type by extension
		ext := filepath.Ext(item)
		if !no_case_IsEqual(ext, ".html") && !no_case_IsEqual(ext, ".htm") {
			continue
		}

		// get the full path of this HTML file
		item_full := item
		if !filepath.IsAbs(item) {
			item_full = filepath.Join(project_dir, item)
		}

		// extract the hyperlinks of this file
		hrefs, err := extractHrefsFromFile(item_full)
		if err != nil {
			fmt.Printf("    warning: %v\n", err)
			continue
		}

		// check each href and update lists accordingly
		item_dir := filepath.Dir(item_full)
		for _, item_hr := range hrefs {
			totalHrefs++
			verifyHref(item_hr, item_dir, item)
		}
	}

	fmt.Printf("    %d hyperlinks checked (+%d missing, +%d unlisted)\n", totalHrefs,
		len(missing_list)-itemsMissingBefore, len(unlisted_list)-itemsUnlistedBefore)

	return nil
}

// Step05_UnlistedList_CheckHyperlinks iterates over every HTML file in the unlisted list,
// extracts local hyperlinks, and checks each target against the present/missing/unlisted lists.
// Targets not in any list are classified by checking disk existence.
func Step05_UnlistedList_CheckHyperlinks() error {
	fmt.Printf("\nStep 5 - checking hyperlinks in unlisted HTML files...\n")

	itemsMissingBefore := len(missing_list)
	itemsUnlistedBefore := len(unlisted_list)
	totalHrefs := 0

	max_items := itemsUnlistedBefore
	for i := 0; i < max_items; i++ {
		item := unlisted_list[i]
		// check file type by extension
		ext := filepath.Ext(item)
		if !no_case_IsEqual(ext, ".html") && !no_case_IsEqual(ext, ".htm") {
			continue
		}

		// get the full path
		item_full := item
		if !filepath.IsAbs(item) {
			item_full = filepath.Join(project_dir, item)
		}

		// extract the hyperlinks of this file
		hrefs, err := extractHrefsFromFile(item_full)
		if err != nil {
			fmt.Printf("    warning: %v\n", err)
			continue
		}

		// check each hyperlink in the unlisted HTML file
		bUpdateMaxItems := false
		item_dir := filepath.Dir(item_full)
		for _, item_hr := range hrefs {
			totalHrefs++
			if !verifyHref(item_hr, item_dir, item) {
				bUpdateMaxItems = true
			}
		}
		// update maxitems if needed
		if bUpdateMaxItems {
			max_items = len(unlisted_list)
		}
	}

	fmt.Printf("    %d hyperlinks checked (+%d missing, +%d unlisted)\n\n", totalHrefs,
		len(missing_list)-itemsMissingBefore, len(unlisted_list)-itemsUnlistedBefore)

	return nil
}
