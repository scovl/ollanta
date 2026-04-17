// Package main is the entry point for the Ollanta CLI scanner.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/scovl/ollanta/ollantascanner/report"
	"github.com/scovl/ollanta/ollantascanner/scan"
	"github.com/scovl/ollanta/ollantascanner/server"
)

func main() {
	opts, err := scan.ParseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx := context.Background()
	r, err := scan.Run(ctx, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan error:", err)
		os.Exit(1)
	}

	scan.PrintSummary(r)

	reportPath := server.ReportPath(opts.ProjectDir)

	switch opts.Format {
	case "json":
		if path, err := r.SaveJSON(opts.ProjectDir); err != nil {
			fmt.Fprintln(os.Stderr, "json error:", err)
		} else {
			fmt.Println("Report saved to", path)
		}
	case "sarif":
		if path, err := r.SaveSARIF(opts.ProjectDir); err != nil {
			fmt.Fprintln(os.Stderr, "sarif error:", err)
		} else {
			fmt.Println("SARIF saved to", path)
		}
	case "all":
		if path, err := r.SaveJSON(opts.ProjectDir); err != nil {
			fmt.Fprintln(os.Stderr, "json error:", err)
		} else {
			fmt.Println("Report saved to", path)
		}
		if path, err := r.SaveSARIF(opts.ProjectDir); err != nil {
			fmt.Fprintln(os.Stderr, "sarif error:", err)
		} else {
			fmt.Println("SARIF saved to", path)
		}
	}

	if opts.Serve {
		if err := server.Serve(reportPath, opts.Bind, opts.Port); err != nil {
			fmt.Fprintln(os.Stderr, "server error:", err)
			os.Exit(1)
		}
	}

	if opts.Server != "" {
		result, err := pushReport(opts.Server, opts.ServerToken, r)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: server push failed: %v\n", err)
		} else {
			toInt := func(v interface{}) int { f, _ := v.(float64); return int(f) }
			fmt.Printf("Server: gate=%s new=%d closed=%d\n",
				result["gate_status"], toInt(result["new_issues"]), toInt(result["closed_issues"]))
			if gs, _ := result["gate_status"].(string); gs == "ERROR" {
				os.Exit(1)
			}
		}
	}
}

// pushReport POSTs the scan report to the given server URL and returns the parsed response body.
func pushReport(serverURL, token string, r *report.Report) (map[string]interface{}, error) {
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

