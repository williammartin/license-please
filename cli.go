package licenseplease

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/alecthomas/kong"
)

// allowedLicenses are the licenses we know how to handle for compliance.
var allowedLicenses = map[string]bool{
	"MIT":          true,
	"Apache-2.0":   true,
	"BSD-2-Clause": true,
	"BSD-3-Clause": true,
	"ISC":          true,
	"MPL-2.0":      true,
	"Unlicense":    true,
	"CC-BY-SA-4.0": true, // docs only
	"Python-2.0":   true, // nested, unused
}

type CLI struct {
	Report ReportCmd `cmd:"" help:"Generate a license report for a Go project."`
}

type ReportCmd struct {
	ProjectDir string `arg:"" optional:"" default:"." help:"Path to Go project directory."`
}

func (r *ReportCmd) Run(ctx context.Context, aggregator *Aggregator) error {
	licenseFiles, err := aggregator.Aggregate(ctx, r.ProjectDir)
	if err != nil {
		return err
	}

	// Sort by module path for consistent output
	sort.Slice(licenseFiles, func(i, j int) bool {
		if licenseFiles[i].Module.Path != licenseFiles[j].Module.Path {
			return licenseFiles[i].Module.Path < licenseFiles[j].Module.Path
		}
		return licenseFiles[i].RelPath < licenseFiles[j].RelPath
	})

	// Check for disallowed licenses
	var disallowed []string
	for _, lf := range licenseFiles {
		for _, l := range lf.Licenses {
			if !allowedLicenses[l.Name] && l.Name != "" {
				disallowed = append(disallowed, fmt.Sprintf("%s@%s: %s (%s)", lf.Module.Path, lf.Module.Version, l.Name, lf.RelPath))
			}
		}
	}
	if len(disallowed) > 0 {
		return fmt.Errorf("found %d dependencies with disallowed licenses:\n  %s", len(disallowed), strings.Join(disallowed, "\n  "))
	}

	// Generate and write report to stdout
	return writeReport(os.Stdout, licenseFiles)
}

func writeReport(w io.Writer, licenseFiles []LicenseFile) error {
	// Header
	fmt.Fprintln(w, "# Third-Party Licenses")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "This file contains the licenses for all third-party dependencies.")
	fmt.Fprintln(w)

	// Manifest section
	fmt.Fprintln(w, "## Manifest")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Module | Version | License | Source |")
	fmt.Fprintln(w, "|--------|---------|---------|--------|")

	for _, lf := range licenseFiles {
		names := licenseNames(lf)
		url := lf.LicenseURL()
		fmt.Fprintf(w, "| %s | %s | %s | [%s](%s) |\n",
			lf.Module.Path, lf.Module.Version, names, lf.RelPath, url)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "---")
	fmt.Fprintln(w)

	// Full license texts
	fmt.Fprintln(w, "## License Texts")
	fmt.Fprintln(w)

	for _, lf := range licenseFiles {
		names := licenseNames(lf)

		fmt.Fprintf(w, "### %s %s\n\n", lf.Module.Path, lf.Module.Version)
		fmt.Fprintf(w, "**License:** %s\n\n", names)
		fmt.Fprintf(w, "**Source:** [%s](%s)\n\n", lf.RelPath, lf.LicenseURL())

		content, err := os.ReadFile(lf.Path)
		if err != nil {
			return fmt.Errorf("reading license file %s: %w", lf.Path, err)
		}

		fmt.Fprintln(w, "```")
		w.Write(content)
		if len(content) > 0 && content[len(content)-1] != '\n' {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w)
	}

	return nil
}

func licenseNames(lf LicenseFile) string {
	names := make([]string, len(lf.Licenses))
	for i, l := range lf.Licenses {
		names[i] = l.Name
	}
	if len(names) == 0 {
		return "Unknown"
	}
	return strings.Join(names, ", ")
}

func Run(args []string) {
	classifier, err := NewGoogleLicenseClassifier()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	aggregator := &Aggregator{
		Resolver:   &GoModResolver{},
		Finder:     &RecursiveLicenseFinder{},
		Classifier: classifier,
	}

	cli := &CLI{}
	kctx := kong.Parse(cli,
		kong.Name("license-please"),
		kong.Description("A tool to help with Go OSS license compliance."),
		kong.BindTo(context.Background(), (*context.Context)(nil)),
		kong.BindTo(aggregator, (*Aggregator)(nil)),
	)

	err = kctx.Run(context.Background(), aggregator)
	kctx.FatalIfErrorf(err)
}
