package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// findHhpFile walks the given directory tree and returns the first .hhp file found.
func findHhpFile(dir string) (string, error) {
	var hhpFile string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && no_case_IsEqual(filepath.Ext(path), ".hhp") {
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

		// look for the need section...
		if line[0] == '[' {
			inOptions = no_case_IsEqual(line, "[OPTIONS]")
		}
		if inOptions {
			// Blank line or next section header means we've left [OPTIONS]
			if line == "" {
				break
			}
			prefix := entry + "="
			if no_case_HasPrefix(line, prefix) {
				idx := no_case_SeekSubstring(line, "=")
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

		if !inFilesSection && no_case_IsEqual(line, "[FILES]") {
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
			list_addIfNew(&present_list, &presentSet, f)
		} else {
			list_missing_addIfNew(f, ".HHP")
		}
	}

	total := len(present_list) + len(missing_list)
	fmt.Printf("    %d files listed (%d present, %d missing)\n", total, len(present_list), len(missing_list))

	return nil
}
