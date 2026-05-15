package llm

import (
	"context"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/paladinai/paladinai/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
)

var (
	llmTracer          = otel.Tracer("github.com/paladinai/paladinai/services/paladin-agent/internal/llm")
	llmMeter           = otel.Meter("github.com/paladinai/paladinai/services/paladin-agent/internal/llm")
	llmCallsCounter    metric.Int64Counter
	llmFailuresCounter metric.Int64Counter
	llmDuration        metric.Float64Histogram
	llmTokenCounter    metric.Int64Counter
	llmInstrumentErr   error
)

func init() {
	llmCallsCounter, llmInstrumentErr = llmMeter.Int64Counter(
		"paladin_llm_calls_total",
		metric.WithDescription("Total LLM calls by provider, tier, and model"),
	)
	if llmInstrumentErr != nil {
		return
	}
	llmFailuresCounter, llmInstrumentErr = llmMeter.Int64Counter(
		"paladin_llm_failures_total",
		metric.WithDescription("Total failed LLM calls by provider, tier, and model"),
	)
	if llmInstrumentErr != nil {
		return
	}
	llmDuration, llmInstrumentErr = llmMeter.Float64Histogram(
		"paladin_llm_duration_seconds",
		metric.WithDescription("LLM call duration"),
		metric.WithUnit("s"),
	)
	if llmInstrumentErr != nil {
		return
	}
	llmTokenCounter, llmInstrumentErr = llmMeter.Int64Counter(
		"paladin_llm_tokens_total",
		metric.WithDescription("LLM token usage by provider, tier, model, and token type"),
	)
}

type observedModel struct {
	inner     model.ToolCallingChatModel
	tier      Tier
	modelName string
}

func newObservedModel(inner model.ToolCallingChatModel, tier Tier, modelName string) model.ToolCallingChatModel {
	return &observedModel{inner: inner, tier: tier, modelName: modelName}
}

func (m *observedModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	ctx, span := llmTracer.Start(ctx, "paladin.llm.generate")
	defer span.End()

	attrs := m.attrs()
	span.SetAttributes(attrs...)
	start := time.Now()
	resp, err := m.inner.Generate(ctx, input, opts...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if llmTelemetryReady() {
			llmFailuresCounter.Add(ctx, 1, telemetry.Attrs(attrs...))
		}
		return nil, err
	}

	if resp != nil && resp.ResponseMeta != nil {
		setResponseAttrs(span, resp.ResponseMeta)
		recordUsage(ctx, attrs, resp.ResponseMeta.Usage)
	}
	if llmTelemetryReady() {
		llmCallsCounter.Add(ctx, 1, telemetry.Attrs(attrs...))
		llmDuration.Record(ctx, time.Since(start).Seconds(), telemetry.Attrs(attrs...))
	}
	return resp, nil
}

func (m *observedModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	ctx, span := llmTracer.Start(ctx, "paladin.llm.stream")
	defer span.End()

	attrs := append(m.attrs(), attribute.Bool("gen_ai.response.streaming", true))
	span.SetAttributes(attrs...)
	reader, err := m.inner.Stream(ctx, input, opts...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if llmTelemetryReady() {
			llmFailuresCounter.Add(ctx, 1, telemetry.Attrs(attrs...))
		}
		return nil, err
	}
	if llmTelemetryReady() {
		llmCallsCounter.Add(ctx, 1, telemetry.Attrs(attrs...))
	}
	return reader, nil
}

func (m *observedModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	withTools, err := m.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &observedModel{inner: withTools, tier: m.tier, modelName: m.modelName}, nil
}

func (m *observedModel) attrs() []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("gen_ai.system", "openrouter"),
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("gen_ai.request.model", m.modelName),
		attribute.String("gen_ai.response.model", m.modelName),
		attribute.String("langfuse.observation.type", "generation"),
		attribute.String("paladin.llm_tier", string(m.tier)),
	}
}

func setResponseAttrs(span traceSetter, meta *schema.ResponseMeta) {
	if meta == nil {
		return
	}
	if meta.FinishReason != "" {
		span.SetAttributes(attribute.String("gen_ai.response.finish_reasons", meta.FinishReason))
	}
	if meta.Usage != nil {
		span.SetAttributes(
			attribute.Int("gen_ai.usage.input_tokens", meta.Usage.PromptTokens),
			attribute.Int("gen_ai.usage.output_tokens", meta.Usage.CompletionTokens),
			attribute.Int("gen_ai.usage.total_tokens", meta.Usage.TotalTokens),
		)
	}
}

func recordUsage(ctx context.Context, attrs []attribute.KeyValue, usage *schema.TokenUsage) {
	if !llmTelemetryReady() || usage == nil {
		return
	}
	recordTokens(ctx, attrs, "input", usage.PromptTokens)
	recordTokens(ctx, attrs, "output", usage.CompletionTokens)
	recordTokens(ctx, attrs, "total", usage.TotalTokens)
}

func recordTokens(ctx context.Context, attrs []attribute.KeyValue, kind string, count int) {
	if count <= 0 {
		return
	}
	llmTokenCounter.Add(ctx, int64(count), telemetry.Attrs(append(attrs, attribute.String("token_type", kind))...))
}

func llmTelemetryReady() bool {
	return llmInstrumentErr == nil
}

type traceSetter interface {
	SetAttributes(...attribute.KeyValue)
}
