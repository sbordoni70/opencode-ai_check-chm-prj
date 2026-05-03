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
	Version     = "2026.05.4.0"
)

// Global tracking lists and dedup sets for three categories of files.
var (
	project_dir      string
	project_dir_len  int
	project_dir_len2 int
	present_list     []string
	missing_list     []string
	missing_ref_list []string
	unlisted_list    []string
	presentSet       = make(map[string]bool)
	missingSet       = make(map[string]bool)
	unlistedSet      = make(map[string]bool)
)

// list_addIfNew appends an item to a list only if it hasn't been seen before (case-insensitive).
func list_addIfNew(list *[]string, set *map[string]bool, item string) bool {
	key := strings.ToLower(item)
	if (*set)[key] {
		return false
	}
	(*set)[key] = true
	*list = append(*list, item)
	return true
}

// like list_addIfNew but for missing list only
func list_missing_addIfNew(item string, ref string) {
	if list_addIfNew(&missing_list, &missingSet, item) {
		missing_ref_list = append(missing_ref_list, ref)
	}
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
	// print footer
	fmt.Printf("\n\n============================================================\n\n")
}

func main() {

	// print program header
	fmt.Fprintf(os.Stderr, "\n%s v%s\n  a small utility to check & report HTML files references problems in CHM project\n\n", ProgramName, Version)

	/*
		if len(os.Args) < 2 {
			fmt.Fprintf(os.Stderr, "  Usage: %s <project-folder>\n", ProgramName)
			os.Exit(1)
		}
	*/

	// set project dir
	//project_dir = os.Args[1]
	project_dir = "v:\\work\\0-git\\WinEPTS Tech. Ref"

	fmt.Fprintf(os.Stderr, "project dir:  \"%s\"\n\n", project_dir)

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

	fmt.Printf("Found HHP file: %s\n", hhpPath[project_dir_len2:])

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
		fmt.Printf("\nFound HHC file: %s\n", hhcPath[project_dir_len2:])

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
		fmt.Printf("\nFound index file: %s\n", hhkPath[project_dir_len2:])

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
