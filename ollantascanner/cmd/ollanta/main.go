// Package main is the entry point for the Ollanta CLI scanner.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/scovl/ollanta/ollantascanner/scan"
	"github.com/scovl/ollanta/ollantascanner/server"
)

type serverScanJob struct {
	ID        int64  `json:"id"`
	Status    string `json:"status"`
	ScanID    *int64 `json:"scan_id,omitempty"`
	LastError string `json:"last_error,omitempty"`
}

type serverScanResult struct {
	ID           int64  `json:"id"`
	Status       string `json:"status"`
	GateStatus   string `json:"gate_status"`
	NewIssues    int    `json:"new_issues"`
	ClosedIssues int    `json:"closed_issues"`
}

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
	if gs, ok := result["gate_status"].(string); ok && gs != "" {
		fmt.Printf("Server: gate=%s new=%d closed=%d\n",
			result["gate_status"], toInt(result["new_issues"]), toInt(result["closed_issues"]))
		if gs == "ERROR" {
			os.Exit(1)
		}
		return
	}

	fmt.Printf("Server: accepted job=%d status=%v\n", toInt(result["id"]), result["status"])
	if !opts.ServerWait {
		return
	}

	jobID := int64(toInt(result["id"]))
	if jobID == 0 {
		fmt.Fprintln(os.Stderr, "warning: accepted response did not include a valid scan job id")
		os.Exit(1)
	}

	finalScan, err := waitForServerJob(opts.Server, opts.ServerToken, jobID, opts.WaitTimeout, opts.WaitPoll)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: waiting for server job failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Server: gate=%s new=%d closed=%d\n", finalScan.GateStatus, finalScan.NewIssues, finalScan.ClosedIssues)
	if finalScan.GateStatus == "ERROR" {
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

func waitForServerJob(serverURL, token string, jobID int64, timeout, poll time.Duration) (*serverScanResult, error) {
	if poll <= 0 {
		poll = 2 * time.Second
	}

	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}

	for {
		job, err := getServerScanJob(serverURL, token, jobID)
		if err != nil {
			return nil, err
		}

		switch job.Status {
		case "completed":
			if job.ScanID == nil || *job.ScanID == 0 {
				return nil, fmt.Errorf("scan job %d completed without a linked scan id", jobID)
			}
			return getServerScanResult(serverURL, token, *job.ScanID)
		case "failed":
			if job.LastError != "" {
				return nil, fmt.Errorf("scan job %d failed: %s", jobID, job.LastError)
			}
			return nil, fmt.Errorf("scan job %d failed", jobID)
		}

		if !deadline.IsZero() && time.Now().After(deadline) {
			return nil, fmt.Errorf("scan job %d did not finish before timeout", jobID)
		}
		time.Sleep(poll)
	}
}

func getServerScanJob(serverURL, token string, jobID int64) (*serverScanJob, error) {
	req, err := authorizedRequest(http.MethodGet, fmt.Sprintf("%s/api/v1/scan-jobs/%d", serverURL, jobID), token, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("get scan job: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get scan job returned %d", resp.StatusCode)
	}

	var job serverScanJob
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return nil, fmt.Errorf("decode scan job: %w", err)
	}
	return &job, nil
}

func getServerScanResult(serverURL, token string, scanID int64) (*serverScanResult, error) {
	req, err := authorizedRequest(http.MethodGet, fmt.Sprintf("%s/api/v1/scans/%d", serverURL, scanID), token, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("get scan result: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get scan result returned %d", resp.StatusCode)
	}

	var result serverScanResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode scan result: %w", err)
	}
	return &result, nil
}

func authorizedRequest(method, url, token string, body *bytes.Reader) (*http.Request, error) {
	var reader *bytes.Reader
	if body != nil {
		reader = body
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}
