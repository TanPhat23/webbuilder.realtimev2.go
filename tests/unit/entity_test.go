package unit

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	wsentity "webbuilder/internal/delivery/ws/entity"
	domainEntity "webbuilder/internal/domain/entity"
)

// mockLogger implements logger.Logger for testing purposes.
type mockLogger struct{}

func (m *mockLogger) Info(args ...interface{})                  {}
func (m *mockLogger) Infof(format string, args ...interface{})  {}
func (m *mockLogger) Error(args ...interface{})                 {}
func (m *mockLogger) Errorf(format string, args ...interface{}) {}
func (m *mockLogger) Debug(args ...interface{})                 {}
func (m *mockLogger) Debugf(format string, args ...interface{}) {}
func (m *mockLogger) Fatal(args ...interface{})                 {}
func (m *mockLogger) Fatalf(format string, args ...interface{}) {}

func TestClient_RoomID(t *testing.T) {
	cases := []struct {
		name      string
		projectID string
		pageID    string
		want      string
	}{
		{"both IDs present", "proj-1", "page-1", "proj-1:page-1"},
		{"only project ID", "proj-1", "", "proj-1"},
		{"only page ID", "", "page-1", "page-1"},
		{"no IDs", "", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// websocket.Conn is nil here as we are only testing RoomID logic
			c := wsentity.NewClient(nil, tc.projectID, nil)
			c.SetPageID(tc.pageID)
			if got := c.RoomID(); got != tc.want {
				t.Errorf("RoomID() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMessageFactory_Create(t *testing.T) {
	projectID := "test-project"
	pageID := "test-page"
	f := wsentity.NewMessageFactory(projectID, pageID)

	t.Run("create with payload", func(t *testing.T) {
		payload := domainEntity.PayloadPresence{
			UserID:   "user-123",
			UserName: "Tester",
		}
		msg, err := f.Create(domainEntity.TypePresence, payload)
		if err != nil {
			t.Fatalf("failed to create message: %v", err)
		}

		if msg.Type != domainEntity.TypePresence {
			t.Errorf("expected type %s, got %s", domainEntity.TypePresence, msg.Type)
		}
		if msg.ProjectID != projectID {
			t.Errorf("expected projectID %s, got %s", projectID, msg.ProjectID)
		}
		if msg.PageID != pageID {
			t.Errorf("expected pageID %s, got %s", pageID, msg.PageID)
		}

		var decoded domainEntity.PayloadPresence
		if err := json.Unmarshal(msg.Payload, &decoded); err != nil {
			t.Fatalf("failed to unmarshal payload: %v", err)
		}
		if decoded.UserID != payload.UserID || decoded.UserName != payload.UserName {
			t.Errorf("payload data mismatch")
		}
	})

	t.Run("create without payload", func(t *testing.T) {
		msg, err := f.Create(domainEntity.TypeLeave, nil)
		if err != nil {
			t.Fatalf("failed to create message: %v", err)
		}
		if msg.Payload != nil {
			t.Errorf("expected nil payload, got %v", msg.Payload)
		}
	})
}

func TestMessageFactory_Helpers(t *testing.T) {
	f := wsentity.NewMessageFactory("p1", "pg1")

	t.Run("ElementDelete", func(t *testing.T) {
		id := "el-delete-id"
		msg, _ := f.DeleteElement(id)
		if msg.Type != domainEntity.TypeElementDelete {
			t.Errorf("wrong type: %s", msg.Type)
		}

		var payload domainEntity.PayloadElementDelete
		json.Unmarshal(msg.Payload, &payload)
		if payload.ID != id {
			t.Errorf("expected id %s, got %s", id, payload.ID)
		}
	})

	t.Run("Error", func(t *testing.T) {
		code := "TEST_ERROR"
		errMsg := "something went wrong"
		msg := f.Error(code, errMsg)

		if msg.Type != domainEntity.TypeError {
			t.Errorf("wrong type: %s", msg.Type)
		}

		var payload domainEntity.PayloadError
		json.Unmarshal(msg.Payload, &payload)
		if payload.Code != code || payload.Message != errMsg {
			t.Errorf("unexpected error payload: %+v", payload)
		}
	})
}

func TestDecodePayload(t *testing.T) {
	data := []byte(`{"id": "element-1", "order": 5}`)
	msg := domainEntity.Message{
		Payload: json.RawMessage(data),
	}

	decoded, err := wsentity.DecodePayload[domainEntity.PayloadElementDelete](msg)
	if err != nil {
		t.Fatalf("DecodePayload failed: %v", err)
	}

	if decoded.ID != "element-1" {
		t.Errorf("expected ID element-1, got %s", decoded.ID)
	}
}

func TestHub_Basic(t *testing.T) {
	ctx := context.Background()
	wg := &sync.WaitGroup{}
	lg := &mockLogger{}

	hub := wsentity.NewHub(ctx, wg, 64, nil, lg)
	if hub == nil {
		t.Fatal("failed to create hub")
	}

	t.Run("get participants of non-existent room", func(t *testing.T) {
		users := hub.GetRoomParticipants("none")
		if len(users) != 0 {
			t.Errorf("expected 0 users, got %d", len(users))
		}
	})

	t.Run("register nil client", func(t *testing.T) {
		err := hub.Register(ctx, nil)
		if err != nil {
			t.Errorf("expected no error registering nil client, got %v", err)
		}
	})

	t.Run("unregister nil client", func(t *testing.T) {
		err := hub.Unregister(ctx, nil)
		if err != nil {
			t.Errorf("expected no error unregistering nil client, got %v", err)
		}
	})
}

func TestRoom_Initialization(t *testing.T) {
	ctx := context.Background()
	wg := &sync.WaitGroup{}
	lg := &mockLogger{}
	roomID := "room-123"

	room := wsentity.NewRoom(roomID, ctx, wg, 10, lg)
	if room == nil {
		t.Fatal("failed to create room")
	}

	if room.PageID != roomID {
		t.Errorf("expected room ID %s, got %s", roomID, room.PageID)
	}

	t.Run("get participants from empty room", func(t *testing.T) {
		users := room.GetParticipants()
		if len(users) != 0 {
			t.Errorf("expected 0 participants, got %d", len(users))
		}
	})
}
