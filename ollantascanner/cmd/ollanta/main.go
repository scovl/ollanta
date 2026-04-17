// Package main is the entry point for the Ollanta CLI scanner.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/user/ollanta/ollantascanner/scan"
	"github.com/user/ollanta/ollantascanner/server"
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
		if err := server.Serve(reportPath, opts.Port); err != nil {
			fmt.Fprintln(os.Stderr, "server error:", err)
			os.Exit(1)
		}
	}
}

