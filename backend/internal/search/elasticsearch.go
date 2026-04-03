package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/tracing"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
)

type Client struct {
	enabled     bool
	indexPrefix string
	es          *elasticsearch.Client
}

type Hit struct {
	Kind   string         `json:"kind"`
	ID     string         `json:"id"`
	Score  float64        `json:"score"`
	Source map[string]any `json:"source"`
}

func New(cfg config.ElasticConfig) (*Client, error) {
	_, span, startedAt := tracing.StartModuleOperation(context.Background(), "search", "new")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "search", "new", err)
	}()

	if !cfg.Enabled {
		return &Client{enabled: false}, nil
	}

	var es *elasticsearch.Client
	es, err = elasticsearch.NewClient(elasticsearch.Config{
		Addresses: cfg.AddressList(),
		Username:  cfg.Username,
		Password:  cfg.Password,
	})
	if err != nil {
		err = fmt.Errorf("new elasticsearch client: %w", err)
		return nil, err
	}

	return &Client{enabled: true, indexPrefix: cfg.IndexPrefix, es: es}, nil
}

func (c *Client) IndexDocument(ctx context.Context, kind, id string, doc any) error {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "search", "index_document")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "search", "index_document", err)
	}()

	if c == nil || !c.enabled {
		return nil
	}
	body, err := json.Marshal(doc)
	if err != nil {
		err = fmt.Errorf("marshal document: %w", err)
		return err
	}
	idx := fmt.Sprintf("%s-%s", c.indexPrefix, kind)
	res, err := c.es.Index(idx, bytes.NewReader(body), c.es.Index.WithContext(ctx), c.es.Index.WithDocumentID(id), c.es.Index.WithRefresh("false"))
	if err != nil {
		err = fmt.Errorf("index document: %w", err)
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.IsError() {
		log.Printf("elasticsearch indexing warning: %s", res.Status())
	}
	return nil
}

func (c *Client) DeleteDocument(ctx context.Context, kind, id string) error {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "search", "delete_document")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "search", "delete_document", err)
	}()

	if c == nil || !c.enabled {
		return nil
	}
	idx := fmt.Sprintf("%s-%s", c.indexPrefix, kind)
	res, err := c.es.Delete(idx, id, c.es.Delete.WithContext(ctx), c.es.Delete.WithRefresh("false"))
	if err != nil {
		err = fmt.Errorf("delete document: %w", err)
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode == http.StatusNotFound {
		return nil
	}
	if res.IsError() {
		log.Printf("elasticsearch delete warning: %s", res.Status())
	}
	return nil
}

func (c *Client) Enabled() bool {
	return c != nil && c.enabled
}

func (c *Client) Ping(ctx context.Context) error {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "search", "ping")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "search", "ping", err)
	}()

	if c == nil || !c.enabled {
		return nil
	}
	res, err := c.es.Info(c.es.Info.WithContext(ctx))
	if err != nil {
		err = fmt.Errorf("elasticsearch ping: %w", err)
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.IsError() {
		err = fmt.Errorf("elasticsearch ping error: %s", res.Status())
		return err
	}
	return nil
}

func (c *Client) Search(ctx context.Context, tenantID, query string, kinds []string, size int) ([]Hit, error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "search", "search")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "search", "search", err)
	}()

	if c == nil || !c.enabled {
		return []Hit{}, nil
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return []Hit{}, nil
	}
	if size <= 0 || size > 200 {
		size = 50
	}
	if len(kinds) == 0 {
		kinds = []string{"alerts", "cases", "forum_threads"}
	}

	indices := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		kind = strings.TrimSpace(strings.ToLower(kind))
		if kind == "" {
			continue
		}
		indices = append(indices, fmt.Sprintf("%s-%s", c.indexPrefix, kind))
	}
	if len(indices) == 0 {
		return []Hit{}, nil
	}

	body := keywordSearchBody(q, tenantID, size)
	return c.searchWithPayload(ctx, indices, kinds, tenantID, body)
}

func (c *Client) Recent(ctx context.Context, tenantID string, kinds []string, size int) ([]Hit, error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "search", "recent")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "search", "recent", err)
	}()

	if c == nil || !c.enabled {
		return []Hit{}, nil
	}
	if size <= 0 || size > 200 {
		size = 50
	}
	if len(kinds) == 0 {
		kinds = []string{"alerts", "cases", "forum_threads"}
	}

	indices := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		kind = strings.TrimSpace(strings.ToLower(kind))
		if kind == "" {
			continue
		}
		indices = append(indices, fmt.Sprintf("%s-%s", c.indexPrefix, kind))
	}
	if len(indices) == 0 {
		return []Hit{}, nil
	}

	body := recentTenantBody(tenantID, size)
	return c.searchWithPayload(ctx, indices, kinds, tenantID, body)
}

func (c *Client) searchWithPayload(ctx context.Context, indices []string, kinds []string, tenantID string, body map[string]any) ([]Hit, error) {
	rawBody, err := json.Marshal(body)
	if err != nil {
		err = fmt.Errorf("marshal search body: %w", err)
		return nil, err
	}

	res, err := c.es.Search(
		c.es.Search.WithContext(ctx),
		c.es.Search.WithIndex(indices...),
		c.es.Search.WithBody(bytes.NewReader(rawBody)),
		c.es.Search.WithAllowNoIndices(true),
		c.es.Search.WithIgnoreUnavailable(true),
	)
	if err != nil {
		err = fmt.Errorf("elasticsearch search request: %w", err)
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.IsError() {
		err = fmt.Errorf("elasticsearch search error: %s", res.Status())
		return nil, err
	}

	var parsed struct {
		Hits struct {
			Hits []struct {
				ID     string         `json:"_id"`
				Index  string         `json:"_index"`
				Score  float64        `json:"_score"`
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		err = fmt.Errorf("decode search response: %w", err)
		return nil, err
	}

	results := make([]Hit, 0, len(parsed.Hits.Hits))
	for _, hit := range parsed.Hits.Hits {
		if tenantID != "" {
			sourceTenant := ""
			if value, ok := hit.Source["tenant_id"]; ok {
				sourceTenant = fmt.Sprint(value)
			}
			if sourceTenant != "" && sourceTenant != tenantID {
				continue
			}
		}

		kind := ""
		for _, candidate := range kinds {
			indexName := fmt.Sprintf("%s-%s", c.indexPrefix, candidate)
			if indexName == hit.Index {
				kind = candidate
				break
			}
		}
		if kind == "" {
			kind = hit.Index
		}

		score := hit.Score
		if score == 0 {
			if sourceScore, ok := hit.Source["_score"]; ok {
				if parsedScore, err := strconv.ParseFloat(fmt.Sprint(sourceScore), 64); err == nil {
					score = parsedScore
				}
			}
		}

		results = append(results, Hit{
			Kind:   kind,
			ID:     hit.ID,
			Score:  score,
			Source: hit.Source,
		})
	}

	return results, nil
}

func keywordSearchBody(query, tenantID string, size int) map[string]any {
	boolQuery := map[string]any{
		"must": []map[string]any{
			{
				"multi_match": map[string]any{
					"query":    query,
					"type":     "best_fields",
					"operator": "and",
					"fields":   []string{"title^4", "description^2", "source^2", "name^2", "body", "value"},
				},
			},
		},
		"should": []map[string]any{
			{
				"multi_match": map[string]any{
					"query":  query,
					"type":   "phrase_prefix",
					"fields": []string{"title^4", "description^2", "source^2", "name^2", "body", "value"},
				},
			},
		},
	}
	if trimmedTenant := strings.TrimSpace(tenantID); trimmedTenant != "" {
		boolQuery["filter"] = []map[string]any{
			{
				"bool": map[string]any{
					"should": []map[string]any{
						{
							"term": map[string]any{
								"tenant_id.keyword": trimmedTenant,
							},
						},
						{
							"term": map[string]any{
								"tenant_id": trimmedTenant,
							},
						},
					},
					"minimum_should_match": 1,
				},
			},
		}
	}
	return map[string]any{
		"size": size,
		"query": map[string]any{
			"bool": boolQuery,
		},
	}
}

func recentTenantBody(tenantID string, size int) map[string]any {
	boolQuery := map[string]any{}
	if trimmedTenant := strings.TrimSpace(tenantID); trimmedTenant != "" {
		boolQuery["filter"] = []map[string]any{
			{
				"bool": map[string]any{
					"should": []map[string]any{
						{
							"term": map[string]any{
								"tenant_id.keyword": trimmedTenant,
							},
						},
						{
							"term": map[string]any{
								"tenant_id": trimmedTenant,
							},
						},
					},
					"minimum_should_match": 1,
				},
			},
		}
	}
	if len(boolQuery) == 0 {
		return map[string]any{
			"size": size,
			"query": map[string]any{
				"match_all": map[string]any{},
			},
			"sort": []map[string]any{
				{"updated_at": map[string]any{"order": "desc", "missing": "_last"}},
				{"created_at": map[string]any{"order": "desc", "missing": "_last"}},
			},
		}
	}
	return map[string]any{
		"size": size,
		"query": map[string]any{
			"bool": boolQuery,
		},
		"sort": []map[string]any{
			{"updated_at": map[string]any{"order": "desc", "missing": "_last"}},
			{"created_at": map[string]any{"order": "desc", "missing": "_last"}},
		},
	}
}
