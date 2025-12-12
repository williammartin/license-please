package licenseplease

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	classifier "github.com/google/licenseclassifier/v2"
	"github.com/google/licenseclassifier/v2/assets"
)

// LicenseType represents a specific license with its compliance requirements.
type LicenseType interface {
	// SPDX returns the SPDX identifier for this license.
	SPDX() string

	// CollectArtifacts returns the files that must be included in a distribution
	// to comply with this license. The moduleDir is the root of the module.
	// Returns paths relative to moduleDir.
	CollectArtifacts(moduleDir string, licenseRelPath string) ([]string, error)
}

// MIT is the MIT License.
// Requirements: Include copyright notice and license text in all copies.
type MIT struct{}

func (MIT) SPDX() string { return "MIT" }
func (MIT) CollectArtifacts(moduleDir string, licenseRelPath string) ([]string, error) {
	// Just need the license file itself
	return []string{licenseRelPath}, nil
}

// Apache2 is the Apache License 2.0.
// Requirements: Include copyright notice, license text, and NOTICE file (if present).
// State changes if modified.
type Apache2 struct{}

func (Apache2) SPDX() string { return "Apache-2.0" }
func (Apache2) CollectArtifacts(moduleDir string, licenseRelPath string) ([]string, error) {
	artifacts := []string{licenseRelPath}
	// Apache-2.0 requires including NOTICE file if present
	noticePatterns := []string{"NOTICE", "NOTICE.txt", "NOTICE.md"}
	for _, pattern := range noticePatterns {
		noticePath := filepath.Join(moduleDir, pattern)
		if _, err := os.Stat(noticePath); err == nil {
			artifacts = append(artifacts, pattern)
		}
	}
	return artifacts, nil
}

// BSD2Clause is the 2-Clause BSD License (Simplified BSD).
// Requirements: Include copyright notice and license text in all copies.
type BSD2Clause struct{}

func (BSD2Clause) SPDX() string { return "BSD-2-Clause" }
func (BSD2Clause) CollectArtifacts(moduleDir string, licenseRelPath string) ([]string, error) {
	return []string{licenseRelPath}, nil
}

// BSD3Clause is the 3-Clause BSD License (New BSD).
// Requirements: Include copyright notice and license text. Don't use author names for endorsement.
type BSD3Clause struct{}

func (BSD3Clause) SPDX() string { return "BSD-3-Clause" }
func (BSD3Clause) CollectArtifacts(moduleDir string, licenseRelPath string) ([]string, error) {
	return []string{licenseRelPath}, nil
}

// ISC is the ISC License.
// Requirements: Include copyright notice and license text in all copies.
// Functionally equivalent to MIT.
type ISC struct{}

func (ISC) SPDX() string { return "ISC" }
func (ISC) CollectArtifacts(moduleDir string, licenseRelPath string) ([]string, error) {
	return []string{licenseRelPath}, nil
}

// MPL2 is the Mozilla Public License 2.0.
// Requirements: Include license text. If you modify MPL-licensed files,
// you must make the source of those specific files available.
// This is "file-level" copyleft - your own code is not affected.
// NOTE: We assume dependencies are unmodified, so we only collect the license.
// If you modify MPL files, you must provide source separately.
type MPL2 struct{}

func (MPL2) SPDX() string { return "MPL-2.0" }
func (MPL2) CollectArtifacts(moduleDir string, licenseRelPath string) ([]string, error) {
	// Only license needed for unmodified dependencies
	// Modified files would require source, but we assume unmodified
	return []string{licenseRelPath}, nil
}

// Unlicense is a public domain dedication.
// Requirements: None. The author has waived all rights.
// We still include it for attribution even though not required.
type Unlicense struct{}

func (Unlicense) SPDX() string { return "Unlicense" }
func (Unlicense) CollectArtifacts(moduleDir string, licenseRelPath string) ([]string, error) {
	// Not required, but good practice to include
	return []string{licenseRelPath}, nil
}

// CCBYSA4 is the Creative Commons Attribution-ShareAlike 4.0 license.
// This is typically used for documentation, not code.
// Requirements: Attribution required. ShareAlike if you distribute derivatives.
type CCBYSA4 struct{}

func (CCBYSA4) SPDX() string { return "CC-BY-SA-4.0" }
func (CCBYSA4) CollectArtifacts(moduleDir string, licenseRelPath string) ([]string, error) {
	return []string{licenseRelPath}, nil
}

// Python2 is the Python Software Foundation License 2.0.
// Requirements: Include copyright notice and license text.
type Python2 struct{}

func (Python2) SPDX() string { return "Python-2.0" }
func (Python2) CollectArtifacts(moduleDir string, licenseRelPath string) ([]string, error) {
	return []string{licenseRelPath}, nil
}

// UnknownLicense represents a license that could not be classified.
// We still include the file content for safety.
type UnknownLicense struct {
	name string
}

func (u UnknownLicense) SPDX() string { return u.name }
func (UnknownLicense) CollectArtifacts(moduleDir string, licenseRelPath string) ([]string, error) {
	// Safe default: include whatever file we found
	return []string{licenseRelPath}, nil
}

// NoticeFile represents a NOTICE or COPYRIGHT file, not a license.
// These are included because Apache-2.0 requires them and they contain
// important attribution information.
type NoticeFile struct{}

func (NoticeFile) SPDX() string { return "(NOTICE)" }
func (NoticeFile) CollectArtifacts(moduleDir string, licenseRelPath string) ([]string, error) {
	return []string{licenseRelPath}, nil
}

// knownLicenses maps SPDX identifiers to their LicenseType.
var knownLicenses = map[string]LicenseType{
	"MIT":          MIT{},
	"Apache-2.0":   Apache2{},
	"BSD-2-Clause": BSD2Clause{},
	"BSD-3-Clause": BSD3Clause{},
	"ISC":          ISC{},
	"MPL-2.0":      MPL2{},
	"Unlicense":    Unlicense{},
	"CC-BY-SA-4.0": CCBYSA4{},
	"Python-2.0":   Python2{},
}

// LicenseTypeFromSPDX returns the LicenseType for a given SPDX identifier.
func LicenseTypeFromSPDX(spdx string) LicenseType {
	if lt, ok := knownLicenses[spdx]; ok {
		return lt
	}
	return UnknownLicense{name: spdx}
}

// AllowedLicenses returns the set of license SPDX identifiers we accept.
func AllowedLicenses() map[string]bool {
	allowed := make(map[string]bool)
	for spdx := range knownLicenses {
		allowed[spdx] = true
	}
	return allowed
}

// Module represents a Go module dependency.
type Module struct {
	Path    string
	Version string
	Dir     string
}

// License represents a classified license.
type License struct {
	Name string      // SPDX identifier
	Type LicenseType // The typed license with compliance requirements
}

// LicenseFile represents a discovered license file.
type LicenseFile struct {
	Path     string
	RelPath  string
	Module   Module
	Licenses []License
}

// ModuleResolver lists all modules from a Go project.
type ModuleResolver interface {
	Resolve(ctx context.Context, projectDir string) ([]Module, error)
}

// LicenseFinder discovers license files within a module.
type LicenseFinder interface {
	Find(ctx context.Context, module Module) ([]string, error)
}

// LicenseClassifier identifies license types from file content.
type LicenseClassifier interface {
	Classify(ctx context.Context, path string) ([]License, error)
}

// GoModResolver implements ModuleResolver using go mod download.
type GoModResolver struct{}

func (r *GoModResolver) Resolve(ctx context.Context, projectDir string) ([]Module, error) {
	cmd := exec.CommandContext(ctx, "go", "mod", "download", "-json")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go mod download: %w", err)
	}

	var modules []Module
	// Parse JSON stream (one object per module)
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	for decoder.More() {
		var m struct {
			Path    string `json:"Path"`
			Version string `json:"Version"`
			Dir     string `json:"Dir"`
		}
		if err := decoder.Decode(&m); err != nil {
			return nil, fmt.Errorf("parsing module JSON: %w", err)
		}
		modules = append(modules, Module{
			Path:    m.Path,
			Version: m.Version,
			Dir:     m.Dir,
		})
	}
	return modules, nil
}

// RecursiveLicenseFinder implements LicenseFinder by walking module directories.
type RecursiveLicenseFinder struct{}

var licenseFilePattern = regexp.MustCompile(`(?i)^((UN)?LICEN[SC]E|COPYING|NOTICE|COPYRIGHT)(\.[a-z]+)?$`)

func (f *RecursiveLicenseFinder) Find(ctx context.Context, module Module) ([]string, error) {
	if module.Dir == "" {
		return nil, nil
	}

	var paths []string
	err := filepath.WalkDir(module.Dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			// Skip vendor directories
			if d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if licenseFilePattern.MatchString(d.Name()) {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking module %s: %w", module.Path, err)
	}
	return paths, nil
}

// GoogleLicenseClassifier implements LicenseClassifier using Google's licenseclassifier.
type GoogleLicenseClassifier struct {
	c *classifier.Classifier
}

func NewGoogleLicenseClassifier() (*GoogleLicenseClassifier, error) {
	c, err := assets.DefaultClassifier()
	if err != nil {
		return nil, fmt.Errorf("creating classifier: %w", err)
	}
	return &GoogleLicenseClassifier{c: c}, nil
}

func (g *GoogleLicenseClassifier) Classify(ctx context.Context, path string) ([]License, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading license file: %w", err)
	}

	results := g.c.Match(content)
	seen := make(map[string]bool)
	var licenses []License
	for _, match := range results.Matches {
		if match.MatchType != "License" {
			continue
		}
		if seen[match.Name] {
			continue
		}
		seen[match.Name] = true
		licenses = append(licenses, License{
			Name: match.Name,
			Type: LicenseTypeFromSPDX(match.Name),
		})
	}
	return licenses, nil
}

// Aggregator combines all components to produce a complete license report.
type Aggregator struct {
	Resolver   ModuleResolver
	Finder     LicenseFinder
	Classifier LicenseClassifier
}

func (a *Aggregator) Aggregate(ctx context.Context, projectDir string) ([]LicenseFile, error) {
	modules, err := a.Resolver.Resolve(ctx, projectDir)
	if err != nil {
		return nil, fmt.Errorf("resolving modules: %w", err)
	}

	var result []LicenseFile
	for _, mod := range modules {
		paths, err := a.Finder.Find(ctx, mod)
		if err != nil {
			return nil, fmt.Errorf("finding licenses in %s: %w", mod.Path, err)
		}

		for _, path := range paths {
			licenses, err := a.Classifier.Classify(ctx, path)
			if err != nil {
				return nil, fmt.Errorf("classifying %s: %w", path, err)
			}

			relPath, _ := filepath.Rel(mod.Dir, path)
			result = append(result, LicenseFile{
				Path:     path,
				RelPath:  relPath,
				Module:   mod,
				Licenses: licenses,
			})
		}
	}
	return result, nil
}

// LicenseURL returns a URL to view the license on pkg.go.dev.
func (lf *LicenseFile) LicenseURL() string {
	return fmt.Sprintf("https://pkg.go.dev/%s@%s?tab=licenses", lf.Module.Path, lf.Module.Version)
}
