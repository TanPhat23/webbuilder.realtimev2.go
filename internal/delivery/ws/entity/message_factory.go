package entity

import (
	"encoding/json"
	"fmt"
	"time"

	domainEntity "webbuilder/internal/domain/entity"
)

// MessageFactory provides a structured way to create WebSocket messages
// within a specific project and page context.
type MessageFactory struct {
	projectID string
	pageID    string
}

// NewMessageFactory initializes a factory with routing context.
func NewMessageFactory(projectID, pageID string) *MessageFactory {
	return &MessageFactory{
		projectID: projectID,
		pageID:    pageID,
	}
}

// ElementCreate constructs a message for new element additions.
func (f *MessageFactory) ElementCreate(element domainEntity.Element) (domainEntity.Message, error) {
	return f.Create(domainEntity.TypeElementCreate, domainEntity.PayloadElementCreate{
		Element: element,
	})
}

// ElementUpdate constructs a message for granular property or style changes.
func (f *MessageFactory) ElementUpdate(payload domainEntity.PayloadElementUpdate) (domainEntity.Message, error) {
	return f.Create(domainEntity.TypeElementUpdate, payload)
}

// ElementDelete constructs a message for element removals.
func (f *MessageFactory) ElementDelete(elementID string) (domainEntity.Message, error) {
	return f.Create(domainEntity.TypeElementDelete, domainEntity.PayloadElementDelete{
		ID: elementID,
	})
}

// ElementMove constructs a message for reordering or reparenting elements.
func (f *MessageFactory) ElementMove(payload domainEntity.PayloadElementMove) (domainEntity.Message, error) {
	return f.Create(domainEntity.TypeElementMove, payload)
}

// Sync constructs a synchronization message with the full page state.
func (f *MessageFactory) Sync(elements []domainEntity.Element, users []domainEntity.UserPresence) (domainEntity.Message, error) {
	return f.Create(domainEntity.TypeSync, domainEntity.PayloadSync{
		Elements: elements,
		Users:    users,
	})
}

// Presence constructs a message for broadcasting live user activity.
func (f *MessageFactory) Presence(payload domainEntity.PayloadPresence) (domainEntity.Message, error) {
	return f.Create(domainEntity.TypePresence, payload)
}

// Error constructs a standardized error message.
func (f *MessageFactory) Error(code, errMsg string) domainEntity.Message {
	payload := domainEntity.PayloadError{
		Code:    code,
		Message: errMsg,
	}
	msg, _ := f.Create(domainEntity.TypeError, payload)
	return msg
}

// Create is the underlying engine for generating the Message envelope.
func (f *MessageFactory) Create(msgType domainEntity.MessageType, payload any) (domainEntity.Message, error) {
	var payloadData json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return domainEntity.Message{}, fmt.Errorf("failed to marshal payload: %w", err)
		}
		payloadData = json.RawMessage(b)
	}

	return domainEntity.Message{
		Type:      msgType,
		ProjectID: f.projectID,
		PageID:    f.pageID,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payloadData,
	}, nil
}

// DecodePayload unmarshals raw payload data into a concrete type.
func DecodePayload[T any](m domainEntity.Message) (T, error) {
	var dst T
	if len(m.Payload) == 0 {
		return dst, nil
	}
	if err := json.Unmarshal(m.Payload, &dst); err != nil {
		return dst, fmt.Errorf("failed to decode payload: %w", err)
	}
	return dst, nil
}
