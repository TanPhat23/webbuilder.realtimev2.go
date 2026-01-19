package entity

import (
	"encoding/json"
	"time"
)

// MessageType defines the action being performed via WebSocket
type MessageType string

const (
	// Presence & Connection
	TypeJoin     MessageType = "join"     // User joined page
	TypeLeave    MessageType = "leave"    // User left page
	TypePresence MessageType = "presence" // Cursor movement/selection

	// Element Operations
	TypeElementCreate  MessageType = "element:create"  // New element added
	TypeElementUpdate  MessageType = "element:update"  // Granular update (styles, props)
	TypeElementDelete  MessageType = "element:delete"  // Element removed
	TypeElementMove    MessageType = "element:move"    // Element reordered or reparented
	TypeElementContent MessageType = "element:content" // Real-time text/content editing

	// Collaboration Features
	TypeCommentCreate MessageType = "comment:create" // New comment added
	TypeCommentUpdate MessageType = "comment:update" // Comment resolved/edited
	TypeCommentDelete MessageType = "comment:delete" // Comment removed

	// Synchronization
	TypeSync     MessageType = "sync"     // Initial state request/response
	TypeSnapshot MessageType = "snapshot" // Save/Load full state
	TypeError    MessageType = "error"    // Server-side error reporting
)

// Message is the top-level envelope for all WebSocket communication
type Message struct {
	Type      MessageType     `json:"type"`
	ProjectID string          `json:"projectId"`
	PageID    string          `json:"pageId"`
	UserID    string          `json:"userId,omitempty"`
	Timestamp int64           `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// PayloadPresence tracks user activity for "Who's here" and cursor pins
type PayloadPresence struct {
	UserID    string         `json:"userId"`
	UserName  string         `json:"userName"`
	CursorX   float64        `json:"cursorX"`
	CursorY   float64        `json:"cursorY"`
	ElementID string         `json:"elementId,omitempty"` 
	Meta      map[string]any `json:"meta,omitempty"`
}

// PayloadElementCreate used when a new component is dragged onto the canvas
type PayloadElementCreate struct {
	Element Element `json:"element"`
}

// PayloadElementUpdate for granular patches to avoid sending the whole object
type PayloadElementUpdate struct {
	ID         string         `json:"id"`
	Properties map[string]any `json:"properties,omitempty"`
	Styles     map[string]any `json:"styles,omitempty"`
	Settings   *Setting       `json:"settings,omitempty"`
}

// PayloadElementMove for drag-and-drop reordering
type PayloadElementMove struct {
	ID       string  `json:"id"`
	ParentID *string `json:"parentId"` // Null if root
	Order    int     `json:"order"`
}

// PayloadElementDelete
type PayloadElementDelete struct {
	ID string `json:"id"`
}

// PayloadElementContent for real-time text input synchronization
type PayloadElementContent struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

// PayloadComment for real-time feedback
type PayloadComment struct {
	ElementID string         `json:"elementId"`
	Comment   ElementComment `json:"comment"`
}

// PayloadSync provides the initial state to a joining client
type PayloadSync struct {
	Elements []Element      `json:"elements"`
	Users    []UserPresence `json:"users"` // Current active users on page
}

// PayloadSnapshot allows the client to request a forced save to DB
type PayloadSnapshot struct {
	Name string       `json:"name"`
	Type SnapshotType `json:"type"` // "working" or "version"
}

// PayloadError
type PayloadError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// UserPresence is a lightweight view of an active collaborator
type UserPresence struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	LastSeen  time.Time `json:"lastSeen"`
	ActiveOn  string    `json:"activeOn"` // ElementID being edited
}

// NewMessage helper to construct a standard response
func NewMessage(msgType MessageType, projectID, pageID string, payload any) (*Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Message{
		Type:      msgType,
		ProjectID: projectID,
		PageID:    pageID,
		Timestamp: time.Now().UnixMilli(),
		Payload:   data,
	}, nil
}
