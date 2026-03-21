package codeindex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultQdrantURL  = "http://127.0.0.1:6333"
	defaultQdrantPort = "6333"
	defaultQdrantGRPC = "6334"
)

type storedFile struct {
	Path        string
	ContentHash string
}

type store struct {
	client       *http.Client
	runtime      *qdrantRuntime
	baseURL      string
	collection   string
	workspace    string
	dimensions   int
	ready        bool
	managedLocal bool
}

func openStore(dataDir, workspace, model string, dimensions int, qdrantURL string) (*store, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("code index: data directory is required")
	}
	if workspace == "" {
		return nil, fmt.Errorf("code index: workspace root is required")
	}
	if dimensions <= 0 {
		return nil, fmt.Errorf("code index: dimensions must be positive")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(qdrantURL), "/")
	if baseURL == "" {
		baseURL = defaultQdrantURL
	}
	runtime := &qdrantRuntime{
		storageDir: filepath.Join(dataDir, "vectordb", "qdrant"),
		baseURL:    baseURL,
	}
	if err := ensureDir(runtime.storageDir); err != nil {
		return nil, err
	}
	s := &store{
		client:       &http.Client{Timeout: 30 * time.Second},
		runtime:      runtime,
		baseURL:      baseURL,
		collection:   "sapphire_code_chunks_" + workspaceKey(fmt.Sprintf("%s:%s:%d", workspace, model, dimensions)),
		workspace:    workspace,
		dimensions:   dimensions,
		managedLocal: shouldManageLocalQdrant(baseURL, qdrantURL),
	}
	return s, nil
}

func (s *store) Close() error {
	if s == nil || s.runtime == nil {
		return nil
	}
	return s.runtime.Close()
}

func (s *store) init(ctx context.Context) error {
	if err := s.ensureRuntime(ctx); err != nil {
		return err
	}
	exists, err := s.collectionExists(ctx)
	if err != nil {
		return err
	}
	if exists {
		s.ready = true
		return nil
	}
	body := map[string]any{
		"vectors": map[string]any{
			"size":     s.dimensions,
			"distance": "Cosine",
		},
		"optimizers_config": map[string]any{
			"default_segment_number": 2,
		},
	}
	if err := s.requestJSON(ctx, http.MethodPut, "/collections/"+s.collection, body, nil); err != nil {
		return err
	}
	s.ready = true
	return nil
}

func (s *store) ensureRuntime(ctx context.Context) error {
	if s.ready {
		if err := s.ping(ctx); err == nil {
			return nil
		}
		s.ready = false
	}
	if err := s.ping(ctx); err == nil {
		s.ready = true
		return nil
	}
	if !s.managedLocal {
		return fmt.Errorf("code index: qdrant is not reachable at %s; set SAPPHIRE_QDRANT_URL to a running Qdrant server or use the default local endpoint", s.baseURL)
	}
	if err := s.startBundledQdrant(ctx); err != nil {
		return err
	}
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		if err := s.ping(ctx); err == nil {
			s.ready = true
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("code index: bundled qdrant started but the service did not become ready at %s", s.baseURL)
}

func (s *store) ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/collections", nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("qdrant ping returned status %d", resp.StatusCode)
}

func (s *store) startBundledQdrant(ctx context.Context) error {
	if err := s.ensurePortAvailable(); err != nil {
		return err
	}
	return s.runtime.Start(ctx)
}

func (s *store) ensurePortAvailable() error {
	listener, err := net.Listen("tcp", "127.0.0.1:"+defaultQdrantPort)
	if err != nil {
		return fmt.Errorf("code index: port %s is already in use and qdrant is not reachable there", defaultQdrantPort)
	}
	_ = listener.Close()
	return nil
}

func (s *store) collectionExists(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/collections/"+s.collection, nil)
	if err != nil {
		return false, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, nil
	}
	body, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("code index: get collection: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func (s *store) Clear(ctx context.Context) error {
	s.ready = false
	if err := s.requestJSON(ctx, http.MethodDelete, "/collections/"+s.collection, nil, nil); err != nil {
		if !isQdrantMissingCollection(err) {
			return err
		}
	}
	return s.init(ctx)
}

func (s *store) ListFiles(ctx context.Context) (map[string]storedFile, error) {
	if err := s.init(ctx); err != nil {
		return nil, err
	}
	points, err := s.scrollPoints(ctx, map[string]any{
		"include": []string{"path", "content_hash"},
	})
	if err != nil {
		return nil, err
	}
	files := make(map[string]storedFile)
	for _, point := range points {
		path, _ := payloadString(point.Payload, "path")
		hash, _ := payloadString(point.Payload, "content_hash")
		if path == "" || hash == "" {
			continue
		}
		files[path] = storedFile{Path: path, ContentHash: hash}
	}
	return files, nil
}

func (s *store) ReplaceFile(ctx context.Context, file indexedFile) error {
	if err := s.init(ctx); err != nil {
		return err
	}
	if file.NeedsDelete {
		if err := s.deletePath(ctx, file.Path); err != nil {
			return err
		}
	}
	return s.upsertPoints(ctx, pointsFromFile(file))
}

func (s *store) ReplaceFiles(ctx context.Context, files []indexedFile) error {
	if err := s.init(ctx); err != nil {
		return err
	}
	for _, file := range files {
		if !file.NeedsDelete {
			continue
		}
		if err := s.deletePath(ctx, file.Path); err != nil {
			return err
		}
	}
	points := make([]map[string]any, 0, 1024)
	for _, file := range files {
		filePoints := pointsFromFile(file)
		if len(filePoints) == 0 {
			continue
		}
		if len(points)+len(filePoints) > 768 && len(points) > 0 {
			if err := s.upsertPoints(ctx, points); err != nil {
				return err
			}
			points = points[:0]
		}
		points = append(points, filePoints...)
	}
	return s.upsertPoints(ctx, points)
}

func pointsFromFile(file indexedFile) []map[string]any {
	points := make([]map[string]any, 0, len(file.Chunks))
	indexedAtUnix := time.Now().Unix()
	for _, chunk := range file.Chunks {
		points = append(points, map[string]any{
			"id":     chunk.ID,
			"vector": chunk.Embedding,
			"payload": map[string]any{
				"path":            chunk.Path,
				"language":        chunk.Language,
				"kind":            chunk.Kind,
				"chunk_index":     chunk.ChunkIndex,
				"start_line":      chunk.StartLine,
				"end_line":        chunk.EndLine,
				"content_hash":    file.ContentHash,
				"chunk_hash":      chunk.ContentHash,
				"search_text":     chunk.SearchText,
				"content":         chunk.Content,
				"token_estimate":  chunk.TokenEstimate,
				"size_bytes":      file.Size,
				"mod_time_unix":   file.ModTimeUnix,
				"indexed_at_unix": indexedAtUnix,
			},
		})
	}
	return points
}

func (s *store) upsertPoints(ctx context.Context, points []map[string]any) error {
	if len(points) == 0 {
		return nil
	}
	return s.requestJSON(ctx, http.MethodPut, "/collections/"+s.collection+"/points?wait=true", map[string]any{
		"points": points,
	}, nil)
}

func (s *store) DeleteFile(ctx context.Context, path string) error {
	if err := s.init(ctx); err != nil {
		return err
	}
	return s.deletePath(ctx, path)
}

func (s *store) UpdateStats(context.Context, Stats) error { return nil }

func (s *store) Stats(ctx context.Context) (Stats, error) {
	if err := s.init(ctx); err != nil {
		return Stats{}, err
	}
	points, err := s.scrollPoints(ctx, map[string]any{
		"include": []string{"path", "token_estimate", "indexed_at_unix"},
	})
	if err != nil {
		return Stats{}, err
	}
	files := make(map[string]struct{})
	stats := Stats{
		ChunkCount:    len(points),
		EmbeddedCount: len(points),
	}
	var latest int64
	for _, point := range points {
		if path, _ := payloadString(point.Payload, "path"); path != "" {
			files[path] = struct{}{}
		}
		stats.EstimatedTokens += payloadInt(point.Payload, "token_estimate")
		if indexedAt := payloadInt64(point.Payload, "indexed_at_unix"); indexedAt > latest {
			latest = indexedAt
		}
	}
	stats.FileCount = len(files)
	if latest > 0 {
		stats.LastIndexedAt = time.Unix(latest, 0)
	}
	return stats, nil
}

func (s *store) Search(ctx context.Context, query string, queryVector []float32, limit int) ([]SearchResult, error) {
	if err := s.init(ctx); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 10
	}
	var response qdrantQueryResponse
	body := map[string]any{
		"query":        queryVector,
		"limit":        limit,
		"with_payload": true,
		"with_vector":  false,
	}
	if err := s.requestJSON(ctx, http.MethodPost, "/collections/"+s.collection+"/points/query", body, &response); err != nil {
		return nil, err
	}
	results := make([]SearchResult, 0, len(response.Result.Points))
	for _, point := range response.Result.Points {
		results = append(results, SearchResult{
			Path:      payloadStringDefault(point.Payload, "path"),
			Language:  payloadStringDefault(point.Payload, "language"),
			Kind:      payloadStringDefault(point.Payload, "kind"),
			StartLine: payloadInt(point.Payload, "start_line"),
			EndLine:   payloadInt(point.Payload, "end_line"),
			Score:     point.Score,
			Snippet:   snippet(payloadStringDefault(point.Payload, "content")),
		})
	}
	return results, nil
}

func (s *store) deletePath(ctx context.Context, path string) error {
	err := s.requestJSON(ctx, http.MethodPost, "/collections/"+s.collection+"/points/delete?wait=true", map[string]any{
		"filter": map[string]any{
			"must": []map[string]any{
				{
					"key": "path",
					"match": map[string]any{
						"value": path,
					},
				},
			},
		},
	}, nil)
	if isQdrantMissingCollection(err) {
		s.ready = false
		return nil
	}
	return err
}

func (s *store) scrollPoints(ctx context.Context, payload any) ([]qdrantPoint, error) {
	results := make([]qdrantPoint, 0, 512)
	var offset any
	for {
		var response qdrantScrollResponse
		body := map[string]any{
			"limit":        256,
			"with_payload": payload,
			"with_vector":  false,
		}
		if offset != nil {
			body["offset"] = offset
		}
		if err := s.requestJSON(ctx, http.MethodPost, "/collections/"+s.collection+"/points/scroll", body, &response); err != nil {
			if isQdrantMissingCollection(err) {
				return results, nil
			}
			return nil, err
		}
		results = append(results, response.Result.Points...)
		if response.Result.NextPageOffset == nil {
			break
		}
		offset = response.Result.NextPageOffset
	}
	return results, nil
}

func (s *store) requestJSON(ctx context.Context, method, path string, body any, out any) error {
	if err := s.ensureRuntime(ctx); err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("code index: qdrant %s %s failed with status %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	return json.Unmarshal(respBody, out)
}

type qdrantPoint struct {
	ID      any            `json:"id"`
	Score   float64        `json:"score"`
	Payload map[string]any `json:"payload"`
}

type qdrantScrollResponse struct {
	Result struct {
		Points         []qdrantPoint `json:"points"`
		NextPageOffset any           `json:"next_page_offset"`
	} `json:"result"`
}

type qdrantQueryResponse struct {
	Result struct {
		Points []qdrantPoint `json:"points"`
	} `json:"result"`
}

func payloadString(payload map[string]any, key string) (string, bool) {
	value, ok := payload[key]
	if !ok {
		return "", false
	}
	switch typed := value.(type) {
	case string:
		return typed, true
	default:
		return "", false
	}
}

func payloadStringDefault(payload map[string]any, key string) string {
	value, _ := payloadString(payload, key)
	return value
}

func payloadInt(payload map[string]any, key string) int {
	switch value := payload[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case json.Number:
		n, _ := value.Int64()
		return int(n)
	default:
		return 0
	}
}

func payloadInt64(payload map[string]any, key string) int64 {
	switch value := payload[key].(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case int:
		return int64(value)
	case json.Number:
		n, _ := value.Int64()
		return n
	default:
		return 0
	}
}

func isQdrantMissingCollection(err error) bool {
	return err != nil && strings.Contains(err.Error(), "status 404")
}

func shouldManageLocalQdrant(baseURL, rawConfiguredURL string) bool {
	baseURL = strings.TrimSpace(baseURL)
	if strings.TrimSpace(rawConfiguredURL) == "" {
		return baseURL == defaultQdrantURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	return (host == "127.0.0.1" || host == "localhost") && (port == "" || port == defaultQdrantPort)
}
