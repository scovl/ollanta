// Package main is the entry point for the Ollanta CLI scanner.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	telemetry "github.com/scovl/ollanta/adapter/secondary/telemetry"
	"github.com/scovl/ollanta/ollantascanner/scan"
	"github.com/scovl/ollanta/ollantascanner/server"
)

type serverScanJob struct {
	ID        int64  `json:"id"`
	Status    string `json:"status"`
	ScanID    *int64 `json:"scan_id,omitempty"`
	LastError string `json:"last_error,omitempty"`
}

var serverHTTPClient = &http.Client{Timeout: 30 * time.Second}

func printRunPlan(opts *scan.ScanOptions) {
	sources := strings.Join(opts.Sources, ",")
	if sources == "" {
		sources = "./..."
	}
	slog.Info("scan plan", "project", opts.ProjectDir, "sources", sources)
	if opts.Tests.Enabled {
		slog.Info("test signals enabled", "modules", len(opts.Tests.Modules))
	}
	if opts.Server != "" {
		slog.Info("will push results to server", "server", opts.Server)
	}
	if opts.Serve {
		slog.Info("will open local UI", "bind", opts.Bind, "port", opts.Port)
	}
}

type serverScanResult struct {
	ID           int64              `json:"id"`
	Status       string             `json:"status"`
	GateStatus   string             `json:"gate_status"`
	GateResult   *gateResultSummary `json:"gate_result,omitempty"`
	NewIssues    int                `json:"new_issues"`
	ClosedIssues int                `json:"closed_issues"`
}

type gateResultSummary struct {
	Status     string               `json:"status"`
	Conditions []gateConditionEval  `json:"conditions"`
}

type gateConditionEval struct {
	Metric    string  `json:"metric"`
	Operator  string  `json:"operator"`
	Threshold float64 `json:"threshold"`
	Actual    float64 `json:"actual"`
	HasValue  bool    `json:"has_value"`
	Status    string  `json:"status"`
}

func main() {
	slog.SetDefault(telemetry.SetupLogger(os.Getenv("OLLANTA_LOG_LEVEL"), "service", "ollantascanner", "role", "server"))
	shutdownTracing, err := telemetry.SetupTracing(context.Background(), "ollantascanner")
	if err != nil {
		slog.Error("setup tracing", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			slog.Warn("shutdown tracing", "error", err)
		}
	}()

	opts := mustParseOptions()
	printRunPlan(opts)
	r := mustRunScan(opts)

	scan.PrintSummary(r)

	reportPath := server.ReportPath(opts.ProjectDir)
	saveOutputs(opts, r)
	serveReport(opts, reportPath)
	handleServerPush(opts, r)
}

func mustParseOptions() *scan.ScanOptions {
	opts, err := parseOptions(os.Args[1:])
	if err != nil {
		slog.Error("parse options", "error", err)
		os.Exit(2)
	}
	return opts
}

func mustRunScan(opts *scan.ScanOptions) *scan.Report {
	started := time.Now()
	slog.Info("analyzing source files")
	r, err := scan.Run(context.Background(), opts)
	if err != nil {
		slog.Error("scan failed", "error", err)
		os.Exit(1)
	}
	slog.Info("analysis completed", "duration_seconds", time.Since(started).Seconds())
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
		slog.Error("save JSON report", "error", err)
		return
	}
	slog.Info("report saved", "format", "json", "path", path)
}

func saveSARIF(opts *scan.ScanOptions, r *scan.Report) {
	path, err := r.SaveSARIF(opts.ProjectDir)
	if err != nil {
		slog.Error("save SARIF report", "error", err)
		return
	}
	slog.Info("report saved", "format", "sarif", "path", path)
}

func serveReport(opts *scan.ScanOptions, reportPath string) {
	if !opts.Serve {
		return
	}
	if err := server.Serve(reportPath, opts.Bind, opts.Port); err != nil {
		slog.Error("local UI server failed", "error", err)
		os.Exit(1)
	}
}

func handleServerPush(opts *scan.ScanOptions, r *scan.Report) {
	if opts.Server == "" {
		return
	}

	slog.Info("pushing report to server", "server", opts.Server)
	result, err := pushReport(opts.Server, opts.ServerToken, r)
	if err != nil {
		slog.Warn("server push failed", "error", err)
		return
	}

	if gs, ok := result["gate_status"].(string); ok && gs != "" {
		slog.Info("server response",
			"gate", gs,
			"new_issues", responseInt(result, "new_issues"),
			"closed_issues", responseInt(result, "closed_issues"),
		)
		logGateConditions(result)
		if gs == "ERROR" {
			os.Exit(1)
		}
		return
	}

	slog.Info("server accepted job",
		"job_id", responseInt(result, "id"),
		"status", result["status"],
	)
	if !opts.ServerWait {
		return
	}

	jobID := int64(responseInt(result, "id"))
	if jobID == 0 {
		slog.Error("accepted response did not include a valid scan job id", "response", result)
		os.Exit(1)
	}

	slog.Info("waiting for server job", "job_id", jobID, "timeout", opts.WaitTimeout, "poll", opts.WaitPoll)
	finalScan, err := waitForServerJob(opts.Server, opts.ServerToken, jobID, opts.WaitTimeout, opts.WaitPoll)
	if err != nil {
		slog.Error("waiting for server job failed", "job_id", jobID, "error", err)
		os.Exit(1)
	}

	slog.Info("server scan completed",
		"gate", finalScan.GateStatus,
		"new_issues", finalScan.NewIssues,
		"closed_issues", finalScan.ClosedIssues,
	)
	if finalScan.GateResult != nil {
		logGateConditionsSummary(finalScan.GateResult)
	}
	if finalScan.GateStatus == "ERROR" {
		os.Exit(1)
	}
}

// responseInt extracts a numeric field from a JSON-decoded response map.
// JSON numbers decode to float64; missing/non-numeric values return 0 and
// are logged so silent metric loss is visible in operator logs.
func responseInt(result map[string]interface{}, key string) int {
	raw, present := result[key]
	if !present {
		return 0
	}
	f, ok := raw.(float64)
	if !ok {
		slog.Warn("server response field is not numeric", "key", key, "value", raw)
		return 0
	}
	return int(f)
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
	if idempotencyKey := reportIdempotencyKey(r); idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := serverHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, readResponsePreview(resp.Body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}

func reportIdempotencyKey(report any) string {
	metadata, ok := reportMetadata(report)
	if !ok || metadata.ProjectKey == "" || metadata.AnalysisDate == "" {
		return ""
	}
	parts := []string{
		"ollanta-scan",
		metadata.ProjectKey,
		metadata.ScopeType,
		metadata.Branch,
		metadata.PullRequestKey,
		metadata.PullRequestBase,
		metadata.CommitSHA,
		metadata.AnalysisDate,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "scanner:" + hex.EncodeToString(sum[:])
}

func reportMetadata(report any) (scan.Metadata, bool) {
	switch typed := report.(type) {
	case *scan.Report:
		if typed == nil {
			return scan.Metadata{}, false
		}
		return typed.Metadata, true
	case scan.Report:
		return typed.Metadata, true
	default:
		return scan.Metadata{}, false
	}
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
		printServerJobStatus(jobID, job)

		result, done, err := completedServerJobResult(serverURL, token, jobID, job)
		if done || err != nil {
			return result, err
		}

		if !deadline.IsZero() && time.Now().After(deadline) {
			return nil, fmt.Errorf("scan job %d did not finish before timeout", jobID)
		}
		time.Sleep(poll)
	}
}

func printServerJobStatus(jobID int64, job *serverScanJob) {
	attrs := []any{"job_id", jobID, "status", job.Status}
	if job.ScanID != nil {
		attrs = append(attrs, "scan_id", *job.ScanID)
	}
	if job.LastError != "" {
		attrs = append(attrs, "last_error", job.LastError)
	}
	slog.Info("server job status", attrs...)
}

func completedServerJobResult(serverURL, token string, jobID int64, job *serverScanJob) (*serverScanResult, bool, error) {
	switch job.Status {
	case "completed":
		if job.ScanID == nil || *job.ScanID == 0 {
			return nil, true, fmt.Errorf("scan job %d completed without a linked scan id", jobID)
		}
		result, err := getServerScanResult(serverURL, token, *job.ScanID)
		return result, true, err
	case "failed":
		return nil, true, failedServerJobError(jobID, job.LastError)
	default:
		return nil, false, nil
	}
}

func failedServerJobError(jobID int64, lastError string) error {
	if lastError != "" {
		return fmt.Errorf("scan job %d failed: %s", jobID, lastError)
	}
	return fmt.Errorf("scan job %d failed", jobID)
}

func getServerScanJob(serverURL, token string, jobID int64) (*serverScanJob, error) {
	req, err := authorizedRequest(http.MethodGet, fmt.Sprintf("%s/api/v1/scan-jobs/%d", serverURL, jobID), token, nil)
	if err != nil {
		return nil, err
	}

	resp, err := serverHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get scan job: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get scan job returned %d: %s", resp.StatusCode, readResponsePreview(resp.Body))
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

	resp, err := serverHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get scan result: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get scan result returned %d: %s", resp.StatusCode, readResponsePreview(resp.Body))
	}

	var result serverScanResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode scan result: %w", err)
	}
	return &result, nil
}

func readResponsePreview(body io.Reader) string {
	data, err := io.ReadAll(io.LimitReader(body, 4096))
	if err != nil {
		return err.Error()
	}
	return strings.TrimSpace(string(data))
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

func logGateConditions(result map[string]interface{}) {
	gr, ok := result["gate_result"]
	if !ok {
		return
	}
	grMap, ok := gr.(map[string]interface{})
	if !ok {
		return
	}
	conditions, ok := grMap["conditions"].([]interface{})
	if !ok {
		return
	}
	for _, c := range conditions {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		status, _ := cm["status"].(string)
		if status == "OK" {
			continue
		}
		metric, _ := cm["metric"].(string)
		operator, _ := cm["operator"].(string)
		threshold, _ := cm["threshold"].(float64)
		actual, _ := cm["actual"].(float64)
		slog.Warn("gate condition",
			"metric", metric,
			"operator", operator,
			"threshold", threshold,
			"actual", actual,
			"status", status,
		)
	}
}

func logGateConditionsSummary(gr *gateResultSummary) {
	if gr == nil {
		return
	}
	for _, c := range gr.Conditions {
		if c.Status == "OK" {
			continue
		}
		slog.Warn("gate condition",
			"metric", c.Metric,
			"operator", c.Operator,
			"threshold", c.Threshold,
			"actual", c.Actual,
			"status", c.Status,
		)
	}
}
