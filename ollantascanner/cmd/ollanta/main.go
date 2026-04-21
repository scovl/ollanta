// Package main is the entry point for the Ollanta CLI scanner.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/scovl/ollanta/ollantascanner/scan"
	"github.com/scovl/ollanta/ollantascanner/server"
)

func main() {
	opts := mustParseOptions()
	r := mustRunScan(opts)

	scan.PrintSummary(r)

	reportPath := server.ReportPath(opts.ProjectDir)
	saveOutputs(opts, r)
	serveReport(opts, reportPath)
	handleServerPush(opts, r)
}

func mustParseOptions() *scan.ScanOptions {
	opts, err := scan.ParseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	return opts
}

func mustRunScan(opts *scan.ScanOptions) *scan.Report {
	r, err := scan.Run(context.Background(), opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan error:", err)
		os.Exit(1)
	}
	return r
}

func saveOutputs(opts *scan.ScanOptions, r *scan.Report) {
	switch opts.Format {
	case "json":
		saveJSON(opts, r)
	case "sarif":
		saveSARIF(opts, r)
	case "all":
		saveJSON(opts, r)
		saveSARIF(opts, r)
	}
}

func saveJSON(opts *scan.ScanOptions, r *scan.Report) {
	path, err := r.SaveJSON(opts.ProjectDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "json error:", err)
		return
	}
	fmt.Println("Report saved to", path)
}

func saveSARIF(opts *scan.ScanOptions, r *scan.Report) {
	path, err := r.SaveSARIF(opts.ProjectDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sarif error:", err)
		return
	}
	fmt.Println("SARIF saved to", path)
}

func serveReport(opts *scan.ScanOptions, reportPath string) {
	if !opts.Serve {
		return
	}
	if err := server.Serve(reportPath, opts.Bind, opts.Port); err != nil {
		fmt.Fprintln(os.Stderr, "server error:", err)
		os.Exit(1)
	}
}

func handleServerPush(opts *scan.ScanOptions, r *scan.Report) {
	if opts.Server == "" {
		return
	}

	result, err := pushReport(opts.Server, opts.ServerToken, r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: server push failed: %v\n", err)
		return
	}

	toInt := func(v interface{}) int { f, _ := v.(float64); return int(f) }
	fmt.Printf("Server: gate=%s new=%d closed=%d\n",
		result["gate_status"], toInt(result["new_issues"]), toInt(result["closed_issues"]))
	if gs, _ := result["gate_status"].(string); gs == "ERROR" {
		os.Exit(1)
	}
}

// pushReport POSTs the scan report to the given server URL and returns the parsed response body.
func pushReport(serverURL, token string, r interface{}) (map[string]interface{}, error) {
	body, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("marshal report: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, serverURL+"/api/v1/scans", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}
