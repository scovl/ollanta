// Package configfile locates and decodes shared Ollanta TOML configuration files.
package configfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// DefaultName is the default Ollanta runtime configuration file name.
const DefaultName = "config.toml"

// Load resolves the configuration file path and decodes it into target.
// If no explicit path is provided, config.toml is loaded from the current
// working directory when present. A missing default config file is not an error.
func Load(explicitPath string, target any) (string, bool, error) {
	path, found, err := ResolvePath(explicitPath)
	if err != nil || !found {
		return path, found, err
	}
	if _, err := toml.DecodeFile(path, target); err != nil {
		return "", false, fmt.Errorf("decode %s: %w", path, err)
	}
	return path, true, nil
}

// ResolvePath returns the absolute path to the configuration file.
// Missing default config.toml returns found=false and err=nil.
// Missing explicit paths return an error.
func ResolvePath(explicitPath string) (string, bool, error) {
	if explicitPath != "" {
		path, err := filepath.Abs(explicitPath)
		if err != nil {
			return "", false, fmt.Errorf("resolve config path: %w", err)
		}
		if _, err := os.Stat(path); err != nil {
			return "", false, fmt.Errorf("stat %s: %w", path, err)
		}
		return path, true, nil
	}

	path, err := filepath.Abs(DefaultName)
	if err != nil {
		return "", false, fmt.Errorf("resolve default config path: %w", err)
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("stat %s: %w", path, err)
	}
	return path, true, nil
}
