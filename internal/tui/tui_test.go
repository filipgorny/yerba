package tui

import (
	"testing"

	"github.com/filipgorny/agent/stream"

	"github.com/filipgorny/yerba/internal/config"
)

func TestHandleStreamFilterAndModes(t *testing.T) {
	m := newModel(nil, config.UIConfig{LogSubtypes: []string{stream.LogToolCall}})

	(&m).handleStream(stream.Record{Type: stream.TypeLog, Subtype: stream.LogToolCall, Payload: map[string]any{"action": "grep"}})
	(&m).handleStream(stream.Record{Type: stream.TypeLog, Subtype: stream.LogReasoning, Payload: "x"})

	if len(m.blocks) != 1 {
		t.Errorf("expected 1 block (REASONING filtered out), got %d", len(m.blocks))
	}

	(&m).handleStream(stream.Record{
		Type:    stream.TypeAskUser,
		Subtype: stream.SubtypeChoice,
		Payload: map[string]any{"question": "Pick", "choices": []string{"a", "b"}},
	})

	if m.mode != modeChoice || len(m.choices) != 2 {
		t.Errorf("choice mode not set: mode=%d choices=%v", m.mode, m.choices)
	}

	(&m).handleStream(stream.Record{Type: stream.TypeAnswerUser, Payload: "done"})

	if m.mode != modeNormal {
		t.Errorf("ANSWER_USER should reset to normal mode, got %d", m.mode)
	}
}

func TestWaitingTracksLLMLifecycle(t *testing.T) {
	m := newModel(nil, config.UIConfig{})

	if m.waiting {
		t.Fatal("should not be waiting before any input")
	}

	(&m).handleStream(stream.Record{Type: stream.TypeStatus, Subtype: stream.StatusLLMRequest})

	if !m.waiting {
		t.Error("LLM_REQUEST should set waiting")
	}

	(&m).handleStream(stream.Record{Type: stream.TypeStatus, Subtype: stream.StatusLLMResponse})

	if m.waiting {
		t.Error("LLM_RESPONSE should clear waiting")
	}

	(&m).handleStream(stream.Record{Type: stream.TypeStatus, Subtype: stream.StatusLLMRequest})
	(&m).handleStream(stream.Record{Type: stream.TypeAnswerUser, Payload: "done"})

	if m.waiting {
		t.Error("ANSWER_USER should clear waiting")
	}
}

func TestLogBodyHumanReadable(t *testing.T) {
	call := logBody(stream.Record{
		Type:    stream.TypeLog,
		Subtype: stream.LogToolCall,
		Payload: map[string]any{"action": "web_get", "params": map[string]any{"url": "https://go.dev"}},
	})

	if call != `tool: web_get, params: {"url":"https://go.dev"}` {
		t.Errorf("tool call body = %q", call)
	}

	res := logBody(stream.Record{
		Type:    stream.TypeLog,
		Subtype: stream.LogToolResult,
		Payload: map[string]any{"action": "web_get", "result": "hello world"},
	})

	if res != "tool web_get result: hello world" {
		t.Errorf("tool result body = %q", res)
	}
}

func TestAskPayloadAnySlice(t *testing.T) {
	q, choices := askPayload(map[string]any{"question": "Q", "choices": []any{"x", "y"}})

	if q != "Q" || len(choices) != 2 || choices[0] != "x" {
		t.Errorf("q=%q choices=%v", q, choices)
	}
}
