// Package scan finds source files under configured directories and
// determines each file's language from its extension.
package scan

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/scovl/ollanta/domain/model"
)

// defaultExcludedDirs lists directory names that are never descended into.
var defaultExcludedDirs = map[string]bool{
	"vendor":       true,
	".git":         true,
	"testdata":     true,
	"node_modules": true,
	"_build":       true,
	".ollanta":     true,
}

// DiscoveredFile is a source file whose language has been detected by extension.
type DiscoveredFile struct {
	Path     string // absolute path
	Language string // canonical language identifier (e.g. "go", "javascript")
}

// makeWalkFunc returns a WalkDir callback that appends discovered source files to results.
func makeWalkFunc(baseDir string, exclusions []string, seen map[string]bool, results *[]DiscoveredFile) func(string, os.DirEntry, error) error {
	return func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if defaultExcludedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		lang, ok := model.ExtensionToLanguage[strings.ToLower(filepath.Ext(path))]
		if !ok {
			return nil
		}

		abs, err := filepath.Abs(path)
		if err != nil || seen[abs] {
			return nil
		}

		rel, _ := filepath.Rel(baseDir, abs)
		if matchesAny(rel, exclusions) {
			return nil
		}

		seen[abs] = true
		*results = append(*results, DiscoveredFile{Path: abs, Language: lang})
		return nil
	}
}

// discoverDir walks a single root directory and appends discovered files to results.
func discoverDir(root, baseDir string, exclusions []string, seen map[string]bool, results *[]DiscoveredFile) error {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil
	}
	return filepath.WalkDir(root, makeWalkFunc(baseDir, exclusions, seen, results))
}

// Discover walks sourceDirs under baseDir and returns all source files whose
// extension is recognised by model.ExtensionToLanguage.
//
// sourceDirs may use Go-style recursive patterns (e.g. "./..."); the trailing
// "/..." is stripped before walking. Non-existent directories are silently
// skipped. exclusions is a list of glob patterns matched against the path
// relative to baseDir; matching files are omitted from the result.
func Discover(baseDir string, sourceDirs []string, exclusions []string) ([]DiscoveredFile, error) {
	if len(sourceDirs) == 0 {
		sourceDirs = []string{"."}
	}

	seen := map[string]bool{}
	var results []DiscoveredFile

	for _, dir := range sourceDirs {
		root := dir
		if !filepath.IsAbs(dir) {
			root = filepath.Join(baseDir, dir)
		}
		root = strings.TrimSuffix(root, "/...")
		root = strings.TrimSuffix(root, `\...`)

		if err := discoverDir(root, baseDir, exclusions, seen, &results); err != nil {
			return nil, err
		}
	}
	return results, nil
}

// matchesAny returns true if path matches any of the glob patterns.
func matchesAny(path string, patterns []string) bool {
	for _, p := range patterns {
		if ok, _ := filepath.Match(p, path); ok {
			return true
		}
		// Also try matching against the base name alone.
		if ok, _ := filepath.Match(p, filepath.Base(path)); ok {
			return true
		}
	}
	return false
}
