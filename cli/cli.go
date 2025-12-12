package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/williammartin/licenseplease"
)

type CLI struct {
	Report ReportCmd `cmd:"" help:"Generate a license report for a Go project."`
}

type ReportCmd struct {
	ProjectDir string `arg:"" optional:"" default:"." help:"Path to Go project directory."`
}

func (r *ReportCmd) Run(ctx context.Context) error {
	result, err := licenseplease.Run(ctx, r.ProjectDir)
	if err != nil {
		return err
	}

	return WriteReport(os.Stdout, result)
}

// WriteReport writes the license report in markdown format to the given writer.
func WriteReport(w io.Writer, result *licenseplease.Result) error {
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

	for _, lf := range result.LicenseFiles {
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

	for _, lf := range result.LicenseFiles {
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

func licenseNames(lf licenseplease.LicenseFile) string {
	if len(lf.Licenses) == 0 {
		// For NOTICE/COPYRIGHT files that aren't licenses, use the filename
		base := strings.ToUpper(strings.TrimSuffix(lf.RelPath, filepath.Ext(lf.RelPath)))
		if strings.Contains(base, "NOTICE") || strings.Contains(base, "COPYRIGHT") {
			return "(NOTICE file)"
		}
		return "Unknown"
	}
	names := make([]string, len(lf.Licenses))
	for i, l := range lf.Licenses {
		names[i] = l.Type.SPDX()
	}
	return strings.Join(names, ", ")
}

func Execute() {
	cli := &CLI{}
	kctx := kong.Parse(cli,
		kong.Name("license-please"),
		kong.Description("A tool to help with Go OSS license compliance."),
		kong.BindTo(context.Background(), (*context.Context)(nil)),
	)

	err := kctx.Run(context.Background())
	kctx.FatalIfErrorf(err)
}
