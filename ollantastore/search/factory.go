// factory.go provides a factory function that returns the correct ISearcher
// and IIndexer implementations based on configuration.
package search

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewBackend creates ISearcher and IIndexer based on the backend name.
// Supported backends: "zincsearch" (default), "postgres".
func NewBackend(backend string, zincCfg ZincConfig, pool *pgxpool.Pool) (ISearcher, IIndexer, error) {
	switch backend {
	case "zincsearch", "":
		b := NewZincBackend(zincCfg)
		return b, b, nil
	case "postgres":
		b := NewPgFTSBackend(pool)
		return b, b, nil
	default:
		return nil, nil, fmt.Errorf("unsupported search backend: %q", backend)
	}
}
