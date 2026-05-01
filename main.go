package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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

		if strings.ToUpper(line) == "[OPTIONS]" {
			inOptions = true
			continue
		}

		if inOptions {
			if line == "" || (strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")) {
				break
			}
			lower := strings.ToLower(line)
			entry := entry + "="
			if strings.HasPrefix(lower, entry) {
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
			if line == "" || (strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")) {
				break
			}
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

var (
	present     []string
	missing     []string
	unlisted    []string
	presentSet  = make(map[string]bool)
	missingSet  = make(map[string]bool)
	unlistedSet = make(map[string]bool)
)

func addIfNew(list *[]string, set *map[string]bool, item string) {
	key := strings.ToLower(item)
	if (*set)[key] {
		return
	}
	(*set)[key] = true
	*list = append(*list, item)
}

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

func Step02_ProcessFile_HHC(projectDir string, hhcPath string) (int, error) {
	fmt.Printf("Step 2 - importing HHC file and checking the listed files...\n")
	localRefs, err := parseLocalParams(hhcPath)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %s: %w", hhcPath, err)
	}

	hhcDir := filepath.Dir(hhcPath)

	for _, ref := range localRefs {
		fullPath := ref
		if !filepath.IsAbs(ref) {
			fullPath = filepath.Join(hhcDir, ref)
		}

		relPath, err := filepath.Rel(hhcDir, fullPath)
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

	return len(localRefs), nil
}

func parseLocalParams(hhcPath string) ([]string, error) {
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

	for {
		objStart := strings.Index(content, "<object")
		if objStart == -1 {
			break
		}

		objEnd := strings.Index(content[objStart:], "</object>")
		if objEnd == -1 {
			break
		}
		objEnd += objStart + len("</object>")
		objBlock := content[objStart:objEnd]
		content = content[objEnd:]

		paramStart := strings.Index(objBlock, `<param`)
		if paramStart == -1 {
			continue
		}

		nameLower := strings.ToLower(objBlock[paramStart:])
		localIdx := strings.Index(nameLower, `name="local"`)
		if localIdx == -1 {
			continue
		}

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
		if ref != "" {
			refs = append(refs, ref)
		}
	}

	return refs, nil
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

	hhcRelPath, err := parse_HHP_section_OPTIONS(hhpPath, "contents file")
	if err != nil {
		fmt.Printf("\nNo HHC file specified in HHP: %v\n", err)
	} else {
		hhcPath := hhcRelPath
		if !filepath.IsAbs(hhcRelPath) {
			hhcPath = filepath.Join(filepath.Dir(hhpPath), hhcRelPath)
		}
		fmt.Printf("\nFound template file: %s\n", hhcPath)

		processed, err := Step02_ProcessFile_HHC(projectDir, hhcPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("HHC: %d items processed\n", processed)
	}

	OutputFinalReport()

	if len(missing) > 0 {
		os.Exit(2)
	}
}

func OutputFinalReport() {
	fmt.Printf("\n--- Final Report ---\n")
	// report present files
	fmt.Printf("==== Present files: %d\n", len(present))
	// report missing items
	items := len(missing)
	fmt.Printf("==== Missing files: %d\n", items)
	if items > 0 {
		for _, f := range missing {
			fmt.Printf("%s\n", f)
		}
	}
	// report unlisted items
	items = len(unlisted)
	fmt.Printf("==== Unlisted files: %d\n", items)
	if items > 0 {
		for _, f := range unlisted {
			fmt.Printf("%s\n", f)
		}
	}
}
