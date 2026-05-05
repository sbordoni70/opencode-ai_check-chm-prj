package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// It checks if a file has an .html or .htm extension (case-insensitive).
func isHref_HTML_page(href string) bool {
	return no_case_HasSuffix(href, ".html") || no_case_HasSuffix(href, ".htm")
}

// It determines whether an href value is a local file reference.
// It returns false for empty strings, external protocols, and fragment-only links.
func isHref_Local(href string) bool {
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
		aStart := no_case_SeekSubstring(content, 0, "<a ")
		if aStart == -1 {
			break
		}

		aEnd := no_case_SeekSubstring(content, aStart, ">")
		if aEnd == -1 {
			break
		}
		aTag := content[aStart : aStart+aEnd]
		content = content[aStart+aEnd:]

		hrefIdx := no_case_SeekSubstring(aTag, 0, `href="`)
		if hrefIdx == -1 {
			continue
		}

		valueStart := hrefIdx + len(`href="`)
		valueEnd := no_case_SeekSubstring(aTag, valueStart, `"`)
		if valueEnd == -1 {
			continue
		}

		href := strings.TrimSpace(aTag[valueStart : valueStart+valueEnd])

		// skip any non-local hrefs
		if !isHref_Local(href) {
			continue
		}

		// remove any trailing #fragment
		idx := no_case_SeekSubstring(href, 0, "#")
		if idx != -1 {
			href = href[:idx]
		}

		// add only if it's non-empty or meaningful
		if href != "" {
			hrefs = append(hrefs, href)
		}
	}

	return hrefs, nil
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
			process_project_item_ref(item_hr, item_dir, item)
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
			if !process_project_item_ref(item_hr, item_dir, item) {
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
