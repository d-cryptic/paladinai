package cache

import (
	"testing"
)

func TestInjectMessageCacheBreakpoints_LastUserMessageMarked(t *testing.T) {
	msgs := PlainChatMessages([][2]string{
		{"user", "hello"},
		{"assistant", "hi"},
		{"user", "what is the incident status?"},
	})
	out := InjectMessageCacheBreakpoints(msgs)

	if out[2].Content[0].CacheControl == nil {
		t.Error("last user message should have cache control")
	}
	if out[2].Content[0].CacheControl.Type != CacheTypeEphemeral {
		t.Errorf("cache type = %q, want %q", out[2].Content[0].CacheControl.Type, CacheTypeEphemeral)
	}
	if out[0].Content[0].CacheControl != nil {
		t.Error("first user message should NOT have cache control")
	}
}

func TestInjectMessageCacheBreakpoints_AssistantLast_MarksLatestUser(t *testing.T) {
	msgs := PlainChatMessages([][2]string{
		{"user", "triage this"},
		{"assistant", "working on it"},
	})
	out := InjectMessageCacheBreakpoints(msgs)

	if out[0].Content[0].CacheControl == nil {
		t.Error("user message should be marked even when assistant message follows")
	}
	if out[1].Content[0].CacheControl != nil {
		t.Error("assistant message should not be marked")
	}
}

func TestInjectMessageCacheBreakpoints_Empty(t *testing.T) {
	out := InjectMessageCacheBreakpoints(nil)
	if len(out) != 0 {
		t.Errorf("expected 0 messages, got %d", len(out))
	}
}

func TestInjectMessageCacheBreakpoints_NoUserMessages(t *testing.T) {
	msgs := PlainChatMessages([][2]string{
		{"assistant", "ready"},
	})
	out := InjectMessageCacheBreakpoints(msgs)
	if out[0].Content[0].CacheControl != nil {
		t.Error("assistant message should not be marked")
	}
}

func TestInjectMessageCacheBreakpoints_DoesNotMutateInput(t *testing.T) {
	msgs := PlainChatMessages([][2]string{
		{"user", "original"},
	})
	originalCC := msgs[0].Content[0].CacheControl

	out := InjectMessageCacheBreakpoints(msgs)
	out[0].Content[0].CacheControl = nil

	if msgs[0].Content[0].CacheControl != originalCC {
		t.Error("InjectMessageCacheBreakpoints mutated the input slice")
	}
}

func TestPlainChatMessages_BuildsCorrectly(t *testing.T) {
	msgs := PlainChatMessages([][2]string{
		{"user", "hello"},
		{"assistant", "world"},
	})
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content[0].Text != "hello" {
		t.Errorf("first message wrong: %+v", msgs[0])
	}
}

func TestTokenSavingsEstimate_EightyPercent(t *testing.T) {
	if got := TokenSavingsEstimate(1000); got != 800 {
		t.Errorf("expected 800, got %d", got)
	}
}

func TestTokenSavingsEstimate_Zero(t *testing.T) {
	if TokenSavingsEstimate(0) != 0 {
		t.Error("0 tokens should yield 0 savings")
	}
}

func TestBuildSystemBlocks_HasCacheControl(t *testing.T) {
	blocks := BuildSystemBlocks("You are a triage agent.")
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].CacheControl == nil || blocks[0].CacheControl.Type != CacheTypeEphemeral {
		t.Error("system block should have ephemeral cache control")
	}
}

func TestAppendToolCacheControl_MarksLast(t *testing.T) {
	tools := []ToolEntry{
		{Name: "query_metrics"},
		{Name: "get_logs"},
		{Name: "describe_pod"},
	}
	out := AppendToolCacheControl(tools)
	if out[2].CacheControl == nil {
		t.Error("last tool should have cache control")
	}
	if out[0].CacheControl != nil || out[1].CacheControl != nil {
		t.Error("non-last tools should not have cache control")
	}
}

func TestAppendToolCacheControl_Empty(t *testing.T) {
	out := AppendToolCacheControl(nil)
	if len(out) != 0 {
		t.Errorf("expected empty, got %d", len(out))
	}
}
