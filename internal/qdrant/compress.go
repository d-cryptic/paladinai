// compress.go implements extractive compression (Stage 6 §19) and freshness
// decay scoring (Stage 6 §20) for the RAG retrieval pipeline.
//
// ExtractiveCompress reduces synthesis input by 60-70% by keeping only the
// highest-BM25-scoring sentences per chunk. FreshnessScore applies an
// exponential time-decay penalty to retrieved documents.
package qdrant

import (
	"math"
	"strings"
	"time"
	"unicode"
)

// ─── Extractive Compression ────────────────────────────────────────────────────

// CompressConfig controls extractive compression behaviour.
type CompressConfig struct {
	// MinTokens is the minimum chunk length (in approximate tokens) below
	// which compression is skipped. Default: 200.
	MinTokens int
	// MaxOutputTokens is the approximate token budget for the compressed output.
	// Default: 200.
	MaxOutputTokens int
	// Context is the number of surrounding sentences to include on each side
	// of a selected sentence. Default: 1.
	Context int
}

// DefaultCompressConfig is the spec-recommended configuration.
var DefaultCompressConfig = CompressConfig{
	MinTokens:       200,
	MaxOutputTokens: 200,
	Context:         1,
}

// ExtractiveCompress compresses rawText by extracting the highest-BM25-scoring
// sentences relative to query. Returns rawText unchanged when:
//   - rawText is shorter than cfg.MinTokens approximate tokens, OR
//   - rawText has ≤ 3 sentences (too little to compress meaningfully).
//
// The returned string is always valid (never empty when input is non-empty).
func ExtractiveCompress(query, rawText string, cfg CompressConfig) string {
	if cfg.MinTokens <= 0 {
		cfg.MinTokens = DefaultCompressConfig.MinTokens
	}
	if cfg.MaxOutputTokens <= 0 {
		cfg.MaxOutputTokens = DefaultCompressConfig.MaxOutputTokens
	}
	if cfg.Context < 0 {
		cfg.Context = DefaultCompressConfig.Context
	}

	approxTokens := approxTokenCount(rawText)
	if approxTokens < cfg.MinTokens {
		return rawText
	}

	sentences := splitSentences(rawText)
	if len(sentences) <= 3 {
		return rawText
	}

	// Build BM25-style term-frequency scores for each sentence.
	queryTerms := tokenise(query)
	scores := make([]float64, len(sentences))
	for i, s := range sentences {
		scores[i] = bm25Score(queryTerms, s)
	}

	// Greedy selection: pick top-scoring sentence indices within token budget.
	selected := make([]bool, len(sentences))
	tokenBudget := cfg.MaxOutputTokens
	for {
		best, bestScore := -1, -1.0
		for i, s := range scores {
			if !selected[i] && s > bestScore {
				best, bestScore = i, s
			}
		}
		if best == -1 || bestScore == 0 {
			break
		}
		selected[best] = true
		scores[best] = -1 // mark as used

		// Expand context window.
		for d := 1; d <= cfg.Context; d++ {
			if best-d >= 0 {
				selected[best-d] = true
			}
			if best+d < len(sentences) {
				selected[best+d] = true
			}
		}

		// Re-estimate remaining budget.
		var used int
		for i, s := range sentences {
			if selected[i] {
				used += approxTokenCount(s)
			}
		}
		if used >= tokenBudget {
			break
		}
	}

	var parts []string
	for i, s := range sentences {
		if selected[i] && strings.TrimSpace(s) != "" {
			parts = append(parts, strings.TrimSpace(s))
		}
	}
	if len(parts) == 0 {
		return rawText
	}
	return strings.Join(parts, " ")
}

// approxTokenCount estimates token count as word-count × 1.3 (sub-word factor).
func approxTokenCount(text string) int {
	words := strings.Fields(text)
	return int(float64(len(words)) * 1.3)
}

// splitSentences splits text into sentences on [.!?\n] boundaries.
func splitSentences(text string) []string {
	var sentences []string
	var buf strings.Builder
	for _, r := range text {
		buf.WriteRune(r)
		if r == '.' || r == '!' || r == '?' || r == '\n' {
			s := strings.TrimSpace(buf.String())
			if s != "" {
				sentences = append(sentences, s)
			}
			buf.Reset()
		}
	}
	if tail := strings.TrimSpace(buf.String()); tail != "" {
		sentences = append(sentences, tail)
	}
	return sentences
}

// tokenise lowercases and splits text on non-alphanumeric runes.
func tokenise(text string) []string {
	lower := strings.ToLower(text)
	return strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// bm25Score computes an approximate BM25 score for sentence against queryTerms.
// k1=1.5, b=0.75, avgDocLen=50 (fixed approximation).
func bm25Score(queryTerms []string, sentence string) float64 {
	const (
		k1       = 1.5
		b        = 0.75
		avgDocLen = 50
	)
	sentTerms := tokenise(sentence)
	docLen := float64(len(sentTerms))
	tf := termFrequencies(sentTerms)

	var score float64
	for _, qt := range queryTerms {
		f := float64(tf[qt])
		if f == 0 {
			continue
		}
		idf := 1.0 // simplified: assume moderate IDF for all query terms
		tfNorm := f * (k1 + 1) / (f + k1*(1-b+b*docLen/avgDocLen))
		score += idf * tfNorm
	}
	return score
}

func termFrequencies(terms []string) map[string]int {
	m := make(map[string]int, len(terms))
	for _, t := range terms {
		m[t]++
	}
	return m
}

// ─── Freshness Decay Scoring ───────────────────────────────────────────────────

// DocType controls freshness decay rate (half-life varies per document type).
type DocType string

const (
	DocTypeRunbook      DocType = "runbook"      // half-life ~14 days
	DocTypeArchitecture DocType = "architecture" // half-life ~70 days
	DocTypePostmortem   DocType = "postmortem"   // no decay — historical record
)

// decayLambda maps document type to the exponential decay constant λ.
// freshness = exp(-λ × days_since_modified).
var decayLambda = map[DocType]float64{
	DocTypeRunbook:      0.05,  // half-life ≈ 14 days
	DocTypeArchitecture: 0.01,  // half-life ≈ 70 days
	DocTypePostmortem:   0.0,   // no decay
}

// FreshnessScore computes the freshness multiplier for a document.
// Returns a value in [0, 1] where 1.0 = just updated, approaching 0 for old docs.
// For unknown DocTypes the runbook lambda is used as a conservative default.
func FreshnessScore(docType DocType, lastModified time.Time) float64 {
	λ, ok := decayLambda[docType]
	if !ok {
		λ = decayLambda[DocTypeRunbook]
	}
	if λ == 0 {
		return 1.0 // no decay
	}
	daysSince := time.Since(lastModified).Hours() / 24
	if daysSince < 0 {
		daysSince = 0
	}
	return math.Exp(-λ * daysSince)
}

// ApplyFreshness blends reranker score with freshness using the spec weights:
//
//	finalScore = 0.85 × rerankerScore + 0.15 × freshnessScore
func ApplyFreshness(rerankerScore float32, docType DocType, lastModified time.Time) float32 {
	fresh := FreshnessScore(docType, lastModified)
	return 0.85*rerankerScore + float32(0.15*fresh)
}
