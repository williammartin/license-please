package licenseplease_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/williammartin/licenseplease"
)

// TestE2E_KnownDependencies tests against a real Go module with known dependencies.
// This test requires the e2e testdata module to have its dependencies downloaded.
func TestE2E_KnownDependencies(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	// Get the path to the e2e test module
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to get current file path")
	}
	e2eDir := filepath.Join(filepath.Dir(thisFile), "testdata", "e2e")

	// Verify the e2e module exists
	if _, err := os.Stat(filepath.Join(e2eDir, "go.mod")); err != nil {
		t.Fatalf("e2e test module not found at %s: %v", e2eDir, err)
	}

	// Create the aggregator with real implementations
	classifier, err := licenseplease.NewGoogleLicenseClassifier()
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	aggregator := &licenseplease.Aggregator{
		Resolver:   &licenseplease.GoModResolver{},
		Finder:     &licenseplease.RecursiveLicenseFinder{},
		Classifier: classifier,
	}

	// Run aggregation
	ctx := context.Background()
	licenseFiles, err := aggregator.Aggregate(ctx, e2eDir)
	if err != nil {
		t.Fatalf("Aggregate() error = %v", err)
	}

	// Build a map of module -> licenses for easier assertions
	moduleToLicenses := make(map[string][]string)
	for _, lf := range licenseFiles {
		key := lf.Module.Path
		for _, lic := range lf.Licenses {
			moduleToLicenses[key] = append(moduleToLicenses[key], lic.Name)
		}
	}

	// Expected dependencies and their licenses
	// These are the known dependencies from the e2e test module
	expectedLicenses := map[string][]string{
		"github.com/spf13/cobra":        {"Apache-2.0"},
		"github.com/spf13/pflag":        {"BSD-3-Clause"},
		"github.com/stretchr/testify":   {"MIT"},
		"github.com/davecgh/go-spew":    {"ISC"},
		"github.com/pmezard/go-difflib": {"BSD-3-Clause"},
	}

	// Verify expected modules are present with correct licenses
	for module, expectedLics := range expectedLicenses {
		foundLics, ok := moduleToLicenses[module]
		if !ok {
			t.Errorf("expected module %s not found in results", module)
			continue
		}

		for _, expLic := range expectedLics {
			found := false
			if slices.Contains(foundLics, expLic) {
				found = true
				break
			}
			if !found {
				t.Errorf("module %s: expected license %s, got %v", module, expLic, foundLics)
			}
		}
	}

	// Log all found modules for debugging
	t.Logf("Found %d license files across modules:", len(licenseFiles))
	modules := make([]string, 0, len(moduleToLicenses))
	for m := range moduleToLicenses {
		modules = append(modules, m)
	}
	sort.Strings(modules)
	for _, m := range modules {
		t.Logf("  %s: %v", m, moduleToLicenses[m])
	}
}

// TestE2E_NestedLicenses verifies that nested license files are discovered.
func TestE2E_NestedLicenses(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to get current file path")
	}
	e2eDir := filepath.Join(filepath.Dir(thisFile), "testdata", "e2e")

	classifier, err := licenseplease.NewGoogleLicenseClassifier()
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	aggregator := &licenseplease.Aggregator{
		Resolver:   &licenseplease.GoModResolver{},
		Finder:     &licenseplease.RecursiveLicenseFinder{},
		Classifier: classifier,
	}

	licenseFiles, err := aggregator.Aggregate(context.Background(), e2eDir)
	if err != nil {
		t.Fatalf("Aggregate() error = %v", err)
	}

	// Check if any modules have multiple license files (nested licenses)
	moduleToFiles := make(map[string][]string)
	for _, lf := range licenseFiles {
		moduleToFiles[lf.Module.Path] = append(moduleToFiles[lf.Module.Path], lf.RelPath)
	}

	// Log modules with multiple license files
	for module, files := range moduleToFiles {
		if len(files) > 1 {
			t.Logf("Module %s has %d license files: %v", module, len(files), files)
		}
	}

	// Ensure we found at least some licenses
	if len(licenseFiles) < 3 {
		t.Errorf("expected at least 3 license files, got %d", len(licenseFiles))
	}
}

// TestE2E_LicenseTypes verifies that license type classification works correctly.
func TestE2E_LicenseTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to get current file path")
	}
	e2eDir := filepath.Join(filepath.Dir(thisFile), "testdata", "e2e")

	classifier, err := licenseplease.NewGoogleLicenseClassifier()
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	aggregator := &licenseplease.Aggregator{
		Resolver:   &licenseplease.GoModResolver{},
		Finder:     &licenseplease.RecursiveLicenseFinder{},
		Classifier: classifier,
	}

	licenseFiles, err := aggregator.Aggregate(context.Background(), e2eDir)
	if err != nil {
		t.Fatalf("Aggregate() error = %v", err)
	}

	// Collect license types by SPDX
	spdxCount := make(map[string]int)
	for _, lf := range licenseFiles {
		for _, lic := range lf.Licenses {
			spdxCount[lic.Type.SPDX()]++
		}
	}

	t.Logf("License types found: %v", spdxCount)

	// All licenses in our e2e deps should be known permissive licenses
	permissive := []string{"MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC"}
	foundPermissive := false
	for _, p := range permissive {
		if spdxCount[p] > 0 {
			foundPermissive = true
			break
		}
	}
	if !foundPermissive {
		t.Error("expected to find permissive licenses")
	}

	// Should not have any copyleft (GPL/AGPL) in our test deps
	copyleft := []string{"GPL-2.0", "GPL-3.0", "AGPL-3.0", "LGPL-2.1", "LGPL-3.0"}
	for _, c := range copyleft {
		if spdxCount[c] > 0 {
			t.Errorf("unexpected copyleft license found: %s", c)
		}
	}
}

// TestE2E_ModuleVersions verifies that module versions are captured correctly.
func TestE2E_ModuleVersions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to get current file path")
	}
	e2eDir := filepath.Join(filepath.Dir(thisFile), "testdata", "e2e")

	classifier, err := licenseplease.NewGoogleLicenseClassifier()
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	aggregator := &licenseplease.Aggregator{
		Resolver:   &licenseplease.GoModResolver{},
		Finder:     &licenseplease.RecursiveLicenseFinder{},
		Classifier: classifier,
	}

	licenseFiles, err := aggregator.Aggregate(context.Background(), e2eDir)
	if err != nil {
		t.Fatalf("Aggregate() error = %v", err)
	}

	// Check that versions are present and look valid
	for _, lf := range licenseFiles {
		if lf.Module.Version == "" {
			t.Errorf("module %s has empty version", lf.Module.Path)
		}
		if !strings.HasPrefix(lf.Module.Version, "v") {
			t.Errorf("module %s has invalid version format: %s", lf.Module.Path, lf.Module.Version)
		}
	}
}
