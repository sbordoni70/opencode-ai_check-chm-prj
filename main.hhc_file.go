package main

import (
	"fmt"
	"os"
	"strings"
)

// parse_HHC_object_param_Local extracts value="..." from
// <param name="Local" value="..."> inside <OBJECT> blocks of the HHC file,
// and strips any trailing #fragment anchor.
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
		objStart := no_case_SeekSubstring(content, 0, "<OBJECT")
		if objStart == -1 {
			break
		}

		objEnd := no_case_SeekSubstring(content, objStart, "</OBJECT>")
		if objEnd == -1 {
			break
		}
		objEnd += objStart + len("</OBJECT>")
		objBlock := content[objStart:objEnd]
		content = content[objEnd:]

		// Look for a <param tag inside this object block
		offset := no_case_SeekSubstring(objBlock, 0, `<param name="Local" value="`)
		if offset == -1 {
			continue
		}

		// update offset to the start of this param tag for the next search
		offset += len(`<param name="Local" value="`)
		// get value end quote position
		index := no_case_SeekSubstring(objBlock, offset, `">`)
		if index == -1 {
			continue
		}

		// extract the value and strip trailing fragment if any
		ref := strings.TrimSpace(objBlock[offset : offset+index])
		// Strip trailing fragment anchor (e.g. "page.html#section" -> "page.html")
		idx := no_case_SeekSubstring(ref, 0, "#")
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

// It checks every file referenced in the HHC table-of-contents.
// A reference that exists on disk but isn't in the HHP [FILES] list is marked unlisted.
// A reference that doesn't exist on disk is marked missing.
func Step02_ProcessFile_HHC(hhcPath string) error {
	// output test header
	fmt.Printf("Step 2 - importing HHC file and checking the listed files...\n")

	// extract local references from HHC objects
	localRefs, err := parse_HHC_object_param_Local(hhcPath)
	if err != nil {
		return fmt.Errorf("cannot parse %s: %w", hhcPath, err)
	}

	// Snapshot current counts so we can report incremental deltas
	items_missing := len(missing_list)
	items_unlisted := len(unlisted_list)

	for _, item := range localRefs {
		process_project_item_ref(item, project_dir, ".HHC")
	}

	items_processed := len(localRefs)

	fmt.Printf("    %d files listed (+%d missing, +%d unlisted)\n", items_processed,
		len(missing_list)-items_missing, len(unlisted_list)-items_unlisted)

	return nil
}
