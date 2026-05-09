package ollantaparser

import (
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
)

// QueryMatch holds the captures produced by a single match of an S-expression query.
type QueryMatch struct {
	// Captures maps capture name to the matched syntax node.
	// e.g. "@fn.name" → the identifier node.
	Captures map[string]*sitter.Node
}

// QueryRunner executes tree-sitter S-expression queries against a ParsedFile.
// It is the primary API for declarative rules: write a pattern → get all matches.
//
// Example query (JavaScript):
//
//	(function_declaration name: (identifier) @fn.name) @fn
type QueryRunner struct{}

// NewQueryRunner creates a QueryRunner. The zero value is also usable.
func NewQueryRunner() *QueryRunner { return &QueryRunner{} }

// Run compiles and executes an S-expression query using the language embedded in f.
// lang must be the same grammar that was used to parse f.
func (qr *QueryRunner) Run(f *ParsedFile, query string, lang Language) ([]QueryMatch, error) {
	q, err := sitter.NewQuery([]byte(query), lang.tsLanguage())
	if err != nil {
		return nil, fmt.Errorf("ollantaparser: compile query for %s: %w", f.Path, err)
	}
	defer q.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(q, f.RootNode())

	var matches []QueryMatch
	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}
		m = cursor.FilterPredicates(m, f.Source)
		if len(m.Captures) == 0 {
			continue
		}
		qm := QueryMatch{Captures: make(map[string]*sitter.Node, len(m.Captures))}
		for _, cap := range m.Captures {
			name := q.CaptureNameForId(cap.Index)
			qm.Captures[name] = cap.Node
		}
		matches = append(matches, qm)
	}
	return matches, nil
}

// Text returns the source text of a syntax node.
func (qr *QueryRunner) Text(f *ParsedFile, node *sitter.Node) string {
	if node == nil {
		return ""
	}
	return node.Content(f.Source)
}

// Position converts a tree-sitter node to 1-indexed (startLine, startCol, endLine, endCol).
func (qr *QueryRunner) Position(node *sitter.Node) (startLine, startCol, endLine, endCol int) {
	if node == nil {
		return 0, 0, 0, 0
	}
	sp := node.StartPoint()
	ep := node.EndPoint()
	return int(sp.Row) + 1, int(sp.Column) + 1, int(ep.Row) + 1, int(ep.Column) + 1
}
