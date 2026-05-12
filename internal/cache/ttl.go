// Package cache: ttl.go implements adaptive TTL computation that weights cache
// lifetime by the token cost saved per cache hit. Expensive Opus responses get
// longer TTL (more value preserved); trivially cheap responses evict sooner.
package cache

import "time"

// QueryType classifies the nature of an LLM request for base TTL selection.
type QueryType string

const (
	// QueryTypeClassification is a short intent-classification call (fast path).
	QueryTypeClassification QueryType = "classification"
	// QueryTypeTriage is a full triage analysis call (moderate cost).
	QueryTypeTriage QueryType = "triage"
	// QueryTypeRCA is a deep root-cause analysis call (expensive).
	QueryTypeRCA QueryType = "rca"
	// QueryTypeRunbook is a runbook step execution call.
	QueryTypeRunbook QueryType = "runbook"
	// QueryTypeComms is a communication draft call.
	QueryTypeComms QueryType = "comms"
)

// TTLForQueryType returns the base cache TTL for a given query type.
// These base values are scaled by the cost multiplier in ComputeTTL.
func TTLForQueryType(qt QueryType) time.Duration {
	switch qt {
	case QueryTypeClassification:
		return 30 * time.Minute
	case QueryTypeTriage:
		return 2 * time.Hour
	case QueryTypeRCA:
		return 4 * time.Hour
	case QueryTypeRunbook:
		return 1 * time.Hour
	case QueryTypeComms:
		return 30 * time.Minute
	default:
		return 1 * time.Hour
	}
}

// TTLRequest holds the parameters needed to compute an adaptive TTL.
type TTLRequest struct {
	QueryType    QueryType
	InputTokens  int
	InputPrice   float64 // USD per million input tokens
	OutputTokens int
	OutputPrice  float64 // USD per million output tokens
}

// ComputeTTL returns the adaptive cache TTL for a given LLM request/response.
// It scales the base TTL for the query type by a cost multiplier:
//
//	totalCost ≥ $0.05  → 4× multiplier (Opus, large output)
//	totalCost ≥ $0.01  → 2× multiplier (Sonnet, normal output)
//	totalCost ≥ $0.001 → 1× multiplier (Qwen, short output)
//	otherwise          → 0.25× multiplier (trivially cheap, evict sooner)
//
// Result is clamped to [10 minutes, 16 hours].
func ComputeTTL(req TTLRequest) time.Duration {
	baseTTL := TTLForQueryType(req.QueryType)

	inputCost := float64(req.InputTokens) / 1e6 * req.InputPrice
	outputCost := float64(req.OutputTokens) / 1e6 * req.OutputPrice
	totalCost := inputCost + outputCost

	var multiplier float64
	switch {
	case totalCost >= 0.05:
		multiplier = 4.0
	case totalCost >= 0.01:
		multiplier = 2.0
	case totalCost >= 0.001:
		multiplier = 1.0
	default:
		multiplier = 0.25
	}

	ttl := time.Duration(float64(baseTTL) * multiplier)
	if ttl < 10*time.Minute {
		ttl = 10 * time.Minute
	}
	if ttl > 16*time.Hour {
		ttl = 16 * time.Hour
	}
	return ttl
}

// ─── Prompt cache breakpoint helpers ─────────────────────────────────────────

// CacheTypeEphemeral is the only Anthropic prompt cache type in the public API.
const CacheTypeEphemeral = "ephemeral"

// CacheControl is the Anthropic cache_control ephemeral marker.
// Injected at system prompt end and tool catalog end.
type CacheControl struct {
	Type string `json:"type"` // always "ephemeral"
}

// EphemeralCacheControl is the singleton cache control value for prompt caching.
var EphemeralCacheControl = &CacheControl{Type: "ephemeral"}

// ContentBlock represents an Anthropic content block with optional cache control.
// Used to construct the system prompt and tool catalog in BuildAnthropicRequest.
type ContentBlock struct {
	Type         string        `json:"type"`
	Text         string        `json:"text,omitempty"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// ToolEntry represents one tool in the Anthropic tool catalog.
// The last entry in the catalog carries the cache_control breakpoint.
type ToolEntry struct {
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	InputSchema  any           `json:"input_schema"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// BuildSystemBlocks returns the Anthropic system content blocks for a given
// system prompt, with a cache_control breakpoint at the end.
func BuildSystemBlocks(systemPrompt string) []ContentBlock {
	return []ContentBlock{
		{Type: "text", Text: systemPrompt, CacheControl: EphemeralCacheControl},
	}
}

// AppendToolCacheControl adds a cache_control breakpoint to the last tool in the catalog.
// The input slice is NOT modified; a shallow copy is returned.
// If tools is empty the original slice is returned unchanged.
func AppendToolCacheControl(tools []ToolEntry) []ToolEntry {
	if len(tools) == 0 {
		return tools
	}
	out := make([]ToolEntry, len(tools))
	copy(out, tools)
	out[len(out)-1].CacheControl = EphemeralCacheControl
	return out
}

// ─── Chat message cache injection ─────────────────────────────────────────────

// Anthropic role constants for chat messages.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// ChatMessage is one turn in an Anthropic chat completion request.
// Content uses []ContentBlock so cache_control can be attached per block.
type ChatMessage struct {
	Role    string         `json:"role"` // RoleUser | RoleAssistant
	Content []ContentBlock `json:"content"`
}

// InjectMessageCacheBreakpoints marks the last user message in msgs for
// ephemeral caching. It deep-copies so the caller's slice is not mutated.
// The returned slice should be used in the Anthropic messages field alongside
// BuildSystemBlocks for the system field.
func InjectMessageCacheBreakpoints(msgs []ChatMessage) []ChatMessage {
	out := make([]ChatMessage, len(msgs))
	for i, m := range msgs {
		blocks := make([]ContentBlock, len(m.Content))
		copy(blocks, m.Content)
		out[i] = ChatMessage{Role: m.Role, Content: blocks}
	}
	for i := len(out) - 1; i >= 0; i-- {
		if out[i].Role == RoleUser && len(out[i].Content) > 0 {
			last := len(out[i].Content) - 1
			out[i].Content[last].CacheControl = EphemeralCacheControl
			break
		}
	}
	return out
}

// PlainChatMessages converts (role, text) pairs to []ChatMessage. Convenience
// helper for callers that build prompts from strings.
func PlainChatMessages(pairs [][2]string) []ChatMessage {
	msgs := make([]ChatMessage, 0, len(pairs))
	for _, p := range pairs {
		msgs = append(msgs, ChatMessage{
			Role:    p[0],
			Content: []ContentBlock{{Type: "text", Text: p[1]}},
		})
	}
	return msgs
}

// TokenSavingsEstimate returns the estimated tokens saved by a prompt cache
// hit on the prefix up to the injected breakpoint. Uses 80% as the heuristic
// fraction of a typical prompt that is cacheable prefix. Returns 0 for non-positive input.
func TokenSavingsEstimate(tokenCount int) int {
	if tokenCount <= 0 {
		return 0
	}
	return int(float64(tokenCount) * 0.8)
}
