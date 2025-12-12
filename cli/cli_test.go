package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/williammartin/licenseplease"
	"github.com/williammartin/licenseplease/cli"
)

func TestWriteReport_Format(t *testing.T) {
	// Create temp file for license content
	tmpDir := t.TempDir()
	licensePath := filepath.Join(tmpDir, "LICENSE")
	if err := os.WriteFile(licensePath, []byte("MIT License\n\nCopyright (c) 2024"), 0644); err != nil {
		t.Fatal(err)
	}

	result := &licenseplease.Result{
		LicenseFiles: []licenseplease.LicenseFile{
			{
				Path:    licensePath,
				RelPath: "LICENSE",
				Module: licenseplease.Module{
					Path:    "github.com/test/module",
					Version: "v1.0.0",
					Dir:     tmpDir,
				},
				Licenses: []licenseplease.License{
					{Name: "MIT", Type: licenseplease.MIT{}},
				},
			},
		},
	}

	var buf bytes.Buffer
	err := cli.WriteReport(&buf, result)
	if err != nil {
		t.Fatalf("WriteReport() error = %v", err)
	}

	output := buf.String()

	// Verify markdown structure
	expectedSections := []string{
		"# Third-Party Licenses",
		"## Manifest",
		"| Module | Version | License | Source |",
		"## License Texts",
		"### github.com/test/module v1.0.0",
		"**License:** MIT",
		"**Source:**",
		"```",
		"MIT License", // License content should be included
	}

	for _, section := range expectedSections {
		if !strings.Contains(output, section) {
			t.Errorf("output missing expected section: %q", section)
		}
	}

	// Verify manifest row format
	if !strings.Contains(output, "| github.com/test/module | v1.0.0 | MIT |") {
		t.Error("output missing properly formatted manifest row")
	}
}

func TestWriteReport_MultipleModules(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create license files
	aDir := filepath.Join(tmpDir, "a")
	bDir := filepath.Join(tmpDir, "b")
	os.MkdirAll(aDir, 0755)
	os.MkdirAll(bDir, 0755)
	os.WriteFile(filepath.Join(aDir, "LICENSE"), []byte("MIT License"), 0644)
	os.WriteFile(filepath.Join(bDir, "LICENSE"), []byte("Apache License 2.0"), 0644)

	result := &licenseplease.Result{
		LicenseFiles: []licenseplease.LicenseFile{
			{
				Path:    filepath.Join(aDir, "LICENSE"),
				RelPath: "LICENSE",
				Module: licenseplease.Module{
					Path:    "github.com/aaa/first",
					Version: "v1.0.0",
					Dir:     aDir,
				},
				Licenses: []licenseplease.License{
					{Name: "MIT", Type: licenseplease.MIT{}},
				},
			},
			{
				Path:    filepath.Join(bDir, "LICENSE"),
				RelPath: "LICENSE",
				Module: licenseplease.Module{
					Path:    "github.com/bbb/second",
					Version: "v2.0.0",
					Dir:     bDir,
				},
				Licenses: []licenseplease.License{
					{Name: "Apache-2.0", Type: licenseplease.Apache2{}},
				},
			},
		},
	}

	var buf bytes.Buffer
	err := cli.WriteReport(&buf, result)
	if err != nil {
		t.Fatalf("WriteReport() error = %v", err)
	}

	output := buf.String()

	// Both modules should appear
	if !strings.Contains(output, "github.com/aaa/first") {
		t.Error("output missing first module")
	}
	if !strings.Contains(output, "github.com/bbb/second") {
		t.Error("output missing second module")
	}

	// Both license types should appear
	if !strings.Contains(output, "MIT") {
		t.Error("output missing MIT license")
	}
	if !strings.Contains(output, "Apache-2.0") {
		t.Error("output missing Apache-2.0 license")
	}
}

func TestWriteReport_NoticeFile(t *testing.T) {
	tmpDir := t.TempDir()
	noticePath := filepath.Join(tmpDir, "NOTICE")
	os.WriteFile(noticePath, []byte("Copyright notice content"), 0644)

	result := &licenseplease.Result{
		LicenseFiles: []licenseplease.LicenseFile{
			{
				Path:    noticePath,
				RelPath: "NOTICE",
				Module: licenseplease.Module{
					Path:    "github.com/test/module",
					Version: "v1.0.0",
					Dir:     tmpDir,
				},
				Licenses: []licenseplease.License{}, // No recognized license
			},
		},
	}

	var buf bytes.Buffer
	err := cli.WriteReport(&buf, result)
	if err != nil {
		t.Fatalf("WriteReport() error = %v", err)
	}

	output := buf.String()

	// NOTICE files should be labeled appropriately
	if !strings.Contains(output, "(NOTICE file)") {
		t.Error("output should label NOTICE files as '(NOTICE file)'")
	}
}

func TestE2E_CLIReport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to get current file path")
	}
	e2eDir := filepath.Join(filepath.Dir(thisFile), "..", "testdata", "e2e")

	result, err := licenseplease.Run(context.Background(), e2eDir)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var buf bytes.Buffer
	err = cli.WriteReport(&buf, result)
	if err != nil {
		t.Fatalf("WriteReport() error = %v", err)
	}

	output := buf.String()

	// Verify real output has expected structure
	if !strings.Contains(output, "# Third-Party Licenses") {
		t.Error("missing title")
	}
	if !strings.Contains(output, "## Manifest") {
		t.Error("missing manifest section")
	}
	if !strings.Contains(output, "github.com/spf13/cobra") {
		t.Error("missing expected dependency: cobra")
	}
	if !strings.Contains(output, "Apache-2.0") {
		t.Error("missing expected license type: Apache-2.0")
	}
	if !strings.Contains(output, "pkg.go.dev") {
		t.Error("missing pkg.go.dev links")
	}

	t.Logf("Report output length: %d bytes", len(output))
}
