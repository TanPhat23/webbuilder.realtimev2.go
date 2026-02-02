package handlers

import (
	"context"
	"encoding/json"

	wsentity "webbuilder/internal/delivery/ws/entity"
	domainEntity "webbuilder/internal/domain/entity"
	domainErrors "webbuilder/internal/domain/errors"
	"webbuilder/internal/utils/validation"
)

func (h *Handler) handleElementCreate(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	h.logger.Debug("handleElementCreate called")
	h.logger.Debugf("%v", msg)
	var payload domainEntity.PayloadElementCreate
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}


	if client.PageID != "" {
		payload.Element.PageID = client.PageID
	}
	if err := validation.ValidateElementCreate(&payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if err := h.uc.Element.AddElement(ctx, client.ProjectID, &payload.Element); err != nil {
		return err
	}

	broadcastMsg, _ := client.Factory().CreateElement(payload.Element)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) handleElementUpdate(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if client.PageID == "" {
		return domainErrors.ErrNotJoined
	}

	var payload domainEntity.PayloadElementUpdate
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if err := validation.ValidateElementUpdate(&payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	// Use the specific PatchElement logic moved to use case for better scalability/reusability
	if err := h.uc.Element.PatchElement(ctx, client.ProjectID, client.PageID, payload.ID, payload.Properties, payload.Styles, payload.Settings); err != nil {
		return err
	}

	broadcastMsg, _ := client.Factory().UpdateElement(payload)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) handleElementDelete(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if client.PageID == "" {
		return domainErrors.ErrNotJoined
	}

	var payload domainEntity.PayloadElementDelete
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	if err := h.uc.Element.RemoveElement(ctx, client.ProjectID, client.PageID, payload.ID); err != nil {
		return err
	}

	broadcastMsg, _ := client.Factory().DeleteElement(payload.ID)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) handleElementMove(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if client.PageID == "" {
		return domainErrors.ErrNotJoined
	}

	var payload domainEntity.PayloadElementMove
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	parentID := ""
	if payload.ParentID != nil {
		parentID = *payload.ParentID
	}

	if _, err := h.uc.Element.MoveElement(ctx, client.ProjectID, client.PageID, payload.ID, parentID, payload.Order); err != nil {
		return err
	}

	broadcastMsg, _ := client.Factory().MoveElement(payload)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}
