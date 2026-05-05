# check-chm-prj

A command-line tool that validates Microsoft HTML Help Workshop project files by checking for missing source files, files referenced outside the project manifest, and broken hyperlinks within HTML files.

## What It Does

The tool performs a five-phase audit of a CHM help project directory:

1. **HHP Analysis** — Parses the `[FILES]` section of the `.hhp` project file and verifies that every listed file exists on disk. Non-HTML files (e.g. images) are classified as invalid references.
2. **HHC Analysis** — Reads the HHC file path from the HHP `[OPTIONS]` section, parses the table-of-contents file, extracts all `name="local"` references from `<object>` tags, and cross-references them against the HHP manifest using unified reference processing.
3. **HHK Analysis** — Reads the HHK file path from the HHP `[OPTIONS]` section, parses the index file, extracts all `name="local"` references from `<object>` tags (including multiple references per object), and cross-references them against the HHP manifest using unified reference processing.
4. **Hyperlink Validation (Present)** — Extracts all local hyperlinks from every file in the present list, resolves them against the manifest, and classifies unknown targets by checking disk existence.
5. **Hyperlink Validation (Unlisted)** — Repeats hyperlink extraction on every file in the unlisted list, discovering additional missing or unlisted files transitively.

At the end it produces a final report with four categories:
- **Present** — files listed in the HHP and found on disk
- **Missing** — files referenced but not found on disk (each entry shows the source file that references it)
- **Unlisted** — files found on disk and referenced in the HHC, HHK, or HTML hyperlinks, but missing from the HHP `[FILES]` section
- **Invalid** — non-HTML file references (e.g. images, PDFs)

## Building

Requires a [Go](https://go.dev/) toolchain (1.16+).

```bash
go build -o check-chm-prj.exe .
```

## Usage

```bash
check-chm-prj.exe <project-folder>
```

The tool will recursively search `<project-folder>` for a `.hhp` file. The `.hhc` and `.hhk` file paths are then read from the HHP's `[OPTIONS]` section. The program header and project directory info are printed to stderr; step progress and the final report go to stdout.

### Exit Codes

| Code | Meaning |
|------|---------|
| 0    | Check completed (regardless of findings) |
| 1    | Usage error or missing project files |

### Example Output

```
check-chm-prj v2026.05.5.1
  a small utility to check & report HTML files references problems in CHM project

-project dir:  "C:\help"

-found HHP file: myproject.hhp
Step 1 - importing HHP file and checking the listed files...
    42 files listed (38 present, 4 missing)

-found HHC file: myproject.hhc
Step 2 - importing HHC file and checking the listed files...
    51 files listed (+2 missing, +3 unlisted)

-found HHK file: myproject.hhk
Step 3 - importing HHK file and checking the listed files...
    37 files listed (+1 missing, +1 unlisted)

Step 4 - checking hyperlinks in present HTML files...
    128 hyperlinks checked (+3 missing, +2 unlisted)

Step 5 - checking hyperlinks in unlisted HTML files...
    19 hyperlinks checked (+1 missing, +0 unlisted)

==== Final Report ==========================================

---- Present files: 38

---- Missing files (i.e. broken links/references): 11
intro_overview.html
 > from: .HHP
api_reference.html
 > from: .HHC
troubleshooting.html
 > from: index.html
changelog.html
 > from: .HHK
quick_start.html
 > from: intro_overview.html
faq.html
 > from: .HHP
glossary.html
 > from: contents.html
broken_link.html
 > from: troubleshooting.html
old_page.html
 > from: changelog.html
deprecated_api.html
 > from: api_reference.html
missing_image.html
 > from: quick_start.html

---- Unlisted files to be added to HHP file: 5
appendix_b.html
draft_notes.html
hidden_page.html
known_issues.html
legacy_v1.html

============================================================
```

## File Format Details

### HHP (`[FILES]` section)

The tool reads the `.hhp` file line-by-line, enters parsing mode upon hitting `[FILES]`, and stops at the next section header or blank line. Lines starting with `;` or `#` are skipped as comments.

### HHP (`[OPTIONS]` section)

The tool reads the `[OPTIONS]` section of the `.hhp` file to locate the `contents file` (HHC) and `index file` (HHK) paths. Both are resolved relative to the HHP file's directory if not absolute.

### HHC (`name="local"` references)

The tool scans the `.hhc` file for `<object>` blocks and extracts the `value` attribute from the first `<param name="local" value="...">` tag found per block. Trailing `#fragment` anchors are stripped. These values are treated as file paths and resolved relative to the project directory. Each reference is processed through a unified function that classifies it as invalid (non-HTML), missing, unlisted, or already known.

### HHK (`name="local"` references)

The tool scans the `.hhk` file for `<object>` blocks and extracts the `value` attribute from **all** `<param name="local" value="...">` tags within each block. A single object may reference multiple files. Trailing `#fragment` anchors are stripped. These values are treated as file paths and resolved relative to the project directory. Each reference is processed through the same unified function as HHC references.

### HTML Hyperlinks (Steps 4-5)

The tool scans each file in the present and unlisted lists for `<a href="...">` tags. External protocols (`http://`, `https://`, `ftp://`, `mailto:`, `javascript:`, `data:`) and fragment-only links (`#...`) are skipped. Trailing `#fragment` anchors are stripped from local links. Each resolved target is classified using the same unified reference processing as Steps 1-3:
- Non-HTML file references are added to the invalid list.
- If already in the present, missing, or unlisted lists, the reference is skipped.
- Otherwise, the tool checks whether the file exists on disk: existing files are added to the unlisted list, non-existing to the missing list.
- In Step 5, newly discovered unlisted HTML files are dynamically added to the scan queue, so hyperlinks within them are also checked.

## Notes

- All file comparisons are case-insensitive.
- Duplicate entries are deduplicated across all four categories.
- The tool searches for the first `.hhp` file in the project directory and stops at the first match.
- The HHC and HHK file paths are read from the `[OPTIONS]` section of the `.hhp` file. If either is not specified, the corresponding phase is skipped.
- Steps 2-5 use a unified reference processing function for consistent classification of project items.
- Non-HTML file references (e.g. images, PDFs) are reported in the invalid category.
- Step 5 dynamically expands: when a hyperlink in an unlisted file reveals another unlisted HTML file, that file's hyperlinks are also checked.
- If an HTML file cannot be read during Steps 4-5, a warning is printed and the file is skipped.
- Hyperlinks are extracted via simple string scanning; complex or malformed HTML may produce false positives.
- The program version is customizable via the `Version` constant in `main.go`.
- The program name is customizable via the `ProgramName` constant in `main.go`.
