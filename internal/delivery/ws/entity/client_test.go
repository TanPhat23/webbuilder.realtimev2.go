package entity

import (
	"encoding/json"
	"testing"

	domainEntity "webbuilder/internal/domain/entity"
)

func TestClient_SetAndGetPresence_AgentLifecycle(t *testing.T) {
	first := "Alice"
	client := NewClient(nil, "proj-1", &domainEntity.User{
		ID:        "user-1",
		FirstName: &first,
	})
	client.SetPageID("page-1")

	agentEl := "el-agent-target"
	payload := domainEntity.PayloadPresence{
		UserID:    "user-1",
		UserName:  "Alice",
		CursorX:   100,
		CursorY:   200,
		ElementID: "el-human-selection",
		Agent: &domainEntity.AgentPresence{
			Active:    true,
			Name:      "Canvas Copilot",
			Status:    "editing",
			ElementID: &agentEl,
			Action:    "Styling header",
		},
	}

	client.SetPresence(payload)

	// Mutating the input pointer after SetPresence must not affect stored state.
	agentEl = "mutated"
	payload.Agent.Name = "Mutated"

	got := client.GetPresence()
	if got.CursorX != 100 || got.CursorY != 200 {
		t.Fatalf("unexpected cursor: (%v, %v)", got.CursorX, got.CursorY)
	}
	if got.ElementID == nil || *got.ElementID != "el-human-selection" {
		t.Fatalf("expected ElementID=el-human-selection, got %v", got.ElementID)
	}
	if got.Agent == nil {
		t.Fatalf("expected Agent to be preserved on GetPresence")
	}
	if !got.Agent.Active || got.Agent.Name != "Canvas Copilot" || got.Agent.Status != "editing" || got.Agent.Action != "Styling header" {
		t.Fatalf("unexpected Agent: %+v", got.Agent)
	}
	if got.Agent.ElementID == nil || *got.Agent.ElementID != "el-agent-target" {
		t.Fatalf("expected Agent.ElementID=el-agent-target, got %v", got.Agent.ElementID)
	}

	// Verify Room.GetParticipants preserves Agent in sync snapshots.
	room := &Room{
		PageID:  client.RoomID(),
		clients: map[*Client]bool{client: true},
	}
	participants := room.GetParticipants()
	if len(participants) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(participants))
	}
	if participants[0].Presence.Agent == nil || participants[0].Presence.Agent.Name != "Canvas Copilot" {
		t.Fatalf("expected participant presence to include Agent, got %+v", participants[0].Presence.Agent)
	}

	// Verify BroadcastPresence preserves Agent in broadcast payload.
	broadcastPayload := domainEntity.PayloadPresence{
		UserID:   "user-1",
		UserName: "Alice",
		CursorX:  100,
		CursorY:  200,
		Agent:    got.Agent,
	}
	msg, err := client.Factory().BroadcastPresence(broadcastPayload)
	if err != nil {
		t.Fatalf("BroadcastPresence error: %v", err)
	}
	var decoded domainEntity.PayloadPresence
	if err := json.Unmarshal(msg.Payload, &decoded); err != nil {
		t.Fatalf("unmarshal BroadcastPresence payload: %v", err)
	}
	if decoded.Agent == nil || decoded.Agent.Name != "Canvas Copilot" {
		t.Fatalf("expected BroadcastPresence to preserve Agent, got %+v", decoded.Agent)
	}

	// Clearing Agent when subsequent presence update omits Agent.
	client.SetPresence(domainEntity.PayloadPresence{
		UserID:   "user-1",
		UserName: "Alice",
		CursorX:  150,
		CursorY:  250,
	})
	cleared := client.GetPresence()
	if cleared.Agent != nil {
		t.Fatalf("expected Agent to be cleared when omitted, got %+v", cleared.Agent)
	}
	if cleared.CursorX != 150 || cleared.CursorY != 250 {
		t.Fatalf("expected updated cursor (150, 250), got (%v, %v)", cleared.CursorX, cleared.CursorY)
	}
}
