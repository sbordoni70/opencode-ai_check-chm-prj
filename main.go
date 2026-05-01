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

func parseFilesSection(hhpPath string) ([]string, error) {
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
	present      []string
	missing      []string
	unlisted     []string
	presentSet   = make(map[string]bool)
	missingSet   = make(map[string]bool)
	unlistedSet  = make(map[string]bool)
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
	files, err := parseFilesSection(hhpPath)
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

	hhpDir := filepath.Dir(hhpPath)
	err = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(hhpDir, path)
		if err != nil {
			return nil
		}

		if !presentSet[strings.ToLower(relPath)] {
			addIfNew(&unlisted, &unlistedSet, relPath)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error scanning project dir: %w", err)
	}

	return nil
}

func Step02_ProcessFile_HHT(projectDir string, hhtPath string) (int, error) {
	fmt.Printf("Step 2 - importing HHT file and checking the listed files...\n")
	localRefs, err := parseLocalParams(hhtPath)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %s: %w", hhtPath, err)
	}

	hhtDir := filepath.Dir(hhtPath)

	for _, ref := range localRefs {
		fullPath := ref
		if !filepath.IsAbs(ref) {
			fullPath = filepath.Join(hhtDir, ref)
		}

		relPath, err := filepath.Rel(hhtDir, fullPath)
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

func parseLocalParams(hhtPath string) ([]string, error) {
	f, err := os.Open(hhtPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s: %w", hhtPath, err)
	}
	defer f.Close()

	data, err := os.ReadFile(hhtPath)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", hhtPath, err)
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

	total := len(present) + len(missing)
	fmt.Printf("HHP: %d files listed (%d present, %d missing)\n", total, len(present), len(missing))
	if len(unlisted) > 0 {
		fmt.Printf("     %d files on disk not listed in HHP\n", len(unlisted))
	}

	hhtPath, err := findHhtFile(projectDir)
	if err == nil {
		fmt.Printf("\nFound template file: %s\n", hhtPath)

		processed, err := Step02_ProcessFile_HHT(projectDir, hhtPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("HHT: %d items processed\n", processed)
		fmt.Printf("     Total present: %d, missing: %d, unlisted: %d\n", len(present), len(missing), len(unlisted))
	}

	if len(missing) > 0 {
		fmt.Printf("\nFinal: %d missing files.\n", len(missing))
		os.Exit(2)
	}
	fmt.Printf("\nFinal: All files accounted for.\n")
}

func findHhtFile(dir string) (string, error) {
	var hhtFile string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.EqualFold(filepath.Ext(path), ".hht") {
			hhtFile = path
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if hhtFile == "" {
		return "", fmt.Errorf("no .hht file found in %s", dir)
	}
	return hhtFile, nil
}
