// Package model defines the shared vocabulary for Ollanta: canonical language
// identifiers, metric keys, and the file-extension-to-language mapping.
// Inspired by GraphConstants.h from OpenStaticAnalyzer.
package model

// Language identifiers — canonical values used in Component.Language and
// Executor routing to determine which sensor analyzes a file.
const (
	LangGo         = "go"
	LangJavaScript = "javascript"
	LangTypeScript = "typescript"
	LangPython     = "python"
	LangRust       = "rust"
	LangUnknown    = "unknown"
)

// ExtensionToLanguage maps source file extensions to canonical language identifiers.
// Used by Discovery to populate DiscoveredFile.Language.
var ExtensionToLanguage = map[string]string{
	".go":  LangGo,
	".js":  LangJavaScript,
	".mjs": LangJavaScript,
	".ts":  LangTypeScript,
	".tsx": LangTypeScript,
	".py":  LangPython,
	".rs":  LangRust,
}

// Metric key constants — canonical string identifiers matching the MetricDef entries.
const (
	MetricLines                  = "lines"
	MetricNcloc                  = "ncloc"
	MetricFiles                  = "files"
	MetricFunctions              = "functions"
	MetricStatements             = "statements"
	MetricComplexity             = "complexity"
	MetricCognitiveComplexity    = "cognitive_complexity"
	MetricNestingLevel           = "nesting_level"
	MetricCoupling               = "coupling"
	MetricBugs                   = "bugs"
	MetricVulnerabilities        = "vulnerabilities"
	MetricCodeSmells             = "code_smells"
	MetricCoverage               = "coverage"
	MetricLinesToCover           = "lines_to_cover"
	MetricCoveredLines           = "covered_lines"
	MetricUncoveredLines         = "uncovered_lines"
	MetricDuplicatedLines        = "duplicated_lines"
	MetricDuplicatedBlocks       = "duplicated_blocks"
	MetricCommentLines           = "comment_lines"
	MetricCommentDensity         = "comment_density"
	MetricTests                  = "tests"
	MetricTestFailures           = "test_failures"
	MetricTestErrors             = "test_errors"
	MetricTestSkipped            = "test_skipped"
	MetricTestDurationMs         = "test_duration_ms"
	MetricMutationScore          = "mutation_score"
	MetricMutantsTotal           = "mutants_total"
	MetricMutantsKilled          = "mutants_killed"
	MetricMutantsSurvived        = "mutants_survived"
	MetricMutantsTimeout         = "mutants_timeout"
	MetricMutantsSkipped         = "mutants_skipped"
	MetricMutantsError           = "mutants_error"
	MetricChangedMutationScore   = "changed_mutation_score"
	MetricChangedMutantsTotal    = "changed_mutants_total"
	MetricChangedMutantsKilled   = "changed_mutants_killed"
	MetricChangedMutantsSurvived = "changed_mutants_survived"
)
