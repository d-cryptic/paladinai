package qdrant

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// RunbookChunk is a chunk of a runbook document.
type RunbookChunk struct {
	ID         string
	Source     string
	Title      string
	Content    string
	ChunkIndex int
	TenantID   string
	Tags       []string
}

// ToPoint converts a chunk to a Qdrant Point. vector must be supplied by an
// embedding model.
func (c *RunbookChunk) ToPoint(vector []float32) Point {
	return Point{
		ID:     c.ID,
		Vector: vector,
		Payload: map[string]any{
			"source":      c.Source,
			"title":       c.Title,
			"content":     c.Content,
			"chunk_index": c.ChunkIndex,
			"tenant_id":   c.TenantID,
			"tags":        c.Tags,
		},
	}
}

// ChunkID generates a stable ID from source and chunk index.
func ChunkID(source string, chunkIndex int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", source, chunkIndex)))
	return fmt.Sprintf("%x", h[:16])
}

// ChunkText splits text into overlapping ~500-word chunks with 50-word overlap.
// Returns nil when text is empty.
func ChunkText(source, title, tenantID, text string, tags []string) []RunbookChunk {
	const chunkWords = 500
	const overlapWords = 50

	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var chunks []RunbookChunk
	start := 0
	idx := 0
	for start < len(words) {
		end := start + chunkWords
		if end > len(words) {
			end = len(words)
		}
		content := strings.Join(words[start:end], " ")
		chunks = append(chunks, RunbookChunk{
			ID:         ChunkID(source, idx),
			Source:     source,
			Title:      title,
			Content:    content,
			ChunkIndex: idx,
			TenantID:   tenantID,
			Tags:       tags,
		})
		if end == len(words) {
			break
		}
		start = end - overlapWords
		if start < 0 {
			start = 0
		}
		idx++
	}
	return chunks
}
