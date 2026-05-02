package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ProgramName = "check-chm-prj"
	Version     = "2026.05.1.0"
)

// Global tracking lists and dedup sets for three categories of files.
var (
	present     []string
	missing     []string
	unlisted    []string
	presentSet  = make(map[string]bool)
	missingSet  = make(map[string]bool)
	unlistedSet = make(map[string]bool)
)

// addIfNew appends an item to a list only if it hasn't been seen before (case-insensitive).
func addIfNew(list *[]string, set *map[string]bool, item string) {
	key := strings.ToLower(item)
	if (*set)[key] {
		return
	}
	(*set)[key] = true
	*list = append(*list, item)
}

// findHhpFile walks the given directory tree and returns the first .hhp file found.
func findHhpFile(dir string) (string, error) {
	var hhpFile string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.EqualFold(filepath.Ext(path), ".hhp") {
			hhpFile = path
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if hhpFile == "" {
		return "", fmt.Errorf("no .hhp file found in %s", dir)
	}
	return hhpFile, nil
}

// parse_HHP_section_OPTIONS finds a key=value entry inside the [OPTIONS] block of an HHP file.
// The `entry` argument is matched case-insensitively (e.g. "contents file").
func parse_HHP_section_OPTIONS(hhpPath string, entry string) (string, error) {
	f, err := os.Open(hhpPath)
	if err != nil {
		return "", fmt.Errorf("cannot open %s: %w", hhpPath, err)
	}
	defer f.Close()

	inOptions := false
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Enter [OPTIONS] section
		if strings.ToUpper(line) == "[OPTIONS]" {
			inOptions = true
			continue
		}

		if inOptions {
			// Blank line or next section header means we've left [OPTIONS]
			if line == "" || (strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")) {
				break
			}
			lower := strings.ToLower(line)
			prefix := entry + "="
			if strings.HasPrefix(lower, prefix) {
				idx := strings.Index(line, "=")
				val := strings.TrimSpace(line[idx+1:])
				return val, nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading %s: %w", hhpPath, err)
	}

	return "", fmt.Errorf("no Contents file entry found in [OPTIONS] section of %s", hhpPath)
}

// parse_HHP_section_FILES reads the [FILES] block and returns every non-comment line.
func parse_HHP_section_FILES(hhpPath string) ([]string, error) {
	f, err := os.Open(hhpPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s: %w", hhpPath, err)
	}
	defer f.Close()

	var files []string
	inFilesSection := false
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if section := strings.ToUpper(line); section == "[FILES]" {
			inFilesSection = true
			continue
		}

		if inFilesSection {
			// Blank line or next section header means we've left [FILES]
			if line == "" || (strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")) {
				break
			}
			// Skip comment lines starting with ; or #
			if !strings.HasPrefix(line, ";") && !strings.HasPrefix(line, "#") {
				files = append(files, line)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading %s: %w", hhpPath, err)
	}

	return files, nil
}

// Step01_ProcessFile_HHP checks every file listed in the HHP [FILES] section
// and classifies each as present (found on disk) or missing.
func Step01_ProcessFile_HHP(projectDir string, hhpPath string) error {
	fmt.Printf("Step 1 - importing HHP file and checking the listed files...\n")
	files, err := parse_HHP_section_FILES(hhpPath)
	if err != nil {
		return fmt.Errorf("cannot parse %s: %w", hhpPath, err)
	}

	for _, f := range files {
		fullPath := f
		if !filepath.IsAbs(f) {
			fullPath = filepath.Join(projectDir, f)
		}
		if _, err := os.Stat(fullPath); err == nil {
			addIfNew(&present, &presentSet, f)
		} else {
			addIfNew(&missing, &missingSet, f)
		}
	}

	total := len(present) + len(missing)
	fmt.Printf("    %d files listed (%d present, %d missing)\n", total, len(present), len(missing))

	return nil
}

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
		objStart := strings.Index(content, "<OBJECT")
		if objStart == -1 {
			break
		}

		objEnd := strings.Index(content[objStart:], "</OBJECT>")
		if objEnd == -1 {
			break
		}
		objEnd += objStart + len("</OBJECT>")
		objBlock := content[objStart:objEnd]
		content = content[objEnd:]

		// Look for a <param tag inside this object block
		offset := strings.Index(objBlock, `<param name="Local" value="`)
		if offset == -1 {
			continue
		}

		// update offset to the start of this param tag for the next search
		offset += len(`<param name="Local" value="`)
		// get value end quote position
		index := strings.Index(objBlock[offset:], `">`)
		if index == -1 {
			continue
		}

		// extract the value and strip trailing fragment if any
		ref := objBlock[offset : offset+index]
		ref = strings.TrimSpace(ref)
		// Strip trailing fragment anchor (e.g. "page.html#section" -> "page.html")
		idx := strings.Index(ref, "#")
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
	items_missing := len(missing)
	items_unlisted := len(unlisted)

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
			addIfNew(&missing, &missingSet, ref)
		} else if !presentSet[strings.ToLower(relPath)] {
			// File exists but wasn't listed in the HHP [FILES] section
			addIfNew(&unlisted, &unlistedSet, relPath)
		}
	}

	items_processed := len(localRefs)

	fmt.Printf("    %d files listed (+%d missing, +%d unlisted)\n\n", items_processed,
		len(missing)-items_missing, len(unlisted)-items_unlisted)

	return nil
}

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
		objStart := strings.Index(content, "<OBJECT ")
		if objStart == -1 {
			break
		}

		objEnd := strings.Index(content[objStart:], "</OBJECT>")
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
			index := strings.Index(objBlock[offset:], `<param name="local" value="`)
			if index == -1 {
				break
			}

			// update offset to the start of this param tag for the next iteration
			offset += index + len(`<param name="Local" value="`)
			// update offset to the start of the actual value after value=""
			index = strings.Index(objBlock[offset:], `">`)
			if index == -1 {
				continue
			}

			// extract the value and strip trailing fragment if any
			ref := objBlock[offset : offset+index]
			ref = strings.TrimSpace(ref)
			idx := strings.Index(ref, "#")
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

// isLocalHref determines whether an href value is a local file reference.
// It returns false for empty strings, external protocols, and fragment-only links.
func isLocalHref(href string) bool {
	lower := strings.ToLower(href)
	if lower == "" {
		return false
	}
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "ftp://") || strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") {
		return false
	}
	if strings.HasPrefix(href, "#") {
		return false
	}
	return true
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
		aStart := strings.Index(content, "<a ")
		if aStart == -1 {
			aStart = strings.Index(content, "<A ")
		}
		if aStart == -1 {
			break
		}

		aEnd := strings.Index(content[aStart:], ">")
		if aEnd == -1 {
			break
		}
		aTag := content[aStart : aStart+aEnd]
		content = content[aStart+aEnd:]

		tagLower := strings.ToLower(aTag)
		hrefIdx := strings.Index(tagLower, `href="`)
		if hrefIdx == -1 {
			continue
		}

		valueStart := hrefIdx + len(`href="`)
		valueEnd := strings.Index(aTag[valueStart:], `"`)
		if valueEnd == -1 {
			continue
		}

		href := aTag[valueStart : valueStart+valueEnd]
		href = strings.TrimSpace(href)

		if idx := strings.Index(href, "#"); idx != -1 {
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

	itemsMissingBefore := len(missing)
	itemsUnlistedBefore := len(unlisted)
	totalHrefs := 0

	for _, f := range present {
		ext := strings.ToLower(filepath.Ext(f))
		if ext != ".html" && ext != ".htm" {
			continue
		}

		fullPath := f
		if !filepath.IsAbs(f) {
			fullPath = filepath.Join(projectDir, f)
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
				addIfNew(&unlisted, &unlistedSet, relPath)
			} else {
				addIfNew(&missing, &missingSet, relPath)
			}
		}
	}

	fmt.Printf("    %d hyperlinks checked (+%d missing, +%d unlisted)\n\n", totalHrefs,
		len(missing)-itemsMissingBefore, len(unlisted)-itemsUnlistedBefore)

	return nil
}

// Step05_UnlistedList_CheckHyperlinks iterates over every HTML file in the unlisted list,
// extracts local hyperlinks, and checks each target against the present/missing/unlisted lists.
// Targets not in any list are classified by checking disk existence.
func Step05_UnlistedList_CheckHyperlinks(projectDir string) error {
	fmt.Printf("Step 5 - checking hyperlinks in unlisted HTML files...\n")

	itemsMissingBefore := len(missing)
	itemsUnlistedBefore := len(unlisted)
	totalHrefs := 0

	for _, f := range unlisted {
		ext := strings.ToLower(filepath.Ext(f))
		if ext != ".html" && ext != ".htm" {
			continue
		}

		fullPath := f
		if !filepath.IsAbs(f) {
			fullPath = filepath.Join(projectDir, f)
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
				addIfNew(&unlisted, &unlistedSet, relPath)
			} else {
				addIfNew(&missing, &missingSet, relPath)
			}
		}
	}

	fmt.Printf("    %d hyperlinks checked (+%d missing, +%d unlisted)\n\n", totalHrefs,
		len(missing)-itemsMissingBefore, len(unlisted)-itemsUnlistedBefore)

	return nil
}

// OutputFinalReport prints a summary of all three file categories.
func OutputFinalReport() {
	// print header
	fmt.Printf("\n==== Final Report ==========================================\n\n")
	// report present files
	fmt.Printf("---- Present files: %d\n\n", len(present))
	// report missing items
	items := len(missing)
	fmt.Printf("---- Missing files (i.e. broken links/references): %d\n", items)
	if items > 0 {
		sort.Strings(missing)
		for _, f := range missing {
			fmt.Printf("%s\n", f)
		}
		fmt.Printf("\n")
	}
	// report unlisted items
	items = len(unlisted)
	fmt.Printf("---- Unlisted files to be added to HHP file: %d\n", items)
	if items > 0 {
		sort.Strings(unlisted)
		for _, f := range unlisted {
			fmt.Printf("%s\n", f)
		}
		fmt.Printf("\n")
	}
	// print footer
	fmt.Printf("\n============================================================\n\n")
}

func main() {

	// print program header
	fmt.Fprintf(os.Stderr, "\n%s v%s\n  a small utility to check & report HTML files references problems in CHM project\n\n", ProgramName, Version)

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "  Usage: %s <project-folder>\n", ProgramName)
		os.Exit(1)
	}

	projectDir := os.Args[1]

	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: directory %s does not exist\n", projectDir)
		os.Exit(1)
	}

	// Step 01 - locate the HHP project file and validate its [FILES] list
	hhpPath, err := findHhpFile(projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found project file: %s\n", hhpPath)

	err = Step01_ProcessFile_HHP(projectDir, hhpPath)
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
		fmt.Printf("\nFound template file: %s\n", hhcPath)

		err := Step02_ProcessFile_HHC(projectDir, hhcPath)
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
		fmt.Printf("\nFound index file: %s\n", hhkPath)

		err := Step03_ProcessFile_HHK(projectDir, hhkPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	// Step 04 - check hyperlinks in all present HTML files
	err = Step04_PresentList_CheckHyperlinks(projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Step 05 - check hyperlinks in all unlisted HTML files
	err = Step05_UnlistedList_CheckHyperlinks(projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Print the final summary report
	OutputFinalReport()

	// Exit with code 2 if any files are missing
	if len(missing) > 0 {
		os.Exit(2)
	}
}
