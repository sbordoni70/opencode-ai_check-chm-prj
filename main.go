package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ProgramName = "check-chm-prj"
	Version     = "2026.05.5.0"
)

// Global tracking lists and dedup sets for three categories of files.
var (
	project_dir      string
	project_dir_len  int
	project_dir_len2 int
	present_list     []string
	unlisted_list    []string
	missing_list     []string
	missing_ref_list []string
	invalid_list     []string
	invalid_ref_list []string
	presentSet       = make(map[string]bool)
	missingSet       = make(map[string]bool)
	unlistedSet      = make(map[string]bool)
)

// list_addIfNew appends an item to a list only if it hasn't been seen before (case-insensitive).
func list_addIfNew(list *[]string, set *map[string]bool, item string) bool {
	// check if it's already present in the set (case-insensitive)
	key := strings.ToLower(item)
	if (*set)[key] {
		return false
	}
	// append it
	(*set)[key] = true
	*list = append(*list, item)
	return true
}

// like list_addIfNew but for missing list only
func list_addIfInvalid(item string, ref string) bool {
	// check if it's an html page
	if isHref_HTML_page(item) {
		return false
	}
	// add it to invalid list
	invalid_list = append(invalid_list, (item))
	invalid_ref_list = append(invalid_ref_list, (ref))
	return true
}

// like list_addIfNew but for missing list only
func list_missing_addIfNew(item string, ref string) {
	if list_addIfNew(&missing_list, &missingSet, item) {
		missing_ref_list = append(missing_ref_list, ref)
	}
}

// check a generic project item reference (without anchor)
func process_project_item_ref(item, origin_dir, origin_item string) bool {
	// check if it's an invalid reference (e.g. non-html file)
	if list_addIfInvalid(item, origin_item) {
		return true
	}
	// Compute path relative to the HHC directory for cross-comparison with HHP list
	item_fullPath := item
	if !filepath.IsAbs(item) {
		item_fullPath = filepath.Join(origin_dir, item)
	}
	// check if it really local reference...
	if !no_case_HasPrefix(item_fullPath, project_dir) {
		return true
	}
	// get the project relative href
	item_rel := item_fullPath[project_dir_len2:]
	// is it already present in one of the target lists?
	key := strings.ToLower(item_rel)
	if missingSet[key] || unlistedSet[key] || presentSet[key] {
		return true
	}
	ret := true
	// check if it exists on disk
	_, statErr := os.Stat(item_fullPath)
	if statErr != nil {
		// File doesn't exist on disk
		list_missing_addIfNew(item_rel, origin_item)
		// if it isn't present in the project listed files...
	} else if list_addIfNew(&unlisted_list, &unlistedSet, item_rel) {
		// if added to unlisted, return false
		ret = false
	}
	return ret
}

// OutputFinalReport prints a summary of all three file categories.
func OutputFinalReport() {
	// print header
	fmt.Printf("\n==== Final Report ==========================================\n\n")
	// report present files
	fmt.Printf("---- Present files: %d\n\n", len(present_list))
	// report missing items
	items := len(missing_list)
	fmt.Printf("---- Missing files (i.e. broken links/references): %d\n", items)
	if items > 0 {
		//sort.Strings(missing_list)
		for i := 0; i < items; i++ {
			fmt.Printf("%s\n > from: %s\n", missing_list[i], missing_ref_list[i])
		}
	}
	// report unlisted items
	items = len(unlisted_list)
	fmt.Printf("\n---- Unlisted files to be added to HHP file: %d\n", items)
	if items > 0 {
		sort.Strings(unlisted_list)
		for _, f := range unlisted_list {
			fmt.Printf("%s\n", f)
		}
	}
	// invalid items
	items = len(invalid_list)
	if items > 0 {
		fmt.Printf("\n---- invalid/malformed local URLs: %d\n", items)
		//sort.Strings(invalid_list)
		for i := 0; i < items; i++ {
			fmt.Printf("\"%s\"\n > from: %s\n", invalid_list[i], invalid_ref_list[i])
		}
	}
	// print footer
	fmt.Printf("\n\n============================================================\n\n")
}

func main() {

	// print program header
	fmt.Fprintf(os.Stderr,
		"\n%s v%s\n  a small utility to check & report HTML files references problems in CHM project\n\n",
		ProgramName, Version)

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "  Usage: %s <project-folder>\n", ProgramName)
		os.Exit(1)
	}

	// set project dir
	project_dir = os.Args[1]
	if !filepath.IsAbs(project_dir) {
		absDir, err := filepath.Abs(project_dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid directory (err: %v)\n", err)
			os.Exit(1)
		}
		project_dir = absDir
	}

	// let's go
	fmt.Fprintf(os.Stderr, "-project dir:  \"%s\"\n\n", project_dir)

	// check project dir
	if _, err := os.Stat(project_dir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: directory %s does not exist\n", project_dir)
		os.Exit(1)
	}

	// init project_dir_len/len2 for later use
	project_dir_len = len(project_dir)
	switch project_dir[project_dir_len-1] {
	case '\\', '/':
		project_dir_len2 = project_dir_len
		project_dir_len--
	default:
		project_dir_len2 = project_dir_len + 1
	}

	// Step 01 - locate the HHP project file and validate its [FILES] list
	hhpPath, err := findHhpFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("-found HHP file: %s\n", hhpPath[project_dir_len2:])

	err = Step01_ProcessFile_HHP(hhpPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Step 02 - resolve the HHC file from the HHP [OPTIONS] section and validate references
	hhcRelPath, err := parse_HHP_section_OPTIONS(hhpPath, "contents file")
	if err != nil {
		fmt.Printf("\nNo HHC file specified in HHP: %v\n", err)
	} else {
		hhcPath := hhcRelPath
		if !filepath.IsAbs(hhcRelPath) {
			hhcPath = filepath.Join(filepath.Dir(hhpPath), hhcRelPath)
		}
		fmt.Printf("\n-found HHC file: %s\n", hhcPath[project_dir_len2:])

		err := Step02_ProcessFile_HHC(hhcPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	// Step 03 - resolve the HHK file from the HHP [OPTIONS] section and validate references
	hhkRelPath, err := parse_HHP_section_OPTIONS(hhpPath, "index file")
	if err != nil {
		fmt.Printf("\nNo HHK file specified in HHP: %v\n", err)
	} else {
		hhkPath := hhkRelPath
		if !filepath.IsAbs(hhkRelPath) {
			hhkPath = filepath.Join(filepath.Dir(hhpPath), hhkRelPath)
		}
		fmt.Printf("\n-found HHK file: %s\n", hhkPath[project_dir_len2:])

		err := Step03_ProcessFile_HHK(hhkPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	// Step 04 - check hyperlinks in all present HTML files
	err = Step04_PresentList_CheckHyperlinks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Step 05 - check hyperlinks in all unlisted HTML files
	err = Step05_UnlistedList_CheckHyperlinks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Print the final summary report
	OutputFinalReport()

	os.Exit(0)
}
