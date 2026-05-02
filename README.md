# check-chm-prj

A command-line tool that validates Microsoft HTML Help Workshop project files by checking for missing source files and files referenced outside the project manifest.

## What It Does

The tool performs a three-phase audit of a CHM help project directory:

1. **HHP Analysis** — Parses the `[FILES]` section of the `.hhp` project file and verifies that every listed file exists on disk.
2. **HHC Analysis** — Parses the `.hhc` table-of-contents file, extracts all `name="local"` references from `<object>` tags, and cross-references them against the HHP manifest.
3. **HHK Analysis** — Parses the `.hhk` index file, extracts all `name="local"` references from `<object>` tags (including multiple references per object), and cross-references them against the HHP manifest.

At the end it produces a final report with three categories:
- **Present** — files listed in the HHP and found on disk
- **Missing** — files referenced but not found on disk
- **Unlisted** — files found on disk and referenced in the HHC or HHK, but missing from the HHP `[FILES]` section

## Building

Requires a [Go](https://go.dev/) toolchain (1.16+).

```bash
go build -o check-chm-prj.exe .
```

## Usage

```bash
check-chm-prj.exe <project-folder>
```

The tool will recursively search `<project-folder>` for a `.hhp` file, and optional `.hhc` and `.hhk` files.

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
    42 files listed (38 present, 4 missing)

Found template file: C:\help\myproject.hhc
Step 2 - importing HHC file and checking the listed files...
    51 files listed (+2 missing, +3 unlisted)

Found index file: C:\help\myproject.hhk
Step 3 - importing HHK file and checking the listed files...
    37 files listed (+1 missing, +1 unlisted)

--- Final Report ---
==== Present files: 38
==== Missing files: 7
    intro_overview.html
    api_reference.html
    troubleshooting.html
    changelog.html
    quick_start.html
    faq.html
    glossary.html
==== Unlisted files: 4
    draft_notes.html
    legacy_v1.html
    appendix_b.html
    known_issues.html
```

## File Format Details

### HHP (`[FILES]` section)

The tool reads the `.hhp` file line-by-line, enters parsing mode upon hitting `[FILES]`, and stops at the next section header or blank line. Lines starting with `;` or `#` are skipped as comments.

### HHP (`[OPTIONS]` section)

The tool reads the `[OPTIONS]` section of the `.hhp` file to locate the `contents file` (HHC) and `index file` (HHK) paths. Both are resolved relative to the HHP file's directory if not absolute.

### HHC (`name="local"` references)

The tool scans the `.hhc` file for `<object>` blocks and extracts the `value` attribute from the first `<param name="local" value="...">` tag found per block. These values are treated as file paths and resolved relative to the HHC file's directory.

### HHK (`name="local"` references)

The tool scans the `.hhk` file for `<object>` blocks and extracts the `value` attribute from **all** `<param name="local" value="...">` tags within each block. A single object may reference multiple files. These values are treated as file paths and resolved relative to the HHK file's directory.

## Notes

- All file comparisons are case-insensitive.
- Duplicate entries are deduplicated across all three categories.
- If no `.hhc` file is found, only the HHP phase runs.
- If no `.hhk` file is found, only the HHP and HHC phases run.
