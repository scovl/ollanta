package ollantarules

import (
	"encoding/json"
	"fmt"
	"io/fs"
)

// LoadMeta reads all JSON files matching pattern from the given embed.FS and
// returns a map keyed by RuleMeta.Key. It returns an error if any file cannot
// be parsed or if two files share the same key.
func LoadMeta(fsys fs.FS, pattern string) (map[string]RuleMeta, error) {
	matches, err := fs.Glob(fsys, pattern)
	if err != nil {
		return nil, fmt.Errorf("LoadMeta glob %q: %w", pattern, err)
	}

	out := make(map[string]RuleMeta, len(matches))
	for _, path := range matches {
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, fmt.Errorf("LoadMeta read %q: %w", path, err)
		}
		var meta RuleMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil, fmt.Errorf("LoadMeta parse %q: %w", path, err)
		}
		if meta.Key == "" {
			return nil, fmt.Errorf("LoadMeta: %q has empty key", path)
		}
		if _, dup := out[meta.Key]; dup {
			return nil, fmt.Errorf("LoadMeta: duplicate key %q in %q", meta.Key, path)
		}
		out[meta.Key] = meta
	}
	return out, nil
}

// WithMeta returns a copy of the Rule with Meta replaced by the given RuleMeta.
func (r Rule) WithMeta(meta RuleMeta) Rule {
	r.Meta = meta
	return r
}
