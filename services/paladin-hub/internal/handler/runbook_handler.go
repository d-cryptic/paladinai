package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/internal/qdrant"
	"go.uber.org/zap"
)

const maxRunbookContentBytes = 2 << 20

// RunbookRecord is the metadata record returned by list and import.
type RunbookRecord struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Source    string    `json:"source"`
	Path      string    `json:"path,omitempty"`
	Embedded  bool      `json:"embedded"`
	Chunks    int       `json:"chunks"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RunbookHandler serves runbook import, list, and search routes.
// It is intentionally separate from the MCP server registry handler.
type RunbookHandler struct {
	indexer    *qdrant.Indexer
	log        *zap.Logger
	httpClient *http.Client
	// records is a simple in-memory index of imported runbooks.
	// Production would persist this to Postgres.
	records map[string]*RunbookRecord
}

// NewRunbookHandler constructs a RunbookHandler backed by a qdrant.Indexer.
func NewRunbookHandler(indexer *qdrant.Indexer, log *zap.Logger) *RunbookHandler {
	return &RunbookHandler{
		indexer:    indexer,
		log:        log,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		records:    make(map[string]*RunbookRecord),
	}
}

// RunbookRoutes mounts runbook routes on the given chi router subtree.
func (h *RunbookHandler) RunbookRoutes(r chi.Router) {
	r.Post("/runbooks/import", h.importRunbook)
	r.Get("/runbooks", h.listRunbooks)
	r.Post("/runbooks/search", h.searchRunbooks)
}

type importRequest struct {
	Source string `json:"source"`
	Repo   string `json:"repo,omitempty"`
	Path   string `json:"path,omitempty"`
	Space  string `json:"space,omitempty"`
}

func (h *RunbookHandler) importRunbook(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if err := alert.ValidateTenantID(tenantID); err != nil {
		jsonErr(w, "INVALID_TENANT", "missing or invalid X-Tenant-ID header", http.StatusBadRequest)
		return
	}

	var req importRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "INVALID_BODY", "request body must be valid JSON", http.StatusBadRequest)
		return
	}
	if req.Source == "" {
		jsonErr(w, "VALIDATION_FAILED", "source is required", http.StatusUnprocessableEntity)
		return
	}

	text, title, srcPath, err := h.fetchContent(r.Context(), req)
	if err != nil {
		jsonErr(w, "FETCH_FAILED", err.Error(), http.StatusBadGateway)
		return
	}

	now := time.Now().UTC()
	rec := &RunbookRecord{
		ID:        uuid.New().String(),
		Title:     title,
		Source:    req.Source,
		Path:      srcPath,
		Embedded:  false,
		Chunks:    0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	n, err := h.indexer.Index(r.Context(), srcPath, title, tenantID, text, []string{req.Source})
	if err != nil {
		h.log.Error("runbook index failed", zap.String("source", req.Source), zap.Error(err))
		// Still record the import, mark as not embedded.
	} else {
		rec.Embedded = true
		rec.Chunks = n
	}

	h.records[rec.ID] = rec
	h.log.Info("runbook imported",
		zap.String("id", rec.ID),
		zap.String("tenant", tenantID),
		zap.String("source", req.Source),
		zap.Int("chunks", n),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"imported": 1,
		"job_id":   "",
		"message":  "runbook indexed synchronously",
		"data":     rec,
	})
}

func (h *RunbookHandler) listRunbooks(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if err := alert.ValidateTenantID(tenantID); err != nil {
		jsonErr(w, "INVALID_TENANT", "missing or invalid X-Tenant-ID header", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}
	sourceFilter := r.URL.Query().Get("source")

	out := make([]*RunbookRecord, 0, len(h.records))
	for _, rec := range h.records {
		if sourceFilter != "" && rec.Source != sourceFilter {
			continue
		}
		out = append(out, rec)
		if len(out) >= limit {
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": out, "total": len(out)})
}

type searchRequest struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k"`
}

func (h *RunbookHandler) searchRunbooks(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if err := alert.ValidateTenantID(tenantID); err != nil {
		jsonErr(w, "INVALID_TENANT", "missing or invalid X-Tenant-ID header", http.StatusBadRequest)
		return
	}

	var req searchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "INVALID_BODY", "request body must be valid JSON", http.StatusBadRequest)
		return
	}
	if req.Query == "" {
		jsonErr(w, "VALIDATION_FAILED", "query is required", http.StatusUnprocessableEntity)
		return
	}
	if req.TopK <= 0 {
		req.TopK = 5
	}

	chunks, err := h.indexer.Search(r.Context(), req.Query, tenantID, req.TopK)
	if err != nil {
		h.log.Error("runbook search failed", zap.Error(err))
		jsonErr(w, "SEARCH_FAILED", "semantic search failed", http.StatusInternalServerError)
		return
	}

	type result struct {
		ID      string  `json:"id"`
		Title   string  `json:"title"`
		Source  string  `json:"source"`
		Score   float64 `json:"score"`
		Excerpt string  `json:"excerpt"`
	}
	results := make([]result, 0, len(chunks))
	for i, c := range chunks {
		excerpt := c.Content
		if len(excerpt) > 200 {
			excerpt = excerpt[:200] + "..."
		}
		results = append(results, result{
			ID:      c.ID,
			Title:   c.Title,
			Source:  c.Source,
			Score:   float64(len(chunks)-i) / float64(len(chunks)),
			Excerpt: excerpt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": results, "total": len(results)})
}

// fetchContent resolves runbook content based on source type.
func (h *RunbookHandler) fetchContent(ctx context.Context, req importRequest) (text, title, srcPath string, err error) {
	switch req.Source {
	case "file":
		if req.Path == "" {
			return "", "", "", errMsg("path is required for source=file")
		}
		data, readErr := os.ReadFile(req.Path)
		if readErr != nil {
			return "", "", "", readErr
		}
		title = req.Path
		if idx := lastSlash(req.Path); idx >= 0 {
			title = req.Path[idx+1:]
		}
		return string(data), title, req.Path, nil
	case "github":
		if req.Repo == "" {
			return "", "", "", errors.New("repo is required for source=github")
		}
		title = req.Repo
		if req.Path != "" {
			title = req.Repo + "/" + req.Path
		}
		srcPath, err = githubRunbookURL(req)
		if err != nil {
			return "", "", "", err
		}
		text, err = h.fetchRemoteText(ctx, srcPath)
		if err != nil {
			return "", "", "", err
		}
		return text, title, srcPath, nil
	case "confluence":
		title = "Confluence:" + req.Space
		srcPath, err = remoteRunbookURL(req.Space, req.Path, "space is required as an HTTP URL for source=confluence")
		if err != nil {
			return "", "", "", err
		}
		text, err = h.fetchRemoteText(ctx, srcPath)
		if err != nil {
			return "", "", "", err
		}
		return text, title, srcPath, nil
	case "notion":
		title = "Notion:" + req.Space
		srcPath, err = remoteRunbookURL(req.Space, req.Path, "space is required as an HTTP URL for source=notion")
		if err != nil {
			return "", "", "", err
		}
		text, err = h.fetchRemoteText(ctx, srcPath)
		if err != nil {
			return "", "", "", err
		}
		return text, title, srcPath, nil
	default:
		return "", "", "", errMsg("unsupported source: " + req.Source)
	}
}
func githubRunbookURL(req importRequest) (string, error) {
	if isHTTPURL(req.Repo) {
		return remoteRunbookURL(req.Repo, req.Path, "repo must be an HTTP URL or owner/repo")
	}
	parts := strings.Split(req.Repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", errors.New("repo must be owner/repo for source=github")
	}
	if req.Path == "" {
		return "", errors.New("path is required for source=github when repo is owner/repo")
	}
	return url.JoinPath("https://raw.githubusercontent.com", parts[0], parts[1], "main", req.Path)
}

func remoteRunbookURL(base, path, missingMsg string) (string, error) {
	if base == "" {
		return "", errors.New(missingMsg)
	}
	if !isHTTPURL(base) {
		return "", errors.New(missingMsg)
	}
	if path == "" {
		return strings.TrimRight(base, "/"), nil
	}
	joined, err := url.JoinPath(base, path)
	if err != nil {
		return "", fmt.Errorf("invalid remote path: %w", err)
	}
	return joined, nil
}

func isHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func (h *RunbookHandler) fetchRemoteText(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("build remote request: %w", err)
	}
	req.Header.Set("Accept", "text/markdown,text/plain,text/*;q=0.9,*/*;q=0.1")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch remote runbook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch remote runbook: status %d", resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, maxRunbookContentBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("read remote runbook: %w", err)
	}
	if len(data) > maxRunbookContentBytes {
		return "", fmt.Errorf("remote runbook exceeds %d bytes", maxRunbookContentBytes)
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return "", errors.New("remote runbook is empty")
	}
	return text, nil
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' || s[i] == '\\' {
			return i
		}
	}
	return -1
}

type strErr string

func (e strErr) Error() string { return string(e) }

func errMsg(s string) error { return strErr(s) }
