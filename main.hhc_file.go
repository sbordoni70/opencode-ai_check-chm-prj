package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// parse_HHC_object_param_Local extracts every value="..." from <param name="Local" value="...">
// inside <OBJECT> blocks of the HHC file, and strips any trailing #fragment anchor.
func parse_HHC_object_param_Local(hhcPath string) ([]string, error) {
	f, err := os.Open(hhcPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s: %w", hhcPath, err)
	}
	defer f.Close()

	data, err := os.ReadFile(hhcPath)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", hhcPath, err)
	}

	var refs []string
	content := string(data)

	// Iterate over every <OBJECT ... </OBJECT> block
	for {
		objStart := no_case_SeekSubstring(content, "<OBJECT")
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

		// Look for a <param tag inside this object block
		offset := no_case_SeekSubstring(objBlock, `<param name="Local" value="`)
		if offset == -1 {
			continue
		}

		// update offset to the start of this param tag for the next search
		offset += len(`<param name="Local" value="`)
		// get value end quote position
		index := no_case_SeekSubstring(objBlock[offset:], `">`)
		if index == -1 {
			continue
		}

		// extract the value and strip trailing fragment if any
		ref := objBlock[offset : offset+index]
		ref = strings.TrimSpace(ref)
		// Strip trailing fragment anchor (e.g. "page.html#section" -> "page.html")
		idx := no_case_SeekSubstring(ref, "#")
		if idx != -1 {
			ref = ref[:idx]
		}
		// if not null, append to the array of references to check
		if ref != "" {
			refs = append(refs, ref)
		}
	}

	return refs, nil
}

// Step02_ProcessFile_HHC checks every file referenced in the HHC table-of-contents.
// A reference that exists on disk but isn't in the HHP [FILES] list is marked unlisted.
// A reference that doesn't exist on disk is marked missing.
func Step02_ProcessFile_HHC(projectDir string, hhcPath string) error {
	fmt.Printf("Step 2 - importing HHC file and checking the listed files...\n")
	localRefs, err := parse_HHC_object_param_Local(hhcPath)
	if err != nil {
		return fmt.Errorf("cannot parse %s: %w", hhcPath, err)
	}

	// Snapshot current counts so we can report incremental deltas
	items_missing := len(missing_list)
	items_unlisted := len(unlisted_list)

	hhcDir := filepath.Dir(hhcPath)

	for _, ref := range localRefs {
		fullPath := ref
		if !filepath.IsAbs(ref) {
			fullPath = filepath.Join(hhcDir, ref)
		}

		// Compute path relative to the HHC directory for cross-comparison with HHP list
		relPath, err := filepath.Rel(hhcDir, fullPath)
		if err != nil {
			relPath = ref
		}

		_, statErr := os.Stat(fullPath)
		if statErr != nil {
			// File doesn't exist on disk
			list_missing_addIfNew(ref, ".HHC")
		} else if !presentSet[strings.ToLower(relPath)] {
			// File exists but wasn't listed in the HHP [FILES] section
			list_addIfNew(&unlisted_list, &unlistedSet, relPath)
		}
	}

	items_processed := len(localRefs)

	fmt.Printf("    %d files listed (+%d missing, +%d unlisted)\n\n", items_processed,
		len(missing_list)-items_missing, len(unlisted_list)-items_unlisted)

	return nil
}
