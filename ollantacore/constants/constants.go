// Package constants defines the shared vocabulary for Ollanta: canonical language
// identifiers, metric keys, and the file-extension-to-language mapping.
// Inspired by GraphConstants.h from OpenStaticAnalyzer.
package constants

// Version is the current Ollanta product version reported by the scanner and server.
const Version = "0.2.0"

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

// Metric key constants — canonical string identifiers matching the metrics.Registry entries.
const (
	MetricLines               = "lines"
	MetricNcloc               = "ncloc"
	MetricFiles               = "files"
	MetricFunctions           = "functions"
	MetricStatements          = "statements"
	MetricComplexity          = "complexity"
	MetricCognitiveComplexity = "cognitive_complexity"
	MetricNestingLevel        = "nesting_level"
	MetricCoupling            = "coupling"
	MetricBugs                = "bugs"
	MetricVulnerabilities     = "vulnerabilities"
	MetricCodeSmells          = "code_smells"
	MetricCoverage            = "coverage"
	MetricDuplicatedLines     = "duplicated_lines"
	MetricDuplicatedBlocks    = "duplicated_blocks"
	MetricCommentLines        = "comment_lines"
	MetricCommentDensity      = "comment_density"
)
