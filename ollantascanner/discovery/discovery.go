// Package discovery finds source files under configured directories and
// determines each file's language from its extension using constants.ExtensionToLanguage.
package discovery

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/scovl/ollanta/ollantacore/constants"
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

// Discover walks sourceDirs under baseDir and returns all source files whose
// extension is recognised by constants.ExtensionToLanguage.
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
		// Strip Go-style recursive suffix
		root = strings.TrimSuffix(root, "/...")
		root = strings.TrimSuffix(root, `\...`)

		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}

		err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable entries
			}
			if d.IsDir() {
				if defaultExcludedDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}

			lang, ok := constants.ExtensionToLanguage[strings.ToLower(filepath.Ext(path))]
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
			results = append(results, DiscoveredFile{Path: abs, Language: lang})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

// matchesAny reports whether rel matches any of the glob patterns.
// Each pattern is tried against the full relative path and against the base name.
func matchesAny(rel string, patterns []string) bool {
	for _, pat := range patterns {
		if pat == "" {
			continue
		}
		if m, _ := filepath.Match(pat, rel); m {
			return true
		}
		if m, _ := filepath.Match(pat, filepath.Base(rel)); m {
			return true
		}
	}
	return false
}
