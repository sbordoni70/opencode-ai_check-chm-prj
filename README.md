# check-chm-prj

A command-line tool that validates Microsoft HTML Help Workshop project files by checking for missing source files and files referenced outside the project manifest.

## What It Does

The tool performs a two-phase audit of a CHM help project directory:

1. **HHP Analysis** — Parses the `[FILES]` section of the `.hhp` project file and verifies that every listed file exists on disk.
2. **HHC Analysis** — Parses the `.hhc` table-of-contents file, extracts all `name="local"` references from `<object>` tags, and cross-references them against the HHP manifest.

At the end it produces a final report with three categories:
- **Present** — files listed in the HHP and found on disk
- **Missing** — files referenced but not found on disk
- **Unlisted** — files found on disk and referenced in the HHC, but missing from the HHP `[FILES]` section

## Building

Requires a [Go](https://go.dev/) toolchain (1.16+).

```bash
go build -o check-chm-prj.exe .
```

## Usage

```bash
check-chm-prj.exe <project-folder>
```

The tool will recursively search `<project-folder>` for a `.hhp` file and an optional `.hhc` file.

### Exit Codes

| Code | Meaning |
|------|---------|
| 0    | All files accounted for |
| 1    | Usage error or missing project files |
| 2    | One or more files are missing |

### Example Output

```
Found project file: C:\help\myproject.hhp
Step 1 - importing HHP file and checking the listed files...
HHP: 42 files listed (38 present, 4 missing)

Found template file: C:\help\myproject.hhc
Step 2 - importing HHC file and checking the listed files...
HHC: 51 items processed

--- Final Report ---
Present files: 38
Missing files: 4
  [MISSING]  intro_overview.html
  [MISSING]  api_reference.html
  [MISSING]  troubleshooting.html
  [MISSING]  changelog.html
Unlisted files: 3
  [UNLISTED] draft_notes.html
  [UNLISTED] legacy_v1.html
  [UNLISTED] appendix_b.html
```

## File Format Details

### HHP (`[FILES]` section)

The tool reads the `.hhp` file line-by-line, enters parsing mode upon hitting `[FILES]`, and stops at the next section header or blank line. Lines starting with `;` or `#` are skipped as comments.

### HHC (`name="local"` references)

The tool scans the `.hhc` file for `<object>` blocks and extracts the `value` attribute from any `<param name="local" value="...">` tag. These values are treated as file paths and resolved relative to the HHC file's directory.

## Notes

- All file comparisons are case-insensitive.
- Duplicate entries are deduplicated across all three categories.
- If no `.hhc` file is found, only the HHP phase runs.
