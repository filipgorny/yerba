package tui

import (
	"testing"

	"github.com/filipgorny/agent/stream"

	"github.com/filipgorny/yerba/internal/config"
)

func TestHandleStreamFilterAndModes(t *testing.T) {
	m := newModel(nil, config.UIConfig{LogSubtypes: []string{stream.LogToolCall}})

	(&m).handleStream(stream.Message{Type: stream.TypeLog, Subtype: stream.LogToolCall, Payload: map[string]any{"action": "grep"}})
	(&m).handleStream(stream.Message{Type: stream.TypeLog, Subtype: stream.LogReasoning, Payload: "x"})

	if len(m.lines) != 1 {
		t.Errorf("expected 1 line (REASONING filtered out), got %d", len(m.lines))
	}

	(&m).handleStream(stream.Message{
		Type:    stream.TypeAskUser,
		Subtype: stream.SubtypeChoice,
		Payload: map[string]any{"question": "Pick", "choices": []string{"a", "b"}},
	})

	if m.mode != modeChoice || len(m.choices) != 2 {
		t.Errorf("choice mode not set: mode=%d choices=%v", m.mode, m.choices)
	}

	(&m).handleStream(stream.Message{Type: stream.TypeAnswerUser, Payload: "done"})

	if m.mode != modeNormal {
		t.Errorf("ANSWER_USER should reset to normal mode, got %d", m.mode)
	}
}

func TestAskPayloadAnySlice(t *testing.T) {
	q, choices := askPayload(map[string]any{"question": "Q", "choices": []any{"x", "y"}})

	if q != "Q" || len(choices) != 2 || choices[0] != "x" {
		t.Errorf("q=%q choices=%v", q, choices)
	}
}
