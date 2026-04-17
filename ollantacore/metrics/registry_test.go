package metrics_test

import (
	"testing"

	"github.com/scovl/ollanta/ollantacore/metrics"
)

func TestAll_Returns17Metrics(t *testing.T) {
	all := metrics.All()
	if len(all) != 17 {
		t.Errorf("expected 17 metrics, got %d", len(all))
	}
}

func TestAll_ReturnsCopy(t *testing.T) {
	all1 := metrics.All()
	all2 := metrics.All()
	all1[0].Key = "modified"
	if all2[0].Key == "modified" {
		t.Error("All() should return a copy, not share the backing slice")
	}
}

func TestFind_ExistingMetrics(t *testing.T) {
	cases := []struct {
		key    string
		name   string
		domain string
	}{
		{"lines", "Lines", "Size"},
		{"ncloc", "Lines of Code", "Size"},
		{"complexity", "Cyclomatic Complexity (McCC)", "Complexity"},
		{"bugs", "Bugs", "Reliability"},
		{"vulnerabilities", "Vulnerabilities", "Security"},
		{"coverage", "Code Coverage", "Coverage"},
	}
	for _, tc := range cases {
		m := metrics.Find(tc.key)
		if m == nil {
			t.Errorf("Find(%q): expected non-nil", tc.key)
			continue
		}
		if m.Name != tc.name {
			t.Errorf("Find(%q).Name: got %q, want %q", tc.key, m.Name, tc.name)
		}
		if m.Domain != tc.domain {
			t.Errorf("Find(%q).Domain: got %q, want %q", tc.key, m.Domain, tc.domain)
		}
	}
}

func TestFind_NonexistentMetric(t *testing.T) {
	if metrics.Find("nonexistent_metric_xyz") != nil {
		t.Error("expected nil for nonexistent metric")
	}
}

func TestFind_AllKeysPresent(t *testing.T) {
	keys := []string{
		"lines", "ncloc", "files", "functions", "statements",
		"complexity", "cognitive_complexity", "nesting_level", "coupling",
		"bugs", "vulnerabilities", "code_smells", "coverage",
		"duplicated_lines", "duplicated_blocks", "comment_lines", "comment_density",
	}
	for _, key := range keys {
		if metrics.Find(key) == nil {
			t.Errorf("Find(%q): expected non-nil", key)
		}
	}
}

func TestMetricDef_HasRequiredFields(t *testing.T) {
	for _, m := range metrics.All() {
		if m.Key == "" {
			t.Errorf("metric has empty Key")
		}
		if m.Name == "" {
			t.Errorf("metric %q has empty Name", m.Key)
		}
		if m.Domain == "" {
			t.Errorf("metric %q has empty Domain", m.Key)
		}
		if len(m.Levels) == 0 {
			t.Errorf("metric %q has no Levels", m.Key)
		}
	}
}

func TestFind_ReturnsCopy(t *testing.T) {
	m := metrics.Find("bugs")
	if m == nil {
		t.Fatal("expected non-nil for 'bugs'")
	}
	m.Key = "tampered"
	m2 := metrics.Find("bugs")
	if m2 == nil || m2.Key != "bugs" {
		t.Error("Find() should return a copy; mutating one must not affect the global")
	}
}
