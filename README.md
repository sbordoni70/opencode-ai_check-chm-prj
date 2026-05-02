# check-chm-prj

A command-line tool that validates Microsoft HTML Help Workshop project files by checking for missing source files, files referenced outside the project manifest, and broken hyperlinks within HTML files.

## What It Does

The tool performs a five-phase audit of a CHM help project directory:

1. **HHP Analysis** — Parses the `[FILES]` section of the `.hhp` project file and verifies that every listed file exists on disk.
2. **HHC Analysis** — Parses the `.hhc` table-of-contents file, extracts all `name="local"` references from `<object>` tags, and cross-references them against the HHP manifest.
3. **HHK Analysis** — Parses the `.hhk` index file, extracts all `name="local"` references from `<object>` tags (including multiple references per object), and cross-references them against the HHP manifest.
4. **Hyperlink Validation (Present)** — Extracts all local hyperlinks from every HTML file in the present list, resolves them against the manifest, and classifies unknown targets by checking disk existence.
5. **Hyperlink Validation (Unlisted)** — Repeats hyperlink extraction on every file in the unlisted list, discovering additional missing or unlisted files transitively.

At the end it produces a final report with three categories:
- **Present** — files listed in the HHP and found on disk
- **Missing** — files referenced but not found on disk
- **Unlisted** — files found on disk and referenced in the HHC, HHK, or HTML hyperlinks, but missing from the HHP `[FILES]` section

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
  check-chm-prj v1.0.0
  CHM Project File Validator

Found project file: C:\help\myproject.hhp
Step 1 - importing HHP file and checking the listed files...
    42 files listed (38 present, 4 missing)

Found template file: C:\help\myproject.hhc
Step 2 - importing HHC file and checking the listed files...
    51 files listed (+2 missing, +3 unlisted)

Found index file: C:\help\myproject.hhk
Step 3 - importing HHK file and checking the listed files...
    37 files listed (+1 missing, +1 unlisted)

Step 4 - checking hyperlinks in present HTML files...
    128 hyperlinks checked (+3 missing, +2 unlisted)

Step 5 - checking hyperlinks in unlisted HTML files...
    19 hyperlinks checked (+1 missing, +0 unlisted)

--- Final Report ---
==== Present files: 38
==== Missing files (i.e. broken links/references): 11
    intro_overview.html
    api_reference.html
    troubleshooting.html
    changelog.html
    quick_start.html
    faq.html
    glossary.html
    broken_link.html
    old_page.html
    deprecated_api.html
    missing_image.html
==== Unlisted files to be added to HHP file: 5
    draft_notes.html
    legacy_v1.html
    appendix_b.html
    known_issues.html
    hidden_page.html
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

### HTML Hyperlinks (Steps 4-5)

The tool scans each HTML file in the present and unlisted lists for `<a href="...">` tags. Only local hyperlinks (relative paths or `file://` URLs) are processed. Each resolved target is classified:
- If already in the present, missing, or unlisted lists, it is skipped.
- Otherwise, the tool checks whether the file exists on disk: existing files are added to the unlisted list, non-existing to the missing list.

## Notes

- All file comparisons are case-insensitive.
- Duplicate entries are deduplicated across all three categories.
- If no `.hhc` file is found, only the HHP phase runs.
- If no `.hhk` file is found, only the HHP and HHC phases run.
- Hyperlink validation (Steps 4-5) only processes files with `.html` or `.htm` extensions.
- Hyperlinks are extracted using a simple regex pattern; complex or malformed HTML may produce false positives.
- The program version is customizable via the `Version` constant in `main.go`.
