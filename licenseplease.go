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

// Module represents a Go module dependency.
type Module struct {
	Path    string
	Version string
	Dir     string
}

// License represents a classified license.
type License struct {
	Name string // SPDX identifier
	Type string // permissive, restricted, etc.
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
			Type: classifyType(match.Name),
		})
	}
	return licenses, nil
}

func classifyType(name string) string {
	// Simplified classification based on common licenses
	switch {
	case strings.Contains(name, "MIT"),
		strings.Contains(name, "Apache"),
		strings.Contains(name, "BSD"),
		strings.Contains(name, "ISC"):
		return "permissive"
	case strings.Contains(name, "GPL"),
		strings.Contains(name, "LGPL"),
		strings.Contains(name, "AGPL"):
		return "copyleft"
	case strings.Contains(name, "MPL"):
		return "weak-copyleft"
	default:
		return "unknown"
	}
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
