package licenseplease

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRecursiveLicenseFinder_Find(t *testing.T) {
	t.Parallel()

	// Create temp directory structure
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"LICENSE":            "MIT License",
		"README.md":          "# Test",
		"subpkg/LICENSE":     "Apache License",
		"subpkg/code.go":     "package subpkg",
		"vendor/dep/LICENSE": "Should be skipped",
		"internal/COPYING":   "BSD License",
		"docs/NOTICE":        "Notice file",
		"COPYRIGHT":          "Copyright notice",
		"UNLICENSE":          "Public domain",
		"License.txt":        "License with extension",
		"subpkg/LICENSE.md":  "License markdown",
		"subpkg/COPYING.txt": "Copying with extension",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
	}

	finder := &RecursiveLicenseFinder{}
	module := Module{Path: "test/module", Version: "v1.0.0", Dir: tmpDir}

	paths, err := finder.Find(context.Background(), module)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}

	// Expected files (vendor should be skipped)
	expected := map[string]bool{
		"LICENSE":            true,
		"subpkg/LICENSE":     true,
		"subpkg/LICENSE.md":  true,
		"subpkg/COPYING.txt": true,
		"internal/COPYING":   true,
		"docs/NOTICE":        true,
		"COPYRIGHT":          true,
		"UNLICENSE":          true,
		"License.txt":        true,
	}

	found := make(map[string]bool)
	for _, p := range paths {
		rel, _ := filepath.Rel(tmpDir, p)
		found[rel] = true
	}

	for exp := range expected {
		if !found[exp] {
			t.Errorf("expected to find %s, but didn't", exp)
		}
	}

	// Verify vendor was skipped
	for f := range found {
		if filepath.HasPrefix(f, "vendor") {
			t.Errorf("should not find files in vendor directory, found: %s", f)
		}
	}
}

func TestRecursiveLicenseFinder_Find_EmptyDir(t *testing.T) {
	t.Parallel()

	finder := &RecursiveLicenseFinder{}
	module := Module{Path: "test/module", Version: "v1.0.0", Dir: ""}

	paths, err := finder.Find(context.Background(), module)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected empty paths for empty Dir, got %d", len(paths))
	}
}

func TestRecursiveLicenseFinder_Find_ContextCancellation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "LICENSE"), []byte("MIT"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	finder := &RecursiveLicenseFinder{}
	module := Module{Path: "test/module", Version: "v1.0.0", Dir: tmpDir}

	_, err := finder.Find(ctx, module)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestLicenseTypeFromSPDX(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		spdx     string
		wantSPDX string
	}{
		{"MIT", "MIT", "MIT"},
		{"Apache-2.0", "Apache-2.0", "Apache-2.0"},
		{"BSD-3-Clause", "BSD-3-Clause", "BSD-3-Clause"},
		{"BSD-2-Clause", "BSD-2-Clause", "BSD-2-Clause"},
		{"ISC", "ISC", "ISC"},
		{"MPL-2.0", "MPL-2.0", "MPL-2.0"},
		{"Unlicense", "Unlicense", "Unlicense"},
		{"Unknown", "Proprietary", "Proprietary"},
		{"Empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := LicenseTypeFromSPDX(tt.spdx)
			if result.SPDX() != tt.wantSPDX {
				t.Errorf("LicenseTypeFromSPDX(%q).SPDX() = %q, want %q", tt.spdx, result.SPDX(), tt.wantSPDX)
			}
		})
	}
}

func TestLicenseTypeCollectArtifacts(t *testing.T) {
	t.Parallel()

	// Create a temp dir with a NOTICE file for Apache testing
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "NOTICE"), []byte("Notice content"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		license       LicenseType
		wantArtifacts []string
	}{
		{MIT{}, []string{"LICENSE"}},
		{Apache2{}, []string{"LICENSE", "NOTICE"}}, // Apache collects NOTICE if present
		{BSD2Clause{}, []string{"LICENSE"}},
		{BSD3Clause{}, []string{"LICENSE"}},
		{ISC{}, []string{"LICENSE"}},
		{MPL2{}, []string{"LICENSE"}},
		{Unlicense{}, []string{"LICENSE"}},
	}

	for _, tt := range tests {
		t.Run(tt.license.SPDX(), func(t *testing.T) {
			t.Parallel()
			got, err := tt.license.CollectArtifacts(tmpDir, "LICENSE")
			if err != nil {
				t.Fatalf("CollectArtifacts() error = %v", err)
			}
			if len(got) != len(tt.wantArtifacts) {
				t.Errorf("%s.CollectArtifacts() = %v, want %v", tt.license.SPDX(), got, tt.wantArtifacts)
			}
		})
	}
}

// Mock implementations for Aggregator tests
type mockResolver struct {
	modules []Module
	err     error
}

func (m *mockResolver) Resolve(ctx context.Context, projectDir string) ([]Module, error) {
	return m.modules, m.err
}

type mockFinder struct {
	paths map[string][]string // module path -> license paths
	err   error
}

func (m *mockFinder) Find(ctx context.Context, module Module) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.paths[module.Path], nil
}

type mockClassifier struct {
	licenses map[string][]License // file path -> licenses
	err      error
}

func (m *mockClassifier) Classify(ctx context.Context, path string) ([]License, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.licenses[path], nil
}

func TestAggregator_Aggregate(t *testing.T) {
	t.Parallel()

	modules := []Module{
		{Path: "github.com/foo/bar", Version: "v1.0.0", Dir: "/tmp/mod/foo/bar"},
		{Path: "github.com/baz/qux", Version: "v2.0.0", Dir: "/tmp/mod/baz/qux"},
	}

	finderPaths := map[string][]string{
		"github.com/foo/bar": {"/tmp/mod/foo/bar/LICENSE"},
		"github.com/baz/qux": {"/tmp/mod/baz/qux/LICENSE", "/tmp/mod/baz/qux/third_party/COPYING"},
	}

	classifierLicenses := map[string][]License{
		"/tmp/mod/foo/bar/LICENSE":             {{Name: "MIT", Type: MIT{}}},
		"/tmp/mod/baz/qux/LICENSE":             {{Name: "Apache-2.0", Type: Apache2{}}},
		"/tmp/mod/baz/qux/third_party/COPYING": {{Name: "BSD-3-Clause", Type: BSD3Clause{}}},
	}

	aggregator := &Aggregator{
		Resolver:   &mockResolver{modules: modules},
		Finder:     &mockFinder{paths: finderPaths},
		Classifier: &mockClassifier{licenses: classifierLicenses},
	}

	result, err := aggregator.Aggregate(context.Background(), "/project")
	if err != nil {
		t.Fatalf("Aggregate() error = %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 license files, got %d", len(result))
	}

	// Verify structure
	for _, lf := range result {
		if lf.Module.Path == "" {
			t.Error("license file has empty module path")
		}
		if len(lf.Licenses) == 0 {
			t.Errorf("license file %s has no licenses", lf.Path)
		}
	}
}

func TestAggregator_Aggregate_ResolverError(t *testing.T) {
	t.Parallel()

	aggregator := &Aggregator{
		Resolver:   &mockResolver{err: os.ErrNotExist},
		Finder:     &mockFinder{},
		Classifier: &mockClassifier{},
	}

	_, err := aggregator.Aggregate(context.Background(), "/project")
	if err == nil {
		t.Error("expected error when resolver fails")
	}
}

func TestLicenseFilePattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		match    bool
	}{
		{"LICENSE", true},
		{"LICENSE.txt", true},
		{"LICENSE.md", true},
		{"license", true},
		{"License", true},
		{"LICENCE", true},
		{"COPYING", true},
		{"COPYING.txt", true},
		{"NOTICE", true},
		{"NOTICE.txt", true},
		{"COPYRIGHT", true},
		{"UNLICENSE", true},
		{"Unlicense", true},
		{"README.md", false},
		{"main.go", false},
		{"go.mod", false},
		{"licenses.go", false}, // Should not match 'licenses' plural
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()
			result := licenseFilePattern.MatchString(tt.filename)
			if result != tt.match {
				t.Errorf("licenseFilePattern.MatchString(%q) = %v, want %v", tt.filename, result, tt.match)
			}
		})
	}
}
