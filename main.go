package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		paramStart := strings.Index(objBlock, `<param`)
		if paramStart == -1 {
			continue
		}

		// Work on a lowercased copy for case-insensitive matching
		nameLower := strings.ToLower(objBlock[paramStart:])
		localIdx := strings.Index(nameLower, `name="local"`)
		if localIdx == -1 {
			continue
		}

		// Find the corresponding value="..." after name="local"
		valueIdx := strings.Index(nameLower[localIdx:], `value="`)
		if valueIdx == -1 {
			continue
		}
		valueStart := localIdx + valueIdx + len(`value="`)

		valueEnd := strings.Index(nameLower[valueStart:], `"`)
		if valueEnd == -1 {
			continue
		}

		ref := nameLower[valueStart : valueStart+valueEnd]
		ref = strings.TrimSpace(ref)
		// Strip trailing fragment anchor (e.g. "page.html#section" -> "page.html")
		if idx := strings.Index(ref, "#"); idx != -1 {
			ref = ref[:idx]
		}
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

	fmt.Printf("    %d files listed (+%d missing, +%d unlisted)\n", items_processed,
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

		// Collect ALL name="local" params within this object block
		remain := objBlock
		for {
			paramStart := strings.Index(strings.ToLower(remain), `<param`)
			if paramStart == -1 {
				break
			}

			afterParam := strings.ToLower(remain[paramStart:])
			localIdx := strings.Index(afterParam, `name="local"`)
			if localIdx == -1 {
				remain = remain[paramStart+1:]
				continue
			}

			valueIdx := strings.Index(afterParam[localIdx:], `value="`)
			if valueIdx == -1 {
				remain = remain[paramStart+1:]
				continue
			}
			valueStartAbs := paramStart + localIdx + valueIdx + len(`value="`)
			quoteEnd := strings.Index(remain[valueStartAbs:], `"`)
			if quoteEnd == -1 {
				remain = remain[paramStart+1:]
				continue
			}

			ref := remain[valueStartAbs : valueStartAbs+quoteEnd]
			ref = strings.TrimSpace(ref)
			if idx := strings.Index(ref, "#"); idx != -1 {
				ref = ref[:idx]
			}
			if ref != "" {
				refs = append(refs, ref)
			}

			remain = remain[paramStart+1:]
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
		fullPath := ref
		if !filepath.IsAbs(ref) {
			fullPath = filepath.Join(hhkDir, ref)
		}

		relPath, err := filepath.Rel(hhkDir, fullPath)
		if err != nil {
			relPath = ref
		}

		_, statErr := os.Stat(fullPath)
		if statErr != nil {
			addIfNew(&missing, &missingSet, ref)
		} else if !presentSet[strings.ToLower(relPath)] {
			addIfNew(&unlisted, &unlistedSet, relPath)
		}
	}

	items_processed := len(localRefs)

	fmt.Printf("    %d files listed (+%d missing, +%d unlisted)\n", items_processed,
		len(missing)-items_missing, len(unlisted)-items_unlisted)

	return nil
}

// OutputFinalReport prints a summary of all three file categories.
func OutputFinalReport() {
	fmt.Printf("\n--- Final Report ---\n\n")
	// report present files
	fmt.Printf("==== Present files: %d\n", len(present))
	// report missing items
	items := len(missing)
	fmt.Printf("==== Missing files: %d\n", items)
	if items > 0 {
		for _, f := range missing {
			fmt.Printf("%s\n", f)
		}
		fmt.Printf("\n")
	}
	// report unlisted items
	items = len(unlisted)
	fmt.Printf("==== Unlisted files: %d\n", items)
	if items > 0 {
		for _, f := range unlisted {
			fmt.Printf("%s\n", f)
		}
		fmt.Printf("\n")
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <project-folder>\n", os.Args[0])
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

	// Print the final summary report
	OutputFinalReport()

	// Exit with code 2 if any files are missing
	if len(missing) > 0 {
		os.Exit(2)
	}
}
