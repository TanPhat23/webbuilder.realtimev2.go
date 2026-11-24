package entity

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPayloadPresence_WithAgent(t *testing.T) {
	elID := "el-hero-1"
	orig := PayloadPresence{
		UserID:    "user-1",
		UserName:  "Alice",
		CursorX:   120.5,
		CursorY:   240.25,
		ElementID: "el-selected",
		Agent: &AgentPresence{
			Active:    true,
			Name:      "Canvas Copilot",
			Status:    "editing",
			ElementID: &elID,
			Action:    "Updating hero title",
		},
	}

	raw, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal PayloadPresence: %v", err)
	}

	var decoded PayloadPresence
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal PayloadPresence: %v", err)
	}

	if decoded.Agent == nil {
		t.Fatalf("expected Agent to be non-nil after round-trip")
	}
	if !decoded.Agent.Active || decoded.Agent.Name != "Canvas Copilot" || decoded.Agent.Status != "editing" || decoded.Agent.Action != "Updating hero title" {
		t.Fatalf("unexpected Agent fields: %+v", decoded.Agent)
	}
	if decoded.Agent.ElementID == nil || *decoded.Agent.ElementID != elID {
		t.Fatalf("expected Agent.ElementID=%q, got %v", elID, decoded.Agent.ElementID)
	}
}

func TestPayloadPresence_NilAgentOmitted(t *testing.T) {
	orig := PayloadPresence{
		UserID:   "user-1",
		UserName: "Alice",
		CursorX:  10,
		CursorY:  20,
	}

	raw, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal PayloadPresence: %v", err)
	}

	if strings.Contains(string(raw), `"agent"`) {
		t.Fatalf("expected \"agent\" key to be omitted when nil, got %s", string(raw))
	}

	// Backward compatibility: unmarshaling legacy payload without "agent" succeeds with Agent == nil.
	legacyJSON := `{"userId":"user-2","userName":"Bob","cursorX":42,"cursorY":84}`
	var decoded PayloadPresence
	if err := json.Unmarshal([]byte(legacyJSON), &decoded); err != nil {
		t.Fatalf("unmarshal legacy PayloadPresence: %v", err)
	}
	if decoded.Agent != nil {
		t.Fatalf("expected nil Agent for legacy payload, got %+v", decoded.Agent)
	}
}

func TestPresence_WithAgentAndOmitEmpty(t *testing.T) {
	elID := "el-card-2"
	agentElID := "el-btn-3"
	withAgent := Presence{
		CursorX:   15,
		CursorY:   30,
		ElementID: &elID,
		Agent: &AgentPresence{
			Active:    true,
			Name:      "Layout Agent",
			Status:    "thinking",
			ElementID: &agentElID,
		},
	}

	raw, err := json.Marshal(withAgent)
	if err != nil {
		t.Fatalf("marshal Presence with agent: %v", err)
	}

	var decoded Presence
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal Presence with agent: %v", err)
	}
	if decoded.Agent == nil || !decoded.Agent.Active || decoded.Agent.Name != "Layout Agent" || decoded.Agent.Status != "thinking" {
		t.Fatalf("unexpected decoded Agent: %+v", decoded.Agent)
	}
	if decoded.Agent.ElementID == nil || *decoded.Agent.ElementID != agentElID {
		t.Fatalf("expected Agent.ElementID=%q, got %v", agentElID, decoded.Agent.ElementID)
	}
	if strings.Contains(string(raw), `"action"`) {
		t.Fatalf("expected empty Action to be omitted, got %s", string(raw))
	}

	withoutAgent := Presence{
		CursorX: 1,
		CursorY: 2,
	}
	rawNoAgent, err := json.Marshal(withoutAgent)
	if err != nil {
		t.Fatalf("marshal Presence without agent: %v", err)
	}
	if strings.Contains(string(rawNoAgent), `"agent"`) {
		t.Fatalf("expected \"agent\" key to be omitted when nil, got %s", string(rawNoAgent))
	}
}
