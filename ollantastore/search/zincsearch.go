// zincsearch.go implements ISearcher and IIndexer using the ZincSearch v1 API.
//
// ZincSearch is an open-source, lightweight, Elasticsearch-compatible search
// engine written in Go. Unlike Meilisearch, it has no license restrictions on
// horizontal scaling — any number of replicas can run freely.
//
// Communication uses plain HTTP + JSON with Basic Auth.
// Index creation is implicit (auto-created on first document insert).
package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/scovl/ollanta/ollantastore/postgres"
)

// ZincConfig holds connection parameters for ZincSearch.
type ZincConfig struct {
	Host     string // e.g. "http://localhost:4080"
	User     string // basic-auth user
	Password string // basic-auth password
}

// ZincBackend implements ISearcher and IIndexer using ZincSearch HTTP API.
type ZincBackend struct {
	cfg    ZincConfig
	client *http.Client
}

// compile-time interface checks
var (
	_ ISearcher = (*ZincBackend)(nil)
	_ IIndexer  = (*ZincBackend)(nil)
)

// NewZincBackend creates a ZincSearch backend.
func NewZincBackend(cfg ZincConfig) *ZincBackend {
	return &ZincBackend{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ─── IIndexer ──────────────────────────────────────────────────────────────────

// Health pings ZincSearch /healthz endpoint.
func (z *ZincBackend) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, z.cfg.Host+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := z.client.Do(req)
	if err != nil {
		return fmt.Errorf("zincsearch health: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("zincsearch health: status %d", resp.StatusCode)
	}
	return nil
}

// ConfigureIndexes creates the issues and projects indexes with proper mappings.
// ZincSearch auto-creates indexes on first document, but explicit creation lets
// us define field types for keyword (exact-match) vs text (full-text) fields.
func (z *ZincBackend) ConfigureIndexes(ctx context.Context) error {
	indexes := map[string]map[string]interface{}{
		indexIssues: {
			"name":         indexIssues,
			"storage_type": "disk",
			"mappings": map[string]interface{}{
				"properties": map[string]interface{}{
					"id":             map[string]string{"type": "numeric"},
					"scan_id":        map[string]string{"type": "numeric"},
					"project_id":     map[string]string{"type": "numeric"},
					"project_key":    map[string]string{"type": "keyword"},
					"rule_key":       map[string]string{"type": "keyword"},
					"component_path": map[string]string{"type": "text"},
					"line":           map[string]string{"type": "numeric"},
					"message":        map[string]string{"type": "text"},
					"type":           map[string]string{"type": "keyword"},
					"severity":       map[string]string{"type": "keyword"},
					"status":         map[string]string{"type": "keyword"},
					"tags":           map[string]string{"type": "keyword"},
					"created_at":     map[string]string{"type": "date"},
				},
			},
		},
		indexProjects: {
			"name":         indexProjects,
			"storage_type": "disk",
			"mappings": map[string]interface{}{
				"properties": map[string]interface{}{
					"id":          map[string]string{"type": "numeric"},
					"key":         map[string]string{"type": "keyword"},
					"name":        map[string]string{"type": "text"},
					"description": map[string]string{"type": "text"},
					"tags":        map[string]string{"type": "keyword"},
					"created_at":  map[string]string{"type": "date"},
				},
			},
		},
	}

	for name, body := range indexes {
		if err := z.putIndex(ctx, name, body); err != nil {
			return err
		}
	}
	return nil
}

// putIndex creates or updates a ZincSearch index via PUT /api/index/:target.
func (z *ZincBackend) putIndex(ctx context.Context, name string, body map[string]interface{}) error {
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, z.cfg.Host+"/api/index/"+name, bytes.NewReader(data))
	if err != nil {
		return err
	}
	z.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := z.client.Do(req)
	if err != nil {
		return fmt.Errorf("zincsearch create index %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("zincsearch create index %s: status %d: %s", name, resp.StatusCode, string(b))
	}
	return nil
}

// IndexIssues bulk-inserts issues into the issues index using the v2 bulk API.
func (z *ZincBackend) IndexIssues(ctx context.Context, projectKey string, issues []postgres.IssueRow) error {
	if len(issues) == 0 {
		return nil
	}

	records := make([]map[string]interface{}, len(issues))
	for i, iss := range issues {
		tags := iss.Tags
		if tags == nil {
			tags = []string{}
		}
		records[i] = map[string]interface{}{
			"_id":            strconv.FormatInt(iss.ID, 10),
			"id":             iss.ID,
			"scan_id":        iss.ScanID,
			"project_id":     iss.ProjectID,
			"project_key":    projectKey,
			"rule_key":       iss.RuleKey,
			"component_path": iss.ComponentPath,
			"line":           iss.Line,
			"message":        iss.Message,
			"type":           iss.Type,
			"severity":       iss.Severity,
			"status":         iss.Status,
			"tags":           tags,
			"created_at":     iss.CreatedAt.Format(time.RFC3339),
		}
	}

	return z.bulkV2(ctx, indexIssues, records)
}

// IndexProject upserts a project document.
func (z *ZincBackend) IndexProject(ctx context.Context, p *postgres.Project) error {
	doc := map[string]interface{}{
		"_id":         strconv.FormatInt(p.ID, 10),
		"id":          p.ID,
		"key":         p.Key,
		"name":        p.Name,
		"description": p.Description,
		"tags":        p.Tags,
		"created_at":  p.CreatedAt.Format(time.RFC3339),
	}
	return z.bulkV2(ctx, indexProjects, []map[string]interface{}{doc})
}

// DeleteScanIssues removes all issues belonging to a scan.
// ZincSearch doesn't support delete-by-query, so we search for IDs then delete one by one.
func (z *ZincBackend) DeleteScanIssues(ctx context.Context, scanID int64) error {
	// Search for all issue IDs with this scan_id
	query := map[string]interface{}{
		"search_type": "querystring",
		"query": map[string]interface{}{
			"term": fmt.Sprintf("scan_id:%d", scanID),
		},
		"max_results": 10000,
		"_source":     []string{"id"},
	}

	data, _ := json.Marshal(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, z.cfg.Host+"/api/"+indexIssues+"/_search", bytes.NewReader(data))
	if err != nil {
		return err
	}
	z.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := z.client.Do(req)
	if err != nil {
		return fmt.Errorf("zincsearch delete scan search: %w", err)
	}
	defer resp.Body.Close()

	var sr zincSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return fmt.Errorf("zincsearch delete scan decode: %w", err)
	}

	// Delete each document
	for _, hit := range sr.Hits.Hits {
		delReq, err := http.NewRequestWithContext(ctx, http.MethodDelete,
			z.cfg.Host+"/api/"+indexIssues+"/_doc/"+hit.ID, nil)
		if err != nil {
			return err
		}
		z.setAuth(delReq)
		delResp, err := z.client.Do(delReq)
		if err != nil {
			return fmt.Errorf("zincsearch delete doc %s: %w", hit.ID, err)
		}
		delResp.Body.Close()
	}
	return nil
}

// ReindexAll rebuilds the entire search index from the database.
func (z *ZincBackend) ReindexAll(ctx context.Context, issueRepo *postgres.IssueRepository, projectRepo *postgres.ProjectRepository) error {
	// Delete and recreate indexes
	for _, idx := range []string{indexIssues, indexProjects} {
		delReq, err := http.NewRequestWithContext(ctx, http.MethodDelete,
			z.cfg.Host+"/api/index/"+idx, nil)
		if err != nil {
			return err
		}
		z.setAuth(delReq)
		delResp, err := z.client.Do(delReq)
		if err != nil {
			return fmt.Errorf("zincsearch delete index %s: %w", idx, err)
		}
		delResp.Body.Close()
	}

	if err := z.ConfigureIndexes(ctx); err != nil {
		return fmt.Errorf("zincsearch reconfigure indexes: %w", err)
	}

	// Iterate projects and re-index
	offset := 0
	const batch = 200
	for {
		projects, _, err := projectRepo.List(ctx, batch, offset)
		if err != nil {
			return fmt.Errorf("list projects for reindex: %w", err)
		}
		if len(projects) == 0 {
			break
		}
		for _, p := range projects {
			if err := z.IndexProject(ctx, p); err != nil {
				return fmt.Errorf("index project %s: %w", p.Key, err)
			}

			issOffset := 0
			pid := p.ID
			for {
				issues, _, err := issueRepo.Query(ctx, postgres.IssueFilter{
					ProjectID: &pid,
					Limit:     1000,
					Offset:    issOffset,
				})
				if err != nil {
					return fmt.Errorf("query issues for reindex project %s: %w", p.Key, err)
				}
				if len(issues) == 0 {
					break
				}
				rows := make([]postgres.IssueRow, len(issues))
				for i, iss := range issues {
					rows[i] = *iss
				}
				if err := z.IndexIssues(ctx, p.Key, rows); err != nil {
					return fmt.Errorf("index issues for project %s: %w", p.Key, err)
				}
				issOffset += len(issues)
				if len(issues) < 1000 {
					break
				}
			}
		}
		offset += len(projects)
		if len(projects) < batch {
			break
		}
	}
	return nil
}

// ─── ISearcher ─────────────────────────────────────────────────────────────────

// SearchIssues executes a full-text query against the issues index.
func (z *ZincBackend) SearchIssues(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	return z.search(ctx, indexIssues, req)
}

// SearchProjects executes a full-text query against the projects index.
func (z *ZincBackend) SearchProjects(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	return z.search(ctx, indexProjects, req)
}

func (z *ZincBackend) search(ctx context.Context, index string, req SearchRequest) (*SearchResult, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}

	// Build ES-compatible query DSL (v2 API)
	must := []interface{}{}
	if req.Query != "" {
		must = append(must, map[string]interface{}{
			"query_string": map[string]interface{}{
				"query": req.Query,
			},
		})
	}

	filters := []interface{}{}
	for k, v := range req.Filter {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{
				k: v,
			},
		})
	}

	boolQuery := map[string]interface{}{}
	if len(must) > 0 {
		boolQuery["must"] = must
	} else {
		boolQuery["must"] = []interface{}{map[string]interface{}{"match_all": map[string]interface{}{}}}
	}
	if len(filters) > 0 {
		boolQuery["filter"] = filters
	}

	body := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": boolQuery,
		},
		"from": req.Offset,
		"size": limit,
	}

	if len(req.Sort) > 0 {
		sortClauses := make([]interface{}, 0, len(req.Sort))
		for _, s := range req.Sort {
			parts := strings.SplitN(s, ":", 2)
			field := parts[0]
			dir := "asc"
			if len(parts) == 2 && strings.EqualFold(parts[1], "desc") {
				dir = "desc"
			}
			sortClauses = append(sortClauses, map[string]interface{}{field: dir})
		}
		body["sort"] = sortClauses
	}

	if len(req.Facets) > 0 {
		aggs := map[string]interface{}{}
		for _, facet := range req.Facets {
			aggs[facet] = map[string]interface{}{
				"terms": map[string]interface{}{
					"field": facet,
					"size":  100,
				},
			}
		}
		body["aggs"] = aggs
	}

	data, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		z.cfg.Host+"/es/"+index+"/_search", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	z.setAuth(httpReq)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := z.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("zincsearch search %s: %w", index, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("zincsearch search %s: status %d: %s", index, resp.StatusCode, string(b))
	}

	var sr zincSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("zincsearch decode: %w", err)
	}

	result := &SearchResult{
		TotalHits:        int64(sr.Hits.Total.Value),
		ProcessingTimeMs: int64(sr.Took),
		Hits:             make([]map[string]interface{}, 0, len(sr.Hits.Hits)),
	}

	for _, hit := range sr.Hits.Hits {
		if hit.Source != nil {
			result.Hits = append(result.Hits, hit.Source)
		}
	}

	// Map aggregations to FacetDistribution
	if len(sr.Aggregations) > 0 {
		result.FacetDistribution = make(map[string]map[string]int, len(sr.Aggregations))
		for name, agg := range sr.Aggregations {
			m := make(map[string]int, len(agg.Buckets))
			for _, b := range agg.Buckets {
				m[fmt.Sprint(b.Key)] = b.DocCount
			}
			result.FacetDistribution[name] = m
		}
	}

	return result, nil
}

// ─── helpers ───────────────────────────────────────────────────────────────────

func (z *ZincBackend) setAuth(req *http.Request) {
	req.SetBasicAuth(z.cfg.User, z.cfg.Password)
}

// bulkV2 uses ZincSearch's JSON-array bulk API: POST /api/:target/_bulkv2
func (z *ZincBackend) bulkV2(ctx context.Context, index string, records []map[string]interface{}) error {
	body := map[string]interface{}{
		"index":   index,
		"records": records,
	}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		z.cfg.Host+"/api/"+index+"/_bulkv2", bytes.NewReader(data))
	if err != nil {
		return err
	}
	z.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := z.client.Do(req)
	if err != nil {
		return fmt.Errorf("zincsearch bulkv2 %s: %w", index, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("zincsearch bulkv2 %s: status %d: %s", index, resp.StatusCode, string(b))
	}
	return nil
}

// zincSearchResponse maps the ES-compatible search response from ZincSearch.
type zincSearchResponse struct {
	Took int `json:"took"`
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			ID     string                 `json:"_id"`
			Source map[string]interface{} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
	Aggregations map[string]struct {
		Buckets []struct {
			Key      interface{} `json:"key"`
			DocCount int         `json:"doc_count"`
		} `json:"buckets"`
	} `json:"aggregations"`
}
