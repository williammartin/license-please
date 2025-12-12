# license-please

A tool to help with Go OSS license compliance. Scans your Go project's dependencies, classifies their licenses, and generates a comprehensive report.

## Installation

```bash
go install github.com/williammartin/licenseplease/cmd/license-please@latest
```

## Usage

Generate a license report for your Go project:

```bash
# Run in current directory
license-please report

# Or specify a project directory
license-please report /path/to/project
```

## Example Output

```markdown
# Third-Party Licenses

This file contains the licenses for all third-party dependencies.

## Manifest

| Module | Version | License | Source |
|--------|---------|---------|--------|
| github.com/alecthomas/kong | v1.13.0 | MIT | [LICENSE](https://pkg.go.dev/github.com/alecthomas/kong@v1.13.0?tab=licenses) |
| github.com/google/licenseclassifier/v2 | v2.0.0 | Apache-2.0 | [LICENSE](https://pkg.go.dev/github.com/google/licenseclassifier/v2@v2.0.0?tab=licenses) |

---

## License Texts

### github.com/alecthomas/kong v1.13.0

**License:** MIT

**Source:** [LICENSE](https://pkg.go.dev/github.com/alecthomas/kong@v1.13.0?tab=licenses)

\`\`\`
MIT License
...
\`\`\`
```

## Supported Licenses

The following licenses are recognized and allowed:

- MIT
- Apache-2.0
- BSD-2-Clause
- BSD-3-Clause
- ISC
- MPL-2.0
- Unlicense
- CC-BY-SA-4.0
- Python-2.0

Dependencies with licenses not in this list will cause the tool to exit with an error.

## How It Works

1. Runs `go mod download -json` to discover all dependencies
2. Recursively searches each module for license files (LICENSE, COPYING, NOTICE, etc.)
3. Uses Google's [licenseclassifier](https://github.com/google/licenseclassifier) to identify license types
4. Generates a markdown report with a manifest table and full license texts
