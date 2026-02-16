package entity

import (
	"sync"

	"github.com/gofiber/contrib/websocket"

	domainEntity "webbuilder/internal/domain/entity"
)

// Client represents a single websocket client connected to a page/room.
// It owns a buffered send channel which is used by a single writer goroutine.
type Client struct {
	ProjectID string
	Conn      *websocket.Conn
	PageID    string
	User      *domainEntity.User
	send      chan domainEntity.Message
	closeOnce sync.Once

	// Presence state: stores the latest cursor/selection information for the client.
	// Protected by PresenceMu when accessed concurrently.
	PresenceMu sync.Mutex
	CursorX    float64
	CursorY    float64
	ElementID  *string
}

// NewClient constructs a new client for the given connection, project, and user.
func NewClient(conn *websocket.Conn, projectID string, user *domainEntity.User) *Client {
	return &Client{
		Conn:      conn,
		ProjectID: projectID,
		User:      user,
	}
}

// CloseSend closes the client's send channel exactly once.
func (c *Client) CloseSend() {
	c.closeOnce.Do(func() {
		if c.send != nil {
			close(c.send)
		}
	})
}

// TrySend attempts to enqueue a message onto the client's send channel.
// Returns false if the channel is not initialized or if the client is slow
// (i.e., the channel buffer is full).
func (c *Client) TrySend(msg domainEntity.Message) bool {
	if c.send == nil {
		return false
	}
	select {
	case c.send <- msg:
		return true
	default:
		return false
	}
}

// Send attempts to enqueue the message to the client's send channel, falling
// back to a direct WriteJSON if the client is slow.
func (c *Client) Send(msg domainEntity.Message) error {
	if c.TrySend(msg) {
		return nil
	}
	return c.Conn.WriteJSON(msg)
}

// SetPageID sets the client's current page ID.
func (c *Client) SetPageID(pageID string) {
	c.PageID = pageID
}

// RoomID returns a composite room identifier to ensure uniqueness across projects.
func (c *Client) RoomID() string {
	if c.ProjectID == "" {
		return c.PageID
	}
	if c.PageID == "" {
		return c.ProjectID
	}
	return c.ProjectID + ":" + c.PageID
}

// Close gracefully closes the client connection and channel.
func (c *Client) Close() {
	c.CloseSend()
	_ = c.Conn.Close()
}

	func (c *Client) Factory() *MessageFactory {
		if c.User != nil && c.User.ID != "" {
			return NewMessageFactory(c.ProjectID, c.PageID, c.User.ID)
		}
		return NewMessageFactory(c.ProjectID, c.PageID)
	}

	func (c *Client) SetPresence(p domainEntity.PayloadPresence) {
		c.PresenceMu.Lock()
		defer c.PresenceMu.Unlock()
		c.CursorX = p.CursorX
		c.CursorY = p.CursorY
		if p.ElementID == "" {
			c.ElementID = nil
		} else {
			e := p.ElementID
			c.ElementID = &e
		}
	}

	func (c *Client) GetPresence() domainEntity.Presence {
		c.PresenceMu.Lock()
		defer c.PresenceMu.Unlock()
		return domainEntity.Presence{
			CursorX:   c.CursorX,
			CursorY:   c.CursorY,
			ElementID: c.ElementID,
		}
	}
