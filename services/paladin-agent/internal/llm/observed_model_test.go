package llm

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type observedStubModel struct {
	response *schema.Message
	err      error
	tools    []*schema.ToolInfo
}

func (m *observedStubModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return m.response, m.err
}

func (m *observedStubModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, m.err
}

func (m *observedStubModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &observedStubModel{response: m.response, err: m.err, tools: tools}, nil
}

func TestObservedModelGeneratePassesThroughResponse(t *testing.T) {
	resp := schema.AssistantMessage("ok", nil)
	resp.ResponseMeta = &schema.ResponseMeta{
		FinishReason: "stop",
		Usage: &schema.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}
	m := newObservedModel(&observedStubModel{response: resp}, TierB, "model-b")

	got, err := m.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	require.NoError(t, err)
	assert.Equal(t, resp, got)
}

func TestObservedModelGeneratePassesThroughError(t *testing.T) {
	want := errors.New("provider down")
	m := newObservedModel(&observedStubModel{err: want}, TierA, "model-a")

	got, err := m.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	assert.Nil(t, got)
	assert.ErrorIs(t, err, want)
}

func TestObservedModelWithToolsKeepsWrapper(t *testing.T) {
	base := &observedStubModel{response: schema.AssistantMessage("ok", nil)}
	m := newObservedModel(base, TierC, "model-c")

	withTools, err := m.WithTools([]*schema.ToolInfo{{Name: "query_metrics"}})
	require.NoError(t, err)
	_, ok := withTools.(*observedModel)
	assert.True(t, ok)
}
