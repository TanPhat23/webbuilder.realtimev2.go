package handlers

import (
	"context"
	"encoding/json"

	wsentity "webbuilder/internal/delivery/ws/entity"
	domainEntity "webbuilder/internal/domain/entity"
	domainErrors "webbuilder/internal/domain/errors"
)

func (h *Handler) handleJoin(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if msg.PageID == "" {
		return domainErrors.New(domainErrors.CodeValidation, "pageId is required to join", nil)
	}

	oldPage := client.PageID
	if oldPage != "" && oldPage != msg.PageID {
		_ = h.hub.Unregister(ctx, client)
	}

	client.SetPageID(msg.PageID)
	if err := h.hub.Register(ctx, client); err != nil {
		return err
	}

	// Send initial synchronization state (refresh cache and propagate errors)
	elements, err := h.uc.Element.SyncPage(ctx, client.ProjectID, client.PageID)
	if err != nil {
		return err
	}
	workflows, _ := h.uc.Workflow.GetWorkflowsByProject(ctx, client.ProjectID)
	elementWorkflows, _ := h.uc.ElementWorkflows.GetWorkflowsByPage(ctx, client.ProjectID, client.PageID)

	users := h.hub.GetRoomParticipants(client.RoomID())
	syncMsg, _ := client.Factory().Sync(elements, workflows, elementWorkflows, users)
	_ = client.Send(syncMsg)
	return nil
}

func (h *Handler) handleLeave(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if client.PageID != "" {
		_ = h.hub.Unregister(ctx, client)
		client.SetPageID("")
	}
	return nil
}

func (h *Handler) handlePresence(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	var payload domainEntity.PayloadPresence
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return domainErrors.Wrap(domainErrors.CodeValidation, err)
	}

	broadcastMsg, _ := client.Factory().Presence(payload)
	h.hub.Broadcast(client.RoomID(), broadcastMsg, client)
	return nil
}

func (h *Handler) handleSync(ctx context.Context, client *wsentity.Client, msg domainEntity.Message) error {
	if client.PageID == "" {
		return domainErrors.ErrNotJoined
	}

	elements, err := h.uc.Element.SyncPage(ctx, client.ProjectID, client.PageID)
	if err != nil {
		return err
	}

	workflows, _ := h.uc.Workflow.GetWorkflowsByProject(ctx, client.ProjectID)
	elementWorkflows, _ := h.uc.ElementWorkflows.GetWorkflowsByPage(ctx, client.ProjectID, client.PageID)

	users := h.hub.GetRoomParticipants(client.RoomID())
	syncMsg, _ := client.Factory().Sync(elements, workflows, elementWorkflows, users)
	_ = client.Send(syncMsg)
	return nil
}