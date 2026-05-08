package scan

import "fmt"

type roleExpectation struct {
	CoverageThreshold   float64
	MutationThreshold   float64
	IntegrationRequired bool
	HealthWeight        int
}

var defaultRoleExpectations = map[string]roleExpectation{
	"domain":         {CoverageThreshold: 85, MutationThreshold: 70, HealthWeight: 3},
	"application":    {CoverageThreshold: 80, MutationThreshold: 65, HealthWeight: 2},
	"adapter":        {CoverageThreshold: 65, IntegrationRequired: true, HealthWeight: 1},
	"infrastructure": {CoverageThreshold: 60, IntegrationRequired: true, HealthWeight: 1},
	"frontend":       {CoverageThreshold: 70, HealthWeight: 1},
	"web":            {CoverageThreshold: 70, HealthWeight: 1},
	"service":        {CoverageThreshold: 75, HealthWeight: 1},
	"library":        {CoverageThreshold: 80, HealthWeight: 1},
	"unknown":        {CoverageThreshold: 60, HealthWeight: 1},
}

func evaluateTestHealth(report *TestSignalReport) {
	if report == nil {
		return
	}
	summary := &TestHealthSummary{Status: "unavailable", Score: 100, Modules: len(report.Modules)}
	if len(report.Modules) == 0 {
		summary.Score = 0
		summary.Recommendations = append(summary.Recommendations, "No test modules were discovered. Run doctor mode or configure [[tests.modules]].")
		report.Health = summary
		return
	}
	projectScore := 0
	totalWeight := 0
	for index := range report.Modules {
		module := &report.Modules[index]
		module.Health = evaluateModuleHealth(module)
		weight := moduleRoleExpectation(module).HealthWeight
		if weight <= 0 {
			weight = 1
		}
		projectScore += module.Health.Score * weight
		totalWeight += weight
		if module.Health.Status == "at_risk" {
			summary.ModulesAtRisk++
		}
		if module.Health.Partial {
			summary.PartialModules++
		}
		summary.Recommendations = append(summary.Recommendations, module.Health.Recommendations...)
		addModuleMeasures(&report.Summary, module)
	}
	if totalWeight > 0 {
		summary.Score = projectScore / totalWeight
	} else {
		summary.Score = projectScore / len(report.Modules)
	}
	switch {
	case summary.ModulesAtRisk > 0:
		summary.Status = "at_risk"
	case summary.PartialModules > 0:
		summary.Status = "partial"
	default:
		summary.Status = "healthy"
	}
	report.Health = summary
}

func evaluateModuleHealth(module *TestModuleSignal) *TestModuleHealth {
	health := &TestModuleHealth{Status: "healthy", Score: 100, Confidence: "high"}
	if module.TestPolicy == TestPolicyIgnored {
		health.Status = "ignored"
		health.Confidence = "not_applicable"
		health.Reasons = append(health.Reasons, firstNonEmpty(module.IgnoreReason, "module is configured as ignored"))
		return health
	}
	expectation := moduleRoleExpectation(module)
	checkSuiteAvailability(module, health)
	checkCoverageHealth(module, health, coverageThreshold(module, expectation))
	checkNewCoverageHealth(module, health)
	checkFailureHealth(module, health)
	checkIntegrationHealth(module, health, expectation)
	checkMutationHealth(module, health, mutationThreshold(module, expectation))
	checkStaleReportHealth(module, health)
	if health.Score < 0 {
		health.Score = 0
	}
	if health.Score < 75 {
		health.Status = "at_risk"
	} else if health.Partial {
		health.Status = "partial"
	}
	return health
}

func moduleRoleExpectation(module *TestModuleSignal) roleExpectation {
	expectation := defaultRoleExpectations[firstNonEmpty(module.ArchitectureRole, "unknown")]
	if expectation.CoverageThreshold == 0 {
		return defaultRoleExpectations["unknown"]
	}
	return expectation
}

func coverageThreshold(module *TestModuleSignal, expectation roleExpectation) float64 {
	if module.CoverageThreshold != nil {
		return *module.CoverageThreshold
	}
	return expectation.CoverageThreshold
}

func mutationThreshold(module *TestModuleSignal, expectation roleExpectation) float64 {
	if module.MutationThreshold != nil {
		return *module.MutationThreshold
	}
	return expectation.MutationThreshold
}

func checkSuiteAvailability(module *TestModuleSignal, health *TestModuleHealth) {
	if len(module.Suites) > 0 {
		return
	}
	if hasMutationEvidence(module) {
		module.Availability = EvidenceAvailabilityPartial
		health.Partial = true
		health.Confidence = EvidenceConfidenceMedium
		health.Reasons = append(health.Reasons, "automated test suite report unavailable; mutation evidence collected")
		health.Recommendations = append(health.Recommendations, fmt.Sprintf("Add JUnit or native test-suite evidence for module %s to increase confidence.", module.Name))
		return
	}
	module.Availability = EvidenceAvailabilityUnavailable
	health.Partial = true
	health.Confidence = EvidenceConfidenceMedium
	health.Score -= 10
	health.Reasons = append(health.Reasons, "automated test suite report unavailable")
	health.Recommendations = append(health.Recommendations, fmt.Sprintf("Add a JUnit or native test report for module %s.", module.Name))
}

func checkCoverageHealth(module *TestModuleSignal, health *TestModuleHealth, threshold float64) {
	if module.Coverage == nil || module.Coverage.Coverage == nil {
		health.Partial = true
		health.Confidence = "medium"
		health.Score -= 15
		health.Reasons = append(health.Reasons, "coverage report unavailable")
		health.Recommendations = append(health.Recommendations, fmt.Sprintf("Add a coverage report for module %s or configure coverage_reports.", module.Name))
		return
	}
	if *module.Coverage.Coverage >= threshold {
		return
	}
	health.Score -= 25
	health.Reasons = append(health.Reasons, fmt.Sprintf("coverage %.1f is below %.1f threshold", *module.Coverage.Coverage, threshold))
	health.Recommendations = append(health.Recommendations, fmt.Sprintf("Increase coverage for %s %s module.", module.ArchitectureRole, module.Name))
}

func checkNewCoverageHealth(module *TestModuleSignal, health *TestModuleHealth) {
	if module.Coverage == nil || module.Coverage.NewCodeCoverage == nil || module.NewCoverageThreshold == nil || *module.Coverage.NewCodeCoverage >= *module.NewCoverageThreshold {
		return
	}
	health.Score -= 20
	health.Reasons = append(health.Reasons, fmt.Sprintf("new-code coverage %.1f is below %.1f threshold", *module.Coverage.NewCodeCoverage, *module.NewCoverageThreshold))
	health.Recommendations = append(health.Recommendations, fmt.Sprintf("Add tests for changed code in module %s.", module.Name))
}

func checkFailureHealth(module *TestModuleSignal, health *TestModuleHealth) {
	failures, errors := moduleFailures(module)
	if failures+errors == 0 {
		return
	}
	health.Score -= 35
	health.Reasons = append(health.Reasons, fmt.Sprintf("%d failing or errored tests", failures+errors))
	health.Recommendations = append(health.Recommendations, fmt.Sprintf("Fix failing tests in module %s before relying on coverage.", module.Name))
}

func checkIntegrationHealth(module *TestModuleSignal, health *TestModuleHealth, expectation roleExpectation) {
	if (!expectation.IntegrationRequired && !module.IntegrationRequired) || hasIntegrationSuite(module) {
		return
	}
	health.Score -= 15
	health.Reasons = append(health.Reasons, "integration evidence unavailable")
	health.Recommendations = append(health.Recommendations, fmt.Sprintf("Provide integration test evidence for %s module %s.", module.ArchitectureRole, module.Name))
}

func checkMutationHealth(module *TestModuleSignal, health *TestModuleHealth, threshold float64) {
	if threshold <= 0 {
		return
	}
	if module.Mutation == nil || (module.Mutation.Score == nil && module.Mutation.ChangedCodeScore == nil) {
		health.Partial = true
		health.Score -= 5
		health.Reasons = append(health.Reasons, "mutation report unavailable")
		return
	}
	if module.Mutation.Stale || module.Mutation.Availability == EvidenceAvailabilityStale {
		health.Partial = true
		health.Confidence = EvidenceConfidenceLow
		health.Reasons = append(health.Reasons, "mutation report is stale")
	}
	if module.Mutation.ChangedCodeScore != nil && module.ChangedMutationThreshold != nil && *module.Mutation.ChangedCodeScore < *module.ChangedMutationThreshold {
		health.Score -= 20
		health.Reasons = append(health.Reasons, fmt.Sprintf("changed-code mutation score %.1f is below %.1f threshold", *module.Mutation.ChangedCodeScore, *module.ChangedMutationThreshold))
		health.Recommendations = append(health.Recommendations, fmt.Sprintf("Prioritize survived changed-code mutants in module %s.", module.Name))
	}
	if module.Mutation.Score == nil {
		return
	}
	if *module.Mutation.Score >= threshold {
		if module.Mutation.Survived > 0 {
			health.Recommendations = append(health.Recommendations, fmt.Sprintf("Review %d survived mutants in module %s.", module.Mutation.Survived, module.Name))
		}
		return
	}
	health.Score -= 15
	health.Reasons = append(health.Reasons, fmt.Sprintf("mutation score %.1f is below %.1f threshold", *module.Mutation.Score, threshold))
	health.Recommendations = append(health.Recommendations, fmt.Sprintf("Review survived mutants in module %s and add targeted tests.", module.Name))
}

func hasMutationEvidence(module *TestModuleSignal) bool {
	return module.Mutation != nil && (module.Mutation.Score != nil || module.Mutation.ChangedCodeScore != nil || module.Mutation.Testable > 0 || module.Mutation.Total > 0 || len(module.Mutation.Reports) > 0)
}

func checkStaleReportHealth(module *TestModuleSignal, health *TestModuleHealth) {
	if !hasStaleReports(module) {
		return
	}
	health.Confidence = "low"
	health.Score -= 10
	health.Reasons = append(health.Reasons, "one or more reports are stale")
	health.Recommendations = append(health.Recommendations, fmt.Sprintf("Refresh test reports for module %s or adjust max_report_age.", module.Name))
}

func moduleFailures(module *TestModuleSignal) (int, int) {
	failures := 0
	errors := 0
	for _, suite := range module.Suites {
		failures += suite.Failures
		errors += suite.Errors
	}
	return failures, errors
}

func hasIntegrationSuite(module *TestModuleSignal) bool {
	for _, suite := range module.Suites {
		if suite.Kind == SuiteKindIntegration {
			return true
		}
	}
	return false
}

func hasStaleReports(module *TestModuleSignal) bool {
	for _, report := range module.Reports {
		if report.Freshness == "stale" {
			return true
		}
	}
	return false
}

func addModuleMeasures(summary *TestSignalSummary, module *TestModuleSignal) {
	for _, suite := range module.Suites {
		summary.Tests += suite.Tests
		summary.TestFailures += suite.Failures
		summary.TestErrors += suite.Errors
		summary.TestSkipped += suite.Skipped
		summary.TestDurationMs += suite.DurationMs
	}
	if module.Coverage != nil && module.Coverage.Coverage != nil {
		summary.ModulesWithCoverage++
		summary.CoveredLines += module.Coverage.CoveredLines
		summary.LinesToCover += module.Coverage.LinesToCover
		summary.NewCoveredLines += module.Coverage.NewCoveredLines
		summary.NewLinesToCover += module.Coverage.NewLinesToCover
	}
	if module.Mutation != nil {
		summary.MutantsTotal += module.Mutation.Total
		summary.MutantsKilled += module.Mutation.Killed
		summary.MutantsSurvived += module.Mutation.Survived
		summary.MutantsTimeout += module.Mutation.Timeout
		summary.MutantsSkipped += module.Mutation.Skipped
		summary.MutantsError += module.Mutation.Errors
		summary.ChangedMutantsTotal += module.Mutation.ChangedTotal
		summary.ChangedMutantsKilled += module.Mutation.ChangedKilled
		summary.ChangedMutantsSurvived += module.Mutation.ChangedSurvived
	}
	if summary.LinesToCover > 0 {
		coverage := float64(summary.CoveredLines) * 100 / float64(summary.LinesToCover)
		summary.Coverage = &coverage
	}
	if summary.NewLinesToCover > 0 {
		newCodeCoverage := float64(summary.NewCoveredLines) * 100 / float64(summary.NewLinesToCover)
		summary.NewCodeCoverage = &newCodeCoverage
	}
	mutationDenominator := 0
	if summary.MutantsKilled+summary.MutantsSurvived > 0 {
		mutationDenominator = summary.MutantsKilled + summary.MutantsSurvived
	}
	if mutationDenominator > 0 {
		mutationScore := float64(summary.MutantsKilled) * 100 / float64(mutationDenominator)
		summary.MutationScore = &mutationScore
	}
	changedDenominator := summary.ChangedMutantsKilled + summary.ChangedMutantsSurvived
	if changedDenominator > 0 {
		changedMutationScore := float64(summary.ChangedMutantsKilled) * 100 / float64(changedDenominator)
		summary.ChangedMutationScore = &changedMutationScore
	}
}
