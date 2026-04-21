// Package scan keeps the CLI-facing scanner API stable while delegating the
// execution flow to application/scan.
package scan

import (
	"context"

	appscan "github.com/scovl/ollanta/application/scan"
)

// ScanOptions defines the CLI-visible scanner options.
type ScanOptions = appscan.ScanOptions

// Report is the scanner report returned by Run.
type Report = appscan.Report

// ParseFlags parses args (typically os.Args[1:]) into ScanOptions.
func ParseFlags(args []string) (*ScanOptions, error) {
	return appscan.ParseFlags(args)
}

// Run executes the full scan pipeline through application/scan.
func Run(ctx context.Context, opts *ScanOptions) (*Report, error) {
	return appscan.NewScanUseCase(newParserBridge(), newAnalyzerBridges()).Run(ctx, opts)
}

// PrintSummary writes a human-readable scan summary to stdout.
func PrintSummary(r *Report) {
	appscan.PrintSummary(r)
}
