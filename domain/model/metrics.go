// Package model provides the built-in metric registry for Ollanta.
// Inspired by the metric dispatch map from OpenStaticAnalyzer's limmetrics,
// each MetricDef captures the key, human-readable name, type, quality domain,
// and the entity levels at which the metric is meaningful.
package model

// MetricDef describes a single metric available in Ollanta.
type MetricDef struct {
	Key  string
	Name string
	// Type is one of "int", "float", "percent", "rating".
	Type string
	// Domain groups metrics into quality categories (Size, Complexity, Coupling, etc.).
	Domain string
	// Levels lists the entity types at which this metric is computed,
	// e.g. "function", "file", "package", "project".
	Levels []string
}

var builtinMetrics = []MetricDef{
	{Key: "lines", Name: "Lines", Type: "int", Domain: "Size", Levels: []string{"file", "package", "project"}},
	{Key: "ncloc", Name: "Lines of Code", Type: "int", Domain: "Size", Levels: []string{"file", "package", "project"}},
	{Key: "files", Name: "Number of Files", Type: "int", Domain: "Size", Levels: []string{"package", "project"}},
	{Key: "functions", Name: "Number of Functions", Type: "int", Domain: "Size", Levels: []string{"file", "package"}},
	{Key: "statements", Name: "Number of Statements", Type: "int", Domain: "Size", Levels: []string{"function", "file"}},
	{Key: "complexity", Name: "Cyclomatic Complexity (McCC)", Type: "int", Domain: "Complexity", Levels: []string{"function", "file", "package"}},
	{Key: "cognitive_complexity", Name: "Cognitive Complexity", Type: "int", Domain: "Complexity", Levels: []string{"function"}},
	{Key: "nesting_level", Name: "Max Nesting Level (NL)", Type: "int", Domain: "Complexity", Levels: []string{"function"}},
	{Key: "coupling", Name: "Coupling Between Objects (CBO)", Type: "int", Domain: "Coupling", Levels: []string{"package", "file"}},
	{Key: "bugs", Name: "Bugs", Type: "int", Domain: "Reliability", Levels: []string{"project"}},
	{Key: "vulnerabilities", Name: "Vulnerabilities", Type: "int", Domain: "Security", Levels: []string{"project"}},
	{Key: "code_smells", Name: "Code Smells", Type: "int", Domain: "Maintainability", Levels: []string{"project"}},
	{Key: "coverage", Name: "Code Coverage", Type: "percent", Domain: "Coverage", Levels: []string{"file", "package", "project"}},
	{Key: MetricLinesToCover, Name: "Lines to Cover", Type: "int", Domain: "Coverage", Levels: []string{"file", "project"}},
	{Key: MetricCoveredLines, Name: "Covered Lines", Type: "int", Domain: "Coverage", Levels: []string{"file", "project"}},
	{Key: MetricUncoveredLines, Name: "Uncovered Lines", Type: "int", Domain: "Coverage", Levels: []string{"file", "project"}},
	{Key: "duplicated_lines", Name: "Duplicated Lines", Type: "int", Domain: "Duplication", Levels: []string{"file", "project"}},
	{Key: "duplicated_blocks", Name: "Duplicated Blocks", Type: "int", Domain: "Duplication", Levels: []string{"file", "project"}},
	{Key: "comment_lines", Name: "Comment Lines (CLOC)", Type: "int", Domain: "Documentation", Levels: []string{"file", "package"}},
	{Key: "comment_density", Name: "Comment Density (CD)", Type: "percent", Domain: "Documentation", Levels: []string{"file", "package"}},
	{Key: MetricTests, Name: "Tests", Type: "int", Domain: "Testability", Levels: []string{"project"}},
	{Key: MetricTestFailures, Name: "Test Failures", Type: "int", Domain: "Testability", Levels: []string{"project"}},
	{Key: MetricTestErrors, Name: "Test Errors", Type: "int", Domain: "Testability", Levels: []string{"project"}},
	{Key: MetricTestSkipped, Name: "Skipped Tests", Type: "int", Domain: "Testability", Levels: []string{"project"}},
	{Key: MetricTestDurationMs, Name: "Test Duration", Type: "int", Domain: "Testability", Levels: []string{"project"}},
	{Key: MetricMutationScore, Name: "Mutation Score", Type: "percent", Domain: "Testability", Levels: []string{"project"}},
	{Key: MetricMutantsTotal, Name: "Total Mutants", Type: "int", Domain: "Testability", Levels: []string{"project"}},
	{Key: MetricMutantsKilled, Name: "Killed Mutants", Type: "int", Domain: "Testability", Levels: []string{"project"}},
	{Key: MetricMutantsSurvived, Name: "Survived Mutants", Type: "int", Domain: "Testability", Levels: []string{"project"}},
	{Key: MetricMutantsTimeout, Name: "Timed Out Mutants", Type: "int", Domain: "Testability", Levels: []string{"project"}},
	{Key: MetricMutantsSkipped, Name: "Skipped Mutants", Type: "int", Domain: "Testability", Levels: []string{"project"}},
	{Key: MetricMutantsError, Name: "Errored Mutants", Type: "int", Domain: "Testability", Levels: []string{"project"}},
	{Key: MetricChangedMutationScore, Name: "Changed Code Mutation Score", Type: "percent", Domain: "Testability", Levels: []string{"project"}},
	{Key: MetricChangedMutantsTotal, Name: "Changed Code Mutants", Type: "int", Domain: "Testability", Levels: []string{"project"}},
	{Key: MetricChangedMutantsKilled, Name: "Changed Code Killed Mutants", Type: "int", Domain: "Testability", Levels: []string{"project"}},
	{Key: MetricChangedMutantsSurvived, Name: "Changed Code Survived Mutants", Type: "int", Domain: "Testability", Levels: []string{"project"}},
}

// AllMetrics returns a copy of all built-in MetricDefs.
func AllMetrics() []MetricDef {
	result := make([]MetricDef, len(builtinMetrics))
	copy(result, builtinMetrics)
	return result
}

// FindMetric returns a copy of the MetricDef with the given key, or nil if not found.
func FindMetric(key string) *MetricDef {
	for i := range builtinMetrics {
		if builtinMetrics[i].Key == key {
			m := builtinMetrics[i]
			return &m
		}
	}
	return nil
}
