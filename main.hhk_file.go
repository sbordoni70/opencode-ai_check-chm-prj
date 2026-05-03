package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// parse_HHK_object_param_Local extracts every value="..." from <param name="Local" value="...">
// inside <OBJECT> blocks of the HHK file. Unlike the HHC parser, a single OBJECT may contain
// multiple <param name="local"> entries, all of which are collected. Trailing #fragment anchors are stripped.
func parse_HHK_object_param_Local(hhkPath string) ([]string, error) {
	data, err := os.ReadFile(hhkPath)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", hhkPath, err)
	}

	var refs []string
	content := string(data)

	for {
		objStart := no_case_SeekSubstring(content, "<OBJECT ")
		if objStart == -1 {
			break
		}

		objEnd := no_case_SeekSubstring(content[objStart:], "</OBJECT>")
		if objEnd == -1 {
			break
		}
		objEnd += objStart + len("</OBJECT>")
		objBlock := content[objStart:objEnd]
		content = content[objEnd:]

		offset := len("<OBJECT ")
		// Collect ALL name="local" params within this object block
		for {
			// seek the correct param tag
			index := no_case_SeekSubstring(objBlock[offset:], `<param name="local" value="`)
			if index == -1 {
				break
			}

			// update offset to the start of this param tag for the next iteration
			offset += index + len(`<param name="Local" value="`)
			// update offset to the start of the actual value after value=""
			index = no_case_SeekSubstring(objBlock[offset:], `">`)
			if index == -1 {
				continue
			}

			// extract the value and strip trailing fragment if any
			ref := objBlock[offset : offset+index]
			ref = strings.TrimSpace(ref)
			idx := no_case_SeekSubstring(ref, "#")
			if idx != -1 {
				ref = ref[:idx]
			}
			if ref != "" {
				refs = append(refs, ref)
			}

			// update offset to continue searching for the next name="local" param within this object block
			offset += index + len(`">`)
		}
	}

	return refs, nil
}

// Step03_ProcessFile_HHK checks every file referenced in the HHK index file.
// A reference that exists on disk but isn't in the HHP [FILES] list is marked unlisted.
// A reference that doesn't exist on disk is marked missing.
func Step03_ProcessFile_HHK(projectDir string, hhkPath string) error {
	fmt.Printf("Step 3 - importing HHK file and checking the listed files...\n")
	localRefs, err := parse_HHK_object_param_Local(hhkPath)
	if err != nil {
		return fmt.Errorf("cannot parse %s: %w", hhkPath, err)
	}

	items_missing := len(missing)
	items_unlisted := len(unlisted)

	hhkDir := filepath.Dir(hhkPath)

	for _, ref := range localRefs {
		// if it's in the present set,... no need to check anything
		if presentSet[strings.ToLower(ref)] {
			continue
		}

		// else we need to understand where to put it
		fullPath := ref
		if !filepath.IsAbs(ref) {
			fullPath = filepath.Join(hhkDir, ref)
		}

		_, statErr := os.Stat(fullPath)
		if statErr != nil {
			addIfNew(&missing, &missingSet, ref)
		} else {
			addIfNew(&unlisted, &unlistedSet, ref)
		}
	}

	items_processed := len(localRefs)

	fmt.Printf("    %d files listed (+%d missing, +%d unlisted)\n\n", items_processed,
		len(missing)-items_missing, len(unlisted)-items_unlisted)

	return nil
}
