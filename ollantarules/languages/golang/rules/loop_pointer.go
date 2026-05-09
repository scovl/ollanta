package rules

import (
	"go/ast"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// LoopPointer detects capturing a loop variable by reference inside a goroutine.
// This is a classic Go bug where all goroutines see the final value of the loop variable.
var LoopPointer = ollantarules.Rule{
	MetaKey: "go:loop-pointer",
	Check: func(ctx *ollantarules.AnalysisContext) []*domain.Issue {
		var issues []*domain.Issue
		ast.Inspect(ctx.AST, func(n ast.Node) bool {
			forStmt, ok := n.(*ast.ForStmt)
			if !ok {
				// Also check range statements
				rangeStmt, ok := n.(*ast.RangeStmt)
				if !ok {
					return true
				}
				issues = append(issues, checkRangeStmt(ctx, rangeStmt)...)
				return true
			}
			issues = append(issues, checkForStmt(ctx, forStmt)...)
			return true
		})
		return issues
	},
}

func checkForStmt(ctx *ollantarules.AnalysisContext, stmt *ast.ForStmt) []*domain.Issue {
	loopVars := extractForLoopVars(stmt)
	return checkGoStmtsInBody(ctx, stmt.Body, loopVars, "Loop variable captured by goroutine closure; pass it as an argument instead")
}

func checkRangeStmt(ctx *ollantarules.AnalysisContext, stmt *ast.RangeStmt) []*domain.Issue {
	loopVars := extractRangeLoopVars(stmt)
	return checkGoStmtsInBody(ctx, stmt.Body, loopVars, "Range variable captured by goroutine closure; pass it as an argument instead")
}

func extractForLoopVars(stmt *ast.ForStmt) map[string]bool {
	loopVars := map[string]bool{}
	if init, ok := stmt.Init.(*ast.AssignStmt); ok {
		for _, lhs := range init.Lhs {
			if id, ok := lhs.(*ast.Ident); ok {
				loopVars[id.Name] = true
			}
		}
	}
	if post, ok := stmt.Post.(*ast.IncDecStmt); ok {
		if id, ok := post.X.(*ast.Ident); ok {
			loopVars[id.Name] = true
		}
	}
	return loopVars
}

func extractRangeLoopVars(stmt *ast.RangeStmt) map[string]bool {
	loopVars := map[string]bool{}
	if id, ok := stmt.Key.(*ast.Ident); ok && id.Name != "_" {
		loopVars[id.Name] = true
	}
	if id, ok := stmt.Value.(*ast.Ident); ok && id.Name != "_" {
		loopVars[id.Name] = true
	}
	return loopVars
}

func checkGoStmtsInBody(ctx *ollantarules.AnalysisContext, body *ast.BlockStmt, loopVars map[string]bool, msg string) []*domain.Issue {
	var issues []*domain.Issue
	ast.Inspect(body, func(n ast.Node) bool {
		goStmt, ok := n.(*ast.GoStmt)
		if !ok {
			return true
		}
		fnLit, ok := goStmt.Call.Fun.(*ast.FuncLit)
		if !ok {
			return true
		}
		issues = append(issues, checkLoopVarsInFuncLit(ctx, fnLit, loopVars, msg)...)
		return true
	})
	return issues
}

func checkLoopVarsInFuncLit(ctx *ollantarules.AnalysisContext, fnLit *ast.FuncLit, loopVars map[string]bool, msg string) []*domain.Issue {
	var issues []*domain.Issue
	ast.Inspect(fnLit.Body, func(inner ast.Node) bool {
		id, ok := inner.(*ast.Ident)
		if !ok {
			return true
		}
		if loopVars[id.Name] && !isParamOfFuncLit(id, fnLit) {
			line := ctx.FileSet.Position(id.Pos()).Line
			issue := domain.NewIssue("go:loop-pointer", ctx.Path, line)
			issue.Severity = domain.SeverityMajor
			issue.Type = domain.TypeBug
			issue.Message = msg
			issues = append(issues, issue)
			return false
		}
		return true
	})
	return issues
}

func isParamOfFuncLit(id *ast.Ident, fnLit *ast.FuncLit) bool {
	if fnLit.Type == nil || fnLit.Type.Params == nil {
		return false
	}
	for _, p := range fnLit.Type.Params.List {
		for _, n := range p.Names {
			if n.Name == id.Name {
				return true
			}
		}
	}
	return false
}
