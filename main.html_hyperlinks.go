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

		idx := no_case_SeekSubstring(href, "#")
		if idx != -1 {
			href = href[:idx]
		}

		if href != "" {
			hrefs = append(hrefs, href)
		}
	}

	return hrefs, nil
}

// Step04_PresentList_CheckHyperlinks iterates over every HTML file in the present list,
// extracts local hyperlinks, and checks each target against the present/missing/unlisted lists.
// Targets not in any list are classified by checking disk existence.
func Step04_PresentList_CheckHyperlinks(projectDir string) error {
	fmt.Printf("Step 4 - checking hyperlinks in present HTML files...\n")

	itemsMissingBefore := len(missing_list)
	itemsUnlistedBefore := len(unlisted_list)
	totalHrefs := 0

	for _, item := range present_list {
		ext := filepath.Ext(item)
		if !no_case_IsEqual(ext, ".html") && !no_case_IsEqual(ext, ".htm") {
			continue
		}

		fullPath := item
		if !filepath.IsAbs(item) {
			fullPath = filepath.Join(projectDir, item)
		}

		hrefs, err := extractHrefsFromFile(fullPath)
		if err != nil {
			fmt.Printf("    warning: %v\n", err)
			continue
		}

		fileDir := filepath.Dir(fullPath)

		for _, href := range hrefs {
			if !isLocalHref(href) {
				continue
			}

			totalHrefs++

			targetPath := href
			if !filepath.IsAbs(href) {
				targetPath = filepath.Join(fileDir, href)
			}

			relPath, err := filepath.Rel(projectDir, targetPath)
			if err != nil {
				relPath = href
			}

			key := strings.ToLower(relPath)

			if presentSet[key] {
				continue
			}
			if missingSet[key] {
				continue
			}
			if unlistedSet[key] {
				continue
			}

			if _, err := os.Stat(targetPath); err == nil {
				list_addIfNew(&unlisted_list, &unlistedSet, relPath)
			} else {
				list_missing_addIfNew(relPath, item)
			}
		}
	}

	fmt.Printf("    %d hyperlinks checked (+%d missing, +%d unlisted)\n\n", totalHrefs,
		len(missing_list)-itemsMissingBefore, len(unlisted_list)-itemsUnlistedBefore)

	return nil
}

// Step05_UnlistedList_CheckHyperlinks iterates over every HTML file in the unlisted list,
// extracts local hyperlinks, and checks each target against the present/missing/unlisted lists.
// Targets not in any list are classified by checking disk existence.
func Step05_UnlistedList_CheckHyperlinks(projectDir string) error {
	fmt.Printf("Step 5 - checking hyperlinks in unlisted HTML files...\n")

	itemsMissingBefore := len(missing_list)
	itemsUnlistedBefore := len(unlisted_list)
	totalHrefs := 0

	for _, item := range unlisted_list {
		ext := filepath.Ext(item)
		if !no_case_IsEqual(ext, ".html") && !no_case_IsEqual(ext, ".htm") {
			continue
		}

		fullPath := item
		if !filepath.IsAbs(item) {
			fullPath = filepath.Join(projectDir, item)
		}

		hrefs, err := extractHrefsFromFile(fullPath)
		if err != nil {
			fmt.Printf("    warning: %v\n", err)
			continue
		}

		fileDir := filepath.Dir(fullPath)

		for _, href := range hrefs {
			if !isLocalHref(href) {
				continue
			}

			totalHrefs++

			targetPath := href
			if !filepath.IsAbs(href) {
				targetPath = filepath.Join(fileDir, href)
			}

			relPath, err := filepath.Rel(projectDir, targetPath)
			if err != nil {
				relPath = href
			}

			key := strings.ToLower(relPath)

			if presentSet[key] {
				continue
			}
			if missingSet[key] {
				continue
			}
			if unlistedSet[key] {
				continue
			}

			if _, err := os.Stat(targetPath); err == nil {
				list_addIfNew(&unlisted_list, &unlistedSet, relPath)
			} else {
				list_missing_addIfNew(relPath, item)
			}
		}
	}

	fmt.Printf("    %d hyperlinks checked (+%d missing, +%d unlisted)\n\n", totalHrefs,
		len(missing_list)-itemsMissingBefore, len(unlisted_list)-itemsUnlistedBefore)

	return nil
}
